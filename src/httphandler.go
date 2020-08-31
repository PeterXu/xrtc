package webrtc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	log "github.com/PeterXu/xrtc/util"
	util "github.com/PeterXu/xrtc/util"
	uuid "github.com/PeterXu/xrtc/uuid"
)

const (
	kAgentVersion = "xrtc-v1.0"

	// client Api
	kApiWebrtcVersion = "/webrtc/version"
	kApiWebrtcStatus  = "/webrtc/status"

	// new api
	kApiWebrtcRoute = "/webrtc/route"

	// old api
	kApiWebrtcRequest = "/webrtc/request"
	kApiWebrtcBoard   = "/board"
)

/// The /webrtc/route api json (default)
// client send json(with server-candaidates) to proxy.

// client recv response from proxy.
//  if recv server candidates, client will connect to media server directly,
//  if recv proxy candidates, client will connect to proxy, and proxy to media server.

/// The http server handler

type HttpServerHandler struct {
	TAG string

	Name string

	Config RestNetParams

	// UUID returns a unique id in uuid format.
	// If UUID is nil, uuid.NewUUID() is used.
	UUID func() string
}

func NewHttpServeHandler(name string, cfg *RestNetParams) http.Handler {
	return &HttpServerHandler{
		TAG:    "[HTTP]",
		Name:   name,
		Config: *cfg,
		UUID:   uuid.NewUUID,
	}
}

func (p *HttpServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//log.Print2f(p.TAG, "ServeHTTP, http/https req: %v, method:%v", r.URL.Path, r.Method)
	if p.Config.RequestID != "" {
		r.Header.Set(p.Config.RequestID, p.UUID())
	}

	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers",
		"Content-Type, Content-Range, Content-Disposition, Content-Description")

	if r.Method == http.MethodOptions {
		log.Warnln(p.TAG, "http options")
		w.Write(createJsonStatus("OK"))
		return
	}

	p.handleRequest(w, r)
}

func (p *HttpServerHandler) handleRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	log.Println(p.TAG, "http path=", path, r.RemoteAddr)

	if r.Method == http.MethodGet {
		if strings.HasPrefix(path, kApiWebrtcVersion) {
			w.Write(createJsonBytes("version", kAgentVersion))
			return
		}

		if strings.HasPrefix(path, kApiWebrtcStatus) {
			w.Write(createJsonBytes("status", "OK"))
			return
		}
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write(createJsonStatus("Only Post Allowed"))
		return
	}

	p.handlePostRequest(w, r)
}

func (p *HttpServerHandler) handlePostRequest(w http.ResponseWriter, r *http.Request) {
	encoding := r.Header.Get("Content-Encoding")
	body, err := util.ReadHttpBody(r.Body, encoding)
	if body == nil || err != nil {
		log.Warnln(p.TAG, "http invalid reqeust body, err=", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(createJsonStatus(err.Error()))
		return
	}

	path := r.URL.Path
	raddr := r.RemoteAddr
	log.Println(p.TAG, "handle http body-len=", len(body), raddr)

	var handleErr error
	switch {
	case strings.HasPrefix(path, kApiWebrtcRoute):
		handleErr = p.handleWebrtcRoute(w, raddr, body)
	case strings.HasPrefix(path, kApiWebrtcRequest),
		strings.HasPrefix(path, kApiWebrtcBoard):
		handleErr = p.handleWebrtcRequest(w, raddr, body)
	default:
		w.WriteHeader(http.StatusNotFound)
	}

	if handleErr != nil {
		log.Warnln(p.TAG, "handle", path, "error:", handleErr)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(createJsonStatus(handleErr.Error()))
	}
}

func (p *HttpServerHandler) sendJson(w http.ResponseWriter, jdata interface{}) error {
	if data, err := json.Marshal(jdata); err == nil {
		w.Write(data)
		return nil
	} else {
		return err
	}
}

// process /webrtc/route.
func (p *HttpServerHandler) handleWebrtcRoute(w http.ResponseWriter, raddr string, body []byte) error {
	var jreq RestPacket
	if err := json.Unmarshal(body, &jreq); err != nil {
		return err
	}

	log.Println(p.TAG, "http webrtc route=", raddr, jreq)

	candidates := p.handleCandidates(raddr, &jreq)
	if len(candidates) == 0 {
		return errors.New("No candidates")
	}

	jresp := &RestPacket{
		SessionKey: jreq.SessionKey,
		Candidates: candidates,
	}

	return p.sendJson(w, jresp)
}

// process /webrtc/request and the flow is: client <--> proxy <--> server.
func (p *HttpServerHandler) handleWebrtcRequest(w http.ResponseWriter, raddr string, body []byte) error {
	// parse request
	var jreq RestBoardRequest
	if err := json.Unmarshal(body, &jreq); err != nil {
		return err
	}

	log.Println(p.TAG, "http webrtc request=", raddr, jreq)

	// get board offer
	var szOffer string
	if channel := getBoardChannel(jreq.Action); channel != nil {
		szOffer = channel.GetWebrtcOffer()
	}

	// parse offer which must have 'a=candidate:' lines
	var offer util.SdpDesc
	if len(szOffer) == 0 || !offer.Parse([]byte(szOffer)) {
		return errors.New("Invalid offer in request")
	}

	// request must have 'dst_url' (e.g. url of one media server).
	if jreq.DstUrl == nil {
		return errors.New("No dst_url in request")
	}
	dst_url := *jreq.DstUrl
	if _, err := url.ParseRequestURI(dst_url); err != nil {
		return err
	}

	// generate response
	var jresp RestBoardResponse

	// send offer to 'dst_url'(media server) which will response with answer.
	//  like a http reverse-proxy
	if rdata, err := util.HttpSendPost(dst_url, body); err == nil {
		if err := json.Unmarshal(rdata, &jresp); err != nil {
			return err
		}
	} else {
		return err
	}

	// get board answer
	var szAnswer string
	if channel := getBoardChannel(jresp.Action); channel != nil {
		szAnswer = channel.GetWebrtcAnswer()
	}

	// parse answer
	var answer util.SdpDesc
	if len(szAnswer) == 0 || !answer.Parse([]byte(szAnswer)) {
		return errors.New("Invalid answer")
	}

	// parse ICE from offer/answer
	var packet RestPacket
	writeToRestSdpIce(packet.OfferIce, &offer)
	writeToRestSdpIce(packet.AnswerIce, &answer)

	// parse candidates from offer
	_, packet.Candidates = util.ParseSdpCandidates(offer.GetCandidates())

	// handle to get new candidates
	candidates := p.handleCandidates(raddr, &packet)
	if len(candidates) == 0 {
		return errors.New("No candidates for client-use")
	}

	// update answer with new candidates and then reply to client
	newAnswerBuf := string(util.UpdateSdpCandidates([]byte(szAnswer), candidates))
	jresp.Action.UserRoster[0].AudioStatus.Channels[0].WebrtcAnswer = &newAnswerBuf

	return p.sendJson(w, jresp)
}

// srcAddr: client address,
// route.Candidates: the address of media server,
// Current Service will generate candidates for client using:
//      (a) if new (different with route.Candidates), client will connenct webrtc to one proxy,
//      (b) if not-new (the same as route.Candidates), client will connect webrtc to media server.
func (p *HttpServerHandler) handleCandidates(srcAddr string, pkt *RestPacket) []string {
	// default use original dst-candidates(server)
	dstCands := pkt.Candidates

	proxyCands := Inst().Candidates()
	if len(proxyCands) == 0 {
		return dstCands
	}

	srcIp := util.ParseHostIp(srcAddr)
	proxyIp := util.ParseSdpCandidateIp(proxyCands[0])
	dstIp := util.ParseSdpCandidateIp(dstCands[0])
	log.Println(p.TAG, "check candidate ips:", srcIp, proxyIp, dstIp)

	// default use server-candidates
	candidates := dstCands

	if checkGeoOptimal(srcIp, proxyIp, dstIp) {
		log.Println(p.TAG, "use proxy between client and server")
		// use proxy-candidates: client -> proxy -> server
		candidates = proxyCands

		// add to proxy cache for processing
		info := &WebrtcIce{byRoute: false}
		writeToSdpIceAttr(&info.OfferIce, pkt.OfferIce)
		writeToSdpIceAttr(&info.AnswerIce, pkt.AnswerIce)
		info.Candidates = util.CloneArray(pkt.Candidates)

		item := NewCacheItem(info)
		key := info.AnswerIce.Ufrag + ":" + info.OfferIce.Ufrag
		Inst().Cache().Set(key, item)
	} else {
		// use server-candidaates: client -> server
		// And nop for proxy
		log.Println(p.TAG, "use direct between client and server")
	}

	return candidates
}

/// misc tools

func createJsonString(key, value string) string {
	if len(key) > 0 && len(value) > 0 {
		return fmt.Sprintf("{%s: %s}", key, value)
	} else {
		return "{}"
	}
}

func createJsonBytes(key, value string) []byte {
	return []byte(createJsonString(key, value))
}

func createJsonStatus(value string) []byte {
	return createJsonBytes("status", value)
}

func writeToRestSdpIce(to *RestSdpIce, from *util.SdpDesc) {
	ufrag, pwd, options := from.GetUfrag(), from.GetPwd(), from.GetOptions()
	to.Ufrag = &ufrag
	to.Pwd = &pwd
	to.Options = &options
}

func writeToSdpIceAttr(to *util.SdpIceAttr, from *RestSdpIce) {
	ufrag, pwd, options := from.GetUfrag(), from.GetPwd(), from.GetOptions()
	to.Ufrag = ufrag
	to.Pwd = pwd
	to.Options = options
}

func getBoardChannel(action *RestBoardAction) *RestBoardChannel {
	if action != nil && len(action.UserRoster) > 0 {
		roster := action.UserRoster[0]
		if roster.AudioStatus != nil && len(roster.AudioStatus.Channels) > 0 {
			return roster.AudioStatus.Channels[0]
		}
	}
	return nil
}
