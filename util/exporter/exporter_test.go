// Copyright 2018 The ChuBao Authors.
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

package exporter

import (
	"os"
	"testing"
	"time"

	"github.com/tiglabs/baudengine/util/stats/prometheusbackend"
	"vitess.io/vitess/go/stats"
)

func TestCounter(t *testing.T) {
	reqCount := stats.NewCounter("req_count1", "req count")
	reqCount.Add(10)
	reqCount.Add(20)
	reqCount.Add(30)
}

func TestGauge(t *testing.T) {
	startTime := stats.NewGauge("start_time1", "Exporter start time")
	startTime.Set(time.Now().Unix() * 1000)
}

func TestHistogram(t *testing.T) {
	cpuHistogram := stats.NewHistogram("cpu_histogram1", "cpu Histogram", []int64{10, 20, 80})
	cpuHistogram.Add(20)
	cpuHistogram.Add(30)
	cpuHistogram.Add(40)
}

func TestMain(m *testing.M) {
	config := &Config{
		Prometheus: prometheusbackend.PrometheusConfig{
			Enabled:      true,
			ExporterPort: 8917,
			RegisterAddr: "127.0.0.1:8500",
		},
		/*
			Baudtime: baudtimebackend.BaudTimeConfig{
				Enabled: true,
				ProxyAddr: "127.0.0.1:8087",
				Period: "10s",
			},
		*/
	}
	config.Host = "127.0.0.1"
	config.App = "cbdb"
	config.Role = "master"
	config.Idc = "idc1"

	Init(config)

	os.Exit(m.Run())
}
