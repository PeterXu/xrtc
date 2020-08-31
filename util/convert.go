package util

import (
	"encoding/binary"
	"errors"
	"io"
	"reflect"
	"strconv"
)

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
func BytesToIntValue(bytes []byte) interface{} {
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
	return BytesToIntValue(bytes).(uint16)
}
func BytesToUint32(bytes []byte) uint32 {
	return BytesToIntValue(bytes).(uint32)
}
func BytesToUint64(bytes []byte) uint64 {
	return BytesToIntValue(bytes).(uint64)
}

// convert a uint16/uint32/uint64(LittleEndian/BigEndian) to
// another uint16/uint32/uint64(BigEndian/LittleEndian).
func ValueOrderChange(T interface{}, order binary.ByteOrder) interface{} {
	bytes := ValueToBytes(T)
	if bytes == nil {
		LogWarnln(uTAG, "invalid bytes in ValueOrderChange")
		return 0
	}

	if len(bytes) == 2 {
		return order.Uint16(bytes[0:])
	} else if len(bytes) == 4 {
		return order.Uint32(bytes[0:])
	} else if len(bytes) == 8 {
		return order.Uint64(bytes[0:])
	} else {
		LogWarnln(uTAG, "invalid length in ValueOrderChange")
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
