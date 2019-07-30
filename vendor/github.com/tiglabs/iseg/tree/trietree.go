// 实现了Tire数，做了一些优化
package tree

import (
	"bufio"
	"github.com/tiglabs/iseg/utils"
	. "strings"
)

type TrieTree struct {
	branches []*TrieTree
	param    interface{}
	c        rune
	status   int8
}

func (t *TrieTree) add(branch *TrieTree) *TrieTree {
	index := t.getIndex(branch.c)
	if index >= 0 {
		bs := t.branches
		if bs == nil {
			bs[index] = branch
		}

		tempBranch := bs[index]

		switch branch.status {
		case -1:
			tempBranch.status = 1
		case 1:
			if tempBranch.status == 3 {
				tempBranch.status = 2
			}
		case 3:
			if tempBranch.status != 3 {
				tempBranch.status = 2
			}
			tempBranch.param = branch.param
		}

		return tempBranch

	} else {

		insert := -(index + 1)

		leng := 0

		if t.branches != nil {
			leng = len(t.branches)
		}

		if t.branches == nil {
			t.branches = []*TrieTree{branch}
		} else {
			bs := t.branches

			arr := make([]*TrieTree, 0, leng+1)

			if insert == 0 {
				arr = []*TrieTree{branch}
			} else {
				for i := 0; i < insert; i++ {
					arr = append(arr, bs[i])
				}

				arr = append(arr, branch)
			}

			for i := insert; i < leng; i++ {
				arr = append(arr, bs[i])
			}

			t.branches = arr

		}

		return branch
	}

}

func (t *TrieTree) getIndex(r rune) int {
	return binarySearch(t.branches, r)
}

func (t *TrieTree) GetSubTree(r rune) *TrieTree {
	search := binarySearch(t.branches, r)
	if search < 0 {
		return nil
	}
	return t.branches[search]
}

func (t *TrieTree) GetStatus() int8 {
	return t.status
}

func (t *TrieTree) GetParam() interface{} {
	return t.param
}

//二分查找是否存在
func (t TrieTree) Contains(c rune) bool {
	return t.getIndex(c) > -1
}

//增加一个新词
func (t *TrieTree) Add(keyWord string, param interface{}) {
	temp := t

	rs := []rune(keyWord)

	for i, c := range rs {
		b := TrieTree{nil, param, c, 3}
		if i == len(rs)-1 {
			temp = temp.add(&b)
		} else {
			b.status = 1
			b.param = nil
			temp = temp.add(&b)
		}
	}

}

//获取字符串的跟节点
func (t *TrieTree) getBranch(keyWord string) *TrieTree {

	temp := t

	rs := []rune(keyWord)

	for _, c := range rs {

		if temp == nil {
			return nil
		}

		index := temp.getIndex(c)

		if index < 0 {
			return nil
		}

		bs := temp.branches
		temp = bs[index]
	}

	return temp
}

func (t *TrieTree) getBranchByRune(c rune) *TrieTree {

	index := t.getIndex(c)

	if index < 0 {
		return nil
	}
	return t.branches[index]
}

func binarySearch(arr []*TrieTree, val rune) int {

	if arr == nil {
		return -1
	}

	low, high, mid := 0, len(arr)-1, 0

	for low <= high {
		mid = (low + high) >> 1

		midVal := arr[mid].c

		if midVal > val {
			high = mid - 1
		} else if midVal < val {
			low = mid + 1
		} else {
			return mid
		}
	}

	return -(low + 1)

}

//加载一个文件到词典树
// path: 文件路径
func (t *TrieTree) Load(path string) (*TrieTree, error) {
	file, e := utils.NewMyFile(path)
	if e != nil { //如果打开文件出错，那么我们可以给用户一些提示，然后在推出函数。
		return nil, e // exit the function on error
	}
	defer file.Close()

	scanner := file.Scanner(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		split := Split(line, "\t")
		if len(split) > 2 {
			t.Add(split[0], split[1:])
		} else {
			t.Add(split[0], nil)
		}
	}

	return t, nil

}

//构造一个空 的树

func MakeTree() *TrieTree {
	return &TrieTree{nil, nil, 0, 1}
}
