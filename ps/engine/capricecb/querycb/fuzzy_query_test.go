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

package querycb

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/tiglabs/caprice/search/query"
)

func TestFuzzyQuery(t *testing.T) {
	groups := []QueryTestGroup{
		{input: `{ "user" : "ki" }`,
			output: func() query.Query {
				utq := query.NewFuzzyQuery("user", "ki")
				utq.SetField("user")
				q := qb.NewFuzzyQuery()
				q.SetQuery(utq)
				return q
			}()},
		{
			input: `{
        "user" : {
            "value" :         "ki",
            "boost" :         1.0,
            "fuzziness" :     2,
            "prefix_length" : 0,
            "max_expansions": 100
        }
    }`,
			output: func() query.Query {
				utq := query.NewFuzzyQuery("user", "ki")
				utq.SetField("user")
				utq.SetBoost(1.0)
				utq.SetPrefix(0)
				utq.SetFuzziness(2)
				q := qb.NewFuzzyQuery()
				q.SetMaxExpansions(100)
				q.SetQuery(utq)
				return q
			}()},
	}

	for _, group := range groups {
		tq := qb.NewFuzzyQuery()
		err := json.Unmarshal([]byte(group.input), tq)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tq, group.output) {
			t.Fatalf("parse failed %v %v", tq, group.output)
		}
	}
}
