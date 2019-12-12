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

package master

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/vearch/vearch/config"
	pkg "github.com/vearch/vearch/proto"
	"github.com/vearch/vearch/proto/entity"
	"github.com/vearch/vearch/util/log"
	"github.com/vearch/vearch/util/metrics/mserver"
	"github.com/vearch/vearch/util/monitoring"
	"github.com/vearch/vearch/util/uuid"
	"strings"
	"time"
)

//masterService is used for master administrator purpose.It should not be used by router and partition server program
type monitorService struct {
	*masterService
}

func (this *monitorService) statsService(ctx context.Context) ([]*mserver.ServerStats, error) {
	servers, err := this.Master().QueryServers(ctx)
	if err != nil {
		return nil, err
	}

	statsChan := make(chan *mserver.ServerStats, len(servers))

	for _, s := range servers {
		go func(s *entity.Server) {
			defer func() {
				if r := recover(); r != nil {
					statsChan <- mserver.NewErrServerStatus(s.RpcAddr(), errors.New(cast.ToString(r)))
				}
			}()
			statsChan <- this.Client.PS().Beg(ctx, uuid.FlakeUUID()).Admin(s.RpcAddr()).ServerStats()
		}(s)
	}

	result := make([]*mserver.ServerStats, 0, len(servers))

	for {
		select {
		case s := <-statsChan:
			result = append(result, s)
		case <-ctx.Done():
			return nil, pkg.CodeErr(pkg.ERRCODE_TIMEOUT)
		default:
			time.Sleep(time.Millisecond * 10)
			if len(result) >= len(servers) {
				close(statsChan)
				goto out
			}
		}
	}

out:

	return result, nil
}

func newMonitorService(masterService *masterService) *monitorService {
	return &monitorService{masterService}
}

func (ms *monitorService) Register() {
	msConf := config.Conf().Masters.Self()
	if msConf != nil && msConf.Monitor {
		monitoring.RegisterMaster(ms.monitorCallBack)
	} else {
		log.Debug("skip register master monitor")
	}
}

func (this *monitorService) partitionInfo(ctx context.Context, dbName, spaceName string) ([]map[string]interface{}, error) {
	dbNames := make([]string, 0)
	if dbName != "" {
		dbNames = strings.Split(dbName, ",")
	}

	if len(dbNames) == 0 {
		dbs, err := this.queryDBs(ctx)
		if err != nil {
			return nil, err
		}
		dbNames = make([]string, len(dbs))
		for i, db := range dbs {
			dbNames[i] = db.Name
		}
	}

	color := []string{"green", "yellow", "red"}

	var errors []string

	spaceNames := strings.Split(spaceName, ",")

	resultInsideDbs := make([]map[string]interface{}, 0)
	for i := range dbNames {
		dbName := dbNames[i]

		dbId, err := this.Master().QueryDBName2Id(ctx, dbName)
		if err != nil {
			errors = append(errors, dbName+" find dbID err: "+err.Error())
			continue
		}

		spaces, err := this.Master().QuerySpaces(ctx, dbId)
		if err != nil {
			errors = append(errors, dbName+" find space err: "+err.Error())
			continue
		}

		dbStatus := 0

		resultInsideSpaces := make([]map[string]interface{}, 0, len(spaces))
		for _, space := range spaces {

			spaceName := space.Name

			if len(spaceNames) > 1 || spaceNames[0] != "" { //filter spaceName by user define
				var index = -1

				for i, name := range spaceNames {
					if name == spaceName {
						index = i
						break
					}
				}

				if index < 0 {
					continue
				}
			}

			spaceStatus := 0
			resultInsidePartition := make([]*entity.PartitionInfo, 0)
			for _, spacePartition := range space.Partitions {
				p, err := this.Master().QueryPartition(ctx, spacePartition.Id)
				if err != nil {
					errors = append(errors, fmt.Sprintf("partition:[%d] not found in space: [%s]", spacePartition.Id, spaceName))
					continue
				}

				pStatus := 0

				nodeID := p.LeaderID
				if nodeID == 0 {
					errors = append(errors, fmt.Sprintf("partition:[%d] no leader in space: [%s]", spacePartition.Id, spaceName))
					pStatus = 2
					nodeID = p.Replicas[0]
				}

				server, err := this.Master().QueryServer(ctx, nodeID)
				if err != nil {
					errors = append(errors, fmt.Sprintf("server:[%d] not found in space: [%s] , partition:[%d]", nodeID, spaceName, spacePartition.Id))
					pStatus = 2
					continue
				}

				partitionInfo, err := this.Client.PS().Beg(ctx, uuid.FlakeUUID()).Admin(server.RpcAddr()).PartitionInfo(p.Id)
				if err != nil {
					errors = append(errors, fmt.Sprintf("query space:[%s] server:[%d] partition:[%d] info err :[%s]", spaceName, nodeID, spacePartition.Id, err.Error()))
					partitionInfo = &entity.PartitionInfo{}
					pStatus = 2
				} else {
					if len(partitionInfo.Unreachable) > 0 {
						pStatus = 1
					}
				}

				//this must from space.Partitions
				partitionInfo.PartitionID = spacePartition.Id
				partitionInfo.Color = color[pStatus]
				partitionInfo.ReplicaNum = len(p.Replicas)
				partitionInfo.Ip = server.Ip
				partitionInfo.NodeID = server.ID

				resultInsidePartition = append(resultInsidePartition, partitionInfo)

				if pStatus > spaceStatus {
					spaceStatus = pStatus
				}
			}

			docNum := uint64(0)
			size := int64(0)
			for _, p := range resultInsidePartition {
				docNum += cast.ToUint64(p.DocNum)
				size += cast.ToInt64(p.Size)
			}
			resultSpace := make(map[string]interface{})
			resultSpace["name"] = spaceName
			resultSpace["partition_num"] = len(resultInsidePartition)
			resultSpace["replica_num"] = len(resultInsidePartition) * int(space.ReplicaNum)
			resultSpace["doc_num"] = docNum
			resultSpace["size"] = size
			resultSpace["partitions"] = resultInsidePartition
			resultSpace["status"] = color[spaceStatus]
			resultInsideSpaces = append(resultInsideSpaces, resultSpace)

			if spaceStatus > dbStatus {
				dbStatus = spaceStatus
			}
		}

		docNum := uint64(0)
		size := int64(0)
		for _, s := range resultInsideSpaces {
			docNum += cast.ToUint64(s["doc_num"])
			size += cast.ToInt64(s["size"])
		}
		resultDb := make(map[string]interface{})
		resultDb["db_name"] = dbName
		resultDb["space_num"] = len(spaces)
		resultDb["doc_num"] = docNum
		resultDb["size"] = size
		resultDb["spaces"] = resultInsideSpaces
		resultDb["status"] = color[dbStatus]
		resultDb["errors"] = errors
		resultInsideDbs = append(resultInsideDbs, resultDb)
	}

	return resultInsideDbs, nil
}


func (ms *monitorService) monitorCallBack(masterMonitor *monitoring.MasterMonitor) {

}
