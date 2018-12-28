package webrtc

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"unsafe"

	log "github.com/Sirupsen/logrus"
)

// These are the types of STUN messages defined in RFC 5389.
type StunMessageType uint16

const (
	STUN_BINDING_REQUEST        StunMessageType = 0x0001
	STUN_BINDING_INDICATION                     = 0x0011
	STUN_BINDING_RESPONSE                       = 0x0101
	STUN_BINDING_ERROR_RESPONSE                 = 0x0111
)

// These are all known STUN attributes, defined in RFC 5389 and elsewhere.
// Next to each is the name of the class (T is StunTAttribute) that implements
// that type.
// RETRANSMIT_COUNT is the number of outstanding pings without a response at
// the time the packet is generated.
type StunAttributeType uint16

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

// These are the types of the values associated with the attributes above.
// This allows us to perform some basic validation when reading or adding
// attributes. Note that these values are for our own use, and not defined in
// RFC 5389.
type StunAttributeValueType int

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

// These are the types of STUN addresses defined in RFC 5389.
type StunAddressFamily uint8

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

// Records a complete STUN/TURN message. Each message consists of a type and
// any number of attributes. Each attribute is parsed into an instance of an
// appropriate class (see above).  The Get* methods will return instances of
// that attribute class.
type StunMessage struct {
	dtype   StunMessageType
	length  uint16
	magic   uint32
	transId string
	attrs   map[StunAttributeType]StunAttribute
}

type IceMessage struct {
	StunMessage
}

func (m *StunMessage) Read(data []byte) bool {
	if len(data) < kStunHeaderSize {
		log.Warnln("[ice] invalid stun size=", len(data))
		return false
	}

	// check the 1st byte
	utype := data[0]
	if utype != 0 && utype != 1 {
		log.Warnln("[ice] invalid utype=", utype)
		return false
	}

	// create Reader
	buf := bytes.NewReader(data)

	// 0-2, read stun message type
	ReadBig(buf, &m.dtype)
	//log.Println("[ice] message type=", m.dtype)
	if (m.dtype & 0x8000) != 0 {
		// RTP and RTCP
		log.Warnln("[ice] not stun message, (RTP/RTCP)type=", m.dtype)
		return false
	}

	// 2-4, read stun message size
	ReadBig(buf, &m.length)
	//log.Println("[ice] message length=", m.length)
	if (m.length & 0x0003) != 0 {
		log.Warnln("[ice] invalid message length=", m.length)
		return false
	}

	// 4-8, read stun magic
	ReadBig(buf, &m.magic)

	// 8-20, read stun transaction id
	var transId [kStunTransactionIdLength]byte
	if ret, err := buf.Read(transId[:]); err != nil || ret != kStunTransactionIdLength {
		log.Println("[ice] invalid transid ret=", ret, ", err=", err)
		return false
	}

	if m.magic != kStunMagicCookie {
		// If magic cookie is invalid it means that the peer implements
		// RFC3489 instead of RFC5389.
		m.transId = string(ValueToBytes(m.magic)[:]) + string(transId[:])
	} else {
		m.transId = string(transId[:])
	}
	//log.Printf("[ice] message magic=%x, transId=%s\n", m.magic, m.transId)

	if int(m.length) != buf.Len() {
		log.Println("[ice] invalid length=", m.length, ", Len=", buf.Len())
		return false
	}

	if m.attrs == nil {
		m.attrs = make(map[StunAttributeType]StunAttribute)
	}

	for {
		if buf.Len() < 4 {
			//log.Println("[ice] no more data len=", buf.Len())
			break
		}

		var attrType StunAttributeType
		var attrLen uint16
		ReadBig(buf, &attrType)
		ReadBig(buf, &attrLen)
		//log.Println("[ice] attrType, attrLen=", attrType, attrLen)

		var attr StunAttribute
		switch attrType {
		case STUN_ATTR_MAPPED_ADDRESS:
			attr = &StunAddressAttribute{}
		case STUN_ATTR_XOR_MAPPED_ADDRESS:
			attr = &StunXorAddressAttribute{}
		case STUN_ATTR_USERNAME:
			attr = &StunByteStringAttribute{}
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
			attr.SetInfo(attrType, attrLen, m.transId)
			attr.Read(buf)
			m.attrs[attrType] = attr
		}
	}

	return true
}

func (m *StunMessage) Write(buf *bytes.Buffer) bool {
	// 0-2, stun type
	WriteBig(buf, m.dtype)
	// 2-4, stun body length
	WriteBig(buf, m.length)
	if !m.IsLegacy() {
		// 4-8, stun magic
		WriteBig(buf, kStunMagicCookie)
	}
	// 8-20, stun transId
	buf.WriteString(m.transId)
	// head: 2+2+[4]+12
	//log.Println("[ice] write stun headLen=", buf.Len(), ", bodyLen=", m.length)

	// m.length: stun body
	// STUN_ATTR_USERNAME: 2+2+username
	// STUN_ATTR_MESSAGE_INTEGRITY: 2+2+20
	// STUN_ATTR_FINGERPRINT: 2+2+4
	for _, attr := range m.attrs {
		// 2bytes attr type
		WriteBig(buf, attr.GetType())
		// 2bytes attr len
		WriteBig(buf, attr.GetLen2())
		//log.Println("[ice] write, attrType, attrLen=", attr.GetType(), attr.GetLen2())
		if !attr.Write(buf) {
			log.Println("[ice] fail to write buf from stunmessage")
			return false
		}
	}
	return true
}

func (m *StunMessage) IsLegacy() bool {
	if len(m.transId) == kStunLegacyTransactionIdLength {
		return true
	}
	// kStunTransactionIdLength
	return false
}

func (m *StunMessage) SetType(dtype StunMessageType) {
	m.dtype = dtype
}

func (m *StunMessage) SetTransactionID(transId string) bool {
	if !m.IsValidTransactionId(transId) {
		return false
	}
	m.transId = transId
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
	if m.attrs != nil {
		if attr, ok := m.attrs[atype]; ok {
			return attr
		}
	}
	return nil
}

func (m *StunMessage) AddAttribute(attr StunAttribute) {
	if m.attrs == nil {
		m.attrs = make(map[StunAttributeType]StunAttribute)
	}
	m.attrs[attr.GetType()] = attr
	attr_length := attr.GetLen2()
	if (attr_length % 4) != 0 {
		attr_length += (4 - (attr_length % 4))
	}
	m.length += (attr_length + 4)
}

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
		log.Println("[ice] hmac buf is null")
		return false
	}

	attrLen := int(integrityAttr.GetLen2())
	msg_len_for_hmac := buf.Len() - kStunAttributeHeaderSize - attrLen
	macFunc := hmac.New(sha1.New, []byte(key))
	macFunc.Write(buf.Bytes()[0:msg_len_for_hmac])
	digest := macFunc.Sum(nil)
	//log.Printf("[ice] hmac bufLen=%d, attrLen=%d, msglen=%d, digestlen=%d\n",
	//	buf.Len(), attrLen, msg_len_for_hmac, len(digest))
	if len(digest) != kStunMessageIntegritySize {
		log.Println("[ice] hmac digest wrong, len=", len(digest))
		return false
	}
	//log.Println("[ice] hmac digest, len=", len(digest))
	integrityAttr.CopyBytes(digest)

	return true
}

func (m *StunMessage) AddFingerprint() bool {
	fingerprinAttr := &StunUInt32Attribute{}
	fingerprinAttr.SetType(STUN_ATTR_FINGERPRINT)
	m.AddAttribute(fingerprinAttr)

	var buf bytes.Buffer
	if !m.Write(&buf) {
		log.Println("[ice] fail to AddFingerprint")
		return false
	}

	attrLen := int(fingerprinAttr.GetLen2())
	msg_len_for_crc32 := buf.Len() - kStunAttributeHeaderSize - attrLen
	const kCrc32Polynomial uint32 = 0xEDB88320
	crc32q := crc32.MakeTable(kCrc32Polynomial)
	crc := crc32.Checksum(buf.Bytes()[0:msg_len_for_crc32], crc32q)
	//log.Printf("[ice] fp, buflen=%d, attrLen=%d, msglen=%d, crc=%d\n",
	//	buf.Len(), attrLen, msg_len_for_crc32, crc)
	fingerprinAttr.SetValue(crc ^ STUN_FINGERPRINT_XOR_VALUE)
	return true
}

/// StunAttribute
type StunAttribute interface {
	Read(buf *bytes.Reader) bool
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
		log.Println("[ice] no enough data, length=", buf.Len(), ", require len=", a.attrLen)
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
			log.Println("[ice] read ipv4 failed")
			return false
		}
	} else if a.family == STUN_ADDRESS_IPV6 {
		a.ip = make([]byte, 20)
		if ReadBig(buf, a.ip) != nil {
			log.Println("[ice] read ipv6 failed")
			return false
		}
	}

	return true
}

func (a *StunAddressAttribute) Write(buf *bytes.Buffer) bool {
	if a.family == STUN_ADDRESS_UNDEF {
		log.Println("[ice] Error writing address attribute: unknown family.")
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
		//log.Println("[ice] set addr:", strAddr)
		a.SetIP(net.ParseIP(host))
		a.SetPort(Atou16(port))
	} else {
		log.Println("[ice] fail to set addr:", strAddr)
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

type StunXorAddressAttribute struct {
	StunAttributeBase
	addr    StunAddressAttribute
	xorIP   net.IP
	xorPort uint16
}

func (a *StunXorAddressAttribute) GetLen2() uint16 {
	if a.addr.family == STUN_ADDRESS_IPV4 {
		return 1 + 1 + 2 + 4
	} else {
		return 1 + 1 + 2 + 20
	}
}

func (a *StunXorAddressAttribute) Read(buf *bytes.Reader) bool {
	if !a.addr.Read(buf) {
		return false
	}
	a.xorPort = a.addr.port ^ uint16(kStunMagicCookie>>16)
	a.GetXoredIP()
	return true
}

func (a *StunXorAddressAttribute) Write(buf *bytes.Buffer) bool {
	if a.addr.family == STUN_ADDRESS_UNDEF {
		log.Println("[ice] invalid addr family in xoraddr")
		return false
	}
	var zero uint8 = 0
	WriteBig(buf, zero)
	WriteBig(buf, a.addr.family)

	a.xorPort = a.addr.port ^ uint16(kStunMagicCookie>>16)
	a.GetXoredIP()
	WriteBig(buf, a.xorPort)
	WriteBig(buf, a.xorIP)
	return true
}

func (a *StunXorAddressAttribute) GetXoredIP() {
	magic := HostToNet32(kStunMagicCookie)
	if a.addr.family == STUN_ADDRESS_IPV4 {
		a.xorIP = make([]byte, 4)
		dst := (*[1]uint32)(unsafe.Pointer(&a.xorIP[0]))[:]
		ipv4 := (*[1]uint32)(unsafe.Pointer(&a.addr.ip[0]))[:]
		dst[0] = ipv4[0] ^ magic
	} else if a.addr.family == STUN_ADDRESS_IPV6 {
		a.xorIP = make([]byte, 20)
		if len(a.addr.transId) == kStunTransactionIdLength {
			var transIds [3]uint32
			copy((*[12]byte)(unsafe.Pointer(&transIds[0]))[:], a.transId)

			dst := (*[4]uint32)(unsafe.Pointer(&a.xorIP[0]))[:]
			ipv6 := (*[4]uint32)(unsafe.Pointer(&a.addr.ip[0]))[:]
			dst[0] = ipv6[0] ^ magic
			for i := 1; i < 4; i++ {
				dst[i] = ipv6[i] ^ transIds[i-1]
			}
		}
	}
}

type StunByteStringAttribute struct {
	StunAttributeBase
	data []byte
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
	return uint16(len(a.data))
}

func (a *StunByteStringAttribute) Read(buf *bytes.Reader) bool {
	if !a.Check(buf) {
		log.Println("[ice] invalid buf for StunByteStringAttribute")
		return false
	}

	a.data = make([]byte, a.attrLen)
	if _, err := buf.Read(a.data); err != nil {
		a.data = nil
		log.Println("[ice] fail to read for StunByteStringAttribute")
		return false
	}
	a.ConsumePadding(buf, int(a.attrLen))
	return true
}

func (a *StunByteStringAttribute) Write(buf *bytes.Buffer) bool {
	buf.Write(a.data)
	a.WritePadding(buf, len(a.data))
	return true
}

func (a *StunByteStringAttribute) CopyBytes(data []byte) {
	if len(a.data) != len(data) {
		a.data = make([]byte, len(data))
	}
	copy(a.data[0:], data)
}

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
