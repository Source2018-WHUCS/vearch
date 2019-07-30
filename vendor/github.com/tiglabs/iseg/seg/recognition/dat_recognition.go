package recognition

import (
	. "github.com/tiglabs/iseg/seg/dat"
)

type datRecognition struct {
	gp *graph
}

func MakeDatRecognition(gp *graph) *datRecognition {
	return &datRecognition{
		gp: gp,
	}
}

//填充图
func (r *datRecognition) Full() *datRecognition {
	gp := r.gp

	start, base := 0, 0

	var item *Item

	for i := 0; i < gp.len; i++ {

		if base == 0 { //说明是一个开始
			start = i
		}

		r := gp.rs[i]

		base, item = statement(base, r)
		switch item.Status {
		case 0:
			if start == i {
				runes := gp.rs[start : start+1]
				gp.add(start, NewItem(NullNatures), runes)
			} else {
				i = start
			}
			base = 0
		case 1:
			if start == i {
				runes := gp.rs[start : start+1]
				gp.add(start, NewItem(NullNatures), runes)
			} else if i == gp.len-1 {
				i = start
				base = 0
			}

		case 2:
			gp.add(start, item, gp.rs[start:i+1])
			if i == gp.len-1 { //如果走到最后一个字那么和3一样
				base, i = 0, start
			}
		case 3:
			gp.add(start, item, gp.rs[start:i+1])
			base, i = 0, start
		case 4:
			for i++; i < gp.len; i++ {
				if getItem(int(gp.rs[i])).Status == 4 {
					continue
				} else {
					break
				}
			}
			gp.add(start, NewItem(EnNatures), gp.rs[start:i])
			i--
			base = 0
		case 5:
			for i++; i < gp.len; i++ {
				if getItem(int(gp.rs[i])).Status == 5 {
					continue
				} else {
					break
				}
			}
			gp.add(start, NewItem(NumNatures), gp.rs[start:i])
			i--
			base = 0

		}
	}

	return r
}

func (r *datRecognition) Walk() {
	r.gp.walk(compute)
}

func statement(base int, r rune) (int, *Item) {
	check := base

	temp := Items[check]

	base = Items[check].Base + int(r)
	temp = getItem(base)
	if temp != NullItem && (temp.Check == check || temp.Check == -1) {
		return base, temp
	} else {
		return 0, NewItem(NumNatures)
	}

}

func getItem(index int) *Item {
	if index >= len(Items) {
		return NullItem
	}
	temp := Items[index]
	if temp == nil {
		return NullItem
	} else {
		return temp
	}
}
