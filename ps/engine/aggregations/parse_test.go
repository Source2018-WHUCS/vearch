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

package aggregations_test

import (
	"encoding/json"
	"fmt"
	"github.com/tiglabs/baudengine/ps/engine/aggregations"
	"github.com/tiglabs/baudengine/util/assert"
	_ "github.com/tiglabs/baudengine/util/init"
	"testing"
)

func TestParseAggs(t *testing.T) {
	aggs := `{
        "genders" : {
            "terms" : {
                "field" : "gender",
                "order" : { "_term" : "asc" }
            },
            "aggs" : {
        "ip_ranges" : {
            "ip_range" : {
                "field" : "ip",
                "ranges" : [
                    { "to" : "10.0.0.5" },
                    { "from" : "10.0.0.5" }
                ]
            }
        }
    },
    "aggs" : {
        "intraday_return" : { "sum" : { "field" : "change" } }
    }
        }
    }`

	fs, err := aggregations.ParseAggs([]byte(aggs))
	if err != nil {
		t.Fatalf("parse failed, err %v", err)
	}
	t.Log(len(fs), fs, )
}

func TestParseTopHits(t *testing.T) {

	if 1==1{
		fmt.Println("skip TestParseTopHits")
		return
	}

	v := `{
        "top_tags": {
            "terms": {
                "field": "age"
            },
            "aggs": {
                "top_sales_hits": {
                    "top_hits": {
                        "sort": [
        { "post_date" : {"order" : "asc"}},
        "user",
        { "name" : "desc" },
        { "age" : "desc" },
        "_score"
    ],
                        "size" : 3
                    }
                }
            }
        }
    }`

	if agg, err := aggregations.ParseAggs([]byte(v)); err != nil {
		panic(err)
	} else {
		fmt.Println(agg)
		//aggregations := ((agg)[0].B().Subs)[0]
		//orders, e := search.ParseSort(aggregations.(*metrics.TopHits).Sort)
		//fmt.Println(e)
		//fmt.Println(orders)
		assert.Equal(t, len((agg)[0].B().Subs), 1, "parse err")
	}

}

func TestParseSort(t *testing.T) {

	v := `{
      "agg1": {
         "terms": {
            "field": "age",
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
   }`

	agg, err := aggregations.ParseAggs([]byte(v))
	if err != nil {
		t.Fatal(err)
	} else {
		assert.Equal(t, len(agg), 1, "parse err")
	}

	err = agg[0].Combine()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(*agg[0].B().Order)
	orderMap := make([]map[string]string,0,2)

	err = json.Unmarshal(*agg[0].B().Order, &orderMap)

	if err != nil {
		t.Fatal(err)
	}


}

func TestMaxMinAgg(t *testing.T) {
	aggs := `
		{
			"maxAge": {
				"max": {"field":"age"}
			},
			"minAge": {
				"min": {"field":"age"}
			}
		}`

	agg, err := aggregations.ParseAggs([]byte(aggs))

	if err != nil {
		t.Fatal(err)
	}


	assert.Equal(t, len(agg) ,2,"")

}