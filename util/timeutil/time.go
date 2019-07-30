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
	"errors"
	"time"
)

const DEFAULT_FORMAT = "2006-01-02 15:04:05"

func Since(t time.Time) time.Duration {
	return time.Now().Sub(t)
}

func ParseTime(t string, patterns ...string) (time.Time, error) {
	if len(patterns) == 0 {
		return time.Parse(DEFAULT_FORMAT, t)
	}

	for _, pattern := range patterns {
		if date, err := time.Parse(pattern, t); err == nil {
			return date, nil
		}
	}

	return time.Time{}, errors.New("Unable to parse the time: " + t)
}

func FormatTime(t time.Time, patterns ...string) string {
	if len(patterns) == 0 {
		return t.Format(DEFAULT_FORMAT)
	}

	return t.Format(patterns[0])
}

func FormatNow(patterns ...string) string {
	return FormatTime(time.Now(), patterns...)
}
