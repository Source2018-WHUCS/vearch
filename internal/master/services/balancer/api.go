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
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vearch/vearch/v3/internal/client"
	"github.com/vearch/vearch/v3/internal/entity"
	"github.com/vearch/vearch/v3/internal/pkg/log"
)

/*
RegisterAPI mounts /balancer/* routes. Caller must pass a router group with
BasicAuth already applied — these endpoints can trigger migrations.
*/
func (b *Balancer) RegisterAPI(r gin.IRouter) {
	g := r.Group("/balancer")
	g.GET("/config", b.handleGetConfig)
	g.PUT("/config", b.handleUpdateConfig)
	g.GET("/snapshot", b.handleSnapshot)
	g.GET("/tasks", b.handleListTasks)
	g.POST("/migrate", b.handleManualMigrate)
	g.POST("/pause", b.handlePause)
	g.POST("/resume", b.handleResume)
	g.POST("/reaper/trigger", b.handleReaperTrigger)
}

func (b *Balancer) handleGetConfig(c *gin.Context) {
	/*
		Multi-master read consistency: each master's cfgHolder caches the last
		ReloadConfigFromEtcd snapshot. PUT on master X updates X's cache and
		etcd; if the next GET round-robins to master Y, Y's cache is stale
		until its cron tick reloads. Pull the latest from etcd on every read
		so writes are observable across masters.
	*/
	b.ReloadConfigFromEtcd(c.Request.Context())
	c.JSON(http.StatusOK, b.cfg.Get())
}

func (b *Balancer) handleUpdateConfig(c *gin.Context) {
	var req Config
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Atomic RMW serializes against pause/resume on the same master.
	if err := b.updateConfigAtomic(c.Request.Context(), func(cfg *Config) error {
		*cfg = req
		return nil
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Infof("[balancer] config updated via API: enabled=%v score_based_placement=%v",
		req.Enabled, req.ScoreBasedPlacement)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (b *Balancer) handleSnapshot(c *gin.Context) {
	ctx := c.Request.Context()
	/*
		Multi-master: each master's tracker.inflight is populated only by
		local Submit + the scheduler-cron SyncFromEtcd. Between cron ticks
		(30s), a recently submitted task on another master is invisible here.
		Force a sync so inflight_count reflects the etcd-persisted truth on
		every read.
	*/
	if err := b.scheduler.SyncFromEtcd(ctx); err != nil {
		log.Warnf("[balancer] /snapshot: SyncFromEtcd failed (returning stale view): %s", err.Error())
	}
	snap, err := b.builder.Build(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	view := make([]map[string]any, 0)
	for _, item := range snap.LiveItems() {
		view = append(view, map[string]any{
			"node_id":         item.NodeID,
			"ip":              item.Server.Ip,
			"current_score":   item.CurrentScore,
			"assigned_score":  item.AssignedScore,
			"priority":        item.Priority,
			"partition_count": item.PartitionCount(),
			"leader_count":    item.Stat.LeaderCount,
			"data_bytes":      item.Stat.TotalDataBytes,
			"disk_usage":      item.Stat.DiskUsage,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"items":          view,
		"score_std_dev":  DataBytesStdDev(snap.LiveItems()),
		"inflight_count": b.tracker.CountInflight(),
	})
}

func (b *Balancer) handleListTasks(c *gin.Context) {
	/*
		Same multi-master read-consistency concern as /snapshot: tracker is
		populated per-master, so we sync from etcd before serving.
	*/
	if err := b.scheduler.SyncFromEtcd(c.Request.Context()); err != nil {
		log.Warnf("[balancer] /tasks: SyncFromEtcd failed (returning stale view): %s", err.Error())
	}
	c.JSON(http.StatusOK, b.tracker.AllInflight())
}

// handleManualMigrate kicks off a one-off migration.
type manualMigrateReq struct {
	PartitionID entity.PartitionID `json:"partition_id"`
	FromNodeID  entity.NodeID      `json:"from_node_id"`
	ToNodeID    entity.NodeID      `json:"to_node_id"`
	Reason      string             `json:"reason,omitempty"`
}

func (b *Balancer) handleManualMigrate(c *gin.Context) {
	var req manualMigrateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	partition, err := b.cli.Master().QueryPartition(ctx, req.PartitionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	/*
		Fail-fast on bad input so callers get an immediate error instead of
		discovering it via /balancer/tasks after the advance loop has tried.
	*/
	if !containsReplica(partition.Replicas, req.FromNodeID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("from_node_id %d is not a replica of partition %d (current replicas: %v)",
				req.FromNodeID, req.PartitionID, partition.Replicas),
		})
		return
	}
	if containsReplica(partition.Replicas, req.ToNodeID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("to_node_id %d already has a replica of partition %d",
				req.ToNodeID, req.PartitionID),
		})
		return
	}
	toSrv, err := b.cli.Master().QueryServer(ctx, req.ToNodeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("to_node_id %d not registered: %s", req.ToNodeID, err.Error()),
		})
		return
	}
	if !client.IsLive(toSrv.RpcAddr()) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("to_node_id %d (%s) is not live", req.ToNodeID, toSrv.RpcAddr()),
		})
		return
	}
	/*
		Manual migrate is intentionally NOT STM-locked: if two masters receive
		concurrent requests for the same partition, each Submit's local dedup
		passes and etcd ends up with two tasks. On restart LoadFromEtcd loads
		both; the second's preAdd then sees ToNodeID already in replicas and
		fails. Self-healing at the cost of one spurious failure metric.
	*/
	reason := req.Reason
	if reason == "" {
		reason = "manual"
	}
	op := MigrateOp{
		PartitionID: req.PartitionID,
		SpaceID:     partition.SpaceId,
		FromNodeID:  req.FromNodeID,
		ToNodeID:    req.ToNodeID,
		Reason:      reason,
	}
	task, err := b.scheduler.Submit(ctx, op)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Infof("[balancer] manual migrate submitted via API: task=%s partition=%d %d→%d reason=%s",
		task.ID, op.PartitionID, op.FromNodeID, op.ToNodeID, op.Reason)
	c.JSON(http.StatusOK, task)
}

func (b *Balancer) handlePause(c *gin.Context) {
	if err := b.setEnabled(c.Request.Context(), false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Infof("[balancer] paused via API")
	c.JSON(http.StatusOK, gin.H{"ok": true, "enabled": false})
}

func (b *Balancer) handleResume(c *gin.Context) {
	if err := b.setEnabled(c.Request.Context(), true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Infof("[balancer] resumed via API")
	c.JSON(http.StatusOK, gin.H{"ok": true, "enabled": true})
}

/*
setEnabled persists the new Enabled flag to etcd so multi-master deployments
converge. Other masters still need to re-read on cron entry to fully honor it;
see refreshConfig in registerBalancerJobs.
*/
func (b *Balancer) setEnabled(ctx context.Context, enabled bool) error {
	return b.updateConfigAtomic(ctx, func(cfg *Config) error {
		cfg.Enabled = enabled
		return nil
	})
}

func (b *Balancer) handleReaperTrigger(c *gin.Context) {
	/*
		Multi-master: in a 3-master HA cluster the request lands on a random
		master via LB. The previous "try-once" pattern dropped the trigger
		when the receiving master did not hold the reaper STM lock at that
		moment, which made /reaper/trigger effectively unreliable when called
		through a round-robin LB.

		Spawn a background goroutine that retries lock acquisition for up to
		~90s (covers the 60s lock TTL plus margin). The HTTP response returns
		immediately so callers do not block. Tests poll the cluster state to
		observe the effect of RunOnce; ~90s is well within their tolerance.
	*/
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		b.ReloadConfigFromEtcd(ctx)
		for {
			err := AcquireSTMLock(ctx, b.cli, LockKeyReaper, 60)
			if err == nil {
				break
			}
			if !IsSkip(err) {
				log.Errorf("[balancer:reaper] manual trigger lock acquire failed: %s", err.Error())
				return
			}
			select {
			case <-ctx.Done():
				log.Warnf("[balancer:reaper] manual trigger gave up after 90s; another master held the lock the entire time")
				return
			case <-time.After(2 * time.Second):
			}
		}
		if err := b.reaper.RunOnce(ctx); err != nil {
			log.Errorf("[balancer:reaper] manual trigger run failed: %s", err.Error())
		}
	}()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
