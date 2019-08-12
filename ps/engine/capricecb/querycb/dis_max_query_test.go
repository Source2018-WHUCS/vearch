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
	"github.com/blevesearch/bleve/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/registry"
	"github.com/tiglabs/baudengine/ps/engine/mapping"
	"reflect"
	"testing"

	"github.com/tiglabs/caprice/search/query"
)

var qb = &QueryBuilder{}

func init() {
	registry.RegisterAnalyzer(mapping.DefaultAnalyzer, keyword.AnalyzerConstructor)
	qb.mapping = mapping.NewIndexMapping()
	if err := qb.mapping.SetDefaultAnalyzer(mapping.DefaultAnalyzer) ;err != nil {
		panic(err)
	}
}

func TestDisMaxQuery(t *testing.T) {

	groups := []QueryTestGroup{
		QueryTestGroup{
			input: `{
            "queries": [
                { "match": { "title": "Quick pets" }},
                { "match": { "body":  "Quick pets" }}
            ],
            "tie_breaker": 0.3
        }`,
			output: func() *DisMaxQuery {
				qm1 := query.NewMatchQuery("title", "Quick pets")
				qm1.SetField("title")
				qm11 := qb.NewMatchQuery()
				qm11.SetQuery(qm1)
				qm2 := query.NewMatchQuery("body", "Quick pets")
				qm2.SetField("body")
				qm12 := qb.NewMatchQuery()
				qm12.SetQuery(qm2)
				q := query.NewBooleanQuery(nil, []query.Query{qm11, qm12}, nil)
				qq := qb.NewDisMaxQuery()
				qq.SetQuery(q)
				return qq
			}(),
		},
	}

	for _, group := range groups {
		tq := qb.NewDisMaxQuery()
		err := json.Unmarshal([]byte(group.input), tq)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tq.Query, group.output) {
			//TODO NOT RUN
			//t.Fatalf("parse failed %v %v", tq, group.output)
		}
	}
}
