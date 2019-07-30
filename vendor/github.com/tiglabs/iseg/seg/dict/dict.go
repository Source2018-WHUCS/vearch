package dict

import (
	"github.com/tiglabs/iseg/tree"
	"io/ioutil"
	"log"
	"sync"
)

var logger = log.New(ioutil.Discard, "bleve_seg", log.LstdFlags)

var conf = make(map[string]string)

var dics = make(map[string]*tree.TrieTree)

var once sync.Once

func init() {
	conf["default"] = "../test_library/default.dic"
}

func Add(key, path string) {
	conf[key] = path
}

func AddDic(key string, tree *tree.TrieTree) {
	dics[key] = tree
}

var lock = sync.Mutex{}

func GetOrCreate(key string) *tree.TrieTree {
	if trieTree, ok := dics[key]; ok {
		return trieTree
	}

	lock.Lock()
	defer lock.Unlock()
	if dic, found := dics[key]; found {
		return dic
	}
	dics[key] = tree.MakeTree()
	return dics[key]

}

func GetDic(key string) (*tree.TrieTree, bool) {

	if trieTree, ok := dics[key]; ok {
		return trieTree, true
	}
	if path, ok := conf[key]; ok {
		var trieTree *tree.TrieTree
		var e error
		lock.Lock()
		defer lock.Unlock()
		if dic, found := dics[key]; found {
			return dic, found
		}
		if trieTree, e = tree.MakeTree().Load(path); e == nil {
			dics[key] = trieTree
		} else {
			logger.Printf("load %s err,  file for %s", key, path)
		}
		dic, found := dics[key]
		return dic, found
	} else {
		return nil, false
	}
}
