// Copyright 2018 The Couchbase Authors.
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

package querycb

import (
	"bytes"
	"encoding/json"
	"github.com/spf13/cast"
	"github.com/tiglabs/caprice/search/query"
	"strconv"
)

type BoolQuery struct {
	*QueryBuilder
	query.Query
}

func (qb *QueryBuilder) NewBoolQuery() *BoolQuery {
	return &BoolQuery{QueryBuilder: qb}
}

func (b *BoolQuery) SetQuery(query query.Query) {
	b.Query = query
}

func (b *BoolQuery) UnmarshalJSON(data []byte) error {
	tmp := make(map[string]json.RawMessage)
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	var mustRaw, shouldRaw, filterRaw, mustNotRaw json.RawMessage
	var hasMust, hasShould, hasFilter, hasMustNot bool
	var must, must_not, should []query.Query

	mustRaw, hasMust = tmp["must"]
	if hasMust {
		qs, err := b.ParseQueryAuto(mustRaw)
		if err != nil {
			return err
		}
		for _, q := range qs {
			must = append(must, q)
		}
	}

	mustNotRaw, hasMustNot = tmp["must_not"]
	if hasMustNot {
		qs, err := b.ParseQueryAuto(mustNotRaw)
		if err != nil {
			return err
		}
		for _, q := range qs {
			must_not = append(must_not, q)
		}
	}

	filterRaw, hasFilter = tmp["filter"]
	if hasFilter {
		qs, err := b.ParseQueryAuto(filterRaw)
		if err != nil {
			return err
		}
		for _, q := range qs {
			must = append(must, q)
		}
	}

	shouldRaw, hasShould = tmp["should"]
	if hasShould {
		qs, err := b.ParseQueryAuto(shouldRaw)
		if err != nil {
			return err
		}
		for _, q := range qs {
			should = append(should, q)
		}
	}

	if len(must) == 0 && len(should) == 0 {
		adjustRaw, hasAdjust := tmp["adjust_pure_negative"]
		if hasAdjust {
			var isAdjust bool
			if err = json.Unmarshal(adjustRaw, &isAdjust); err != nil {
				return err
			}
			if isAdjust {
				must = append(must, query.NewMatchAllQuery())
			}
		} else { //default not set adjust_pure_negative and must is null so use match_all
			must = append(must, query.NewMatchAllQuery())
		}
	}

	q := query.NewBooleanQuery(must, should, must_not)
	if raw, hasBoost := tmp["boost"]; hasBoost {
		var boost float64
		err = json.Unmarshal(raw, &boost)
		if err != nil {
			return err
		}
		q.SetBoost(boost)
	}
	if raw, hasMin := tmp["minimum_should_match"]; hasMin {
		min, err := ParseMinShould(raw, len(should))
		if err != nil {
			return err
		}
		q.SetMinShould(min)
	}
	b.Query = q
	return nil
}

/*
   Integer                 3
   Negative integer        -2
   Percentage              75%
   Negative percentage     -25%
   Combination             3<90%
   Multiple combinations   2<-25% 9<-3 [baudengine not support now]
*/
func parseMinimumShouldMatch(data []byte, maxShould int) (min float64, err error) {
	if bytes.ContainsAny(data, "<") {
		ms := bytes.Split(data, []byte{'<'})
		if len(ms) != 2 {
			err = ErrInvalidMinMatch
			return
		}
		var m1, m2 float64
		m1, err = parseMinimumShouldMatch(ms[0], maxShould)
		if err != nil {
			return 0.0, err
		}
		m2, err = parseMinimumShouldMatch(ms[1], maxShould)
		if err != nil {
			return 0.0, err
		}
		if maxShould <= int(m1) {
			if m2 < m1 {
				min = m2
			} else {
				min = m1
			}
		} else {
			min = m2
		}
		return
	} else if bytes.ContainsAny(data, "%") {
		data = data[:len(data)-1]
		min, err = cast.ToFloat64E(string(data))
		if err != nil {
			return
		}

		if min < -100 || min > 100 {
			return 0.0, ErrInvalidMinMatch
		}
		min = min * 0.01 * float64(maxShould)
		if min < 0 {
			min = float64(maxShould + int(min))
		}
		return min, nil
	} else {
		min, err = strconv.ParseFloat(string(data), 64)
		if err != nil {
			return
		}
		if min < 0 {
			min = float64(maxShould) + min
		}
	}

	return
}

func ParseMinShould(data []byte, maxShould int) (float64, error) {
	min, err := parseMinimumShouldMatch(data, maxShould)
	if err != nil {
		return 0.0, err
	}
	min = float64(int(min))
	return min, nil
}
