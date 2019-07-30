package utils

//数组往后追加,超出数组会发生越界，没有做扩容。感觉没啥必要
func AddLast() func([]interface{}, interface{}) int {
	var index = -2
	return func(arr []interface{}, obj interface{}) int {
		if index == -2 {
			index = len(arr)
		}
		index--
		arr[index] = obj
		return index
	}
}
