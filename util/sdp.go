package util

import (
	"strings"
)

const kSdpOwner string = "xrtc"
const kSdpCname string = "xrtc_endpoint"

var kSdpNewlineChar = []byte{'\n'}
var kSdpSpaceChar = []byte{' '}

// m=audio/video/application
type SdpMediaType int

// These are different media types
const (
	kSdpMediaNil   SdpMediaType = 0
	kSdpMediaAudio SdpMediaType = 1 << (iota - 1)
	kSdpMediaVideo
	kSdpMediaApplication
)

// SSRC pair: main/rtx
// a=ssrc
// a=fmtp:. apt=rtx
type SdpSsrc struct {
	main uint32
	rtx  uint32
}

// Media Direction
type SdpMediaDirection int

// These are different sdp directions: sendrecv/sendonly/recvonly/inactive
const (
	kSdpDirectionInactive SdpMediaDirection = iota
	kSdpDirectionSendOnly
	kSdpDirectionRecvOnly
	kSdpDirectionSendRecv
)

// SDP a=msid-semantic
// a=msid-semantic:WMS *
// a=msid-semantic:WMS id1
type SdpMsidSemantic struct {
	name  string
	msids []string
}

// SDP a=rtpmap
// a=rtpmap:111 opus/48000/2
// a=rtpmap:126 H264/90000
type SdpRtpMapInfo struct {
	ptype     int
	codec     string
	frequency int
	channels  int
	misc      string
	apt_ptype int
}

func (r *SdpRtpMapInfo) Clone() *SdpRtpMapInfo {
	d := &SdpRtpMapInfo{}
	d.ptype = r.ptype
	d.codec = r.codec
	d.frequency = r.frequency
	d.channels = r.channels
	d.misc = r.misc
	d.apt_ptype = r.apt_ptype
	return d
}

func (r SdpRtpMapInfo) a_rtpmap() string {
	return Itoa(r.ptype) + " " + r.codec + "/" + Itoa(r.frequency)
}

func (r SdpRtpMapInfo) a_rtxmap() string {
	return Itoa(r.apt_ptype) + " rtx/90000"
}

func (r SdpRtpMapInfo) a_fmtp_apt() string {
	return Itoa(r.apt_ptype) + " apt=" + Itoa(r.ptype)
}

// NewSdpFmtpInfo return a SdpFmtpInfo object
// a=fmtp:111 maxplaybackrate=48000;stereo=1;useinbandfec=1
// a=fmtp:126 profile-level-id=42e01f;level-asymmetry-allowed=1;packetization-mode=1
// a=fmtp:101 0-15
func NewSdpFmtpInfo(ptype int) *SdpFmtpInfo {
	return &SdpFmtpInfo{ptype: ptype, props: make(map[string]int)}
}

// SDP media format-specific parameters: a=fmtp
type SdpFmtpInfo struct {
	ptype int
	props map[string]int
	misc  string
}

// SDP rtcp feedback: a=rtcp-fb
// a=rtcp-fb:126 nack
// a=rtcp-fb:126 nack pli
type SdpRtcpFbInfo struct {
	ptype  int
	fbtype string
}

// SDP rtp-ext-header: a=extmap
// a=extmap:1 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time
// a=extmap:2/sendrecv urn:ietf:params:rtp-hdrext:toffset
type SdpExtMapInfo struct {
	id        int
	direction string
	uri       string
}

// SDP ssrc: a=ssrc
// a=ssrc:1081040086 cname:{name}
// a=ssrc:1081040086 cname:name
// a=ssrc:1081040086 msid:id1 id2
// a=ssrc:1081040086 mslabel:id1
// a=ssrc:1081040086 label:id2
type SdpSsrcInfo struct {
	ssrc    uint32
	cname   string
	msids   []string
	label   string
	mslabel string
}

// SDP a=ssrc-group:FID
// a=ssrc-group:FID 1081040086 1081040087
type SdpFidInfo struct {
	main uint32
	rtx  uint32
}

// SDP sctp: a=sctpmap
// a=sctpmap:5000 webrtc-datachannel 1024
type SdpSctpInfo struct {
	port       int
	name       string
	number     int
	is_sctpmap bool
}

// SDP ice: a=ice-..
type SdpIceAttr struct {
	Ufrag   string // a=ice-ufrag:..
	Pwd     string // a=ice-pwd:..
	Options string // a=ice-options:..
}

// SDP m=
type SdpMLine struct {
	mtype  string   // m=
	proto  string   // m=
	ptypes []string // m=
}

func NewSdpMediaAttr(mtype, proto string) *SdpMediaAttr {
	attr := &SdpMediaAttr{
		fmtps: make(map[int]*SdpFmtpInfo),
	}
	attr.mtype = mtype
	attr.proto = proto
	attr.av_rtpmaps = make(map[string]*SdpRtpMapInfo)
	return attr
}

// SDP media attribute lines
type SdpMediaAttr struct {
	SdpMLine                              // m=..
	ice_attr         SdpIceAttr           // a=ice-
	fingerprint      StringPair           // a=fingerprint:sha-256 ..
	setup            string               // a=setup:..
	direction        SdpMediaDirection    // a=sendrecv/sendonly/recvonly
	mid              string               // a=mid:..
	msid             []*StringPair        // a=msid:{id1} {id2}
	rtcp_mux         bool                 // a=rtcp-mux
	rtcp_rsize       bool                 // a=rtcp-rsize
	rtpmaps          []*SdpRtpMapInfo     // a=rtpmap:..
	fmtps            map[int]*SdpFmtpInfo // a=fmtp:..
	rtcp_fbs         []*SdpRtcpFbInfo     // a=rtcp-fb:..
	extmaps          []*SdpExtMapInfo     // a=extmap:..
	fid_ssrcs        []*SdpFidInfo        // a=ssrc-group:FID ..
	ssrcs            []*SdpSsrcInfo       // a=ssrc:..
	msids            []string             // a=msid:..
	sctp             *SdpSctpInfo         // a=sctpmap: or a=sctp-port:
	max_message_size int                  // a=max-message-size:
	candidates       []string             // a=candidate:
	maxptime         int                  // a=maxptime:

	SdpMediaAnswer
}

// for anwser
type SdpMediaAnswer struct {
	av_rtpmaps  map[string]*SdpRtpMapInfo
	av_ice_attr SdpIceAttr
	use_rtx     bool
	use_red_fec bool
	use_red_rtx bool
	use_fid     bool
}

func (a *SdpMediaAttr) GetSsrcs() *SdpSsrc {
	ssrc := &SdpSsrc{}
	if len(a.fid_ssrcs) > 0 {
		ssrc.main = a.fid_ssrcs[0].main
		ssrc.rtx = a.fid_ssrcs[0].rtx
	} else if len(a.ssrcs) > 0 {
		ssrc.main = a.ssrcs[0].ssrc
	} else {
		ssrc = nil
	}
	return ssrc
}

// SDP media lines
type SdpAll struct {
	owner         string          // o=..
	source        string          // s=..
	ice_lite      bool            // a=ice-lite
	ice_options   string          // global a=ice-options:..
	fingerprint   StringPair      // global a=fingerprint:sha-256 ..
	group_bundles []string        // a=group:BUNDLE ..
	msid_semantic SdpMsidSemantic // a=msid-sematic: ..
	audios        []*SdpMediaAttr // m=audio ..
	videos        []*SdpMediaAttr // m=video ..
	applications  []*SdpMediaAttr // m=application ..
}

// parseSdp to parse SDP lines, return true if ok
func (m *SdpAll) parseSdp(data []byte) bool {
	var mattr *SdpMediaAttr
	lines := strings.Split(string(data), "\r\n")
	if len(lines) <= 1 {
		lines = strings.Split(string(data), "\n")
	}

	//LogPrintln("[sdp] parseSdp, lines=", len(lines))
	for item := range lines {
		line := []byte(lines[item])
		if len(line) <= 2 || line[1] != '=' {
			//LogWarnln("invalid sdp line: ", string(line))
			continue
		}

		switch line[0] {
		case 'v':
			// nop
		case 'o':
			fields := strings.SplitN(string(line[2:]), " ", 2)
			if len(fields) == 2 {
				attrs := strings.SplitN(fields[1], " ", 2)
				m.owner = attrs[0]
			}
		case 's':
			// nop
		case 't':
			// nop
		case 'm':
			fields := strings.Split(string(line[2:]), " ")
			if len(fields) >= 4 {
				mattr = NewSdpMediaAttr(fields[0], fields[2])
				mattr.ptypes = append(mattr.ptypes, fields[3:]...)
			} else {
				mattr = NewSdpMediaAttr(fields[0], "")
			}
			if fields[0] == "audio" {
				m.audios = append(m.audios, mattr)
			} else if fields[0] == "video" {
				m.videos = append(m.videos, mattr)
			} else if fields[0] == "application" {
				m.applications = append(m.applications, mattr)
			} else {
				break
			}
		case 'c':
			// nop
		case 'a':
			m.parseSdp_a(line, mattr)
		default:
		}
	}
	return true
}

// parseSdp_a to parse SDP attribute: 'a='
func (m *SdpAll) parseSdp_a(line []byte, media *SdpMediaAttr) {
	fields := strings.SplitN(string(line[2:]), ":", 2)
	akey := fields[0]
	if len(fields) == 1 {
		if akey == "ice-lite" {
			m.ice_lite = true
			return
		}

		if media == nil {
			LogWarnln("[sdp] no valid media for line=", string(line[:]))
			return
		}

		if akey == "inactive" {
			media.direction = kSdpDirectionInactive
		} else if akey == "sendonly" {
			media.direction = kSdpDirectionSendOnly
		} else if akey == "recvonly" {
			media.direction = kSdpDirectionRecvOnly
		} else if akey == "sendrecv" {
			media.direction = kSdpDirectionSendRecv
		} else if akey == "rtcp-mux" {
			media.rtcp_mux = true
		} else if akey == "rtcp-rsize" {
			media.rtcp_rsize = true
		}
		return
	}

	if akey == "group" {
		attrs := strings.Split(fields[1], " ")
		//LogPrintln("[sdp] a=group:", attrs, len(attrs))
		if len(attrs) >= 1 {
			aval := strings.ToLower(attrs[0])
			switch aval {
			case "bundle":
				if len(attrs) >= 2 {
					m.group_bundles = append(m.group_bundles, attrs[1:]...)
				}
			default:
				LogWarnln("[sdp] unsupported attr - a=group:", aval)
			}
		}
		return
	}

	if akey == "msid-semantic" {
		attrs := strings.Split(fields[1], " ")
		if len(attrs) >= 1 {
			m.msid_semantic.name = attrs[0]
		}
		if len(attrs) >= 2 {
			props := attrs[1:]
			m.msid_semantic.msids = append(m.msid_semantic.msids, props...)
		}
		return
	}

	if media == nil {
		if akey == "ice-options" {
			m.ice_options = fields[1]
			return
		} else if akey == "fingerprint" {
			attrs := strings.SplitN(fields[1], " ", 2)
			if len(attrs) == 2 {
				m.fingerprint.First = attrs[0]
				m.fingerprint.Second = attrs[1]
			}
			return
		}

		LogWarnln("[sdp] no valid media for line=", string(line[:]))
		return
	}

	if akey == "rtcp" {
		// nop
	} else if akey == "ice-ufrag" {
		media.ice_attr.Ufrag = strings.TrimSpace(fields[1])
	} else if akey == "ice-pwd" {
		media.ice_attr.Pwd = strings.TrimSpace(fields[1])
	} else if akey == "ice-options" {
		media.ice_attr.Options = fields[1]
	} else if akey == "fingerprint" {
		attrs := strings.SplitN(fields[1], " ", 2)
		if len(attrs) == 2 {
			media.fingerprint.First = attrs[0]
			media.fingerprint.Second = attrs[1]
		}
	} else if akey == "setup" {
		media.setup = fields[1]
	} else if akey == "mid" {
		media.mid = fields[1]
	} else if akey == "rtpmap" {
		attrs := strings.SplitN(fields[1], " ", 2)
		if len(attrs) == 2 {
			rmap := &SdpRtpMapInfo{ptype: Atoi(attrs[0])}
			props := strings.Split(attrs[1], "/")
			if len(props) >= 2 {
				rmap.codec = props[0]
				rmap.frequency = Atoi(props[1])
				if len(props) >= 3 {
					rmap.channels = Atoi(props[2])
				}
			} else {
				rmap.misc = attrs[1]
			}
			media.rtpmaps = append(media.rtpmaps, rmap)
		}
	} else if akey == "fmtp" {
		attrs := strings.SplitN(fields[1], " ", 2)
		if len(attrs) == 2 {
			fmtp := NewSdpFmtpInfo(Atoi(attrs[0]))
			props := strings.Split(attrs[1], ";")
			for k := range props {
				kv := strings.Split(props[k], "=")
				if len(kv) == 2 {
					fmtp.props[kv[0]] = Atoi(kv[1])
				} else {
					fmtp.misc = props[k]
				}
			}
			media.fmtps[fmtp.ptype] = fmtp
		}
	} else if akey == "rtcp-fb" {
		attrs := strings.SplitN(fields[1], " ", 2)
		if len(attrs) == 2 {
			rtcpfb := &SdpRtcpFbInfo{Atoi(attrs[0]), attrs[1]}
			media.rtcp_fbs = append(media.rtcp_fbs, rtcpfb)
		}
	} else if akey == "extmap" {
		attrs := strings.SplitN(fields[1], " ", 2)
		if len(attrs) == 2 {
			extmap := &SdpExtMapInfo{id: Atoi(attrs[0]), uri: attrs[1]}
			keys := strings.Split(attrs[0], "/")
			if len(keys) >= 2 {
				extmap.direction = keys[1]
			}
			media.extmaps = append(media.extmaps, extmap)
		}
	} else if akey == "ssrc-group" {
		attrs := strings.SplitN(fields[1], " ", 2)
		if len(attrs) == 2 {
			if attrs[0] == "FID" {
				props := strings.Split(attrs[1], " ")
				if len(props) == 2 {
					fid := &SdpFidInfo{Atou32(props[0]), Atou32(props[1])}
					media.fid_ssrcs = append(media.fid_ssrcs, fid)
				}
			} else if attrs[0] == "SIM" {
				// not support
			}
		}
	} else if akey == "ssrc" {
		attrs := strings.SplitN(fields[1], " ", 2)
		if len(attrs) == 2 {
			ssrc := &SdpSsrcInfo{ssrc: Atou32(attrs[0])}
			props := strings.SplitN(attrs[1], ":", 2)
			if len(props) == 2 {
				if props[0] == "cname" {
					ssrc.cname = strings.Trim(props[0], "{}")
				} else if props[0] == "msid" {
					msids := strings.Split(props[1], " ")
					ssrc.msids = append(ssrc.msids, msids...)
				} else if props[0] == "mslabel" {
					ssrc.mslabel = props[1]
				} else if props[0] == "label" {
					ssrc.label = props[1]
				}
			}
			media.ssrcs = append(media.ssrcs, ssrc)
		}
	} else if akey == "msid" {
		msids := strings.Split(fields[1], " ")
		if len(msids) > 0 {
			media.msids = append(media.msids, msids...)
		}
	} else if akey == "sctpmap" {
		attrs := strings.Split(fields[1], " ")
		if len(attrs) >= 3 {
			media.sctp = &SdpSctpInfo{Atoi(attrs[0]), attrs[1], Atoi(attrs[2]), true}

		}
	} else if akey == "sctp-port" {
		media.sctp = &SdpSctpInfo{port: Atoi(fields[1])}
	} else if akey == "max-message-size" {
		media.max_message_size = Atoi(fields[1])
	} else if akey == "candidate" {
		media.candidates = append(media.candidates, string(line))
	} else if akey == "maxptime" {
		media.maxptime = Atoi(fields[1])
	} else {
		LogWarnln("[sdp] unsupported attr=", akey)
	}
}

// Media description (sdp offer/answer)
type SdpDesc struct {
	Sdp        SdpAll
	haveAnswer bool
}

func (m *SdpDesc) Parse(data []byte) bool {
	return m.Sdp.parseSdp(data)
}

func (m *SdpDesc) GetMediaType() SdpMediaType {
	mt := kSdpMediaNil
	if len(m.Sdp.audios) > 0 {
		mt |= kSdpMediaAudio
	}
	if len(m.Sdp.videos) > 0 {
		mt |= kSdpMediaVideo
	}
	if len(m.Sdp.applications) > 0 {
		mt |= kSdpMediaApplication
	}
	return mt
}

func (m *SdpDesc) GetUfrag() string {
	mt := m.GetMediaType()
	if (mt & kSdpMediaAudio) != 0 {
		return m.Sdp.audios[0].ice_attr.Ufrag
	} else if (mt & kSdpMediaVideo) != 0 {
		return m.Sdp.videos[0].ice_attr.Ufrag
	} else if (mt & kSdpMediaApplication) != 0 {
		return m.Sdp.applications[0].ice_attr.Ufrag
	} else {
		LogWarnln("[desc] invalid media type = ", mt)
		return ""
	}
}

func (m *SdpDesc) GetPwd() string {
	mt := m.GetMediaType()
	if (mt & kSdpMediaAudio) != 0 {
		return m.Sdp.audios[0].ice_attr.Pwd
	} else if (mt & kSdpMediaVideo) != 0 {
		return m.Sdp.videos[0].ice_attr.Pwd
	} else if (mt & kSdpMediaApplication) != 0 {
		return m.Sdp.applications[0].ice_attr.Pwd
	} else {
		LogWarnln("[desc] invalid media type = ", mt)
		return ""
	}
}

func (m *SdpDesc) GetOptions() string {
	mt := m.GetMediaType()
	if (mt & kSdpMediaAudio) != 0 {
		return m.Sdp.audios[0].ice_attr.Options
	} else if (mt & kSdpMediaVideo) != 0 {
		return m.Sdp.videos[0].ice_attr.Options
	} else if (mt & kSdpMediaApplication) != 0 {
		return m.Sdp.applications[0].ice_attr.Options
	} else {
		LogWarnln("[desc] invalid media type = ", mt)
		return ""
	}
}

func (m *SdpDesc) GetCandidates() []string {
	mt := m.GetMediaType()
	if (mt & kSdpMediaAudio) != 0 {
		return m.Sdp.audios[0].candidates
	} else if (mt & kSdpMediaVideo) != 0 {
		return m.Sdp.videos[0].candidates
	} else if (mt & kSdpMediaApplication) != 0 {
		return m.Sdp.applications[0].candidates
	} else {
		LogWarnln("[desc] invalid media type = ", mt)
		return nil
	}
}

func (m *SdpDesc) CreateAnswer() bool {
	var ret bool
	send_ice_ufrag := "rtc" + RandomString(29)
	send_ice_pwd := RandomString(24)

	// select app
	if len(m.Sdp.applications) > 0 {
		for i := range m.Sdp.applications {
			app := m.Sdp.applications[i]
			app.av_ice_attr.Ufrag = send_ice_ufrag
			app.av_ice_attr.Pwd = send_ice_pwd
			ret = true
		}
	}

	// select audio
	if len(m.Sdp.audios) > 0 {
		for i := range m.Sdp.audios {
			have_opus := false
			have_isac := false
			audio := m.Sdp.audios[i]
			for j := range audio.rtpmaps {
				rtpmap := audio.rtpmaps[j]
				if rtpmap.codec == "opus" && rtpmap.frequency == 48000 {
					have_opus = true
				} else if rtpmap.codec == "isac" && rtpmap.frequency == 16000 {
					have_isac = true
				}
				if have_opus || have_isac {
					audio.av_rtpmaps["main"] = rtpmap.Clone()
					break
				}
			}
			if have_opus || have_isac {
				audio.av_ice_attr.Ufrag = send_ice_ufrag
				audio.av_ice_attr.Pwd = send_ice_pwd
				ret = true
				break
			}
		}
	}

	// select video
	if len(m.Sdp.videos) > 0 {
		for i := range m.Sdp.videos {
			have_h264 := false
			video := m.Sdp.videos[i]
			for j := range video.rtpmaps {
				rtpmap := video.rtpmaps[j]
				if rtpmap.codec == "h264" {
					have_h264 = true
					video.av_rtpmaps["main"] = rtpmap.Clone()
				} else if rtpmap.codec == "rtx" || rtpmap.codec == "red" || rtpmap.codec == "ulpfec" {
					video.av_rtpmaps[rtpmap.codec] = rtpmap.Clone()
				}
			}

			if have_h264 {
				// hardcode to select supported features
				video.use_rtx = true
				video.use_red_fec = false
				video.use_red_rtx = false
				video.use_fid = false
				if len(video.fid_ssrcs) > 0 {
					video.use_fid = true
				}

				video.av_ice_attr.Ufrag = send_ice_ufrag
				video.av_ice_attr.Pwd = send_ice_pwd
				ret = true
				break
			}
		}
	}

	m.haveAnswer = ret
	return ret
}

func (m *SdpDesc) ParseDrection(direction SdpMediaDirection) string {
	switch direction {
	case kSdpDirectionSendRecv:
		return "sendrecv"
	case kSdpDirectionRecvOnly:
		return "sendonly"
	case kSdpDirectionSendOnly:
		return "recvonly"
	case kSdpDirectionInactive:
		return "inactive"
	}
	return ""
}

func (m *SdpDesc) GetAudioCodec() string {
	if m.haveAnswer {
		for j := range m.Sdp.audios {
			audio := m.Sdp.audios[j]
			if rtpmap, ok := audio.av_rtpmaps["main"]; ok {
				return rtpmap.codec
			}
		}
	}
	return ""
}

func (m *SdpDesc) GetVideoCodec() string {
	if m.haveAnswer {
		for j := range m.Sdp.videos {
			video := m.Sdp.videos[j]
			if rtpmap, ok := video.av_rtpmaps["main"]; ok {
				return rtpmap.codec
			}
		}
	}
	return ""
}

func (m *SdpDesc) AnswerSdp() string {
	var prefix []string
	prefix = append(prefix, "v=0")
	prefix = append(prefix, "o="+kSdpOwner+" 123456789 2 IN IP4 127.0.0.1")
	prefix = append(prefix, "s=-")
	prefix = append(prefix, "t=0 0")

	bundles := "a=group:BUNDLE"
	semantics := "a=msid-semantic:WMS"
	LogPrintln("[desc] all bundles: ", m.Sdp.group_bundles)

	var body []string
	var oldSdp bool = true
	for i := range m.Sdp.group_bundles {
		bundle := m.Sdp.group_bundles[i]
		bundles += " " + bundle
		LogPrintln("[desc] one media bundle=", bundle)

		// check m=audio
		for j := range m.Sdp.audios {
			audio := m.Sdp.audios[j]
			if audio.mid == bundle {
				rtpmap, _ := audio.av_rtpmaps["main"]
				mline := "m=audio 1 " + audio.proto
				if rtpmap != nil {
					mline += " " + Itoa(rtpmap.ptype)
				}
				mline += " 126" // add telephone-event
				body = append(body, mline)
				body = append(body, "c=IN IP4 0.0.0.0")
				body = append(body, "a=ice-ufrag:"+audio.av_ice_attr.Ufrag)
				body = append(body, "a=ice-pwd:"+audio.av_ice_attr.Pwd)
				body = append(body, "a=fingerprint:"+audio.fingerprint.ToStringBySpace())
				body = append(body, "a=setup:passive")
				aextmap := "a=extmap:"
				for k := range audio.extmaps {
					extmap := audio.extmaps[k]
					if strings.Contains(extmap.uri, "ssrc-audio-level") {
						aextmap += " " + Itoa(extmap.id) + " " + extmap.uri
					}
				}
				body = append(body, aextmap)
				if adir := m.ParseDrection(audio.direction); len(adir) > 0 {
					body = append(body, adir)
				}
				body = append(body, "a=mid:"+bundle)
				body = append(body, "a=rtcp-mux")
				if rtpmap != nil {
					artpmap := "a=rtpmap:" + rtpmap.a_rtpmap()
					if rtpmap.channels > 0 {
						artpmap += "/" + Itoa(rtpmap.channels)
					}
					body = append(body, artpmap)

					if rtpmap.codec == "opus" {
						afmtp := "a=fmtp:" + Itoa(rtpmap.ptype) + " minptime=20;useinbandfec=1;usedtx=0"
						body = append(body, afmtp)
					}
				}
				body = append(body, "a=maxptime:60")
				body = append(body, "a=rtpmap:126 telephone-event/8000")
				if oldSdp {
					semantics += " stream_audio_label"
					body = append(body, "a=ssrc:1 cname:"+kSdpCname)
					body = append(body, "a=ssrc:1 msid:stream_audio_label track_audio_label")
					body = append(body, "a=ssrc:1 mslabel:stream_audio_label")
					body = append(body, "a=ssrc:1 label:track_audio_label")
				} else {
					semantics += " {stream_audio_label}"
					body = append(body, "a=msid:{stream_audio_label} {track_audio_label}")
					body = append(body, "a=ssrc:1 cname:{"+kSdpCname+"}")
				}
				break
			}
		}

		// check m=application
		for j := range m.Sdp.applications {
			app := m.Sdp.applications[j]
			if app.mid == bundle {
				body = append(body, "m=application 9 "+app.proto+" "+app.ptypes[0])
				body = append(body, "c=IN IP4 0.0.0.0")
				body = append(body, "a=ice-ufrag:"+app.av_ice_attr.Ufrag)
				body = append(body, "a=ice-pwd:"+app.av_ice_attr.Pwd)
				body = append(body, "a=fingerprint:"+app.fingerprint.ToStringBySpace())
				body = append(body, "a=setup:passive")
				body = append(body, "a=mid:"+bundle)
				if adir := m.ParseDrection(app.direction); len(adir) > 0 {
					body = append(body, adir)
				}
				if app.sctp.is_sctpmap {
					body = append(body, "a=sctpmap:"+Itoa(app.sctp.port)+" "+app.sctp.name+" "+Itoa(app.sctp.number))
				} else {
					body = append(body, "a=sctp-port:"+Itoa(app.sctp.port))
				}
				break
			}
		}

		// check m=video
		for j := range m.Sdp.videos {
			video := m.Sdp.videos[j]
			if video.mid == bundle {
				rtpmap, _ := video.av_rtpmaps["main"]
				redmap, _ := video.av_rtpmaps["red"]
				fecmap, _ := video.av_rtpmaps["ulpfec"]

				mline := "m=video 1 " + video.proto
				if rtpmap != nil {
					mline += " " + Itoa(rtpmap.ptype)
					if video.use_rtx {
						mline += " " + Itoa(rtpmap.apt_ptype)
					}
				}
				if video.use_red_fec {
					if redmap != nil && fecmap != nil {
						mline += " " + Itoa(redmap.ptype)
						if video.use_red_rtx {
							mline += " " + Itoa(redmap.apt_ptype)
						}
						mline += " " + Itoa(fecmap.ptype)
					}
				}
				body = append(body, mline)

				body = append(body, "c=IN IP4 0.0.0.0")
				body = append(body, "b=AS:1500") // refine
				body = append(body, "a=ice-ufrag:"+video.av_ice_attr.Ufrag)
				body = append(body, "a=ice-pwd:"+video.av_ice_attr.Pwd)
				body = append(body, "a=fingerprint:"+video.fingerprint.ToStringBySpace())
				body = append(body, "a=setup:passive")
				body = append(body, "a=mid:"+bundle)
				if adir := m.ParseDrection(video.direction); len(adir) > 0 {
					body = append(body, adir)
				}
				body = append(body, "a=rtcp-mux")
				if rtpmap != nil {
					body = append(body, "a=rtpmap:"+rtpmap.a_rtpmap())
					if video.use_rtx {
						body = append(body, "a=rtcp-fb:"+Itoa(rtpmap.ptype)+" nack")
					}
					body = append(body, "a=rtcp-fb:"+Itoa(rtpmap.ptype)+" nack pli")
					body = append(body, "a=rtcp-fb:"+Itoa(rtpmap.ptype)+" ccm fir")
					body = append(body, "a=rtcp-fb:"+Itoa(rtpmap.ptype)+" goog-remb")
					body = append(body, "a=fmtp:"+Itoa(rtpmap.ptype)+" level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f")
					if video.use_rtx {
						body = append(body, "a=rtpmap:"+rtpmap.a_rtxmap())
						body = append(body, "a=fmtp:"+rtpmap.a_fmtp_apt())
					}
				}
				if redmap != nil && fecmap != nil {
					if video.use_red_fec {
						body = append(body, "a=rtpmap:"+redmap.a_rtpmap())
						body = append(body, "a=rtpmap:"+fecmap.a_rtpmap())
						if video.use_red_rtx {
							body = append(body, "a=rtpmap:"+redmap.a_rtxmap())
							body = append(body, "a=fmtp:"+redmap.a_fmtp_apt())
						}
					}
				}

				// ssrc template which will be processed in client
				//   keyword: <main_ssrc>, <rtx_ssrc>
				if video.use_fid {
					body = append(body, "a=ssrc-group:FID main_ssrc rtx_ssrc")
				}
				if oldSdp {
					body = append(body, "a=ssrc:main_ssrc cname:"+kSdpCname)
					body = append(body, "a=ssrc:main_ssrc msid:stream_video_label track_video_label")
					body = append(body, "a=ssrc:main_ssrc mslabel:stream_video_label")
					body = append(body, "a=ssrc:main_ssrc label:track_video_label")
				} else {
					body = append(body, "a=msid:{stream_video_label} {track_video_label}")
					body = append(body, "a=ssrc:main_ssrc cname:{"+kSdpCname+"}")
				}
				if video.use_fid {
					if oldSdp {
						body = append(body, "a=ssrc:rtx_ssrc cname:"+kSdpCname)
						body = append(body, "a=ssrc:rtx_ssrc msid:stream_video_label track_video_label")
						body = append(body, "a=ssrc:rtx_ssrc mslabel:stream_video_label")
						body = append(body, "a=ssrc:rtx_ssrc label:track_video_label")
					} else {
						body = append(body, "a=ssrc:rtx_ssrc cname:{"+kSdpCname+"}")
					}
				}
				if oldSdp {
					semantics += " stream_video_label"
				} else {
					semantics += " {stream_video_label}"
				}
				break
			}
		}
	}

	prefix = append(prefix, bundles, semantics)
	sdp := append(prefix, body...)
	return strings.Join(sdp, "\n")
}

// UpdateSdpCandidates to replace sdp candidates with new.
func UpdateSdpCandidates(data []byte, candidates []string) []byte {
	if len(candidates) == 0 {
		return data
	}

	sp := "\r\n"
	lines := strings.Split(string(data), "\r\n")
	if len(lines) <= 1 {
		sp = "\n"
		lines = strings.Split(string(data), "\n")
	}

	var newMline bool
	var hadCandidate bool
	var sdp []string

	//LogPrintln("[sdp] replace candidates, sdp lines=", len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "m=") {
			if newMline && !hadCandidate {
				sdp = append(sdp, candidates...)
				sdp = append(sdp, "a=end-of-candidates")
			}
			newMline = true
			hadCandidate = false
			sdp = append(sdp, line)
		} else if strings.HasPrefix(line, "a=candidate:") {
			// drop it
		} else if strings.HasPrefix(line, "a=end-of-candidates") {
			hadCandidate = true
			sdp = append(sdp, candidates...)
			sdp = append(sdp, line)
		} else if len(line) > 2 {
			sdp = append(sdp, line)
		} else {
			if newMline && !hadCandidate {
				sdp = append(sdp, candidates...)
				sdp = append(sdp, "a=end-of-candidates")
			}
			sdp = append(sdp, line)
		}
	}

	return []byte(strings.Join(sdp, sp))
}

// GetSdpCandidates to parse candidates from sdp
func GetSdpCandidates(data []byte) []string {
	lines := strings.Split(string(data), "\r\n")
	if len(lines) <= 1 {
		lines = strings.Split(string(data), "\n")
	}

	var candidates []string
	//LogPrintln("[sdp] replace candidates, sdp lines=", len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "a=candidate:") {
			candidates = append(candidates, line)
			// skip
		} else if strings.HasPrefix(line, "a=end-of-candidates") {
			break
		}
	}
	return candidates
}

// a=candidate:1 1 udp 2113937151 192.168.1.1 5000 typ host
// a=candidate:2 1 tcp 1518280447 192.168.1.1 443 typ host tcptype passive
type SdpCandidate struct {
	Foundation  string
	ComponentId int    // 1-256, e.g., RTP-1, RTCP-2
	Transport   string // udp/tcp
	Priority    int    // 1-(2^31 - 1)
	RelAddr     string // raddr
	RelPort     string // rport
	CandType    string // typ host/srflx/prflx/relay
	NetType     string // network type
}

func ParseSdpCandidates(lines []string) ([]SdpCandidate, []string) {
	var cands []SdpCandidate
	var candLines []string
	for _, line := range lines {
		if !strings.HasPrefix(line, "a=candidate:") {
			continue
		}
		if cand := ParseSdpCandidate(line); cand != nil {
			cands = append(cands, *cand)
			candLines = append(candLines, line)
		}
	}
	return cands, candLines
}

func ParseSdpCandidate(line string) *SdpCandidate {
	items := strings.Split(line, " ")
	if len(items) < 8 {
		LogWarnln("[sdp] invalid sdp candidate:", line)
		return nil
	}
	foundation := ""
	if heads := strings.Split(items[0], ":"); len(heads) == 2 {
		foundation = heads[1]
	}

	// typ host/srflx/prflx/relay
	candType := items[6] + " " + items[7]

	// tcptype passive
	netType := ""
	if len(items) >= 9 {
		netType = strings.Join(items[8:], " ")
	}

	return &SdpCandidate{
		foundation,
		Atoi(items[1]), items[2], Atoi(items[3]), items[4], items[5],
		candType,
		netType,
	}
}

func ParseSdpCandidateHost(line string) string {
	if cand := ParseSdpCandidate(line); cand != nil {
		return cand.RelAddr
	} else {
		return ""
	}
}

func ParseSdpCandidateIp(line string) string {
	if host := ParseSdpCandidateHost(line); len(host) > 0 {
		return LookupIP(host)
	} else {
		return ""
	}
}
