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
	"errors"
	"regexp"
	"strconv"

	"github.com/tiglabs/caprice/search/query"
)

var boostReg = regexp.MustCompile(`\^`)
var wildcardReg = regexp.MustCompile(`\*|\?`)

type MultiMatch struct {
	*QueryBuilder
	query.Query
}

func (qb *QueryBuilder) NewMultiMatch() *MultiMatch {
	return &MultiMatch{QueryBuilder: qb}
}

func (m *MultiMatch) SetQuery(query query.Query) {
	m.Query = query
}

func (m *MultiMatch) UnmarshalJSON(data []byte) error {
	tmp := struct {
		Query          string          `json:"query"`
		Type           *string         `json:"type,omitempty"`
		Fields         []string        `json:"fields"`
		TieBreaker     *float64        `json:"tie_breaker,omitempty"`
		Operator       *string         `json:"operator,omitempty"`
		Analyzer       *string         `json:"analyzer,omitempty"`
		MinShouldMatch json.RawMessage `json:"minimum_should_match, omitempty"`
	}{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	var querys []query.Query
	for _, field := range tmp.Fields {
		list := boostReg.Split(field, -1)
		num := len(list)
		if num == 0 || num > 2 {
			return errors.New("invalid query")
		}
		var fieldName string
		var boost *Boost
		for i, l := range list {
			if i == 0 {
				fieldName = l
			} else {
				b, err := strconv.ParseFloat(l, 64)
				if err != nil {
					return err
				}
				boost = NewBoost(b)
			}
			if wildcardReg.MatchString(fieldName) {
				return errors.New("not support wildcard field")
			}
		}
		q := query.NewMatchQuery(field, tmp.Query)
		q.SetField(fieldName)
		if boost != nil {
			q.SetBoost(boost.Value())
		}
		var tokenizerName string
		if tmp.Analyzer != nil {
			tokenizerName = *tmp.Analyzer
		}
		if analyzer, err := m.mapping.GetAnalyzer(tokenizerName); err != nil {
			return err
		} else {
			q.SetAnalyzer(AdaptAnalyzer(analyzer))
		}

		if tmp.Operator != nil {
			switch *tmp.Operator {
			case "and":
				q.SetOperator(query.MatchQueryOperatorAnd)
			case "or":
				q.SetOperator(query.MatchQueryOperatorOr)
			default:
				return errors.New("invalid operator")
			}
		}

		if len(tmp.MinShouldMatch) > 0 {
			q.SetMinMatchRaw(tmp.MinShouldMatch)
		}
		querys = append(querys, q)
	}

	m.SetQuery(query.NewBooleanQuery(nil, querys, nil))
	return nil
}
