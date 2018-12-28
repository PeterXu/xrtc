package webrtc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	log "github.com/PeterXu/xrtc/util"
	util "github.com/PeterXu/xrtc/util"
	uuid "github.com/PeterXu/xrtc/uuid"
)

const (
	kVersion    = "xrtc-agent"
	kApiVersion = "/webrtc/version"
	kApiRequest = "/webrtc/request"
)

type SdpIceInfo struct {
	Ufrag   string `json:"ufrag"`
	Pwd     string `json:"pwd"`
	Options string `json:"options"`
}

type RegisterRequest struct {
	SessionKey string     `json:"session_key,omitempty"`
	OfferIce   SdpIceInfo `json:"offer_ice"`
	AnswerIce  SdpIceInfo `json:"answer_ice"`
	Candidates []string   `json:"candidates"` // dest candidates to server
}

type RegisterResponse struct {
	SessionKey string   `json:"session_key,omitempty"`
	Candidates []string `json:"candidates"` // proxy candidates for client
}

type HttpServerHandler struct {
	TAG string

	Name string

	Config HttpParams

	// UUID returns a unique id in uuid format.
	// If UUID is nil, uuid.NewUUID() is used.
	UUID func() string
}

func NewHttpServeHandler(name string, cfg *HttpParams) http.Handler {
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

	path := r.URL.Path
	log.Println(p.TAG, "http path=", path, r.RemoteAddr)
	switch {
	case strings.HasPrefix(path, kApiVersion):
		w.Write(createJsonBytes("version", kVersion))
	case strings.HasPrefix(path, kApiRequest):
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write(createJsonStatus("Only Post Allowed"))
			break
		}
		encoding := r.Header.Get("Content-Encoding")
		body, err := util.ReadHttpBody(r.Body, encoding)
		if body == nil || err != nil {
			log.Warnln(p.TAG, "http invalid reqeust body, err=", err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write(createJsonStatus(err.Error()))
			break
		}

		raddr := r.RemoteAddr
		log.Println(p.TAG, "http body=", len(body), raddr)

		if err := p.handleRequest(w, raddr, body); err != nil {
			log.Warnln(p.TAG, "handle request error:", err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write(createJsonStatus(err.Error()))
			break
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (p *HttpServerHandler) handleRequest(w http.ResponseWriter, raddr string, body []byte) error {
	var jreq RegisterRequest
	if err := json.Unmarshal(body, &jreq); err != nil {
		return err
	}

	log.Println(p.TAG, "http req=", raddr, jreq)

	// default use orignal server-candidates
	serverCandidates := jreq.Candidates
	if len(serverCandidates) == 0 {
		return errors.New("no server candidates")
	}

	proxyCandidates := Inst().Candidates()
	if len(proxyCandidates) == 0 {
		return errors.New("no proxy candidates")
	}

	clientIp := util.ParseHostIp(raddr)
	serverIp := util.ParseCandidateIp(serverCandidates[0])
	proxyIp := util.ParseCandidateIp(proxyCandidates[0])

	log.Println(p.TAG, "ips:", serverIp, proxyIp, clientIp)

	candidates := serverCandidates
	isOptimal := checkGeoOptimal(clientIp, proxyIp, serverIp)
	if isOptimal {
		// client -> proxy -> server
		log.Println(p.TAG, "use proxy between client and server")

		// use proxy ip-candidates to client
		candidates = proxyCandidates

		// add to cache for processing
		item := NewCacheItem(&jreq)
		key := jreq.AnswerIce.Ufrag + ":" + jreq.OfferIce.Ufrag
		Inst().Cache().Set(key, item)
	}

	resp := RegisterResponse{
		SessionKey: jreq.SessionKey,
		Candidates: candidates,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	} else {
		w.Write(data)
		return nil
	}
}
