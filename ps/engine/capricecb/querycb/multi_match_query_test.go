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
	"errors"
	"fmt"
	"github.com/blevesearch/bleve/registry"
	"reflect"
	"testing"

	"github.com/tiglabs/caprice/search/query"
)

func TestMultiMatchQuery(t *testing.T) {
	if 1==1{
		fmt.Println("skip TestMultiMatchQuery")
		return
	}
	groups := []QueryTestGroup{QueryTestGroup{input: `{
  "multi_match" : {
    "query":    "this is a test", 
    "fields": [ "subject", "message" ] 
  }
}`,
		output: func() query.Query {
			q1 := query.NewMatchQuery("subject", "this is a test")
			q2 := query.NewMatchQuery("subject", "this is a test")
			utq := query.NewBooleanQuery(nil, []query.Query{q1, q2}, nil)
			q := qb.NewMultiMatch()
			q.SetQuery(utq)
			return q
		}()},

		QueryTestGroup{input: `{
  "multi_match" : {
    "query":    "Will Smith",
    "fields": [ "title", "*_name" ] 
  }
}`,
			err: errors.New("not support wildcard field")},

		QueryTestGroup{input: `{
  "multi_match" : {
    "query" : "this is a test",
    "fields" : [ "subject^3", "message" ] 
  }
}`,
			output: func() query.Query {
				q1 := query.NewMatchQuery("subject", "this is a test")
				q1.SetBoost(3)
				q2 := query.NewMatchQuery("message", "this is a test")
				utq := query.NewBooleanQuery(nil, []query.Query{q1, q2}, nil)
				q := qb.NewMultiMatch()
				q.SetQuery(utq)
				return q
			}()},

		QueryTestGroup{input: `{
  "multi_match" : {
    "query":      "Will Smith",
    "type":       "best_fields",
    "fields":     [ "first_name", "last_name" ],
    "operator":   "and" 
  }
}`,
			output: func() query.Query {
				q1 := query.NewMatchQuery("first_name", "Will Smith")
				q1.SetOperator(query.MatchQueryOperatorAnd)
				q2 := query.NewMatchQuery("first_name", "Will Smith")
				q2.SetOperator(query.MatchQueryOperatorAnd)
				utq := query.NewBooleanQuery(nil, []query.Query{q1, q2}, nil)
				q := qb.NewMultiMatch()
				q.SetQuery(utq)
				return q
			}()},

		QueryTestGroup{input: `{
  "multi_match" : {
    "query":      "Jon",
    "type":       "cross_fields",
    "analyzer":   "standard", 
    "fields":     [ "first", "last", "edge" ]
  }
}`,
			output: func() query.Query {
				analyzer , _ := registry.NewCache().AnalyzerNamed("standard")
				q1 := query.NewMatchQuery("first", "Jon")
				q1.Analyzer = analyzer
				q2 := query.NewMatchQuery("last", "Jon")
				q2.Analyzer = analyzer
				q3 := query.NewMatchQuery("edge", "Jon")
				q3.Analyzer = analyzer
				utq := query.NewBooleanQuery(nil, []query.Query{q1, q2, q3}, nil)
				q := qb.NewMultiMatch()
				q.SetQuery(utq)
				return q
			}()},
	}
	for i, g := range groups {
		output, err := qb.ParseQuery([]byte(g.input))
		if err != nil {
			if g.err != nil && g.err.Error() == err.Error() {
				continue
			}
			t.Fatalf("parse failed %v", err)
		}
		if !reflect.DeepEqual(output, g.output.(*MultiMatch).Query) {
			t.Fatalf("parse failed %d", i)
		}
	}
}
