// Copyright 2016 PingCAP, Inc.
// Modified work copyright (C) 2018 The ChuBao Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.
package util

import (
	"strconv"
	"strings"
	"github.com/juju/errors"
	"bytes"
	"html/template"
)

//StringSlice is more friendly to json encode/decode
type StringSlice []string

// MarshalJSON returns the size as a JSON string.
func (s StringSlice) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(strings.Join(s, ","))), nil
}

// UnmarshalJSON parses a JSON string into the bytesize.
func (s *StringSlice) UnmarshalJSON(text []byte) error {
	data, err := strconv.Unquote(string(text))
	if err != nil {
		return err
	}
	if len(data) == 0 {
		*s = nil
		return nil
	}
	*s = strings.Split(data, ",")
	return nil
}

// StringComparatorDesc provides a fast comparison on strings
func StringComparatorDesc(a, b interface{}) int {
	s1 := a.(string)
	s2 := b.(string)
	min := len(s2)
	if len(s1) < len(s2) {
		min = len(s1)
	}
	diff := 0
	for i := 0; i < min && diff == 0; i++ {
		diff = int(s1[i]) - int(s2[i])
	}
	if diff == 0 {
		diff = len(s1) - len(s2)
	}
	if diff < 0 {
		return 1
	}
	if diff > 0 {
		return -1
	}
	return 0
}

func ExeRepalce(tempName string, temp string,data interface{}) (j string, err error) {
	tpl, err := template.New(tempName).Parse(temp)
	if err != nil {
		return "", errors.Trace(err)
	}
	var b bytes.Buffer
	err = tpl.Execute(&b, data)
	if err != nil {
		return "", errors.Trace(err)
	}
	s:=b.String()
	return  s,nil
}
