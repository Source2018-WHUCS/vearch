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
	"fmt"

	"github.com/tiglabs/caprice/search/query"
)

type QueryStringQuery struct {
	*QueryBuilder
	query.Query
}

func NewQueryStringQuery() *QueryStringQuery {
	return &QueryStringQuery{}
}

func (qb *QueryBuilder) NewQueryStringQuery() *QueryStringQuery {
	return &QueryStringQuery{QueryBuilder: qb}
}

func (m *QueryStringQuery) SetQuery(query query.Query) {
	m.Query = query
}

func (m *QueryStringQuery) UnmarshalJSON(data []byte) error {
	tmp := struct {
		Query           string `json:"query"`
		DefaultField    string `json:"default_field"`
		DefaultOperator string `json:"default_operator,omitempty"`
	}{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	if tmp.Query == "" {
		return fmt.Errorf("QueryStringQuery query param can not set empty :[%s]", string(data))
	}

	stringQuery := query.NewQueryStringQuery(tmp.Query)

	stringQuery.SetDefaultAnalyzer(AdaptAnalyzer(m.mapping.DefaultAnalyzer))
	stringQuery.SetDefaultField(m.mapping.DefaultField)

	analyzers, err := m.mapping.GetFieldsAnalyzer()
	if err != nil {
		return err
	}
	stringQuery.SetFieldsAnalyzer(AdaptFieldsAnalyzer(analyzers))

	stringQuery.SetFieldsType(m.mapping.GetFieldsType())

	m.SetQuery(stringQuery)

	return nil
}
