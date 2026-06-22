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
	"math"
	"sort"
	"time"

	"github.com/vearch/vearch/v3/internal/entity"
	"github.com/vearch/vearch/v3/internal/pkg/log"
)

// Planner periodically scans the snapshot and produces migration ops.
type Planner struct {
	builder     *SnapshotBuilder
	scheduler   *TaskScheduler
	taskTracker *TaskTracker
	cfgHolder   *configHolder
}

func NewPlanner(builder *SnapshotBuilder, sched *TaskScheduler, tracker *TaskTracker, cfg *configHolder) *Planner {
	return &Planner{builder: builder, scheduler: sched, taskTracker: tracker, cfgHolder: cfg}
}

// PlanOnce runs one planning round.
func (p *Planner) PlanOnce(ctx context.Context) error {
	cfg := p.cfgHolder.Get()
	if !cfg.Enabled {
		return nil
	}

	// GC expired cooldowns each round.
	if removed := p.taskTracker.GCExpiredCooldowns(time.Now().UnixMilli()); removed > 0 {
		log.Debugf("[balancer] GC expired cooldowns: %d", removed)
	}

	snap, err := p.builder.Build(ctx)
	if err != nil {
		return err
	}

	ops := p.collectOps(snap, cfg)
	if len(ops) == 0 {
		return nil
	}
	log.Infof("[balancer] planner generated %d ops", len(ops))

	ops = p.cap(ops, cfg)

	for _, op := range ops {
		if _, err := p.scheduler.Submit(ctx, op); err != nil {
			log.Errorf("[balancer] submit op failed: %s", err.Error())
		}
	}
	return nil
}

// collectOps unions ops from three trigger conditions.
func (p *Planner) collectOps(snap *Snapshot, cfg *Config) []MigrateOp {
	ops := make([]MigrateOp, 0)

	// Trigger 1: disk pressure (highest priority).
	for _, item := range snap.LiveItems() {
		if item.Stat != nil && item.Stat.DiskUsage > cfg.DiskRejectThreshold {
			ops = append(ops, p.relieveDiskFull(item, snap, cfg)...)
		}
	}

	// Trigger 2: uneven partition count (topology fix first).
	signatures := p.uniqueSignatures(snap)
	for _, sig := range signatures {
		ops = append(ops, p.balancePartitionCount(sig, snap, cfg)...)
	}

	/*
		Trigger 3: data volume skew (only after partition counts are balanced).
		Compare absolute spread against PartitionDiffThreshold (stdDev > 0
		would almost always be true and skip data balancing).
	*/
	for _, sig := range signatures {
		items := snap.LiveItems()
		maxC, minC := maxMinPartitionCountForSig(sig, items, snap)
		if maxC-minC > cfg.PartitionDiffThreshold {
			// Partition counts not yet balanced; do not move data.
			continue
		}
		ops = append(ops, p.balanceDataBytes(sig, snap, cfg)...)
	}

	return p.dedup(ops)
}

// maxMinPartitionCountForSig returns the max/min partition counts across live nodes.
func maxMinPartitionCountForSig(sig entity.SpaceSignature, items []*PSItem, snap *Snapshot) (int, int) {
	if len(items) == 0 {
		return 0, 0
	}
	first := items[0].PartitionCountForSig(sig, snap)
	maxC, minC := first, first
	for _, it := range items[1:] {
		c := it.PartitionCountForSig(sig, snap)
		if c > maxC {
			maxC = c
		}
		if c < minC {
			minC = c
		}
	}
	return maxC, minC
}

func (p *Planner) uniqueSignatures(snap *Snapshot) []entity.SpaceSignature {
	set := make(map[entity.SpaceSignature]bool)
	for _, sig := range snap.signatureOfSpc {
		set[sig] = true
	}
	out := make([]entity.SpaceSignature, 0, len(set))
	for sig := range set {
		out = append(out, sig)
	}
	return out
}

/*
balancePartitionCount hard-balances per-signature partition counts.
Uses two pointers + local bubbling to keep complexity at O(N log N).
*/
func (p *Planner) balancePartitionCount(sig entity.SpaceSignature, snap *Snapshot, cfg *Config) []MigrateOp {
	items := snap.LiveItems()
	if len(items) < 2 {
		return nil
	}

	type nodeCount struct {
		item  *PSItem
		count int
	}
	counts := make([]nodeCount, 0, len(items))
	for _, item := range items {
		counts = append(counts, nodeCount{item, item.PartitionCountForSig(sig, snap)})
	}
	// Initial sort: O(N log N).
	sort.Slice(counts, func(i, j int) bool { return counts[i].count > counts[j].count })

	if counts[0].count-counts[len(counts)-1].count <= cfg.PartitionDiffThreshold {
		return nil
	}

	ops := make([]MigrateOp, 0)
	hi, lo := 0, len(counts)-1
	for hi < lo {
		if counts[hi].count-counts[lo].count <= cfg.PartitionDiffThreshold {
			break
		}
		op := p.pickVictim(counts[hi].item, counts[lo].item, sig, snap, cfg, "partition_skew")
		if op != nil {
			ops = append(ops, *op)
			counts[hi].count--
			counts[lo].count++
			// Bubble hi backward (O(N) worst-case, typically 1–2 steps).
			for hi+1 < lo && counts[hi].count < counts[hi+1].count {
				counts[hi], counts[hi+1] = counts[hi+1], counts[hi]
				hi++
			}
			// Bubble lo forward.
			for lo-1 > hi && counts[lo].count > counts[lo-1].count {
				counts[lo], counts[lo-1] = counts[lo-1], counts[lo]
				lo--
			}
		} else {
			// No suitable victim on hi; advance to the next hot node.
			hi++
		}
	}
	return ops
}

// balanceDataBytes moves data when nodes with the same signature are skewed.
func (p *Planner) balanceDataBytes(sig entity.SpaceSignature, snap *Snapshot, cfg *Config) []MigrateOp {
	items := snap.LiveItems()
	if len(items) < 2 {
		return nil
	}

	stdDev := DataBytesStdDev(items)
	if stdDev < cfg.DataBytesSkewThreshold {
		return nil
	}

	sorted := make([]*PSItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Priority > sorted[j].Priority })

	high := sorted[0]
	low := sorted[len(sorted)-1]
	op := p.pickVictim(high, low, sig, snap, cfg, "data_skew")
	if op == nil {
		return nil
	}
	return []MigrateOp{*op}
}

/*
relieveDiskFull migrates partitions out of disk-pressured nodes.
Target must also be under DiskRejectThreshold; otherwise we just shift
the problem and may trigger cascading migrations.
*/
func (p *Planner) relieveDiskFull(hot *PSItem, snap *Snapshot, cfg *Config) []MigrateOp {
	var target *PSItem
	minDiskUsage := math.MaxFloat64
	for _, item := range snap.LiveItems() {
		if item.NodeID == hot.NodeID || item.IsInWarmup {
			continue
		}
		if item.Stat == nil {
			continue
		}
		if item.Stat.DiskUsage >= cfg.DiskRejectThreshold {
			continue
		}
		if item.Stat.DiskUsage < minDiskUsage {
			minDiskUsage = item.Stat.DiskUsage
			target = item
		}
	}
	if target == nil {
		// Whole cluster is over threshold — no valid target, defer to alerts / capacity work.
		log.Warnf("[balancer] node %d disk full but no target below threshold; skipping",
			hot.NodeID)
		return nil
	}
	for _, sig := range p.uniqueSignatures(snap) {
		op := p.pickVictim(hot, target, sig, snap, cfg, "disk_full")
		if op != nil {
			return []MigrateOp{*op}
		}
	}
	return nil
}

// pickVictim picks a partition on source to migrate to target.
func (p *Planner) pickVictim(
	source, target *PSItem, sig entity.SpaceSignature,
	snap *Snapshot, cfg *Config, reason string,
) *MigrateOp {
	candidates := make([]*entity.Partition, 0)
	for _, part := range snap.allPartitions {
		if snap.signatureOfSpc[part.SpaceId] != sig {
			continue
		}
		if !containsReplica(part.Replicas, source.NodeID) {
			continue
		}
		if !cfg.AllowLeaderMigration && part.LeaderID == source.NodeID {
			continue
		}
		if containsReplica(part.Replicas, target.NodeID) {
			continue
		}
		if snap.taskTracker != nil && snap.taskTracker.InCooldown(part.Id, time.Now().UnixMilli()) {
			continue
		}
		candidates = append(candidates, part)
	}
	if len(candidates) == 0 {
		return nil
	}

	// Use exact partition-level data volume.
	type partWithSize struct {
		p    *entity.Partition
		size uint64
	}
	withSize := make([]partWithSize, 0, len(candidates))
	for _, part := range candidates {
		size := uint64(0)
		if source.Stat != nil {
			if load, ok := source.Stat.SpaceStats[part.SpaceId]; ok {
				if load.PartitionBytes != nil {
					if exact, found := load.PartitionBytes[part.Id]; found {
						size = exact
					}
				}
				if size == 0 && load.PartitionCount > 0 {
					size = load.DataBytes / uint64(load.PartitionCount)
				}
			}
		}
		withSize = append(withSize, partWithSize{part, size})
	}
	sort.Slice(withSize, func(i, j int) bool { return withSize[i].size < withSize[j].size })
	median := withSize[len(withSize)/2]

	if !p.hasEnoughBenefit(source, target, median.size, cfg) {
		return nil
	}

	return &MigrateOp{
		PartitionID:    median.p.Id,
		SpaceID:        median.p.SpaceId,
		FromNodeID:     source.NodeID,
		ToNodeID:       target.NodeID,
		EstimatedBytes: median.size,
		Reason:         reason,
	}
}

func (p *Planner) hasEnoughBenefit(source, target *PSItem, bytes uint64, cfg *Config) bool {
	if cfg.MinBenefitRatio <= 0 {
		return true
	}
	diffBefore := math.Abs(source.CurrentScore - target.CurrentScore)
	if diffBefore == 0 {
		return false
	}
	srcAfter := source.CurrentScore - float64(bytes)
	tgtAfter := target.CurrentScore + float64(bytes)
	diffAfter := math.Abs(srcAfter - tgtAfter)
	return (diffBefore - diffAfter) > diffBefore*cfg.MinBenefitRatio
}

/*
dedup keeps only the highest-priority op per partition.
Sort by reason priority explicitly so the result is order-independent.
*/
func (p *Planner) dedup(ops []MigrateOp) []MigrateOp {
	if len(ops) == 0 {
		return ops
	}
	prio := map[string]int{
		"disk_full":      3,
		"partition_skew": 2,
		"data_skew":      1,
		"manual":         4,
	}
	pri := func(r string) int {
		if v, ok := prio[r]; ok {
			return v
		}
		return 0
	}
	sort.SliceStable(ops, func(i, j int) bool {
		return pri(ops[i].Reason) > pri(ops[j].Reason)
	})

	seen := make(map[entity.PartitionID]bool)
	out := make([]MigrateOp, 0, len(ops))
	for _, op := range ops {
		if seen[op.PartitionID] {
			continue
		}
		seen[op.PartitionID] = true
		out = append(out, op)
	}
	return out
}

func (p *Planner) cap(ops []MigrateOp, cfg *Config) []MigrateOp {
	out := make([]MigrateOp, 0, len(ops))
	perSpace := make(map[entity.SpaceID]int)
	for _, op := range ops {
		if len(out) >= cfg.MaxOpsPerRound {
			break
		}
		inflightForSpace := p.taskTracker.CountInflightForSpace(op.SpaceID)
		if perSpace[op.SpaceID]+inflightForSpace >= cfg.MaxOpsPerSpacePerRound {
			continue
		}
		perSpace[op.SpaceID]++
		out = append(out, op)
	}
	return out
}
