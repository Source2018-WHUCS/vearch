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
	"reflect"

	"errors"

	"github.com/tiglabs/caprice/search/query"
)

type MatchPhraseQuery struct {
	*QueryBuilder
	query.Query
}

func (qb *QueryBuilder) NewMatchPhraseQuery() *MatchPhraseQuery {
	return &MatchPhraseQuery{QueryBuilder: qb}
}

func (m *MatchPhraseQuery) SetQuery(query query.Query) {
	m.Query = query
}

func (m *MatchPhraseQuery) parseMatch(data []byte) (query.Query, error) {
	tmp := make(map[string]struct {
		Query           interface{} `json:"query"`
		Type            string      `json:"type,omitempty"`
		MaxExpansions   *int        `json:"max_expansions,omitempty"`
		CutoffFrequency *float64    `json:"cutoff_frequency,omitempty"`
		ZeroTermsQuery  string      `json:"zero_terms_query,omitempty"`
		Analyzer        string      `json:"analyzer,omitempty"`
		Boost           *Boost      `json:"boost,omitempty"`
	})
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return nil, err
	}
	for field, fv := range tmp {
		if fv.Analyzer == "" {
			fv.Analyzer = m.mapping.GetAnalyzerName(field)
		}

		switch fv.Type {
		case "boolean", "phrase", "":
			match, err := parseQueryValue(fv.Query)
			if err != nil {
				return nil, err
			}

			var q query.Query

			if fv.Analyzer == "keyword" {
				temp := query.NewMatchQuery(field, match)
				if analyzer, err := m.mapping.GetAnalyzer(fv.Analyzer); err != nil {
					return nil, err
				} else {
					temp.SetAnalyzer(AdaptAnalyzer(analyzer))
				}
				q = temp
			} else {
				temp := query.NewMatchPhraseQuery(field, match)
				if analyzer, err := m.mapping.GetAnalyzer(fv.Analyzer); err != nil {
					return nil, err
				} else {
					temp.SetAnalyzer(AdaptAnalyzer(analyzer))
				}
				q = temp
			}

			if fv.Boost != nil {
				q.(query.BoostableQuery).SetBoost(fv.Boost.Value())
			}
			q.(query.FieldableQuery).SetField(field)

			return q, nil
		default:
			return nil, fmt.Errorf("invalid match type %s", fv.Type)
		}
	}
	return nil, errors.New("invalid match query")
}

func (m *MatchPhraseQuery) UnmarshalJSON(data []byte) error {
	tmp := make(map[string]interface{})
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	var q query.Query
	for field, fv := range tmp {
		val := reflect.ValueOf(fv)
		typ := val.Type()
		switch typ.Kind() {
		case reflect.Map:
			q, err = m.parseMatch(data)
		default:
			match, err := parseQueryValue(fv)
			if err != nil {
				return err
			}

			mq := query.NewMatchPhraseQuery(field, match)

			if analyzer, err := m.mapping.GetAnalyzer(m.mapping.GetAnalyzerName(field)); err != nil {
				return err
			} else {
				mq.SetAnalyzer(AdaptAnalyzer(analyzer))
			}

			mq.SetField(field)
			q = mq
		}
	}
	if q != nil {
		m.SetQuery(q)
		return nil
	}
	return err
}
