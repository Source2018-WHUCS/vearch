// +build appengine

package bytes

// ByteToString convert bytes to string
func ByteToString(b []byte) string {
	return string(b)
}

// StringToByte convert string to bytes
func StringToByte(s string) []byte {
	return []byte(s)
}
