package webrtc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math/rand"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
)

const (
	kDtlsRecordHeaderLen int = 13
	kMinRtcpPacketLen    int = 4
	kMinRtpPacketLen     int = 12
)

func NowMs() uint32 {
	return uint32(time.Now().UTC().UnixNano() / int64(time.Millisecond))
}

func NowMs64() uint64 {
	return uint64(time.Now().UTC().UnixNano() / int64(time.Millisecond))
}

func Sleep(ms int) {
	timer := time.NewTimer(time.Millisecond * time.Duration(ms))
	<-timer.C
}

func RandomInt(n int) int {
	return rand.Intn(n)
}

func RandomUint32() uint32 {
	return rand.Uint32()
}

func RandomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

func Atou16(s string) uint16 {
	return uint16(Atoi(s))
}

func Atou32(s string) uint32 {
	return uint32(Atoi(s))
}

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

func Itoa(i int) string {
	return strconv.Itoa(i)
}

// uint16/uint32/uint64 ---> []byte
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

// bytes --Little-> uint16/uint32/uint64
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

// uint16/uint32/uint64
func ValueOrderChange(T interface{}, order binary.ByteOrder) interface{} {
	bytes := ValueToBytes(T)
	if bytes == nil {
		log.Println("[util] invalid bytes in ValueOrderChange")
		return 0
	}

	if len(bytes) == 2 {
		return order.Uint16(bytes[0:])
	} else if len(bytes) == 4 {
		return order.Uint32(bytes[0:])
	} else if len(bytes) == 8 {
		return order.Uint64(bytes[0:])
	} else {
		log.Println("[util] invalid length in ValueOrderChange")
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

func ReadBig(r io.Reader, data interface{}) error {
	return binary.Read(r, binary.BigEndian, data)
}

func ReadLittle(r io.Reader, data interface{}) error {
	return binary.Read(r, binary.LittleEndian, data)
}

func WriteBig(w io.Writer, data interface{}) error {
	return binary.Write(w, binary.BigEndian, data)
}

func WriteLittle(w io.Writer, data interface{}) error {
	return binary.Write(w, binary.LittleEndian, data)
}

func Min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func Max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

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

func Int16ToByteSlice(vals []int16) []byte {
	buf := make([]byte, len(vals)*2)
	for i, v := range vals {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
	}
	return buf
}

func IsDtlsPacket(data []byte) bool {
	if len(data) < kDtlsRecordHeaderLen {
		return false
	}
	return (data[0] > 19 && data[0] < 64)
}

func IsRtcpPacket(data []byte) bool {
	// If we're muxing RTP/RTCP, we must inspect each packet delivered and
	// determine whether it is RTP or RTCP. We do so by checking the packet type,
	// and assuming RTP if type is 0-63 or 96-127. For additional details, see
	// http://tools.ietf.org/html/rfc5761.
	// Note that if we offer RTCP mux, we may receive muxed RTCP before we
	// receive the answer, so we operate in that state too.

	if len(data) < kMinRtcpPacketLen {
		return false
	}
	flag := (data[0] & 0xC0)
	utype := (data[1] & 0x7F)
	return (flag == 0x80 && utype >= 64 && utype < 96)
}

func IsRtpRtcpPacket(data []byte) bool {
	if len(data) < kMinRtcpPacketLen {
		return false
	}
	return ((data[0] & 0xC0) == 0x80)
}

func IsRtpPacket(data []byte) bool {
	if len(data) < kMinRtpPacketLen {
		return false
	}
	return (IsRtpRtcpPacket(data) && !IsRtcpPacket(data))
}

func IsStunPacket(data []byte) bool {
	if len(data) < kStunHeaderSize {
		return false
	}

	if data[0] != 0 && data[0] != 1 {
		return false
	}

	buf := bytes.NewReader(data)

	var dtype uint16
	binary.Read(buf, binary.BigEndian, &dtype)
	if (dtype & 0x8000) != 0 {
		// RTP/RTCP
		return false
	}

	var length uint16
	binary.Read(buf, binary.BigEndian, &length)
	if (length & 0x0003) != 0 {
		// It should be multiple of 4
		return false
	}

	var magic uint32
	binary.Read(buf, binary.BigEndian, &magic)
	if magic != kStunMagicCookie {
		// If magic cookie is invalid, only support RFC5389, not including RFC3489
		return false
	}

	return true
}

func ParseRtpSeqInRange(seqn, start, size uint16) bool {
	var n int = int(seqn)
	var nh int = ((1 << 16) + n)
	var s int = int(start)
	var e int = s + int(size)
	return (s <= n && n < e) || (s <= nh && nh < e)
}

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

/// StringPair like std::pair

type StringPair struct {
	first  string
	second string
}

func (sp StringPair) ToStringBySpace() string {
	return sp.first + " " + sp.second
}

func NetAddrString(addr net.Addr) string {
	if strings.Contains(addr.String(), "://") {
		return addr.String()
	} else {
		return addr.Network() + "://" + addr.String()
	}
}

/// Cached Conn

func NewNetConn(c net.Conn) *NetConn {
	return &NetConn{nil, c, c}
}

type NetConn struct {
	cached   []byte
	nc       net.Conn
	net.Conn // most methods of net.Conn are embedded
}

func (c *NetConn) LocalAddr() net.Addr {
	return c.nc.LocalAddr()
}

func (c *NetConn) RemoteAddr() net.Addr {
	return c.nc.RemoteAddr()
}

func (c *NetConn) preload_(n int) error {
	if n <= 0 {
		return nil
	}
	hadLen := len(c.cached)
	if hadLen >= n {
		return nil
	} else {
		buf := make([]byte, n-hadLen)
		nret, err := c.nc.Read(buf)
		if err != nil {
			return err
		}
		c.cached = append(c.cached, buf[0:nret]...)
		if nret != len(buf) {
			return errors.New("[NetConn] no enough data")
		}
		return nil
	}
}

func (c *NetConn) Peek(n int) ([]byte, error) {
	err := c.preload_(n)
	if err != nil {
		return nil, err
	}
	return c.cached[0:n], nil
}

func (c *NetConn) Read(p []byte) (int, error) {
	need := Min(len(c.cached), len(p))
	if need > 0 {
		copy(p, c.cached[0:need])
		c.cached = c.cached[need:]
		return need, nil
	} else {
		return c.nc.Read(p)
	}
}

func (c *NetConn) Write(p []byte) (int, error) {
	return c.nc.Write(p)
}

func (c *NetConn) Close() error {
	return c.nc.Close()
}

type SocketFD interface {
	File() (f *os.File, err error)
}

func SetSocketReuseAddr(sock SocketFD) {
	if file, err := sock.File(); err == nil {
		log.Println("[util] set reuse addr")
		syscall.SetsockoptInt(int(file.Fd()), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	}
}

// LocalIP tries to determine a non-loopback address for the local machine
func LocalIP() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.IsGlobalUnicast() {
			if ipnet.IP.To4() != nil || ipnet.IP.To16() != nil {
				return ipnet.IP, nil
			}
		}
	}
	return nil, nil
}

func LocalIPString() string {
	ip, err := LocalIP()
	if err != nil {
		log.Print("[WARN] Error determining local ip address. ", err)
		return ""
	}
	if ip == nil {
		log.Print("[WARN] Could not determine local ip address")
		return ""
	}
	return ip.String()
}
