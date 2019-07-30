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
	"runtime/debug"
	"fmt"
	"github.com/tiglabs/log"
)

func PanicHandler(cb func()) {
	if r := recover(); r != nil {
		stackTrace := debug.Stack()
		log.Error(fmt.Sprintf("panic cause %v stack trace:\n%v", r, string(stackTrace)))
		if cb != nil {
			cb()
		}
	}
}