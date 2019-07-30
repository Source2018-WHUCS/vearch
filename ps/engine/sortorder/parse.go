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

package sortorder

import (
	"encoding/json"
	"errors"
	"github.com/mmcloughlin/geohash"
	. "github.com/tiglabs/caprice/search/sort"
	"reflect"
)

var defaultSort = SortOrder{&SortScore{Desc: true}}

func ParseSort(bytes []byte) (SortOrder, error) {
	if len(bytes) == 0 {
		return defaultSort, nil
	}
	arr := make([]interface{}, 0, 3)
	if err := json.Unmarshal(bytes, &arr); err != nil {
		return nil, err
	} else {
		return parseSortInterface(arr)
	}
}

func parseSort(s interface{}) (Sort, error) {
	val := reflect.ValueOf(s)
	typ := val.Type()
	switch typ.Kind() {
	case reflect.String:
		if val.String() == "_score" {
			return &SortScore{Desc: true}, nil
		} else if val.String() == "_id" {
			return &SortField{Field: "_id", Desc: false}, nil
		} else {
			return &SortField{Field: val.String(), Desc: true}, nil
		}

	case reflect.Map:
		if typ.Key().Kind() == reflect.String {
			for _, key := range val.MapKeys() {
				fieldName := key.String()
				sortVal := val.MapIndex(key).Interface()
				sVal := reflect.ValueOf(sortVal)
				switch sVal.Type().Kind() {
				case reflect.String:
					if sVal.String() == "desc" {
						return &SortField{Field: fieldName, Desc: true}, nil
					} else if sVal.String() == "asc" {
						return &SortField{Field: fieldName, Desc: false}, nil
					} else {
						return nil, errors.New("invalid sort")
					}
				case reflect.Map:
					if fieldName == "_geo_distance" {
						return parseGeoSortOrder(sortVal)
					} else {
						var sort SortField
						sort.Field = fieldName
						for _, subKey := range sVal.MapKeys() {
							switch subKey.String() {
							case "order":
								order, ok := sVal.MapIndex(subKey).Interface().(string)
								if !ok {
									return nil, errors.New("invalid sort")
								}
								if order == "desc" {
									sort.Desc = true
								} else if order == "asc" {
									sort.Desc = false
								}
							case "mode":
								mode, ok := sVal.MapIndex(subKey).Interface().(string)
								if !ok {
									return nil, errors.New("invalid sort")
								}
								if mode == "min" {
									sort.Mode = SortFieldMin
								} else if mode == "max" {
									sort.Mode = SortFieldMax
								} else {
									// fixme not support avg/sum
								}
							case "missing":
								missing, ok := sVal.MapIndex(subKey).Interface().(string)
								if !ok {
									return nil, errors.New("invalid sort")
								}
								if missing == "_last" {
									sort.Missing = SortFieldMissingLast
								} else if missing == "_first" {
									sort.Missing = SortFieldMissingFirst
								} else {
									return nil, errors.New("invalid sort")
								}
							case "unmapped_type":
								// todo
							}
						}
						return &sort, nil
					}

				}
			}
		}
	case reflect.Ptr:
		ptrElem := val.Elem()
		if ptrElem.IsValid() && ptrElem.CanInterface() {
			return parseSort(ptrElem.Interface())
		}
	default:
		return nil, errors.New("invalid sort type " + typ.Kind().String())
	}
	return nil, errors.New("invalid sort")
}

// ParseGeoSortOrder
// Raw example:
//
//  { "post_date" : {"order" : "asc"}},
//  	"user",
//      { "name" : "desc" },
//      { "age" : "desc" },
//      "_score"
//  }
func parseGeoSortOrder(s interface{}) (*SortGeoDistance, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	tmp := make(map[string]json.RawMessage)
	err = json.Unmarshal(data, &tmp)
	if err != nil {
		return nil, err
	}
	sort := &SortGeoDistance{Desc: true}
	for key, val := range tmp {
		switch key {
		case "order":
			var order string
			err = json.Unmarshal(val, &order)
			if err != nil {
				return nil, err
			}
			if order == "desc" {
				sort.Desc = true
			} else if order == "asc" {
				sort.Desc = false
			} else {
				return nil, errors.New("invalid geo sort")
			}
		case "mode":
			var mode string
			err = json.Unmarshal(val, &mode)
			if err != nil {
				return nil, err
			}
			switch mode {
			case "avg", "sum", "min", "max":
			default:
				return nil, errors.New("invalid geo sort")
			}
			// fixme do nothing now
		case "unit":
			var unit string
			err = json.Unmarshal(val, &unit)
			if err != nil {
				return nil, err
			}
			sort.Unit = unit
		case "distance_type":
			// fixme do nothing
		default:
			if sort.Field != "" {
				return nil, errors.New("invalid geo sort")
			}
			sort.Field = key
			var geoHash string
			err = json.Unmarshal(val, &geoHash)
			if err == nil {
				sort.Lat, sort.Lon = geohash.Decode(geoHash)
			} else {
				var geo []float64
				geo = make([]float64, 0)
				err = json.Unmarshal(val, &geo)
				if err == nil {
					if len(geo) != 2 {
						return nil, errors.New("invalid geo sort")
					}
					sort.Lat = geo[0]
					sort.Lon = geo[1]
				} else {
					geoLatLon := struct {
						Lat float64 `json:"lat"`
						Lon float64 `json:"lon"`
					}{}
					err = json.Unmarshal(val, &geoLatLon)
					if err == nil {
						sort.Lat = geoLatLon.Lat
						sort.Lon = geoLatLon.Lon
					} else {
						return nil, err
					}
				}
			}

		}
	}
	if sort.Field == "" {
		return nil, errors.New("invalid geo sort")
	}
	return sort, err
}
func parseSortInterface(s interface{}) (SortOrder, error) {
	if s == nil {
		return nil, nil
	}
	var sortOrder SortOrder
	val := reflect.ValueOf(s)
	typ := val.Type()

	switch typ.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < val.Len(); i++ {
			if val.Index(i).CanInterface() {
				sortVal := val.Index(i).Interface()
				sort, err := parseSort(sortVal)
				if err != nil {
					return nil, err
				}
				sortOrder = append(sortOrder, sort)
			}
		}
	case reflect.String:
		sort, err := parseSort(s)
		if err != nil {
			return nil, err
		}
		sortOrder = append(sortOrder, sort)
	default:
		sort, err := parseSort(val)
		if err != nil {
			return nil, err
		}
		sortOrder = append(sortOrder, sort)
	}
	return sortOrder, nil
}
