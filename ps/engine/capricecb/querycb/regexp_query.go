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
	"errors"
	"reflect"

	"github.com/tiglabs/caprice/search/query"
)

type RegexpQuery struct {
	*QueryBuilder
	query.Query
	// bleve no flags
	flags string
	// bleve no states
	max_determinized_states int
}

func (qb *QueryBuilder) NewRegexpQuery() *RegexpQuery {
	return &RegexpQuery{QueryBuilder:qb,flags: "ALL", max_determinized_states: 10000}
}

func (r *RegexpQuery) SetFlags(flags string) {
	r.flags = flags
}

func (r *RegexpQuery) SetMaxDeterminizedStates(states int) {
	r.max_determinized_states = states
}

func (r *RegexpQuery) SetQuery(query query.Query) {
	r.Query = query
}

/*
{
        "name.first": "s.*y"
    }
{
        "name.first":{
            "value":"s.*y",
            "boost":1.2
        }
    }

{
        "name.first": {
            "value": "s.*y",
            "flags" : "INTERSECTION|COMPLEMENT|EMPTY"
        }
    }

{
        "name.first": {
            "value": "s.*y",
            "flags" : "INTERSECTION|COMPLEMENT|EMPTY",
            "max_determinized_states": 20000
        }
    }


*/
func (r *RegexpQuery) UnmarshalJSON(data []byte) error {
	tmp := make(map[string]interface{})
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	var tt *query.RegexpQuery
	for field, tq := range tmp {
		val := reflect.ValueOf(tq)
		typ := val.Type()
		switch typ.Kind() {
		case reflect.String:
			tt = query.NewRegexpQuery(field, val.String())
			tt.SetBoost(1.0)
		case reflect.Map:
			var regexp string
			var flags string
			boost := 1.0
			var states int64 = 10000
			for _, key := range val.MapKeys() {
				if key.String() == "value" {
					regexp = reflect.ValueOf(val.MapIndex(key).Interface()).String()
				} else if key.String() == "flags" {
					flags = reflect.ValueOf(val.MapIndex(key).Interface()).String()
					r.SetFlags(flags)
				} else if key.String() == "boost" {
					boost, err = toFloat(val.MapIndex(key).Interface())
					if err != nil {
						return err
					}
				} else if key.String() == "max_determinized_states" {
					states, err = toInt(val.MapIndex(key).Interface())
					if err != nil {
						return err
					}
					r.SetMaxDeterminizedStates(int(states))
				} else {
					return errors.New("invalid regexp query")
				}
			}
			tt = query.NewRegexpQuery(field, regexp)
			tt.SetBoost(boost)
		default:
			return errors.New("invalid regexp query")
		}
		if tt == nil {
			return errors.New("invalid regexp query")
		}
		r.Query = tt
		return nil
	}
	return nil
}
