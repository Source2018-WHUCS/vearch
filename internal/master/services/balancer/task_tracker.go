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
	"sync"

	"github.com/vearch/vearch/v3/internal/entity"
)

/*
TaskTracker tracks inflight migration tasks and feeds "reservation"
adjustments back into scoring.
*/
type TaskTracker struct {
	mu       sync.RWMutex
	inflight map[string]*MigrateTask
	cooldown map[entity.PartitionID]int64 // partition_id → cooldown release unixMilli
}

func NewTaskTracker() *TaskTracker {
	return &TaskTracker{
		inflight: make(map[string]*MigrateTask),
		cooldown: make(map[entity.PartitionID]int64),
	}
}

func (t *TaskTracker) Add(task *MigrateTask) {
	t.mu.Lock()
	t.inflight[task.ID] = task
	t.mu.Unlock()
}

/*
UpdateTask applies fn to task under tracker.mu.Lock, serializing with
AllInflight readers. All MigrateTask field writes from scheduler.advance()
must go through this helper.
*/
func (t *TaskTracker) UpdateTask(task *MigrateTask, fn func(*MigrateTask)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fn(task)
}

func (t *TaskTracker) Remove(task *MigrateTask) {
	t.mu.Lock()
	delete(t.inflight, task.ID)
	t.mu.Unlock()
}

// SetCooldown puts a partition into cooldown after a completed migration.
func (t *TaskTracker) SetCooldown(pid entity.PartitionID, releaseAt int64) {
	t.mu.Lock()
	t.cooldown[pid] = releaseAt
	t.mu.Unlock()
}

// InCooldown reports whether a partition is still in cooldown.
func (t *TaskTracker) InCooldown(pid entity.PartitionID, nowMs int64) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	releaseAt, ok := t.cooldown[pid]
	if !ok {
		return false
	}
	return releaseAt > nowMs
}

/*
GCExpiredCooldowns drops expired cooldown entries so the map cannot grow
without bound. Called from Planner each round.
*/
func (t *TaskTracker) GCExpiredCooldowns(nowMs int64) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	removed := 0
	for pid, releaseAt := range t.cooldown {
		if releaseAt <= nowMs {
			delete(t.cooldown, pid)
			removed++
		}
	}
	return removed
}

// GetTaskDelta returns the byte delta to apply to a PS due to inflight tasks.
func (t *TaskTracker) GetTaskDelta(nodeID entity.NodeID) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var delta float64
	for _, op := range t.inflight {
		if op.ToNodeID == nodeID {
			delta += float64(op.EstimatedBytes)
		}
		if op.FromNodeID == nodeID {
			delta -= float64(op.EstimatedBytes)
		}
	}
	return delta
}

// CountInflightForSpace returns the number of inflight tasks for a space.
func (t *TaskTracker) CountInflightForSpace(spaceID entity.SpaceID) int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n := 0
	for _, op := range t.inflight {
		if op.SpaceID == spaceID {
			n++
		}
	}
	return n
}

// CountInflight returns the total number of inflight tasks.
func (t *TaskTracker) CountInflight() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.inflight)
}

/*
HasInflightForPartition reports whether a partition has any inflight task.
Used by Reaper to skip partitions in the AddNode → RemoveNode transition.
*/
func (t *TaskTracker) HasInflightForPartition(pid entity.PartitionID) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, op := range t.inflight {
		if op.PartitionID == pid {
			return true
		}
	}
	return false
}

/*
AllInflight returns a snapshot of all inflight tasks.
Returns value copies (not shared pointers) so JSON marshallers cannot
race with scheduler.advance() writes.
*/
func (t *TaskTracker) AllInflight() []*MigrateTask {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]*MigrateTask, 0, len(t.inflight))
	for _, op := range t.inflight {
		cp := *op
		out = append(out, &cp)
	}
	return out
}
