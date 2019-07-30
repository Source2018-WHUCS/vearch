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

package client

import (
	"fmt"
	"github.com/tiglabs/baudengine/proto/request"
	"github.com/tiglabs/baudengine/proto/response"
	"github.com/tiglabs/baudengine/util"
	"math"
	"sync"
	"time"

	"github.com/tiglabs/baudengine/proto"
	"github.com/tiglabs/baudengine/proto/pspb"
	"github.com/tiglabs/baudengine/util/bytes"

	"github.com/spaolacci/murmur3"
	"github.com/spf13/cast"
	"github.com/tiglabs/baudengine/proto/entity"
	"github.com/tiglabs/log"
	"runtime/debug"
)

type spaceSender struct {
	*sender
	routingValue  string
	writeTryTimes int
	clientType    ClientType
	db            string
	space         string
}

func (this *spaceSender) Type(clientType ClientType) *spaceSender {
	this.clientType = clientType
	return this
}

func (this *spaceSender) partitionSlot(slot entity.SlotID) *partitionSender {
	return &partitionSender{spaceSender: this, slot: slot}
}

//if user partitionId meas id not change , so retry is not a good idea ,
// example delete(ids...) this partitionID is changed , the slot will not same as groupIdMap
func (this *spaceSender) partitionId(pid entity.PartitionID) *partitionSender {
	return &partitionSender{spaceSender: this, pid: pid}
}

func (this *spaceSender) SetRoutingValue(routingValue string) *spaceSender {
	this.routingValue = routingValue
	return this
}

func (this *spaceSender) SetWriteTryTimes(times int) *spaceSender {
	this.writeTryTimes = times
	return this
}

func (this *spaceSender) GetDoc(id string) *response.DocResult {
	resp, retry := this._getDoc(id)
	if !retry {
		return resp
	}

	for i := 0; i < spaceRetry && retry; i++ {
		resp, retry = this._getDoc(id)
		if resp != nil {
			return resp
		}
	}
	return response.NewErrDocResult(id, pkg.ErrPartitionNotExist)
}

func (this *spaceSender) _getDoc(id string) (*response.DocResult, bool) {
	err := this.interceptFrozenFunc()
	if err != nil {
		return response.NewErrDocResult(id, err), false
	}

	resp, err := this.partitionSlot(this.Slot(id)).getDoc(id)
	if err != nil {
		if pkg.ErrCode(err) == pkg.ERRCODE_PARTITION_NOT_EXIST {
			this.ps.client.master.ReloadCacheAsync(this.Ctx.GetContext(), this.db, this.space)
			time.Sleep(1 * time.Second)
			return nil, true
		}
		log.Error("Fail to search ps.  db[%s], space[%s]. err[%v]", this.db, this.space, err)
		return response.NewErrDocResult(id, err), false
	}
	return resp, false
}

//use this function by default slot hash [id , murmur3] ,
// if you want define your slot method ,please use SetRoutingValue to define it
// return entity.DocumentResponse
func (this *spaceSender) GetDocs(ids ...string) response.DocResults {
	resp, retry := this._getDocs(ids)
	if !retry {
		return resp
	}

	for i := 0; i < spaceRetry && retry; i++ {
		resp, retry = this._getDocs(ids)
		if resp != nil {
			return resp
		}
	}
	return response.NewErrDocResults(ids, pkg.ErrPartitionNotExist)
}

func (this *spaceSender) _getDocs(ids []string) (response.DocResults, bool) {
	err := this.interceptFrozenFunc()
	if err != nil {
		return response.NewErrDocResults(ids, err), false
	}

	idMap, err := this.groupPartition(ids)
	if err != nil {
		return response.NewErrDocResults(ids, err), false
	}

	var wg sync.WaitGroup
	respChain := make(chan response.DocResults, len(idMap))
	retry := false

	for pID, idArr := range idMap {
		wg.Add(1)
		go func(paritionID entity.PartitionID, idArr []string) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					respChain <- response.NewErrDocResults(idArr, err)
					log.Error(fmt.Sprint(r))
				}
			}()
			resp, err := this.partitionId(paritionID).getDocs(idArr)

			if err != nil {
				if pkg.ErrCode(err) == pkg.ERRCODE_PARTITION_NOT_EXIST {
					this.ps.client.master.ReloadCacheAsync(this.Ctx.GetContext(), this.db, this.space)
					time.Sleep(1 * time.Second)
					retry = true
				}
				log.Error("Fail to search ps.  db[%s], space[%s]. err[%v]", this.db, this.space, err)
				resp = response.NewErrDocResults(idArr, err)
			} else {
				respChain <- resp
			}
		}(pID, idArr)

	}

	wg.Wait()
	close(respChain)
	if retry {
		return nil, retry
	}

	result := make(response.DocResults, len(ids))
	resultIdMap := make(map[string]int, len(ids))
	for i, id := range ids {
		resultIdMap[id] = i
	}

	for rs := range respChain {
		for _, r := range rs {
			result[resultIdMap[r.Id]] = r
		}
	}

	return result, false
}

//search from space, by partitions
//clientType LEADER or RANDOM
// return entity.SearchResult
func (this *spaceSender) MSearch(req *request.SearchRequest) response.SearchResponses {
	space, err := this.ps.Client().Master().SpaceByCache(this.Ctx.GetContext(), this.db, this.space)
	if err != nil {
		return response.SearchResponses{response.NewSearchResponseErr(err)}
	}

	resp, retry, err := this.MSearchByPartitions(space.Partitions, req)
	if err != nil {
		return response.SearchResponses{response.NewSearchResponseErr(err)}
	}
	if resp != nil {
		return resp
	}

	for i := 0; i < spaceRetry && retry; i++ {
		space, err = this.ps.Client().Master().SpaceByCache(this.Ctx.GetContext(), this.db, this.space)
		if err != nil {
			return response.SearchResponses{response.NewSearchResponseErr(err)}
		}
		resp, retry, err = this.MSearchByPartitions(space.Partitions, req)
		if err != nil {
			return response.SearchResponses{response.NewSearchResponseErr(err)}
		}
		if resp != nil {
			return resp
		}
	}

	return response.SearchResponses{response.NewSearchResponseErr(pkg.ErrPartitionNotExist)}
}

func (this *spaceSender) MSearchByPartitions(partitions []*entity.Partition, req *request.SearchRequest) (resp response.SearchResponses, retry bool, err error) {

	var wg sync.WaitGroup
	respChain := make(chan response.SearchResponses, len(partitions))

	if req.Start == nil { //filter query range
		req.Start = util.PInt64(0)
	}
	if req.End == nil {
		req.End = util.PInt64(math.MaxUint32)
	}

	for _, p := range partitions {

		if *req.Start > p.MaxValue {
			continue
		}

		if *req.End < p.MinValue {
			continue
		}

		wg.Add(1)
		go func(par *entity.Partition) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Error("search has panic :[%s]", r)
					respChain <- response.SearchResponses{newSearchResponseWithError(this.db, this.space, par.Id, fmt.Errorf(cast.ToString(r)))}
				}
			}()
			resp, err := this.partitionId(par.Id).mSearch(req)
			if err != nil {
				if pkg.ErrCode(err) == pkg.ERRCODE_PARTITION_NOT_EXIST {
					this.ps.client.master.ReloadCacheAsync(this.Ctx.GetContext(), this.db, this.space)
					time.Sleep(1 * time.Second)
					retry = true
				}
				log.Error("Fail to search ps. partition[%d], db[%s], space[%s]. err[%v]", par.Id, this.db, this.space, err)
				resp = response.SearchResponses{newSearchResponseWithError(this.db, this.space, p.Id, err)}
			}
			respChain <- resp
		}(p)
	}

	wg.Wait()
	close(respChain)

	if retry {
		return nil, retry, nil
	}

	var result response.SearchResponses
	for r := range respChain {
		if result == nil {
			result = r
			continue
		}
		var err error

		if len(result) < len(r) {
			err = mergeResultArr(r, result, req)
			result = r
		} else {
			err = mergeResultArr(result, r, req)
		}

		if err != nil {
			return nil, false, err
		}
	}

	return result, false, nil
}

//search from space, by partitions
//clientType LEADER or RANDOM
// return entity.SearchResult
func (this *spaceSender) Search(req *request.SearchRequest) *response.SearchResponse {
	space, err := this.ps.Client().Master().SpaceByCache(this.Ctx.GetContext(), this.db, this.space)
	if err != nil {
		return response.NewSearchResponseErr(err)
	}

	resp, retry, err := this.SearchByPartitions(space.Partitions, req)
	if err != nil {
		return response.NewSearchResponseErr(err)
	}
	if resp != nil {
		return resp
	}

	for i := 0; i < spaceRetry && retry; i++ {
		space, err = this.ps.Client().Master().SpaceByCache(this.Ctx.GetContext(), this.db, this.space)
		if err != nil {
			return response.NewSearchResponseErr(err)
		}
		resp, retry, err = this.SearchByPartitions(space.Partitions, req)
		if err != nil {
			return response.NewSearchResponseErr(err)
		}
		if resp != nil {
			return resp
		}
	}

	return response.NewSearchResponseErr(pkg.ErrPartitionNotExist)
}

func (this *spaceSender) SearchByPartitions(partitions []*entity.Partition, req *request.SearchRequest) (resp *response.SearchResponse, retry bool, err error) {

	var wg sync.WaitGroup
	respChain := make(chan *response.SearchResponse, len(partitions))

	if req.Start == nil { //filter query range
		req.Start = util.PInt64(0)
	}
	if req.End == nil {
		req.End = util.PInt64(math.MaxUint32)
	}

	for _, p := range partitions {

		if *req.Start > p.MaxValue {
			continue
		}

		if *req.End < p.MinValue {
			continue
		}

		wg.Add(1)
		go func(par *entity.Partition) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Error("search has panic :[%s]", r)
					respChain <- newSearchResponseWithError(this.db, this.space, par.Id, fmt.Errorf(cast.ToString(r)))
				}
			}()
			resp, err := this.partitionId(par.Id).search(req)
			if err != nil {
				if pkg.ErrCode(err) == pkg.ERRCODE_PARTITION_NOT_EXIST {
					this.ps.client.master.ReloadCacheAsync(this.Ctx.GetContext(), this.db, this.space)
					time.Sleep(1 * time.Second)
					retry = true
				}
				log.Error("Fail to search ps. partition[%d], db[%s], space[%s]. err[%v]", par.Id, this.db, this.space, err)
				resp = newSearchResponseWithError(this.db, this.space, p.Id, err)
			}
			respChain <- resp
		}(p)
	}

	wg.Wait()
	close(respChain)

	if retry {
		return nil, retry, nil
	}

	sortOrder, err := req.SortOrder()

	if err != nil {
		return nil, false, err
	}

	var maxTook int64
	var maxPID uint32
	var first *response.SearchResponse
	for r := range respChain {
		if r.Took > maxTook {
			maxTook = r.Took
			maxPID = r.PID
		}
		if first == nil {
			first = r
			continue
		}
		err := first.Merge(r, sortOrder, req.From, *req.Size)
		if err != nil {
			return nil, false, err
		}
	}

	log.Debug("Max search partitionID:[%d] use time:[%d]", maxPID, maxTook/1000000)

	return first, false, nil
}

func (this *spaceSender) StreamSearch(req *request.SearchRequest) *response.DocStreamResult {
	dsr := response.NewDocStreamResult(this.Ctx.GetContext())

	space, err := this.ps.Client().Master().SpaceByCache(this.Ctx.GetContext(), this.db, this.space)
	if err != nil {
		dsr.AddErr(err)
		return dsr
	}

	if req.Start == nil { //filter query range
		req.Start = util.PInt64(0)
	}
	if req.End == nil {
		req.End = util.PInt64(math.MaxUint32)
	}

	go func() {
		defer func() {
			dsr.AddDoc(nil)
		}()

		for _, p := range space.Partitions {

			if *req.Start > p.MaxValue {
				continue
			}

			if *req.End < p.MinValue {
				continue
			}
			this.partitionId(p.Id).streamSearch(req, dsr)
		}
	}()

	return dsr
}

// Flush space, all partiitons
func (this *spaceSender) Flush() (*response.Shards, error) {
	space, err := this.ps.client.master.SpaceByCache(this.Ctx.GetContext(), this.db, this.space)
	if err != nil {
		return nil, err
	}
	resp, retry, err := this.FlushByPartitions(space.Partitions)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		return resp, nil
	}

	for i := 0; i < spaceRetry && retry; i++ {
		space, err := this.ps.client.master.SpaceByCache(this.Ctx.GetContext(), this.db, this.space)
		if err != nil {
			return nil, err
		}
		resp, retry, err = this.FlushByPartitions(space.Partitions)
		if err != nil {
			return nil, err
		}
		if resp != nil {
			return resp, nil
		}
	}

	return nil, err
}

func (this *spaceSender) FlushByPartitions(partitions []*entity.Partition) (resp *response.Shards, retry bool, err error) {

	var wg sync.WaitGroup
	respChain := make(chan bool, len(partitions))

	for _, p := range partitions {
		wg.Add(1)
		go func(par *entity.Partition) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Error(string(debug.Stack()))
					log.Error(cast.ToString(r))
					respChain <- false
				}
			}()
			err := this.partitionId(par.Id).flush(LEADER)
			if err != nil {
				if pkg.ErrCode(err) == pkg.ERRCODE_PARTITION_NOT_EXIST {
					this.ps.client.master.ReloadCacheAsync(this.Ctx.GetContext(), this.db, this.space)
					time.Sleep(1 * time.Second)
					retry = true
				}
				log.Error("Fail to flush. partition[%d], db[%s], space[%s], partitionId:[%d]. err[%v]", par.Id, this.db, this.space, par.Id, err)
			}
			respChain <- true
		}(p)
	}

	wg.Wait()
	close(respChain)

	resp = new(response.Shards)
	resp.Total = len(partitions)

	for res := range respChain {
		if res {
			resp.Successful++
		} else {
			resp.Failed++
		}
	}

	if retry {
		return nil, retry, nil
	}
	return resp, false, nil
}

// ForceMerge space, all partiitons
func (this *spaceSender) ForceMerge() (*response.Shards, error) {
	space, err := this.ps.client.master.SpaceByCache(this.Ctx.GetContext(), this.db, this.space)
	if err != nil {
		return nil, err
	}
	resp, retry, err := this.ForceMergeByPartitions(space.Partitions)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		return resp, nil
	}

	for i := 0; i < spaceRetry && retry; i++ {
		space, perr := this.ps.client.master.SpaceByCache(this.Ctx.GetContext(), this.db, this.space)
		if perr != nil {
			return nil, err
		}
		resp, retry, err = this.ForceMergeByPartitions(space.Partitions)
		if err != nil {
			return nil, err
		}
		if resp != nil {
			return resp, nil
		}
	}

	return nil, err
}

func (this *spaceSender) ForceMergeByPartitions(partitions []*entity.Partition) (resp *response.Shards, retry bool, err error) {

	var wg sync.WaitGroup
	respChain := make(chan bool, len(partitions))

	for _, p := range partitions {
		wg.Add(1)
		go func(par *entity.Partition) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Error(string(debug.Stack()))
					log.Error(cast.ToString(r))
					respChain <- false
				}
			}()
			err := this.partitionId(par.Id).forceMerge(ALL)
			if err != nil {
				if pkg.ErrCode(err) == pkg.ERRCODE_PARTITION_NOT_EXIST {
					this.ps.client.master.ReloadCacheAsync(this.Ctx.GetContext(), this.db, this.space)
					time.Sleep(1 * time.Second)
					retry = true
				}
				log.Error("Fail to forceMerge. partition[%d], db[%s], space[%s], partitionId:[%d]. err[%v]", par.Id, this.db, this.space, par.Id, err)
			}
			respChain <- true
		}(p)
	}

	wg.Wait()
	close(respChain)

	resp = new(response.Shards)
	resp.Total = len(partitions)

	for res := range respChain {
		if res {
			resp.Successful++
		} else {
			resp.Failed++
		}
	}

	if retry {
		return nil, retry, nil
	}
	return resp, false, nil
}

//batch handler , if you use it you must make sure it is docs type is right by slot
func (this *spaceSender) Batch(docs []*pspb.DocCmd) (response.WriteResponse, error) {
	for _, doc := range docs {
		if doc.Type == pspb.OpType_MERGE || (doc.Type == pspb.OpType_REPLACE && doc.Version != -1) || doc.Type == pspb.OpType_DELETE {
			err := this.interceptFrozenFunc()
			if err != nil {
				return nil, err
			}
		}
	}
	docMap, err := this.groupPartitionByDocs(docs)
	if err != nil {
		return nil, err
	}

	var writeResponse response.WriteResponse
	for pid, gDocs := range docMap {
		wrs, err := this.partitionId(pid).batch(gDocs)
		if err != nil {
			for _, gdoc := range gDocs {
				docResult := response.NewErrDocResult(gdoc.DocId, err)
				writeResponse = append(writeResponse, response.WriteResponse{docResult}...)
			}
		} else {
			writeResponse = append(writeResponse, *wrs...)
		}
	}
	return writeResponse, err
}

func (this *spaceSender) Write(doc *pspb.DocCmd) (*response.DocResult, error) {
	if doc.Type == pspb.OpType_MERGE || (doc.Type == pspb.OpType_REPLACE && doc.Version != -1) || doc.Type == pspb.OpType_DELETE {
		err := this.interceptFrozenFunc()
		if err != nil {
			return nil, err
		}
	}
	sender := this.partitionSlot(doc.Slot)
	resp, err := sender.write(doc)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

//default version is 1
func (this *spaceSender) CreateDoc(id string, source []byte) *response.DocResult {
	return this.writeAndRetry(pspb.NewDocCreateWithSlot(id, this.Slot(id), source))
}

// if version is zero  update version++
func (this *spaceSender) MergeDoc(id string, source []byte, version int64) *response.DocResult {
	err := this.interceptFrozenFunc()
	if err != nil {
		return response.NewErrDocResult(id, err)
	}
	return this.writeAndRetry(pspb.NewDocMergeWithSlot(id, this.Slot(id), source, version))
}

// relace the doc version will set 1
func (this *spaceSender) ReplaceDoc(id string, source []byte) *response.DocResult {
	return this.writeAndRetry(pspb.NewDocReplaceWithSlot(id, this.Slot(id), source, -1))
}

// if version == -1 same as replace, over write and not check version ,new version is 1 , if version == 0 , update version++
func (this *spaceSender) UpdateDoc(id string, source []byte, version int64) *response.DocResult {
	if version != -1 {
		err := this.interceptFrozenFunc()
		if err != nil {
			return response.NewErrDocResult(id, err)
		}
	}
	return this.writeAndRetry(pspb.NewDocReplaceWithSlot(id, this.Slot(id), source, version))
}

//delete doc without version check
func (this *spaceSender) DeleteDoc(id string) *response.DocResult {
	err := this.interceptFrozenFunc()
	if err != nil {
		return response.NewErrDocResult(id, err)
	}
	return this.writeAndRetry(pspb.NewDocDeleteWithSlot(id, this.Slot(id), 0))
}

// if version == 0 , delete not check version
func (this *spaceSender) DeleteDocWithVersion(id string, version int64) *response.DocResult {
	err := this.interceptFrozenFunc()
	if err != nil {
		return response.NewErrDocResult(id, err)
	}
	return this.writeAndRetry(pspb.NewDocDeleteWithSlot(id, this.Slot(id), version))
}

func (this *spaceSender) Bulk(doc *pspb.DocCmd) *response.DocResult {
	return this.writeAndRetry(doc)
}

func (this *spaceSender) writeAndRetry(doc *pspb.DocCmd) *response.DocResult {
	sender := this.partitionSlot(doc.Slot)

	tryTimes := 0
	for {
		resp, err := sender.write(doc)
		if err == nil {
			return resp
		}

		tryTimes++
		// if for cycle, break force
		if tryTimes > 100 {
			return response.NewErrDocResult(doc.DocId, err)
		}

		log.Error("client ps write doc error: %s", err.Error())
		if pkg.ErrCode(err) == pkg.ERRCODE_PARTITION_NOT_EXIST {
			if tryTimes > 5 {
				return response.NewErrDocResult(doc.DocId, err)
			}
			this.ps.client.master.ReloadCacheAsync(this.Ctx.GetContext(), this.db, this.space)
			time.Sleep(1 * time.Second)
		} else if pkg.ErrCode(err) == pkg.ERRCODE_PARTITION_FROZEN {
			sender.pid = 0
			if tryTimes > 5 {
				return response.NewErrDocResult(doc.DocId, err)
			}
			this.ps.client.master.ReloadCacheAsync(this.Ctx.GetContext(), this.db, this.space)
			time.Sleep(1 * time.Second)
		} else if pkg.ErrCode(err) == pkg.ERRCODE_PULL_OUT_VERSION_NOT_MATCH {
			if this.writeTryTimes == 0 {
				return response.NewErrDocResult(doc.DocId, err)
			}

			if tryTimes > this.writeTryTimes {
				return response.NewErrDocResult(doc.DocId, err)
			}
		} else {
			return response.NewErrDocResult(doc.DocId, err)
		}

	}
}

func (this *spaceSender) groupPartition(ids []string) (map[entity.PartitionID][]string, error) {
	space, err := this.ps.client.master.SpaceByCache(this.Ctx.GetContext(), this.db, this.space)
	if err != nil {
		return nil, err
	}
	result := make(map[entity.PartitionID][]string)

	if this.routingValue != "" {
		slot := entity.SlotID(murmur3.Sum32WithSeed([]byte(this.routingValue), 0))
		result[space.PartitionId(slot)] = ids
		return result, nil
	}

	for _, id := range ids {
		partitionId := space.PartitionId(entity.SlotID(murmur3.Sum32WithSeed([]byte(id), 0)))
		result[partitionId] = append(result[partitionId], id)
	}
	return result, nil
}

func doc2Ids(docs []*pspb.DocCmd) []string {
	ids := make([]string, len(docs))
	for i := 0; i < len(ids); i++ {
		ids[i] = docs[i].DocId
	}
	return ids
}

func (this *spaceSender) groupPartitionByDocs(docs []*pspb.DocCmd) (map[entity.PartitionID][]*pspb.DocCmd, error) {
	space, err := this.ps.client.master.SpaceByCache(this.Ctx.GetContext(), this.db, this.space)
	if err != nil {
		return nil, err
	}
	result := make(map[entity.PartitionID][]*pspb.DocCmd)

	for _, doc := range docs {
		partitionId := space.PartitionId(doc.Slot)
		result[partitionId] = append(result[partitionId], doc)
	}
	return result, nil
}

func (this *spaceSender) Slot(docID string) uint32 {
	if this.routingValue != "" {
		return Slot(this.routingValue)
	}
	return Slot(docID)
}

func Slot(routingValue string) uint32 {
	return murmur3.Sum32WithSeed(bytes.StringToByte(routingValue), 0)
}

func (this *spaceSender) interceptFrozenFunc() error {
	space, err := this.ps.client.master.SpaceByCache(this.Ctx.GetContext(), this.db, this.space)
	if err != nil {
		return err
	}
	if space.CanFrozen() {
		return pkg.ErrFuncCanNotInvokeInFrozenEngine
	}
	return nil
}
