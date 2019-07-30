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

package adminapi

import (
	"fmt"
	"github.com/spf13/cast"
	"github.com/tiglabs/baudengine/proto/entity"
	"github.com/tiglabs/baudengine/test/testutil"
	"github.com/tiglabs/baudengine/util"
	"github.com/tiglabs/baudengine/util/assert"
	"github.com/tiglabs/baudengine/util/cbjson"
	"github.com/tiglabs/log"
	"strings"
	"sync"
	"testing"
)

func TestCreateAuthCode(t *testing.T) {
	code := util.AuthEncrypt("logbookuser", "logbookpasswd")
	fmt.Println(code)
}

func TestDB(t *testing.T) {
	idx := 0

	dbName := "TestDB"

	client := testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, "TestDB", "TestDB")

	if _, err := client.DbDrop(dbName); err != nil {
		log.Error(err.Error())
	}

	for {

		response, err := client.DbCreate(&entity.DB{Name: dbName})
		if err != nil {
			t.Errorf("create db failed, err: %v\n", err)
			break
		}

		resp := response.Resp
		fmt.Println("create db result: ", string(resp))
		var rpl testutil.StringMap
		if err = cbjson.Unmarshal(resp, &rpl); err != nil {
			t.Errorf("failed to unmarshal db create result, err: %v", err)
		}

		db, err := client.DBGet(dbName)
		if err != nil {
			t.Errorf("get db failed, res: %s, err: %v\n", string(resp), err)
			break
		} else if db == nil {
			t.Fatal(fmt.Errorf("db not found"))
		} else if db.Name != dbName {
			t.Fatal(fmt.Errorf("db name not same"))
		}

		assert.Equal(t, db.Name, dbName, "db name unmatch")

		if _, err := client.DbDelete(dbName); err != nil {
			t.Fatal(err)
		}

		if idx > 10 {
			break
		}
		idx++
	}
}

func TestSpace(t *testing.T) {

	dbName := "TestDB"
	spaceName := "TestSpace"

	client := testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, dbName, spaceName)

	if _, err := client.DbDrop(dbName); err != nil {
		log.Error(err.Error())
	}

	if _, err := client.DbCreate(&entity.DB{Name: dbName}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.DBGet(dbName); err != nil {
		t.Fatal(err)
	}

	idx := 0
	for {

		response, err := client.SpaceCreate(dbName, &entity.Space{
			Name:         spaceName,
			PartitionNum: (idx%10 + 1),
		})
		if err != nil {
			t.Errorf("create space failed, err: %v\n", err)
			break
		}
		fmt.Println("space create result: ", string(response.Resp))

		space, err := client.SpaceGet(dbName,spaceName)
		if err != nil {
			t.Fatal(err)
		}

		if space == nil || space.Name != spaceName {
			t.Fatal("space get err ")
		}

		_, err = client.DbDelete(dbName)
		if !strings.Contains(err.Error(), "db not empty") {
			t.Fatal(err)
		}

		spaceDelete, err := client.SpaceDelete(dbName, spaceName)
		if err != nil {
			t.Fatal(err)
		}

		log.Info("delete space ok : %s", spaceDelete.Resp)

		if idx > 10 {
			break
		}
		idx++
	}
}


func TestCreateSpace(t *testing.T) {

	dbName := "TestDB"
	spaceName := "TestSpace"

	client := testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, dbName, spaceName)

	if _, err := client.DbDrop(dbName); err != nil {
		log.Error(err.Error())
	}

	if _, err := client.DbCreate(&entity.DB{Name: dbName}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.DBGet(dbName); err != nil {
		t.Fatal(err)
	}

	response, err := client.SpaceCreate(dbName, &entity.Space{
		Name:         spaceName,
		PartitionNum: 3,
	})
	if err != nil {
		t.Errorf("create space failed, err: %v\n", err)
	}
	fmt.Println("space create result: ", string(response.Resp))
}

func TestSpaceSameDB(t *testing.T) {
	dbName := "TestDB"
	spaceName := "TestSpace"

	client := testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, dbName, spaceName)

	if _, err := client.DbDrop(dbName); err != nil {
		log.Error(err.Error())
	}

	if _, err := client.DbCreate(&entity.DB{Name: dbName}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.DBGet(dbName); err != nil {
		t.Fatal(err)
	}

	idx := 0
	for {

		response, err := client.SpaceCreate(dbName, &entity.Space{
			Name: spaceName,
		})
		if err != nil {
			t.Errorf("create space failed, err: %v\n", err)
			break
		}
		fmt.Println("space create result: ", string(response.Resp))

		_, err = client.SpaceCreate(dbName, &entity.Space{
			Name: spaceName,
		})
		if !strings.Contains(err.Error(), "duplicated space") {
			t.Fatal(err)
		}

		_, err = client.DbDelete(dbName)
		if !strings.Contains(err.Error(), "db not empty") {
			t.Fatal(err)
		}

		spaceDelete, err := client.SpaceDelete(dbName, spaceName)
		if err != nil {
			t.Fatal(err)
		}

		log.Info("delete space ok : %s", spaceDelete.Resp)

		if idx > 10 {
			break
		}
		idx++
	}
}

func TestSpaceGo(t *testing.T) {

	dbName := "TestDB"
	spaceName := "TestSpaceGo"

	client := testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, dbName, spaceName)

	if _, err := client.DbDrop(dbName); err != nil {
		log.Error(err.Error())
	}

	if _, err := client.DbCreate(&entity.DB{Name: dbName}); err != nil {
		t.Fatal(err)
	}

	if _, err := client.DBGet(dbName); err != nil {
		t.Fatal(err)
	}

	times := 10

	wg := sync.WaitGroup{}

	wg.Add(times)

	for i := 0; i < times; i++ {

		go func(idx int) {
			defer wg.Done()

			sName := dbName + cast.ToString(idx)

			_, _ = client.SpaceDelete(dbName, sName)

			response, err := client.SpaceCreate(dbName, &entity.Space{
				Name: sName,
			})
			if err != nil {
				t.Fatal(fmt.Sprintf("create space failed, err: %v", err))
			}

			log.Info(string(response.Resp))

		}(i)

	}

	wg.Wait()

	if _, err := client.DbDrop(dbName); err != nil {
		t.Fatal(err)
	}
}
