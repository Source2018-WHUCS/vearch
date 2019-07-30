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
	"fmt"
	"github.com/parnurzeal/gorequest"
	"github.com/tiglabs/log"
)

var (
	NodeRole string = "node"
)

type ConsulRegistInfo struct {
	Name string
	ID string
	Address string
	Port int64
	Tags []string
}

func GetConsulId(app string, role string, host string, port int64) string {
	return fmt.Sprintf("%s_%s_%s_%d", app, role, host, port)
}

func RegistConsul(config *PrometheusConfig) {
	url := config.RegisterAddr
	SendRegistReq(url, config.App, config.Role, config.Host, config.Cluster, config.ExporterPort)
	if config.NodePort > 0 {
		SendRegistReq(url, config.App, NodeRole, config.Host, config.Cluster, config.NodePort)
	}
}

func SendRegistReq(url string, name string, role string, host string, cluster string, port int64) {
	id := GetConsulId(name, role, host, port)
	resp, body, errs := gorequest.New().Put(url).SendMap(ConsulRegistInfo{
		Name: name,
		ID: id,
		Address: host,
		Port: port,
		Tags: []string {
			"app=" + name,
			"role=" + role,
			"cluster=" + cluster,
		},
	}).End()
	if errs != nil {
		log.Error("Error on regist consul resp: %v, body: %v", body, resp)
	}
	if log.IsDebugEnabled() {
		log.Debug("regist consul %v %v", body, resp)
	}

	log.Info("regist exporter %s %s %s", id, url, cluster)
}