package util

import (
	"encoding/binary"
	"errors"
	"io"
	"math/rand"
	"reflect"
	"strconv"
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

// convert a string to uint16
func Atou16(s string) uint16 {
	return uint16(Atoi(s))
}

// convert a string to uint32
func Atou32(s string) uint32 {
	return uint32(Atoi(s))
}

// convert a string to int
func Atoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		buf := []byte(s)
		for k := range buf {
			if buf[k] < '0' || buf[k] > '9' {
				i, _ = strconv.Atoi(string(buf[0:k]))
				break
			}
		}
	}
	return i
}

// convert int to a string
func Itoa(i int) string {
	return strconv.Itoa(i)
}

// convert a uint16/uint32/uint64(Little-Endian) to []byte.
func ValueToBytes(T interface{}) []byte {
	size := reflect.TypeOf(T).Size()
	if size != 2 && size != 4 && size != 8 {
		return nil
	}

	bytes := make([]byte, size)
	if size == 2 {
		binary.LittleEndian.PutUint16(bytes, T.(uint16))
	} else if size == 4 {
		binary.LittleEndian.PutUint32(bytes, T.(uint32))
	} else if size == 8 {
		binary.LittleEndian.PutUint64(bytes, T.(uint64))
	} else {
		return nil
	}
	return bytes
}
func Uint16ToBytes(val uint16) []byte {
	return ValueToBytes(val)
}
func Uint32ToBytes(val uint32) []byte {
	return ValueToBytes(val)
}

// convert []byte to a uint16/uint32/uint64(Little-Endian)
func BytesToValue(bytes []byte) interface{} {
	size := len(bytes)
	if size == 2 {
		return binary.LittleEndian.Uint16(bytes)
	} else if size == 4 {
		return binary.LittleEndian.Uint32(bytes)
	} else if size == 8 {
		return binary.LittleEndian.Uint64(bytes)
	} else {
		return 0
	}
}
func BytesToUint16(bytes []byte) uint16 {
	return BytesToValue(bytes).(uint16)
}
func BytesToUint32(bytes []byte) uint32 {
	return BytesToValue(bytes).(uint32)
}

// convert a uint16/uint32/uint64(LittleEndian/BigEndian) to
// another uint16/uint32/uint64(BigEndian/LittleEndian).
func ValueOrderChange(T interface{}, order binary.ByteOrder) interface{} {
	bytes := ValueToBytes(T)
	if bytes == nil {
		Warnln(uTAG, "invalid bytes in ValueOrderChange")
		return 0
	}

	if len(bytes) == 2 {
		return order.Uint16(bytes[0:])
	} else if len(bytes) == 4 {
		return order.Uint32(bytes[0:])
	} else if len(bytes) == 8 {
		return order.Uint64(bytes[0:])
	} else {
		Warnln(uTAG, "invalid length in ValueOrderChange")
	}
	return 0
}
func HostToNet16(v uint16) uint16 {
	return ValueOrderChange(v, binary.BigEndian).(uint16)
}
func HostToNet32(v uint32) uint32 {
	return ValueOrderChange(v, binary.BigEndian).(uint32)
}
func NetToHost16(v uint16) uint16 {
	return ValueOrderChange(v, binary.LittleEndian).(uint16)
}
func NetToHost32(v uint32) uint32 {
	return ValueOrderChange(v, binary.LittleEndian).(uint32)
}

// read a uint16/uint32/uint64(BigEndian) from io.Reader
func ReadBig(r io.Reader, data interface{}) error {
	return binary.Read(r, binary.BigEndian, data)
}

// read a uint16/uint32/uint64(LittleEndian) from io.Reader
func ReadLittle(r io.Reader, data interface{}) error {
	return binary.Read(r, binary.LittleEndian, data)
}

// write a uint16/uint32/uint64(BigEndian) to io.Writer
func WriteBig(w io.Writer, data interface{}) error {
	return binary.Write(w, binary.BigEndian, data)
}

// write a uint16/uint32/uint64(LittleEndian) to io.Writer
func WriteLittle(w io.Writer, data interface{}) error {
	return binary.Write(w, binary.LittleEndian, data)
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

// converts []byte to []int16(LittleEndian).
func ByteToInt16Slice(buf []byte) ([]int16, error) {
	if len(buf)%2 != 0 {
		return nil, errors.New("trailing bytes")
	}
	vals := make([]int16, len(buf)/2)
	for i := 0; i < len(vals); i++ {
		val := binary.LittleEndian.Uint16(buf[i*2:])
		vals[i] = int16(val)
	}
	return vals, nil
}

// converts []int16(LittleEndian) to []byte.
func Int16ToByteSlice(vals []int16) []byte {
	buf := make([]byte, len(vals)*2)
	for i, v := range vals {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
	}
	return buf
}

// return true if RTP-SEQ(uint16) seqn between (start, start+size).
func ParseRtpSeqInRange(seqn, start, size uint16) bool {
	var n int = int(seqn)
	var nh int = ((1 << 16) + n)
	var s int = int(start)
	var e int = s + int(size)
	return (s <= n && n < e) || (s <= nh && nh < e)
}

// return true if RTP-SEQ(uint16) seq1 > seq2.
func CompareRtpSeq(seq1, seq2 uint16) int {
	diff := seq1 - seq2
	if diff != 0 {
		if diff <= 0x8000 {
			return 1
		} else {
			return -1
		}
	} else {
		return 0
	}
}

// like c++ std::pair
type StringPair struct {
	First  string
	Second string
}

type IntPair struct {
	First  int
	Second int
}

func (sp StringPair) ToStringBySpace() string {
	return sp.First + " " + sp.Second
}
