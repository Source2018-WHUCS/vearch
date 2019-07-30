package bleve

import (
	"github.com/tiglabs/baudengine/third_party/bleve/analysis"
	"github.com/tiglabs/baudengine/third_party/bleve/registry"
)

const NAME = "iseg"
const NAME_INDEX = "iseg_index"
const NAME_SMART = "iseg_smart"
const NAME_BASE = "iseg_base"

func init() {
	registry.RegisterAnalyzer(NAME, func(config map[string]interface{}, cache *registry.Cache) (*analysis.Analyzer, error) {
		tokenizer, err := cache.TokenizerNamed(NAME)
		if err != nil {
			return nil, err
		}
		alz := &analysis.Analyzer{
			Tokenizer: tokenizer,
		}
		return alz, nil
	})

	registry.RegisterAnalyzer(NAME_INDEX, func(config map[string]interface{}, cache *registry.Cache) (*analysis.Analyzer, error) {
		tokenizer, err := cache.TokenizerNamed(NAME_INDEX)
		if err != nil {
			return nil, err
		}
		alz := &analysis.Analyzer{
			Tokenizer: tokenizer,
		}
		return alz, nil
	})

	registry.RegisterAnalyzer(NAME_SMART, func(config map[string]interface{}, cache *registry.Cache) (*analysis.Analyzer, error) {
		tokenizer, err := cache.TokenizerNamed(NAME_SMART)
		if err != nil {
			return nil, err
		}
		alz := &analysis.Analyzer{
			Tokenizer: tokenizer,
		}
		return alz, nil
	})

	registry.RegisterAnalyzer(NAME_BASE, func(config map[string]interface{}, cache *registry.Cache) (*analysis.Analyzer, error) {
		tokenizer, err := cache.TokenizerNamed(NAME_BASE)
		if err != nil {
			return nil, err
		}
		alz := &analysis.Analyzer{
			Tokenizer: tokenizer,
		}
		return alz, nil
	})

}
