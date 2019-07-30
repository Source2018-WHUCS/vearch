package recognition

import (
	"github.com/tiglabs/iseg/seg/dat"
	"github.com/tiglabs/iseg/utils"
)

var j_NUM map[rune]int
var f_NUM map[rune]int

func init() {
	j_NUM = make(map[rune]int)
	f_NUM = make(map[rune]int)

	j_NUM['零'] = 1
	j_NUM['一'] = 1
	j_NUM['两'] = 1
	j_NUM['二'] = 1
	j_NUM['三'] = 1
	j_NUM['四'] = 1
	j_NUM['五'] = 1
	j_NUM['六'] = 1
	j_NUM['七'] = 1
	j_NUM['八'] = 1
	j_NUM['九'] = 1
	j_NUM['十'] = 1
	j_NUM['百'] = 1
	j_NUM['千'] = 1
	j_NUM['万'] = 1
	j_NUM['亿'] = 1
	j_NUM['○'] = 1

	f_NUM['零'] = 1
	f_NUM['壹'] = 1
	f_NUM['贰'] = 1
	f_NUM['叁'] = 1
	f_NUM['肆'] = 1
	f_NUM['伍'] = 1
	f_NUM['陆'] = 1
	f_NUM['柒'] = 1
	f_NUM['捌'] = 1
	f_NUM['玖'] = 1
	f_NUM['拾'] = 1
	f_NUM['佰'] = 1
	f_NUM['仟'] = 1
	f_NUM['万'] = 1
	f_NUM['亿'] = 1
}

type numRecognition struct {
	gp         *graph
	quantifier bool
}

func MakeNumRecognition(gp *graph, quantifier bool) *numRecognition {
	return &numRecognition{
		gp:         gp,
		quantifier: quantifier,
	}
}

//填充图
func (r *numRecognition) Full() *numRecognition {
	return r
}

//数字+数字合并,zheng
func (r *numRecognition) Walk() *numRecognition {
	gp := r.gp
	for i := 0; i < gp.len-1; i++ {
		if len(gp.terms[i]) == 0 {
			continue
		}
		temp := gp.terms[i][0]

		if len(temp.name) == 1 {
			if merge(gp, i, temp.name[0], &j_NUM) {
				i--
				continue
			} else if merge(gp, i, temp.name[0], &f_NUM) {
				i--
				continue
			}

		}

		if temp.item.Natures != dat.NumNatures && !utils.IsNumByRunes(&temp.name) {
			continue
		}

		dotNum, begin, end := 0, i, i

		for j := i + len(temp.name); j < gp.len; j++ {
			if len(gp.terms[j]) == 0 {
				continue
			}

			if utils.IsNumByRunes(&gp.terms[j][0].name) {
				if gp.terms[j][0].name[0] == '.' {
					dotNum++
				} else {
					if dotNum > 1 {
						end = begin
						i = gp.terms[j][0].getToIndex()
						break
					}
					end = gp.terms[j][0].getToIndex()
				}
			} else {
				break
			}
		}

		if begin < end {
			gp.terms[begin][0] = &term{
				item:  dat.NewItem(dat.NumNatures),
				name:  gp.rs[begin:end],
				index: begin,
			}

			for j := begin + 1; j < end; j++ {
				gp.terms[j] = gp.terms[j][0:0]
			}
		}

		if r.quantifier { //开启量词识别

			toIndex := gp.terms[begin][0].getToIndex()

			if toIndex >= gp.len {
				continue
			}

			to := gp.terms[toIndex][0]

			if to.item.Natures.GetQua() {
				gp.terms[begin][0] = &term{
					item:  dat.NewItem(dat.MqNatures),
					name:  gp.rs[begin:to.getToIndex()],
					index: begin,
				}

				gp.terms[toIndex] = gp.terms[toIndex][0:0]
			}

		}

	}

	return r
}

//中文，数字合并
func merge(gp *graph, i int, r rune, m *map[rune]int) bool {
	if _, ok := (*m)[r]; !ok {
		return false;
	}

	end := 0

	for j := i + 1; j < gp.len-1; j++ {
		if len(gp.terms[j]) == 0 {
			continue
		}

		temp := gp.terms[j][0]

		if len(temp.name) == 1 {
			if _, ok := (*m)[temp.name[0]]; ok {
				end = j
				gp.terms[j] = gp.terms[j][0:0]
			} else {
				break
			}
		} else {
			break
		}
	}

	if end != 0 {

		gp.terms[i][0] = &term{
			item:  dat.NewItem(dat.NumNatures),
			name:  gp.rs[i : end+1],
			index: i,
		}

		return true
	}

	return false
}
