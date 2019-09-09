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

package logic

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/vearch/vearch/proto"
	"github.com/vearch/vearch/util/cbjson"
	"github.com/vearch/vearch/util/netutil"
)

func NewNormal(master, router, datafile string) *normal {
	return &normal{
		master:   master,
		router:   router,
		datafile: datafile,
	}
}

type normal struct {
	master   string
	router   string
	datafile string
}

func (this normal) Load() error {
	fi, err := os.Open(this.datafile)
	if err != nil {
		panic(err)
	}
	defer fi.Close()

	dbName := ""
	spaceName := ""

	br := bufio.NewScanner(fi)
	for br.Scan() {
		jsonMap, err := cbjson.ByteToJsonMap(br.Bytes())
		if err != nil {
			fmt.Println(br.Text())
			panic(err)
		}

		isDb := jsonMap.GetJsonVal("_db")
		if isDb != nil {
			dbName = isDb.(string)
			found, err := this.apiFindDb(dbName)
			if err != nil {
				return err
			}
			if !found {
				err := this.apiCreateDb(dbName)
				if err != nil {
					return err
				}
			}
			continue
		}

		isSpace := jsonMap.GetJsonVal("_space")
		if isSpace != nil {
			spaceName = isSpace.(string)

			br.Scan()
			schema := br.Text()

			this.apiDeleteSpace(dbName, spaceName)
			this.apiCreateSpace(dbName, spaceName, schema)
			continue
		}

		isDoc := jsonMap.GetJsonVal("_id")
		if isDoc != nil {
			docID := isDoc.(string)

			br.Scan()
			doc := br.Text()

			this.apiSaveDoc(dbName, spaceName, docID, doc)
			continue
		}
	}

	fmt.Println("success.")

	return nil
}

func (this normal) apiCreateSpace(dbName, spaceName string, schema string) error {
	address := "http://" + this.master
	query := netutil.NewQuery()
	query.SetMethod(http.MethodPut)
	query.SetAddress(address)
	query.SetUrlPath("/space/" + dbName + "/_create")
	query.SetReqBody(schema)
	query.SetContentTypeJson()
	//fmt.Printf("\n test es normal create space url %v", query.GetUrl())
	response, err := query.Do()
	if err != nil {
		return err
	}
	//fmt.Printf("\n test es normal create space result: %v", string(response))

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		panic(err)
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		panic(err)
	}
	if code != pkg.ERRCODE_SUCCESS {
		return fmt.Errorf("create space err: %v", jsonMap.GetJsonValString("msg"))
	}

	return nil
}

func (this normal) apiFindDb(dbName string) (bool, error) {
	address := "http://" + this.master
	query := netutil.NewQuery()
	query.SetMethod(http.MethodGet)
	query.SetAddress(address)
	query.SetUrlPath("/db/" + dbName)
	//fmt.Printf("\n test es normal find db url %v", query.GetUrl())
	response, err := query.Do()
	if err != nil {
		return false, err
	}
	//fmt.Printf("\n test es normal find db result: %v", string(response))

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		return false, err
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		panic(err)
	}
	if code != pkg.ERRCODE_SUCCESS {
		if code == pkg.ERRCODE_DB_NOTEXISTS {
			return false, nil
		}
		return false, fmt.Errorf("find db err: %v", jsonMap.GetJsonValString("msg"))
	}

	return true, nil
}

func (this normal) apiCreateDb(dbName string) error {
	data := make(map[string]interface{})
	data["Name"] = dbName
	jsonData, err := cbjson.Marshal(data)

	address := "http://" + this.master
	query := netutil.NewQuery()
	query.SetMethod(http.MethodPut)
	query.SetAddress(address)
	query.SetUrlPath("/db/_create")
	query.SetReqBody(string(jsonData))
	query.SetContentTypeJson()
	//fmt.Printf("\n test es normal create db url %v, data: %v", query.GetUrl(), string(jsonData))
	response, err := query.Do()
	if err != nil {
		return err
	}
	//fmt.Printf("\n test es normal create db result: %v", string(response))

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		return err
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		panic(err)
	}
	if code != pkg.ERRCODE_SUCCESS {
		return fmt.Errorf("create db err: %v", jsonMap.GetJsonValString("msg"))
	}

	return nil
}

func (this normal) apiSaveDoc(dbName, spaceName string, docID, doc string) error {
	address := "http://" + this.router
	query := netutil.NewQuery()
	query.SetMethod(http.MethodPost)
	query.SetAddress(address)
	query.SetUrlPath("/" + dbName + "/" + spaceName + "/" + docID)
	query.SetReqBody(doc)
	query.SetContentTypeJson()
	//fmt.Printf("\n test es normal insert doc url %v", query.GetUrl())
	response, err := query.Do()
	if err != nil {
		return err
	}
	//fmt.Printf("\n test es normal insert doc result: %v", string(response))

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		panic(err)
	}

	result := jsonMap.GetJsonVal("result")
	if result == nil {
		status, err := jsonMap.GetJsonValIntE("status")
		if err != nil {
			panic(err)
		}
		if status != pkg.ERRCODE_SUCCESS {
			panic(jsonMap.GetJsonValString("error"))
		}
	}

	return nil
}

func (this normal) deleteDb(dbName string) error {
	data, err := this.apiListSpace(dbName)
	if err != nil {
		return err
	}
	for i := range data {
		jsonMap := data[i]
		spaceName := jsonMap.GetJsonValString("name")
		err := this.apiDeleteSpace(dbName, spaceName)
		if err != nil {
			return err
		}
	}

	err = this.apiDeleteDb(dbName)
	if err != nil {
		return err
	}

	return nil
}

func (this normal) apiDeleteSpace(dbName string, spaceName string) error {
	address := "http://" + this.master
	query := netutil.NewQuery()
	query.SetMethod(http.MethodDelete)
	query.SetAddress(address)
	query.SetUrlPath("/space/" + dbName + "/" + spaceName)
	//fmt.Printf("\n test es normal delete space url %v", query.GetUrl())
	response, err := query.Do()
	//fmt.Printf("\n test es normal delete space result: %v", string(response))
	if err != nil {
		panic(err)
	}

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		panic(err)
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		panic(err)
	}
	if code != pkg.ERRCODE_SUCCESS {
		return fmt.Errorf("delete space err: %v", jsonMap.GetJsonValString("msg"))
	}

	return nil
}

func (this normal) apiDeleteDb(dbName string) error {
	address := "http://" + this.master
	query := netutil.NewQuery()
	query.SetMethod(http.MethodDelete)
	query.SetAddress(address)
	query.SetUrlPath("/db/" + dbName)
	//fmt.Printf("\n test es normal delete db url %v", query.GetUrl())
	response, err := query.Do()
	if err != nil {
		return err
	}
	//fmt.Printf("\n test es normal delete db result: %v", string(response))

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		return err
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		panic(err)
	}
	if code != pkg.ERRCODE_SUCCESS {
		return fmt.Errorf("delete db err: %v", jsonMap.GetJsonValString("msg"))
	}

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
	//fmt.Printf("\n test es normal list space url %v", query.GetUrl())
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
