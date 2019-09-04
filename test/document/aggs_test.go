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
	"fmt"
	. "github.com/vearch/vearch/test"
	"github.com/spf13/cast"
	"github.com/vearch/vearch/proto"
	"github.com/vearch/vearch/test/testutil"
	"github.com/vearch/vearch/util/assert"
	"github.com/vearch/vearch/util/cbjson"
	"github.com/tiglabs/log"
	"net/http"
	"testing"
	"time"
)

//template for agg
//func TestXXXX(t *testing.T) {
//
//	client := InitAggBegin()
//	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", ``)
//	if err != nil {
//		t.Fatal(err)
//	}
//	log.Info(string(response.Resp))
//	m := testutil.Json2map(response.Resp)
//}

func TestIpRangeAgg(t *testing.T) {
	client := InitAggBegin()
	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
    "size": 0,
    "query": {
        "match_all": {
            "boost": 1.0
        }
    },
    "aggregations": {
        "ip_ranges": {
            "ip_range": {
                "field": "ip",
                "ranges": [
                    {
                        "to": "80.127.0.0"
                    }, 
                    {
                        "from": "80.127.0.0",
                        "to": "160.127.0.0"
                    },
                    {
                        "from": "160.127.0.0"
                    }
                ],
                "keyed":true
            }
        }
    }
}
	`)
	if err != nil {
		t.Fatal(err)
	}
	log.Info(string(response.Resp))
	m := testutil.Json2map(response.Resp)
	buckets := m["aggregations"].(map[string]interface{})["ip_range#ip_ranges"].(map[string]interface{})["buckets"]
	fmt.Println(buckets)
	fmt.Println(cast.ToInt(buckets.(map[string]interface{})["*-80.127.0.0"].(map[string]interface{})["doc_count"]))
	fmt.Println(cast.ToInt(buckets.(map[string]interface{})["80.127.0.0-160.127.0.0"].(map[string]interface{})["doc_count"]))
	fmt.Println(cast.ToInt(buckets.(map[string]interface{})["160.127.0.0-*"].(map[string]interface{})["doc_count"]))
	if cast.ToInt(buckets.(map[string]interface{})["*-80.127.0.0"].(map[string]interface{})["doc_count"]) != 81 ||
		cast.ToInt(buckets.(map[string]interface{})["80.127.0.0-160.127.0.0"].(map[string]interface{})["doc_count"]) != 80 ||
		cast.ToInt(buckets.(map[string]interface{})["160.127.0.0-*"].(map[string]interface{})["doc_count"]) != 39 {
		t.Fatal(fmt.Errorf("ip range agg buckets count does not match doc inserted"))
	}

	total := m["hits"].(map[string]interface{})["total"]
	//fmt.Println(total)
	if cast.ToInt(total) != testutil.TotalDocCnt {
		t.Fatal(fmt.Errorf("ip range agg result does not match doc inserted"))
	}
}

func TestDateRangeAgg(t *testing.T) {
	client := InitAggBegin()
	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
    "size": 0,
    "query": {
        "match_all": {
            "boost": 1.0
        }
    },
    "aggregations": {
        "birthday": {
            "date_range": {
                "field": "birthday",
                "ranges": [
                    {
                        "to": "now-10M/M"
                    }, 
                    {
                        "from": "now-10M/M"
                    }
                ]
            }
        }
    }
}
	`)
	if err != nil {
		t.Fatal(err)
	}
	log.Info(string(response.Resp))
	m := testutil.Json2map(response.Resp)

	buckets := m["aggregations"].(map[string]interface{})["date_range#birthday"].(map[string]interface{})["buckets"].([]interface{})
	if len(buckets) != 2 {
		t.Fatal(fmt.Errorf("xxxdate range agg buckets count does not match doc inserted"))
	}
	b1 := buckets[0]
	b2 := buckets[1]
	fmt.Println("b1 doc_count: ", cast.ToInt(b1.(map[string]interface{})["doc_count"]))
	fmt.Println("b1 key: ", cast.ToString(b1.(map[string]interface{})["key"]))
	fmt.Println("b1 to_as_string: ", cast.ToString(b1.(map[string]interface{})["to_as_string"]))
	fmt.Println("b2 doc_count: ", cast.ToInt(b2.(map[string]interface{})["doc_count"]))
	fmt.Println("b2 key: ", cast.ToString(b2.(map[string]interface{})["key"]))
	fmt.Println("b2 from_as_string: ", cast.ToString(b2.(map[string]interface{})["from_as_string"]))
	keyTag := addMonth(time.Now(), 10).Format("2006-01-02 15:04:05.000")

	assert.Equal(t, cast.ToString(b1.(map[string]interface{})["key"]), "*-"+keyTag, "key equal")
	assert.True(t, cast.ToString(b1.(map[string]interface{})["to_as_string"]) == keyTag)
	assert.True(t, cast.ToString(b2.(map[string]interface{})["key"]) == keyTag+"-*")
	assert.True(t, cast.ToString(b2.(map[string]interface{})["from_as_string"]) == keyTag)

	total := m["hits"].(map[string]interface{})["total"]
	//fmt.Println(total)
	if cast.ToInt(total) != testutil.TotalDocCnt {
		t.Fatal(fmt.Errorf("***date range agg result does not match doc inserted"))
	}
}

func addMonth(t time.Time, months int) time.Time {
	year := t.Year()
	month := t.Month()
	if months >= int(month) {
		year -= 1
		month = month + time.Month(12-months)
	}
	return time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
}

func TestDateHistogramMonth(t *testing.T) {

	client := InitAggBegin()
	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
		{"size":0,"query":{"match_all":{"boost":1.0}},"aggregations":{"birthday":{"date_histogram":{"field":"birthday", "interval":"month", "offset":"+3d"}}}} 
	`)
	if err != nil {
		t.Fatal(err)
	}
	log.Info(string(response.Resp))
	m := testutil.Json2map(response.Resp)

	buckets := m["aggregations"].(map[string]interface{})["date_histogram#birthday"].(map[string]interface{})["buckets"].([]interface{})

	if len(buckets) != testutil.TotalDocCnt || cast.ToString(buckets[0].(map[string]interface{})["key_as_string"]) != "1920-04-04 00:00:00.000" ||
		cast.ToInt(buckets[0].(map[string]interface{})["doc_count"]) != 1 {
		t.Fatal(fmt.Errorf("histogram agg buckets count does not match doc inserted"))
	}

	total := m["hits"].(map[string]interface{})["total"]
	if cast.ToInt(total) != testutil.TotalDocCnt {
		t.Fatal(fmt.Errorf("histogram agg result does not match doc inserted"))
	}
}

func TestDateHistogramYear(t *testing.T) {
	if 1 == 1 {
		fmt.Println("haihua haihua")
	}
	client := InitAggBegin()
	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
		{"size":0,"query":{"match_all":{"boost":1.0}},"aggregations":{"birthday":{"date_histogram":{"field":"birthday", "interval":"year", "offset":"-3d"}}}} 
	`)
	if err != nil {
		t.Fatal(err)
	}
	log.Info(string(response.Resp))
	m := testutil.Json2map(response.Resp)

	buckets := m["aggregations"].(map[string]interface{})["date_histogram#birthday"].(map[string]interface{})["buckets"].([]interface{})
	//fmt.Println(len(buckets))
	//fmt.Println(cast.ToString(buckets[0].(map[string]interface{})["key_as_string"]))
	//fmt.Println(cast.ToInt(buckets[0].(map[string]interface{})["doc_count"]))
	if len(buckets) != 100 || cast.ToString(buckets[0].(map[string]interface{})["key_as_string"]) != "1919-12-29 00:00:00.000" ||
		cast.ToInt(buckets[0].(map[string]interface{})["doc_count"]) != 2 {
		t.Fatal(fmt.Errorf("histogram agg buckets count does not match doc inserted"))
	}

	total := m["hits"].(map[string]interface{})["total"]
	//fmt.Println(total)
	if cast.ToInt(total) != testutil.TotalDocCnt {
		t.Fatal(fmt.Errorf("histogram agg result does not match doc inserted"))
	}
}

func TestHistogramMinDocCntAgg(t *testing.T) {
	client := InitAggBegin()
	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
		{"size":0,"query":{"match_all":{"boost":1.0}},"aggregations":{"age":{"histogram":{"field":"age", "interval":20}}}}
	`)
	if err != nil {
		t.Fatal(err)
	}
	log.Info(string(response.Resp))
	m := testutil.Json2map(response.Resp)

	buckets := m["aggregations"].(map[string]interface{})["histogram#age"].(map[string]interface{})["buckets"].([]interface{})
	if len(buckets) != 5 {
		t.Fatal(fmt.Errorf("histogram agg buckets count does not match doc inserted"))
	}
	for _, b := range buckets {
		if cast.ToInt(b.(map[string]interface{})["doc_count"]) != 40 {
			t.Fatal(fmt.Errorf("histogram agg doc count in bucket does not match doc inserted"))
		}
	}

	total := m["hits"].(map[string]interface{})["total"]
	//fmt.Println(total)
	if cast.ToInt(total) != testutil.TotalDocCnt {
		t.Fatal(fmt.Errorf("histogram agg result does not match doc inserted"))
	}
}

func TestCardinalityAgg(t *testing.T) {
	client := InitAggBegin()
	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
		{"size":0,"query":{"match_all":{"boost":1.0}},"aggregations":{"age":{"cardinality":{"field":"age"}}}} 
	`)
	if err != nil {
		t.Fatal(err)
	}
	log.Info(string(response.Resp))
	m := testutil.Json2map(response.Resp)

	cardinality, ok := m["aggregations"].(map[string]interface{})["cardinality#age"].(map[string]interface{})["value"]
	if !ok || cast.ToInt(cardinality) != 100 {
		t.Fatal(fmt.Errorf("cardinality agg sum result does not match doc inserted"))
	}

	total := m["hits"].(map[string]interface{})["total"]
	//fmt.Println(total)
	if cast.ToInt(total) != testutil.TotalDocCnt {
		t.Fatal(fmt.Errorf("cardinality agg result does not match doc inserted"))
	}
}

func TestMinAgg(t *testing.T) {
	client := InitAggBegin()
	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
		{"size":0,"query":{"match_all":{"boost":1.0}},"aggregations":{"age":{"min":{"field":"age"}}}} 
	`)
	if err != nil {
		t.Fatal(err)
	}
	log.Info(string(response.Resp))
	m := testutil.Json2map(response.Resp)

	max, ok := m["aggregations"].(map[string]interface{})["min#age"].(map[string]interface{})["value"]
	if !ok || cast.ToInt(max) != 0 {
		t.Fatal(fmt.Errorf("min agg sum result does not match doc inserted"))
	}

	total := m["hits"].(map[string]interface{})["total"]
	//fmt.Println(total)
	if cast.ToInt(total) != testutil.TotalDocCnt {
		t.Fatal(fmt.Errorf("min agg result does not match doc inserted"))
	}
}

func TestMaxAggs(t *testing.T) {
	client := InitAggBegin()
	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
		{"size":0,"query":{"match_all":{"boost":1.0}},"aggregations":{"age":{"max":{"field":"age"}}}} 
	`)
	if err != nil {
		t.Fatal(err)
	}
	log.Info(string(response.Resp))
	m := testutil.Json2map(response.Resp)

	max, ok := m["aggregations"].(map[string]interface{})["max#age"].(map[string]interface{})["value"]
	if !ok || cast.ToInt(max) != 99 {
		t.Fatal(fmt.Errorf("max agg sum result does not match doc inserted"))
	}

	total := m["hits"].(map[string]interface{})["total"]
	//fmt.Println(total)
	if cast.ToInt(total) != testutil.TotalDocCnt {
		t.Fatal(fmt.Errorf("max agg result does not match doc inserted"))
	}
}

func TestStatsAgg(t *testing.T) {
	client := InitAggBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
		{"size":0,"query":{"match_all":{"boost":1.0}},"aggregations":{"age":{"stats":{"field":"age"}}}}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	buckets := m["aggregations"].(map[string]interface{})["stats#age"].(map[string]interface{})
	if len(buckets) != 5 {
		t.Fatal(fmt.Errorf("stats agg result does not match doc inserted"))
	}
	val, ok := buckets["avg"]
	if !ok || cast.ToFloat32(val) != 49.5 {
		t.Fatal(fmt.Errorf("stats agg avg result does not match doc inserted"))
	}
	val, ok = buckets["count"]
	if !ok || cast.ToInt(val) != testutil.TotalDocCnt {
		t.Fatal(fmt.Errorf("stats agg count result does not match doc inserted"))
	}
	val, ok = buckets["max"]
	if !ok || cast.ToInt(val) != 99 {
		t.Fatal(fmt.Errorf("stats agg max result does not match doc inserted"))
	}
	val, ok = buckets["min"]
	if !ok || cast.ToInt(val) != 0 {
		t.Fatal(fmt.Errorf("stats agg min result does not match doc inserted"))
	}
	val, ok = buckets["sum"]
	if !ok || cast.ToInt(val) != 9900 {
		t.Fatal(fmt.Errorf("stats agg sum result does not match doc inserted"))
	}

	total := m["hits"].(map[string]interface{})["total"]
	if cast.ToInt(total) != testutil.TotalDocCnt {
		t.Fatal(fmt.Errorf("sum agg result does not match doc inserted"))
	}

}

func TestCountAgg(t *testing.T) {
	client := InitAggBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
    "size":0,
    "query":{
        "match_all":{
            "boost":1
        }
    },
    "aggregations":{
        "age":{
            "value_count":{
                "field":"age"
            }
        }
    }
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	count := m["aggregations"].(map[string]interface{})["value_count#age"].(map[string]interface{})["count"]
	missing := m["aggregations"].(map[string]interface{})["value_count#age"].(map[string]interface{})["missing"]
	//fmt.Println(count)
	//fmt.Println(missing)
	if cast.ToInt(count) != 200 || cast.ToInt(missing) != 0 {
		t.Fatal(fmt.Errorf("count agg result does not match doc inserted"))
	}

	total := m["hits"].(map[string]interface{})["total"]
	//fmt.Println(total)
	if cast.ToInt(total) != 200 {
		t.Fatal(fmt.Errorf("sum agg result does not match doc inserted"))
	}

}

func TestSumAgg(t *testing.T) {
	client := InitAggBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
 {
    "size":0,
    "query":{
        "match_all":{
            "boost":1
        }
    },
    "aggregations":{
        "age":{
            "sum":{
                "field":"age"
            }
        }
    }
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	val := m["aggregations"].(map[string]interface{})["sum#age"].(map[string]interface{})["value"]
	if cast.ToInt(val) != 9900 {
		fmt.Println(fmt.Errorf("sum agg result does not match doc inserted"))
	}

	total := m["hits"].(map[string]interface{})["total"]
	if cast.ToInt(total) != testutil.TotalDocCnt {
		t.Fatal(fmt.Errorf("sum agg result does not match doc inserted"))
	}

}

func TestAvgAgg(t *testing.T) {
	client := InitAggBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
 {
    "size":0,
    "query":{
        "match_all":{
            "boost":1
        }
    },
    "aggregations":{
        "age":{
            "avg":{
                "field":"age"
            }
        }
    }
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	val := m["aggregations"].(map[string]interface{})["avg#age"].(map[string]interface{})["value"]
	if cast.ToFloat32(val) != 49.5 {
		t.Fatal(fmt.Errorf("avg agg result does not match doc inserted"))
	}

	total := m["hits"].(map[string]interface{})["total"]
	if cast.ToInt(total) != testutil.TotalDocCnt {
		t.Fatal(fmt.Errorf("avg agg result does not match doc inserted"))
	}

}

func TestTermsOrder(t *testing.T) {
	client := InitAggBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
   "aggregations": {
      "agg1": {
         "terms": {
            "field": "age",
            "size": 200,
            "min_doc_count": 1,
            "shard_min_doc_count": 0,
            "show_term_doc_count_error": false,
            "order": {
                  "_term": "desc"
			}
         }
      }
   }
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	bucket := m["aggregations"].(map[string]interface{})["sterms#agg1"].(map[string]interface{})["buckets"].([]interface{})

	if len(bucket) != 100 || cast.ToInt(bucket[0].(map[string]interface{})["key"]) != 99 ||
		cast.ToInt(bucket[1].(map[string]interface{})["key"]) != 98 ||
		cast.ToInt(bucket[2].(map[string]interface{})["key"]) != 97 {
		t.Fatal(fmt.Errorf("terms order agg result does not match doc inserted"))
	}

}

func TestTermsOrders(t *testing.T) {
	client := InitAggBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
   "aggregations": {
      "agg1": {
         "terms": {
            "field": "age",
            "size": 200,
            "min_doc_count": 1,
            "shard_min_doc_count": 0,
            "show_term_doc_count_error": false,
            "order": [
				{
                  "_term": "desc"
               },
               {
                  "_key": "asc"
               }

            ]
         }
      }
   }
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	arr := m["aggregations"].(map[string]interface{})["sterms#agg1"].(map[string]interface{})["buckets"].([]interface{})

	if cast.ToInt(arr[0].(map[string]interface{})["key"]) != 99 ||
		cast.ToInt(arr[99].(map[string]interface{})["key"]) != 0 {
		t.Error(fmt.Errorf("terms orders agg result does not match doc inserted"))
	}

}

func TestTermsCount(t *testing.T) {
	client := InitAggBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
    "size": 0,
    "query": {
        "match_all": {
            "boost": 1
        }
    },
    "aggs": {
		"age": {
			"terms": {
				"field": "age"
			},
			"aggs": {
	            "avg_name": {
	               "value_count": {
	                  "field": "age"
	               }
	            }
	         }
		}
	}
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	bucket := m["aggregations"].(map[string]interface{})["sterms#age"].(map[string]interface{})["buckets"].([]interface{})

	if len(bucket) != 20 || cast.ToInt(bucket[0].(map[string]interface{})["doc_count"]) != 2 ||
		cast.ToInt(bucket[0].(map[string]interface{})["value_count#avg_name"].(map[string]interface{})["count"]) != 2 {
		t.Fatal(fmt.Errorf("terms count agg result does not match doc inserted"))
	}

}

func TestTermsNestedCount(t *testing.T) {

	client := InitAggBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
	"size": 0,
	"query": {
		"match_all": {
			"boost": 1
		}
	},
	"aggs": {
		"age": {
			"terms": {
				"field": "age",
				"size":3
			},
			"aggs": {
				"name":{
					"terms": {
						"field": "name"
					},
					"aggs":{
						"title": {
							"terms": {
								"field": "title"
							},
							"aggs": {
								"avg_name": {
									"avg": {
										"field": "age"
									}
								}
							}
						}
					}
				}
			}
		}
	}
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	arr := m["aggregations"].(map[string]interface{})["sterms#age"].(map[string]interface{})["buckets"].([]interface{})
	//fmt.Println(len(arr) == 3)

	name := arr[2].(interface{})
	//fmt.Println(name.(map[string]interface{})["doc_count"])
	arr1 := name.(map[string]interface{})["sterms#name"].(map[string]interface{})["buckets"].([]interface{})
	if len(arr) != 3 || cast.ToInt(name.(map[string]interface{})["doc_count"]) != 2 ||
		cast.ToInt(arr1[0].(map[string]interface{})["sterms#title"].(map[string]interface{})["buckets"].([]interface{})[0].(map[string]interface{})["doc_count"]) != 2 {
		t.Fatal(fmt.Errorf("term nested count agg result does not match doc inserted"))
	}

}

func TestTermsAvg(t *testing.T) {

	client := InitAggBegin()

	response, err := client.SearchByParam(http.MethodPost, "typed_keys=true", `
{
    "size": 0, 
    "query": {
        "match_all": {
            "boost": 1
        }
    }, 
    "aggs": {
		"age": {
			"terms": {
				"field": "age",
				"order":{"_term":"asc"},
				"size":50
			},
			"aggs": {
	            "avg_name": {
	               "avg": {
	                  "field": "age"
	               }
	            }
	         }
		}
	}
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	log.Info(string(response.Resp))

	m := testutil.Json2map(response.Resp)

	arr := m["aggregations"].(map[string]interface{})["sterms#age"].(map[string]interface{})["buckets"]

	for i, a := range arr.([]interface{}) {
		if !(cast.ToString(a.(map[string]interface{})["key"]) == cast.ToString(i)) {
			//fmt.Println(a)
		}
	}

	if len(arr.([]interface{})) != 50 {
		t.Fatal(fmt.Errorf("term avg agg result does not match doc inserted"))
	}

}

func TestRangeAvg(t *testing.T) {
	client := InitAggBegin()
	response, err := client.Search(http.MethodPost, `
{
    "size": 0, 
    "query": {
        "match_all": {
            "boost": 1
        }
    }, 
    "aggregations": {
        "age_term": {
            "range" : {
                "field" : "age",
                "ranges" : [
                    { "to" : 20.0 },
                    { "from" : 20.0, "to" : 80.0 },
                    { "from" : 80 }
                ]
            },
            "aggs": {
	            "avg_name": {
	               "avg": {
	                  "field": "age"
	               }
	            }
	        }
        }
    }
}
	`)
	if err != nil {
		t.Fatal(err)
	}

	m := testutil.Json2map(response.Resp)

	arr := m["aggregations"].(map[string]interface{})["age_term"].(map[string]interface{})["buckets"].([]interface{})

	if arr[0].(map[string]interface{})["key"] != "*-20" || cast.ToInt(arr[0].(map[string]interface{})["doc_count"]) != 40 ||
		arr[1].(map[string]interface{})["key"] != "20-80" || cast.ToInt(arr[1].(map[string]interface{})["doc_count"]) != 120 ||
		arr[2].(map[string]interface{})["key"] != "80-*" || cast.ToInt(arr[2].(map[string]interface{})["doc_count"]) != 40 {
		t.Fatal(fmt.Errorf("range avg agg result does not match doc inserted %s", string(response.Resp)))
	}

}

func TestMaxAgg(t *testing.T) {
	data := `
{
	"aggs" : {
       "max_age" : {
           "max" : {
               "field" : "age"
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

	log.Info("test es normal ciSearchAll doc result: %v", string(response.Resp))

	jsonMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	aggsMap := jsonMap.GetJsonMap("aggregations")
	if aggsMap == nil {
		status, err := jsonMap.GetJsonValIntE("status")
		if err != nil {
			panic(err)
		}
		if status != pkg.ERRCODE_SUCCESS {
			panic(jsonMap.GetJsonValStringOrDefault("error", ""))
		}
	}

	maxAge, err := aggsMap.GetJsonMap("max_age").GetJsonValIntE("value")
	if err != nil {
		t.Fatal(err)
	}
	if maxAge != 40 {
		t.Fatal(fmt.Errorf("\n test es normal agg max err: max is not equals 40"))
	}

}

//
//func filterAgg(routerAddr string) error {
//	if 1 == 1 { //TODO mofei add agg
//		return nil
//	}
//
//	defer func() {
//		fmt.Println("filter agg finish")
//	}()
//	result, status := testutil.PostWithHeader("http://"+routerAddr+"/"+testutil.C().DbName+"/"+testutil.C().SpaceName+"/_search?size=0&typed_keys=true", strings.NewReader(`
//{
//    "size": 0,
//    "query": {
//        "match_all": {
//            "boost": 1.0
//        }
//    },
//    "aggregations": {
//        "marriaged" : {
//            "filter" : {
//                    "match" : { "content" : "结婚"   }
//            }
//        }
//    }
//}
//	`))
//
//	if status != "200 OK" {
//		return fmt.Errorf("filter agg query failed, result: %s", string(result))
//	}
//	m := testutil.Json2map(result)
//
//	bucket := m["aggregations"].(map[string]interface{})["filter#marriaged"]
//	//fmt.Println(bucket)
//	if cast.ToInt(bucket.(map[string]interface{})["count"]) != testutil.TotalDocCnt {
//		return fmt.Errorf("filter agg buckets count does not match doc inserted")
//	}
//
//	total := m["hits"].(map[string]interface{})["total"]
//	//fmt.Println(total)
//	if cast.ToInt(total) != testutil.TotalDocCnt {
//		return fmt.Errorf("filter agg result does not match doc inserted")
//	}
//	return nil
//}
//
//func filtersAgg(routerAddr string) error {
//
//	if 1 == 1 { //TODO mofei add agg
//		return nil
//	}
//
//	defer func() {
//		fmt.Println("filters agg finish")
//	}()
//	result, status := testutil.PostWithHeader("http://"+routerAddr+"/"+testutil.C().DbName+"/"+testutil.C().SpaceName+"/_search?size=0&typed_keys=true", strings.NewReader(`
//{
//    "size": 0,
//    "query": {
//        "match_all": {
//            "boost": 1.0
//        }
//    },
//    "aggregations": {
//        "marriaged" : {
//            "filter" : {
//                    "match" : { "content" : "结婚"   }
//            }
//        }
//    }
//}
//	`))
//
//	if status != "200 OK" {
//		return fmt.Errorf("filters agg query failed, result: %s", string(result))
//	}
//	m := testutil.Json2map(result)
//
//	bucket := m["aggregations"].(map[string]interface{})["filter#marriaged"]
//	//fmt.Println(bucket)
//	if cast.ToInt(bucket.(map[string]interface{})["count"]) != testutil.TotalDocCnt {
//		return fmt.Errorf("filter agg buckets count does not match doc inserted")
//	}
//
//	total := m["hits"].(map[string]interface{})["total"]
//	//fmt.Println(total)
//	if cast.ToInt(total) != testutil.TotalDocCnt {
//		return fmt.Errorf("filter agg result does not match doc inserted")
//	}
//	return nil
//}
