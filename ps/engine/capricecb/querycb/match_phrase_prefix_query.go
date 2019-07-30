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
	"fmt"
	"github.com/spf13/cast"
	"github.com/tiglabs/caprice/search/query"
	"reflect"
)

type MatchPhrasePrefixQuery struct {
	*QueryBuilder
	query.Query
}

func (qb *QueryBuilder) NewMatchPhrasePrefixQuery() *MatchPhrasePrefixQuery {
	return &MatchPhrasePrefixQuery{QueryBuilder: qb}
}

func (m *MatchPhrasePrefixQuery) SetQuery(query query.Query) {
	m.Query = query
}

func (m *MatchPhrasePrefixQuery) UnmarshalJSON(data []byte) error {
	tmp := make(map[string]interface{})
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	var q query.Query

	for field, fv := range tmp {
		switch reflect.ValueOf(fv).Kind() {
		case reflect.Map:
			//"match_phrase_prefix" : {
			//	"message" : {
			//		"query" : "this is a test",
			//			"analyzer" : "my_analyzer"
			//	}
			//}

			queryInfo := fv.(map[string]interface{})
			match, ok := queryInfo["query"]
			if !ok {
				return fmt.Errorf("not found query in [%s]", string(data))
			}
			mq := query.NewMatchPhrasePrefixQuery(field, match.(string))

			if analyzer, ok := queryInfo["analyzer"]; ok {
				if a, err := m.mapping.GetAnalyzer(analyzer.(string)); err != nil {
					return err
				} else {
					mq.SetAnalyzer(AdaptAnalyzer(a))
				}
			}
			if boost, ok := queryInfo["boost"]; ok {
				mq.SetBoost(cast.ToFloat64(boost))
			}
			if me, ok := queryInfo["max_expansions"]; ok {
				mq.SetMaxExpansions(cast.ToInt(me))
			}
			q = mq

		default:
			//"match_phrase_prefix" : {
			//	"message" : "this is a te"
			//}
			match, ok := fv.(string)
			if !ok {
				return fmt.Errorf("can not case value to string [%v]", fv)
			}
			mq := query.NewMatchPhrasePrefixQuery(field, match)
			mq.SetField(field)
			q = mq
		}
		break // it may be only one key value
	}
	if q != nil {
		if q.(*query.MatchPhrasePrefixQuery).Analyzer== nil {
			q.(*query.MatchPhrasePrefixQuery).SetAnalyzer(AdaptAnalyzer(m.mapping.DefaultAnalyzer))
		}
		m.SetQuery(q)
		return nil
	}
	return err
}
