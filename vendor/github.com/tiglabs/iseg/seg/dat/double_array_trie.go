package dat

import (
	"bufio"
	"github.com/tiglabs/iseg/library"
	"strconv"
	"strings"
)

var Items []*Item

type Item struct {
	Base, Check, Status int
	Name                string
	Natures             *Natures
}

var NullItem = NewItem(NullNatures)

func NewItem(nr *Natures) *Item {
	return &Item{Natures: nr}
}

func CreateItem(name string, freq int) *Item {
	return &Item{
		Natures: &Natures{
			name: name,
			freq: freq,
		},
	}
}

func makeItem(text string) (*Item, int) {
	split := strings.Split(text, "\t")
	item := new(Item)
	index, _ := strconv.Atoi(split[0])

	item.Base, _ = strconv.Atoi(split[2])
	item.Check, _ = strconv.Atoi(split[3])
	item.Status, _ = strconv.Atoi(split[4])

	if item.Status > 1 {
		item.Name = split[1]
		item.Natures = new(Natures).makeNatures(split[5])
	}

	return item, index

}
func (t *Natures) makeNatures(natureStr string) *Natures {
	split := strings.Split(natureStr, ",")
	t.all = make([]nature, len(split))

	sum := 0
	for i := 0; i < len(split); i++ {
		n, qua := makeNature(split[i])

		if qua {
			t.qua = qua
		}

		if t.name == "" || t.freq < n.freq { //将最大的设置为默认词性
			t.freq, t.name = n.freq, n.name
		}
		sum += n.freq
		t.all[i] = n
	}
	t.freq = sum //词频设置为所有词性的和
	return t
}
func makeNature(s string) (nature, bool) {
	split := strings.Split(s, "=")
	freq, _ := strconv.Atoi(split[1])

	sl := strings.Split(split[0], "_")

	return nature{sl[0], freq}, len(sl) == 2
}

func Init() {
	file, e := library.GetCoreDic()
	if e != nil {
		panic(e)
	}
	defer file.Close()
	scanner := file.Scanner(bufio.ScanLines)
	scanner.Scan()
	if i, err := strconv.Atoi(scanner.Text()); err != nil { //读取数组长度
		panic(err)
	} else {
		Items = make([]*Item, i)
	}

	Items[0] = NewItem(NullNatures)
	for scanner.Scan() {
		text := scanner.Text()
		item, index := makeItem(text)
		Items[index] = item
	}
}

//find a item by name chars
func GetItem(rs []rune) *Item {
	if rs[0] > 66536 {
		return nil
	}
	item := Items[rs[0]]
	if item == nil {
		return nil
	}
	var c1 int
	var c2 int = int(rs[0])
	for i := 1; i < len(rs); i++ {
		code := int(rs[i])
		if item.Base+code > len(Items)-1 {
			return nil
		}
		c1 = item.Base + code

		item = Items[c1]

		if item == nil {
			return nil
		}
		if item.Check != -1 && item.Check != c2 {
			return nil
		}
		c2 = c1
	}
	if item.Status < 2 && item.Status != -1 {
		return nil
	}
	return item
}
