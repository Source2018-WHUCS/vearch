// Copyright 2018 The Couchbase Authors.
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

package querycb

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/blevesearch/bleve/numeric"
	"github.com/tiglabs/caprice/search/query"
)

type QueryTestGroup struct {
	input  string
	output query.Query
	err    error
}

func TestParseTermQuery(t *testing.T) {
	groups := []QueryTestGroup{QueryTestGroup{input: `{
            "status": {
              "value": "urgent",
              "boost": 2.0
            }
          }`,
		output: func() query.Query {
			utq := query.NewTermQuery("status", "urgent")
			utq.SetBoost(2.0)
			return utq
		}()},
		QueryTestGroup{input: `{
            "status": {
              "value": 12,
              "boost": 2.0
            }
          }`,
			output: func() query.Query {
				i64 := numeric.Float64ToInt64(12.0)
				pc, err := numeric.NewPrefixCodedInt64(i64, 0)
				if err != nil {
					t.Fatal(err)
				}
				utq := query.NewTermQuery("status", string(pc))
				utq.SetBoost(2.0)
				return utq
			}()},
		QueryTestGroup{
			input: `{
            "status": "normal"
          }`,
			output: func() query.Query {
				utq := query.NewTermQuery("status", "normal")
				return utq
			}()},
		QueryTestGroup{
			input: `{
            "price": 12
          }`,
			output: func() query.Query {
				i64 := numeric.Float64ToInt64(12.0)
				pc, err := numeric.NewPrefixCodedInt64(i64, 0)
				if err != nil {
					t.Fatal(err)
				}
				utq := query.NewTermQuery("price", string(pc))
				return utq
			}()},
	}

	for i, group := range groups {
		tq := qb.NewTermQuery()
		err := json.Unmarshal([]byte(group.input), tq)
		if err != nil {
			t.Fatal(err)
		}
		ttq, ok := tq.Query.(*query.TermQuery)
		if !ok {
			t.Fatal("parse failed")
		}

		if !reflect.DeepEqual(ttq, group.output) {
			t.Fatalf("%d parse failed %v %v", i, ttq, group.output)
		}
	}
}
