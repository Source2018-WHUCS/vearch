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
	"github.com/tiglabs/baudengine/ps/engine/capricecb/ext_query"
	"github.com/tiglabs/baudengine/ps/engine/mapping"
	"github.com/tiglabs/baudengine/util/cbjson"
	"github.com/tiglabs/caprice"
	"github.com/tiglabs/caprice/search/query"

	"encoding/base64"
)

func NewQueryBuilder(mapping *mapping.IndexMapping) *QueryBuilder {
	return &QueryBuilder{mapping: mapping}
}

type QueryBuilder struct {
	mapping *mapping.IndexMapping
}

func (qb *QueryBuilder) ParseQueryAuto(data []byte) ([]query.Query, error) {
	var querys []query.Query

	if len(data) == 0 {
		querys = append(querys, caprice.NewMatchAllQuery())
		return querys, nil
	}

	if string(data)[0] == '[' {
		var result []json.RawMessage
		err := cbjson.Unmarshal(data, &result)
		if err != nil {
			return nil, err
		}

		for i:=0;i<len(result);i++ {
			query, err := qb.ParseQuery(result[i])
			if err != nil {
				return nil, err
			}
			querys = append(querys, query)
		}
		return querys, nil
	}

	query, err := qb.ParseQuery(data)
	if err != nil {
		return nil, err
	}
	querys = append(querys, query)
	return querys, nil
}

func (qb *QueryBuilder) ParseQuery(data []byte) (query.Query, error) {
	if len(data) == 0 {
		return caprice.NewMatchAllQuery(), nil
	}
	tmp := make(map[string]json.RawMessage)
	err := cbjson.Unmarshal(data, &tmp)
	if err != nil {
		return nil, err
	}

	rawMessage, hasQuery := tmp["query"]
	if hasQuery {
		return qb.ParseQuery(rawMessage)
	}

	rawMessage, hasFiltered := tmp["filtered"]
	if hasFiltered {
		filtered := qb.NewFilteredQuery()
		err = json.Unmarshal(rawMessage, filtered)
		if err != nil {
			return nil, err
		}
		return filtered.Query, nil
	}

	rawMessage, hasBool := tmp["bool"]
	if hasBool {
		bool := qb.NewBoolQuery()
		err = json.Unmarshal(rawMessage, bool)
		if err != nil {
			return nil, err
		}
		return bool.Query, nil
	}
	rawMessage, hasRange := tmp["range"]
	if hasRange {
		range_ := qb.NewRangeQuery()
		err = json.Unmarshal(rawMessage, range_)
		if err != nil {
			return nil, err
		}
		return range_.Query, nil
	}
	rawMessage, hasTerm := tmp["term"]
	if hasTerm {
		term := qb.NewTermQuery()
		err = json.Unmarshal([]byte(rawMessage), term)
		if err != nil {
			return nil, err
		}
		return term.Query, nil
	}
	rawMessage, hasTerms := tmp["terms"]
	if hasTerms {
		terms := qb.NewTermsQuery()
		err = json.Unmarshal([]byte(rawMessage), terms)
		if err != nil {
			return nil, err
		}
		return terms.Query, nil
	}
	rawMessage, hasPrefix := tmp["prefix"]
	if hasPrefix {
		prefix := qb.NewPrefixQuery()
		err = json.Unmarshal([]byte(rawMessage), prefix)
		if err != nil {
			return nil, err
		}
		return prefix.Query, nil
	}
	rawMessage, hasMatch := tmp["match"]
	if hasMatch {
		match := qb.NewMatchQuery()
		err = json.Unmarshal([]byte(rawMessage), match)
		if err != nil {
			return nil, err
		}
		return match.Query, nil
	}
	rawMessage, hasPhraseMatch := tmp["match_phrase"]
	if hasPhraseMatch {
		matchPhrase := qb.NewMatchPhraseQuery()
		err = json.Unmarshal([]byte(rawMessage), matchPhrase)
		if err != nil {
			return nil, err
		}
		return matchPhrase.Query, nil
	}
	rawMessage, hasPhrasePrefixMatch := tmp["match_phrase_prefix"]
	if hasPhrasePrefixMatch {
		matchPhrasePrefix := qb.NewMatchPhrasePrefixQuery()
		err = json.Unmarshal([]byte(rawMessage), matchPhrasePrefix)
		if err != nil {
			return nil, err
		}
		return matchPhrasePrefix.Query, nil
	}
	rawMessage, hasMatchAll := tmp["match_all"]
	if hasMatchAll {
		matchAll := qb.NewMatchAllQuery()
		err = json.Unmarshal([]byte(rawMessage), matchAll)
		if err != nil {
			return nil, err
		}
		return matchAll.Query, nil
	}

	rawMessage, hasQueryString := tmp["query_string"]
	if hasQueryString {
		queryStringQuery := qb.NewQueryStringQuery()
		err = json.Unmarshal([]byte(rawMessage), queryStringQuery)
		if err != nil {
			return nil, err
		}
		return queryStringQuery.Query, nil
	}

	rawMessage, hasWildCard := tmp["wildcard"]
	if hasWildCard {
		wildcard := qb.NewWildcardQuery()
		err = json.Unmarshal([]byte(rawMessage), wildcard)
		if err != nil {
			return nil, err
		}
		return wildcard.Query, nil
	}
	rawMessage, hasFuzzy := tmp["fuzzy"]
	if hasFuzzy {
		fuzzy := qb.NewFuzzyQuery()
		err = json.Unmarshal([]byte(rawMessage), fuzzy)
		if err != nil {
			return nil, err
		}
		return fuzzy.Query, nil
	}
	rawMessage, hasConstantScore := tmp["constant_score"]
	if hasConstantScore {
		score := qb.NewConstantScoreQuery()
		err = json.Unmarshal([]byte(rawMessage), score)
		if err != nil {
			return nil, err
		}
		return score.Query, nil
	}
	rawMessage, hasRegexp := tmp["regexp"]
	if hasRegexp {
		regexp := qb.NewRegexpQuery()
		err = json.Unmarshal([]byte(rawMessage), regexp)
		if err != nil {
			return nil, err
		}
		return regexp.Query, nil
	}
	rawMessage, hasDisMax := tmp["dis_max"]
	if hasDisMax {
		disMax := qb.NewDisMaxQuery()
		err = json.Unmarshal([]byte(rawMessage), disMax)
		if err != nil {
			return nil, err
		}
		return disMax.Query, nil
	}
	rawMessage, hasMultiMatch := tmp["multi_match"]
	if hasMultiMatch {
		multiMatch := qb.NewMultiMatch()
		err = json.Unmarshal(rawMessage, multiMatch)
		if err != nil {
			return nil, err
		}
		return multiMatch.Query, nil
	}
	rawMessage, hasIdsMatch := tmp["ids"]
	if hasIdsMatch {
		idsMatch := qb.NewIdsQuery()
		err = json.Unmarshal(rawMessage, idsMatch)
		if err != nil {
			return nil, err
		}
		return idsMatch.Query, nil
	}
	// TODO: geo bounding box is not supported now
	//rawMessage, hasGeoBox := tmp["geo_bounding_box"]
	//if hasGeoBox {
	//	geoBoxQuery := qb.NewGeoBoundingBoxQuery()
	//	err = json.Unmarshal(rawMessage, geoBoxQuery)
	//	if err != nil {
	//		return nil, err
	//	}
	//	return geoBoxQuery.Query, nil
	//}
	// TODO: geo distance is not supported now
	//rawMessage, hasGeoDistance := tmp["geo_distance"]
	//if hasGeoDistance {
	//	geoDistanceQuery := qb.NewGeoDistanceQuery()
	//	err = json.Unmarshal(rawMessage, geoDistanceQuery)
	//	if err != nil {
	//		return nil, err
	//	}
	//	return geoDistanceQuery.Query, nil
	//}
	rawMessage, hasExists := tmp["exists"]
	if hasExists {
		existsQuery := qb.NewExistQuery()
		err = json.Unmarshal(rawMessage, existsQuery)
		if err != nil {
			return nil, err
		}
		return existsQuery.Query, nil
	}
	rawMessage, hasMissing := tmp["missing"]
	if hasMissing {
		missQuery := qb.NewMissingQuery()

		err = json.Unmarshal(rawMessage, missQuery)
		if err != nil {
			return nil, err
		}
		return missQuery.Query, nil
	}

	rawMessage, hasWrapper := tmp["wrapper"]
	if hasWrapper {
		wrapMap := make(map[string]string)
		if err = json.Unmarshal(rawMessage, &wrapMap); err != nil {
			return nil, err
		}
		qBytes, err := base64.StdEncoding.DecodeString(wrapMap["query"])
		if err != nil {
			return nil, err
		}
		return qb.ParseQuery(qBytes)
	}

	if rawMessage, hasBoosting := tmp["boosting"]; hasBoosting {
		boostingQuery := qb.NewBoostingQuery()
		if err = json.Unmarshal(rawMessage, boostingQuery); err != nil {
			return nil, err
		}
		return boostingQuery.Query, nil
	}

	if rawMessage, hasBoosting := tmp["vector"]; hasBoosting {
		vectorQuery := ext_query.NewVectorQuery()
		if err = json.Unmarshal(rawMessage, vectorQuery); err != nil {
			return nil, err
		}
		return vectorQuery, nil
	}

	bytes, _ := json.Marshal(tmp)

	return nil, fmt.Errorf("invalid query by query:[%s]", string(bytes))
}
