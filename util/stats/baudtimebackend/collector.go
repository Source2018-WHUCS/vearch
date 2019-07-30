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

package baudtimebackend

import (
	"expvar"
	"time"
	"strings"

	"vitess.io/vitess/go/stats"

	pb "git.jd.com/baudtime/baudtime/proxy/proxypb"
)

type metricsCollector struct {
	metrics []*pb.Series
	commLabels []pb.Label
	points []pb.Point
}

func (mc *metricsCollector) collectPoints(points ...pb.Point) []pb.Point {
	return append(mc.points, points...)
}

func (mc *metricsCollector) collectPoint(k int64, v float64) []pb.Point {
	return mc.collectPoints(pb.Point{k, v})
}

func (mc *metricsCollector) collectLabels(labels ...pb.Label) []pb.Label {
	return append(mc.commLabels, labels...)
}

func (mc *metricsCollector) collectNameLabels(name string, labels ...pb.Label) []pb.Label {
	nameLabels := append(mc.commLabels, pb.Label{nameLabelKey, name})
	return append(nameLabels, labels...)
}

func timeNow() int64 {
	return time.Now().UnixNano()/1e6
}

func (mc *metricsCollector) collectMetrics(name string, kv *expvar.KeyValue) {
	switch v := kv.Value.(type) {
	case *stats.Histogram:
		mc.collectHistogramMetrics(name, v)
	case *stats.Counter:
		mc.collectSingleMetrics(name, timeNow(), float64(v.Get()))
	case *stats.Gauge:
		mc.collectSingleMetrics(name, timeNow(), float64(v.Get()))
	case *stats.CounterDuration:
		mc.collectSingleMetrics(name, timeNow(), float64(v.Get()))
	case *stats.CountersWithSingleLabel:
		mc.collectMultiPointMetrics(name, v)
	case *stats.CountersWithMultiLabels:
		mc.collectMultiPointMetrics(name, v)
	case *stats.GaugesWithSingleLabel:
		mc.collectMultiPointMetrics(name, v)
	case *stats.GaugesWithMultiLabels:
		mc.collectMultiPointMetrics(name, v)
	case *stats.CounterFunc:
		mc.collectSingleMetrics(name, timeNow(), float64(v.F()))
	case *stats.GaugeFunc:
		mc.collectSingleMetrics(name, timeNow(), float64(v.F()))
	case *stats.CounterDurationFunc:
		mc.collectSingleMetrics(name, timeNow(), v.F().Seconds())
	case stats.FloatFunc:
		mc.collectSingleMetrics(name, timeNow(), float64(v()))
	case *stats.CountersFuncWithMultiLabels:
	case *stats.GaugesFuncWithMultiLabels:
	default:
		//other type v
	}
}

func (mc *metricsCollector) collectSingleMetrics(name string, ts int64, val float64) {
	s := &pb.Series{
		Labels: mc.collectNameLabels(name),
		Points: mc.collectPoint(ts, val),
	}
	mc.metrics = append(mc.metrics, s)
}

func makeLabels(labelNames []string, labelValsCombined string) []pb.Label {
	tags := make([]pb.Label, len(labelNames))
	labelVals := strings.Split(labelValsCombined, ".")
	for i, v := range labelVals {
		tags = append(tags, pb.Label{labelNames[i], v})
	}
	return tags
}

func (mc *metricsCollector) collectCountersWithLabelsMetrics(name string, counters *stats.CountersWithMultiLabels) {
	points := make([]pb.Point, len(counters.Counts()))
	for labelVals, val := range counters.Counts() {
		labels := makeLabels(counters.Labels(), labelVals)
		points = append(points, pb.Point{timeNow(), float64(val)})
		s := &pb.Series{
			Labels: mc.collectNameLabels(name, labels...),
			Points: points,
		}
		mc.metrics = append(mc.metrics, s)
	}
}

func (mc *metricsCollector) collectMultiPointMetrics(name string, v interface{}) {
	switch counters := v.(type) {
	case *stats.CountersWithSingleLabel:
		label := counters.Label()
		counterLabels := make([]pb.Label, len(counters.Counts()))
		points := make([]pb.Point, len(counters.Counts()))
		for labelVal, val := range counters.Counts() {
			counterLabels = append(counterLabels, pb.Label{label, labelVal})
			points = append(points, pb.Point{timeNow(), float64(val)})
		}
		s := &pb.Series{
			Labels: mc.collectNameLabels(name, counterLabels...),
			Points: points,
		}
		mc.metrics = append(mc.metrics, s)
	case *stats.GaugesWithSingleLabel:
		label := counters.Label()
		counterLabels := make([]pb.Label, len(counters.Counts()))
		points := make([]pb.Point, len(counters.Counts()))
		for labelVal, val := range counters.Counts() {
			counterLabels = append(counterLabels, pb.Label{label, labelVal})
			points = append(points, pb.Point{timeNow(), float64(val)})
		}
		s := &pb.Series{
			Labels: mc.collectNameLabels(name, counterLabels...),
			Points: points,
		}
		mc.metrics = append(mc.metrics, s)
	case *stats.CountersWithMultiLabels:
		points := make([]pb.Point, len(counters.Counts()))
		for labelVals, val := range counters.Counts() {
			labels := makeLabels(counters.Labels(), labelVals)
			points = append(points, pb.Point{timeNow(), float64(val)})
			s := &pb.Series{
				Labels: mc.collectNameLabels(name, labels...),
				Points: points,
			}
			mc.metrics = append(mc.metrics, s)
		}
	case *stats.GaugesWithMultiLabels:
		points := make([]pb.Point, len(counters.Counts()))
		for labelVals, val := range counters.Counts() {
			labels := makeLabels(counters.Labels(), labelVals)
			points = append(points, pb.Point{timeNow(), float64(val)})
			s := &pb.Series{
				Labels: mc.collectNameLabels(name, labels...),
				Points: points,
			}
			mc.metrics = append(mc.metrics, s)
		}
	default:
	}
}

func (mc *metricsCollector) collectHistogramMetrics(name string, histogram *stats.Histogram) {
	buckets := histogram.Buckets()
	for i, label := range histogram.Labels() {
		s := &pb.Series{
			Labels: mc.collectNameLabels(name + "_" + label),
			Points: mc.collectPoint(timeNow(), float64(buckets[i])),
		}
		mc.metrics = append(mc.metrics, s)
	}

	mcount := &pb.Series{
		Labels: mc.collectNameLabels(name + "_" + histogram.CountLabel()),
		Points: mc.collectPoint(timeNow(), float64((*histogram).Count())),
	}
	mc.metrics = append(mc.metrics, mcount)

	mt := &pb.Series{
		Labels: mc.collectNameLabels(name + "_" + histogram.TotalLabel()),
		Points: mc.collectPoint(timeNow(), float64((*histogram).Total())),
	}
	mc.metrics = append(mc.metrics, mt)
}
