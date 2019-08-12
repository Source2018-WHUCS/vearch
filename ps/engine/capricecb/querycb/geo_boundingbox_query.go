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

// TODO: geo bounding box query not support now
//import (
//	"encoding/json"
//	"fmt"
//	"errors"
//
//	"github.com/tiglabs/caprice/search/query"
//	"github.com/blevesearch/bleve/geo"
//
//)
//
//type  GeoBoundingBoxQuery struct {
//	query.Query     `json:"-"`
//}
//
//func (qb *QueryBuilder) NewGeoBoundingBoxQuery() *GeoBoundingBoxQuery {
//	return &GeoBoundingBoxQuery{}
//}
//
//func (g *GeoBoundingBoxQuery) SetQuery(q query.Query) {
//	g.Query = q
//}
//
//func (g *GeoBoundingBoxQuery)UnmarshalJSON(data []byte) error {
//	tmp := make(map[string]interface{})
//	err := json.Unmarshal(data, &tmp)
//	if err != nil {
//		return err
//	}
//	var boost float64
//	var bottomRight, topLeft []float64
//	var fieldName string
//	for k, v := range tmp {
//		if k == "boost" {
//			boost, err = toFloat(v)
//			if err != nil {
//				return err
//			}
//		} else if k == "type" {
//			// TODO ???
//		}else {
//			if len(fieldName) > 0 {
//				continue
//			}
//			fieldName = k
//			if box, ok := v.(map[string]interface{}); ok {
//				if _topLeft, found := box["top_left"]; found {
//					lon, lat, success := geo.ExtractGeoPoint(_topLeft)
//					if !success {
//						return errors.New("invalid geo bound box")
//					}
//					topLeft = append(topLeft, lon, lat)
//				} else {
//					return errors.New("invalid geo bound box")
//				}
//
//				if _bottomRight, found := box["bottom_right"]; found {
//					lon, lat, success := geo.ExtractGeoPoint(_bottomRight)
//					if !success {
//						return errors.New("invalid geo bound box")
//					}
//					bottomRight = append(bottomRight, lon, lat)
//				} else {
//					return errors.New("invalid geo bound box")
//				}
//			}
//		}
//	}
//	if len(fieldName) == 0 || len(bottomRight) != 2 || len(topLeft) != 2 {
//		return errors.New("invalid geo bound box")
//	}
//	fmt.Println(fieldName, bottomRight, topLeft)
//	q := query.NewGeoBoundingBoxQuery(fieldName,topLeft[0], topLeft[1], bottomRight[0], bottomRight[1])
//    if boost != 0 {
//	    q.SetBoost(boost)
//    }
//	g.Query = q
//	return nil
//}