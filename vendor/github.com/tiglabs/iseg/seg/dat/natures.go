package dat

var DefaultNatures = &Natures{"null", 0, make([]nature, 0), false}

var BeginNatures = &Natures{"B", 50610, make([]nature, 0), false}

var EndNatures = &Natures{"E", 50610, make([]nature, 0), false}

var NullNatures = &Natures{"", 0, make([]nature, 0), false}

var NumNatures = &Natures{"num", 0, make([]nature, 0), false}

var MqNatures = &Natures{"mq", 0, make([]nature, 0), false}

var EnNatures = &Natures{"en", 0, make([]nature, 0), false}

var PersonNatures = &Natures{"nr", 0, make([]nature, 0), false}

var ForeignNatures = &Natures{"nrf", 0, make([]nature, 0), false}

type nature struct {
	name string
	freq int
}

type Natures struct {
	name string
	freq int
	all  []nature
	qua  bool
}

//得到一个term的freq
func (n Natures) GetAllFreq() int {
	return n.freq
}

//得到一个term的name
func (n Natures) GetName() string {
	return n.name
}

//得到一个term的name
func (n Natures) GetQua() bool {
	return n.qua
}
