package webrtc

import (
	"encoding/binary"
	"errors"
)

const (
	WebRTCControlPPID       = 50
	WebRTCStringPPID        = 51
	WebRTCBinaryPartialPPID = 52
	WebRTCBinaryPPID        = 53
	WebRTCStringPartialPPID = 54
	WebRTCStringEmptyPPID   = 56
	WebRTCBinaryEmptyPPID   = 57

	DcepRequest = 0x03
	DcepAck     = 0x02

	ChannelReliable        = 0x00
	ChannelUnordered       = 0x80
	ChannelRexmit          = 0x01
	ChannelUnorderedRexmit = 0x81
	ChannelTimed           = 0x02
	ChannelUnorderedTimed  = 0x82

	PriorityBelowNormal = 128
	PriorityNormal      = 256
	PriorityHigh        = 512
	PriorityExtraHigh   = 1024
)

var (
	errNotEnoughData = errors.New("not enough data")
)

type DcepRequestMessage struct {
	MessageType uint8
	ChannelType uint8
	Priority    uint16
	Reliability uint32
	Label       string
	Protocol    string
}

func (d *DcepRequestMessage) Encode() []byte {
	length := 12 + len(d.Label) + len(d.Protocol)
	req := make([]byte, length)
	req[0] = d.MessageType
	req[1] = d.ChannelType
	binary.BigEndian.PutUint16(req[2:4], d.Priority)
	binary.BigEndian.PutUint32(req[4:8], d.Reliability)
	binary.BigEndian.PutUint16(req[8:10], (uint16)(len(d.Label)))
	binary.BigEndian.PutUint16(req[10:12], (uint16)(len(d.Protocol)))
	if len(d.Label) > 0 {
		copy(req[12:12+len(d.Label)], []byte(d.Label))
	}
	if len(d.Protocol) > 0 {
		base := 12 + len(d.Label)
		copy(req[base:base+len(d.Protocol)], []byte(d.Protocol))
	}
	return req
}

func (d *DcepRequestMessage) Decode(data []byte) error {
	if len(data) < 12 {
		return errNotEnoughData
	}
	d.MessageType = data[0]
	d.ChannelType = data[1]
	d.Priority = binary.BigEndian.Uint16(data[2:4])
	d.Reliability = binary.BigEndian.Uint32(data[4:8])
	labelLength := binary.BigEndian.Uint16(data[8:10])
	protocolLength := binary.BigEndian.Uint16(data[10:12])
	if len(data) < int(12+labelLength+protocolLength) {
		return errNotEnoughData
	}
	if labelLength > 0 {
		d.Label = string(data[12 : 12+labelLength])
	}
	if protocolLength > 0 {
		d.Protocol = string(data[12+labelLength : 12+labelLength+protocolLength])
	}
	return nil
}

type DcepAckMessage struct {
	MessageType uint8
}

func (d *DcepAckMessage) Encode() []byte {
	ack := make([]byte, 1)
	ack[0] = d.MessageType
	return ack
}

func (d *DcepAckMessage) Decode(data []byte) error {
	if len(data) < 1 {
		return errNotEnoughData
	}
	d.MessageType = data[0]
	return nil
}
