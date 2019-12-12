package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"time"
)

var masterCallBack func(masterMonitor *masterMonitor)

type custom struct {
	Cpu      prometheus.Gauge
	Mem      prometheus.Gauge
	Fs       prometheus.Gauge
	Net      prometheus.Gauge
	Gc       prometheus.Gauge
	Routines prometheus.Gauge
}

type MasterMonitor struct {
	*custom
	ServerNum    prometheus.Gauge
	DBNum        prometheus.Gauge
	SpaceNum     prometheus.Gauge
	SpaceDoc     *prometheus.GaugeVec
	PartitionNum prometheus.Gauge
}

func RegisterMaster(call func(masterMonitor *MasterMonitor)) {
	masterCallBack = call
}

func init() {
	custom := newCustom()
	mm := &MasterMonitor{
		custom:       custom,
		ServerNum:    newGauge("serverNum", "server number"),
		DBNum:        newGauge("dbNum", "database number"),
		SpaceNum:     newGauge("spaceNum", "space number"),
		SpaceDoc:     newGaugeVec("spaceDoc", "space document number", "db_name", "table_name", "table_id"),
		PartitionNum: newGauge("spaceNum", "space number"),
	}

	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Errorf("monitor has err:[%v]", e)
			}
		}()
		for {
			if masterCallBack != nil {
				masterCallBack(mm)
			}
			time.Sleep(time.Second * 15)
		}
	}()
}

func newCustom() *custom {
	custom := &custom{
		Cpu:      newGauge("cpu", "cpu"),
		Mem:      newGauge("mem", "memory"),
		Fs:       newGauge("fs", "file disk"),
		Net:      newGauge("net", "net"),
		Gc:       newGauge("gc", "go gc"),
		Routines: newGauge("routines", "go routines"),
	}
	prometheus.MustRegister(custom.Cpu, custom.Mem, custom.Fs, custom.Net, custom.Gc, custom.Routines)
	return custom
}

func newCount(name, help string, ) *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: name,
			Help: help,
		},
		nil,
	)
}

func newGaugeVec(Name, help string, labels ...string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: Name,
		Help: help,
	}, labels)

}

func newGauge(Name, help string) prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Name: Name,
		Help: help,
	})

}
