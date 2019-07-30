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

package util

import (
	"github.com/tiglabs/log"
	"reflect"
	"unsafe"
)

// SliceToString slice to string with out data copy
func SliceToString(b []byte) (s string) {
	pbytes := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	pstring := (*reflect.StringHeader)(unsafe.Pointer(&s))
	pstring.Data = pbytes.Data
	pstring.Len = pbytes.Len
	return
}

// StringToSlice string to slice with out data copy
func StringToSlice(s string) (b []byte) {
	pbytes := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	pstring := (*reflect.StringHeader)(unsafe.Pointer(&s))
	pbytes.Data = pstring.Data
	pbytes.Len = pstring.Len
	pbytes.Cap = pstring.Len
	return
}

//copy from https://github.com/Re-volution/sizestruct
type sStruct struct {
	npm   map[interface{}]bool
	exNum int
	deep  int
}

func SizeOf(data interface{}) int {
	var npm = &sStruct{make(map[interface{}]bool), 0, 0}
	num := npm.sizeof(reflect.ValueOf(data))
	return num //+ npm.exNum
}

func SizeStructAndType(data interface{}) int {
	var npm = &sStruct{make(map[interface{}]bool), 0, 0}
	num := npm.sizeof(reflect.ValueOf(data))
	return num + npm.exNum
}

func (s *sStruct) sizeof(v reflect.Value) int {
	s.deep++
	if s.deep > 1000000 {
		log.Error("struts has more elements  so skip sizeOf")
		return 0 //
	}

	switch v.Kind() {
	case reflect.Map:
		sum := 0
		keys := v.MapKeys()
		for i := 0; i < len(keys); i++ {
			mapkey := keys[i]
			num := s.sizeof(mapkey)
			if num < 0 {
				return -1
			}
			sum += num
			num = s.sizeof(v.MapIndex(mapkey))
			if num < 0 {
				return -1
			}
			sum += num
		}
		s.exNum += int(v.Type().Size())
		return sum
	case reflect.Slice:
		sum := 0
		for i, n := 0, v.Len(); i < n; i++ {
			num := s.sizeof(v.Index(i))
			if num < 0 {
				return -1
			}
			sum += num
		}
		s.exNum += int(v.Type().Size())
		return sum

	case reflect.Array:
		sum := 0
		for i, n := 0, v.Len(); i < n; i++ {
			num := s.sizeof(v.Index(i))
			if num < 0 {
				return -1
			}
			sum += num
		}
		return sum

	case reflect.String:
		sum := 0
		for i, n := 0, v.Len(); i < n; i++ {
			num := s.sizeof(v.Index(i))
			if num < 0 {
				return -1
			}
			sum += num
		}
		s.exNum += int(v.Type().Size())
		return sum

	case reflect.Ptr, reflect.Interface:
		s.exNum += int(v.Type().Size())
		if v.IsNil() {
			return 0
		}

		if _, ok := s.npm[v]; ok {
			return 0
		} else {
			s.npm[v] = true
		}
		return s.sizeof(v.Elem())
	case reflect.Struct:
		sum := 0
		for i, n := 0, v.NumField(); i < n; i++ {
			if v.Type().Field(i).Tag.Get("ss") == "-" {
				continue
			}
			num := s.sizeof(v.Field(i))
			if num < 0 {
				return -1
			}
			sum += num
		}
		return sum

	case reflect.Func, reflect.Chan:
		s.exNum += int(v.Type().Size())
		if v.IsNil() {
			return 0
		}
		return 0 //Temporary non handling func,chan.
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
		reflect.Int:
		return int(v.Type().Size())
	case reflect.Bool:
		return int(v.Type().Size())
	default:
		//fmt.Println("t.Kind() no found:", v.Kind())
	}

	return -1
}

//将对象转为指针
func PStr(v string) *string {
	return &v
}


func PInt(v int) *int {
	return &v
}

func PInt8(v int8) *int8 {
	return &v
}

func PInt16(v int16) *int16 {
	return &v
}

func PInt32(v int32) *int32 {
	return &v
}

func PInt64(v int64) *int64 {
	return &v
}

func PFloat32(v float32) *float32 {
	return &v
}

func PFloat64(v float64) *float64 {
	return &v
}

func PRune(v rune) *rune {
	return &v
}

func PBool(v bool) *bool {
	return &v
}

//all point to value
func P2Str(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func P2Int(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func P2Int8(v *int8) int8 {
	if v == nil {
		return 0
	}
	return *v
}

func P2Int16(v *int16) int16 {
	if v == nil {
		return 0
	}
	return *v
}

func P2Int32(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func P2Int64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func P2Float32(v *float32) float32 {
	if v == nil {
		return 0
	}
	return *v
}

func P2Float64(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func P2Rune(v *rune) rune {
	if v == nil {
		return 0
	}
	return *v
}

func P2Bool(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

