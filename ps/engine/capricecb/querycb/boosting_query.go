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
	"fmt"
	"github.com/tiglabs/caprice/search/query"
)

type BoostingQuery struct {
	*QueryBuilder
	query.Query
}

func (qb *QueryBuilder)  NewBoostingQuery() *BoostingQuery {
	return &BoostingQuery{QueryBuilder:qb}
}

func (b *BoostingQuery) UnmarshalJSON(data []byte) error {
	tmp := struct {
		Positive      json.RawMessage `json:"positive"`
		Negative      json.RawMessage `json:"negative"`
		NegativeBoost float64         `json:"negative_boost"`
	}{}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	if tmp.Positive == nil {
		return fmt.Errorf("BoostingQuery positive not exist")
	}
	positive, err := b.ParseQuery(tmp.Positive)
	if err != nil {
		return err
	}
	var negative query.Query
	if tmp.Negative != nil {
		negative, err = b.ParseQuery(tmp.Negative)
		if err != nil {
			return err
		}
	}
	q := query.NewBoostingQuery(positive, negative)
	q.SetBoost(tmp.NegativeBoost)
	b.Query = q
	return nil
}
