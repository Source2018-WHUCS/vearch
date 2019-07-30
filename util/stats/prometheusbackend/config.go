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

package prometheusbackend

import (
	"github.com/tiglabs/log"
	"github.com/tiglabs/baudengine/util/stats"
	"github.com/tiglabs/baudengine/util/netutil"
)

type PrometheusConfig struct {
	stats.Config
	Enabled   bool   `toml:"enabled,omitempty" json:"enabled"`
	RegisterAddr      string `toml:"register-addr,omitempty" json:"register-addr"`
	ExporterInterface      string `toml:"exporter-interface,omitempty" json:"exporter-interface"`
	ExporterPort      int64 `toml:"exporter-port,omitempty" json:"exporter-port"`
	NodePort      int64 `toml:"node-port,omitempty" json:"node-port"`
}

func (this *PrometheusConfig)InitConfig(config *stats.Config) *PrometheusConfig{
	this.App = config.App
	this.Role = config.Role
	this.Idc = config.Idc
	this.Cluster = config.Cluster
	hostip := netutil.GetPrivateIP().To4().String()
	if len(this.ExporterInterface) > 0 {
		ip := netutil.GetPrivateIPByName(this.ExporterInterface)
		if ip != nil {
			hostip = ip.To4().String()
		}
		log.Info("get hostip %s at interface %s" , hostip, this.ExporterInterface)
	}
	this.Host = hostip

	return this
}
