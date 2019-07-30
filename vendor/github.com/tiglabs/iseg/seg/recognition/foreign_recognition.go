package recognition

import (
	"bufio"
	"github.com/tiglabs/iseg/library"
	"github.com/tiglabs/iseg/seg/dat"
)

//外国人名识别
type foreignRecognition struct {
	gp *graph
}

var foreignMap = make(map[string]string)

func InitForeign() {
	if file, e := library.GetForeign(); e != nil {
		panic(e)
	} else {
		defer file.Close()
		scanner := file.Scanner(bufio.ScanLines)
		for scanner.Scan() {
			name := scanner.Text()
			foreignMap[name] = name
		}

	}
}

func MakeForeignRecognition(gp *graph) *foreignRecognition {
	return &foreignRecognition{
		gp: gp,
	}
}

//填充图
func (r *foreignRecognition) Full() *foreignRecognition {
	return r
}

func (r *foreignRecognition) Walk() {
	gp := r.gp

	begin, end := -1, -2
	for i := 0; i < gp.len; i++ {

		if len(gp.terms[i]) == 0 {
			continue
		}

		t := gp.terms[i][0]

		flag := t.foreign()

		if flag {
			if begin == -1 {
				begin = i
			}

			if t.getName() != "·" {
				end = i
			}

		} else {

			if begin >= 0 && end - begin >3 { //可以合并了,国外人名识别，只识别大于三字的
				gp.terms[begin][0] = &term{
					item:  dat.NewItem(dat.ForeignNatures),
					name:  gp.rs[begin:gp.terms[end][0].getToIndex()],
					index: begin,
				}
				gp.terms[begin] = gp.terms[begin][0:1]

				for j := begin + 1; j < end+1; j++ {
					gp.terms[j] = gp.terms[j][0:0]
				}
			}

			begin, end = -1, -2

		}

	}
}

//取得人名补充
func (t *term) foreign() bool {

	name := t.getName()

	if _, ok := foreignMap[name]; ok {
		return true
	}

	if !(t.item.Natures.GetName() == "nr" && t.item.Natures.GetAllFreq() == -1) {
		return false
	}

	for _, r := range t.name {
		if _, ok := foreignMap[string(r)]; !ok {
			return false
		}
	}

	return true

}
