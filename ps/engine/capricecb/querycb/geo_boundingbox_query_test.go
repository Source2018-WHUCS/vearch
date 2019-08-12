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

	"github.com/tiglabs/caprice/search/query"
)

func TestGeoBoundingBoxQuery(t *testing.T) {
	groups := []QueryTestGroup{
		QueryTestGroup{input: `{
          "location": {
            "top_left": {
              "lat":  40.8,
              "lon": -74.0
            },
            "bottom_right": {
              "lat":  40.7,
              "lon": -73.0
            }
          }}`,
			output: func() query.Query {
				utq := query.NewGeoBoundingBoxQuery("location", -74.0, 40.8, -73.0, 40.7)
				utq.SetField("location")
				q := qb.NewGeoBoundingBoxQuery()
				q.SetQuery(utq)
				return q
			}()},
	}

	for _, group := range groups {
		tq := qb.NewGeoBoundingBoxQuery()
		err := json.Unmarshal([]byte(group.input), tq)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tq, group.output) {
			t.Fatalf("parse failed %v %v", tq, group.output)
		}
	}
}
