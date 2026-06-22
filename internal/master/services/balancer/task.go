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
	"github.com/vearch/vearch/v3/internal/entity"
)

// MigrateStep is the state-machine stage of a MigrateTask.
type MigrateStep int

const (
	StepPending MigrateStep = iota
	StepAddingMember
	StepWaitingCaughtUp
	StepRemovingMember
	StepDone
	StepFailed
)

func (s MigrateStep) String() string {
	switch s {
	case StepPending:
		return "Pending"
	case StepAddingMember:
		return "AddingMember"
	case StepWaitingCaughtUp:
		return "WaitingCaughtUp"
	case StepRemovingMember:
		return "RemovingMember"
	case StepDone:
		return "Done"
	case StepFailed:
		return "Failed"
	}
	return "Unknown"
}

/*
MigrateTask is persisted to etcd.

Concurrency: scheduler.advance() is the sole writer (AdvanceAll iterates
serially in one goroutine). All field writes must go through
TaskTracker.UpdateTask so that AllInflight readers (under tracker.mu.RLock)
see a consistent snapshot.
*/
type MigrateTask struct {
	ID             string             `json:"id"`
	PartitionID    entity.PartitionID `json:"partition_id"`
	SpaceID        entity.SpaceID     `json:"space_id"`
	FromNodeID     entity.NodeID      `json:"from_node_id"`
	ToNodeID       entity.NodeID      `json:"to_node_id"`
	LockedLeaderID entity.NodeID      `json:"locked_leader_id"`

	EstimatedBytes uint64 `json:"estimated_bytes"`
	Reason         string `json:"reason"`

	Step          MigrateStep `json:"step"`
	StartTime     int64       `json:"start_time"`
	LastTickTime  int64       `json:"last_tick_time"`
	StepEnterTime int64       `json:"step_enter_time"`
	AttemptCount  int         `json:"attempt_count"`
	LastError     string      `json:"last_error,omitempty"`
}

// IsTerminal reports whether the step is StepDone or StepFailed.
func (s MigrateStep) IsTerminal() bool {
	return s == StepDone || s == StepFailed
}

// MigrateOp describes a migration before it is submitted to the Scheduler.
type MigrateOp struct {
	PartitionID    entity.PartitionID
	SpaceID        entity.SpaceID
	FromNodeID     entity.NodeID
	ToNodeID       entity.NodeID
	EstimatedBytes uint64
	Reason         string
}
