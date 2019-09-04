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

package testutil

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/tiglabs/log"
	"os"
	"strings"
	"sync"
)
import tigos "github.com/vearch/vearch/util/runtime/os"

type StringMap map[string]interface{}

const (
	TotalDocCnt = 200
)

type TestConf struct {
	RootName     string
	RootPassword string
	UserName     string
	UserPassword string
	MasterAddr   string
	RouterAddr   string
}

var Config *TestConf

var ConfigPath string

var lock sync.Mutex

func C() *TestConf {

	if Config != nil {
		return Config
	}

	var path string

	if ConfigPath != "" {
		path = ConfigPath
	} else {
		path = FindConfigPath()
	}

	if path == "" {
		panic("can not found config path , please check it ")
	}

	fmt.Println("find config path ", path)

	config := &TestConf{}

	if _, e := toml.DecodeFile(path, config); e != nil {
		panic(e)
	}

	Config = config

	return Config

}

func FindConfigPath() string {

	if currentExePath, err := tigos.GetCurrentPath(); err == nil {
		path := currentExePath + "test.toml"
		if ok, err := pathExists(path); ok {
			return path
		} else if err != nil {
			log.Error("check path:%s err : %s", path, err.Error())
		}
	}

	if sourceCodeFileName, err := tigos.GetCurrentSourceCodePath(); nil == err {
		path := sourceCodeFileName
		for i := 0; i < 10; i++ {
			lastIndex := strings.LastIndex(path, "/")
			if lastIndex <= 0 {
				break
			}
			path = path[0:lastIndex]
			filePath := path + "/test.toml"
			if ok, err := pathExists(filePath); ok {
				return filePath
			} else if err != nil {
				log.Error("check path:%s err : %s", path, err.Error())
			}

		}

	}

	return ""
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

const DocumentFirstSpaceConfig = `{
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
const DocumentExtraSpaceConfig = `{
    "address":{
        "type":"text"
    },
    "lastname":{
        "type":"text"
    },
    "state":{
        "type":"text"
    },
    "KW_INFO":{
        "type":"text"
    }
}`
