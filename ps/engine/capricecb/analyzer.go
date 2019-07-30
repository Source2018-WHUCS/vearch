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

package capricecb

import (
	"unicode"
	"unicode/utf8"

	"github.com/blevesearch/bleve/analysis"
	"github.com/blevesearch/bleve/analysis/lang/en"
	"github.com/blevesearch/bleve/analysis/token/lowercase"
	"github.com/blevesearch/bleve/registry"
	"github.com/tiglabs/baudengine/ps/engine/mapping"
	"github.com/tiglabs/caprice/index"
)

func init() {
	// registry cb_standard / baud_standard analyzer
	registry.RegisterAnalyzer(mapping.DefaultAnalyzer, baudStandardAnalyzerConstructor)
	// registry cb_standard / baud_standard tokenizer
	registry.RegisterTokenizer(mapping.DefaultTokenizer, baudStandardTokenizerConstructor)
}

func NewCapriceAnalyzer(mapping *mapping.IndexMapping) index.Analyzer {
	return &CapriceAnalyzer{
		mapping:         mapping,
		tokenizer:       NewCBTokenizer(),
		defaultAnalyzer: index.DefaultTextAnalyzer,
	}
}

type CapriceAnalyzer struct {
	mapping         *mapping.IndexMapping
	tokenizer       *baudStandardTokenizer
	defaultAnalyzer index.Analyzer
}

func (ca *CapriceAnalyzer) Analyze(field string, input []byte) index.Tokens {
	if ca.mapping != nil {
		analyzer, err := ca.mapping.GetAnalyzer(field)
		if err != nil {
			return ca.defaultAnalyzer.Analyze(field, input)
		}
		return TokenStreamEx(analyzer.Analyze(input)).ToCapriceTokens()
	}
	return ca.defaultAnalyzer.Analyze(field, input)
}

type TokenStreamEx analysis.TokenStream

func (tse TokenStreamEx) ToCapriceTokens() index.Tokens {
	tokens := index.NewTokens(len(tse))
	for _, t := range tse {
		tokens.Add(&index.Token{
			Term:     t.Term,
			Start:    t.Start,
			End:      t.End,
			Position: t.Position,
		})
	}
	return tokens
}

var baudStandardAnalyzerConstructor registry.AnalyzerConstructor = func(config map[string]interface{}, cache *registry.Cache) (analyzer *analysis.Analyzer, e error) {

	tokenizer, err := cache.TokenizerNamed(mapping.DefaultTokenizer)
	if err != nil {
		return nil, err
	}
	toLowerFilter, err := cache.TokenFilterNamed(lowercase.Name)
	if err != nil {
		return nil, err
	}
	stopEnFilter, err := cache.TokenFilterNamed(en.StopName)
	if err != nil {
		return nil, err
	}
	rv := analysis.Analyzer{
		Tokenizer: tokenizer,
		TokenFilters: []analysis.TokenFilter{
			toLowerFilter,
			stopEnFilter,
		},
	}
	return &rv, nil
}

//begin tokenizer

type baudStandardTokenizer struct {
}

func NewCBTokenizer() *baudStandardTokenizer {
	return &baudStandardTokenizer{}
}

func (rt *baudStandardTokenizer) Tokenize(input []byte) analysis.TokenStream {

	result := make([]*analysis.Token, 0, len(input)/3+3)

	position := 0
	var token *analysis.Token
	for current := 0; current < len(input); current++ {
		r, size := utf8.DecodeRune(input[current:])
		if size == 1 {
			if unicode.IsLetter(r) {
				if token != nil && token.Type != analysis.AlphaNumeric {
					token.End = current
					token.Term = input[token.Start:current]
					result = append(result, token)
					token = nil
				}

				if token == nil {
					token = &analysis.Token{
						Start:    current,
						Position: position,
						Type:     analysis.AlphaNumeric,
					}
					position++
				}
			} else if unicode.IsNumber(r) {

				if token != nil && token.Type != analysis.Numeric {
					token.End = current
					token.Term = input[token.Start:current]
					result = append(result, token)
					token = nil
				}

				if token == nil {
					token = &analysis.Token{
						Start:    current,
						Position: position,
						Type:     analysis.Numeric,
					}
					position++
				}
			} else {
				if token != nil {
					token.End = current
					token.Term = input[token.Start:current]
					result = append(result, token)
					token = nil
				}
				if !unicode.IsSpace(r) {
					result = append(result, &analysis.Token{
						Start:    current,
						End:      current + size,
						Term:     input[current : current+size],
						Position: position,
						Type:     analysis.Ideographic,
					})
					position++
				}
			}
		} else {
			if token != nil {
				token.End = current
				token.Term = input[token.Start:current]
				result = append(result, token)
				token = nil
			}
			result = append(result, &analysis.Token{
				Start:    current,
				End:      current + size,
				Term:     input[current : current+size],
				Position: position,
				Type:     analysis.Ideographic,
			})
			position++
		}

		current += size - 1
	}

	if token != nil {
		token.End = len(input)
		token.Term = input[token.Start:]
		result = append(result, token)
		token = nil
	}

	return result
}

var baudStandardTokenizerConstructor registry.TokenizerConstructor = func(config map[string]interface{}, cache *registry.Cache) (tokenizer analysis.Tokenizer, e error) {
	return &baudStandardTokenizer{}, nil
}
