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
	"context"
	"fmt"
	"github.com/patrickmn/go-cache"
	"github.com/tiglabs/baudengine/proto"
	"github.com/tiglabs/baudengine/util/baudlog"
	"github.com/tiglabs/baudengine/util/cbjson"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cast"
	. "github.com/tiglabs/baudengine/proto/entity"
	"github.com/tiglabs/baudengine/util/atomic"
	"github.com/tiglabs/log"
	"go.etcd.io/etcd/mvcc/mvccpb"
	"runtime/debug"
	"time"
)

var userCache = cache.New(cache.NoExpiration, cache.NoExpiration)
var spaceCache = cache.New(cache.NoExpiration, cache.NoExpiration)
var spaceCacheLock sync.Mutex
var spaceIDCache = cache.New(cache.NoExpiration, cache.NoExpiration)
var partitionCache = cache.New(cache.NoExpiration, cache.NoExpiration)
var serverCache = cache.New(cache.NoExpiration, cache.NoExpiration)

var psClientCache = &clientCache{}

type spaceEntry struct {
	lastUpdateTime time.Time
	mutex          sync.Mutex
	refCount       *atomic.AtomicInt64
}

var spaceEntryMap = make(map[string]*spaceEntry)
var spaceEntryMapLock sync.Mutex

func cachePartitionKey(space string, pid PartitionID) string {
	return space + "/" + strconv.FormatInt(int64(pid), 10)
}

func cacheSpaceKey(db, space string) string {
	return db + "/" + space
}

func cacheServerKey(nodeId NodeID) string {
	return cast.ToString(nodeId)
}

//find a user by cache
func (this *masterClient) UserByCache(ctx context.Context, userName string) (*User, error) {

	get, found := userCache.Get(userName)
	if found {
		return get.(*User), nil
	}

	_ = this.reloadUserCache(ctx, false, userName)

	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		if get, found = spaceCache.Get(userName); found {
			return get.(*User), nil
		}
	}

	return nil, fmt.Errorf("user:[%s] err:[%s]", userName, pkg.ErrPartitionNotExist)
}

var userReloadWorkder sync.Map

func (this *masterClient) reloadUserCache(ctx context.Context, sync bool, userName string) error {
	_, ok := userReloadWorkder.LoadOrStore(userName, struct{}{})
	if ok {
		return nil
	}

	fun := func() error {
		defer userReloadWorkder.Delete(userName)

		log.Info("to reload user:[%s]", userName)

		fmt.Println(ctx)
		ctx, _ = context.WithTimeout(ctx, 10*time.Second)

		user, err := this.QueryUser(ctx, userName)
		if err != nil {
			return fmt.Errorf("can not found user by name:[%s] err:[%s]", userName, err.Error())
		}
		userCache.Set(userName, user, cache.NoExpiration)
		return nil
	}

	if sync {
		return fun()
	} else {
		go baudlog.FunIfNotNil(fun)
	}

	return nil
}

func (this *masterClient) SpaceByCacheWithOutRetry(ctx context.Context, db, space string) (*Space, bool) {
	key := cacheSpaceKey(db, space)

	get, found := spaceCache.Get(key)
	if found {
		return get.(*Space), found
	}
	return nil, false
}

//find a space by db and space name , if not exist so query it from db
func (this *masterClient) SpaceByCache(ctx context.Context, db, space string) (*Space, error) {

	key := cacheSpaceKey(db, space)

	get, found := spaceCache.Get(key)
	if found {
		return get.(*Space), nil
	}

	_ = this.reloadSpaceCache(ctx, false, db, space)

	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		if get, found = spaceCache.Get(key); found {
			return get.(*Space), nil
		}
	}

	return nil, fmt.Errorf("db:[%s] space:[%s] err:[%s]", db, space, pkg.ErrPartitionNotExist)
}

var spaceReloadWorkder sync.Map

func (this *masterClient) reloadSpaceCache(ctx context.Context, sync bool, db string, space string) error {
	key := cacheSpaceKey(db, space)
	_, ok := spaceReloadWorkder.LoadOrStore(key, struct{}{})
	if ok {
		return nil
	}

	fun := func() error {

		log.Info("to reload db:[%s] space:[%s]", db, space)

		ctx, _ = context.WithTimeout(ctx, 10*time.Second)
		dbID, err := this.QueryDBName2Id(ctx, db)
		if err != nil {
			return fmt.Errorf("can not found db by name:[%s] err:[%s]", db, err.Error())
		}

		space, err := this.QuerySpaceByName(ctx, dbID, space)
		if err != nil {
			return fmt.Errorf("can not found db by name:[%s] err:[%s]", db, err.Error())
		}
		spaceCacheLock.Lock()
		defer spaceCacheLock.Unlock()
		spaceCache.Set(key, space, cache.NoExpiration)
		spaceIDCache.Set(cast.ToString(space.Id), space, cache.NoExpiration)
		return nil
	}

	if sync {
		return fun()
	} else {
		go baudlog.FunIfNotNil(fun)
	}

	return nil
}

//partition/[spaceId]/[id]:[body]
func (this *masterClient) PartitionByCache(ctx context.Context, spaceName string, pid PartitionID) (*Partition, error) {
	key := cachePartitionKey(spaceName, pid)
	get, found := partitionCache.Get(key)
	if found {
		return get.(*Partition), nil
	}

	_ = this.reloadPartitionCache(ctx, false, spaceName, pid)

	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		if get, found = partitionCache.Get(key); found {
			return get.(*Partition), nil
		}
	}

	return nil, fmt.Errorf("space:[%s] partition_id:[%d] err:[%s]", spaceName, pid, pkg.ErrPartitionNotExist)
}

var partitionReloadWorkder sync.Map

func (this *masterClient) reloadPartitionCache(ctx context.Context, sync bool, spaceName string, pid PartitionID) error {
	key := cachePartitionKey(spaceName, pid)
	_, ok := partitionReloadWorkder.LoadOrStore(key, struct{}{})
	if ok {
		return nil
	}

	fun := func() error {
		defer partitionCache.Delete(key)

		log.Info("to reload space:[%s] partition_id:[%d] ", spaceName, pid)

		ctx, _ = context.WithTimeout(ctx, 10*time.Second)

		partition, err := this.QueryPartition(ctx, pid)
		if err != nil {
			return fmt.Errorf("can not found db by space:[%s] partition_id:[%d] err:[%s]", spaceName, pid, err.Error())
		}

		partitionCache.Set(key, partition, cache.NoExpiration)

		return nil
	}

	if sync {
		return fun()
	} else {
		go baudlog.FunIfNotNil(fun)
	}

	return nil
}

func (this *masterClient) ServerByCache(ctx context.Context, id NodeID) (*Server, error) {
	key := cast.ToString(id)
	get, found := serverCache.Get(key)
	if found {
		return get.(*Server), nil
	}

	_ = this.reloadServerCache(ctx, false, id)

	for i := 0; i < 3; i++ {
		time.Sleep(time.Microsecond * 200)
		if get, found = serverCache.Get(key); found {
			return get.(*Server), nil
		}
	}

	return nil, fmt.Errorf("node_id:[%d] err:[%s]", id, pkg.ErrPartitionNotExist)
}

var serverReloadWorkder sync.Map

func (this *masterClient) reloadServerCache(ctx context.Context, sync bool, id NodeID) interface{} {
	key := cast.ToString(id)
	_, ok := serverReloadWorkder.LoadOrStore(key, struct{}{})
	if ok {
		return nil
	}

	fun := func() error {
		defer serverCache.Delete(key)

		log.Info("to reload server:[%d] ", id)

		ctx, _ = context.WithTimeout(ctx, 10*time.Second)
		server, err := this.QueryServer(ctx, id)
		if err != nil {
			return fmt.Errorf("can not found server node_id:[%d] err:[%s]", id, err.Error())
		}

		serverCache.Set(key, server, cache.NoExpiration)

		return nil
	}

	if sync {
		return fun()
	} else {
		go baudlog.FunIfNotNil(fun)
	}

	return nil
}

// to async reload cache by space
func (this *masterClient) ReloadCacheAsync(ctx context.Context, dbName, spaceName string) {
	log.Debug("ReloadCacheAsync() invoke, db: %s, space: %s", dbName, spaceName)
	spaceNameKey := cacheSpaceKey(dbName, spaceName)
	spaceEntryMapLock.Lock()
	entry, ok := spaceEntryMap[spaceNameKey]
	if !ok {
		entry = &spaceEntry{refCount: atomic.NewAtomicInt64(0)}
		spaceEntryMap[spaceNameKey] = entry
	}
	spaceEntryMapLock.Unlock()

	entry.mutex.Lock()

	if time.Since(entry.lastUpdateTime) < 5*time.Second {
		log.Debug("ReloadCacheAsync() invoke, space entry just update in 5 second, not need to update, db: %s, space: %s", dbName, spaceName)
		entry.mutex.Unlock()
		return
	}
	if entry.refCount.Get() > 0 {
		log.Debug("ReloadCacheAsync() invoke, space entry has another goroutine access, do nothing, db: %s, space: %s", dbName, spaceName)
		entry.mutex.Unlock()
		time.Sleep(1 * time.Second)
		return
	}

	go func() {
		entry.refCount.Incr()
		entry.mutex.Unlock()

		defer func() {
			if rErr := recover(); rErr != nil {
				log.Error("recover() err:[%v]", rErr)
				log.Error("stack:[%s]", debug.Stack())
			}
		}()
		defer entry.refCount.Decr()

		// space
		dbid, err := this.QueryDBName2Id(ctx, dbName)
		if err != nil {
			log.Error("ReloadCacheAsync QueryDBName2Id err: %s", err.Error())
			return
		}
		spaceMeta, err := this.QuerySpaceByName(ctx, dbid, spaceName)
		if err != nil {
			log.Error("ReloadCacheAsync QuerySpaceByName err: %s", err.Error())
			return
		}
		spaceCacheLock.Lock()
		spaceCache.Set(spaceNameKey, spaceMeta, cache.NoExpiration)
		spaceIDCache.Set(cast.ToString(spaceMeta.Id), spaceMeta, cache.NoExpiration)
		spaceCacheLock.Unlock()

		// partition
		_, values, err := this.Store.PrefixScan(ctx, PrefixPartition)
		if err != nil {
			log.Error("ReloadCacheAsync get partition err , err:[%s]", err.Error())
			return
		}
		partitionKeyPrefix := spaceName + "/"
		for s := range partitionCache.Items() {
			if strings.HasPrefix(s, partitionKeyPrefix) {
				partitionCache.Delete(s)
			}
		}
		for _, bs := range values {
			pt := &Partition{}
			err := cbjson.Unmarshal(bs, pt)
			if err != nil {
				log.Error("ReloadCacheAsync json unmarshal partition err , err:[%s]", err.Error())
				return
			}
			if pt.SpaceId != spaceMeta.Id {
				continue
			}

			partitionKey := cachePartitionKey(spaceMeta.Name, pt.Id)
			partitionCache.Set(partitionKey, pt, cache.NoExpiration)
		}
		entry.lastUpdateTime = time.Now()
		log.Debug("ReloadCacheAsync() invoke, space entry update success, db: %s, space: %s", dbName, spaceName)
	}()

}

var once sync.Once

//it will start cache
func (this *masterClient) StartCacheJob(ctx context.Context) (cacheErr error) {
	once.Do(func() {
		cacheErr = this._startCacheJob(ctx)
	})
	return
}

func (this *masterClient) _startCacheJob(ctx context.Context) error {
	log.Info("to start cache job begin")
	start := time.Now()

	//init user
	if err := this.initUser(ctx); err != nil {
		return err
	}
	userJob := watcherJob{ctx: ctx, prefix: PrefixUser, masterClient: this, cache: userCache,
		put: func(value []byte) (err error) {
			user := &User{}
			if err := cbjson.Unmarshal(value, user); err != nil {
				return fmt.Errorf("put event user cache err, can't unmarshal event value: %s , error: %s", string(value), err.Error())
			}
			userCache.Set(UserKey(user.Name), user, cache.NoExpiration)
			return nil
		},
		delete: func(key string) (err error) {
			userSplit := strings.Split(key, "/")
			if len(userSplit) != 3 {
				log.Error("user delete event got err key")
			}
			username := userSplit[2]
			userCache.Delete(username)
			return nil
		},
	}

	userJob.start()

	//init space
	if err := this.initSpace(ctx); err != nil {
		return err
	}
	spaceJob := watcherJob{ctx: ctx, prefix: PrefixSpace, masterClient: this, cache: spaceCache,
		put: func(value []byte) (err error) {
			space := &Space{}
			if err := cbjson.Unmarshal(value, space); err != nil {
				return err
			}
			dbName, err := this.QueryDBId2Name(ctx, space.DBId)
			if err != nil {
				return fmt.Errorf("change cache space err: %s , not found db content: %s", err.Error(), string(value))
			}
			key := cacheSpaceKey(dbName, space.Name)
			if oldValue, b := spaceCache.Get(key); !b || space.Version > oldValue.(*Space).Version {
				spaceCacheLock.Lock()
				spaceCache.Set(key, space, cache.NoExpiration)
				spaceIDCache.Set(cast.ToString(space.Id), space, cache.NoExpiration)
				spaceCacheLock.Unlock()
			}
			return nil
		},
		delete: func(key string) (err error) {
			dbIdStr := strings.Split(key, "/")[2]
			dbId := cast.ToInt64(dbIdStr)
			spaceIdStr := strings.Split(key, "/")[3]
			spaceId := cast.ToInt64(spaceIdStr)
			for k, v := range spaceCache.Items() {
				if v.Object.(*Space).DBId == dbId && v.Object.(*Space).Id == spaceId {
					log.Info("remove space cache dbID:[%d] space:[%d] ", dbId, spaceId)
					spaceCacheLock.Lock()
					spaceCache.Delete(k)
					spaceIDCache.Delete(cast.ToString(spaceId))
					spaceCacheLock.Unlock()
					break
				}
			}
			return nil
		},
	}
	spaceJob.start()

	//init partition
	if err := this.initPartition(ctx); err != nil {
		return err
	}
	partitionJob := watcherJob{ctx: ctx, prefix: PrefixPartition, masterClient: this, cache: partitionCache,
		put: func(value []byte) (err error) {
			partition := &Partition{}
			if err = cbjson.Unmarshal(value, partition); err != nil {
				return
			}
			space, err := this.QuerySpaceById(ctx, partition.DBId, partition.SpaceId)
			if err != nil {
				return
			}
			cacheKey := cachePartitionKey(space.Name, partition.Id)
			if old, b := partitionCache.Get(cacheKey); !b || partition.UpdateTime > old.(*Partition).UpdateTime {
				partitionCache.Set(cacheKey, partition, cache.NoExpiration)
			}
			return nil
		},
		delete: func(key string) (err error) {
			partitionIdStr := strings.Split(key, "/")[2]
			for k := range partitionCache.Items() {
				if strings.HasSuffix(k, "/"+partitionIdStr) {
					partitionCache.Delete(k)
					break
				}
			}
			return nil
		},
	}
	partitionJob.start()

	//init server
	if err := this.initServer(ctx); err != nil {
		return err
	}
	serverJob := watcherJob{ctx: ctx, prefix: PrefixServer, masterClient: this, cache: serverCache,
		put: func(value []byte) (err error) {
			server := &Server{}
			if err := cbjson.Unmarshal(value, server); err != nil {
				return err
			}
			if value, ok := psClientCache.Load(server.ID); ok {
				if value != nil && value.(*rpcClient).client.GetAddress(0) != server.RpcAddr() {
					value.(*rpcClient).close()
					psClientCache.Delete(server.ID)
				}
			}
			serverCache.Set(cacheServerKey(server.ID), server, cache.NoExpiration)
			return nil
		},
		delete: func(cacheKey string) (err error) {
			nodeIdStr := strings.Split(cacheKey, "/")[2]
			nodeId := cast.ToUint64(nodeIdStr)
			if value, _ := psClientCache.Load(nodeId); value != nil {
				value.(*rpcClient).close()
				psClientCache.Delete(nodeId)
			}
			serverCache.Delete(nodeIdStr)
			return nil
		},
	}
	serverJob.start()

	log.Info("cache inited ok use time %v", time.Now().Sub(start))

	return nil
}

func (this *masterClient) initUser(ctx context.Context) error {
	_, users, err := this.PrefixScan(ctx, PrefixUser)
	if err != nil {
		log.Error("init user cache err: %s", err.Error())
		return err
	}
	for _, v := range users {
		user := &User{}
		err := cbjson.Unmarshal(v, user)
		if err != nil {
			log.Error("init user cache err: %s", err.Error())
			return err
		}
		if err := userCache.Add(user.Name, user, cache.NoExpiration); err != nil {
			log.Error(err.Error())
			return err
		}
	}

	return nil
}

func (this *masterClient) initSpace(ctx context.Context) error {
	spaces, err := this.QuerySpacesByKey(ctx, PrefixSpace)
	if err != nil {
		return err
	}
	for _, s := range spaces {
		db, err := this.QueryDBId2Name(ctx, s.DBId)
		if err != nil {
			log.Error("init spaces cache dbid to id err , err:[%s]", err.Error())
			continue
		}

		spaceCacheLock.Lock()
		if err := spaceCache.Add(cacheSpaceKey(db, s.Name), s, cache.NoExpiration); err != nil {
			log.Error(err.Error())
		} else {
			spaceIDCache.Set(cast.ToString(s.Id), s, cache.NoExpiration)
		}
		spaceCacheLock.Unlock()
	}
	return nil
}

func (this *masterClient) initPartition(ctx context.Context) error {
	_, values, err := this.PrefixScan(ctx, PrefixPartition)
	if err != nil {
		log.Error("init partition cache err , err:[%s]", err.Error())
		return err
	}
	spaceNameMap := make(map[SpaceID]string)

	for _, bs := range values {
		pt := &Partition{}
		err := cbjson.Unmarshal(bs, pt)
		if err != nil {
			log.Error("init partition cache err , err:[%s]", err.Error())
			continue
		}
		spaceName := spaceNameMap[pt.SpaceId]
		if spaceName == "" {
			space, err := this.QuerySpaceById(ctx, pt.DBId, pt.SpaceId)
			if err != nil {
				log.Error("partition can not find space by DBID:[%d] spaceID:[%pt.SpaceId] partitionID:[%d] err:[%s]", pt.DBId, pt.SpaceId, pt.Id, err.Error())
				continue
			}
			spaceName, spaceNameMap[pt.SpaceId] = space.Name, space.Name
		}
		key := cachePartitionKey(spaceName, pt.Id)
		if err := partitionCache.Add(key, pt, cache.NoExpiration); err != nil {
			log.Error(err.Error())
		}
	}

	return nil
}

func (this *masterClient) initServer(ctx context.Context) error {
	_, values, err := this.PrefixScan(ctx, PrefixServer)
	if err != nil {
		log.Error("init server cache err , err:[%s]", err.Error())
		return err
	}
	for _, bs := range values {
		server := &Server{}
		err := cbjson.Unmarshal(bs, server)
		if err != nil {
			log.Error("unmarshal server cache err , err:[%s]", err.Error())
			continue
		}
		if err := serverCache.Add(cast.ToString(server.ID), server, cache.NoExpiration); err != nil {
			log.Error(err.Error())
		}
	}
	return nil
}

type watcherJob struct {
	ctx          context.Context
	prefix       string
	masterClient *masterClient
	wg           sync.WaitGroup
	cache        *cache.Cache
	put          func(value []byte) (err error)
	delete       func(key string) (err error)
}

func Init() error {
	return nil
}

func (wj *watcherJob) start() {
	go func() {
		defer func() {
			if rErr := recover(); rErr != nil {
				log.Error("recover() err:[%v]", rErr)
				log.Error("stack:[%s]", debug.Stack())
			}
		}()
		for {
			wj.wg.Add(1)
			go func() {
				defer func() {
					if rErr := recover(); rErr != nil {
						log.Error("recover() err:[%v]", rErr)
						log.Error("stack:[%s]", debug.Stack())
					}
				}()
				defer wj.wg.Done()

				watcher, err := wj.masterClient.WatchPrefix(wj.ctx, wj.prefix)
				if err != nil {
					log.Error("watch prefix:[%s] err", wj.prefix)
					time.Sleep(1 * time.Second)
					return
				}

				for reps := range watcher {
					if reps.Canceled {
						log.Error("chan is closed by server watcher job")
						return
					}

					for _, event := range reps.Events {
						switch event.Type {
						case mvccpb.PUT:
							err := wj.put(event.Kv.Value)
							if err != nil {
								log.Error("change cache %s, err: %s , content: %s", wj.prefix, err.Error(), string(event.Kv.Value))
							}

						case mvccpb.DELETE:
							err := wj.delete(string(event.Kv.Key))
							if err != nil {
								log.Error("delete cache %s, err: %s , content: %s", wj.prefix, err.Error(), string(event.Kv.Value))
							}
						}
					}
				}
			}()
			wj.wg.Wait()
		}
	}()

}
