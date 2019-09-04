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

package ump

import (
	"github.com/vearch/vearch/util/monitoring"
	"time"
)

var _ monitoring.Monitor = &UmpMonitor{}

type UmpMonitor struct {
	key string
}

func (um *UmpMonitor) New(key string) monitoring.Monitor{
	return &UmpMonitor{key:key}
}

func (um *UmpMonitor) Alive() {
	Alive(um.key)
}

func (um *UmpMonitor) Alarm(detail string) {
	Alarm(um.key, detail)
}

func (um *UmpMonitor) FunctionTP(startTime time.Time, hasErr bool) {
	FunctionTP(um.key, startTime, hasErr)
}
