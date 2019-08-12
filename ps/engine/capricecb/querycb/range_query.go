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
	"github.com/spf13/cast"
	"github.com/tiglabs/baudengine/proto/pspb"
	"github.com/tiglabs/caprice/search/query"
	"time"
)

type RangeQuery struct {
	*QueryBuilder
	query.Query
}

func (qb *QueryBuilder) NewRangeQuery() *RangeQuery {
	return &RangeQuery{QueryBuilder:qb}
}

/*
{
    "range" : {
        "age" : {
            "gte" : 10,
            "lte" : 20,
            "boost" : 2.0
        }
    }
}

{
    "range" : {
        "date" : {
            "gte" : "now-1d/d",
            "lt" :  "now/d"
        }
    }
}
*/
func (r *RangeQuery) UnmarshalJSON(data []byte) error {
	tmp := make(map[string]map[string]interface{})
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	for field, rv := range tmp {

		docField := r.mapping.GetField(field)

		var includeLower, includeUpper, found bool

		var start, end interface{}

		if start, found = rv["from"]; !found {
			if start, found = rv["gt"]; !found {
				if start, found = rv["gte"]; found {
					includeLower = true
				}
			} else {
				includeLower = false
			}
		} else {
			if rv["include_lower"] == nil || !cast.ToBool(rv["include_lower"]) {
				includeLower = false
			} else {
				includeLower = true
			}
		}

		if end, found = rv["to"]; !found {
			if end, found = rv["lt"]; !found {
				if end, found = rv["lte"]; found {
					includeUpper = true
				}
			} else {
				includeUpper = false
			}
		} else {
			if rv["include_upper"] == nil || !cast.ToBool(rv["include_upper"]) {
				includeUpper = false
			} else {
				includeUpper = true
			}
		}



		switch docField.FieldType() { //TODO ANSJ FIX IT
		case pspb.FieldType_INT:
			var minNum, maxNum *int64

			if start != nil {
				if f, e := cast.ToInt64E(start); e != nil {
					return e
				} else {
					minNum = &f
				}
			}

			if end != nil {
				if f, e := cast.ToInt64E(end); e != nil {
					return e
				} else {
					maxNum = &f
				}
			}

			r.Query = query.NewIntRangeInclusiveQuery(field,minNum, maxNum, !includeLower, !includeUpper)

		case pspb.FieldType_FLOAT:
			var minNum, maxNum *float64

			if start != nil {
				if f, e := cast.ToFloat64E(start); e != nil {
					return e
				} else {
					minNum = &f
				}
			}

			if end != nil {
				if f, e := cast.ToFloat64E(end); e != nil {
					return e
				} else {
					maxNum = &f
				}
			}

			r.Query = query.NewFloatRangeInclusiveFilter(field,minNum, maxNum, !includeLower, !includeUpper)

		case pspb.FieldType_DATE:

			//TODO ANSJ we need a interface to date util
			var (
				startTimePtr *time.Time = nil
				endTimePtr *time.Time = nil
			)

			if start != nil {
				if f, e := cast.ToInt64E(start); e != nil {
					startTime, e := cast.ToTimeE(start)
					if e != nil {
						return e
					}
					startTimePtr = &startTime
				} else {
					startTime := time.Unix(0, f*1e6)
					startTimePtr = &startTime
				}
			}

			if end != nil {
				if f, e := cast.ToInt64E(end); e != nil {
					endTime, e := cast.ToTimeE(end)
					if e != nil {
						return e
					}
					endTimePtr = &endTime
				} else {
					endTime := time.Unix(0, f*1e6)
					endTimePtr = &endTime
				}
			}

			r.Query = query.NewDateTimeRangeInclusiveQuery(field, startTimePtr, endTimePtr, !includeLower, !includeUpper)
		default:

			var min, max string

			if start != nil {
				min = cast.ToString(start)
			}

			if end != nil {
				max = cast.ToString(end)
			}

			r.Query = query.NewTermRangeInclusiveQuery(field,min, max, &includeLower, &includeUpper)

		}

		r.Query.(query.FieldableQuery).SetField(field)

		if b, f := rv["boost"]; f {
			r.Query.(query.BoostableQuery).SetBoost(cast.ToFloat64(b))
		}

		return nil

	}
	return nil
}
