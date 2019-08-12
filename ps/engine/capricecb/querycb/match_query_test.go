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

func TestMatchQuery(t *testing.T) {

	groups := []QueryTestGroup{QueryTestGroup{input: `{
        "message" : "this is a test"
    }`,
		output: func() query.Query {
			utq := query.NewMatchQuery("message", "this is a test")
			return utq
		}()},

		QueryTestGroup{input: `{
        "price" : 12
    }`,
			output: func() query.Query {
				i64 := numeric.Float64ToInt64(12.0)
				pc, err := numeric.NewPrefixCodedInt64(i64, 0)
				if err != nil {
					t.Fatal(err)
				}
				utq := query.NewMatchQuery("price", string(pc))
				return utq
			}()},

		QueryTestGroup{input: `{
        "message" : {
            "query" : "this is a test",
            "analyzer" : "cb_standard"
        }
    }`,
			output: func() query.Query {
				utq := query.NewMatchQuery("message", "this is a test")
				return utq
			}()},

		QueryTestGroup{input: `{
        "message" : {
            "query" : "this is a test",
            "operator" : "and"
        }
    }`,
			output: func() query.Query {
				utq := query.NewMatchQuery("message", "this is a test")
				utq.Operator = 1
				return utq
			}()},
	}

	for i, group := range groups {
		tq := qb.NewMatchQuery()
		err := json.Unmarshal([]byte(group.input), tq)
		if err != nil {
			t.Fatal(err)
		}

		group.output.(*query.MatchQuery).Analyzer = tq.Query.(*query.MatchQuery).Analyzer

		if !reflect.DeepEqual(tq.Query, group.output) {
			t.Fatalf("the number %d parse failed %v %v", i, tq.Query, group.output)
		}
	}
}
