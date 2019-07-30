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

package init

import (
	"github.com/tiglabs/baudengine/proto/request"
	"github.com/tiglabs/baudengine/proto/response"
	"github.com/tiglabs/baudengine/util/metrics/mserver"
	"github.com/tiglabs/caprice/search/aggregator"
	"github.com/tiglabs/caprice/search/aggregator/buckets"
	"github.com/tiglabs/caprice/search/aggregator/metrics"
	"github.com/tiglabs/caprice/search/sort"
	"github.com/vmihailenco/msgpack"
)

//in serizable by rpc you must set your entity in this init file
func init() {
	list := []interface{}{
		//aggs bucket
		(*aggregator.Bucket)(nil),
		(*buckets.DateHistogram)(nil),
		(*buckets.DateRange)(nil),
		(*buckets.Terms)(nil),
		(*buckets.Range)(nil),
		(*buckets.Missing)(nil),
		(*buckets.Histogram)(nil),
		(*buckets.GeohashGrid)(nil),
		(*buckets.IpRange)(nil),

		//aggs metrics
		(*metrics.Count)(nil),
		(*metrics.Sum)(nil),
		(*metrics.Stats)(nil),
		(*metrics.Min)(nil),
		(*metrics.Max)(nil),
		(*metrics.GeoCentroid)(nil),
		(*metrics.GeoBounds)(nil),
		(*metrics.ExtendedStats)(nil),
		(*metrics.Cardinality)(nil),
		(*metrics.Avg)(nil),

		//req resp serizable
		(*request.SearchRequest)(nil),
		(*request.ObjRequest)(nil),
		(*response.SearchResponse)(nil),
		(response.SearchResponses)(nil),
		(*response.WriteResponse)(nil),
		(*response.ObjResponse)(nil),
		(*mserver.ServerStats)(nil),
		(*response.DocResult)(nil),

		//sort
		(*sort.FloatSortValue)(nil),
		(*sort.StringSortValue)(nil),
		(*sort.GeoDistanceSortValue)(nil),
		(*sort.InfinitySortValue)(nil),
		(*sort.IntSortValue)(nil),
		(*sort.DateSortValue)(nil),
	}

	for i, v := range list {
		if int8(i+1) < 0 {
			panic("fuck too may serizables ")
		}
		msgpack.RegisterExt(int8(i+1), v)
	}
}
