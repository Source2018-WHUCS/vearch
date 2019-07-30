package utils

const (
	//这个就是(int)'ａ'
	MIN_LOWER = 65345

	/**
	 * 这个就是(int)'ｚ'
	 */
	MAX_LOWER = 65370

	/**
	 * 差距进行转译需要的
	 */
	LOWER_GAP = 65248

	/**
	 * 这个就是(int)'Ａ'
	 */
	MIN_UPPER = 65313

	/**
	 * 这个就是(int)'Ｚ'
	 */
	MAX_UPPER = 65338

	/**
	 * 差距进行转译需要的
	 */
	UPPER_GAP = 65216

	/**
	 * 这个就是(int)'A'
	 */
	MIN_UPPER_E = 65

	/**
	 * 这个就是(int)'Z'
	 */
	MAX_UPPER_E = 90

	/**
	 * 差距进行转译需要的
	 */
	UPPER_GAP_E = -32
	/**
	 * 这个就是(int)'０'
	 */
	MIN_UPPER_N = 65296
	/**
	 * 这个就是(int)'９'
	 */
	MAX_UPPER_N = 65305
	/**
	 * 差距进行转译需要的
	 */
	UPPER_GAP_N = 65248
)

//所有的字符存储于此,
var charcovers = make([]rune, 65536)

func init() {
	for i := 0; i < 65536; i++ {
		if i >= MIN_LOWER && i <= MAX_LOWER {
			charcovers[i] = rune(i - LOWER_GAP)
		} else if i >= MIN_UPPER && i <= MAX_UPPER {
			charcovers[i] = rune(i - UPPER_GAP)
		} else if i >= MIN_UPPER_E && i <= MAX_UPPER_E {
			charcovers[i] = rune(i - UPPER_GAP_E)
		} else if i >= MIN_UPPER_N && i <= MAX_UPPER_N {
			charcovers[i] = rune(i - UPPER_GAP_N)
		} else {
			charcovers[i] = rune(i)
		}
	}
}

//修改一个rune数组中的，全角全部设置为半角
func AlertArr(runes []rune, start, len int) []rune {
	for i := start; i < start+len; i++ {
		runes[i] = Cover(runes[i])
	}
	return runes
}

//修改一个string中的，全角全部设置为半角，返回rune数组
func AlertStr(str string) []rune {
	runes := []rune(str)
	return AlertArr(runes, 0, len(runes))
}

//判断一个字符串是否是english
func IsEn(word string) bool {
	runes := []rune(word)
	return IsEnByRunes(&runes)
}

//判断一个rune是否是英文
func IsEnglish(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= MIN_LOWER && r <= MAX_LOWER) || (r >= MIN_UPPER && r <= MAX_UPPER) || (r >= MIN_UPPER_E && r <= MAX_UPPER_E)
}

//判断一个字符串是否是english
func IsEnByRunes(runes *[]rune) bool {
	for i := 0; i < len(*runes); i++ {
		c := (*runes)[i]
		if IsEnglish(c) {
		} else {
			return false
		}
	}
	return true
}

//判断一个字符串是否是数字
func IsNumByRunes(runes *[]rune) bool {
	for i := 0; i < len(*runes); i++ {
		c := (*runes)[i]
		if (c >= '0' && c <= '9') || c >= MIN_UPPER_N && c <= MAX_UPPER_N || c == '.' {
		} else {
			return false
		}
	}
	return true
}

//判断一个字符串是否是数字
func IsNum(str string) bool {
	runes := []rune(str)
	return IsEnByRunes(&runes)
}

//判断一个rune是否是数字
func IsNumber(r rune) bool {
	return (r >= '0' && r <= '9') || r >= MIN_UPPER_N && r <= MAX_UPPER_N
}

//将一个字符串标准化
func Cover(r rune) rune {
	if r > 65535 {
		return r
	}

	return charcovers[r]
}
