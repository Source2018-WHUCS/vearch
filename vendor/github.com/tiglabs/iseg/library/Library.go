package library

import (
	"github.com/tiglabs/iseg/utils"
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
)

var libraryPath string

var logger = log.New(ioutil.Discard, "iseg_library", log.LstdFlags)

func Init() string { //获取绝对路径...

	defer logger.Println("libraryPath is : " + libraryPath)

	for i := 0; i < 10; i++ {
		if _, file, _, ok := runtime.Caller(i); ok {

			if strings.HasSuffix(file, "Library.go") {
				libraryPath = path.Dir(file)
				if _, err := os.Stat(libraryPath); err == nil {
					return libraryPath
				}
			}
		}
	}

	if env := os.ExpandEnv("$ISEG"); env != "" {
		if _, err := os.Stat(libraryPath); err == nil {
			libraryPath = env
			logger.Println("env exits but not found dir in %s ", env)
			return libraryPath
		}
	}

	if dicPath, err := os.Getwd(); err == nil {
		libraryPath = dicPath + "/dicts/iseg/"
		if _, err := os.Stat(libraryPath); err == nil {
			return libraryPath
		}
	}

	logger.Println("error: not found library path in any where ,please check it....")

	return ""
}

func GetCoreDic() (*utils.MyFile, error) {

	return utils.NewMyFile(libraryPath + "/core.dic")
}

func GetGramDic() (*utils.MyFile, error) {

	return utils.NewMyFile(libraryPath + "/bigramdict.dic")
}

func GetPerson() (*utils.MyFile, error) {
	return utils.NewMyFile(libraryPath + "/person/person.txt")
}

func GetPersonSplit() (*utils.MyFile, error) {
	return utils.NewMyFile(libraryPath + "/person/person_split.txt")
}

func GetForeign() (*utils.MyFile, error) {
	return utils.NewMyFile(libraryPath + "/person/foreign.txt")
}
