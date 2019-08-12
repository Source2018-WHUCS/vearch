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
	"encoding/json"

	"github.com/tiglabs/caprice/search/query"
)

type ConstantScoreQuery struct {
	*QueryBuilder
	query.Query
	BoostVal *Boost
}

func (qb *QueryBuilder) NewConstantScoreQuery() *ConstantScoreQuery {
	return &ConstantScoreQuery{QueryBuilder: qb}
}

func (c *ConstantScoreQuery) SetBoost(boost float64) {
	b := Boost(boost)
	c.BoostVal = &b
}

func (c *ConstantScoreQuery) SetQuery(query query.Query) {
	c.Query = query
}

func (c *ConstantScoreQuery) UnmarshalJSON(data []byte) error {
	tmp := struct {
		Filter json.RawMessage `json:"filter"`
		Boost  *Boost          `json:"boost,omitempty"`
	}{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	q, err := c.ParseQuery(tmp.Filter)
	if err != nil {
		return err
	}
	c.Query = q
	c.BoostVal = tmp.Boost
	return nil
}
