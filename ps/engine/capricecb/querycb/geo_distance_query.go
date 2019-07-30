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

// TODO: geo distance query not support now
//import (
//	"encoding/json"
//	"errors"
//
//	"github.com/tiglabs/caprice/search/query"
//	"github.com/blevesearch/bleve/geo"
//)
//
//type GeoDistanceQuery struct {
//	query.Query     `json:"-"`
//}
//
//func (qb *QueryBuilder) NewGeoDistanceQuery() *GeoDistanceQuery {
//	return &GeoDistanceQuery{}
//}
//
//func (g *GeoDistanceQuery) SetQuery(q query.Query) {
//	g.Query = q
//}
//
//func (g *GeoDistanceQuery)UnmarshalJSON(data []byte) error {
//	tmp := make(map[string]interface{})
//	err := json.Unmarshal(data, &tmp)
//	if err != nil {
//		return err
//	}
//	var boost float64
//	var location []float64
//	var fieldName, distance string
//	var ok bool
//	for k, v := range tmp {
//		if k == "boost" {
//			boost, err = toFloat(v)
//			if err != nil {
//				return err
//			}
//		} else if k == "distance" {
//			if distance, ok = v.(string); !ok {
//				return errors.New("invalid geo distance")
//			}
//		} else if k == "distance_type" {
//            // TODO ???
//		} else {
//			if len(fieldName) > 0 {
//				continue
//			}
//			fieldName = k
//			lon, lat, success := geo.ExtractGeoPoint(v)
//			if !success {
//				return errors.New("invalid geo distance")
//			}
//			location = append(location, lon, lat)
//		}
//	}
//	if len(fieldName) == 0 || len(location) != 2 || len(distance) == 0 {
//		return errors.New("invalid geo distance")
//	}
//	q := query.NewGeoDistanceQuery(fieldName,location[0], location[1], distance)
//	if boost != 0 {
//		q.SetBoost(boost)
//	}
//	g.Query = q
//	return nil
//}