package recognition

import (
	"github.com/tiglabs/iseg/seg/domain"
	"github.com/tiglabs/iseg/tree"
	"strconv"
)

type indexRecognition struct {
	gp    *graph
	trees []*tree.TrieTree
}

func MakeIndexRecognition(gp *graph, trees []*tree.TrieTree) *indexRecognition {
	return &indexRecognition{
		gp:    gp,
		trees: trees,
	}
}

//填充图
func (r *indexRecognition) Others() []domain.Term {

	gp := r.gp

	othersMap := make(map[string]int)
	others := make([]domain.Term, 0, gp.len)

	for _, forest := range r.trees {

		if forest == nil {
			continue
		}

		wordRune := tree.MakeGetWordRune(forest, gp.rs)

		for wordRune.AllWords() {

			if gp.terms[wordRune.Off] != nil && len(gp.terms[wordRune.Off]) > 0 && len(gp.terms[wordRune.Off][0].name) == len([]rune(wordRune.Word)) {
				continue
			}

			if othersMap[wordRune.Word] == 0 {
				othersMap[wordRune.Word] = 1
				newTerm := makeNewTerm(wordRune.Word, wordRune.Off, wordRune.Param.([]string))
				others = append(others, newTerm)
			}
		}

	}

	return others
}

func makeNewTerm(name string, offe int, params []string) domain.Term {
	freq, _ := strconv.Atoi(params[1])
	return domain.Term{
		Name:   name,
		Offset: offe,
		Nature: params[0],
		Freq:   freq,
	}

}
