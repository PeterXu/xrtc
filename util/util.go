package util

import (
	"time"
)

const uTAG = "[UTIL]"

// return crrent UTC time(milliseconds) with 32bit
func NowMs32() uint32 {
	return uint32(time.Now().UTC().UnixNano() / int64(time.Millisecond))
}

// return crrent UTC time(milliseconds) with 64bit
func NowMs() int64 {
	return time.Now().UTC().UnixNano() / int64(time.Millisecond)
}

// wait some milliseconds and then wake
func Sleep(ms int) {
	timer := time.NewTimer(time.Millisecond * time.Duration(ms))
	<-timer.C
}

func Clone(data []byte) []byte {
	nret := len(data)
	buff := make([]byte, nret)
	copy(buff, data[0:nret])
	return buff
}

func CloneArray(array []string) []string {
	var newArray []string
	for _, item := range array {
		newArray = append(newArray, item)
	}
	return newArray
}
