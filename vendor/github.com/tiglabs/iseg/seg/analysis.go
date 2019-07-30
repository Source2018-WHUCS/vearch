package seg

import (
	"github.com/tiglabs/iseg/seg/dict"
	"github.com/tiglabs/iseg/seg/domain"
	"github.com/tiglabs/iseg/seg/recognition"
	"github.com/tiglabs/iseg/tree"
)

//最细颗粒度分词方式
func Base(text string) *domain.Result {
	if len(text) == 0 { //长度为0
		return &domain.Result{}
	}
	graph := recognition.NewGraph(text)
	recognition.MakeDatRecognition(graph).Full().Walk()
	return graph.Result()
}

//智能分词
func Smart(text string, dicKeys ...string) *domain.Result {
	if len(text) == 0 { //长度为0
		return &domain.Result{}
	}

	graph := recognition.NewGraph(text)

	recognition.MakeDatRecognition(graph).Full().Walk()
	recognition.MakeNumRecognition(graph, true).Full().Walk()
	recognition.MakePersonRecognition(graph).Full().Walk()
	recognition.MakeForeignRecognition(graph).Full().Walk()
	recognition.MakeDicRecognition(graph, toTrees(dicKeys)).Full().Walk()





	return graph.Result()
}

//Index
func Index(text string, dicKeys ...string) *domain.Result {
	if len(text) == 0 { //长度为0
		return &domain.Result{}
	}

	graph := recognition.NewGraph(text)
	recognition.MakeDatRecognition(graph).Full().Walk()
	recognition.MakeNumRecognition(graph, true).Full().Walk()
	recognition.MakePersonRecognition(graph).Full().Walk()
	recognition.MakeForeignRecognition(graph).Full().Walk()
	trees := toTrees(dicKeys)
	recognition.MakeDicRecognition(graph, trees).Full().Walk()

	result := graph.Result()

	result.Others = recognition.MakeIndexRecognition(graph, trees).Others()

	return result
}

//load user define library
func toTrees(dicKeys []string) []*tree.TrieTree {
	var dics []*tree.TrieTree
	if len(dicKeys) > 0 {
		dics = make([]*tree.TrieTree, len(dicKeys))
		for i := 0; i < len(dicKeys); i++ {
			if t, found := dict.GetDic(dicKeys[i]); found {
				dics[i] = t
			}
		}
	} else {
		if t, found := dict.GetDic("default"); found {
			dics = []*tree.TrieTree{t,}
		}
	}
	return dics
}
