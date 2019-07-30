package utils

import (
	"bufio"
	"os"
)

type MyFile struct {
	path string
	file *os.File
}

func NewMyFile(path string) (*MyFile, error) {
	if file, e := os.Open(path); e == nil {
		mf := &MyFile{
			path: path,
			file: file,
		}
		return mf, e
	} else {
		return nil, e
	}
}

//获取一个scanner
func (f *MyFile) Scanner(split bufio.SplitFunc) *bufio.Scanner {
	scanner := bufio.NewScanner(f.file)
	scanner.Split(split)
	return scanner
}

func (f *MyFile) Close() {
	if f.file != nil {
		f.file.Close()
	}
}
