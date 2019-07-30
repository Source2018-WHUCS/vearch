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
	"reflect"
	"strconv"
	"fmt"
)

func toFloat(i interface{}) (float64, error) {
	val := reflect.ValueOf(i)
	kind := val.Type().Kind()
	switch kind {
	case reflect.String:
		return strconv.ParseFloat(val.String(), 64)
	case reflect.Float64, reflect.Float32:
		return val.Float(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(val.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(val.Uint()), nil
	default:
		return 0.0, fmt.Errorf("invalid val kind %v", kind)
	}
}

func toInt(i interface{}) (int64, error) {
	val := reflect.ValueOf(i)
	kind := val.Type().Kind()
	switch kind {
	case reflect.String:
		return strconv.ParseInt(val.String(), 10, 64)
	case reflect.Float64, reflect.Float32:
		return int64(val.Float()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(val.Uint()), nil
	default:
		return 0.0, fmt.Errorf("invalid val kind %v", kind)
	}
}
