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

package services

import (
	"context"
	"sync/atomic"

	"github.com/vearch/vearch/v3/internal/entity"
)

/*
PartitionPlacementHook picks PSes for a partition during space creation /
expansion. Returns RpcAddrs; partition.Replicas is filled by the hook.
nil hook means fall back to the legacy selector.
*/
type PartitionPlacementHook func(ctx context.Context, partition *entity.Partition, replicaCount uint8) ([]string, error)

/*
PartitionMemberAddHook picks one new replica node during ChangeReplica.
Returns nil to fall back to the legacy selector.
*/
type PartitionMemberAddHook func(ctx context.Context, partition *entity.Partition) (*entity.Server, error)

var (
	placementHook atomic.Pointer[PartitionPlacementHook]
	memberAddHook atomic.Pointer[PartitionMemberAddHook]
)

// SetPartitionPlacementHook installs the placement hook at master startup.
func SetPartitionPlacementHook(h PartitionPlacementHook) {
	if h == nil {
		placementHook.Store(nil)
		return
	}
	placementHook.Store(&h)
}

// SetPartitionMemberAddHook installs the ChangeReplica hook at master startup.
func SetPartitionMemberAddHook(h PartitionMemberAddHook) {
	if h == nil {
		memberAddHook.Store(nil)
		return
	}
	memberAddHook.Store(&h)
}

func getPlacementHook() PartitionPlacementHook {
	if h := placementHook.Load(); h != nil {
		return *h
	}
	return nil
}

func getMemberAddHook() PartitionMemberAddHook {
	if h := memberAddHook.Load(); h != nil {
		return *h
	}
	return nil
}
