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

	"github.com/tiglabs/caprice/search/query"
)

type IdsQuery struct {
	query.Query     `json:"-"`

}

func (qb *QueryBuilder) NewIdsQuery() *IdsQuery {
	return &IdsQuery{}
}

func (i *IdsQuery) SetQuery(query query.Query) {
	i.Query = query
}

func (i *IdsQuery) UnmarshalJSON(data []byte) error {
	tmp := struct {
		Type  string    `json:"type,omitempty"`
		Ids   []string  `json:"values"`
	}{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	q := query.NewDocIDQuery(tmp.Ids)
	i.Query = q
	return nil
}
