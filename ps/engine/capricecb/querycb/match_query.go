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
	"fmt"
	"reflect"
	"strings"

	"github.com/blevesearch/bleve/numeric"
	"github.com/tiglabs/caprice/search/query"
)

type MatchQuery struct {
	*QueryBuilder
	query.Query
}

func (qb *QueryBuilder) NewMatchQuery() *MatchQuery {
	return &MatchQuery{QueryBuilder: qb}
}

func (m *MatchQuery) SetQuery(query query.Query) {
	m.Query = query
}

func (m *MatchQuery) parseMatch(data []byte) (query.Query, error) {
	tmp := make(map[string]struct {
		Query           interface{} `json:"query"`
		Type            string      `json:"type,omitempty"`
		Operator        string      `json:"operator,omitempty"`
		MaxExpansions   *int        `json:"max_expansions,omitempty"`
		CutoffFrequency *float64    `json:"cutoff_frequency,omitempty"`
		ZeroTermsQuery  string      `json:"zero_terms_query,omitempty"`
		PrefixLength    *int        `json:"prefix_length,omitempty"`
		Fuzziness       *int        `json:"fuzziness,omitempty"`
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
		case "phrase":
			if fv.Analyzer != "keyword" {
				q, err := m.QueryBuilder.NewMatchPhraseQuery().parseMatch(data)
				if err != nil {
					return nil, err
				}
				return q, nil
			}
			fallthrough
		case "boolean", "":
			match, err := parseQueryValue(fv.Query)
			if err != nil {
				return nil, err
			}
			q := query.NewMatchQuery(field, match)
			if fv.Boost != nil {
				q.SetBoost(fv.Boost.Value())
			}
			q.SetField(field)
			if fv.Fuzziness != nil {
				q.SetFuzziness(*fv.Fuzziness)
			}
			if fv.PrefixLength != nil {
				q.SetPrefix(*fv.PrefixLength)
			}
			if fv.Operator != "" {
				switch strings.ToLower(fv.Operator) {
				case "and":
					q.SetOperator(query.MatchQueryOperatorAnd)
				case "or":
					q.SetOperator(query.MatchQueryOperatorOr)
				default:
					return nil, fmt.Errorf("invalid operator %s", strings.ToLower(fv.Operator))
				}
			}

			if analyzer, err := m.mapping.GetAnalyzer(fv.Analyzer); err != nil {
				return nil, err
			} else {
				q.SetAnalyzer(AdaptAnalyzer(analyzer))
			}

			return q, nil
		default:
			return nil, fmt.Errorf("invalid match type %s", fv.Type)
		}
	}
	return nil, errors.New("invalid match query")
}

func (m *MatchQuery) UnmarshalJSON(data []byte) error {
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
			mq := query.NewMatchQuery(field, match)
			mq.SetField(field)
			q = mq
		}
	}
	if q != nil {
		switch q.(type) {
		case *query.MatchQuery:
			if q.(*query.MatchQuery).Analyzer == nil {
				q.(*query.MatchQuery).SetAnalyzer(AdaptAnalyzer(m.mapping.DefaultAnalyzer))
			}
		case *query.MatchPhraseQuery:
			if q.(*query.MatchPhraseQuery).Analyzer == nil {
				q.(*query.MatchPhraseQuery).SetAnalyzer(AdaptAnalyzer(m.mapping.DefaultAnalyzer))
			}
		default:
			panic(q)
		}

		m.SetQuery(q)
		return nil
	}
	return err
}

func parseQueryValue(data interface{}) (string, error) {
	val := reflect.ValueOf(data)
	typ := val.Type()
	switch typ.Kind() {
	case reflect.String:
		return val.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return parseQueryValue(float64(val.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return parseQueryValue(float64(val.Uint()))
	case reflect.Float64:
		numberInt64 := numeric.Float64ToInt64(val.Float())
		prefixCoded := numeric.MustNewPrefixCodedInt64(numberInt64, 0)
		return parseQueryValue(string(prefixCoded))
	case reflect.Bool:
		if val.Bool() {
			return "T", nil
		}
		return "F", nil
	default:
		return "", errors.New("invalid match query")
	}
}
