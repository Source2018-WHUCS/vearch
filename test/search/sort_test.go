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
	"github.com/spf13/cast"
	"github.com/tiglabs/log"
	. "github.com/vearch/vearch/test"
	"github.com/vearch/vearch/test/testutil"
	"net/http"
	"testing"
)

func TestIntegerSortAsc(t *testing.T) {
	client := InitSortBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
	"size": 20,
	"sort": [{
		"age": {
			"order": "asc"
		}
	}]
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	//total := m["hits"].(map[string]interface{})["total"]

	docs := m["hits"].(map[string]interface{})["hits"].([]interface{})

	if len(docs) >= 2 {
		minNum := cast.ToInt(cast.ToStringMap(cast.ToStringMap(docs[0])["_source"])["age"])

		for i := 1; i < len(docs); i++ {
			newNum := cast.ToInt(cast.ToStringMap(cast.ToStringMap(docs[i])["_source"])["age"])
			if newNum > minNum {
				break
			} else if newNum == minNum {
				continue
			} else {
				t.Fatal("test integer sort asc result does not in asc order")
				break
			}
		}
	}
	//assert.Equal(t, cast.ToInt(total), testutil.TotalDocCnt, "test integer sort result does not match doc inserted")

}

func TestIntegerSortDesc(t *testing.T) {
	client := InitSortBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
	"size": 20,
	"sort": [{
		"age": {
			"order": "desc"
		}
	}]
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	//total := m["hits"].(map[string]interface{})["total"]

	//assert.Equal(t, cast.ToInt(total), testutil.TotalDocCnt, "test integer sort result does not match doc inserted")
	docs := m["hits"].(map[string]interface{})["hits"].([]interface{})
	if len(docs) >= 2 {
		maxNum := cast.ToInt(cast.ToStringMap(cast.ToStringMap(docs[0])["_source"])["age"])

		for i := 1; i < len(docs); i++ {
			newNum := cast.ToInt(cast.ToStringMap(cast.ToStringMap(docs[i])["_source"])["age"])
			if newNum < maxNum {
				break
			} else if newNum == maxNum {
				continue
			} else {
				t.Fatal("test integer sort asc result does not in desc order")
				break
			}
		}
	}

}

func TestURISortAsc(t *testing.T) {
	client := InitSortBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true&sort=age:asc", "")
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	//total := m["hits"].(map[string]interface{})["total"]

	docs := m["hits"].(map[string]interface{})["hits"].([]interface{})

	//assert.Equal(t, cast.ToInt(total), testutil.TotalDocCnt, "test URI sort result does not match doc inserted")
	if len(docs) >= 2 {
		minNum := cast.ToInt(cast.ToStringMap(cast.ToStringMap(docs[0])["_source"])["age"])

		for i := 1; i < len(docs); i++ {
			newNum := cast.ToInt(cast.ToStringMap(cast.ToStringMap(docs[i])["_source"])["age"])
			if newNum > minNum {
				break
			} else if newNum == minNum {
				continue
			} else {
				t.Fatal("test URI integer sort asc result does not in asc order")
				break
			}
		}
	}
}

func TestURISortDesc(t *testing.T) {
	client := InitSortBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true&sort=age:desc", "")
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	//total := m["hits"].(map[string]interface{})["total"]

	docs := m["hits"].(map[string]interface{})["hits"].([]interface{})

	//assert.Equal(t, cast.ToInt(total), testutil.TotalDocCnt, "test URI sort result does not match doc inserted")
	if len(docs) >= 2 {
		maxNum := cast.ToInt(cast.ToStringMap(cast.ToStringMap(docs[0])["_source"])["age"])

		for i := 1; i < len(docs); i++ {
			newNum := cast.ToInt(cast.ToStringMap(cast.ToStringMap(docs[i])["_source"])["age"])
			if newNum < maxNum {
				break
			} else if newNum == maxNum {
				continue
			} else {
				t.Fatal("test URI integer sort desc result does not in desc order")
				break
			}
		}
	}
}

func TestFloatSortAsc(t *testing.T) {
	client := InitSortBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
	"size": 20,
	"sort": [{
		"float_value": {
			"order": "asc"
		}
	}]
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	//total := m["hits"].(map[string]interface{})["total"]

	docs := m["hits"].(map[string]interface{})["hits"].([]interface{})

	if len(docs) >= 2 {
		minNum := cast.ToFloat64(cast.ToStringMap(cast.ToStringMap(docs[0])["_source"])["float_value"])

		for i := 1; i < len(docs); i++ {
			newNum := cast.ToFloat64(cast.ToStringMap(cast.ToStringMap(docs[i])["_source"])["float_value"])
			if newNum > minNum {
				break
			} else if newNum == minNum {
				continue
			} else {
				t.Fatal("test float sort asc result does not in asc order")
				break
			}
		}
	}

}

func TestFloatSortDesc(t *testing.T) {
	client := InitSortBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
	"size": 20,
	"sort": [{
		"float_value": {
			"order": "desc"
		}
	}]
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	//total := m["hits"].(map[string]interface{})["total"]

	docs := m["hits"].(map[string]interface{})["hits"].([]interface{})

	if len(docs) >= 2 {
		maxNum := cast.ToFloat64(cast.ToStringMap(cast.ToStringMap(docs[0])["_source"])["float_value"])

		for i := 1; i < len(docs); i++ {
			newNum := cast.ToFloat64(cast.ToStringMap(cast.ToStringMap(docs[i])["_source"])["float_value"])
			if newNum < maxNum {
				break
			} else if newNum == maxNum {
				continue
			} else {
				t.Fatal("test float sort desc result does not in desc order")
				break
			}
		}
	}

}

func TestStringSortAsc(t *testing.T) {
	client := InitSortBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
	"size": 20,
	"sort": [{
		"title": {
			"order": "asc"
		}
	}]
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)
	//total := m["hits"].(map[string]interface{})["total"]
	docs := m["hits"].(map[string]interface{})["hits"].([]interface{})

	if len(docs) >= 2 {
		//fmt.Println(cast.ToStringMap(cast.ToStringMap(docs[0])["_source"])["title"])
		minNum := cast.ToString(cast.ToStringMap(cast.ToStringMap(docs[0])["_source"])["title"])

		for i := 1; i < len(docs); i++ {
			newNum := cast.ToString(cast.ToStringMap(cast.ToStringMap(docs[i])["_source"])["title"])
			if newNum > minNum {
				break
			} else if newNum == minNum {
				continue
			} else {
				t.Fatal("test string sort asc result does not in asc order")
				break
			}
		}
	}

}

func TestStringSortDesc(t *testing.T) {
	client := InitSortBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
	"size": 20,
	"sort": [{
		"title": {
			"order": "desc"
		}
	}]
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)
	//total := m["hits"].(map[string]interface{})["total"]
	docs := m["hits"].(map[string]interface{})["hits"].([]interface{})

	if len(docs) >= 2 {
		//fmt.Println(cast.ToStringMap(cast.ToStringMap(docs[0])["_source"])["title"])
		minNum := cast.ToString(cast.ToStringMap(cast.ToStringMap(docs[0])["_source"])["title"])

		for i := 1; i < len(docs); i++ {
			newNum := cast.ToString(cast.ToStringMap(cast.ToStringMap(docs[i])["_source"])["title"])
			if newNum < minNum {
				break
			} else if newNum == minNum {
				continue
			} else {
				t.Fatal("test string sort desc result does not in desc order")
				break
			}
		}
	}

}

func TestDateSortAsc(t *testing.T) {
	client := InitSortBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
	"size": 20,
	"sort": [{
		"birthday": {
			"order": "asc"
		}
	}]
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)
	//total := m["hits"].(map[string]interface{})["total"]
	docs := m["hits"].(map[string]interface{})["hits"].([]interface{})

	if len(docs) >= 2 {
		fmt.Println(cast.ToStringMap(cast.ToStringMap(docs[0])["_source"])["birthday"])
		minNum := cast.ToString(cast.ToStringMap(cast.ToStringMap(docs[0])["_source"])["birthday"])

		for i := 1; i < len(docs); i++ {
			newNum := cast.ToString(cast.ToStringMap(cast.ToStringMap(docs[i])["_source"])["birthday"])
			if newNum > minNum {
				break
			} else if newNum == minNum {
				continue
			} else {
				t.Fatal("test date sort asc result does not in asc order")
				break
			}
		}
	}

}
