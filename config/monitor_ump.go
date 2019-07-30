// +build ump

package config

import (
	. "github.com/tiglabs/baudengine/util/monitoring"
	"github.com/tiglabs/baudengine/util/ump"
	"sync"
)

var once sync.Once

func newMonitor(conf *Config, key string) Monitor {
	once.Do(func() {
		ump.InitUmp(conf.Global.Name)
	})
	return (&ump.UmpMonitor{}).New(key)
}
