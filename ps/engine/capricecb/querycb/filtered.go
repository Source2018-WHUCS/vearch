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
	"github.com/tiglabs/baudengine/util/cbjson"
	"github.com/tiglabs/caprice/search/query"
)

type FilteredQuery struct {
	*QueryBuilder
	query.Query
}

func (qb *QueryBuilder) NewFilteredQuery() *FilteredQuery {
	return &FilteredQuery{QueryBuilder: qb}
}

func (b *FilteredQuery) SetQuery(query query.Query) {
	b.Query = query
}

func (b *FilteredQuery) UnmarshalJSON(data []byte) error {
	tmp := make(map[string]json.RawMessage)
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	var mustRaw, filterRaw json.RawMessage
	var hasMust, hasFilter bool

	bool := make(map[string]json.RawMessage)
	mustRaw, hasMust = tmp["query"]
	if hasMust {
		bool["must"] = mustRaw
	}
	filterRaw, hasFilter = tmp["filter"]
	if hasFilter {
		bool["filter"] = filterRaw
	}

	newData, err := cbjson.Marshal(bool)
	if err != nil {
		return err
	}

	filtered := b.QueryBuilder.NewBoolQuery()
	err = json.Unmarshal(newData, filtered)
	if err != nil {
		return err
	}
	b.Query = filtered.Query

	return nil
}
