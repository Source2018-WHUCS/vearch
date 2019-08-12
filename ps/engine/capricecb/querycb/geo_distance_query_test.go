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

func TestGeoDistanceQuery(t *testing.T) {
	groups := []QueryTestGroup{
		QueryTestGroup{input: `{
          "distance": "1km",
          "location": {
            "lat":  40.715,
            "lon": -73.988
          }
        }`,
			output: func() query.Query {
				utq := query.NewGeoDistanceQuery("location", -73.988, 40.715, "1km")
				return utq
			}()},
	}

	for _, group := range groups {
		tq := qb.NewGeoDistanceQuery()
		err := json.Unmarshal([]byte(group.input), tq)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tq.Query, group.output) {
			t.Fatalf("parse failed %v %v", tq, group.output)
		}
	}
}
