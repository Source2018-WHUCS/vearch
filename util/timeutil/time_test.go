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

package timeutil

import (
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	dt := "2018-01-12 14:08:08"
	edt := time.Date(2018, time.January, 12, 14, 8, 8, 0, time.Local)
	if rs, err := ParseTime(dt); err != nil {
		t.Error(err.Error())
	} else if edt.Nanosecond() != rs.Nanosecond() {
		t.Error("parse time result error.")
	}

	if rs, err := ParseTime(dt, DEFAULT_FORMAT); err != nil {
		t.Error(err.Error())
	} else if edt.Nanosecond() != rs.Nanosecond() {
		t.Error("parse time result error.")
	}
}

func TestFormatTime(t *testing.T) {
	dt := "2018-01-12 14:08:08"
	edt := time.Date(2018, time.January, 12, 14, 8, 8, 0, time.Local)

	if dt != FormatTime(edt) {
		t.Error("format time result error.")
	}

	if dt != FormatTime(edt, DEFAULT_FORMAT) {
		t.Error("format time result error.")
	}

	t.Log(FormatNow())
}
