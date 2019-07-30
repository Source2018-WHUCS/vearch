package tree

import (
	"github.com/tiglabs/iseg/utils"
)

type getWord struct {
	i, len, tempLen, Off, WordLen int
	tree                          *TrieTree
	temp                          *TrieTree
	Word                          string
	rs                            []rune
	Param                         interface{}
}

func MakeGetWord(tree *TrieTree, content *string) getWord {
	runes := []rune(*content)
	return getWord{
		len:     len(runes),
		tree:    tree,
		temp:    tree,
		rs:      runes,
	}
}


func MakeGetWordRune(tree *TrieTree, runes []rune) getWord {
	return getWord{
		len:     len(runes),
		tree:    tree,
		temp:    tree,
		rs:      runes,
	}
}


//重制对象，一般情况下实力即可，大规模循环调用可以考虑此方式，以减少垃圾回收
func (gw *getWord) ResetGetWord(runes []rune) {
	gw.rs = runes
	gw.len = len(gw.rs)
	gw.temp = gw.tree
	gw.i, gw.tempLen, gw.Off, gw.WordLen = 0, 0, 0, 0
}

const empty = ""

//迭代遍历所有单词，全匹配
func (gw *getWord) AllWords() bool {
	for {
		gw.allWords()
		flag := gw.i < gw.len-1

		if gw.check() {
			return flag
		}

		if !flag {
			return flag
		}
	}

}

//迭代遍历所有单词，全匹配
func (gw *getWord) allWords() {

	for j := gw.i + gw.tempLen; j < gw.len; j++ {
		if branch := gw.temp.getBranchByRune(gw.rs[j]); branch == nil {
			gw.temp = gw.tree
			gw.tempLen, gw.WordLen, gw.Word = 0, 0, empty
			gw.i++
			return
		} else {
			if branch.status == 2 {
				gw.Off = gw.i
				gw.Param = branch.param
				gw.tempLen = j - gw.i + 1
				gw.WordLen = gw.tempLen
				gw.temp = branch
				gw.Word = string(gw.rs[gw.i : j+1])
				return
			} else if branch.status == 3 {
				gw.Off = gw.i
				gw.Param = branch.param
				gw.temp = gw.tree
				gw.tempLen = 0
				gw.WordLen = j - gw.i + 1
				gw.Word = string(gw.rs[gw.i : j+1])
				gw.i++
				return
			} else {
				gw.temp = branch
			}
		}
	}

	gw.temp = gw.tree
	gw.tempLen, gw.WordLen, gw.Word = 0, 0, empty
	gw.i++
}

//正向最大匹配的一个实现
func (gw *getWord) MaxWords() bool {
	for {
		gw.maxWords()

		flag := gw.Off < gw.len

		if gw.check() {
			return flag
		}

		if !flag {
			return flag
		}
	}
}

//正向最大匹配的一个实现
func (gw *getWord) maxWords() {

	gw.Off = gw.i
	if gw.i == gw.len {
		gw.WordLen = 0
		return
	}

	for ; gw.i <= gw.len; gw.i++ {

		if gw.i == gw.len {
			gw.temp = nil
		} else {
			gw.temp = gw.temp.getBranchByRune(gw.rs[gw.i])
		}

		if gw.temp == nil {
			gw.temp = gw.tree
			if gw.tempLen > 0 {
				gw.Word = string(gw.rs[gw.Off : gw.Off+gw.tempLen])
				gw.i = gw.Off + gw.tempLen
				gw.tempLen = 0
				return
			} else {
				gw.Off = gw.i + 1
			}
		} else {
			if gw.temp.status == 2 {
				gw.WordLen = gw.i - gw.Off + 1
				gw.tempLen = gw.WordLen
				gw.Param = gw.temp.param
			} else if gw.temp.status == 3 {
				gw.Param = gw.temp.param
				gw.temp = gw.tree
				gw.WordLen, gw.tempLen = gw.i-gw.Off+1, 0
				gw.Word = string(gw.rs[gw.Off : gw.i+1])
				gw.i++
				return
			}
		}
	}
}

/**
 * 验证两个char是否都是数字或者都是英文
 *
 * @param l
 * @param c
 * @return
 */
func checkSame(l, c rune) bool {
	if utils.IsEnglish(l) && utils.IsEnglish(c) {
		return true
	}
	if utils.IsNumber(l) && utils.IsNumber(c) {
		return true
	}
	return false
}

//验证一个词语的左右边.不是英文和数字
func (gw getWord) check() bool {

	if gw.WordLen == 0 || gw.Off >= gw.len {
		return false
	}

	// 先验证最左面
	r := gw.rs[gw.Off]
	if r < 127 && gw.Off > 0 {
		if checkSame(r, gw.rs[gw.Off-1]) {
			return false
		}
	}

	if gw.Off+gw.WordLen == gw.len {
		return true
	}

	r = gw.rs[gw.Off+gw.WordLen-1]

	if r < 127 {
		if checkSame(r, gw.rs[gw.Off+gw.WordLen]) {
			return false
		}
	}

	return true
}
