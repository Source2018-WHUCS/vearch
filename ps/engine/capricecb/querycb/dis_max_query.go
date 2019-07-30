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

package querycb

import (
	"encoding/json"
	"errors"

	"github.com/tiglabs/caprice/search/query"
)

type DisMaxQuery struct {
	*QueryBuilder
	query.Query
}

func (qb *QueryBuilder) NewDisMaxQuery() *DisMaxQuery {
	return &DisMaxQuery{QueryBuilder: qb}
}

func (d *DisMaxQuery) SetQuery(query query.Query) {
	d.Query = query
}

func (d *DisMaxQuery) UnmarshalJSON(data []byte) error {
	tmp := struct {
		Queries    []json.RawMessage `json:"queries"`
		Boost      *Boost            `json:"boost,omitempty"`
		TieBreaker *float64          `json:"tie_breaker,omitempty"`
	}{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	var should []query.Query
	for _, query := range tmp.Queries {
		q, err := d.ParseQuery([]byte(query))
		if err != nil {
			return err
		}
		should = append(should, q)
	}
	if should == nil {
		return errors.New("invalid dis max query")
	}
	q := query.NewDisjunctionQuery(should)
	if tmp.Boost != nil {
		q.SetBoost(tmp.Boost.Value())
	}
	d.Query = q
	return nil
}
