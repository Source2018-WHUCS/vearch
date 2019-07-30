package bleve

import (
	"github.com/tiglabs/baudengine/third_party/bleve/analysis"
	"github.com/tiglabs/baudengine/third_party/bleve/registry"
	"github.com/tiglabs/iseg/seg"
	"github.com/tiglabs/iseg/seg/domain"
	"io/ioutil"
	"log"
)

const (
	BASE = iota
	SMART
	INDEX
	END  //不做分词用来保底
)

var logger = log.New(ioutil.Discard, "bleve_seg", log.LstdFlags)

type ISegTokenizer struct {
	segType int
}

func NewISegTokenizer(segType int) *ISegTokenizer {

	tokenizer := ISegTokenizer{}

	if segType < END && segType > 0 {
		tokenizer.segType = segType
	} else {
		logger.Printf("segType set err Too big or too small value %d out range [0,%d)", segType, END)
	}

	return &tokenizer

}

func (x *ISegTokenizer) Tokenize(sentence []byte) analysis.TokenStream {
	result := make(analysis.TokenStream, 0)
	pos := 1

	var terms *domain.Result

	switch x.segType {
	case BASE:
		terms = seg.Base(string(sentence))
	case SMART:
		terms = seg.Smart(string(sentence))
	case INDEX:
		terms = seg.Index(string(sentence))
	}

	for _, word := range terms.Terms {
		token := analysis.Token{
			Term:     []byte(word.Name),
			Start:    word.Offset,
			End:      len(word.Name),
			Position: pos,
			Type:     analysis.Ideographic,
		}
		result = append(result, &token)
		pos++
	}
	return result
}

func init() {
	registry.RegisterTokenizer(NAME, func(config map[string]interface{}, cache *registry.Cache) (analysis.Tokenizer, error) {
		return NewISegTokenizer(SMART), nil
	})
	registry.RegisterTokenizer(NAME_BASE, func(config map[string]interface{}, cache *registry.Cache) (analysis.Tokenizer, error) {
		return NewISegTokenizer(BASE), nil
	})
	registry.RegisterTokenizer(NAME_INDEX, func(config map[string]interface{}, cache *registry.Cache) (analysis.Tokenizer, error) {
		return NewISegTokenizer(INDEX), nil
	})
	registry.RegisterTokenizer(NAME_SMART, func(config map[string]interface{}, cache *registry.Cache) (analysis.Tokenizer, error) {
		return NewISegTokenizer(SMART), nil
	})
}
