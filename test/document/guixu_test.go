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

package document

import (
	. "github.com/tiglabs/baudengine/test"
	"fmt"
	"github.com/tiglabs/baudengine/util/assert"
	"github.com/tiglabs/baudengine/util/cbjson"
	"github.com/tiglabs/log"
	"net/http"
	"testing"
)

func TestGuiXuLogbookRangeTime(t *testing.T) {

	if 1==1{
		fmt.Println("this case only set frozen 5 second")
		return
	}

	client,dbName, spaceName := InitGuiXuLogbookBegin()

	data := `
	{
		"query": {
			"range" : {
				"time" : {
					"gte" : 1553225598000
				}
			}
		}
	}
	`
	response, err := client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal TestLogbook doc result : %v\n", string(response.Resp))
	allMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}
	allTotal, _ := allMap.GetJsonMap("hits").GetJsonValIntE("total")

	assert.Equal(t, 2090 , allTotal, "not same")

	space, err := client.SpaceGet(dbName, spaceName)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1 , len(space.WorkedPartitions), "not same")

	assert.Equal(t, 5 , len(space.Partitions), "partitions ")
}
