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
	"time"

	"vitess.io/vitess/go/stats"

	st "github.com/tiglabs/baudengine/util/stats"
	"github.com/tiglabs/baudengine/util/stats/prometheusbackend"
	//"github.com/tiglabs/baudengine/util/stats/baudtimebackend"
)


type Config struct {
	st.Config
	Prometheus prometheusbackend.PrometheusConfig   `toml:"prometheus,omitempty" json:"prometheus"`
	//Baudtime baudtimebackend.BaudTimeConfig   `toml:"baudtime,omitempty" json:"baudtime"`
}


func Init(config *Config) {
	statsCfg := &st.Config{
		Host: config.Host,
		App: config.App,
		Role: config.Role,
		Idc: config.Idc,
		Cluster: config.Cluster,
	}

	config.Prometheus.InitConfig(statsCfg)
	prometheusbackend.Init(&config.Prometheus)

	//config.Baudtime.InitConfig(statsCfg)
	//baudtimebackend.Init(&config.Baudtime)

	startTime := stats.NewGauge("start_time", "Exporter start time")
	startTime.Set(time.Now().Unix() * 1000)
}
