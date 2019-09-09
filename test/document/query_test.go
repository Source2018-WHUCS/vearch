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
	"github.com/goinggo/mapstructure"
	"github.com/spf13/cast"
	"github.com/tiglabs/log"
	"github.com/vearch/vearch/proto"
	. "github.com/vearch/vearch/test"
	"github.com/vearch/vearch/util/assert"
	"github.com/vearch/vearch/util/cbjson"
	"net/http"
	"testing"
)

func TestTermQuery(t *testing.T) {
	client := InitSearchBegin()
	result, err := client.Search(http.MethodPost, `
	{
	    "query": {
	        "term" : {
	            "firstname" : "fulton"
	        }
	    }
	}
	`)

	if err != nil {
		t.Fatal(err)
	}

	jsonMap, err := cbjson.ByteToJsonMap(result.Resp)
	if err != nil {
		t.Fatal(err)
	}

	hitsMap := jsonMap.GetJsonMap("hits")
	if hitsMap == nil {
		code, err := jsonMap.GetJsonValIntE("code")
		if err != nil {
			t.Fatal(err)
		}
		msg, err := jsonMap.GetJsonValStringE("msg")
		if err != nil {
			t.Fatal(err)
		}
		if code != pkg.ERRCODE_SUCCESS {
			t.Fatal(err, msg)
		}
	}

	total, err := hitsMap.GetJsonValIntE("total")
	if err != nil {
		t.Fatal(total, err)
	}
	if total < 1 {
		t.Fatal(total, err)
	}
}

func TestSearchAll(t *testing.T) {
	DocumentCorpusTotal := 200

	client := InitSearchBegin()
	response, err := client.Search(http.MethodGet, "")

	if err != nil {
		t.Fatal(err)
	}

	jsonMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	hitsMap := jsonMap.GetJsonMap("hits")
	if hitsMap == nil {
		code, err := jsonMap.GetJsonValIntE("code")
		if err != nil {
			t.Fatal(fmt.Errorf("test es normal ciSearchAll doc parse response code error: %s", err.Error()))
		}
		msg, err := jsonMap.GetJsonValStringE("msg")
		if err != nil {
			t.Fatal(fmt.Errorf("test es normal ciSearchAll doc parse response msg error: %s", err.Error()))
		}
		if code != pkg.ERRCODE_SUCCESS {
			t.Fatal(fmt.Errorf("test es normal ciSearchAll doc parse response error: %s", msg))
		}
	}

	total, err := hitsMap.GetJsonValIntE("total")
	if err != nil {
		t.Fatal(fmt.Errorf("test es normal ciSearchAll doc parse response hits total error: %s", err.Error()))
	}
	if total < DocumentCorpusTotal {
		t.Fatal(fmt.Errorf("\n test es normal ciSearchAll err: total [%d] is not equals %v \n", total, DocumentCorpusTotal))
	}

	hitsArr := hitsMap.GetJsonArr("hits")
	if hitsArr == nil {
		t.Fatal(fmt.Errorf("test es normal ciSearchAll doc parse response hits error: hits not found"))
	}
	checkColumnName := "firstname"
	for _, v := range hitsArr {
		hitMap := &cbjson.JsonMap{}
		if err := mapstructure.Decode(v, hitMap); err != nil {
			t.Fatal(err)
		}

		sourceMap := hitMap.GetJsonMap("_source")
		columnValue := sourceMap.GetJsonVal(checkColumnName)
		if columnValue == nil {
			log.Info("test es normal ciSearchAll row %v \n", sourceMap)
			t.Fatal(fmt.Errorf("\n test ciSearchAll row column name (`%s`) not found. \n", checkColumnName))
		}
	}

}

func TestSearchTerm(t *testing.T) {
	data := `
{
   "query": {
       "term" : {
           "firstname" : "fulton"
       }
   }
}
`
	client := InitSearchBegin()
	response, err := client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("\n test es normal ciSearchTerm doc result: %v", string(response.Resp))

	jsonMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	hitsMap := jsonMap.GetJsonMap("hits")
	if hitsMap == nil {
		code, err := jsonMap.GetJsonValIntE("code")
		if err != nil {
			t.Fatal(fmt.Errorf("test es normal ciSearchTerm doc parse response code error: %s", err.Error()))
		}
		msg, err := jsonMap.GetJsonValStringE("msg")
		if err != nil {
			t.Fatal(fmt.Errorf("test es normal ciSearchTerm doc parse response msg error: %s", err.Error()))
		}
		if code != pkg.ERRCODE_SUCCESS {
			t.Fatal(fmt.Errorf("test es normal ciSearchTerm doc parse response error: %s", msg))
		}
	}

	total, err := hitsMap.GetJsonValIntE("total")
	if err != nil {
		t.Fatal(fmt.Errorf("test es normal ciSearchTerm doc parse response hits total error: %s", err.Error()))
	}

	assert.True(t, total == 1)

	log.Info("TestSearchTerm ok")
}

func TestSearchMatchAll(t *testing.T) {
	DocumentCorpusTotal := 200
	data := `
{
   "query": {
       "match_all": {}
   }
}`
	client := InitSearchBegin()
	response, err := client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result: %v", string(response.Resp))

	jsonMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	hitsMap := jsonMap.GetJsonMap("hits")
	if hitsMap == nil {
		code, err := jsonMap.GetJsonValIntE("code")
		if err != nil {
			t.Fatal(err)
		}
		msg, err := jsonMap.GetJsonValStringE("msg")
		if err != nil {
			t.Fatal(err)
		}
		if code != pkg.ERRCODE_SUCCESS {
			t.Fatal(msg)
		}
	}

	total, err := hitsMap.GetJsonValIntE("total")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("totoal :%v\n", total)
	if total < DocumentCorpusTotal {
		t.Fatal(err)
	}

	hitsArr := hitsMap.GetJsonArr("hits")
	if hitsArr == nil {
		t.Fatal(fmt.Errorf("test es normal ciSearchAll doc parse response hits error: hits not found"))
	}
	checkColumnName := "firstname"
	for _, v := range hitsArr {
		hitMap := &cbjson.JsonMap{}
		if err := mapstructure.Decode(v, hitMap); err != nil {
			t.Fatal(err)
		}

		sourceMap := hitMap.GetJsonMap("_source")
		columnValue := sourceMap.GetJsonVal(checkColumnName)
		if columnValue == nil {
			t.Fatal(fmt.Errorf("\n test ciSearchAll row column name (`%s`) not found. \n", checkColumnName))
		}
	}

}

func TestBoolQuery(t *testing.T) {
	data := `{
      "query": {"bool" : {"must" : [{"term" : {"age" : 32}}]}}
  }`
	expectValue := 32
	client := InitSearchBegin()
	response, err := client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must not 1: %v\n", string(response.Resp))

	mustMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	mustCounts := mustMap.GetJsonMap("hits").GetJsonArrMap("hits")
	if len(mustCounts) > 0 {
		for _, doc := range mustCounts {
			docField := cast.ToStringMap(doc["_source"])["age"]
			assert.Equal(t, docField, float64(expectValue), "doc get error,method must term actual doc field does not equal to expected")
		}
	} else {
		t.Errorf("doc get error,method must term actual doc size is 0")
	}

	data = `{
      "query": {"bool" : {"must_not" : [{"term" : {"age" : 32}}]}}
  }`

	response, err = client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must 1: %v\n", string(response.Resp))

	mustNotMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	mustNotCounts := mustNotMap.GetJsonMap("hits").GetJsonArrMap("hits")
	if len(mustNotCounts) > 0 {
		for _, doc := range mustNotCounts {
			docField := cast.ToStringMap(doc["_source"])["age"]
			assert.NotEqual(t, docField, float64(expectValue), "doc get error,method must_not term actual doc field does not equal to expected")
		}
	} else {
		t.Errorf("doc get error,method must_not term actual doc size is 0")
	}
	/*
		data = `
		{
		   "query": {
			   "match_all": {}
		   }
		}`

		response, err = client.Search(http.MethodPost, data)
		if err != nil {
			t.Fatal(err)
		}
		log.Info("test es normal ciSearchAll doc result match all: %v\n", string(response.Resp))

		allMap, err := cbjson.ByteToJsonMap(response.Resp)
		if err != nil {
			t.Fatal(err)
		}

		mustNotTotal, _ := mustNotMap.GetJsonMap("hits").GetJsonValIntE("total")
		mustTotal, _ := mustMap.GetJsonMap("hits").GetJsonValIntE("total")
		allTotal, _ := allMap.GetJsonMap("hits").GetJsonValIntE("total")

		if mustTotal+mustNotTotal != allTotal {
			t.Errorf("must not total :%v, must total :%v， all total :%v\n", mustNotTotal, mustTotal, allTotal)
		}
	*/
}

func TestBoolMatchQuery(t *testing.T) {

	data := `{
      "query": {"bool" : {"must" : [{"match" : {"KW_TITLE" : "指标申请"}}]}}
  }`
	client := InitSearchBegin()
	response, err := client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must not 1: %v\n", string(response.Resp))

	mustMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}
	mustCounts, _ := mustMap.GetJsonMap("hits").GetJsonValIntE("total")

	data = `{
	      "query": {"bool" : {"must_not" : [{"match" : {"KW_TITLE" : "指标申请"}}]}}
	  }`

	response, err = client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must 1: %v\n", string(response.Resp))

	mustNotMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}
	mustNotCounts, _ := mustNotMap.GetJsonMap("hits").GetJsonValIntE("total")

	assert.Equal(t, 202, mustCounts+mustNotCounts, "count not same")
}

func TestMatchQuery(t *testing.T) {
	client := InitSearchBegin()

	data := `{"query" : {"term" : {"age" : 32}}}`
	response, err := client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must not 1: %v\n", string(response.Resp))

	age32Map, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	data = `{"query" : {"term" : {"age" : 33}}}`

	response, err = client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must 1: %v\n", string(response.Resp))

	age33Map, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}
	data = `
	{
	   "query": {
		   "match_all": {}
	   }
	}`

	response, err = client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result match all: %v\n", string(response.Resp))

	allMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	age32Total, _ := age32Map.GetJsonMap("hits").GetJsonValIntE("total")
	age33Total, _ := age33Map.GetJsonMap("hits").GetJsonValIntE("total")
	allTotal, _ := allMap.GetJsonMap("hits").GetJsonValIntE("total")

	if age32Total != 10 || age33Total != 11 || allTotal != 202 {
		t.Errorf("age31 total :%v, age 32 total :%v, all total :%v\n", age32Total, age32Total, allTotal)
	}
}

func TestRangeQuery(t *testing.T) {
	data := `
	{
		"query": {
			"range" : {
				"age" : {
					"gte" : 30
				}
			}
		}
	}
	`
	client := InitSearchBegin()
	response, err := client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must not 1: %v\n", string(response.Resp))

	part1Map, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	data = `
	{
		"query": {
			"range" : {
				"age" : {
					"lt" : 30
				}
			}
		}
	}
	`

	response, err = client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must 1: %v\n", string(response.Resp))

	part2Map, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}
	data = `
	{
	   "query": {
		   "match_all": {}
	   }
	}`

	response, err = client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result match all: %v\n", string(response.Resp))

	allMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	part1Total, _ := part1Map.GetJsonMap("hits").GetJsonValIntE("total")
	part2Total, _ := part2Map.GetJsonMap("hits").GetJsonValIntE("total")
	allTotal, _ := allMap.GetJsonMap("hits").GetJsonValIntE("total")

	fmt.Printf("part1 total :%v, part2 total :%v, all total :%v\n", part1Total, part2Total, allTotal)
	if part1Total+part2Total != allTotal {
		t.Errorf("part1 total :%v, part2 total :%v, all total :%v\n", part1Total, part1Total, allTotal)
	}
}

func TestStringRangeQuery(t *testing.T) {
	data := `
	{
		"query": {
			"range" : {
				"firstname" : {
					"gte" : "fffffff"
				}
			}
		}
	}
	`
	client := InitSearchBegin()
	response, err := client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must not 1: %v\n", string(response.Resp))

	part1Map, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	data = `
	{
		"query": {
			"range" : {
				"firstname" : {
					"lt" : "fffffff"
				}
			}
		}
	}
	`

	response, err = client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must 1: %v\n", string(response.Resp))

	part2Map, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}
	data = `
	{
	   "query": {
		   "match_all": {}
	   }
	}`

	response, err = client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result match all: %v\n", string(response.Resp))

	allMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	part1Total, _ := part1Map.GetJsonMap("hits").GetJsonValIntE("total")
	part2Total, _ := part2Map.GetJsonMap("hits").GetJsonValIntE("total")
	allTotal, _ := allMap.GetJsonMap("hits").GetJsonValIntE("total")

	fmt.Printf("part1 total :%v,part2 total :%v, all total :%v\n", part1Total, part2Total, allTotal)
	if part1Total+part2Total != allTotal {
		t.Errorf("part1 total :%v, part2 total :%v, all total :%v\n", part1Total, part1Total, allTotal)
	}
}

func TestTimeRangeQuery(t *testing.T) {
	data := `
	{
		"query": {
			"range" : {
				"insert_time" : {
					"gte" : "2014-08-17T18:51:47.042Z"
				}
			}
		}
	}
	`
	client := InitSearchBegin()
	response, err := client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must not 1: %v\n", string(response.Resp))

	part1Map, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	data = `
	{
		"query": {
			"range" : {
				"insert_time" : {
					"lt" : "2014-08-17T18:51:47.042Z"
				}
			}
		}
	}
	`

	response, err = client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must 1: %v\n", string(response.Resp))

	part2Map, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}
	data = `
	{
	   "query": {
		   "match_all": {}
	   }
	}`

	response, err = client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result match all: %v\n", string(response.Resp))

	allMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	part1Total, _ := part1Map.GetJsonMap("hits").GetJsonValIntE("total")
	part2Total, _ := part2Map.GetJsonMap("hits").GetJsonValIntE("total")
	allTotal, _ := allMap.GetJsonMap("hits").GetJsonValIntE("total")

	fmt.Printf("part1 total :%v, part2 total :%v, all total :%v\n", part1Total, part2Total, allTotal)
	if part1Total+part2Total != allTotal {
		t.Errorf("part1 total :%v, part2 total :%v, all total :%v\n", part1Total, part2Total, allTotal)
	}
}

func TestPhraseRangeQuery(t *testing.T) {
	data := `
	{
    "query": {
        "match_phrase": {
            "email": "alysonirwin@"
        }
    }
	}
	`
	client := InitSearchBegin()
	response, err := client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must not 1: %v\n", string(response.Resp))

	matchPhraseMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	matchPhraseSize, _ := matchPhraseMap.GetJsonMap("hits").GetJsonValIntE("total")

	assert.Equal(t, matchPhraseSize, 1, "size not same")

	fmt.Printf("part1 total :%v\n", matchPhraseSize)
}
