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

package ext_query

import (
	"fmt"
	"github.com/tiglabs/baudengine/util/bytes"
	"github.com/tiglabs/caprice/index"
	"github.com/tiglabs/caprice/search"
	"github.com/tiglabs/caprice/search/context"
	"github.com/tiglabs/caprice/search/match"
	"github.com/tiglabs/caprice/search/query"
	"github.com/tiglabs/caprice/search/scorer"
	"github.com/tiglabs/caprice/util/size"
	"reflect"
)

/**
{
    "query": {
        "vector": {
        	"field":"feature",
            "value": [1,2,3,4,5,6],
            "method":"dp",
            "sort":"desc",
            "boost":1
        }
    }
}
 */
type vectorQuery struct {
	ctx      context.Context
	Field    string       `json:"field"`
	Value    []float32    `json:"value"`
	Method   string       `json:"method,omitempty"`
	Order    string       `json:"order,omitempty"`
	BoostVal *query.Boost `json:"boost,omitempty"`
}

// VectorQuery creates a Query which will
// match all documents in the index.
func NewVectorQuery() *vectorQuery {
	return &vectorQuery{}
}

func (q *vectorQuery) SetBoost(b float64) {
	boost := query.Boost(b)
	q.BoostVal = &boost
}

func (q *vectorQuery) Boost() float64 {
	return q.BoostVal.Value()
}

func (q *vectorQuery) Searcher(ctx context.Context, i index.IndexReader) (search.Searcher, error) {
	if q.ctx != nil {
		ctx = q.ctx
	}
	return NewVectorSearcher(ctx, i, q)
}

//Search
var reflectStaticSizeMatchAllSearcher int

func init() {
	var mas VectorSearcher
	reflectStaticSizeMatchAllSearcher = int(reflect.TypeOf(mas).Size())
}

type VectorSearcher struct {
	indexReader index.IndexReader
	reader      index.DocIDReader
	scorer      *scorer.ConstantScorer
	count       uint64
	query       *vectorQuery
}

func NewVectorSearcher(ctx context.Context, indexReader index.IndexReader, query *vectorQuery) (*VectorSearcher, error) {
	reader, err := indexReader.DocIDReaderAll()
	if err != nil {
		return nil, err
	}
	count, err := indexReader.DocCount()
	if err != nil {
		_ = reader.Close()
		return nil, err
	}
	scorer := scorer.NewConstantScorer(ctx, 1.0, query.Boost())
	return &VectorSearcher{
		indexReader: indexReader,
		reader:      reader,
		scorer:      scorer,
		count:       count,
		query:       query,
	}, nil
}

func (s *VectorSearcher) Size() int {
	return reflectStaticSizeMatchAllSearcher + size.SizeOfPtr +
		s.reader.Size() +
		s.scorer.Size()
}

func (s *VectorSearcher) Count() uint64 {
	return s.count
}

func (s *VectorSearcher) Next(ctx *search.SearchContext) (*match.DocumentMatch, error) {
	id, err := s.reader.Next()

	if err != nil {
		return nil, err
	}

	if id.Invalid() {
		return nil, nil
	}

	// score match
	docMatch, err := s.vectorScore(s.scorer.Score(ctx, id))
	if err != nil {
		return nil, err
	}

	// return doc match
	return docMatch, nil

}

func distance(q, d []float32) (float32, error) {
	if len(q) != len(d) {
		return 0, fmt.Errorf("vector lenght not same document:[%d] query:[%d]", len(d), len(q))
	}
	var dist float32 = 0
	for i := 0; i < len(q); i++ {
		dist += q[i] * d[i]
	}
	return dist, nil
}

func (s *VectorSearcher) Advance(ctx *search.SearchContext, ID index.IndexInternalID) (*match.DocumentMatch, error) {
	id, err := s.reader.Advance(ID)
	if err != nil {
		return nil, err
	}

	if id.Invalid() {
		return nil, nil
	}

	// score match
	docMatch, err := s.vectorScore(s.scorer.Score(ctx, id))
	if err != nil {
		return nil, err
	}

	// return doc match
	return docMatch, nil
}

func (s *VectorSearcher) Close() error {
	return s.reader.Close()
}

func (s *VectorSearcher) Weight() float64 {
	return s.scorer.Weight()
}

func (s *VectorSearcher) SetQueryNorm(qnorm float64) {
	s.scorer.SetQueryNorm(qnorm)
}

func (s *VectorSearcher) Min() int {
	return 0
}

func (s *VectorSearcher) DocumentMatchPoolSize() int {
	return 1
}

func (s *VectorSearcher) vectorScore(docMatch *match.DocumentMatch) (*match.DocumentMatch, error) {

	dvReader, err := s.indexReader.DocValueReader()
	if err != nil {
		return nil, err
	}

	values, err := dvReader.DocValues(docMatch.IndexInternalID, s.query.Field)

	if values == nil || !values.Next() {
		return docMatch, nil
	}

	dv := bytes.ArrayByteFloat(values.Value().Value)

	f, err := distance(s.query.Value, dv)
	if err != nil {
		return nil, err
	}

	docMatch.Score = float64(f) * s.query.Boost()

	return docMatch, nil
}
