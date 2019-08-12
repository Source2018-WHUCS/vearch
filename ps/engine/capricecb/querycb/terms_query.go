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

type TermsQuery struct {
	*QueryBuilder
	query.Query
}

func (qb *QueryBuilder) NewTermsQuery() *TermsQuery {
	return &TermsQuery{QueryBuilder:qb}
}

func (ts *TermsQuery) UnmarshalJSON(data []byte) error {
	tmp := make(map[string][]interface{})
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	for field, terms := range tmp {
		var querys []query.Query
		for _, term := range terms {

			t, _, err := ts.NewTermQuery().parse(field,term)
			if err != nil {
				return err
			}
			sq := query.NewTermQuery(field, t)
			querys = append(querys, sq)
		}
		q := query.NewBooleanQuery(nil, querys, nil)
		q.SetMinShould(1.0)
		ts.Query = q
		break
	}
	return nil
}
