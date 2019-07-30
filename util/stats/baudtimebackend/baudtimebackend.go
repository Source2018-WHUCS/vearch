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

package baudtimebackend

import (
	"context"
	"expvar"
	"strings"

	"github.com/tiglabs/baudengine/util/stats"
	"github.com/tiglabs/log"

	baudtimeClient "git.jd.com/baudtime/baudtime/client"
	pb "git.jd.com/baudtime/baudtime/proxy/proxypb"
)

var (
	backendName      = "baudTime"
	nameLabelKey     = "__name__"
	hostLabelKey     = "host"
	instanceLabelKey = "instance"
	appLabelKey      = "app"
	serviceLabelKey  = "service"
	roleLabelKey     = "role"
	idcLableKey      = "idc"
	clusterLableKey  = "cluster"
)

type BaudTimeBackend struct {
	client *baudtimeClient.BaudClient
	cfg    *BaudTimeConfig
}

func Init(config *BaudTimeConfig) {
	if config == nil || !config.Enabled {
		return
	}

	client := baudtimeClient.New(func(response *pb.WriteResponse) error {
		return nil
	}, baudtimeClient.NewStaticAddrProvider(strings.Split(config.ProxyAddr, ",")...))

	stats.RegisterPushBackend(backendName,
		&BaudTimeBackend{
			client: client,
			cfg:    config,
		},
		config.Period)

	log.Info("init baudtime pushbackend %s %s", config.ProxyAddr, config.Period)
}

func GetNamespace(config *BaudTimeConfig) string {
	return config.App + "_" + config.Role
}

func (backend *BaudTimeBackend) PushAll() error {
	mc := &metricsCollector{commLabels: []pb.Label{
		{hostLabelKey, backend.cfg.Host},
		{instanceLabelKey, backend.cfg.Host},
		{appLabelKey, backend.cfg.App},
		{serviceLabelKey, backend.cfg.App},
		{roleLabelKey, backend.cfg.Role},
		{idcLableKey, backend.cfg.Idc},
		{clusterLableKey, backend.cfg.Cluster},
	}}

	expvar.Do(func(kv expvar.KeyValue) {
		name := GetNamespace(backend.cfg) + "_" + kv.Key
		mc.collectMetrics(name, &kv)
	})

	if log.IsDebugEnabled() {
		log.Debug("push metrics %v ", mc.metrics)
	}

	err := backend.client.Write(context.Background(), mc.metrics...)
	if err != nil {
		log.Error("write to baudtime err", err)
	}

	return nil
}
