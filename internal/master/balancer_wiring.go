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

package master

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/vearch/vearch/v3/internal/client"
	"github.com/vearch/vearch/v3/internal/config"
	"github.com/vearch/vearch/v3/internal/entity"
	"github.com/vearch/vearch/v3/internal/master/services"
	"github.com/vearch/vearch/v3/internal/master/services/balancer"
	"github.com/vearch/vearch/v3/internal/pkg/log"
)

// memberAdapter adapts MemberService to balancer.MemberOps (breaks import cycle).
type memberAdapter struct {
	memberSvc *services.MemberService
	spaceSvc  *services.SpaceService
}

func (a *memberAdapter) ChangeMember(ctx context.Context, cm *entity.ChangeMember) error {
	return a.memberSvc.ChangeMember(ctx, a.spaceSvc, cm)
}

/*
startBalancer wires the balancer at master startup: construct, register hooks,
register cron jobs, expose HTTP API.
*/
func startBalancer(ctx context.Context, ms *Server, ginEngine *gin.Engine, svc *masterService) (*balancer.Balancer, error) {
	memberOps := &memberAdapter{
		memberSvc: svc.Member(),
		spaceSvc:  svc.Space(),
	}

	bal, err := balancer.New(ctx, svc.Client, memberOps)
	if err != nil {
		return nil, fmt.Errorf("init balancer: %w", err)
	}
	bal.Start()

	// Wire service hooks into the balancer's Picker.
	wirePlacementHook(bal, svc.Client)

	// Register Prometheus metrics on the default registerer.
	bal.RegisterPrometheus(prometheus.DefaultRegisterer)

	// HTTP API must run behind BasicAuth so arbitrary callers cannot trigger migrations.
	var apiGroup gin.IRouter
	if !config.Conf().Global.SkipAuth {
		apiGroup = ginEngine.Group("", BasicAuthMiddleware(svc))
	} else {
		apiGroup = ginEngine.Group("")
	}
	bal.RegisterAPI(apiGroup)

	// Register cron jobs.
	// Use bal.Ctx() (cancelled by bal.Stop()) instead of master ctx so the
	// balancer can be torn down independently if needed.
	registerBalancerJobs(bal.Ctx(), ms, bal, svc.Client)

	log.Info("[balancer] wired into master server")
	return bal, nil
}

/*
wirePlacementHook installs the placement hooks. Hook closures use the
ctx passed by the caller (hookCtx), not the wiring-time ctx.
*/
func wirePlacementHook(bal *balancer.Balancer, cli *client.Client) {
	// 1. Create/expansion hook: pick N PSes for a partition.
	services.SetPartitionPlacementHook(func(hookCtx context.Context, partition *entity.Partition, replicaCount uint8) ([]string, error) {
		cfg := bal.ConfigHolder().Get()
		if !cfg.ScoreBasedPlacement {
			return nil, nil // hook disabled → caller falls back to legacy
		}

		mc := cli.Master()
		space, err := mc.QuerySpaceByID(hookCtx, partition.DBId, partition.SpaceId)
		if err != nil {
			return nil, err
		}

		snap, err := bal.Builder().Build(hookCtx)
		if err != nil {
			return nil, err
		}

		excluded := make(map[entity.NodeID]bool)
		for _, r := range partition.Replicas {
			excluded[r] = true
		}

		opts := balancer.PickerOptions{
			ReplicaCount:         int(replicaCount),
			ResourceName:         space.ResourceName,
			AntiAffinityStrategy: int(mc.Config().PS.ReplicaAntiAffinityStrategy),
			Excluded:             excluded,
		}

		picker := balancer.NewPicker(snap, cfg)
		nodeIDs, err := picker.PickByScore(space, opts)
		if err != nil {
			return nil, err
		}

		addrs := make([]string, 0, len(nodeIDs))
		partition.Replicas = make([]entity.NodeID, 0, len(nodeIDs))
		for _, id := range nodeIDs {
			srv, err := mc.QueryServer(hookCtx, id)
			if err != nil {
				return nil, err
			}
			if !client.IsLive(srv.RpcAddr()) {
				return nil, fmt.Errorf("picked server %d not live", id)
			}
			addrs = append(addrs, srv.RpcAddr())
			partition.Replicas = append(partition.Replicas, id)
		}
		// Record pending placement so subsequent hook calls in the same batch see the occupancy.
		snap.RecordPendingPlacement(space.Signature(), nodeIDs)
		return addrs, nil
	})

	// 2. ChangeReplica hook: pick one new replica node.
	services.SetPartitionMemberAddHook(func(hookCtx context.Context, partition *entity.Partition) (*entity.Server, error) {
		cfg := bal.ConfigHolder().Get()
		if !cfg.ScoreBasedPlacement {
			return nil, nil
		}

		mc := cli.Master()
		space, err := mc.QuerySpaceByID(hookCtx, partition.DBId, partition.SpaceId)
		if err != nil {
			return nil, err
		}
		snap, err := bal.Builder().Build(hookCtx)
		if err != nil {
			return nil, err
		}

		excluded := make(map[entity.NodeID]bool)
		for _, r := range partition.Replicas {
			excluded[r] = true
		}

		picker := balancer.NewPicker(snap, cfg)
		nodeIDs, err := picker.PickByScore(space, balancer.PickerOptions{
			ReplicaCount:         1,
			ResourceName:         space.ResourceName,
			AntiAffinityStrategy: int(mc.Config().PS.ReplicaAntiAffinityStrategy),
			Excluded:             excluded,
		})
		if err != nil || len(nodeIDs) == 0 {
			return nil, err
		}
		/*
			Query node first; only record pending after success, otherwise a
			fallback to legacy would leave a phantom pending entry in the cache.
		*/
		srv, err := mc.QueryServer(hookCtx, nodeIDs[0])
		if err != nil {
			return nil, err
		}
		snap.RecordPendingPlacement(space.Signature(), nodeIDs)
		return srv, nil
	})
}

// registerBalancerJobs starts the Planner / Scheduler / Reaper cron loops.
func registerBalancerJobs(ctx context.Context, ms *Server, bal *balancer.Balancer, cli *client.Client) {
	cfg := bal.ConfigHolder()

	// Planner loop.
	go runBalancerCron(ctx, "planner", func() time.Duration {
		return time.Duration(cfg.Get().PlanIntervalSec) * time.Second
	}, func() {
		bal.ReloadConfigFromEtcd(ctx)
		if err := balancer.AcquireSTMLock(ctx, cli, balancer.LockKeyPlanner, 60); err != nil {
			if !balancer.IsSkip(err) {
				log.Errorf("[balancer:planner] lock acquire failed: %s", err.Error())
			}
			return
		}
		if err := bal.Planner().PlanOnce(ctx); err != nil {
			log.Errorf("[balancer:planner] %s", err.Error())
		}
	})

	/*
		Scheduler loop.

		STM lock ensures only one master at a time runs AdvanceAll (writes),
		otherwise concurrent ChangeMember calls would corrupt the task state
		machine.

		SyncFromEtcd runs OUTSIDE the lock so every master (lock-holder or not)
		keeps its local inflight map and tracker in sync with etcd-persisted
		state on every tick. This is required for read-side handlers (/snapshot,
		/tasks, /config) to return consistent state regardless of which master
		the request lands on, and for Submit dedup to catch cross-master
		duplicates. SyncFromEtcd is read-only on etcd; locking is unnecessary.
	*/
	go runBalancerCron(ctx, "scheduler", func() time.Duration {
		return 30 * time.Second
	}, func() {
		bal.ReloadConfigFromEtcd(ctx)
		if err := bal.Scheduler().SyncFromEtcd(ctx); err != nil {
			log.Warnf("[balancer:scheduler] SyncFromEtcd failed: %s", err.Error())
		}
		if err := balancer.AcquireSTMLock(ctx, cli, balancer.LockKeyScheduler, 60); err != nil {
			if !balancer.IsSkip(err) {
				log.Errorf("[balancer:scheduler] lock acquire failed: %s", err.Error())
			}
			return
		}
		bal.Scheduler().AdvanceAll(ctx)
	})

	// Reaper
	go runBalancerCron(ctx, "reaper", func() time.Duration {
		return time.Duration(cfg.Get().RedundantReaperIntervalSec) * time.Second
	}, func() {
		bal.ReloadConfigFromEtcd(ctx)
		if err := balancer.AcquireSTMLock(ctx, cli, balancer.LockKeyReaper, 60); err != nil {
			if !balancer.IsSkip(err) {
				log.Errorf("[balancer:reaper] lock acquire failed: %s", err.Error())
			}
			return
		}
		if err := bal.Reaper().RunOnce(ctx); err != nil {
			log.Errorf("[balancer:reaper] %s", err.Error())
		}
	})
}

func runBalancerCron(ctx context.Context, name string, intervalFn func() time.Duration, fn func()) {
	for {
		interval := intervalFn()
		if interval <= 0 {
			interval = time.Minute
		}
		// Explicit timer + select: exit immediately on ctx cancel; Stop avoids timer leaks.
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			log.Infof("[balancer:%s] cron stopped", name)
			return
		case <-timer.C:
		}
		// Recheck ctx in case fn was slow and parent ctx is now cancelled.
		if ctx.Err() != nil {
			return
		}
		/*
			Recover per tick so a single panic in fn (e.g. nil deref in a planner
			pass) does not kill the entire cron goroutine; without this, the cron
			would silently stop until master restart.
		*/
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("[balancer:%s] panic recovered, continuing: %v", name, r)
				}
			}()
			fn()
		}()
	}
}
