package utils

import (
	"unicode"
)

/**
 * 判断字符串是否为空
 *
 * @param cs
 * @return
 */
func IsBlank(cs *[]rune) bool {
	if cs == nil || len(*cs) == 0 {
		return true
	}
	for i := 0; i < len(*cs); i++ {
		if !unicode.IsSpace((*cs)[i]) {
			return false
		}
	}
	return true
}

func IsBlankStr(str *string) bool {
	if str == nil || len(*str) == 0 {
		return true
	}
	cs := []rune(*str)
	return IsBlank(&cs)
}
