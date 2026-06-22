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
	"math"

	"github.com/vearch/vearch/v3/internal/entity"
)

// PSItem is the scoring item for a PS node (mirrors Milvus NodeItem).
type PSItem struct {
	NodeID        entity.NodeID
	Server        *entity.Server
	Stat          *entity.PSStat
	CurrentScore  float64
	AssignedScore float64
	Priority      int
	IsLive        bool
	IsInWarmup    bool
}

// RecomputePriority is called after CurrentScore / AssignedScore changes.
func (p *PSItem) RecomputePriority() {
	p.Priority = int(math.Ceil(p.CurrentScore - p.AssignedScore))
}

func (p *PSItem) AddCurrentScoreDelta(delta float64) {
	p.CurrentScore += delta
	p.RecomputePriority()
}

func (p *PSItem) AddAssignedScore(delta float64) {
	p.AssignedScore += delta
	p.RecomputePriority()
}

// PartitionCount returns the total partition count on this PS.
func (p *PSItem) PartitionCount() int {
	if p.Stat == nil {
		return 0
	}
	return p.Stat.PartitionCount
}

/*
PartitionCountForSig returns partition count on this PS for a signature,
including pending placements recorded on the snapshot so the same node is
not picked repeatedly for sibling partitions within one CreateSpace batch.
*/
func (p *PSItem) PartitionCountForSig(sig entity.SpaceSignature, snap *Snapshot) int {
	count := 0
	if p.Stat != nil && p.Stat.SpaceStats != nil {
		for spaceID, load := range p.Stat.SpaceStats {
			if snap.SpaceSignatureOf(spaceID) == sig {
				count += load.PartitionCount
			}
		}
	}
	count += snap.PendingForSigOnNode(sig, p.NodeID)
	return count
}
