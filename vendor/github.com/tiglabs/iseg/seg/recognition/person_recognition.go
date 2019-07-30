package recognition

import (
	"bufio"
	"github.com/tiglabs/iseg/library"
	"github.com/tiglabs/iseg/seg/dat"
	"strconv"
	"strings"
)

type personRecognition struct {
	gp *graph
}

// 0 B 姓氏
// 1 C 双名的首字
// 2 D 双名的末字
// 3 E 单名
// 4 K 上文
// 5 L 下文
// 6 M 下文
// 7 Y 姓与单名成词
// 8 X 姓与双名的首字成词
// 9 Z 双名本身成词
// 10 A 其他情况
//  U 人名的上文和姓成词
//  V 名的末字和下文成词

const (
	B = iota
	C
	D
	E
	K
	L
	M
	X
	Y
	Z
	A
)

//转移概率
var transition = make([]float32, 1011)

var personMap map[string]*[]float32

var personSplitMap map[string]rune

var defaultStatus = make([]float32, 11)

func InitPerson() {
	transition[X*100+D] = 0.35999
	transition[Y*100+B] = -3.73687
	transition[Y*100+M] = -0.43878
	transition[Y*100+L] = 0.28621
	transition[Z*100+X] = -2.52373
	transition[Z*100+Y] = -3.11504
	transition[Z*100+B] = -1.83448
	transition[Z*100+L] = 0.26402
	transition[Z*100+M] = -0.06501
	transition[Y*100+Y] = -1.88320
	transition[Y*100+X] = -4.32692
	transition[K*100+Y] = 0.28621
	transition[K*100+X] = -0.49013
	transition[E*100+M] = -0.49013
	transition[K*100+B] = 0.42897
	transition[D*100+L] = 0.48905
	transition[D*100+M] = -0.10071
	transition[L*100+A] = -0.02154
	transition[C*100+D] = 0.76553
	transition[E*100+Y] = -3.73687
	transition[E*100+X] = -4.02912
	transition[M*100+B] = -0.30355
	transition[D*100+B] = -2.05756
	transition[L*100+K] = -0.49884
	transition[M*100+Y] = -0.43878
	transition[M*100+X] = -0.78341
	transition[E*100+B] = -2.80074
	transition[D*100+X] = -2.94393
	transition[D*100+Y] = -3.83297
	transition[A*100+K] = -0.15840
	transition[A*100+A] = 0.89299
	transition[E*100+L] = 0.46686
	transition[B*100+E] = 0.79864
	transition[B*100+C] = 0.76553
	transition[B*100+Z] = 0.26402

	personMap = make(map[string]*[]float32)

	convert := make([]int, 91)

	convert['B'] = B
	convert['C'] = C
	convert['D'] = D
	convert['E'] = E
	convert['K'] = K
	convert['L'] = L
	convert['M'] = M
	convert['X'] = X
	convert['Y'] = Y
	convert['Z'] = Z
	convert['A'] = A

	if file, e := library.GetPerson(); e != nil {
		panic(e)
	} else {
		defer file.Close()
		scanner := file.Scanner(bufio.ScanLines)
		for scanner.Scan() {
			split := strings.Split(scanner.Text(), "\t")
			tag := convert[rune(split[0][0])]
			name := split[1]
			s, _ := strconv.ParseFloat(split[2], 32)
			score := float32(s)

			if status, ok := personMap[name]; ok {
				(*status)[tag] = score
			} else {
				temp := make([]float32, 11)
				status = &temp
				(*status)[tag] = score
				personMap[name] = status
			}
		}

	}

	personSplitMap = make(map[string]rune)
	if file, e := library.GetPersonSplit(); e != nil {
		panic(e)
	} else {
		defer file.Close()
		scanner := file.Scanner(bufio.ScanLines)
		for scanner.Scan() {
			split := strings.Split(scanner.Text(), "\t")
			name := split[1]
			personSplitMap[name] = rune(split[0][0])
		}

	}

}

type personNode struct {
	tag              int
	fromI, fromJ     int
	name             string
	selfScore, score float32
}

func MakePersonRecognition(gp *graph) *personRecognition {
	return &personRecognition{
		gp: gp,
	}
}

//填充图
func (r *personRecognition) Full() *personRecognition {
	splitTerm(r.gp)
	return r
}

//find the best path
func (r *personRecognition) Walk() {

	gp := r.gp

	nodes := getPersonNodeViterbi(gp)

	nextLen := 1

	for i := 0; i < gp.len+1; i++ {

		if i > 0 {
			if len(gp.terms[i-1]) == 0 {
				continue
			}
			nextLen = len(gp.terms[i-1][0].name)
		}

		toLen := len((*nodes)[i+nextLen])

		for j := 0; j < len((*nodes)[i]); j++ {
			for m := 0; m < toLen; m++ {
				if personComputer(nodes, i, j, i+nextLen, m) {
					break
				}
			}
		}
	}

	//得出整个路径,进行合并,手工 ,结尾只能是A ，L

	firstIndex := 0

	if (*nodes)[gp.len+1][0].score == 0 || (*nodes)[gp.len+1][0].score < (*nodes)[gp.len+1][1].score {
		firstIndex = 1
	}

	temp := (*nodes)[gp.len+1][firstIndex]

	for {
		(*nodes)[temp.fromI] = (*nodes)[temp.fromI][temp.fromJ : temp.fromJ+1]

		temp = (*nodes)[temp.fromI][0]

		if temp == nil || temp.fromI < 0 {
			break
		}

	}

	//BE BCD  XD BZ
	//int B = 0, C = 1, D = 2, E = 3, K = 4, L = 5, M = 6, X = 7, Y = 8, Z = 9, A = 10;

	var begin int

	for i := 1; i < gp.len+1; i++ {
		row := (*nodes)[i]
		if row != nil && len(row) > 0 {

			tag := row[0].tag

			if tag == B || tag == X {
				begin = i - 1
			}

			if tag == E || tag == D || tag == Z { //发现end进行合并
				gp.terms[begin][0] = &term{
					item:  dat.NewItem(dat.PersonNatures),
					name:  gp.rs[begin:gp.terms[i-1][0].getToIndex()],
					index: begin,
				}
				gp.terms[begin] = gp.terms[begin][0:1]

				for j := begin + 1; j < i; j++ {
					gp.terms[j] = gp.terms[j][0:0]
				}
			}

		}
	}

	gp.rmLittlePath() //删除之前拆分你的路径

}

//两个点之间的分值计算
func personComputer(nodes *[][]*personNode, fromI, fromJ, toI, toJ int) bool {

	from, to := (*nodes)[fromI][fromJ], (*nodes)[toI][toJ]

	edgeScore := transition[from.tag*100+to.tag]

	if edgeScore == 0 {
		return false
	}

	if fromI > 1 && from.fromJ == 0 && from.score == 0 {
		(*nodes)[fromI][fromJ] = nil
		return true
	}

	if to.score == 0 || to.score < from.score+to.selfScore+edgeScore {
		to.score = from.score + to.selfScore + edgeScore
		if to.score == 0 {
			to.score = 0.00001 //经历过计算的节点永不为0
		}
		to.fromI, to.fromJ = fromI, fromJ
	}

	return false

}

//构建viterbi路径
func getPersonNodeViterbi(gp *graph) *[][]*personNode {

	nodes := make([][]*personNode, gp.len+2)
	for i := 1; i < gp.len+1; i++ {
		if len(gp.terms[i-1]) > 0 {
			nodes[i] = make([]*personNode, 0)
		}
	}

	terms := gp.terms
	var from, first, second, third *term

	preIndex := -1

	var firstStatus *[]float32

	for i := 0; i < gp.len; i++ {
		if len(gp.terms[i]) == 0 {
			continue
		}

		if preIndex != -1 {
			from = terms[preIndex][0]
		}
		preIndex = i

		first = terms[i][0]

		firstStatus = first.personStatus()

		makePersonNode(&nodes, first, A)
		if (*firstStatus)[Y] > 0 {
			makePersonNode(&nodes, first, Y)
		}

		if (*firstStatus)[B] <= 0 && (*firstStatus)[X] <= 0 {
			continue
		}

		if first.getToIndex() >= gp.len {
			continue
		}

		second = terms[first.getToIndex()][0]

		if len(second.name) > 2 {
			continue
		}

		if second.getToIndex() >= gp.len {
			continue

		}
		third = terms[second.getToIndex()][0]

		//XD
		if len(first.name) == 2 {
			makePersonNode(&nodes, from, K)
			makePersonNode(&nodes, from, M)
			makePersonNode(&nodes, first, X)
			makePersonNode(&nodes, second, D)
			makePersonNode(&nodes, third, M)
			makePersonNode(&nodes, third, L)
			continue
		}

		makePersonNode(&nodes, from, K)
		makePersonNode(&nodes, from, M)
		makePersonNode(&nodes, first, B)
		makePersonNode(&nodes, third, M)
		makePersonNode(&nodes, third, L)

		//BZ
		if len(second.name) == 2 {
			makePersonNode(&nodes, second, Z)
			continue
		} else { //BE
			makePersonNode(&nodes, second, E)
		}

		//BCD
		makePersonNode(&nodes, first, B)
		makePersonNode(&nodes, second, C)
		makePersonNode(&nodes, third, D)

		if third.getToIndex() >= gp.len {
			continue
		}

		fourth := terms[third.getToIndex()][0]

		makePersonNode(&nodes, fourth, M)
		makePersonNode(&nodes, fourth, L)
	}

	//设置起始 结束点

	nodes[0] = []*personNode{
		&personNode{
			tag:       K,
			selfScore: 0.74876,
			fromI:     -1,
		},
		&personNode{
			tag:       A,
			selfScore: 1.32058,
			fromI:     -1,
		},
	}

	nodes[gp.len+1] = []*personNode{
		&personNode{
			tag:       L,
			selfScore: 1.28417,
		},
		&personNode{
			tag:       A,
			selfScore: 1.54991,
		},
	}

	return &nodes
}

func makePersonNode(nodes *[][]*personNode, term *term, tag int) {
	if term == nil {
		return
	}

	(*nodes)[term.index+1] = append((*nodes)[term.index+1], &personNode{
		tag:       tag,
		selfScore: (*term.personStatus())[tag],
	})

}

func (term *term) personStatus() *[]float32 {
	name := term.item.Name
	if name == "" {
		term.item.Name = string(term.name)
		name = term.item.Name
	}
	if pa, ok := personMap[name]; ok {
		return pa
	}
	if pa, ok := personMap[":"+term.item.Natures.GetName()]; ok {
		return pa
	}
	return &defaultStatus
}

func (term *term) getUV() (int, int) {
	name := term.item.Name

	if name == "" {
		name = string(term.name)
	}

	r := personSplitMap[name]

	if r == 0 {
		return 0, 0
	} else if r == 'U' {
		return 1, 0
	} else {
		return 0, 1
	}

}

//将可能错误合并的term拆分开
func splitTerm(gp *graph) {
	for i := 0; i < gp.len; i++ {
		if len(gp.terms[i]) == 0 {
			continue
		}
		t := gp.terms[i][0]
		nameLen := len(t.name)
		if nameLen == 1 || nameLen > 3 {
			continue
		}

		status := t.personStatus()

		if status == nil {
			continue
		}

		U, V := t.getUV()

		if U <= 0 && V <= 0 {
			continue
		}

		if nameLen == 2 {
			gp.terms[i] = append(gp.terms[i], t)
			s1 := t.name[0:1]
			s2 := t.name[1:2]

			item1 := dat.GetItem(s1)
			if item1 == nil {
				item1 = dat.NewItem(dat.DefaultNatures)
			}
			gp.terms[i][0] = &term{item1, s1, i}

			item2 := dat.GetItem(s2)
			if item2 == nil {
				item2 = dat.NewItem(dat.DefaultNatures)
			}
			gp.add(i+1, item2, s2)
		} else {
			if U > 0 {
				gp.terms[i] = append(gp.terms[i], t)
				s1 := t.name[0:2]
				s2 := t.name[2:3]

				item1 := dat.GetItem(s1)
				if item1 == nil {
					item1 = dat.NewItem(dat.DefaultNatures)
				}
				gp.terms[i][0] = &term{item1, s1, i}

				item2 := dat.GetItem(s2)
				if item2 == nil {
					item2 = dat.NewItem(dat.DefaultNatures)
				}
				gp.add(i+2, item2, s2)
			} else {
				gp.terms[i] = append(gp.terms[i], t)
				s1 := t.name[0:1]
				s2 := t.name[1:3]

				item1 := dat.GetItem(s1)
				if item1 == nil {
					item1 = dat.NewItem(dat.DefaultNatures)
				}
				gp.terms[i][0] = &term{item1, s1, i}

				item2 := dat.GetItem(s2)
				if item2 == nil {
					item2 = dat.NewItem(dat.DefaultNatures)
				}
				gp.add(i+1, item2, s2)
			}
		}

	}
}
