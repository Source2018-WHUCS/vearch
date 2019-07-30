package recognition

import (
	"bufio"
	"github.com/tiglabs/iseg/library"
	"github.com/tiglabs/iseg/tree"
	"math"
	"strconv"
	"strings"
)

var ngramTree tree.TrieTree

const maxFrequence float64 = 2079997

const min float64 = 0.001

const dSmoothingPara float64 = 0.1

const dTemp = 1 / maxFrequence

const split = '@'

func InitNgram() {
	if file, e := library.GetGramDic(); e != nil {
		panic(e)
	} else {
		defer file.Close()
		scanner := file.Scanner(bufio.ScanLines)
		for scanner.Scan() {
			split := strings.Split(scanner.Text(), "\t")
			score, _ := strconv.ParseFloat(split[1], 64)
			ngramTree.Add(split[0], score)
		}

	}

}

//计算两个节点的分数
func compute(from, to *node, fromT, toT *term, x, y int) {

	var frequency float64 = float64(fromT.item.Natures.GetAllFreq()) + 1

	if frequency < 0 {
		from.score = from.score + maxFrequence

		if to.score == 0 || to.score > from.score {
			to.score = from.score
			if to.score = from.score; to.score == 0 {
				to.score = min //不让to等于0
			}
			to.fromX = x
			to.fromY = y
		}

		return
	}

	tempTree := &ngramTree

	for _, r := range fromT.name {
		if tempTree = tempTree.GetSubTree(r); tempTree == nil {
			break
		}
	}

	if tempTree != nil {
		tempTree = tempTree.GetSubTree(split)
	}

	if tempTree != nil {
		for _, r := range toT.name {
			if tempTree = tempTree.GetSubTree(r); tempTree == nil {
				break
			}
		}
	}

	var twoWordsFreq float64 = 0

	if tempTree != nil && tempTree.GetStatus() > 1 {
		twoWordsFreq = tempTree.GetParam().(float64)
	}

	if frequency < twoWordsFreq {
		frequency = twoWordsFreq
	}

	value := -math.Log(dSmoothingPara*frequency/(maxFrequence+80000) + (1-dSmoothingPara)*((1-dTemp)*twoWordsFreq/frequency+dTemp))

	if value < 0 {
		value += frequency
	}

	if to.score == 0 || from.score+value < to.score {
		if to.score = from.score + value; to.score == 0 {
			to.score = min //不让to等于0
		}
		to.fromX = x
		to.fromY = y
	}

}
