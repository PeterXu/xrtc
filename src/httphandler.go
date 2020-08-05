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
	kApiWebrtcRoute   = "/webrtc/route"
	kApiWebrtcRequest = "/webrtc/request"
	kApiWebrtcBoard   = "/board"

	// proxy Api
	kApiProxyWss        = "/proxy/wss"
	kApiProxyRegister   = "/proxy/register"
	kApiProxyUnRegister = "/proxy/unregister"
	kApiProxyStatus     = "/proxy/status"
	kApiProxyQuery      = "/proxy/query"
)

/// The /webrtc/route api json (default)

type SdpIceJson struct {
	Ufrag   string `json:"ufrag"`
	Pwd     string `json:"pwd"`
	Options string `json:"options"`
}

// client send json(with server-candaidates) to proxy.
type RouteJson struct {
	SessionKey string     `json:"session_key,omitempty"`
	OfferIce   SdpIceJson `json:"offer_ice"`
	AnswerIce  SdpIceJson `json:"answer_ice"`
	Candidates []string   `json:"candidates"` // server-candidates
}

type RouteInfo struct {
	RouteJson
	byRoute bool
}

// client recv response from proxy.
//  if recv server candidates, client will connect to media server directly,
//  if recv proxy candidates, client will connect to proxy, and proxy to media server.
type RouteResponseJson struct {
	SessionKey string   `json:"session_key,omitempty"`
	Candidates []string `json:"candidates"` // returned candidates
}

/// The webrtc/request api json (desperated)

type RequestChannelJson struct {
	Offer  string `json:"webrtc_offer,omitempty"`
	Answer string `json:"webrtc_answer,omitempty"`
}

type RequestStatusJson struct {
	Channels []RequestChannelJson `json:"channels"`
}

type RequestRosterJson struct {
	Status RequestStatusJson `json:"audio_status"`
}

type RequestActionJson struct {
	SessionKey string              `json:"session_key,omitempty"` // confId
	Rosters    []RequestRosterJson `json:"user_roster"`
}

type RequestJson struct {
	Type          string            `json:"type"`
	Action        RequestActionJson `json:"action"`
	MultiConn     bool              `json:"multi_webrtc_conn"`
	Agent         string            `json:"agent"`                 // chrome/firefox
	Version       int               `json:"version"`               // browser version
	WebrtcVersion int               `json:"webrtc_client_version"` // webrtc version
	IsDS          bool              `json:"is_ds,omitempty"`       // camera/ds video
	DstUrl        string            `json:"dst_url, omitmepty"`
}

type RequestResponseJson struct {
	Action RequestActionJson `json:"action"`
	Code   string            `json:"code"`
}

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
	raddr := r.RemoteAddr
	log.Println(p.TAG, "http path=", path, raddr)

	if strings.HasPrefix(path, kApiWebrtcVersion) {
		w.Write(createJsonBytes("version", kAgentVersion))
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write(createJsonStatus("Only Post Allowed"))
		return
	}

	encoding := r.Header.Get("Content-Encoding")
	body, err := util.ReadHttpBody(r.Body, encoding)
	if body == nil || err != nil {
		log.Warnln(p.TAG, "http invalid reqeust body, err=", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(createJsonStatus(err.Error()))
		return
	}

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
	var jreq RouteJson
	if err := json.Unmarshal(body, &jreq); err != nil {
		return err
	}

	log.Println(p.TAG, "http webrtc route=", raddr, jreq)

	candidates := p.handleCandidates(raddr, jreq)
	if len(candidates) == 0 {
		return errors.New("No candidates")
	}

	jresp := RouteResponseJson{
		SessionKey: jreq.SessionKey,
		Candidates: candidates,
	}

	return p.sendJson(w, jresp)
}

// process /webrtc/request and the flow is: client <--> proxy <--> server.
func (p *HttpServerHandler) handleWebrtcRequest(w http.ResponseWriter, raddr string, body []byte) error {
	// parse request
	var jreq RequestJson
	if err := json.Unmarshal(body, &jreq); err != nil {
		return err
	}

	log.Println(p.TAG, "http webrtc request=", raddr, jreq)

	// parse offer which must have 'a=candidate:' lines
	var offer util.MediaDesc
	if !offer.Parse([]byte(jreq.Action.Rosters[0].Status.Channels[0].Offer)) {
		return errors.New("Invalid offer")
	}

	// So request must have 'dst_url' (e.g. url of one media server).
	if _, err := url.ParseRequestURI(jreq.DstUrl); err != nil {
		return err
	}

	// generate response
	var jresp RequestResponseJson

	// send offer to 'dst_url'(media server) which will response with answer.
	// like a http reverse-proxy
	if rdata, err := util.HttpSendPost(jreq.DstUrl, body); err == nil {
		if err := json.Unmarshal(rdata, &jresp); err != nil {
			return err
		}
	} else {
		return err
	}

	// parse answer
	var answer util.MediaDesc
	answerBuf := jresp.Action.Rosters[0].Status.Channels[0].Answer
	if !answer.Parse([]byte(answerBuf)) {
		return errors.New("Invalid answer")
	}

	// parse ICE from offer/answer
	var regInfo RouteJson
	regInfo.OfferIce.Ufrag = offer.GetUfrag()
	regInfo.OfferIce.Pwd = offer.GetPwd()
	regInfo.OfferIce.Options = offer.GetOptions()
	regInfo.AnswerIce.Ufrag = answer.GetUfrag()
	regInfo.AnswerIce.Pwd = answer.GetPwd()
	regInfo.AnswerIce.Options = answer.GetOptions()

	// parse candidates from offer
	_, regInfo.Candidates = util.ParseCandidates(offer.GetCandidates())

	// handle to get new candidates
	candidates := p.handleCandidates(raddr, regInfo)
	if len(candidates) == 0 {
		return errors.New("No candidates for client-use")
	}

	// update answer with new candidates and then reply to client
	newAnswerBuf := util.UpdateSdpCandidates([]byte(answerBuf), candidates)
	jresp.Action.Rosters[0].Status.Channels[0].Answer = string(newAnswerBuf)

	return p.sendJson(w, jresp)
}

// srcAddr: client address,
// route.Candidates: the address of media server,
// Current Service will generate candidates for client using:
//      (a) if new (different with route.Candidates), client will connenct webrtc to one proxy,
//      (b) if not-new (the same as route.Candidates), client will connect webrtc to media server.
func (p *HttpServerHandler) handleCandidates(srcAddr string, route RouteJson) []string {
	// default use original dst-candidates(server)
	dstCands := route.Candidates

	proxyCands := Inst().Candidates()
	if len(proxyCands) == 0 {
		return dstCands
	}

	srcIp := util.ParseHostIp(srcAddr)
	proxyIp := util.ParseCandidateIp(proxyCands[0])
	dstIp := util.ParseCandidateIp(dstCands[0])
	log.Println(p.TAG, "check candidate ips:", srcIp, proxyIp, dstIp)

	// default use server-candidates
	candidates := dstCands

	if checkGeoOptimal(srcIp, proxyIp, dstIp) {
		log.Println(p.TAG, "use proxy between client and server")
		// use proxy-candidates: client -> proxy -> server
		candidates = proxyCands

		// add to proxy cache for processing
		info := &RouteInfo{route, false}
		item := NewCacheItem(info)
		key := info.AnswerIce.Ufrag + ":" + info.OfferIce.Ufrag
		Inst().Cache().Set(key, item)
	} else {
		// use server-candidaates: client -> server
		// And nop for proxy
	}

	return candidates
}

/// The /proxy/query api

type ProxyStatus struct {
	Uuid     string `json:"uuid"`
	Name     string `json:"name"`
	Address  string `json:"address"` // http://ip/host:port/
	Rtt      int    `json:"rtt"`
	BweIn    int    `json:"bwe_in"`  // bandwidth in
	BweOut   int    `json:"bwe_out"` // bandwidth out
	Load     int    `json:"load"`    // current load
	Capacity int    `json:"capacity"`
	City     string `json:"city"`
	Country  string `json:"country"`
	LastTime bool   `json:"last_time"`
}

type ProxyQueryJson struct {
	Self  ProxyStatus `json:"self"`  // info of sender
	Peers ProxyStatus `json:"peers"` // peers in sender
}

type ProxyQueryResponseJson struct {
	Self  ProxyStatus   `json:"self"`  // info of remote
	Peers []ProxyStatus `json:"peers"` // peers in remote
}

// The /proxy/link/create api

type ProxyLinkJson struct {
	Uuid string `json:"uuid"` // link uuid
	Akey string `json:"akey"` // ice akey for data
}

// client -> Proxy -> LinkA -> LinkB -> server
type ProxyCreateLinkJson struct {
	Uuid    string          `json:"uuid"`    // send from proxy(uuid)
	Links   []ProxyLinkJson `json:"links"`   // links info
	Servers []string        `json:"servers"` // "udp|tcp://ip:port"
}
