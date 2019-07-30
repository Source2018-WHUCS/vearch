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
	"github.com/tiglabs/caprice/search/query"
	"encoding/json"
)

type ExistsQuery struct {
	query.Query
}

func (qb *QueryBuilder) NewExistQuery() *ExistsQuery {
	return &ExistsQuery{}
}


func (e *ExistsQuery) UnmarshalJSON(data []byte) error {
	tmp := struct {
		Field string `json:"field"`
	}{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	e.Query = query.NewExistsQuery(tmp.Field)
	return nil
}
