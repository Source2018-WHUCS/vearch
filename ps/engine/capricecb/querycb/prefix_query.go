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
	"errors"
	"reflect"

	"github.com/tiglabs/caprice/search/query"
)

type PrefixQuery struct {
	*QueryBuilder
	query.Query
}

func (qb *QueryBuilder) NewPrefixQuery() *PrefixQuery {
	return &PrefixQuery{QueryBuilder: qb}
}

/*
{ "user" : "ki" }

{ "user" :  { "value" : "ki", "boost" : 2.0 } }
{ "user" :  { "prefix" : "ki", "boost" : 2.0 } }
*/
func (f *PrefixQuery) UnmarshalJSON(data []byte) error {
	tmp := make(map[string]interface{})
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	var tt *query.PrefixQuery
	for field, tq := range tmp {
		val := reflect.ValueOf(tq)
		typ := val.Type()
		switch typ.Kind() {
		case reflect.String:
			tt = query.NewPrefixQuery(field, val.String())
			tt.SetBoost(1.0)
		case reflect.Map:
			var prefix string
			var boost float64
			for _, key := range val.MapKeys() {
				if key.String() == "value" {
					prefix = reflect.ValueOf(val.MapIndex(key).Interface()).String()
				} else if key.String() == "prefix" {
					prefix = reflect.ValueOf(val.MapIndex(key).Interface()).String()
				} else if key.String() == "boost" {
					boost, err = toFloat(val.MapIndex(key).Interface())
					if err != nil {
						return err
					}
				} else {
					return errors.New("invalid prefix query")
				}
			}
			tt = query.NewPrefixQuery(field, prefix)
			tt.SetBoost(boost)
			tt.SetField(field)
		default:
			return errors.New("invalid term query")
		}
		if tt == nil {
			return errors.New("invalid term query")
		}
		f.Query = tt
		return nil
	}
	return nil
}
