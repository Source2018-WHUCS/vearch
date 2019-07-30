package util

import (
	"net"
	"unsafe"
)

func YoloString(b []byte) string {
	return *((*string)(unsafe.Pointer(&b)))
}

func Ping(addr string) (ok bool) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return false
	}

	tcpConn, err := net.DialTCP("tcp4", nil, tcpAddr)
	if err != nil {
		return false
	}

	tcpConn.Close()
	return true
}
