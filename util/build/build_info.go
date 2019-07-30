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

package build

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/tiglabs/baudengine/util/timeutil"
)

// The following fields are populated at buildtime with -ldflags -X.
var (
	AppName     = "unknown"
	AppVersion  = "unknown"
	GitRevision = "unknown"
	User        = "unknown"
	BuildTime   = "unknown"
	Tag         = "unknown"
	Type        = "unknown"
)

// Info build info
type Info struct {
	AppName     string
	AppVersion  string
	GitRevision string
	User        string
	BuildTime   string
	Tag         string
	Type        string // "release" or "snapshot"...
	GoVersion   string
	Platform    string
}

// NewInfo create info object
func NewInfo() *Info {
	return &Info{
		AppName:     AppName,
		AppVersion:  AppVersion,
		GitRevision: GitRevision,
		User:        User,
		BuildTime:   BuildTime,
		Tag:         Tag,
		Type:        Type,
		GoVersion:   runtime.Version(),
		Platform:    fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH),
	}
}

// Timestamp parses the utcTime string and returns the number of seconds since epoch.
func (b *Info) Timestamp() (int64, error) {
	val, err := timeutil.ParseTime(b.BuildTime)
	if err != nil {
		return 0, err
	}
	return val.Unix(), nil
}

func (b *Info) String() string {
	return fmt.Sprintf(`
AppName: %v
AppVersion: %v
GitRevision: %v
User: %v
Tag: %v
Type: %v
GoVersion: %v
Platform: %v
BuildTime: %v
`,
		b.AppName,
		b.AppVersion,
		b.GitRevision,
		b.User,
		b.Tag,
		b.Type,
		b.GoVersion,
		b.Platform,
		b.BuildTime)
}

var info *Info

func init() {
	info = NewInfo()
}

// Version returns a multi-line version information
func Version() string {
	return info.String()
}

// IsRelease return build is release verison
func IsRelease() bool {
	return strings.HasPrefix(info.Type, "release")
}

// GetInfo return build info
func GetInfo() Info {
	return *info
}
