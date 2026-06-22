// Copyright 2019 The Vearch Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package balancer

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/vearch/vearch/v3/internal/client"
	"github.com/vearch/vearch/v3/internal/entity"
	"github.com/vearch/vearch/v3/internal/pkg/log"
	json "github.com/vearch/vearch/v3/internal/pkg/vjson"
	"golang.org/x/sync/singleflight"
)

// Snapshot is a point-in-time view of cluster load.
type Snapshot struct {
	cfg            *Config
	allServers     map[entity.NodeID]*entity.Server
	allPSStats     map[entity.NodeID]*entity.PSStat
	allSpaces      map[entity.SpaceID]*entity.Space
	allPartitions  map[entity.PartitionID]*entity.Partition
	psItems        map[entity.NodeID]*PSItem
	signatureOfSpc map[entity.SpaceID]entity.SpaceSignature
	taskTracker    *TaskTracker

	/*
		Placement overlay for in-batch picks not yet persisted to etcd.
		Hook calls RecordPendingPlacement after picking; PartitionCountForSig
		adds these counts so a 5s cache window cannot cause the same node to
		be picked repeatedly for sibling partitions in one CreateSpace batch.
	*/
	pendingMu       sync.Mutex
	pendingBySigByN map[entity.SpaceSignature]map[entity.NodeID]int
}

/*
SnapshotBuilder builds Snapshots.
Has a short TTL cache + singleflight: avoids scanning etcd on every hook /
cron call, and ensures concurrent misses share one buildLocked invocation
(so pending placements recorded on the result are not lost to a racing build).
*/
type SnapshotBuilder struct {
	cli         *client.Client
	cfgHolder   *configHolder
	taskTracker *TaskTracker

	mu       sync.Mutex
	cached   *Snapshot
	cachedAt time.Time

	sf singleflight.Group
}

func NewSnapshotBuilder(cli *client.Client, cfg *configHolder, tracker *TaskTracker) *SnapshotBuilder {
	return &SnapshotBuilder{cli: cli, cfgHolder: cfg, taskTracker: tracker}
}

// snapshotCacheTTL bounds the snapshot reuse window.
const snapshotCacheTTL = 5 * time.Second

// Build returns a cluster snapshot, using a short TTL cache deduplicated by singleflight.
func (sb *SnapshotBuilder) Build(ctx context.Context) (*Snapshot, error) {
	sb.mu.Lock()
	if sb.cached != nil && time.Since(sb.cachedAt) < snapshotCacheTTL {
		s := sb.cached
		sb.mu.Unlock()
		return s, nil
	}
	sb.mu.Unlock()

	/*
		Concurrent misses share one buildLocked; all callers receive the same
		*Snapshot instance so they share one pendingBySigByN.
	*/
	v, err, _ := sb.sf.Do("build", func() (interface{}, error) {
		/*
			Recheck cache: another sf cohort may have populated it between the
			outer Unlock and this point; reuse to save a buildLocked.
		*/
		sb.mu.Lock()
		if sb.cached != nil && time.Since(sb.cachedAt) < snapshotCacheTTL {
			s := sb.cached
			sb.mu.Unlock()
			return s, nil
		}
		sb.mu.Unlock()

		snap, err := sb.buildLocked(ctx)
		if err != nil {
			return nil, err
		}
		sb.mu.Lock()
		sb.cached = snap
		sb.cachedAt = time.Now()
		sb.mu.Unlock()
		return snap, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*Snapshot), nil
}

// Invalidate forcibly clears the cache (e.g. after a raft config change).
func (sb *SnapshotBuilder) Invalidate() {
	sb.mu.Lock()
	sb.cached = nil
	sb.cachedAt = time.Time{}
	sb.mu.Unlock()
}

// buildLocked performs the actual snapshot build (no caching).
func (sb *SnapshotBuilder) buildLocked(ctx context.Context) (*Snapshot, error) {
	cfg := sb.cfgHolder.Get()
	snap := &Snapshot{
		cfg:             cfg,
		allServers:      make(map[entity.NodeID]*entity.Server),
		allPSStats:      make(map[entity.NodeID]*entity.PSStat),
		allSpaces:       make(map[entity.SpaceID]*entity.Space),
		allPartitions:   make(map[entity.PartitionID]*entity.Partition),
		psItems:         make(map[entity.NodeID]*PSItem),
		signatureOfSpc:  make(map[entity.SpaceID]entity.SpaceSignature),
		taskTracker:     sb.taskTracker,
		pendingBySigByN: make(map[entity.SpaceSignature]map[entity.NodeID]int),
	}

	// Fetch all PS nodes.
	servers, err := sb.cli.Master().QueryServers(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range servers {
		snap.allServers[s.ID] = s
	}

	// Fetch all PSStat entries from etcd.
	_, values, err := sb.cli.Master().Store.PrefixScan(ctx, entity.PrefixPSStat)
	if err != nil {
		return nil, err
	}
	for _, v := range values {
		stat := &entity.PSStat{}
		if err := json.Unmarshal(v, stat); err != nil {
			log.Warnf("[balancer] unmarshal PSStat failed: %s", err.Error())
			continue
		}
		snap.allPSStats[stat.NodeID] = stat
	}

	// Fetch spaces and compute signatures.
	spaces, err := sb.cli.Master().QuerySpacesByKey(ctx, entity.PrefixSpace)
	if err != nil {
		return nil, err
	}
	for _, sp := range spaces {
		snap.allSpaces[sp.Id] = sp
		snap.signatureOfSpc[sp.Id] = sp.Signature()
	}

	// Fetch all partitions.
	partitions, err := sb.cli.Master().QueryPartitions(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range partitions {
		snap.allPartitions[p.Id] = p
	}

	// Build PSItems.
	snap.buildPSItems(cfg)

	// Emit metrics.
	updateSnapshotMetrics(snap)

	return snap, nil
}

func (snap *Snapshot) buildPSItems(cfg *Config) {
	for nodeID, srv := range snap.allServers {
		stat := snap.allPSStats[nodeID]
		item := &PSItem{
			NodeID:     nodeID,
			Server:     srv,
			Stat:       stat,
			IsLive:     stat != nil && !stat.IsStale(int64(cfg.PSStatTTLSec)),
			IsInWarmup: stat == nil,
		}
		if stat != nil {
			item.CurrentScore = float64(stat.TotalDataBytes)
		}
		snap.psItems[nodeID] = item
	}

	// Memory-weighted AssignedScore.
	var totalScore float64
	var totalMem uint64
	for _, item := range snap.psItems {
		if !item.IsLive {
			continue
		}
		totalScore += item.CurrentScore
		totalMem += item.Stat.MemCapacityBytes
	}
	if totalMem > 0 {
		avgPerByte := totalScore / float64(totalMem)
		for _, item := range snap.psItems {
			if !item.IsLive {
				continue
			}
			item.AddAssignedScore(float64(item.Stat.MemCapacityBytes) * avgPerByte)
		}
	}

	// Apply TaskTracker delta.
	if snap.taskTracker != nil {
		for _, item := range snap.psItems {
			delta := snap.taskTracker.GetTaskDelta(item.NodeID)
			item.AddCurrentScoreDelta(delta)
		}
	}
}

// SpaceSignatureOf returns the signature for a given spaceID.
func (snap *Snapshot) SpaceSignatureOf(spaceID entity.SpaceID) entity.SpaceSignature {
	return snap.signatureOfSpc[spaceID]
}

// LiveItems returns all live PSItems.
func (snap *Snapshot) LiveItems() []*PSItem {
	items := make([]*PSItem, 0, len(snap.psItems))
	for _, item := range snap.psItems {
		if item.IsLive {
			items = append(items, item)
		}
	}
	return items
}

// AllPartitions returns the partition map view.
func (snap *Snapshot) AllPartitions() map[entity.PartitionID]*entity.Partition {
	return snap.allPartitions
}

// AllSpaces returns the space map view.
func (snap *Snapshot) AllSpaces() map[entity.SpaceID]*entity.Space {
	return snap.allSpaces
}

// PSItem returns the PSItem for a node.
func (snap *Snapshot) PSItem(nodeID entity.NodeID) *PSItem {
	return snap.psItems[nodeID]
}

/*
AvgPartitionCountForSig returns the average per-node partition count for a signature.
Includes pending placement so it stays symmetric with PSItem.PartitionCountForSig
(otherwise Planner would see inflated individual counts vs. a stale average
and trigger spurious migrations during heavy in-batch placement).
*/
func (snap *Snapshot) AvgPartitionCountForSig(sig entity.SpaceSignature, allowedNodes []entity.NodeID) float64 {
	if len(allowedNodes) == 0 {
		return 0
	}
	totalPartitions := 0
	for _, p := range snap.allPartitions {
		if snap.signatureOfSpc[p.SpaceId] == sig {
			totalPartitions += len(p.Replicas)
		}
	}
	// Add pending placements (mirror PartitionCountForSig).
	snap.pendingMu.Lock()
	if m := snap.pendingBySigByN[sig]; m != nil {
		for _, c := range m {
			totalPartitions += c
		}
	}
	snap.pendingMu.Unlock()
	return float64(totalPartitions) / float64(len(allowedNodes))
}

// FilterByResource filters candidates by ResourceName / Private / allowed IPs.
func (snap *Snapshot) FilterByResource(items []*PSItem, resourceName string, allowPrivate bool, allowedIPs map[string]bool) []*PSItem {
	out := make([]*PSItem, 0, len(items))
	rn := strings.TrimSpace(resourceName)
	if rn == "" {
		rn = "default"
	}
	for _, item := range items {
		if item.Server == nil {
			continue
		}
		srvRn := strings.TrimSpace(item.Server.ResourceName)
		if srvRn == "" {
			srvRn = "default"
		}
		if srvRn != rn {
			continue
		}
		if !allowPrivate && item.Server.Private {
			continue
		}
		if len(allowedIPs) > 0 && !allowedIPs[item.Server.Ip] {
			continue
		}
		out = append(out, item)
	}
	return out
}

// FreshnessWindow is the staleness threshold for PSStat.
func (snap *Snapshot) FreshnessWindow() time.Duration {
	return time.Duration(snap.cfg.PSStatTTLSec) * time.Second
}

/*
RecordPendingPlacement registers a pending placement after a hook pick, so
later hook calls within the same cache window account for the occupancy.
*/
func (snap *Snapshot) RecordPendingPlacement(sig entity.SpaceSignature, nodeIDs []entity.NodeID) {
	if len(nodeIDs) == 0 {
		return
	}
	snap.pendingMu.Lock()
	defer snap.pendingMu.Unlock()
	if snap.pendingBySigByN == nil {
		snap.pendingBySigByN = make(map[entity.SpaceSignature]map[entity.NodeID]int)
	}
	m, ok := snap.pendingBySigByN[sig]
	if !ok {
		m = make(map[entity.NodeID]int)
		snap.pendingBySigByN[sig] = m
	}
	for _, id := range nodeIDs {
		m[id]++
	}
}

// PendingForSigOnNode returns the pending placement count for a signature on a node.
func (snap *Snapshot) PendingForSigOnNode(sig entity.SpaceSignature, nodeID entity.NodeID) int {
	snap.pendingMu.Lock()
	defer snap.pendingMu.Unlock()
	if m := snap.pendingBySigByN[sig]; m != nil {
		return m[nodeID]
	}
	return 0
}
