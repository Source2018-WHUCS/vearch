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

package document

import (
	"fmt"
	"github.com/tiglabs/log"
	. "github.com/vearch/vearch/test"
	"github.com/vearch/vearch/util/assert"
	"github.com/vearch/vearch/util/cbjson"
	"net/http"
	"testing"
)

func TestSearchPhraseQuery(t *testing.T) {
	client := InitSimpleBeginBySchema("TestSearchPhraseQuery", "TestSearchPhraseQuery", `{        
                "msg":{
                    "store":false,
                    "type":"text"
                    },  
                "file":{
                    "store":false,
                    "type":"keyword"
                    },  
                "microsecond":{
                    "store":false,
                    "type":"long"
                    },  
                "ip":{
                    "store":false,
                    "type":"keyword"
                    },  
                "host":{
                    "store":false,
                    "type":"keyword"
                    },  
                "line-num":{
                    "store":false,
                    "type":"long"
                    },  
                "time":{
                    "store":false,
                    "type":"long"
                    },  
                "app-name":{
                    "store":false,
                    "type":"keyword"
                    },  
                "group":{
                    "store":false,
                    "type":"keyword"
                    }   
                }`)

	_, err := client.DocumentCreate("123", []byte(`{"msg": "2019-03-25 06:02:47.016[INFO ]inWhichArea:lat[34.70202672914147],lng[113.72345161871907],gridKey[LAT18LNG17][com.jd.gaia.service.impl.MapAreaServiceImpl.inWhichArea:38]"}`))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.Flush(); err != nil {
		t.Fatal(err)
	}

	response, err := client.Search(http.MethodPost, `
 {
 "query" : {
    "bool" : {
      "must" : [
        {"match_phrase": {"msg": "inWhichArea:lat[34.70202672914147],lng[113.72345161871907]"}}
      ]
    }
  }

 }
		
	`)
	if err != nil {
		t.Fatal(err)
	}

	matchPhraseMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	matchPhraseSize, _ := matchPhraseMap.GetJsonMap("hits").GetJsonValIntE("total")

	assert.Equal(t, matchPhraseSize, 1, "size not same")

	fmt.Printf("part1 total :%v\n", matchPhraseSize)
}
