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
	"encoding/binary"
	"errors"
	"time"

	"github.com/vearch/vearch/v3/internal/client"
	"github.com/vearch/vearch/v3/internal/entity"
	"go.etcd.io/etcd/client/v3/concurrency"
)

var errSkipJob = errors.New("skip job")

/*
AcquireSTMLock gates entry across multi-master via an etcd STM timestamp.
holdSec is the lock validity after acquisition.
Returns nil on success, errSkipJob if another master holds the lock.
*/
func AcquireSTMLock(ctx context.Context, cli *client.Client, key string, holdSec int) error {
	return cli.Master().Store.STM(ctx, func(stm concurrency.STM) error {
		raw := stm.Get(key)
		if len(raw) == 8 {
			expiry := int64(binary.LittleEndian.Uint64([]byte(raw)))
			if time.Now().UnixNano() < expiry {
				return errSkipJob
			}
		}
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, uint64(time.Now().UnixNano()+int64(holdSec)*int64(time.Second)))
		stm.Put(key, string(bytes))
		return nil
	})
}

// IsSkip reports whether the error means "another master is running, skip".
func IsSkip(err error) bool {
	return err == errSkipJob
}

// Cron jobs use distinct lock keys.
var (
	LockKeyPlanner = entity.KeyBalancerLock + "/planner"
	LockKeyReaper  = entity.KeyBalancerLock + "/reaper"
	/*
		Scheduler.AdvanceAll must also be STM-locked: otherwise multi-master
		deployments would have multiple masters advance the same inflight task,
		duplicating ChangeMember calls and corrupting etcd state via last-writer-wins.
	*/
	LockKeyScheduler = entity.KeyBalancerLock + "/scheduler"
)
