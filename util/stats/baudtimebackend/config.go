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
	"github.com/tiglabs/baudengine/util/stats"
	"github.com/tiglabs/baudengine/util/netutil"
)

type BaudTimeConfig struct {
	stats.Config
	Enabled   bool   `toml:"enabled,omitempty" json:"enabled"`
	ProxyAddr string `toml:"proxy-addr,omitempty" json:"proxy-addr"`
	Period   string `toml:"period,omitempty" json:"period"`
}

func (this *BaudTimeConfig)InitConfig(config *stats.Config) *BaudTimeConfig{
	ip := netutil.GetPrivateIP().To4().String()

	this.App = config.App
	this.Host = ip
	this.Role = config.Role
	this.Idc = config.Idc

	return this
}