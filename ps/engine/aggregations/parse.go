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

package aggregations

import (
	"fmt"
	"github.com/dustin/gojson"
	"github.com/tiglabs/caprice/search/aggregator"
	"github.com/tiglabs/log"
	"strings"
)

func ParseAggs(data []byte) ([]aggregator.Aggregator, error) {

	if log.IsDebugEnabled() {
		log.Debug(fmt.Sprintf("parse aggregations %s", strings.Replace(string(data), "\n", "", -1)))
	}

	if data == nil {
		return nil, nil
	}

	tmp := make(map[string]json.RawMessage)

	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return nil, err
	}

	var agg aggregator.Aggregator
	var subAggs []aggregator.Aggregator
	var result []aggregator.Aggregator

	for alias, val := range tmp {
		facets := make(map[string]json.RawMessage)
		err = json.Unmarshal(val, &facets)
		if err != nil {
			return nil, err
		}

		for name, vval := range facets {
			if name == "aggs" || name == "aggregations" {
				if subAggs, err = ParseAggs(vval); err != nil {
					return nil, err
				} else {
					continue
				}
			}

			if decodeFunc, found := aggregator.GlobalAggsDecoder.Get(name); found {
				if agg, err = decodeFunc(alias, vval); err != nil {
					return nil, err
				}
			} else {
				return nil, aggregator.UnsupportedAggErr(name)
			}

			result = append(result, agg)
		}

		if agg == nil {
			return nil, aggregator.ParamNotDefineErr(alias, string(val))
		}

		base := agg.B()
		if subAggs != nil {
			base.AddSub(subAggs)
		}

		if err = agg.Init(); err != nil {
			return nil, err
		}
	}

	return result, nil
}
