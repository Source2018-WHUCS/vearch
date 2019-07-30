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

package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cast"
	"github.com/tiglabs/baudengine/proto"
	"github.com/tiglabs/baudengine/proto/entity"
	"github.com/tiglabs/baudengine/test/testutil"
	"github.com/tiglabs/baudengine/util/cbjson"
	tigos "github.com/tiglabs/baudengine/util/runtime/os"
	"github.com/tiglabs/log"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

func InitSimpleBeginBySchema(dbName, spaceName, schema string) (client *testutil.CBClient) {

	client = testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, dbName, spaceName)

	client.DbDrop(dbName)

	//create user
	obj, err := client.UserCreate(testutil.C().UserName, testutil.C().UserPassword,
		"create|alter|drop|truncate|insert|delete|update|select", "%", dbName)
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	obj, err = client.UserAddDB(testutil.C().UserName, dbName)
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	obj, err = client.DbCreate(&entity.DB{
		Name: dbName,
	})
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	obj, err = client.SpaceCreate(dbName, &entity.Space{
		Name:          spaceName,
		DynamicSchema: "strict",
		Properties:    []byte(schema),
	})
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	return client
}

func InitSimpleBeginByPartitionNum(dbName, spaceName string, pNum int) (client *testutil.CBClient) {

	client = testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, dbName, spaceName)

	client.DbDrop(dbName)

	//create user
	obj, err := client.UserCreate(testutil.C().UserName, testutil.C().UserPassword,
		"create|alter|drop|truncate|insert|delete|update|select", "%", dbName)
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	obj, err = client.UserAddDB(testutil.C().UserName, dbName)
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	obj, err = client.DbCreate(&entity.DB{
		Name: dbName,
	})
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	obj, err = client.SpaceCreate(dbName, &entity.Space{
		Name:          spaceName,
		DynamicSchema: "true",
		Properties:    []byte("{}"),
		PartitionNum:  pNum,
	})
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	return client
}

func InitSimpleBegin(dbName, spaceName string) (client *testutil.CBClient) {

	client = testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, dbName, spaceName)

	client.DbDrop(dbName)

	//create user
	obj, err := client.UserCreate(testutil.C().UserName, testutil.C().UserPassword,
		"create|alter|drop|truncate|insert|delete|update|select", "%", dbName)
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	obj, err = client.UserAddDB(testutil.C().UserName, dbName)
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	obj, err = client.DbCreate(&entity.DB{
		Name: dbName,
	})
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	obj, err = client.SpaceCreate(dbName, &entity.Space{
		Name:          spaceName,
		DynamicSchema: "true",
		Properties:    []byte("{}"),
	})
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	return client
}

var searchOnce = sync.Once{}

func InitSearchBegin() (client *testutil.CBClient) {

	dbName, spaceName := "test_db", "test_document"

	client = testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, dbName, spaceName)

	searchOnce.Do(func() {

		//create user
		response, err := client.UserCreate(testutil.C().UserName, testutil.C().UserPassword,
			"create|alter|drop|truncate|insert|delete|update|select", "%", dbName)
		if err != nil && !strings.Contains(err.Error(), "duplicate user") {
			panic(err)
		}

		response, err = client.UserAddDB(testutil.C().UserName, dbName)
		if err != nil {
			log.Error(err.Error())
		} else {
			log.Info(string(response.Resp))
		}

		if response != nil && response.Status != 200 {
			panic(string(response.Resp))
		}

		db, err := client.DBGet(dbName)

		if db == nil && !strings.Contains(err.Error(), pkg.ErrMasterDbNotExists.Error()) {
			panic(err)
		}

		if db != nil {
			resp, err := client.DbDrop(dbName)
			if err != nil && err.Error() != pkg.ErrMasterDbNotExists.Error() {
				panic(err)
			}
			if resp != nil {
				log.Info(string(resp.Resp))
			}

		}

		create, err := client.DbCreate(&entity.DB{
			Name: dbName,
		})
		if err != nil {
			panic(err)
		}
		create.Check()

		var DocumentFirstSpaceConfig = `{
    "city":{
        "type":"text"
    },
    "firstname":{
        "type":"text"
    },
    "age":{
        "type":"integer"
    },
    "insert_time":{
        "type":"date"
    },
    "content":{
        "type":"text"
    },
    "KW_TITLE":{
        "type":"text"
    },
    "geo_location":{
        "type":"geo_point"
    }
}`

		create, err = client.SpaceCreate(dbName, &entity.Space{
			Name:          spaceName,
			DynamicSchema: "true",
			Properties:    []byte(DocumentFirstSpaceConfig),
		})
		if err != nil {
			panic(err)
		}
		create.Check()

		pwd := ""
		if currentPath, err := tigos.GetCurrentSourceCodePath(); err != nil {
			panic(err)
		} else {
			pwd = currentPath[0 : strings.LastIndex(currentPath, "/")+1]
		}

		corpusPath := pwd + "/corpus/corpus.json"

		f, err := os.Open(corpusPath)
		if err != nil {
			panic(err)
		}
		out, err := ioutil.ReadAll(f)
		if err != nil {
			panic(err)
		}
		fileContent := string(out)
		s := strings.Split(fileContent, "\n")

		log.Info("test es normal insert doc start.")
		for i := 0; i < len(s); i++ {
			line := s[i]

			if len(line) < 1 {
				continue
			}

			var docMap interface{}
			err = cbjson.Unmarshal([]byte(line), &docMap)
			if err != nil {
				panic(err)
			}

			doc, err := cbjson.Marshal(docMap)
			if err != nil {
				panic(err)
			}

			docId := i + 1

			_, err = client.DocumentUpdateOrCreate(cast.ToString(docId), string(doc))
			if err != nil {
				panic(err)
			}
		}
		log.Info("\n test es normal insert doc over.")

		flush, err := client.Flush()

		if err != nil {
			panic(err)
		}

		log.Info("flush ok %s", flush)
	})

	return client
}

var logBookOnce = sync.Once{}

func InitLogbookBegin() (client *testutil.CBClient) {
	dbName, spaceName := "index_25", "applog"

	client = testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, dbName, spaceName)

	logBookOnce.Do(func() {

		//create user
		_, err := client.UserCreate(testutil.C().UserName, testutil.C().UserPassword,
			"create|alter|drop|truncate|insert|delete|update|select", "%", dbName)
		if err != nil {
			log.Error(err.Error())
		}

		db, err := client.DBGet(dbName)

		if db == nil && !strings.Contains(err.Error(), pkg.ErrMasterDbNotExists.Error()) {
			panic(err)
		}

		if db != nil {
			resp, err := client.DbDrop(dbName)
			if err != nil && err.Error() != pkg.ErrMasterDbNotExists.Error() {
				panic(err)
			}
			if resp != nil {
				log.Info(string(resp.Resp))
			}

		}

		create, err := client.DbCreate(&entity.DB{
			Name: dbName,
		})
		if err != nil {
			panic(err)
		}
		create.Check()

		var DocumentFirstSpaceConfig = `
{
    "id":{
        "type":"keyword"
    },
    "time":{
        "type":"date"
    },
    "msg":{
        "type":"text"
    },
    "host":{
        "type":"keyword"
    },
    "ip":{
        "type":"keyword"
    },
    "app-name":{
        "type":"keyword"
    },
    "file":{
        "type":"keyword"
    },
    "microsecond":{
        "type":"integer"
    },
	"group":{
        "type":"keyword"
    }
}`

		create, err = client.SpaceCreate(dbName, &entity.Space{
			PartitionNum:  1,
			Name:          spaceName,
			DynamicSchema: "true",
			DefaultField:  "_id",
			Properties:    []byte(DocumentFirstSpaceConfig),
		})
		if err != nil {
			panic(err)
		}
		create.Check()

		pwd := ""
		if currentPath, err := tigos.GetCurrentSourceCodePath(); err != nil {
			panic(err)
		} else {
			pwd = currentPath[0 : strings.LastIndex(currentPath, "/")+1]
		}
		corpusPath := pwd + "/corpus/logbook.json"

		f, err := os.Open(corpusPath)
		if err != nil {
			panic(err)
		}
		out, err := ioutil.ReadAll(f)
		if err != nil {
			panic(err)
		}
		fileContent := string(out)
		s := strings.Split(fileContent, "\n")

		log.Info("test es normal insert doc start.")
		var buffer bytes.Buffer
		for i := 0; i < len(s); i++ {
			line := s[i]

			if len(line) < 1 {
				continue
			}

			metaIndex := map[string]interface{}{
				"_index": "index_25",
				"_type":  spaceName,
			}
			meta := map[string]interface{}{
				"index": metaIndex,
			}
			v, _ := cbjson.Marshal(meta)
			buffer.Write(v)
			buffer.WriteString("\n")
			mm := make(map[string]interface{})
			_ = json.Unmarshal([]byte(line), &mm)
			mm["id"] = cast.ToString(i)
			marshal, _ := json.Marshal(mm)
			buffer.Write(marshal)
			buffer.WriteString("\n")
		}

		_, err = client.DocumentBulk(buffer.String())
		if err != nil {
			panic(err)
		}

		log.Info("test es normal insert doc over.")

		flush, err := client.Flush()

		if err != nil {
			panic(err)
		}

		log.Info("flush ok %s", flush)
	})

	return client
}

var logGuiXuBookOnce = sync.Once{}

func InitGuiXuLogbookBegin() (client *testutil.CBClient, dbName, spaceName string) {
	dbName, spaceName = "index_26", "app_log"

	client = testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, dbName, spaceName)

	logGuiXuBookOnce.Do(func() {
		//create user
		response, err := client.UserCreate(testutil.C().UserName, testutil.C().UserPassword,
			"create|alter|drop|truncate|insert|delete|update|select", "%", dbName)
		if err != nil {
			panic(err)
		}

		if response.Status != 200 {
			panic(string(response.Resp))
		}

		db, err := client.DBGet(dbName)

		if db == nil && !strings.Contains(err.Error(), pkg.ErrMasterDbNotExists.Error()) {
			panic(err)
		}

		if db != nil {
			resp, err := client.DbDrop(dbName)
			if err != nil && err.Error() != pkg.ErrMasterDbNotExists.Error() {
				panic(err)
			}
			if resp != nil {
				log.Info(string(resp.Resp))
			}

		}

		create, err := client.DbCreate(&entity.DB{
			Name: dbName,
		})
		if err != nil {
			panic(err)
		}
		create.Check()

		var DocumentFirstSpaceConfig = `
{
    "time":{
        "type":"date"
    },
    "msg":{
        "type":"text"
    },
    "host":{
        "type":"text"
    },
    "ip":{
        "type":"text"
    },
    "app-name":{
        "type":"keyword"
    },
    "file":{
        "type":"text"
    },
    "microsecond":{
        "type":"integer"
    },
	"group":{
        "type":"text"
    }
}`

		create, err = client.SpaceCreate(dbName, &entity.Space{
			Name:          spaceName,
			DynamicSchema: "true",
			PartitionNum:  1,
			Engine:        &entity.Engine{Name: entity.GuiXu, ZoneField: "time", ExpireMinute: 100},
			Properties:    []byte(DocumentFirstSpaceConfig),
		})
		if err != nil {
			panic(err)
		}
		create.Check()

		pwd := ""
		if currentPath, err := tigos.GetCurrentSourceCodePath(); err != nil {
			panic(err)
		} else {
			pwd = currentPath[0 : strings.LastIndex(currentPath, "/")+1]
		}
		corpusPath := pwd + "/corpus/logbook.json"

		f, err := os.Open(corpusPath)
		if err != nil {
			panic(err)
		}
		out, err := ioutil.ReadAll(f)
		if err != nil {
			panic(err)
		}
		fileContent := string(out)
		s := strings.Split(fileContent, "\n")

		log.Info("test es normal insert doc start.")
		var buffer bytes.Buffer
		for i := 0; i < len(s); i++ {
			line := s[i]

			if len(line) < 1 {
				continue
			}

			metaIndex := map[string]interface{}{
				"_index": "index_26",
				"_type":  spaceName,
			}
			meta := map[string]interface{}{
				"index": metaIndex,
			}
			v, _ := cbjson.Marshal(meta)
			buffer.Write(v)
			buffer.WriteString("\n")
			buffer.Write([]byte(line))
			buffer.WriteString("\n")
		}

		for i := 0; i < 10; i++ {
			_, err = client.DocumentBulk(buffer.String())
			time.Sleep(5 * time.Second)
		}
		if err != nil {
			panic(err)
		}

		log.Info("\n test es normal insert doc over.")

		flush, err := client.Flush()

		if err != nil {
			panic(err)
		}

		log.Info("flush ok %s", flush)
	})

	return client, dbName, spaceName
}

var aggsOnce = sync.Once{}

func InitAggBegin() (client *testutil.CBClient) {

	dbName, spaceName := "agg_db", "agg_table"

	client = testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, dbName, spaceName)

	aggsOnce.Do(func() {

		//create user
		response, err := client.UserCreate(testutil.C().UserName, testutil.C().UserPassword,
			"create|alter|drop|truncate|insert|delete|update|select", "%", dbName)
		if err != nil && !strings.Contains(err.Error(), "duplicate user") {
			panic(err)
		}

		if response != nil && response.Status != 200 {
			panic(string(response.Resp))
		}

		db, err := client.DBGet(dbName)

		if db == nil && !strings.Contains(err.Error(), pkg.ErrMasterDbNotExists.Error()) {
			panic(err)
		}

		if db != nil {
			resp, err := client.DbDrop(dbName)
			if err != nil && err.Error() != pkg.ErrMasterDbNotExists.Error() {
				panic(err)
			}
			if resp != nil {
				log.Info(string(resp.Resp))
			}

		}

		create, err := client.DbCreate(&entity.DB{
			Name: dbName,
		})
		if err != nil {
			panic(err)
		}
		create.Check()

		var DocumentFirstSpaceConfig = `{
				"title": {
					"type": "keyword"
				},
				"name": {
					"type": "keyword"
				},
				"ip": {
					"type": "keyword"
				},
				"age": {
					"type": "integer"
				},
				"birthday": {
					"type": "date"
				},
				"content": {
					"type": "string",
					"analyzer": "cb_standard"
				},
				"point": {
					"type": "string"
				},
				"location": {
					"type": "geo_point"
				},
				"time_stamp": {
					"type": "date"
				},
				"time_stamp2": {
					"type": "date"
				},
				"time_stamp3": {
					"type": "date"
				},
				"youyou":{
					"properties":{
						"title": {
							"type": "string"
						},
						"name": {
							"type": "string"
						},
						"age": {
							"type": "integer"
						},
						"birthday": {
							"type": "date"
						}
					}

				}
			}`

		create, err = client.SpaceCreate(dbName, &entity.Space{
			Name:          spaceName,
			DynamicSchema: "true",
			Properties:    []byte(DocumentFirstSpaceConfig),
		})
		if err != nil {
			panic(err)
		}
		create.Check()

		if err != nil {
			panic(err)
		}

		var TotalDocCnt = 200

		//插入测试数据
		for i := 0; i < TotalDocCnt; i++ {
			names := []string{"test_" + cast.ToString(i), "test_" + cast.ToString(i)}
			age := i % 100
			year := 2019 - age
			month := i%12 + 1
			day := i % 20
			hour := i % 24
			ipaddr := fmt.Sprintf("%d.%d.%d.%d", i%255, 0, i%192, i%168)
			doc := struct {
				Title      string    `json:"title"`
				Name       []string  `json:"name"`
				Ip         string    `json:"ip"`
				Age        int       `json:"age"`
				Birthday   time.Time `json:"birthday"`
				Content    string    `json:"content"`
				Location   Loc       `json:"location"`
				TimeStamp  int64     `json:"time_stamp"`
				TimeStamp3 string    `json:"time_stamp3"`
				FloatValue float64   `json:"float_value"`
			}{
				Title:      "test_" + cast.ToString(i),
				Name:       names,
				Ip:         ipaddr,
				Age:        int(i % 100),
				Birthday:   time.Date(year, time.Month(month), day, hour, 0, 0, 0, time.UTC),
				TimeStamp3: "1985-08-28 11:22:33",
				Content:    cast.ToString(i) + "I Love TiaAnmen who is me",
				Location:   Loc{12, 12},
				TimeStamp:  time.Now().UnixNano() / 1e6,
				FloatValue: rand.Float64(),
			}

			if bs, e := json.Marshal(doc); e != nil {
				panic(e)
			} else {
				_, err := client.DocumentUpdateOrCreate(cast.ToString(i), bs)
				if err != nil {
					panic(err)
				}
			}
		}

		if resp, err := client.Flush(); err != nil {
			panic(err)
		} else {
			log.Info("flush ok %s ", resp)
		}
	})

	return

}

var sortOnce = sync.Once{}

func InitSortBegin() (client *testutil.CBClient) {

	dbName, spaceName := "sort_db", "sort_table"

	client = testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, dbName, spaceName)

	sortOnce.Do(func() {

		//create user
		response, err := client.UserCreate(testutil.C().UserName, testutil.C().UserPassword,
			"create|alter|drop|truncate|insert|delete|update|select", "%", dbName)
		if err != nil {
			log.Warn(err.Error())
		}

		if response != nil && response.Status != 200 {
			panic(string(response.Resp))
		}

		db, err := client.DBGet(dbName)

		if db == nil && !strings.Contains(err.Error(), pkg.ErrMasterDbNotExists.Error()) {
			panic(err)
		}

		if db != nil {
			resp, err := client.DbDrop(dbName)
			if err != nil && err.Error() != pkg.ErrMasterDbNotExists.Error() {
				panic(err)
			}
			if resp != nil {
				log.Info(string(resp.Resp))
			}

		}

		create, err := client.DbCreate(&entity.DB{
			Name: dbName,
		})
		if err != nil {
			panic(err)
		}
		create.Check()

		var DocumentFirstSpaceConfig = `{
				"title": {
					"type": "keyword"
				},
				"name": {
					"type": "keyword"
				},
				"ip": {
					"type": "keyword"
				},
				"age": {
					"type": "integer"
				},
				"birthday": {
					"type": "date"
				},
				"content": {
					"type": "string",
					"analyzer": "cb_standard"
				},
				"point": {
					"type": "string"
				},
				"location": {
					"type": "geo_point"
				},
				"time_stamp": {
					"type": "date"
				},
				"time_stamp2": {
					"type": "date"
				},
				"time_stamp3": {
					"type": "date"
				},
				"youyou":{
					"properties":{
						"title": {
							"type": "string"
						},
						"name": {
							"type": "string"
						},
						"age": {
							"type": "integer"
						},
						"birthday": {
							"type": "date"
						}
					}

				}
			}`

		create, err = client.SpaceCreate(dbName, &entity.Space{
			Name:          spaceName,
			DynamicSchema: "true",
			Properties:    []byte(DocumentFirstSpaceConfig),
		})
		if err != nil {
			panic(err)
		}
		create.Check()

		if err != nil {
			panic(err)
		}

		var TotalDocCnt = 200

		//插入测试数据
		for i := 0; i < TotalDocCnt; i++ {
			names := []string{"test_" + cast.ToString(i), "test_" + cast.ToString(i)}
			age := i % 100
			year := 2019 - age
			month := i%12 + 1
			day := i % 20
			hour := i % 24
			ipaddr := fmt.Sprintf("%d.%d.%d.%d", i%255, 0, i%192, i%168)
			doc := struct {
				Title      string    `json:"title"`
				Name       []string  `json:"name"`
				Ip         string    `json:"ip"`
				Age        int       `json:"age"`
				Birthday   time.Time `json:"birthday"`
				Content    string    `json:"content"`
				Location   Loc       `json:"location"`
				TimeStamp  int64     `json:"time_stamp"`
				TimeStamp3 string    `json:"time_stamp3"`
				FloatValue float64   `json:"float_value"`
			}{
				Title:      "test_" + cast.ToString(i),
				Name:       names,
				Ip:         ipaddr,
				Age:        int(i % 100),
				Birthday:   time.Date(year, time.Month(month), day, hour, 0, 0, 0, time.UTC),
				TimeStamp3: "1985-08-28 11:22:33",
				Content:    cast.ToString(i) + "I Love TiaAnmen who is me",
				Location:   Loc{12, 12},
				TimeStamp:  time.Now().UnixNano() / 1e6,
				FloatValue: rand.Float64(),
			}

			if bs, e := json.Marshal(doc); e != nil {
				panic(e)
			} else {
				_, err := client.DocumentUpdateOrCreate(cast.ToString(i), bs)
				if err != nil {
					panic(err)
				}
			}
		}

		if resp, err := client.Flush(); err != nil {
			panic(err)
		} else {
			log.Info("flush ok %s ", resp)
		}
	})

	return client

}

type Loc struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

func InitBulk() *testutil.CBClient {
	dbName, spaceName := "bulk_db", "bulk_space"

	client := testutil.NewCBClient(testutil.C().RouterAddr, testutil.C().MasterAddr, dbName, spaceName)

	client.DbDrop(dbName)

	//create user
	obj, err := client.UserCreate(testutil.C().UserName, testutil.C().UserPassword,
		"create|alter|drop|truncate|insert|delete|update|select", "%", dbName)
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	obj, err = client.UserAddDB(testutil.C().UserName, dbName)
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	obj, err = client.DbCreate(&entity.DB{
		Name: dbName,
	})
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	obj, err = client.SpaceCreate(dbName, &entity.Space{
		Name:          spaceName,
		DynamicSchema: "true",
		Properties:    []byte("{}"),
		PartitionNum:  3,
	})
	if err != nil {
		log.Error(err.Error())
	} else {
		log.Info(string(obj.Resp))
	}

	return client
}
