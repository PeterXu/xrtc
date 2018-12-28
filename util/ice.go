package util

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"unsafe"
)

// StunMessageType 2-bytes
type StunMessageType uint16

// These are the types of STUN messages defined in RFC 5389.
const (
	STUN_BINDING_REQUEST        StunMessageType = 0x0001
	STUN_BINDING_INDICATION                     = 0x0011
	STUN_BINDING_RESPONSE                       = 0x0101
	STUN_BINDING_ERROR_RESPONSE                 = 0x0111
)

// StunAttributeType 2-bytes
type StunAttributeType uint16

// These are all known STUN attributes, defined in RFC 5389 and elsewhere.
// Next to each is the name of the class (T is StunTAttribute) that implements
// that type.
// RETRANSMIT_COUNT is the number of outstanding pings without a response at
// the time the packet is generated.
const (
	STUN_ATTR_MAPPED_ADDRESS     StunAttributeType = 0x0001 // Address
	STUN_ATTR_USERNAME                             = 0x0006 // ByteString
	STUN_ATTR_MESSAGE_INTEGRITY                    = 0x0008 // ByteString, 20 bytes
	STUN_ATTR_ERROR_CODE                           = 0x0009 // ErrorCode
	STUN_ATTR_UNKNOWN_ATTRIBUTES                   = 0x000a // UInt16List
	STUN_ATTR_REALM                                = 0x0014 // ByteString
	STUN_ATTR_NONCE                                = 0x0015 // ByteString
	STUN_ATTR_XOR_MAPPED_ADDRESS                   = 0x0020 // XorAddress
	STUN_ATTR_SOFTWARE                             = 0x8022 // ByteString
	STUN_ATTR_ALTERNATE_SERVER                     = 0x8023 // ByteString
	STUN_ATTR_FINGERPRINT                          = 0x8028 // UInt32
	STUN_ATTR_RETRANSMIT_COUNT                     = 0xFF00 // UInt32

	// RFC 5245 ICE STUN attributes.
	STUN_ATTR_PRIORITY        = 0x0024 // UInt32
	STUN_ATTR_USE_CANDIDATE   = 0x0025 // No content, Length = 0
	STUN_ATTR_ICE_CONTROLLING = 0x802A // UInt64
	STUN_ATTR_NETWORK_INFO    = 0xC057 // UInt32
)

// StunAttributeValueType 4bytes
type StunAttributeValueType int

// These are the types of the values associated with the attributes above.
// This allows us to perform some basic validation when reading or adding
// attributes. Note that these values are for our own use, and not defined in
// RFC 5389.
const (
	STUN_VALUE_UNKNOWN     StunAttributeValueType = iota
	STUN_VALUE_ADDRESS                            = 1
	STUN_VALUE_XOR_ADDRESS                        = 2
	STUN_VALUE_UINT32                             = 3
	STUN_VALUE_UINT64                             = 4
	STUN_VALUE_BYTE_STRING                        = 5
	STUN_VALUE_ERROR_CODE                         = 6
	STUN_VALUE_UINT16_LIST                        = 7
)

// StunAddressFamily 1byte
type StunAddressFamily uint8

// These are the types of STUN addresses defined in RFC 5389.
const (
	// NB: UNDEF is not part of the STUN spec.
	STUN_ADDRESS_UNDEF StunAddressFamily = 0
	STUN_ADDRESS_IPV4                    = 1
	STUN_ADDRESS_IPV6                    = 2
)

const (
	// The mask used to determine whether a STUN message is a request/response etc.
	kStunTypeMask uint32 = 0x0110

	// STUN Attribute header length.
	kStunAttributeHeaderSize int = 4

	// Following values correspond to RFC5389.
	kStunHeaderSize          int    = 20
	kStunTransactionIdOffset int    = 8
	kStunTransactionIdLength int    = 12
	kStunMagicCookie         uint32 = 0x2112A442
	kStunMagicCookieLength   int    = 4

	// Following value corresponds to an earlier version of STUN from
	// RFC3489.
	kStunLegacyTransactionIdLength int = 16

	// STUN Message Integrity HMAC length.
	kStunMessageIntegritySize int = 20

	STUN_FINGERPRINT_XOR_VALUE uint32 = 0x5354554E
)

func NewStunMessageRequest() *StunMessage {
	return &StunMessage{
		Dtype:   STUN_BINDING_REQUEST,
		TransId: RandomString(kStunTransactionIdLength),
	}
}

func NewStunMessageResponse(transId string) *StunMessage {
	return &StunMessage{
		Dtype:   STUN_BINDING_RESPONSE,
		TransId: transId,
	}
}

// StunMessage
// Records a complete STUN/TURN message. Each message consists of a type and
// any number of attributes. Each attribute is parsed into an instance of an
// appropriate class (see above).  The Get* methods will return instances of
// that attribute class.
type StunMessage struct {
	Dtype      StunMessageType
	Length     uint16
	Magic      uint32
	TransId    string
	Attrs      map[StunAttributeType]StunAttribute
	OrderAttrs []StunAttribute
}

// IceMessage is A RFC 5245 ICE STUN message.
type IceMessage struct {
	StunMessage
}

// Read Parses the STUN packet in the given buffer and records it here. The
// return value indicates whether this was successful.
func (m *StunMessage) Read(data []byte) bool {
	if len(data) < kStunHeaderSize {
		Warnln("[ice] invalid stun size=", len(data))
		return false
	}

	// check the 1st byte
	utype := data[0]
	if utype != 0 && utype != 1 {
		Warnln("[ice] invalid utype=", utype)
		return false
	}

	// create Reader
	buf := bytes.NewReader(data)

	// 0-2, read stun message type
	ReadBig(buf, &m.Dtype)
	//Println("[ice] message type=", m.Dtype)
	if (m.Dtype & 0x8000) != 0 {
		// RTP and RTCP set the MSB of first byte, since first two bits are version,
		// and version is always 2 (10). If set, this is not a STUN packet.
		Warnln("[ice] not stun message, (RTP/RTCP)type=", m.Dtype)
		return false
	}

	// 2-4, read stun message size
	ReadBig(buf, &m.Length)
	//Println("[ice] message length=", m.Length)
	if (m.Length & 0x0003) != 0 {
		Warnln("[ice] invalid message length=", m.Length)
		return false
	}

	// 4-8, read stun magic
	ReadBig(buf, &m.Magic)
	//Println("[ice] read magic:", m.Magic, kStunMagicCookie)

	// 8-20, read stun transaction id
	var transId [kStunTransactionIdLength]byte
	if ret, err := buf.Read(transId[:]); err != nil || ret != kStunTransactionIdLength {
		Warnln("[ice] invalid transid ret=", ret, ", err=", err)
		return false
	}

	if m.Magic != kStunMagicCookie {
		// If magic cookie is invalid it means that the peer implements
		// RFC3489 instead of RFC5389.
		m.TransId = string(ValueToBytes(m.Magic)[:]) + string(transId[:])
	} else {
		m.TransId = string(transId[:])
	}
	// kStunTransactionIdLength/kStunLegacyTransactionIdLength
	//Printf("[ice] message magic=%x, transId=%s_%d\n", m.Magic, m.TransId, len(m.TransId))

	if int(m.Length) != buf.Len() {
		// TODO: length= 80 , Len= 696
		// invalid length= 108 , Len= 31
		Warnln("[ice] invalid length=", m.Length, ", Len=", buf.Len(), len(data), m.TransId)
		return false
	}

	if m.Attrs == nil {
		m.Attrs = make(map[StunAttributeType]StunAttribute)
	}

	for {
		if buf.Len() < 4 {
			//Warnln("[ice] no more data len=", buf.Len())
			break
		}

		var attrType StunAttributeType
		var attrLen uint16
		ReadBig(buf, &attrType)
		ReadBig(buf, &attrLen)
		//Println("[ice] attrType, attrLen=", attrType, attrLen)

		var attr StunAttribute
		switch attrType {
		case STUN_ATTR_MAPPED_ADDRESS:
			attr = &StunAddressAttribute{}
		case STUN_ATTR_XOR_MAPPED_ADDRESS:
			attr = &StunXorAddressAttribute{}
		case STUN_ATTR_USERNAME:
			attr = &StunByteStringAttribute{}
		case STUN_ATTR_ERROR_CODE:
			attr = &StunErrorCodeAttribute{}
		//case STUN_ATTR_MESSAGE_INTEGRITY:
		//case STUN_ATTR_FINGERPRINT:
		//case STUN_ATTR_PRIORITY:
		//case STUN_ATTR_USE_CANDIDATE:
		//case STUN_ATTR_ICE_CONTROLLING:
		//case STUN_ATTR_NETWORK_INFO:
		default:
			newLen := attrLen
			if remainder := attrLen % 4; remainder > 0 {
				padding := 4 - remainder
				newLen += padding
			}
			buf.Seek(int64(newLen), io.SeekCurrent)
		}

		// save attr
		if attr != nil {
			attr.SetInfo(attrType, attrLen, m.TransId)
			attr.Read(buf)
			m.Attrs[attrType] = attr
			m.OrderAttrs = append(m.OrderAttrs, attr)
		}
	}

	return true
}

// Write writes this object into a STUN packet. The return value indicates whether
// this was successful.
func (m *StunMessage) Write(buf *bytes.Buffer) bool {
	// 0-2, stun type
	WriteBig(buf, m.Dtype)
	// 2-4, stun body length
	WriteBig(buf, m.Length)
	if !m.IsLegacy() {
		// 4-8, stun magic
		WriteBig(buf, kStunMagicCookie)
	}
	// 8-20, stun transId
	buf.WriteString(m.TransId)
	// head: 2+2+[4]+12
	//Println("[ice] write stun headLen=", buf.Len(), ", bodyLen=", m.Length)

	// m.Length: stun body
	// STUN_ATTR_USERNAME: 2+2+username
	// STUN_ATTR_MESSAGE_INTEGRITY: 2+2+20
	// STUN_ATTR_FINGERPRINT: 2+2+4
	for _, attr := range m.OrderAttrs {
		// 2bytes attr type
		WriteBig(buf, attr.GetType())
		// 2bytes attr len
		WriteBig(buf, attr.GetLen2())
		//Println("[ice] write, attrType, attrLen=", attr.GetType(), attr.GetLen2())
		if !attr.Write(buf) {
			Warnln("[ice] fail to write buf from stunmessage")
			return false
		}
	}
	return true
}

// IsLegacy Returns true if the message confirms to RFC3489 rather than
// RFC5389. The main difference between two version of the STUN
// protocol is the presence of the magic cookie and different length
// of transaction ID. For outgoing packets version of the protocol
// is determined by the lengths of the transaction ID.
func (m *StunMessage) IsLegacy() bool {
	if len(m.TransId) == kStunLegacyTransactionIdLength {
		return true
	}
	return false
}

func (m *StunMessage) SetType(dtype StunMessageType) {
	m.Dtype = dtype
}

func (m *StunMessage) SetTransactionID(transId string) bool {
	if !m.IsValidTransactionId(transId) {
		return false
	}
	m.TransId = transId
	return true
}

func (m *StunMessage) IsValidTransactionId(transId string) bool {
	if len(transId) == kStunTransactionIdLength ||
		len(transId) == kStunLegacyTransactionIdLength {
		return true
	}
	return false
}

func (m *StunMessage) GetAttribute(atype StunAttributeType) StunAttribute {
	if m.Attrs != nil {
		if attr, ok := m.Attrs[atype]; ok {
			return attr
		}
	}
	return nil
}

func (m *StunMessage) AddAttribute(attr StunAttribute) {
	if m.Attrs == nil {
		m.Attrs = make(map[StunAttributeType]StunAttribute)
	}
	m.Attrs[attr.GetType()] = attr
	m.OrderAttrs = append(m.OrderAttrs, attr)
	attr_length := attr.GetLen2()
	if (attr_length % 4) != 0 {
		attr_length += (4 - (attr_length % 4))
	}
	m.Length += (attr_length + 4)
}

// AddMessageIntegrity Adds a MESSAGE-INTEGRITY attribute that is valid for the current message.
func (m *StunMessage) AddMessageIntegrity(key string) bool {
	integrityAttr := &StunByteStringAttribute{}
	integrityAttr.SetType(STUN_ATTR_MESSAGE_INTEGRITY)
	zeros := make([]byte, kStunMessageIntegritySize)
	for i := 0; i < len(zeros); i++ {
		zeros[i] = '0'
	}
	integrityAttr.CopyBytes(zeros)
	m.AddAttribute(integrityAttr)

	var buf bytes.Buffer
	if !m.Write(&buf) {
		Warnln("[ice] hmac buf is null")
		return false
	}

	attrLen := int(integrityAttr.GetLen2())
	msg_len_for_hmac := buf.Len() - kStunAttributeHeaderSize - attrLen

	macFunc := hmac.New(sha1.New, []byte(key))
	macFunc.Write(buf.Bytes()[0:msg_len_for_hmac])
	digest := macFunc.Sum(nil)

	//Printf("[ice] hmac bufLen=%d, attrLen=%d, msglen=%d, digestlen=%d\n",
	//	buf.Len(), attrLen, msg_len_for_hmac, len(digest))
	if len(digest) != kStunMessageIntegritySize {
		Warnln("[ice] hmac digest wrong, len=", len(digest), string(digest))
		return false
	}
	//Println("[ice] go hmac digest, len=", len(digest), digest)
	integrityAttr.CopyBytes(digest)

	return true
}

func (m *StunMessage) ValidateMessageIntegrity() bool {
	return false
}

// AddFingerprint Adds a FINGERPRINT attribute that is valid for the current message.
func (m *StunMessage) AddFingerprint() bool {
	fingerprinAttr := &StunUInt32Attribute{}
	fingerprinAttr.SetType(STUN_ATTR_FINGERPRINT)
	m.AddAttribute(fingerprinAttr)

	var buf bytes.Buffer
	if !m.Write(&buf) {
		Warnln("[ice] fail to AddFingerprint")
		return false
	}

	attrLen := int(fingerprinAttr.GetLen2())
	msg_len_for_crc32 := buf.Len() - kStunAttributeHeaderSize - attrLen

	const kCrc32Polynomial uint32 = 0xEDB88320
	crc32q := crc32.MakeTable(kCrc32Polynomial)
	crc := crc32.Checksum(buf.Bytes()[0:msg_len_for_crc32], crc32q)
	//Println("[ice] go stun crc32=", crc, msg_len_for_crc32, buf.Len())

	fingerprinAttr.SetValue(crc ^ STUN_FINGERPRINT_XOR_VALUE)
	return true
}

// StunAttribute: Base class for all STUN/TURN attributes.
type StunAttribute interface {
	// Reads the body (not the type or length) for this type of attribute from
	// the given buffer.  Return value is true if successful.
	Read(buf *bytes.Reader) bool
	// Writes the body (not the type or length) to the given buffer.  Return
	// value is true if successful.
	Write(buf *bytes.Buffer) bool
	SetInfo(attrType StunAttributeType, attrLen uint16, transId string)
	GetType() StunAttributeType
	GetLen() uint16  // for read
	GetLen2() uint16 // for write
	GetTransId() string
}

// StunAttributeBase
type StunAttributeBase struct {
	attrType StunAttributeType
	attrLen  uint16
	transId  string
}

func (a *StunAttributeBase) Check(buf *bytes.Reader) bool {
	if buf.Len() < int(a.attrLen) {
		Warnln("[ice] no enough data, length=", buf.Len(), ", require len=", a.attrLen)
		return false
	}
	return true
}

func (a *StunAttributeBase) SetType(attrType StunAttributeType) {
	a.attrType = attrType
}

func (a *StunAttributeBase) SetInfo(attrType StunAttributeType, attrLen uint16, transId string) {
	a.attrType = attrType
	a.attrLen = attrLen
	a.transId = transId
}

func (a *StunAttributeBase) GetType() StunAttributeType {
	return a.attrType
}

func (a *StunAttributeBase) GetLen() uint16 {
	return a.attrLen
}

func (a *StunAttributeBase) GetLen2() uint16 {
	return a.attrLen
}

func (a *StunAttributeBase) GetTransId() string {
	return a.transId
}

func (a *StunAttributeBase) ConsumePadding(buf *bytes.Reader, length int) bool {
	if remainder := length % 4; remainder > 0 {
		padding := make([]byte, 4-remainder)
		buf.Read(padding)
	}
	return true
}

func (a *StunAttributeBase) WritePadding(buf *bytes.Buffer, length int) bool {
	if remainder := length % 4; remainder > 0 {
		zeros := []byte{0, 0, 0, 0}
		padding := 4 - remainder
		buf.Write(zeros[0:padding])
	}
	return true
}

// StunAddressAttribute implements STUN attributes that record an Internet address.
type StunAddressAttribute struct {
	StunAttributeBase
	family StunAddressFamily
	port   uint16
	ip     net.IP
}

func (a *StunAddressAttribute) String() string {
	return a.ip.String() + ":" + fmt.Sprint(a.port)
}

func (a *StunAddressAttribute) GetLen2() uint16 {
	if a.family == STUN_ADDRESS_IPV4 {
		return 1 + 1 + 2 + 4
	} else {
		return 1 + 1 + 2 + 20
	}
}

func (a *StunAddressAttribute) Read(buf *bytes.Reader) bool {
	if !a.Check(buf) {
		return false
	}

	var dummy uint8
	if ReadBig(buf, &dummy) != nil {
		return false
	}

	// stun network famliy(ipv4/ipv6)
	if ReadBig(buf, &a.family) != nil {
		return false
	}

	// read port
	if ReadBig(buf, &a.port) != nil {
		return false
	}

	// read ip
	if a.family == STUN_ADDRESS_IPV4 {
		a.ip = make([]byte, 4)
		if ReadBig(buf, a.ip) != nil {
			Warnln("[ice] read ipv4 failed")
			return false
		}
	} else if a.family == STUN_ADDRESS_IPV6 {
		a.ip = make([]byte, 20)
		if ReadBig(buf, a.ip) != nil {
			Warnln("[ice] read ipv6 failed")
			return false
		}
	}

	return true
}

func (a *StunAddressAttribute) Write(buf *bytes.Buffer) bool {
	if a.family == STUN_ADDRESS_UNDEF {
		Warnln("[ice] Error writing address attribute: unknown family.")
		return false
	}
	var zero uint8 = 0
	WriteBig(buf, zero)
	WriteBig(buf, a.family)
	WriteBig(buf, a.port)
	WriteBig(buf, a.ip)
	return true
}

func (a *StunAddressAttribute) SetAddr(addr net.Addr) {
	strAddr := addr.String()
	if host, port, err := net.SplitHostPort(strAddr); err == nil {
		//Println("[ice] set addr:", strAddr)
		a.SetIP(net.ParseIP(host))
		a.SetPort(Atou16(port))
	} else {
		Warnln("[ice] fail to set addr:", strAddr)
	}
}

func (a *StunAddressAttribute) SetIP(ip net.IP) {
	a.ip = ip
	if ip.To4() != nil {
		a.family = STUN_ADDRESS_IPV4
	} else {
		a.family = STUN_ADDRESS_IPV6
	}
}

func (a *StunAddressAttribute) SetPort(port uint16) {
	a.port = port
}

// StunXorAddressAttribute implements STUN attributes that record an Internet address. When encoded
// in a STUN message, the address contained in this attribute is XORed with the
// transaction ID of the message.
type StunXorAddressAttribute struct {
	StunAttributeBase
	Addr    StunAddressAttribute
	XorIP   net.IP
	XorPort uint16
}

func (a *StunXorAddressAttribute) GetLen2() uint16 {
	if a.Addr.family == STUN_ADDRESS_IPV4 {
		return 1 + 1 + 2 + 4
	} else {
		return 1 + 1 + 2 + 20
	}
}

func (a *StunXorAddressAttribute) Read(buf *bytes.Reader) bool {
	if !a.Addr.Read(buf) {
		return false
	}
	a.XorPort = a.Addr.port ^ uint16(kStunMagicCookie>>16)
	a.GetXoredIP()
	return true
}

func (a *StunXorAddressAttribute) Write(buf *bytes.Buffer) bool {
	if a.Addr.family == STUN_ADDRESS_UNDEF {
		Warnln("[ice] invalid addr family in xoraddr")
		return false
	}

	var zero uint8 = 0
	// 1byte
	WriteBig(buf, zero)
	// 1byte
	WriteBig(buf, a.Addr.family)

	a.XorPort = a.Addr.port ^ uint16(kStunMagicCookie>>16)
	a.GetXoredIP()
	// 2bytes
	WriteBig(buf, a.XorPort)
	// 4bytes
	WriteBig(buf, a.XorIP)
	return true
}

func (a *StunXorAddressAttribute) GetXoredIP() {
	magic := HostToNet32(kStunMagicCookie)
	if a.Addr.family == STUN_ADDRESS_IPV4 {
		ip32 := HostToNet32(BytesToUint32(a.Addr.ip.To4()))
		xorip := Uint32ToBytes(ip32 ^ magic)
		a.XorIP = xorip
	} else if a.Addr.family == STUN_ADDRESS_IPV6 {
		//TODO
		a.XorIP = make([]byte, 20)
		if len(a.Addr.transId) == kStunTransactionIdLength {
			var transIds [3]uint32
			copy((*[12]byte)(unsafe.Pointer(&transIds[0]))[:], a.transId)

			dst := (*[4]uint32)(unsafe.Pointer(&a.XorIP[0]))[:]
			ipv6 := (*[4]uint32)(unsafe.Pointer(&a.Addr.ip[0]))[:]
			dst[0] = ipv6[0] ^ magic
			for i := 1; i < 4; i++ {
				dst[i] = ipv6[i] ^ transIds[i-1]
			}
		}
	}
}

// StunByteStringAttribute implements STUN attributes that record an arbitrary byte string.
type StunByteStringAttribute struct {
	StunAttributeBase
	Data []byte
}

func NewStunByteStringAttribute(attrType StunAttributeType, data []byte) *StunByteStringAttribute {
	attr := &StunByteStringAttribute{}
	attr.SetType(attrType)
	if data != nil && len(data) > 0 {
		attr.CopyBytes(data)
	}
	return attr
}

func (a *StunByteStringAttribute) GetLen2() uint16 {
	return uint16(len(a.Data))
}

func (a *StunByteStringAttribute) Read(buf *bytes.Reader) bool {
	if !a.Check(buf) {
		Warnln("[ice] invalid buf for StunByteStringAttribute")
		return false
	}

	a.Data = make([]byte, a.attrLen)
	if _, err := buf.Read(a.Data); err != nil {
		a.Data = nil
		Warnln("[ice] fail to read for StunByteStringAttribute")
		return false
	}
	a.ConsumePadding(buf, int(a.attrLen))
	return true
}

func (a *StunByteStringAttribute) Write(buf *bytes.Buffer) bool {
	buf.Write(a.Data)
	a.WritePadding(buf, len(a.Data))
	return true
}

func (a *StunByteStringAttribute) CopyBytes(data []byte) {
	a.Data = data
}

// StunUInt32Attribute implements STUN attributes that record a 32-bit integer.
type StunUInt32Attribute struct {
	StunAttributeBase
	bits uint32
}

func (a *StunUInt32Attribute) GetLen2() uint16 {
	return 4
}

func (a *StunUInt32Attribute) SetValue(value uint32) {
	a.bits = value
}

func (a *StunUInt32Attribute) GetBit(index int) bool {
	return ((a.bits >> uint32(index)) & 0x1) == 0x01
}

func (a *StunUInt32Attribute) SetBit(index int, value bool) {
	a.bits &= ^(1 << uint32(index))
	if value {
		a.bits |= (1 << uint32(index))
	}
}

func (a *StunUInt32Attribute) Read(buf *bytes.Reader) bool {
	if a.GetLen() != 4 {
		return false
	}
	ReadBig(buf, a.bits)
	return true
}

func (a *StunUInt32Attribute) Write(buf *bytes.Buffer) bool {
	WriteBig(buf, a.bits)
	return true
}

// Implements STUN attributes that record an error code.
// MIN_SIZE = 4
type StunErrorCodeAttribute struct {
	StunAttributeBase
	Class  uint8
	Number uint8
	Reason string
}

func (a *StunErrorCodeAttribute) Read(buf *bytes.Reader) bool {
	if buf.Len() < 4 {
		return false
	}

	reasonLen := buf.Len() - 4

	var val uint32
	if ReadBig(buf, &val) != nil {
		Warnln("[ice] fail to read 4-bytes in error-code")
		return false
	}

	if (val >> 11) != 0 {
		Warnln("[ice] error-code bits not zero")
	}

	a.Class = uint8((val >> 8) & 0x7)
	a.Number = uint8(val & 0xff)
	if reasonLen > 0 {
		data := make([]byte, reasonLen)
		if _, err := buf.Read(data); err != nil {
			Warnln("[ice] fail to read error-reason:", err)
			return false
		}
		a.Reason = string(data)
	}
	//Println("[ice] read error-code:", a)
	return true
}

func (a *StunErrorCodeAttribute) Write(buf *bytes.Buffer) bool {
	return false
}

func (a *StunErrorCodeAttribute) Code() int {
	return int(a.Class*100) + int(a.Number)
}

func (a *StunErrorCodeAttribute) SetCode(code int) {
	a.Class = uint8(code / 100)
	a.Number = uint8(code % 100)
}

func (a *StunErrorCodeAttribute) SetReason(reason string) {
	a.Reason = reason
}

// GenStunMessageRequest generates stun request packet
func GenStunMessageRequest(buf *bytes.Buffer, sendUfrag, recvUfrag, recvPwd string) bool {
	sendKey := recvUfrag + ":" + sendUfrag
	usernameAttr := NewStunByteStringAttribute(STUN_ATTR_USERNAME, []byte(sendKey))

	req := NewStunMessageRequest()
	req.AddAttribute(usernameAttr)
	req.AddMessageIntegrity(recvPwd)
	req.AddFingerprint()
	return req.Write(buf)
}

// GenStunMessageResponse generates stun response packet
func GenStunMessageResponse(buf *bytes.Buffer, passwd string, transId string, addr net.Addr) bool {
	xorAttr := &StunXorAddressAttribute{}
	xorAttr.SetType(STUN_ATTR_XOR_MAPPED_ADDRESS)
	xorAttr.Addr.SetAddr(addr)

	resp := NewStunMessageResponse(transId)
	resp.AddAttribute(xorAttr)
	resp.AddMessageIntegrity(passwd)
	resp.AddFingerprint()
	return resp.Write(buf)
}

// The packet length of dtls/rtp/rtcp
const (
	kDtlsRecordHeaderLen int = 13
	kMinRtcpPacketLen    int = 4
	kMinRtpPacketLen     int = 12
)

// IsDtlsPacket returns whether a given packet is a dtls.
func IsDtlsPacket(data []byte) bool {
	if len(data) < kDtlsRecordHeaderLen {
		return false
	}
	return (data[0] > 19 && data[0] < 64)
}

// IsRtcpPacket returns whether a given packet is a rtcp.
// If we're muxing RTP/RTCP, we must inspect each packet delivered and
// determine whether it is RTP or RTCP. We do so by checking the packet type,
// and assuming RTP if type is 0-63 or 96-127. For additional details, see
// http://tools.ietf.org/html/rfc5761.
// Note that if we offer RTCP mux, we may receive muxed RTCP before we
// receive the answer, so we operate in that state too.
func IsRtcpPacket(data []byte) bool {
	if len(data) < kMinRtcpPacketLen {
		return false
	}
	flag := (data[0] & 0xC0)
	utype := (data[1] & 0x7F)
	return (flag == 0x80 && utype >= 64 && utype < 96)
}

// IsRtpRtcpPacket returns whether a given packet is a rtp/rtcp.
func IsRtpRtcpPacket(data []byte) bool {
	if len(data) < kMinRtcpPacketLen {
		return false
	}
	return ((data[0] & 0xC0) == 0x80)
}

// IsRtpPacket returns whether a given packet is a rtp.
func IsRtpPacket(data []byte) bool {
	if len(data) < kMinRtpPacketLen {
		return false
	}
	return (IsRtpRtcpPacket(data) && !IsRtcpPacket(data))
}

// IsStunPacket returns whether a given packet is a stun request/response.
func IsStunPacket(data []byte) bool {
	if len(data) < kStunHeaderSize {
		return false
	}

	if int(data[0]) != 0 && int(data[0]) != 1 {
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
		//Warnln("[ice] check: ", magic, kStunMagicCookie)
		// If magic cookie is invalid, only support RFC5389, not including RFC3489
		return false
	}

	return true
}
