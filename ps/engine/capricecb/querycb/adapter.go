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
	"github.com/blevesearch/bleve/analysis"
	"github.com/tiglabs/caprice/index"
)

func AdaptAnalyzer(analyzer *analysis.Analyzer) index.Analyzer {
	return index.FunctionAnalyzer(func(field string, input []byte) index.Tokens {
		tokenStream := analyzer.Analyze(input)
		tokens := index.NewTokens(len(tokenStream))
		for _, t := range tokenStream {
			tokens.Add(&index.Token{
				Term:     t.Term,
				Start:    t.Start,
				End:      t.End,
				Position: t.Position,
			})
		}
		return tokens
	})
}

func AdaptFieldsAnalyzer(fieldsAnalyzer map[string]*analysis.Analyzer) map[string]index.Analyzer {
	var newFieldsAnalyzer = make(map[string]index.Analyzer, len(fieldsAnalyzer))
	for field, analyzer := range fieldsAnalyzer {
		newFieldsAnalyzer[field] = AdaptAnalyzer(analyzer)
	}
	return newFieldsAnalyzer
}
