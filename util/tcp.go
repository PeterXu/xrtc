package util

import (
	"bytes"
	"errors"
	"io"
	"net"
)

var SslClientHello = []byte{
	0x80, 0x46, // msg len,MSB is 1 ,indicates a 2 byte header
	0x01,       // CLIENT_HELLO
	0x03, 0x01, // SSL 3.1
	0x00, 0x2d, // ciphersuite len
	0x00, 0x00, // session id len
	0x00, 0x10, // challenge len
	0x01, 0x00, 0x80, 0x03, 0x00, 0x80, 0x07, 0x00, 0xc0, // ciphersuites
	0x06, 0x00, 0x40, 0x02, 0x00, 0x80, 0x04, 0x00, 0x80, //
	0x00, 0x00, 0x04, 0x00, 0xfe, 0xff, 0x00, 0x00, 0x0a, //
	0x00, 0xfe, 0xfe, 0x00, 0x00, 0x09, 0x00, 0x00, 0x64, //
	0x00, 0x00, 0x62, 0x00, 0x00, 0x03, 0x00, 0x00, 0x06, //
	0x1f, 0x17, 0x0c, 0xa6, 0x2f, 0x00, 0x78, 0xfc, // challenge
	0x46, 0x55, 0x2e, 0xb1, 0x83, 0x39, 0xf1, 0xea, //
}

var SslServerHello = []byte{
	0x16,       // handshake message
	0x03, 0x01, // SSL 3.1
	0x00, 0x4a, // message len
	0x02,             // SERVER_HELLO
	0x00, 0x00, 0x46, // handshake len
	0x03, 0x01, // SSL 3.1
	0x42, 0x85, 0x45, 0xa7, 0x27, 0xa9, 0x5d, 0xa0, // server random
	0xb3, 0xc5, 0xe7, 0x53, 0xda, 0x48, 0x2b, 0x3f, //
	0xc6, 0x5a, 0xca, 0x89, 0xc1, 0x58, 0x52, 0xa1, //
	0x78, 0x3c, 0x5b, 0x17, 0x46, 0x00, 0x85, 0x3f, //
	0x20,                                           // session id len
	0x0e, 0xd3, 0x06, 0x72, 0x5b, 0x5b, 0x1b, 0x5f, // session id
	0x15, 0xac, 0x13, 0xf9, 0x88, 0x53, 0x9d, 0x9b, //
	0xe8, 0x3d, 0x7b, 0x0c, 0x30, 0x32, 0x6e, 0x38, //
	0x4d, 0xa2, 0x75, 0x57, 0x41, 0x6c, 0x34, 0x5c, //
	0x00, 0x04, // RSA/RC4-128/MD5
	0x00, // null compression
}

// webrtc tcp data format: len(2B) + data(..)
//  so max packet size is 64*1024.
const kMaxIceTcpPacketSize = 64 * 1024

func MaxIcePacketSize() int {
	return kMaxIceTcpPacketSize
}

func WriteIceTcpPacket(conn net.Conn, body []byte) (int, error) {
	if len(body) > kMaxIceTcpPacketSize {
		return 0, errors.New("Too much data for ice-tcp")
	}
	var buf bytes.Buffer
	WriteBig(&buf, uint16(len(body)))
	buf.Write(body)
	return conn.Write(buf.Bytes())
}

func ReadIceTcpPacket(conn net.Conn, body []byte) (int, error) {
	if len(body) < kMaxIceTcpPacketSize {
		return 0, errors.New("No enough size in buf for ice-tcp")
	}

	// read head(2bytes)
	head := make([]byte, 2)
	nret, err := conn.Read(head[0:2])
	if err != nil {
		nret = -1
	}
	if nret != 2 {
		LogWarnln("ice-tcp read head fail, err=", err, nret)
		return 0, errors.New("ice-tcp read head fail(2bytes)")
	}

	// get body size
	dsize := HostToNet16(BytesToUint16(head))
	if dsize == 0 {
		LogWarnln("ice-tcp empty body")
		return 0, nil
	}
	//LogPrintln("ice-tcp body size:", dsize)

	// read body packet
	rpos := 0
	for rpos < int(dsize) {
		need := int(dsize) - rpos
		if nret, err = conn.Read(body[rpos : rpos+need]); err == nil {
			rpos += nret
		} else {
			LogWarnln("ice-tcp read body err:", err)
			if err == io.EOF {
				break // end
			}
			if nerr := err.(*net.OpError); nerr != nil {
				if nerr.Timeout() || nerr.Temporary() {
					LogWarnln("ice-tcp read clear error:", nerr)
					err = nil // clear
				}
			}
			if err != nil {
				break
			}
		}
	}

	return rpos, err
}
