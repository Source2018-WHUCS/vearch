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
	. "github.com/tiglabs/baudengine/test"
	"github.com/tiglabs/baudengine/util/assert"
	"github.com/tiglabs/baudengine/util/cbjson"
	"github.com/tiglabs/log"
	"net/http"
	"testing"
)

func TestLogbookRangTime1(t *testing.T) {
	client := InitLogbookBegin()

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
	if 209 != allTotal {
		t.Errorf("range time total not 209  , total: %v\n", allTotal)
	}
}

func TestLogbookRangTime2(t *testing.T) {
	client := InitLogbookBegin()

	data := `
{
	"range":{
		"time":{
			"format":"epoch_millis",
			"gte":1550237840000,
			"lte":1553237850000
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
	if 385 != allTotal {
		t.Errorf("term query total not 385, total: %v\n", allTotal)
	}
}

func TestLogbookTermAppname(t *testing.T) {
	client := InitLogbookBegin()

	data := `
	{
        "query": {
            "term" : {
                "group" : "备件库使用"
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

	if 3 != allTotal {
		t.Errorf("term query total not 3, total: %v\n", allTotal)
	}
}

func TestLogbookTermAppname2(t *testing.T) {
	client := InitLogbookBegin()

	data := `
	{
        "query": {
            "term" : {
                "group" : "jsf马驹桥分组"
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
	if 7 != allTotal {
		t.Errorf("term query total not 7, total: %v", allTotal)
	}
}

func TestLogbookMatchAppname1(t *testing.T) {
	client := InitLogbookBegin()

	data := `
{
    "query":{
        "match":{
            "msg":{
                "query":"业务标示ID",
                "type":"phrase"
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
	if 6 != allTotal {
		t.Errorf("term query total not 53, total: %v\n", allTotal)
	}
}

func TestLogbookMatchFile(t *testing.T) {

	data1 := `
	{
		"query":{
			"match":{
				"file":{
					"query":"/export/Logs/calc.stock.eclp.jd.com/service.log",
					"type":"phrase"
				}
			}
		}
	}
		`

	data2 := `
{
	"query":{
		"match":{
			"file":{
				"query":"/export/Logs/calc.stock.eclp.jd.com/service.log",
				"type":"phrase",
				"analyzer":"keyword"
			}
		}
	}
}
	`

	data3 := `
	{
		"query":{
			"term":{
				"file":"/export/Logs/calc.stock.eclp.jd.com/service.log"
			}
		}
	}
		`

	data4 := `
	{
		"query":{
			"match_phrase":{
				"file":{
					"query":"/export/Logs/calc.stock.eclp.jd.com/service.log"
				}
			}
		}
	}
		`

	datas := []string{data1, data2, data3, data4}

	for _, data := range datas {
		client := InitLogbookBegin()

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
		if 4 != allTotal {
			t.Errorf("term query total mismatch: expect 4 actual %v\n", allTotal)
		}
	}
}

func TestLogbookMatchIp(t *testing.T) {
	client := InitLogbookBegin()

	data := `
{
	"query":{
		"match":{
			"ip":{
				"query":"10.174.67.178",
				"type":"phrase"
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
	if 3 != allTotal {
		t.Errorf("term query total not 3, total: %v\n", allTotal)
	}
}

func TestLogbookQueryString(t *testing.T) {
	client := InitLogbookBegin()

	data := `
{
	"query":{
		"query_string":{
			"query":"msg:exception",
			"analyze_wildcard":true
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
	if 5 != allTotal {
		t.Errorf("term query total not 4, total: %v\n", allTotal)
	}
}

func TestLogbookQueryStringStar(t *testing.T) {
	client := InitLogbookBegin()

	data := `
{
	"query":{
		"query_string":{
			"query":"*",
			"analyze_wildcard":true
		}
	}
}
	`

	search, err := client.Search(http.MethodGet, "{}")
	if err != nil {
		t.Fatal(err)
	}
	allMap, err := cbjson.ByteToJsonMap(search.Resp)
	if err != nil {
		t.Fatal(err)
	}
	matchAllCount, _ := allMap.GetJsonMap("hits").GetJsonValIntE("total")

	response, err := client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal TestLogbook doc result : %v\n", string(response.Resp))
	allMap, err = cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}
	allTotal, _ := allMap.GetJsonMap("hits").GetJsonValIntE("total")

	assert.Equal(t, matchAllCount, allTotal, "term query total not equal")
}

func TestLogbookHistory(t *testing.T) {
	client := InitLogbookBegin()

	data := `
{
   "sort":[
       {
           "time":{
               "order":"desc",
               "unmapped_type":"long"
           }
       }
   ],
   "highlight":{
       "pre_tags":[
           "@highlighted-field@"
       ],
       "post_tags":[
           "@/highlighted-field@"
       ],
       "fields":{
           "msg":{

           }
       },
       "require_field_match":false,
       "fragment_size":2147483647
   },
   "query":{
       "filtered":{
           "filter":{
               "bool":{
                   "must":[
                       {
                           "query":{
                               "match":{
                                   "app-name":{
                                       "query":"promise-calendar",
                                       "type":"phrase"
                                   }
                               }
                           }
                       },
                       {
                           "range":{
                               "time":{
                                   "format":"epoch_millis",
                                   "gte":1550237840000,
                                   "lte":1553237850000
                               }
                           }
                       }
                   ],
                   "should":[
                       {
                           "bool":{
                               "must":[
                                   {
                                       "query":{
                                           "match":{
                                               "file":{
                                                   "query":"/export/Logs/calc.stock.eclp.jd.com/service.log",
                                                   "type":"phrase"
                                               }
                                           }
                                       }
                                   }
                               ],
                               "should":[
                                   {
                                       "query":{
                                           "match":{
                                               "ip":{
                                                   "query":"10.174.67.178",
                                                   "type":"phrase"
                                               }
                                           }
                                       }
                                   }
                               ]
                           }
                       }
                   ],
                   "must_not":[

                   ]
               }
           },
           "query":{
               "query_string":{
                   "query":"msg:exception",
                   "analyze_wildcard":true
               }
           }
       }
   },
   "aggs":{
       "histogram":{
           "date_histogram":{
               "field":"time",
               "format":"yyyy-MM-dd HH:mm:ss",
               "time_zone":"+08:00",
               "interval":"38s",
               "min_doc_count":0,
               "extended_bounds":{
                   "min":1552354269000,
                   "max":1552355169000
               }
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
	if 2 != allTotal {
		t.Errorf("term query total not 1, total: %v\n", allTotal)
	}
}

func TestLogbookHistoryContext(t *testing.T) {
	client := InitLogbookBegin()

	data := ` 
{
    "sort":[
        {
            "time":{
                "order":"asc",
                "unmapped_type":"long"
            }
        },
        {
            "microsecond":{
                "order":"asc",
                "unmapped_type":"long"
            }
        }
    ],
    "query":{
        "filtered":{
            "filter":{
                "bool":{
                    "must":[
                        {
                            "query":{
                                "match":{
                                    "app-name":{
                                        "query":"promise-calendar",
                                        "type":"phrase"
                                    }
                                }
                            }
                        },
                        {
                            "query":{
                                "match":{
                                    "file":{
                                        "query":"/export/Logs/calc.stock.eclp.jd.com/service.log",
                                        "type":"phrase"
                                    }
                                }
                            }
                        },
                        {
                            "query":{
                                "match":{
                                    "ip":{
                                        "query":"10.174.67.178",
                                        "type":"phrase"
                                    }
                                }
                            }
                        },
                        {
                            "range":{
                                "time":{
                                    "gte":1553237840000,
                                    "lte":1553237850000,
                                    "format":"epoch_millis"
                                }
                            }
                        },
                        {
                            "range":{
                                "microsecond":{
                                    "gte":1550237840529780
                                }
                            }
                        }
                    ],
                    "should":[

                    ],
                    "must_not":[

                    ]
                }
            },
            "query":{
                "query_string":{
                    "query":"*",
                    "analyze_wildcard":true
                }
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
	if 1 != allTotal {
		t.Errorf("term query total not 1, total: %v\n", allTotal)
	}
}

func TestLogbookKeywordCount1(t *testing.T) {
	client := InitLogbookBegin()

	data := ` 
{
 "size": 1,
 "query": {
   "bool": {
     "must": [
       {"range": {"time": { "gte": 1550354269000, "lt": 1555355169000}}},
       {"term": {"app-name": "promise-calendar"}},
       {"match_phrase": {"msg": "exception"}}
     ]
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
	if 2 != allTotal {
		t.Errorf("term query total not 1, total: %v\n", allTotal)
	}
}

func TestLogbookKeywordCount2(t *testing.T) {
	client := InitLogbookBegin()

	data := ` 
{
 "size": 1,
 "query": {
	"match_all": {
		"boost": 1.0
	}
 },
 "aggregations": {
   "distinct_ip": {
     "terms": {
        "field": "ip",
        "size": 10000
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

	assert.Equal(t, 385, allTotal, "")
}

func TestLogbookKeywordCount(t *testing.T) {
	client := InitLogbookBegin()

	data := ` 
{
 "size": 0,
 "query": {
   "bool": {
     "must": [
       {"range": {"time": { "gte": 1550354269000, "lt": 1555355169000}}},
       {"term": {"app-name": "promise-calendar"}},
       {"match_phrase": {"msg": "exception"}}
     ]
   }
 },
 "aggs": {
   "distinct_ip": {
     "terms": {
        "field": "ip",
        "size": 10000
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
	if 2 != allTotal {
		t.Errorf("term query total not 1, total: %v\n", allTotal)
	}
}

func TestLogbookAllAndPerAppCount(t *testing.T) {
	client := InitLogbookBegin()

	data := ` 
{
	"size": 0,
	"fields": ["app-name"],
	"aggs": {
		"disctinct_app_name": {
			"terms": {
				"field": "app-name",
				"size": 1000
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
	assert.Equal(t, allTotal, 385, fmt.Sprintf("term query total not 385, total: %v\n", allTotal))

	data = ` 
{
	"size": 8000,
	"sort": [{
		"time": {
			"order": "asc"
		}
	}],
	"query": {
		"bool": {
			"must": [{
					"term": {
						"app-name": "ordertrack.360buy.com"
					}
				},
				{
					"match_phrase": {
						"msg": "OrderTrackSiteExportImpl"
					}
				}
			]
		}
	}
}

	`
	response, err = client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal TestLogbook doc result : %v\n", string(response.Resp))
	allMap, err = cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}
	allTotal, _ = allMap.GetJsonMap("hits").GetJsonValIntE("total")
	assert.Equal(t, allTotal, 6, fmt.Sprintf("term query total not 6, total: %v\n", allTotal))
}


func TestQueryStringQuery(t *testing.T) {
	data := `
	{
    "query": {
        "query_string": {
            "query": "msg:com.jd.eclp.stock.calc.service.impl.StockCancelServiceimpl AND ip:10.174.67.178 AND microsecond:1553237840529780"
        }
    }
	}
	`
	client := InitLogbookBegin()
	response, err := client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must not 1: %v\n", string(response.Resp))

	allMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(allMap)
}


func TestMinMaxValue(t *testing.T) {
	data := `
	{
	"size":0,
    "aggs": {
        "max": {
            "max": {
                "field": "time"
            }
        },
        "min": {
            "min": {
                "field": "time"
            }
        }
    }
	}
	`
	client := InitLogbookBegin()
	response, err := client.Search(http.MethodPost, data)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("test es normal ciSearchAll doc result _primary_term must not 1: %v\n", string(response.Resp))

	allMap, err := cbjson.ByteToJsonMap(response.Resp)
	if err != nil {
		t.Fatal(err)
	}

	max, err := allMap.GetJsonMap("aggregations").GetJsonMap("max").GetJsonValIntE("value")
	if err != nil {
		t.Fatal(err)
	}

	min, err := allMap.GetJsonMap("aggregations").GetJsonMap("min").GetJsonValIntE("value")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, max , 1553237852000000000,"max not same")
	assert.Equal(t, min , 1552354279000000000,"min not same")

	fmt.Println(max)
}

