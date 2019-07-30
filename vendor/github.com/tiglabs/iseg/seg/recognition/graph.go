package recognition

import (
	"fmt"
	"github.com/tiglabs/iseg/seg/dat"
	"github.com/tiglabs/iseg/seg/domain"
	"github.com/tiglabs/iseg/utils"
	"strconv"
)

type term struct {
	item  *dat.Item
	name  []rune
	index int
}

const UserDefine = "userDefine"

//获得这个term的下一个term
func (t *term) getToIndex() int {
	return t.index + len(t.name)
}
func (term *term) getName() string {
	if term.item.Name == "" {
		term.item.Name = string(term.name)
	}
	return term.item.Name
}

//构建图对象
type graph struct {
	rs    []rune
	terms [][]*term
	len   int
	str   string
}

type node struct {
	fromX int
	fromY int
	score float64
}

var beginTerm = term{dat.NewItem(dat.BeginNatures), []rune{'B'}, 0}

var endTerm = term{dat.NewItem(dat.EnNatures), []rune{'E'}, -1}

//创建一个graph
func NewGraph(text string) *graph {

	rs := utils.AlertStr(text)
	len := len(rs)

	return &graph{
		len:   len,
		rs:    rs,
		terms: make([][]*term, len),
		str:   text,
	}
}

//增加一个分词结果到terms中
func (g *graph) add(index int, item *dat.Item, name []rune) {
	g.terms[index] = append(g.terms[index], &term{item, name, index})
}

//获得标准化的字符串编码
func (g *graph) GetRS() []rune {
	return g.rs
}

//增加一个词到term中如果，存在则不添加
func (g *graph) addNewTerm(index, length int, params interface{}) {

	freq := 1000
	nature := UserDefine

	if params != nil {
		strs := params.([]string)
		freq, _ = strconv.Atoi(strs[1])
		nature = strs[0]
	}

	for _, t := range g.terms[index] {
		if len(t.name) == length { //说明这个词语已经加过了
			return
		}
	}
	g.terms[index] = append(g.terms[index], &term{
		item:  dat.CreateItem(nature, freq),
		name:  g.rs[index : index+length],
		index: index,
	})
}

//优化路径,调用这个方法前保证terms,每一列是从小到大排列.
func (g *graph) rmLittlePath() {
	for i := 0; i < g.len; i++ {
		row := &g.terms[i]
		if len(*row) <= 1 {
			continue
		}

		flag := true
		maxTo := i + len((*row)[len(*row)-1].name)
		for j := i + 1; j < maxTo; j++ {
			temp := &g.terms[j]
			if len(*temp) == 0 {
				continue
			}
			to := j + len((*temp)[len(*temp)-1].name)

			if maxTo < to {
				flag = false
				i = maxTo - 1
				break
			}
		}

		if flag {
			(*row)[0] = (*row)[len(*row)-1]
			*row = (*row)[0:1]
			for m := i + 1; m < maxTo; m++ {
				g.terms[m] = g.terms[m][0:0]
			}
		}
	}
}

//构建最优路径
func (gp *graph) walk(compute func(from *node, to *node, fromT *term, toT *term, x int, y int)) {

	gp.rmLittlePath()

	nodes := make([][]node, gp.len+1)

	for i := 0; i < gp.len; i++ {
		len := len(gp.terms[i])
		if len == 0 {
			continue
		}
		nodes[i] = make([]node, len)
	}
	nodes[gp.len] = make([]node, 1)

	beginNode := node{}

	for i, toT := range gp.terms[0] {
		compute(&beginNode, &nodes[0][i], &beginTerm, toT, -1, 0)
	}

	for i := 0; i < gp.len; i++ {
		terms := gp.terms[i]
		length := len(terms)
		if length == 0 {
			continue
		}
		for j := 0; j < length; j++ {
			fromT := gp.terms[i][j]
			from := &nodes[i][j]
			toIndex := fromT.getToIndex()
			ns := &nodes[toIndex]
			for x := 0; x < len(*ns); x++ {
				toT := &endTerm
				if toIndex < gp.len {
					toT = gp.terms[toIndex][x]
				}
				to := &(*ns)[x]

				compute(from, to, fromT, toT, i, j)
			}

		}
	}

	//进行优化
	temp := nodes[gp.len][0]

	for i := temp.fromX + 1; i < gp.len; i++ { //末尾词需要单独清空
		gp.terms[i] = gp.terms[i][0:0]
	}

	for {

		if len(gp.terms[temp.fromX]) > 1 {
			gp.terms[temp.fromX][0] = gp.terms[temp.fromX][temp.fromY]
			gp.terms[temp.fromX] = gp.terms[temp.fromX][0:1]
		}

		fx := temp.fromX

		temp = nodes[temp.fromX][temp.fromY]

		if temp.fromX+1 < fx {
			for i := temp.fromX + 1; i < fx; i++ {
				gp.terms[i] = gp.terms[i][0:0]
			}
		}

		if temp.fromX < 0 {
			break
		}
	}

}
func PrintNodes(nodes [][]node) {
	for i, m := range nodes {
		fmt.Print(i, " ")
		for _, t := range m {
			fmt.Print(t.score, " ")
		}
		fmt.Println()
	}
}

//特	6.05609361139672 ,特别/d	6.041031680413198 ,
//别	19.37910649670215 ,别是/d	19.37910649670215 ,
//是	6.743314319447215 ,
//应该	13.741752559865876 ,
//用来	25.147874849645557 ,
//促	39.44484905407611 ,促进/v	39.44484905407611 ,
//进	52.82610908410952 ,
//END	44.952172042887476 ,

func (gp *graph) Result() *domain.Result {

	Terms := make([]domain.Term, 0, gp.len/2+1)

	for i := 0; i < gp.len; i++ {
		temp := gp.terms[i]
		if len(temp) > 0 {
			t := temp[0]
			Terms = append(Terms, domain.Term{
				Name:   t.getName(),
				Nature: t.item.Natures.GetName(),
				Freq:   t.item.Natures.GetAllFreq(),
				Offset: i,
			})
		}
	}

	return &domain.Result{Terms: Terms}
}

func (gp *graph) Print() {
	for i, m := range gp.terms {
		fmt.Print(i, " ")
		for _, t := range m {
			fmt.Print(t.getName(), " ")
		}
		fmt.Println()
	}
}
