package util

import (
	"math/rand"
	"time"
)

const uTAG = "[UTIL]"

// return crrent UTC time(milliseconds) with 32bit
func NowMs() uint32 {
	return uint32(time.Now().UTC().UnixNano() / int64(time.Millisecond))
}

// return crrent UTC time(milliseconds) with 64bit
func NowMs64() uint64 {
	return uint64(time.Now().UTC().UnixNano() / int64(time.Millisecond))
}

// wait some milliseconds and then wake
func Sleep(ms int) {
	timer := time.NewTimer(time.Millisecond * time.Duration(ms))
	<-timer.C
}

// return a random int number.
func RandomInt(n int) int {
	return rand.Intn(n)
}

// return a random uint32 number.
func RandomUint32() uint32 {
	return rand.Uint32()
}

// return a random n-char(a-zA-Z0-9) string.
func RandomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

// return the minimum int of x,y
func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// return the maximum int of x,y
func Max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

// Abs return abs value
func Abs(x int) int {
	if x < 0 {
		return 0 - x
	}
	return x
}

// Clamp x in [min, max]
func Clamp(x, min, max int) int {
	if x < min {
		return min
	} else if x > max {
		return max
	}
	return x
}

// If for int value
func IfInt(condition bool, trueVal, falseVal int) int {
	if condition {
		return trueVal
	} else {
		return falseVal
	}
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
