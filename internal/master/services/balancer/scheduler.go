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
	"fmt"
	"sync"
	"time"

	"github.com/cubefs/cubefs/depends/tiglabs/raft/proto"
	"github.com/google/uuid"
	"github.com/vearch/vearch/v3/internal/client"
	"github.com/vearch/vearch/v3/internal/entity"
	"github.com/vearch/vearch/v3/internal/pkg/log"
	json "github.com/vearch/vearch/v3/internal/pkg/vjson"
)

/*
MemberOps is the minimal interface for invoking MemberService from balancer.
Injected at master startup via an adapter to avoid an import cycle.
*/
type MemberOps interface {
	ChangeMember(ctx context.Context, cm *entity.ChangeMember) error
}

// TaskScheduler drives the MigrateTask state machine.
type TaskScheduler struct {
	cli         *client.Client
	cfgHolder   *configHolder
	memberOps   MemberOps
	taskTracker *TaskTracker

	mu       sync.RWMutex
	inflight map[string]*MigrateTask

	confChangePool  chan struct{}
	dataReplicaPool chan struct{}
}

func NewTaskScheduler(cli *client.Client, cfg *configHolder, memberOps MemberOps, tracker *TaskTracker) *TaskScheduler {
	c := cfg.Get()
	return &TaskScheduler{
		cli:             cli,
		cfgHolder:       cfg,
		memberOps:       memberOps,
		taskTracker:     tracker,
		inflight:        make(map[string]*MigrateTask),
		confChangePool:  make(chan struct{}, c.ConfChangePoolSize),
		dataReplicaPool: make(chan struct{}, c.DataReplicaPoolSize),
	}
}

// LoadFromEtcd rehydrates inflight tasks at startup (crash recovery).
func (ts *TaskScheduler) LoadFromEtcd(ctx context.Context) error {
	_, values, err := ts.cli.Master().Store.PrefixScan(ctx, entity.PrefixMigrateTask)
	if err != nil {
		return err
	}
	for _, v := range values {
		t := &MigrateTask{}
		if err := json.Unmarshal(v, t); err != nil {
			log.Warnf("[balancer] unmarshal MigrateTask failed: %s", err.Error())
			continue
		}
		if t.Step.IsTerminal() {
			continue
		}
		// Symmetric with Submit: inflight write + tracker.Add in one critical section.
		ts.mu.Lock()
		ts.inflight[t.ID] = t
		ts.taskTracker.Add(t)
		ts.mu.Unlock()
		log.Infof("[balancer] recovered task %s at step %s", t.ID, t.Step)
	}
	return nil
}

/*
SyncFromEtcd reconciles the local in-memory inflight map with the etcd-persisted
truth, so the master holding the STM scheduler lock (and any read-side handler)
can see the full set of active tasks regardless of which master accepted Submit.

Multi-master HA hazard this addresses: every master's TaskScheduler keeps an
in-memory `inflight` map populated by local Submit + startup LoadFromEtcd. The
scheduler STM lock rotates between masters; if the lock-holder is NOT the one
that received Submit, its AdvanceAll iterates an empty (for that task)
inflight and the task stays at its current step indefinitely. Calling
SyncFromEtcd before AdvanceAll on every tick guarantees the lock-holder sees
every active task regardless of which master originally accepted it.

Three-way reconcile:
  - ADD: tasks in etcd but not in local inflight → load into local + tracker.
  - REMOVE: tasks in local inflight but no longer in etcd (lock-holder completed
    or failed them, deleting the etcd key) → evict from local + tracker.
  - ADVANCE: tasks present in both, with etcd Step strictly newer than local Step
    → overwrite local with etcd state. Required because the previous lock-holder
    transitions a task (e.g. Pending → AddingMember) and persists to etcd, but
    other masters' local copies stay at the old step. When the STM lock rotates,
    the new lock-holder would otherwise rerun the prior step's side effects
    (e.g. preAdd reaches `target already has replica` and the task is failed
    spuriously). The guard `etcd.Step > local.Step` prevents regression in the
    brief window where the lock-holder has updated in-memory but not yet
    persisted (transitionTo writes etcd after the in-memory update).

Terminal tasks in etcd are skipped (complete()/fail() delete them, but we
tolerate brief overlap windows).
*/
func (ts *TaskScheduler) SyncFromEtcd(ctx context.Context) error {
	_, values, err := ts.cli.Master().Store.PrefixScan(ctx, entity.PrefixMigrateTask)
	if err != nil {
		return err
	}

	etcdTasks := make(map[string]*MigrateTask, len(values))
	for _, v := range values {
		t := &MigrateTask{}
		if err := json.Unmarshal(v, t); err != nil {
			log.Warnf("[balancer] sync: unmarshal MigrateTask failed: %s", err.Error())
			continue
		}
		if t.Step.IsTerminal() {
			continue
		}
		etcdTasks[t.ID] = t
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	picked := 0
	advanced := 0
	for id, t := range etcdTasks {
		if local, exists := ts.inflight[id]; exists {
			if int(t.Step) > int(local.Step) {
				etcdTask := t
				oldStep := local.Step
				ts.taskTracker.UpdateTask(local, func(task *MigrateTask) {
					*task = *etcdTask
				})
				advanced++
				log.Infof("[balancer] sync: advanced task %s from step %s to %s (etcd ahead of local)",
					id, oldStep, etcdTask.Step)
			}
			continue
		}
		ts.inflight[id] = t
		ts.taskTracker.Add(t)
		picked++
		log.Infof("[balancer] sync: picked up orphan task %s at step %s", t.ID, t.Step)
	}

	removed := 0
	for id, t := range ts.inflight {
		if _, in := etcdTasks[id]; in {
			continue
		}
		delete(ts.inflight, id)
		ts.taskTracker.Remove(t)
		removed++
		log.Infof("[balancer] sync: removed stale task %s (gone from etcd)", id)
	}

	if picked > 0 || removed > 0 || advanced > 0 {
		log.Infof("[balancer] sync: %d picked up, %d removed, %d advanced", picked, removed, advanced)
	}
	return nil
}

// Submit creates and persists a new task.
func (ts *TaskScheduler) Submit(ctx context.Context, op MigrateOp) (*MigrateTask, error) {
	/*
		Cross-master dedup: a task for the same partition_id may have been
		submitted to a different master and only persisted in etcd. Refresh our
		local inflight from etcd before the dedup check below so we observe such
		tasks. Non-fatal: if the sync fails we proceed with a possibly stale
		view rather than rejecting a legitimate Submit.
	*/
	if err := ts.SyncFromEtcd(ctx); err != nil {
		log.Warnf("[balancer] submit: pre-sync failed (continuing with stale view): %s", err.Error())
	}

	now := time.Now().UnixMilli()
	t := &MigrateTask{
		ID:             "mt-" + uuid.NewString()[:12],
		PartitionID:    op.PartitionID,
		SpaceID:        op.SpaceID,
		FromNodeID:     op.FromNodeID,
		ToNodeID:       op.ToNodeID,
		EstimatedBytes: op.EstimatedBytes,
		Reason:         op.Reason,
		Step:           StepPending,
		StartTime:      now,
		StepEnterTime:  now,
	}

	/*
		Submit holds ts.mu through persist: dedup, reserve, persist, and Add are
		atomic. AdvanceAll only takes RLock briefly (copying the task list).
		This prevents advance() from picking up a task whose persist has not yet
		completed (otherwise a failed persist would leave a phantom task in etcd
		while Submit returned an error to the caller).
	*/
	ts.mu.Lock()
	defer ts.mu.Unlock()

	for _, existing := range ts.inflight {
		if existing.PartitionID == op.PartitionID {
			return nil, fmt.Errorf("partition %d already has inflight task %s at step %s",
				op.PartitionID, existing.ID, existing.Step)
		}
	}
	ts.inflight[t.ID] = t

	if err := ts.persist(ctx, t); err != nil {
		delete(ts.inflight, t.ID)
		return nil, fmt.Errorf("persist task: %w", err)
	}

	ts.taskTracker.Add(t)
	log.Infof("[balancer] submitted task %s: partition=%d %d→%d reason=%s",
		t.ID, t.PartitionID, t.FromNodeID, t.ToNodeID, t.Reason)
	return t, nil
}

// AdvanceAll ticks every inflight task one step.
func (ts *TaskScheduler) AdvanceAll(ctx context.Context) {
	ts.mu.RLock()
	tasks := make([]*MigrateTask, 0, len(ts.inflight))
	for _, t := range ts.inflight {
		tasks = append(tasks, t)
	}
	ts.mu.RUnlock()

	for _, t := range tasks {
		ts.advance(ctx, t)
	}
}

func (ts *TaskScheduler) advance(ctx context.Context, t *MigrateTask) {
	/*
		All t.* writes go through tracker.UpdateTask to interlock with
		AllInflight readers. Reads of t.* in this function are safe — advance()
		is single-threaded per task (AdvanceAll iterates serially).
	*/
	ts.taskTracker.UpdateTask(t, func(task *MigrateTask) {
		task.LastTickTime = time.Now().UnixMilli()
	})
	cfg := ts.cfgHolder.Get()

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[balancer] advance task %s panic: %v", t.ID, r)
		}
	}()

	switch t.Step {
	case StepPending:
		if !ts.preAdd(ctx, t) {
			return
		}
		/*
			defer must register only when the slot is acquired; otherwise the
			default branch would release a slot that was never taken.
		*/
		select {
		case ts.confChangePool <- struct{}{}:
			defer func() { <-ts.confChangePool }()
		default:
			return
		}

		partition, err := ts.cli.Master().QueryPartition(ctx, t.PartitionID)
		if err != nil {
			ts.fail(ctx, t, fmt.Errorf("query partition: %w", err))
			return
		}
		// LockedLeaderID write under lock.
		leaderID := partition.LeaderID
		ts.taskTracker.UpdateTask(t, func(task *MigrateTask) {
			task.LockedLeaderID = leaderID
		})

		cm := &entity.ChangeMember{
			PartitionID: t.PartitionID,
			NodeID:      t.ToNodeID,
			Method:      proto.ConfAddNode,
		}
		if err := ts.memberOps.ChangeMember(ctx, cm); err != nil {
			ts.fail(ctx, t, fmt.Errorf("AddNode: %w", err))
			return
		}
		ts.transitionTo(ctx, t, StepAddingMember)

	case StepAddingMember:
		partition, err := ts.cli.Master().QueryPartition(ctx, t.PartitionID)
		if err != nil {
			// Retried next tick; logged so etcd flakiness is observable.
			// Still check timeout — flaky etcd must not strand the task forever.
			log.Warnf("[balancer] task %s QueryPartition failed at AddingMember: %s", t.ID, err.Error())
			if ts.timedOut(t, cfg.AddMemberTimeoutSec) {
				ts.fail(ctx, t, fmt.Errorf("add member timeout (query partition failing): %w", err))
			}
			return
		}
		if containsReplica(partition.Replicas, t.ToNodeID) {
			ts.transitionTo(ctx, t, StepWaitingCaughtUp)
			return
		}
		/*
			/partition/<id> says ToNodeID is not yet a replica. It may simply
			be stale — MemberService.ChangeMember writes /space/<dbid>/<spaceid>
			synchronously, but /partition/<id> is refreshed asynchronously by
			the PS leader's registerMaster chain and can lag by seconds. Consult
			/space directly as the authoritative source before deciding to wait.
		*/
		if present, ok := ts.replicaPresentInSpace(ctx, partition.DBId, t.SpaceID, t.PartitionID, t.ToNodeID); ok && present {
			log.Infof("[balancer] task %s: /partition lags /space; treating AddNode as done (to=%d)",
				t.ID, t.ToNodeID)
			ts.transitionTo(ctx, t, StepWaitingCaughtUp)
			return
		}
		if ts.timedOut(t, cfg.AddMemberTimeoutSec) {
			ts.fail(ctx, t, fmt.Errorf("add member timeout"))
		}

	case StepWaitingCaughtUp:
		// defer must register only when the slot is acquired.
		select {
		case ts.dataReplicaPool <- struct{}{}:
			defer func() { <-ts.dataReplicaPool }()
		default:
			return
		}

		caughtUp, err := ts.isReplicaCaughtUp(ctx, t)
		if err != nil {
			/*
				Persistent errors here (e.g. partition_not_exist when the space
				was dropped, or PS RPC failures) must still honor the timeout,
				otherwise the task stays in WaitingCaughtUp forever. Mirrors
				the AddingMember / RemovingMember error paths.
			*/
			log.Warnf("[balancer] task %s check caughtUp failed: %s", t.ID, err.Error())
			if ts.timedOut(t, cfg.CaughtUpTimeoutSec) {
				ts.tryRollbackNewReplica(ctx, t)
				ts.fail(ctx, t, fmt.Errorf("caughtUp timeout (check failing): %w", err))
			}
			return
		}
		if caughtUp {
			partition, err := ts.cli.Master().QueryPartition(ctx, t.PartitionID)
			if err != nil {
				/*
					Same reasoning: a flaky QueryPartition here must not strand
					the task indefinitely.
				*/
				log.Warnf("[balancer] task %s QueryPartition failed at WaitingCaughtUp(caughtUp): %s", t.ID, err.Error())
				if ts.timedOut(t, cfg.CaughtUpTimeoutSec) {
					ts.tryRollbackNewReplica(ctx, t)
					ts.fail(ctx, t, fmt.Errorf("caughtUp timeout (query partition failing): %w", err))
				}
				return
			}
			if partition.LeaderID != t.LockedLeaderID {
				ts.fail(ctx, t, fmt.Errorf("leader changed during migration: %d→%d",
					t.LockedLeaderID, partition.LeaderID))
				return
			}

			// defer must register only when the slot is acquired.
			select {
			case ts.confChangePool <- struct{}{}:
				defer func() { <-ts.confChangePool }()
			default:
				return
			}

			cm := &entity.ChangeMember{
				PartitionID: t.PartitionID,
				NodeID:      t.FromNodeID,
				Method:      proto.ConfRemoveNode,
			}
			if err := ts.memberOps.ChangeMember(ctx, cm); err != nil {
				ts.fail(ctx, t, fmt.Errorf("RemoveNode: %w", err))
				return
			}
			ts.transitionTo(ctx, t, StepRemovingMember)
			return
		}
		if ts.timedOut(t, cfg.CaughtUpTimeoutSec) {
			ts.tryRollbackNewReplica(ctx, t)
			ts.fail(ctx, t, fmt.Errorf("caughtUp timeout"))
		}

	case StepRemovingMember:
		partition, err := ts.cli.Master().QueryPartition(ctx, t.PartitionID)
		if err != nil {
			/*
				Retried next tick; still check timeout so a flaky etcd cannot
				strand the task indefinitely.
			*/
			log.Warnf("[balancer] task %s QueryPartition failed at RemovingMember: %s", t.ID, err.Error())
			if ts.timedOut(t, cfg.RemoveMemberTimeoutSec) {
				ts.fail(ctx, t, fmt.Errorf("remove member timeout (query partition failing): %w", err))
			}
			return
		}
		if !containsReplica(partition.Replicas, t.FromNodeID) {
			ts.transitionTo(ctx, t, StepDone)
			ts.complete(ctx, t)
			return
		}
		/*
			/partition/<id> still shows FromNodeID. It may be a stale read —
			MemberService.ChangeMember updates /space/<dbid>/<spaceid> directly
			while /partition/<id> is refreshed asynchronously by the PS leader.
			Consult /space directly: if the remove is already reflected there
			the migration is effectively done; we must not hang waiting for
			the async chain to catch up.
		*/
		if present, ok := ts.replicaPresentInSpace(ctx, partition.DBId, t.SpaceID, t.PartitionID, t.FromNodeID); ok && !present {
			log.Infof("[balancer] task %s: /partition lags /space; treating RemoveNode as done (from=%d)",
				t.ID, t.FromNodeID)
			ts.transitionTo(ctx, t, StepDone)
			ts.complete(ctx, t)
			return
		}
		if ts.timedOut(t, cfg.RemoveMemberTimeoutSec) {
			ts.fail(ctx, t, fmt.Errorf("remove member timeout"))
		}
	}
}

func (ts *TaskScheduler) preAdd(ctx context.Context, t *MigrateTask) bool {
	partition, err := ts.cli.Master().QueryPartition(ctx, t.PartitionID)
	if err != nil {
		ts.fail(ctx, t, fmt.Errorf("query partition: %w", err))
		return false
	}
	if !containsReplica(partition.Replicas, t.FromNodeID) {
		ts.fail(ctx, t, fmt.Errorf("source replica %d not in partition", t.FromNodeID))
		return false
	}
	if containsReplica(partition.Replicas, t.ToNodeID) {
		ts.fail(ctx, t, fmt.Errorf("target node %d already has replica", t.ToNodeID))
		return false
	}
	return true
}

func (ts *TaskScheduler) timedOut(t *MigrateTask, timeoutSec int) bool {
	return time.Now().UnixMilli()-t.StepEnterTime > int64(timeoutSec)*1000
}

func (ts *TaskScheduler) transitionTo(ctx context.Context, t *MigrateTask, next MigrateStep) {
	log.Infof("[balancer] task %s: %s → %s", t.ID, t.Step, next)
	// Field writes under lock; persist (etcd IO) runs outside the lock.
	ts.taskTracker.UpdateTask(t, func(task *MigrateTask) {
		task.Step = next
		task.StepEnterTime = time.Now().UnixMilli()
	})
	if err := ts.persist(ctx, t); err != nil {
		log.Errorf("[balancer] persist task %s failed: %s", t.ID, err.Error())
	}
}

func (ts *TaskScheduler) fail(ctx context.Context, t *MigrateTask, err error) {
	durSec := float64(time.Now().UnixMilli()-t.StartTime) / 1000
	recordMigrationCompleted(t.Reason, false, durSec)
	log.Errorf("[balancer] task %s FAILED at %s: %s", t.ID, t.Step, err.Error())
	// Field writes under lock; err.Error() evaluated outside closure.
	errStr := err.Error()
	ts.taskTracker.UpdateTask(t, func(task *MigrateTask) {
		task.Step = StepFailed
		task.LastError = errStr
		task.StepEnterTime = time.Now().UnixMilli()
	})
	// Delete terminal task from etcd so /migrate_task prefix cannot grow without bound.
	if derr := ts.cli.Master().Store.Delete(ctx, entity.MigrateTaskKey(t.ID)); derr != nil {
		log.Errorf("[balancer] delete failed task from etcd: %s", derr.Error())
	}
	ts.removeInflight(t)
}

func (ts *TaskScheduler) complete(ctx context.Context, t *MigrateTask) {
	durSec := float64(time.Now().UnixMilli()-t.StartTime) / 1000
	recordMigrationCompleted(t.Reason, true, durSec)
	log.Infof("[balancer] task %s DONE", t.ID)
	cfg := ts.cfgHolder.Get()
	cooldownAt := time.Now().UnixMilli() + int64(cfg.CooldownPerPartitionSec)*1000
	ts.taskTracker.SetCooldown(t.PartitionID, cooldownAt)
	// Delete terminal task from etcd.
	if derr := ts.cli.Master().Store.Delete(ctx, entity.MigrateTaskKey(t.ID)); derr != nil {
		log.Errorf("[balancer] delete done task from etcd: %s", derr.Error())
	}
	ts.removeInflight(t)
}

func (ts *TaskScheduler) removeInflight(t *MigrateTask) {
	/*
		Symmetric with Submit: inflight delete + tracker.Remove in one critical
		section, so GetTaskDelta cannot observe an inconsistent intermediate state.
	*/
	ts.mu.Lock()
	defer ts.mu.Unlock()
	delete(ts.inflight, t.ID)
	ts.taskTracker.Remove(t)
}

func (ts *TaskScheduler) persist(ctx context.Context, t *MigrateTask) error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return ts.cli.Master().Store.Put(ctx, entity.MigrateTaskKey(t.ID), data)
}

// isReplicaCaughtUp checks whether the target replica has caught up to the leader.
func (ts *TaskScheduler) isReplicaCaughtUp(ctx context.Context, t *MigrateTask) (bool, error) {
	server, err := ts.cli.Master().QueryServer(ctx, t.LockedLeaderID)
	if err != nil {
		return false, err
	}
	info, err := client.PartitionInfo(server.RpcAddr(), t.PartitionID, true)
	if err != nil {
		return false, err
	}
	if info == nil || info.RaftStatus == nil {
		return false, nil
	}
	leaderReplica, ok := info.RaftStatus.Replicas[info.RaftStatus.Leader]
	if !ok {
		return false, nil
	}
	target, ok := info.RaftStatus.Replicas[t.ToNodeID]
	if !ok || leaderReplica.Match == 0 {
		return false, nil
	}
	// 99% of leader Match counts as caught up.
	return target.Match >= leaderReplica.Match*99/100, nil
}

func (ts *TaskScheduler) tryRollbackNewReplica(ctx context.Context, t *MigrateTask) {
	cm := &entity.ChangeMember{
		PartitionID: t.PartitionID,
		NodeID:      t.ToNodeID,
		Method:      proto.ConfRemoveNode,
	}
	if err := ts.memberOps.ChangeMember(ctx, cm); err != nil {
		log.Warnf("[balancer] rollback new replica failed: %s", err.Error())
	}
}

func containsReplica(replicas []entity.NodeID, id entity.NodeID) bool {
	for _, r := range replicas {
		if r == id {
			return true
		}
	}
	return false
}

/*
replicaPresentInSpace consults /space/<dbid>/<spaceid> as an authoritative
fallback for stale /partition/<id> reads.

Background: MemberService.ChangeMember writes the new replica list to
/space/<dbid>/<spaceid> synchronously after the raft conf-change RPC
succeeds (see member_service.go's UpdateSpace call). The /partition/<id>
key is updated asynchronously by the PS leader's registerMaster chain and
can lag behind by seconds — or, if the PS leader's HandleRaftReplicaEvent
path is skipped (single-replica edge cases, leadership transitions during
the conf change), can lag indefinitely.

Returns (present, ok) where:
  - present: whether nodeID appears in the partition's replica list per /space
  - ok:      true if the lookup succeeded; false on query error or if the
    partition is not found inside the space (caller should keep
    polling /partition in that case rather than trust this view)

Callers should treat (ok=false) as "no information" and keep relying on
/partition + timeout. (ok=true, present=...) gives an authoritative answer.
*/
func (ts *TaskScheduler) replicaPresentInSpace(
	ctx context.Context,
	dbID entity.DBID, spaceID entity.SpaceID, partitionID entity.PartitionID,
	nodeID entity.NodeID,
) (present bool, ok bool) {
	if dbID == 0 || spaceID == 0 {
		return false, false
	}
	space, err := ts.cli.Master().QuerySpaceByID(ctx, dbID, spaceID)
	if err != nil {
		log.Warnf("[balancer] QuerySpaceByID(db=%d,space=%d) failed: %s", dbID, spaceID, err.Error())
		return false, false
	}
	for _, sp := range space.Partitions {
		if sp.Id == partitionID {
			return containsReplica(sp.Replicas, nodeID), true
		}
	}
	return false, false
}
