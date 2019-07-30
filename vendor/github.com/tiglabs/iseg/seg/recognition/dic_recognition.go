package recognition

import (
	"github.com/tiglabs/iseg/tree"
)

type dicRecognition struct {
	gp    *graph
	trees []*tree.TrieTree
	nodes *[][]node
	flag  bool
}

func MakeDicRecognition(gp *graph, trees []*tree.TrieTree) *dicRecognition {
	return &dicRecognition{
		gp:    gp,
		trees: trees,
		flag:  false,
	}
}

//填充图
func (r *dicRecognition) Full() *dicRecognition {
	gp := r.gp

	for _, forest := range r.trees {

		branch := forest
		if forest == nil {
			continue
		}

		offset := 0

		for i := 0; i < gp.len-1; i++ {
			if len(gp.terms[i]) == 0 {
				continue
			}

			if branch == forest {
				offset = i
			}

			branch = termStatus(branch, gp.terms[i][0])

			if branch == nil {
				i = offset
				branch = forest
			} else if branch.GetStatus() == 3 {
				len := len((gp.terms[i][0]).name)
				if len < i-offset+len {
					r.flag = true
					gp.addNewTerm(offset, i-offset+len, branch.GetParam())
				}
				i = offset
				branch = forest
			} else if branch.GetStatus() == 2 {
				len := len((gp.terms[i][0]).name)
				if len < i-offset+len {
					r.flag = true
					gp.addNewTerm(offset, i-offset+len, branch.GetParam())
				}
			}
		}
	}
	return r
}
func termStatus(branch *tree.TrieTree, term *term) *tree.TrieTree {
	for _, r := range term.name {
		branch = branch.GetSubTree(rune(r))
		if branch == nil {
			return nil
		}
	}
	return branch
}

func (r *dicRecognition) Walk() {

	if !r.flag {
		return
	}

	r.gp.walk(dicCompute)
}

func dicCompute(from, to *node, fromT, toT *term, x, y int) {
	fromFreq := float64(fromT.item.Natures.GetAllFreq())
	toFreq := float64(toT.item.Natures.GetAllFreq())

	if fromFreq <= 0 {
		fromFreq = min
	}

	if toFreq <= 0 {
		toFreq = min
	}

	if to.score == 0 || to.score < from.score+toFreq {
		to.score = from.score + toFreq
		if to.score == 0 {
			to.score = 0.00001
		}
		to.fromX = x
		to.fromY = y
	}

}
