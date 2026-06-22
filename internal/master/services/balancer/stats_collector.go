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
	"sync"
	"time"

	"github.com/vearch/vearch/v3/internal/client"
	"github.com/vearch/vearch/v3/internal/entity"
	"github.com/vearch/vearch/v3/internal/pkg/log"
	"github.com/vearch/vearch/v3/internal/pkg/metrics/mserver"
	json "github.com/vearch/vearch/v3/internal/pkg/vjson"
)

// StatsCollector periodically pulls load state from each PS and writes it to etcd.
type StatsCollector struct {
	cli    *client.Client
	cfg    *configHolder
	ctx    context.Context
	cancel context.CancelFunc
}

func NewStatsCollector(ctx context.Context, cli *client.Client, cfg *configHolder) *StatsCollector {
	cctx, cancel := context.WithCancel(ctx)
	return &StatsCollector{cli: cli, cfg: cfg, ctx: cctx, cancel: cancel}
}

// Start launches the periodic collection loop.
func (sc *StatsCollector) Start() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("[balancer:stats] recovered from panic: %v", r)
			}
		}()
		intervalSec := sc.cfg.Get().StatsCollectIntervalSec
		if intervalSec <= 0 {
			intervalSec = 15
		}
		ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-sc.ctx.Done():
				return
			case <-ticker.C:
				sc.collectOnce()
			}
		}
	}()
}

func (sc *StatsCollector) Stop() {
	sc.cancel()
}

func (sc *StatsCollector) collectOnce() {
	servers, err := sc.cli.Master().QueryServers(sc.ctx)
	if err != nil {
		log.Errorf("[balancer:stats] QueryServers failed: %s", err.Error())
		return
	}
	spaces, err := sc.cli.Master().QuerySpacesByKey(sc.ctx, entity.PrefixSpace)
	if err != nil {
		log.Errorf("[balancer:stats] QuerySpacesByKey failed: %s", err.Error())
		return
	}

	// partitionID → spaceID reverse index.
	pidToSpace := make(map[entity.PartitionID]entity.SpaceID)
	for _, sp := range spaces {
		for _, p := range sp.Partitions {
			pidToSpace[p.Id] = sp.Id
		}
	}

	var wg sync.WaitGroup
	for _, srv := range servers {
		wg.Add(1)
		go func(s *entity.Server) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("[balancer:stats] collect node %d panic: %v", s.ID, r)
				}
			}()
			stats := client.ServerStats(s.RpcAddr())
			if stats == nil {
				log.Warnf("[balancer:stats] ServerStats nil for node %d", s.ID)
				return
			}
			psStat := buildPSStat(s, stats, pidToSpace)
			data, err := json.Marshal(psStat)
			if err != nil {
				log.Errorf("[balancer:stats] marshal PSStat failed: %s", err.Error())
				return
			}
			ttl := time.Duration(sc.cfg.Get().PSStatTTLSec) * time.Second
			if err := sc.cli.Master().Store.CreateWithTTL(sc.ctx, entity.PSStatKey(s.ID), data, ttl); err != nil {
				log.Errorf("[balancer:stats] put PSStat failed (node %d): %s", s.ID, err.Error())
			}
		}(srv)
	}
	wg.Wait()
}

func buildPSStat(s *entity.Server, stats *mserver.ServerStats, pidToSpace map[entity.PartitionID]entity.SpaceID) *entity.PSStat {
	psStat := &entity.PSStat{
		NodeID:     s.ID,
		UpdateTime: time.Now().UnixMilli(),
		SpaceStats: make(map[entity.SpaceID]*entity.SpaceLoadOnPS),
	}

	if stats.Mem != nil {
		psStat.MemCapacityBytes = uint64(stats.Mem.Total)
	}
	if stats.Fs != nil {
		psStat.DiskCapacityBytes = uint64(stats.Fs.Total)
		psStat.DiskFreeBytes = uint64(stats.Fs.Free)
		if stats.Fs.Total > 0 {
			psStat.DiskUsage = 1 - float64(stats.Fs.Free)/float64(stats.Fs.Total)
		}
	}

	psStat.PartitionCount = len(stats.PartitionInfos)
	for _, p := range stats.PartitionInfos {
		if p == nil {
			continue
		}
		spaceID, ok := pidToSpace[p.PartitionID]
		if !ok {
			continue
		}
		// Leader?
		if p.RaftStatus != nil && p.RaftStatus.Leader == p.RaftStatus.NodeID {
			psStat.LeaderCount++
		}
		psStat.TotalDocNum += p.DocNum
		psStat.TotalDataBytes += uint64(p.Size)

		load, exists := psStat.SpaceStats[spaceID]
		if !exists {
			load = &entity.SpaceLoadOnPS{
				SpaceID:        spaceID,
				PartitionBytes: make(map[entity.PartitionID]uint64),
			}
			psStat.SpaceStats[spaceID] = load
		}
		load.PartitionCount++
		load.DataBytes += uint64(p.Size)
		load.DocNum += p.DocNum
		load.PartitionIDs = append(load.PartitionIDs, p.PartitionID)
		if load.PartitionBytes == nil {
			load.PartitionBytes = make(map[entity.PartitionID]uint64)
		}
		load.PartitionBytes[p.PartitionID] = uint64(p.Size)
	}

	return psStat
}
