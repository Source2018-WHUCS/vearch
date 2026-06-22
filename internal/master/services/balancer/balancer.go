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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vearch/vearch/v3/internal/client"
	"github.com/vearch/vearch/v3/internal/entity"
	"github.com/vearch/vearch/v3/internal/pkg/log"
	json "github.com/vearch/vearch/v3/internal/pkg/vjson"
)

// Balancer is the Phase 1 module entry point.
type Balancer struct {
	cli       *client.Client
	cfg       *configHolder
	collector *StatsCollector
	builder   *SnapshotBuilder
	tracker   *TaskTracker
	scheduler *TaskScheduler
	planner   *Planner
	reaper    *Reaper

	ctx    context.Context
	cancel context.CancelFunc
}

/*
New constructs a Balancer. memberOps is injected at master startup to avoid
an import cycle.
*/
func New(ctx context.Context, cli *client.Client, memberOps MemberOps) (*Balancer, error) {
	cfg := newConfigHolder()
	// Load existing config from etcd.
	if data, err := cli.Master().Store.Get(ctx, entity.KeyBalancerConfig); err == nil && len(data) > 0 {
		c := &Config{}
		if err := json.Unmarshal(data, c); err == nil {
			// Validate persisted config; a bad value would otherwise feed cron loops.
			if verr := c.Validate(); verr != nil {
				log.Errorf("[balancer] etcd config invalid, fallback to default: %s", verr.Error())
			} else {
				cfg.Set(c)
				log.Infof("[balancer] config loaded from etcd: enabled=%v score_based_placement=%v",
					c.Enabled, c.ScoreBasedPlacement)
			}
		}
	}

	tracker := NewTaskTracker()
	bctx, bcancel := context.WithCancel(ctx)

	collector := NewStatsCollector(bctx, cli, cfg)
	builder := NewSnapshotBuilder(cli, cfg, tracker)
	scheduler := NewTaskScheduler(cli, cfg, memberOps, tracker)
	planner := NewPlanner(builder, scheduler, tracker, cfg)
	// Reaper holds a tracker reference so it can skip partitions with inflight migrations.
	reaper := NewReaper(cfg, memberOps, builder, tracker)

	if err := scheduler.LoadFromEtcd(ctx); err != nil {
		log.Errorf("[balancer] LoadFromEtcd failed: %s", err.Error())
	}

	return &Balancer{
		cli:       cli,
		cfg:       cfg,
		collector: collector,
		builder:   builder,
		tracker:   tracker,
		scheduler: scheduler,
		planner:   planner,
		reaper:    reaper,
		ctx:       bctx,
		cancel:    bcancel,
	}, nil
}

/*
Start launches periodic jobs (StatsCollector + inflight metric reporter).
Planner / Scheduler / Reaper crons are registered in registerBalancerJobs.
*/
func (b *Balancer) Start() {
	b.collector.Start()
	go b.reportInflightLoop()
	log.Info("[balancer] started")
}

func (b *Balancer) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
	if b.collector != nil {
		b.collector.Stop()
	}
}

func (b *Balancer) reportInflightLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[balancer:report] panic: %v", r)
		}
	}()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			setInflightCount(b.tracker.CountInflight())
		}
	}
}

// Builder returns the SnapshotBuilder.
func (b *Balancer) Builder() *SnapshotBuilder {
	return b.builder
}

// Scheduler returns the TaskScheduler.
func (b *Balancer) Scheduler() *TaskScheduler {
	return b.scheduler
}

// Planner returns the Planner.
func (b *Balancer) Planner() *Planner {
	return b.planner
}

// Reaper returns the Reaper.
func (b *Balancer) Reaper() *Reaper {
	return b.reaper
}

// Tracker returns the TaskTracker.
func (b *Balancer) Tracker() *TaskTracker {
	return b.tracker
}

// ConfigHolder returns the configHolder.
func (b *Balancer) ConfigHolder() *configHolder {
	return b.cfg
}

// Ctx returns the balancer's internal context; cancelled by Stop().
// Cron goroutines should listen on this so Stop fully halts them.
func (b *Balancer) Ctx() context.Context {
	return b.ctx
}

// RegisterPrometheus registers balancer metrics on the given registerer.
func (b *Balancer) RegisterPrometheus(reg prometheus.Registerer) {
	RegisterMetrics(reg)
}

/*
ReloadConfigFromEtcd re-reads the persisted config and updates the local holder.
Called from cron entrypoints so multi-master pause/resume / config updates
propagate without requiring an etcd watch.

Holds configHolder.mu through the Get so a concurrent updateConfigAtomic cannot
interleave: otherwise Reload could read stale etcd then store-over a fresh
in-memory write, leaving etcd and memory in drift until the next reload.
*/
func (b *Balancer) ReloadConfigFromEtcd(ctx context.Context) {
	b.cfg.mu.Lock()
	defer b.cfg.mu.Unlock()
	data, err := b.cli.Master().Store.Get(ctx, entity.KeyBalancerConfig)
	if err != nil || len(data) == 0 {
		return
	}
	c := &Config{}
	if err := json.Unmarshal(data, c); err != nil {
		log.Warnf("[balancer] reload config unmarshal: %s", err.Error())
		return
	}
	if err := c.Validate(); err != nil {
		log.Warnf("[balancer] reload config invalid: %s", err.Error())
		return
	}
	b.cfg.cfg.Store(c)
}

/*
updateConfigAtomic serializes pause/resume / config-update read-modify-write on
one master. fn mutates a *copy* of the current config; if fn or Validate returns
an error the in-memory holder is unchanged and nothing is written to etcd.

Etcd Put runs under the same mutex so two concurrent updates cannot interleave
writes. Cross-master races are still last-writer-wins (etcd STM would be needed
for cluster-wide atomicity; deferred to Phase 2).
*/
func (b *Balancer) updateConfigAtomic(ctx context.Context, fn func(*Config) error) error {
	b.cfg.mu.Lock()
	defer b.cfg.mu.Unlock()
	cur := *b.cfg.cfg.Load()
	if err := fn(&cur); err != nil {
		return err
	}
	if err := cur.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(&cur)
	if err != nil {
		return err
	}
	if err := b.cli.Master().Store.Put(ctx, entity.KeyBalancerConfig, data); err != nil {
		return err
	}
	b.cfg.cfg.Store(&cur)
	return nil
}
