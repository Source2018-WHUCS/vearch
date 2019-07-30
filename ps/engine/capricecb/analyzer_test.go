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

package capricecb_test

import (
	"fmt"
	"github.com/blevesearch/bleve/analysis/analyzer/standard"
	"github.com/blevesearch/bleve/registry"
	"github.com/tiglabs/baudengine/ps/engine/mapping"
	"github.com/tiglabs/baudengine/util/assert"
	"github.com/tiglabs/caprice/index"
	"testing"
)


func TestStandard(t *testing.T) {

	analyzer, _ := mapping.BuildAnalyzer(mapping.DefaultAnalyzer, nil)

	input := "who am I my name is Ansj"

	t.Logf("input: %s", input)

	tokens := analyzer.Analyze("", []byte(input))

	_ = tokens.VisitAll(func(token *index.Token) error {
		t.Logf("token: %s", token.Term)
		return nil
	})
}

func TestStandard2(t *testing.T) {
	cache := registry.NewCache()

	analyzer, _ := cache.AnalyzerNamed(mapping.DefaultAnalyzer)

	tokens := analyzer.Analyze([]byte("alysonirwin@quizka.com"))

	for _, t := range tokens {
		fmt.Println(t)
	}

}

func TestStandard3(t *testing.T) {
	cache := registry.NewCache()

	analyzer, _ := cache.AnalyzerNamed(mapping.DefaultAnalyzer)

	tokens := analyzer.Analyze([]byte("Hello@"))

	assert.True(t, len(tokens)==2)

	assert.True(t, string(tokens[0].Term)=="hello")

	assert.True(t, string(tokens[1].Term)=="@")

	for _, t := range tokens {
		fmt.Println(t)
	}

	analyzer, _ = cache.AnalyzerNamed(standard.Name)

	tokens = analyzer.Analyze([]byte("fffffff"))

	for _, t := range tokens {
		fmt.Println(t)

	}
}


func TestStandard4(t *testing.T) {
	cache := registry.NewCache()

	analyzer, _ := cache.AnalyzerNamed(mapping.DefaultAnalyzer)

	tokens := analyzer.Analyze([]byte("promise-yunfeixian"))

	fmt.Println(tokens)

	assert.True(t, len(tokens)==3)

	assert.True(t, string(tokens[0].Term)=="promise")
	assert.True(t, string(tokens[1].Term)=="-")
	assert.True(t, string(tokens[2].Term)=="yunfeixian")

	for _, t := range tokens {
		fmt.Println(t)
	}

	analyzer, _ = cache.AnalyzerNamed(standard.Name)

	tokens = analyzer.Analyze([]byte("fffffff"))

	for _, t := range tokens {
		fmt.Println(t)

	}
}

func TestStandard5(t *testing.T) {
	cache := registry.NewCache()

	analyzer, _ := cache.AnalyzerNamed(mapping.DefaultAnalyzer)

	tokens := analyzer.Analyze([]byte("2019-03-25 06:02:47.016[INFO ]inWhichArea:lat[34.70202672914147],lng[113.72345161871907],gridKey[LAT18LNG17][com.jd.gaia.service.impl.MapAreaServiceImpl.inWhichArea:38]"))

	fmt.Println(tokens)

	tokens = analyzer.Analyze([]byte("inWhichArea"))

	fmt.Println(tokens)

}

func BenchmarkCBAnalyzer_Analyze(b *testing.B) {
	b.ReportAllocs()
	cache := registry.NewCache()

	analyzer, _ := cache.AnalyzerNamed(mapping.DefaultAnalyzer)
	for i := 0; i < b.N; i++ {
		analyzer.Analyze([]byte("ABCab.c我<>,.12323!@#$%^&爱  \n 12.\r3北32\t.123,234,2342京@天安门1abc"))
	}
}

func BenchmarkStandard_Analyze(b *testing.B) {
	b.ReportAllocs()
	cache := registry.NewCache()
	analyzer, _ := cache.AnalyzerNamed(standard.Name)
	for i := 0; i < b.N; i++ {
		analyzer.Analyze([]byte("ABCab.c我<>,.12323!@#$%^&爱  \n 12.\r3北32\t.123,234,2342京@天安门1abc"))
	}
}
