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
	"sync"
	"sync/atomic"
)

// Config holds Phase 1 settings.
type Config struct {
	// Master switches
	Enabled             bool `json:"enabled"`
	ScoreBasedPlacement bool `json:"score_based_placement"`

	// Static scoring factor
	CrossSpaceFactor float64 `json:"cross_space_factor"`

	// Hard constraints
	DiskRejectThreshold float64 `json:"disk_reject_threshold"`

	// Trigger conditions
	PartitionDiffThreshold int     `json:"partition_diff_threshold"`
	DataBytesSkewThreshold float64 `json:"data_bytes_skew_threshold"`
	MinBenefitRatio        float64 `json:"min_benefit_ratio"`

	// Scheduling cadence
	PlanIntervalSec         int `json:"plan_interval_sec"`
	MaxOpsPerRound          int `json:"max_ops_per_round"`
	MaxOpsPerSpacePerRound  int `json:"max_ops_per_space_per_round"`
	CooldownPerPartitionSec int `json:"cooldown_per_partition_sec"`

	// Capacity pools
	ConfChangePoolSize  int `json:"conf_change_pool_size"`
	DataReplicaPoolSize int `json:"data_replica_pool_size"`

	// State-machine timeouts
	AddMemberTimeoutSec    int `json:"add_member_timeout_sec"`
	CaughtUpTimeoutSec     int `json:"caught_up_timeout_sec"`
	RemoveMemberTimeoutSec int `json:"remove_member_timeout_sec"`

	// Leader migration (must be false in Phase 1)
	AllowLeaderMigration bool `json:"allow_leader_migration"`

	/*
		Strict anti-affinity: true = error when zone constraint unmet,
		false = soft fallback that ignores zone.
	*/
	StrictAntiAffinity bool `json:"strict_anti_affinity"`

	// Reaper
	RedundantReaperEnabled     bool `json:"redundant_reaper_enabled"`
	RedundantReaperIntervalSec int  `json:"redundant_reaper_interval_sec"`

	// Data collection
	StatsCollectIntervalSec int `json:"stats_collect_interval_sec"`
	PSStatTTLSec            int `json:"ps_stat_ttl_sec"`
}

// DefaultConfig returns the default (all-off) config; rollout via API.
func DefaultConfig() *Config {
	return &Config{
		Enabled:                    false,
		ScoreBasedPlacement:        false,
		CrossSpaceFactor:           0.5,
		DiskRejectThreshold:        0.85,
		PartitionDiffThreshold:     1,
		DataBytesSkewThreshold:     0.3,
		MinBenefitRatio:            0.2,
		PlanIntervalSec:            300,
		MaxOpsPerRound:             5,
		MaxOpsPerSpacePerRound:     2,
		CooldownPerPartitionSec:    1800,
		ConfChangePoolSize:         20,
		DataReplicaPoolSize:        2,
		AddMemberTimeoutSec:        60,
		CaughtUpTimeoutSec:         1800,
		RemoveMemberTimeoutSec:     60,
		AllowLeaderMigration:       false,
		StrictAntiAffinity:         true,
		RedundantReaperEnabled:     true,
		RedundantReaperIntervalSec: 600,
		StatsCollectIntervalSec:    15,
		PSStatTTLSec:               90,
	}
}

// configHolder allows atomic config swap (supports hot reload).
// mu serializes read-modify-write across pause/resume / updateConfig so two
// concurrent admin actions on the same master cannot lose each other's updates.
// (Cross-master races still last-writer-wins; an etcd STM would be needed there.)
type configHolder struct {
	mu  sync.Mutex
	cfg atomic.Pointer[Config]
}

func newConfigHolder() *configHolder {
	h := &configHolder{}
	h.Set(DefaultConfig())
	return h
}

func (h *configHolder) Get() *Config {
	return h.cfg.Load()
}

// Set replaces the config holder under mu so callers that bypass
// updateConfigAtomic cannot defeat its RMW serialization.
// (Get is lock-free via atomic.Pointer.)
func (h *configHolder) Set(c *Config) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cfg.Store(c)
}

// Validate rejects values that would cause cron loops or unsound math.
func (c *Config) Validate() error {
	if c.CrossSpaceFactor < 0 || c.CrossSpaceFactor > 1 {
		return fmt.Errorf("cross_space_factor must be in [0,1], got %v", c.CrossSpaceFactor)
	}
	if c.DiskRejectThreshold <= 0 || c.DiskRejectThreshold > 1 {
		return fmt.Errorf("disk_reject_threshold must be in (0,1], got %v", c.DiskRejectThreshold)
	}
	if c.PartitionDiffThreshold < 1 {
		return fmt.Errorf("partition_diff_threshold must be >=1, got %d", c.PartitionDiffThreshold)
	}
	if c.DataBytesSkewThreshold < 0 {
		return fmt.Errorf("data_bytes_skew_threshold must be >=0, got %v", c.DataBytesSkewThreshold)
	}
	if c.MinBenefitRatio < 0 || c.MinBenefitRatio >= 1 {
		return fmt.Errorf("min_benefit_ratio must be in [0,1), got %v", c.MinBenefitRatio)
	}
	if c.PlanIntervalSec <= 0 {
		return fmt.Errorf("plan_interval_sec must be >0, got %d", c.PlanIntervalSec)
	}
	if c.MaxOpsPerRound <= 0 {
		return fmt.Errorf("max_ops_per_round must be >0, got %d", c.MaxOpsPerRound)
	}
	if c.MaxOpsPerSpacePerRound <= 0 {
		return fmt.Errorf("max_ops_per_space_per_round must be >0, got %d", c.MaxOpsPerSpacePerRound)
	}
	if c.ConfChangePoolSize <= 0 {
		return fmt.Errorf("conf_change_pool_size must be >0, got %d", c.ConfChangePoolSize)
	}
	if c.DataReplicaPoolSize <= 0 {
		return fmt.Errorf("data_replica_pool_size must be >0, got %d", c.DataReplicaPoolSize)
	}
	if c.AddMemberTimeoutSec <= 0 || c.CaughtUpTimeoutSec <= 0 || c.RemoveMemberTimeoutSec <= 0 {
		return fmt.Errorf("*_timeout_sec must be >0")
	}
	if c.StatsCollectIntervalSec <= 0 {
		return fmt.Errorf("stats_collect_interval_sec must be >0, got %d", c.StatsCollectIntervalSec)
	}
	if c.PSStatTTLSec <= c.StatsCollectIntervalSec {
		return fmt.Errorf("ps_stat_ttl_sec(%d) must be > stats_collect_interval_sec(%d)",
			c.PSStatTTLSec, c.StatsCollectIntervalSec)
	}
	return nil
}
