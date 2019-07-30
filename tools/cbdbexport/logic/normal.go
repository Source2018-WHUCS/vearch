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

package logic

import (
	"fmt"
	"github.com/tiglabs/baudengine/util"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/tiglabs/baudengine/proto"
	"github.com/tiglabs/baudengine/util/cbjson"
	"github.com/tiglabs/baudengine/util/netutil"
)

func NewNormal(master, router, dbName, spaceName, outfile string) *normal {
	return &normal{
		master:    master,
		router:    router,
		dbName:    dbName,
		spaceName: spaceName,
		outfile:   outfile,
	}
}

type normal struct {
	master    string
	router    string
	dbName    string
	spaceName string
	outfile   string
}

func (this normal) Save() error {
	if ioutil.WriteFile(this.outfile, []byte(""), 0644) == nil {
		fmt.Printf("clean file: %s \n", this.outfile)
	}

	var dbNames []string
	if this.dbName != "" {
		dbNames = strings.Split(this.dbName, ",")
	} else {
		dbs, err := this.apiListDb()
		if err != nil {
			return err
		}
		for i := range dbs {
			jsonMap := dbs[i]
			dbName := jsonMap.GetJsonValString("name")
			dbNames = append(dbNames, dbName)
		}
	}

	var spaceNames []string
	if this.spaceName != "" {
		spaceNames = strings.Split(this.spaceName, ",")

		spaceNameCheck := make(map[string]string)
		for i := range dbNames {
			dbName := dbNames[i]

			spaces, err := this.apiListSpace(dbName)
			if err != nil {
				return err
			}
			for j := range spaces {
				jsonMap := spaces[j]

				spaceName, err := jsonMap.GetJsonValStringE("name")
				if err != nil {
					return err
				}

				spaceNameCheck[spaceName] = dbName
			}
		}

		for i := range spaceNames {
			spaceName := spaceNames[i]
			if _, ok := spaceNameCheck[spaceName]; !ok {
				return fmt.Errorf("space `%s` not found", spaceName)
			}
		}
	}

	for i := range dbNames {
		dbName := dbNames[i]

		data := map[string]string{}
		data["_db"] = dbName
		dataByte, err := cbjson.Marshal(data)
		if err != nil {
			return err
		}
		util.WriteWithBufio(this.outfile, string(dataByte))
		util.WriteWithBufio(this.outfile, "\n")

		spaces, err := this.apiListSpace(dbName)
		if err != nil {
			return err
		}
		for j := range spaces {
			jsonMap := spaces[j]

			spaceName, err := jsonMap.GetJsonValStringE("name")
			if err != nil {
				return err
			}
			for i := range spaceNames {
				s := spaceNames[i]
				if s != spaceName {
					continue
				}
			}

			data := map[string]string{}
			data["_space"] = spaceName
			dataByte, err := cbjson.Marshal(data)
			if err != nil {
				return err
			}
			util.WriteWithBufio(this.outfile, string(dataByte))
			util.WriteWithBufio(this.outfile, "\n")

			dynamic, err := jsonMap.GetJsonValBoolE("dynamic_schema")
			if err != nil {
				panic(err)
			}
			properties := jsonMap.GetJsonMap("properties")
			partitionNum, err := jsonMap.GetJsonValIntE("partition_num")
			if err != nil {
				panic(err)
			}
			replicaNum, err := jsonMap.GetJsonValIntE("replica_num")
			if err != nil {
				panic(err)
			}
			schema := make(map[string]interface{})
			schema["name"] = spaceName
			schema["partition_num"] = partitionNum
			schema["replica_num"] = replicaNum
			schema["properties"] = properties
			schema["dynamic_schema"] = dynamic
			dataByte, err = cbjson.Marshal(schema)
			if err != nil {
				panic(err)
			}
			util.WriteWithBufio(this.outfile, string(dataByte))
			util.WriteWithBufio(this.outfile, "\n")

			fmt.Printf("save data `%s`:`%s` \n", dbName, spaceName)
			jsonArrMap, err := this.apiSearch(dbName, spaceName)
			if err != nil {
				return err
			}
			for _, hitMap := range jsonArrMap {
				docID, err := hitMap.GetJsonValStringE("_id")
				if err != nil {
					return err
				}

				data := map[string]string{}
				data["_id"] = docID
				dataByte, err := cbjson.Marshal(data)
				if err != nil {
					return err
				}
				util.WriteWithBufio(this.outfile, string(dataByte))
				util.WriteWithBufio(this.outfile, "\n")

				sourceMap := hitMap.GetJsonMap("_source")
				dataByte, err = cbjson.Marshal(sourceMap)
				if err != nil {
					return err
				}
				util.WriteWithBufio(this.outfile, string(dataByte))
				util.WriteWithBufio(this.outfile, "\n")
			}
		}
	}

	fmt.Println("success.")

	return nil
}

func (this normal) apiListSpace(dbName string) (cbjson.JsonArrMap, error) {
	form := url.Values{}
	form.Add("db", dbName)

	address := "http://" + this.master
	query := netutil.NewQuery()
	query.SetMethod(http.MethodGet)
	query.SetAddress(address)
	query.SetQuery(form.Encode())
	query.SetUrlPath("/list/space")
	//fmt.Printf("\n test es normal list space url: %v", query.GetUrl())
	response, err := query.Do()
	if err != nil {
		panic(err)
	}
	//fmt.Printf("\n test es normal list space result: %v", string(response))

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		panic(err)
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		panic(err)
	}
	if code != pkg.ERRCODE_SUCCESS {
		return nil, fmt.Errorf("list space err: %v", jsonMap.GetJsonValString("msg"))
	}

	data := jsonMap.GetJsonArrMap("data")
	if err != nil {
		panic(err)
	}

	return data, nil
}

func (this normal) apiListDb() (cbjson.JsonArrMap, error) {
	address := "http://" + this.master
	query := netutil.NewQuery()
	query.SetMethod(http.MethodGet)
	query.SetAddress(address)
	query.SetUrlPath("/list/db")
	//fmt.Printf("\n test es normal list db url %v", query.GetUrl())
	response, err := query.Do()
	if err != nil {
		panic(err)
	}
	//fmt.Printf("\n test es normal list db result: %v", string(response))

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		panic(err)
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		panic(err)
	}
	if code != pkg.ERRCODE_SUCCESS {
		return nil, fmt.Errorf("list db err: %v", jsonMap.GetJsonValString("msg"))
	}

	data := jsonMap.GetJsonArrMap("data")
	if err != nil {
		panic(err)
	}

	return data, nil
}

func (this normal) apiSearch(dbName, spaceName string) (cbjson.JsonArrMap, error) {
	address := "http://" + this.router
	query := netutil.NewQuery()
	query.SetMethod(http.MethodPost)
	query.SetAddress(address)
	query.SetUrlPath("/" + dbName + "/" + spaceName + "/_search?size=1000000")
	query.SetContentTypeJson()
	//fmt.Printf("\n test es normal ciSearchAll doc url %v", query.GetUrl())
	response, err := query.Do()
	//fmt.Printf("\n test es normal ciSearchAll doc result: %v", string(response))
	if err != nil {
		return nil, err
	}

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		panic(err)
	}

	hitsMap := jsonMap.GetJsonMap("hits")
	if hitsMap == nil {
		code, err := jsonMap.GetJsonValIntE("code")
		if err != nil {
			return nil, fmt.Errorf("test es normal ciSearchAll doc parse response code error: %s", err.Error())
		}
		if code != pkg.ERRCODE_SUCCESS {
			return nil, fmt.Errorf("test es normal ciSearchAll doc parse response error: %s", jsonMap.GetJsonValStringOrDefault("msg", ""))
		}
	}

	jsonArrMap := hitsMap.GetJsonArrMap("hits")

	return jsonArrMap, nil
}

