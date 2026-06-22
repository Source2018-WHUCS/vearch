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

	"github.com/cubefs/cubefs/depends/tiglabs/raft/proto"
	"github.com/vearch/vearch/v3/internal/entity"
	"github.com/vearch/vearch/v3/internal/pkg/log"
)

// Reaper backstop-cleans excess replicas.
type Reaper struct {
	cfgHolder *configHolder
	memberOps MemberOps
	builder   *SnapshotBuilder
	tracker   *TaskTracker // used to skip partitions with inflight migrations
}

func NewReaper(cfg *configHolder, memberOps MemberOps, builder *SnapshotBuilder, tracker *TaskTracker) *Reaper {
	return &Reaper{cfgHolder: cfg, memberOps: memberOps, builder: builder, tracker: tracker}
}

// RunOnce runs one cleanup pass.
func (r *Reaper) RunOnce(ctx context.Context) error {
	cfg := r.cfgHolder.Get()
	if !cfg.RedundantReaperEnabled {
		return nil
	}

	snap, err := r.builder.Build(ctx)
	if err != nil {
		return err
	}

	for _, partition := range snap.allPartitions {
		space, ok := snap.allSpaces[partition.SpaceId]
		if !ok {
			continue
		}
		expected := int(space.ReplicaNum)
		if len(partition.Replicas) <= expected {
			continue
		}
		/*
			During Scheduler's Add → WaitingCaughtUp → Remove window, partitions
			naturally sit at ReplicaNum+1. Reaper must not act here or it could
			undo Scheduler's newly-added replica (violating §5.3).
		*/
		if r.tracker != nil && r.tracker.HasInflightForPartition(partition.Id) {
			continue
		}

		victim := r.pickWorstReplica(partition, snap)
		if victim == 0 {
			continue
		}
		log.Warnf("[balancer:reaper] partition %d has %d replicas, expected %d, removing %d",
			partition.Id, len(partition.Replicas), expected, victim)

		cm := &entity.ChangeMember{
			PartitionID: partition.Id,
			NodeID:      victim,
			Method:      proto.ConfRemoveNode,
		}
		if err := r.memberOps.ChangeMember(ctx, cm); err != nil {
			log.Errorf("[balancer:reaper] remove %d from partition %d failed: %s",
				victim, partition.Id, err.Error())
		}
	}
	return nil
}

// pickWorstReplica picks the heaviest / dead replica to evict.
func (r *Reaper) pickWorstReplica(p *entity.Partition, snap *Snapshot) entity.NodeID {
	type cand struct {
		id    entity.NodeID
		score float64
	}
	cands := make([]cand, 0, len(p.Replicas))
	for _, id := range p.Replicas {
		if id == p.LeaderID {
			continue // never evict the leader
		}
		item := snap.PSItem(id)
		if item == nil || !item.IsLive {
			cands = append(cands, cand{id, math.MaxFloat64})
			continue
		}
		cands = append(cands, cand{id, item.CurrentScore})
	}
	if len(cands) == 0 {
		return 0
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	return cands[0].id
}
