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
	"github.com/tiglabs/caprice/index"
	"reflect"
	"encoding/json"
	"github.com/tiglabs/caprice/search/query"
)

type FuzzyQuery struct {
	query.Query
	max_expansions int
}

func (qb *QueryBuilder) NewFuzzyQuery() *FuzzyQuery {
	return &FuzzyQuery{max_expansions: 50}
}

func (f *FuzzyQuery) SetMaxExpansions(max int) {
	f.max_expansions = max
}

func (f *FuzzyQuery) SetQuery(query query.Query) {
	f.Query = query
}

/*
{ "user" : "ki" }

{
        "user" : {
            "value" :         "ki",
            "boost" :         1.0,
            "fuzziness" :     2,
            "prefix_length" : 0,
            "max_expansions": 100
        }
    }
*/
func (f *FuzzyQuery) UnmarshalJSON(data []byte) error {
	tmp := make(map[string]interface{})
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	var tt *query.FuzzyQuery
	for field, tq := range tmp {
		val := reflect.ValueOf(tq)
		typ := val.Type()
		switch typ.Kind() {
		case reflect.String:
			tt = query.NewFuzzyQuery(field,index.NewStringTerm(val.String()))
		case reflect.Map:
			var term string
			var boost *float64
			var fuzzy int64 = 1
			var prefix *int64
			var max_expansions *int64
			for _, key := range val.MapKeys() {
				switch key.String() {
				case "value":
					term = reflect.ValueOf(val.MapIndex(key).Interface()).String()
				case "boost":
					var b float64
					b, err = toFloat(val.MapIndex(key).Interface())
					if err != nil {
						return err
					}
					boost = &b
				case "fuzziness":
					fuzzy, err = toInt(val.MapIndex(key).Interface())
					if err != nil {
						return err
					}
				case "prefix_length":
					_prefix, err := toInt(val.MapIndex(key).Interface())
					if err != nil {
						return err
					}
					prefix = &_prefix
				case "max_expansions":
					_max_expansions, err := toInt(val.MapIndex(key).Interface())
					if err != nil {
						return err
					}
					max_expansions = &_max_expansions
				}
			}
			tt = query.NewFuzzyQuery(field,index.NewStringTerm(term))
			if boost != nil {
				tt.SetBoost(*boost)
			}
			if prefix != nil {
				tt.SetPrefix(int(*prefix))
			}
			tt.SetFuzziness(int(fuzzy))
			if max_expansions != nil {
				f.max_expansions = int(*max_expansions)
			}
		default:
			return ErrInvalidTermQuery
		}
		if tt == nil {
			return ErrInvalidTermQuery
		}
		f.Query = tt
		return nil
	}
	return nil
}
