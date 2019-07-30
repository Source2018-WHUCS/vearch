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

func TestRegexpQuery(t *testing.T) {
	groups := []QueryTestGroup{QueryTestGroup{input: `{
        "name.first": "s.*y"
    }`,
		output: func() query.Query {
			utq := query.NewRegexpQuery("name.first", "s.*y")
			utq.SetField("name.first")
			utq.SetBoost(1.0)
			q := qb.NewRegexpQuery()
			q.SetQuery(utq)
			return q
		}()},
		QueryTestGroup{
			input: `{
        "name.first":{
            "value":"s.*y",
            "boost":1.2
        }
    }`,
			output: func() query.Query {
				utq := query.NewRegexpQuery("name.first", "s.*y")
				utq.SetField("name.first")
				utq.SetBoost(1.2)
				q := qb.NewRegexpQuery()
				q.SetQuery(utq)
				return q
			}()},

		QueryTestGroup{
			input: `{
        "name.first": {
            "value": "s.*y",
            "flags" : "INTERSECTION|COMPLEMENT|EMPTY"
        }
    }`,
			output: func() query.Query {
				utq := query.NewRegexpQuery("name.first", "s.*y")
				utq.SetBoost(1.0)
				q := qb.NewRegexpQuery()
				q.SetQuery(utq)
				q.SetFlags("INTERSECTION|COMPLEMENT|EMPTY")
				return q
			}()},

		QueryTestGroup{
			input: `{
        "name.first": {
            "value": "s.*y",
            "flags" : "INTERSECTION|COMPLEMENT|EMPTY",
            "max_determinized_states": 20000
        }
    }`,
			output: func() query.Query {
				utq := query.NewRegexpQuery("name.first", "s.*y")
				utq.SetBoost(1.0)
				q := qb.NewRegexpQuery()
				q.SetQuery(utq)
				q.SetFlags("INTERSECTION|COMPLEMENT|EMPTY")
				q.SetMaxDeterminizedStates(20000)
				return q
			}()},
	}

	for _, group := range groups {
		tq := qb.NewRegexpQuery()
		err := json.Unmarshal([]byte(group.input), tq)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tq, group.output) {
			t.Fatalf("parse failed %v %v", tq, group.output)
		}
	}
}
