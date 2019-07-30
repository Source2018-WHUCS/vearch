package domain

import (
	"bytes"
	"github.com/tiglabs/iseg/utils"
)

type Term struct {
	//词语，词性
	Name, Nature string
	//偏移量，词频
	Offset, Freq int
}

type Result struct {
	Terms []Term
	Others []Term
}

func (r *Result) NameNatureStr() string {
	if r.Terms == nil {
		return ""
	}
	buffer := bytes.Buffer{}

	for i := 0; i < len(r.Terms); i++ {
		term := &r.Terms[i]

		if utils.IsBlankStr(&term.Name) {
			continue
		}

		buffer.WriteString(term.Name)
		buffer.WriteString("/")
		buffer.WriteString(term.Nature)
		buffer.WriteString(" ")
	}


	if len(r.Others)>0 {
		buffer.WriteString("||| ")
		for i := 0; i < len(r.Others); i++ {
			term := &r.Others[i]

			if utils.IsBlankStr(&term.Name) {
				continue
			}

			buffer.WriteString(term.Name)
			buffer.WriteString("/")
			buffer.WriteString(term.Nature)
			buffer.WriteString(" ")
		}
	}

	return buffer.String()
}

func (r *Result) String() string {
	if r.Terms == nil {
		return ""
	}
	buffer := bytes.Buffer{}

	for i := 0; i < len(r.Terms); i++ {
		term := &r.Terms[i]

		if utils.IsBlankStr(&term.Name) {
			continue
		}

		buffer.WriteString(term.Name)
		buffer.WriteString(" ")
	}


	if len(r.Others)>0{
		buffer.WriteString("||| ")

		for i := 0; i < len(r.Others); i++ {
			term := &r.Others[i]

			if utils.IsBlankStr(&term.Name) {
				continue
			}

			buffer.WriteString(term.Name)
			buffer.WriteString(" ")
		}
	}



	return buffer.String()
}
