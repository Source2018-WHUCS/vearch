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
	"github.com/spf13/cast"
	"github.com/tiglabs/baudengine/proto/pspb"
	"github.com/tiglabs/caprice/index"
	"reflect"

	"github.com/tiglabs/caprice/search/query"
)

type TermQuery struct {
	*QueryBuilder
	query.Query
}

func (qb *QueryBuilder) NewTermQuery() *TermQuery {
	return &TermQuery{QueryBuilder: qb}
}

func (t *TermQuery) SetQuery(query query.Query) {
	t.Query = query
}

/*
{
          "term": {
            "status": {
              "value": "urgent",
              "boost": 2.0
            }
          }
        },
        {
          "term": {
            "status": "normal"
          }
        }
*/

func (t *TermQuery) UnmarshalJSON(data []byte) error {
	tmp := make(map[string]interface{})
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	var tt *query.TermQuery
	for field, tq := range tmp {
		term, boost, err := t.parse(field, tq)
		if err != nil {
			return err
		}
		tt = query.NewTermQuery(field, term)
		tt.SetField(field)
		if boost != nil {
			tt.SetBoost(boost.Value())
		}
		t.Query = tt
		return nil
	}
	return nil
}

func (t *TermQuery) parseTerm(field string, data interface{}) (term index.Term, err error) {

	fm := t.mapping.GetField(field)

	if fm == nil {
		return term, nil
	}

	switch fm.FieldType() {
	case pspb.FieldType_TEXT, pspb.FieldType_KEYWORD:
		term = index.NewStringTerm(cast.ToString(data))
	case pspb.FieldType_INT:
		if v, err := cast.ToInt64E(data); err != nil {
			return term, err
		} else {
			term = index.NewIntTerm(v)
		}
	case pspb.FieldType_FLOAT:
		if v, err := cast.ToFloat64E(data); err != nil {
			return term, err
		} else {
			term = index.NewFloatTerm(v)
		}
	case pspb.FieldType_BOOL:
		if v, err := cast.ToBoolE(data); err != nil {
			return term, err
		} else {
			term = index.NewBooleanTerm(v)
		}
	//case pspb.FieldType_GEOPOINT: TODO not support
	case pspb.FieldType_DATE:
		if v, err := cast.ToTimeE(data); err != nil { //TODO it need use time parse
			return term, err
		} else {
			term = index.NewDateTimeTerm(v)
		}

	default:
		return term, errors.New("invalid term query")
	}

	return
}

func (t *TermQuery) parse(field string, data interface{}) (term index.Term, boost *Boost, err error) {
	val := reflect.ValueOf(data)
	typ := val.Type()
	switch typ.Kind() {
	case reflect.Map:
		for _, key := range val.MapKeys() {
			if key.String() == "value" {
				term, err = t.parseTerm(field, val.MapIndex(key).Interface())
				if err != nil {
					return
				}
			} else if key.String() == "boost" {
				var _boost float64
				_boost, err = toFloat(val.MapIndex(key).Interface())
				if err != nil {
					return
				}
				boost = NewBoost(_boost)
			}
		}
	default:
		term, err = t.parseTerm(field, data)
	}
	return
}
