//  Copyright (c) 2014 Couchbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package querycb

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/tiglabs/caprice/search/query"
)

func TestWildcardQuery(t *testing.T) {
	groups := []QueryTestGroup{QueryTestGroup{input: `{ "user" : "ki*y" }`,
		output: func() query.Query {
			utq := query.NewWildcardQuery("user", "ki*y")
			utq.SetBoost(1.0)
			return utq
		}()},
		QueryTestGroup{
			input: `{ "user" : { "value" : "ki*y", "boost" : 2.0 } }`,
			output: func() query.Query {
				utq := query.NewWildcardQuery("user", "ki*y")
				utq.SetBoost(2.0)
				return utq
			}()},

		QueryTestGroup{
			input: `{ "user" : { "wildcard" : "ki*y", "boost" : 2.0 } }`,
			output: func() query.Query {
				utq := query.NewWildcardQuery("user", "ki*y")
				utq.SetBoost(2.0)
				return utq
			}()},
	}

	for _, group := range groups {
		tq := qb.NewWildcardQuery()
		err := json.Unmarshal([]byte(group.input), tq)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tq.Query, group.output) {
			t.Fatalf("parse failed %v %v", tq, group.output)
		}
	}
}
