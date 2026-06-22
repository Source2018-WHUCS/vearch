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
	"fmt"
	"sort"

	"github.com/vearch/vearch/v3/internal/entity"
	"github.com/vearch/vearch/v3/internal/proto/vearchpb"
)

// PickerOptions are extra constraints when picking PSes.
type PickerOptions struct {
	ReplicaCount         int
	ResourceName         string
	AllowedIPs           map[string]bool
	AllowPrivate         bool
	AntiAffinityStrategy int
	Excluded             map[entity.NodeID]bool
}

// Picker selects PSes (partition count hard constraint + data score + anti-affinity).
type Picker struct {
	snap *Snapshot
	cfg  *Config
}

func NewPicker(snap *Snapshot, cfg *Config) *Picker {
	return &Picker{snap: snap, cfg: cfg}
}

// PickByScore picks N PSes for a space.
func (p *Picker) PickByScore(s *entity.Space, opts PickerOptions) ([]entity.NodeID, error) {
	if opts.ReplicaCount <= 0 {
		return []entity.NodeID{}, nil
	}

	candidates := p.snap.LiveItems()
	candidates = p.snap.FilterByResource(candidates, opts.ResourceName, opts.AllowPrivate, opts.AllowedIPs)

	// Hard filter 1: disk threshold + explicit exclusions + skip warmup nodes.
	out := make([]*PSItem, 0, len(candidates))
	for _, item := range candidates {
		if item.IsInWarmup {
			continue
		}
		if item.Stat != nil && item.Stat.DiskUsage > p.cfg.DiskRejectThreshold {
			continue
		}
		if opts.Excluded != nil && opts.Excluded[item.NodeID] {
			continue
		}
		out = append(out, item)
	}
	candidates = out

	if len(candidates) < opts.ReplicaCount {
		return nil, vearchpb.NewError(vearchpb.ErrorEnum_MASTER_PS_NOT_ENOUGH_SELECT,
			fmt.Errorf("only %d candidates after hard filter, need %d", len(candidates), opts.ReplicaCount))
	}

	// Hard filter 2: partition-count balance.
	sig := s.Signature()
	allowedIDs := make([]entity.NodeID, 0, len(candidates))
	for _, item := range candidates {
		allowedIDs = append(allowedIDs, item.NodeID)
	}
	avgForSig := p.snap.AvgPartitionCountForSig(sig, allowedIDs)
	threshold := int(avgForSig) + p.cfg.PartitionDiffThreshold

	preferred := make([]*PSItem, 0, len(candidates))
	rest := make([]*PSItem, 0, len(candidates))
	for _, item := range candidates {
		if item.PartitionCountForSig(sig, p.snap) <= threshold {
			preferred = append(preferred, item)
		} else {
			rest = append(rest, item)
		}
	}

	sortByScore := func(arr []*PSItem) {
		sort.Slice(arr, func(i, j int) bool {
			if arr[i].Priority != arr[j].Priority {
				return arr[i].Priority < arr[j].Priority
			}
			si := ScoreSpaceOnPS(s, arr[i], p.snap, p.cfg)
			sj := ScoreSpaceOnPS(s, arr[j], p.snap, p.cfg)
			return si < sj
		})
	}
	sortByScore(preferred)
	sortByScore(rest)

	/*
		preferred was made with cap; append would alias its underlying array.
		Allocate a fresh slice instead.
	*/
	ordered := make([]*PSItem, 0, len(preferred)+len(rest))
	ordered = append(ordered, preferred...)
	ordered = append(ordered, rest...)
	return p.pickWithAntiAffinity(ordered, opts)
}

func (p *Picker) pickWithAntiAffinity(ordered []*PSItem, opts PickerOptions) ([]entity.NodeID, error) {
	picked := make([]entity.NodeID, 0, opts.ReplicaCount)
	zoneUsed := make(map[string]bool)

	for _, item := range ordered {
		zone := zoneIdentifier(item.Server, opts.AntiAffinityStrategy)
		if zone != "" && zoneUsed[zone] {
			continue
		}
		picked = append(picked, item.NodeID)
		if zone != "" {
			zoneUsed[zone] = true
		}
		if len(picked) >= opts.ReplicaCount {
			break
		}
	}

	/*
		Strict mode: unsatisfied anti-affinity returns an error.
		Non-strict mode: ignore zone constraint and top up by ranking only.
	*/
	if len(picked) < opts.ReplicaCount {
		if p.cfg.StrictAntiAffinity {
			return nil, vearchpb.NewError(vearchpb.ErrorEnum_MASTER_PS_NOT_ENOUGH_SELECT,
				fmt.Errorf("strict anti-affinity not satisfied, picked=%d need=%d", len(picked), opts.ReplicaCount))
		}
		// Soft fallback: top up without honoring zone constraint.
		used := make(map[entity.NodeID]bool, len(picked))
		for _, id := range picked {
			used[id] = true
		}
		for _, item := range ordered {
			if used[item.NodeID] {
				continue
			}
			picked = append(picked, item.NodeID)
			used[item.NodeID] = true
			if len(picked) >= opts.ReplicaCount {
				break
			}
		}
		if len(picked) < opts.ReplicaCount {
			return nil, vearchpb.NewError(vearchpb.ErrorEnum_MASTER_PS_NOT_ENOUGH_SELECT,
				fmt.Errorf("picked=%d need=%d (soft fallback)", len(picked), opts.ReplicaCount))
		}
	}
	return picked, nil
}

func zoneIdentifier(srv *entity.Server, strategy int) string {
	if srv == nil {
		return ""
	}
	switch strategy {
	case 1:
		return srv.HostIp
	case 2:
		return srv.HostRack
	case 3:
		return srv.HostZone
	default:
		return ""
	}
}
