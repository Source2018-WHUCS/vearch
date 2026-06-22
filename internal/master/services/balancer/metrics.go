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
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	psScore = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vearch_balancer_ps_score",
		Help: "PS current score (data bytes)",
	}, []string{"node_id"})

	psAssignedScore = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vearch_balancer_ps_assigned_score",
		Help: "PS assigned score (memory weighted target)",
	}, []string{"node_id"})

	psPartitionCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "vearch_balancer_ps_partition_count",
		Help: "PS partition count",
	}, []string{"node_id"})

	migrationInflight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vearch_balancer_migration_inflight",
		Help: "In-flight migrations",
	})

	migrationTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "vearch_balancer_migration_total",
		Help: "Total migrations",
	}, []string{"reason", "result"})

	migrationDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vearch_balancer_migration_duration_sec",
		Help:    "Migration duration (sec)",
		Buckets: prometheus.ExponentialBuckets(10, 2, 10),
	}, []string{"reason"})

	clusterScoreStdDev = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "vearch_balancer_cluster_score_stddev",
		Help: "Cluster score normalized standard deviation",
	})

	registerOnce sync.Once
)

// RegisterMetrics is idempotent; pass nil to use prometheus.DefaultRegisterer.
func RegisterMetrics(reg prometheus.Registerer) {
	registerOnce.Do(func() {
		if reg == nil {
			reg = prometheus.DefaultRegisterer
		}
		reg.MustRegister(psScore, psAssignedScore, psPartitionCount,
			migrationInflight, migrationTotal, migrationDuration, clusterScoreStdDev)
	})
}

// updateSnapshotMetrics runs after every snapshot build.
func updateSnapshotMetrics(snap *Snapshot) {
	for _, item := range snap.LiveItems() {
		nodeStr := strconv.FormatUint(uint64(item.NodeID), 10)
		psScore.WithLabelValues(nodeStr).Set(item.CurrentScore)
		psAssignedScore.WithLabelValues(nodeStr).Set(item.AssignedScore)
		psPartitionCount.WithLabelValues(nodeStr).Set(float64(item.PartitionCount()))
	}
	clusterScoreStdDev.Set(DataBytesStdDev(snap.LiveItems()))
}

// recordMigrationCompleted records duration and result on task termination.
func recordMigrationCompleted(reason string, success bool, durationSec float64) {
	result := "success"
	if !success {
		result = "fail"
	}
	migrationTotal.WithLabelValues(reason, result).Inc()
	if success {
		migrationDuration.WithLabelValues(reason).Observe(durationSec)
	}
}

// setInflightCount is invoked periodically by the balancer.
func setInflightCount(n int) {
	migrationInflight.Set(float64(n))
}
