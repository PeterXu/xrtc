package util

import (
	"encoding/binary"
)

/*
 *  0                   1                   2                   3
 *  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 * |V=2|P|X|  CC   |M|     PT      |       sequence number         |
 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 * |                           timestamp                           |
 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 * |           synchronization source (SSRC) identifier            |
 * +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
 * |            contributing source (CSRC) identifiers             |
 * |                             ....                              |
 * +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 */

/// rtp basic structures

const (
	kRtpVersion      = 2
	kRtpHeaderLength = 12

	// the first byte
	kRtpVersionShift   = 6
	kRtpVersionMask    = 0x3
	kRtpPaddingShift   = 5
	kRtpPaddingMask    = 0x1
	kRtpExtensionShift = 4
	kRtpExtensionMask  = 0x1
	kRtpCcMask         = 0xF

	// the second byte
	kRtpMarkerShift = 7
	kRtpMarkerMask  = 0x1
	kRtpPtMask      = 0x7F

	// the 3th - 12th bytes
	kRtpSeqNumOffset    = 2
	kRtpTimestampOffset = 4
	kRtpSsrcOffset      = 8
	kRtpCsrcOffset      = 12
)

/// get value of some rtp field

func GetRtpPayloadType(buf []byte) uint8 {
	if len(buf) < kRtpHeaderLength {
		return 0
	}
	return buf[1] & kRtpPtMask
}

func GetRtpSeqNum(buf []byte) uint16 {
	if len(buf) < kRtpHeaderLength {
		return 0
	}
	offset := kRtpSeqNumOffset
	return binary.BigEndian.Uint16(buf[offset : offset+2])
}

func GetRtpTimestamp(buf []byte) uint32 {
	if len(buf) < kRtpHeaderLength {
		return 0
	}
	offset := kRtpTimestampOffset
	return binary.BigEndian.Uint32(buf[offset : offset+4])
}

func GetRtpSsrc(buf []byte) uint32 {
	if len(buf) < kRtpHeaderLength {
		return 0
	}
	offset := kRtpSsrcOffset
	return binary.BigEndian.Uint32(buf[offset : offset+4])
}

/// set value of some rtp field

func SetRtpPayloadType(buf []byte, payloadType uint8) bool {
	if len(buf) < kRtpHeaderLength {
		return false
	}
	marker := (buf[1] >> kRtpMarkerShift & kRtpMarkerMask) > 0
	buf[1] = payloadType
	if marker {
		buf[1] |= 1 << kRtpMarkerShift
	}
	return true
}

func SetRtpSeqNum(buf []byte, seqNum uint16) bool {
	if len(buf) < kRtpHeaderLength {
		return false
	}
	offset := kRtpSeqNumOffset
	binary.BigEndian.PutUint16(buf[offset:offset+2], seqNum)
	return true
}

func SetRtpTimestamp(buf []byte, timestamp uint32) bool {
	if len(buf) < kRtpHeaderLength {
		return false
	}
	offset := kRtpTimestampOffset
	binary.BigEndian.PutUint32(buf[offset:offset+4], timestamp)
	return true
}

func SetRtpSsrc(buf []byte, ssrc uint32) bool {
	if len(buf) < kRtpHeaderLength {
		return false
	}
	offset := kRtpSsrcOffset
	binary.BigEndian.PutUint32(buf[offset:offset+4], ssrc)
	return true
}

/// rtp seq comparing

// return true if RTP-SEQ(uint16) seqn between (start, start+size).
func ParseRtpSeqInRange(seqn, start, size uint16) bool {
	var n int = int(seqn)
	var nh int = ((1 << 16) + n)
	var s int = int(start)
	var e int = s + int(size)
	return (s <= n && n < e) || (s <= nh && nh < e)
}

// Equal or Newer(seq >= prevSeq)
func IsNewerRtpSeq(seq, prevSeq uint16) bool {
	diff := uint16(seq - prevSeq)
	return diff == 0 || diff < 0x8000
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

// return rtp seq distance
func ComputeRtpSeqDistance(seq1, seq2 uint16) int {
	diff := uint16(seq1 - seq2)
	if diff <= 0x8000 {
		return int(diff)
	} else {
		return 65536 - int(diff)
	}
}
