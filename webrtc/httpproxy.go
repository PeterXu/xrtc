package webrtc

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	gziph "github.com/PeterXu/xrtc/gzip"
	uuid "github.com/PeterXu/xrtc/uuid"
	log "github.com/Sirupsen/logrus"
)

const kHttpHeaderWebrtcCheck string = "x-user-webrtc-check"

func readHTTPBody(httpBody io.ReadCloser) ([]byte, error) {
	if body, err := ioutil.ReadAll(httpBody); err == nil {
		err = httpBody.Close()
		return body, err
	} else {
		return nil, err
	}
}

func procHTTPBody(httpBody io.ReadCloser, encoding string) ([]byte, error) {
	var body []byte
	var err error

	if body, err = readHTTPBody(httpBody); err != nil {
		fmt.Println("invalid http body, err=", err)
		return nil, err
	}

	//fmt.Println("http body encoding: ", encoding)
	if encoding == "gzip" {
		var zr *gzip.Reader
		if zr, err = gzip.NewReader(bytes.NewReader(body)); err == nil {
			body, err = ioutil.ReadAll(zr)
			zr.Close()
		}
	} else if encoding == "deflate" {
		var zr io.ReadCloser
		if zr = flate.NewReader(bytes.NewReader(body)); zr != nil {
			body, err = ioutil.ReadAll(zr)
			zr.Close()
		}
	} else if len(encoding) > 0 {
		err = errors.New("unsupport encoding:" + encoding)
	}

	return body, err
}

func newHTTPProxy(target *url.URL, tr http.RoundTripper, flush time.Duration) http.Handler {
	return &httputil.ReverseProxy{
		// this is a simplified director function based on the
		// httputil.NewSingleHostReverseProxy() which does not
		// mangle the request and target URL since the target
		// URL is already in the correct format.
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = target.Path
			req.URL.RawQuery = target.RawQuery
			if _, ok := req.Header["User-Agent"]; !ok {
				// explicitly disable User-Agent so it's not set to default value
				req.Header.Set("User-Agent", "")
			}

			// TODO: process request body
			if req.Method == http.MethodPost && req.ContentLength > 0 {
				req.Header.Add(kHttpHeaderWebrtcCheck, "1")
				encoding := req.Header.Get("Content-Encoding")
				body, err := procHTTPBody(req.Body, encoding)
				if body == nil || err != nil {
					fmt.Println("invalid reqeust body, err=", err)
					return
				}
				if jreq, err := ParseUmsRequest(body); err == nil {
					offer := []byte(jreq.GetOffer())
					//fmt.Println("parse offer: ", len(offer))
					adminChan := Inst().ChanAdmin()
					adminChan <- NewWebrtcAction(offer, WebrtcActionOffer)
				} else {
					fmt.Println("parse offer error:", err)
				}
				//fmt.Println("http request len: ", len(body))
				req.Body = ioutil.NopCloser(bytes.NewReader(body))
			}
		},
		FlushInterval: flush,
		Transport:     tr,
		ModifyResponse: func(resp *http.Response) error {
			if resp.StatusCode != http.StatusOK || resp.ContentLength <= 0 {
				return nil
			}

			check := resp.Request.Header.Get(kHttpHeaderWebrtcCheck)
			if check != "1" {
				return nil
			}

			// TODO: process response body
			encoding := resp.Request.Header.Get("Content-Encoding")
			body, err := procHTTPBody(resp.Body, encoding)
			if body == nil || err != nil {
				fmt.Println("invalid http response body, err:", err)
				return nil
			}

			if jresp, err := ParseUmsResponse(body); err == nil {
				answer := []byte(jresp.GetAnswer())
				//fmt.Println("parse answer: ", len(answer))
				adminChan := Inst().ChanAdmin()
				adminChan <- NewWebrtcAction(answer, WebrtcActionAnswer)
			} else {
				fmt.Println("parse answer error:", err)
			}

			//fmt.Println("http response body: ", len(body), ", request:", resp.Request.ContentLength)
			resp.Body = ioutil.NopCloser(bytes.NewReader(body))
			resp.ContentLength = int64(len(body))
			resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
			resp.Header.Del("Content-Encoding")
			return nil
		},
	}
}

var kDefaultRouteTarget = &RouteTarget{
	Service:       "default",
	TLSSkipVerify: true,
	URL: &url.URL{
		Scheme: "http",
		Host:   "127.0.0.1:8080",
		Path:   "/",
	},
}

var kDefaultRouteTarget2 = &RouteTarget{
	Service:       "default",
	TLSSkipVerify: true,
	URL: &url.URL{
		Scheme: "https",
		Host:   "119.254.195.20",
		Path:   "/",
	},
}

type RouteTarget struct {
	// Service is the name of the service the targetURL points to
	Service string

	// StripPath will be removed from the front of the outgoing
	// request path
	StripPath string

	// TLSSkipVerify disables certificate validation for upstream
	// TLS connections.
	TLSSkipVerify bool

	// Host signifies what the proxy will set the Host header to.
	// The proxy does not modify the Host header by default.
	// When Host is set to 'dst' the proxy will use the host name
	// of the target host for the outgoing request.
	Host string

	// URL is the endpoint the service instance listens on
	URL *url.URL

	// RedirectCode is the HTTP status code used for redirects.
	// When set to a value > 0 the client is redirected to the target url.
	RedirectCode int

	// RedirectURL is the redirect target based on the request.
	// This is cached here to prevent multiple generations per request.
	RedirectURL *url.URL
}

type HTTPProxyHandler struct {
	Config HTTPConfig

	// Transport is the http connection pool configured with timeouts.
	// The proxy will panic if this value is nil.
	Transport http.RoundTripper

	// InsecureTransport is the http connection pool configured with
	// InsecureSkipVerify set. This is used for https proxies with
	// self-signed certs.
	InsecureTransport http.RoundTripper

	// Lookup returns a target host for the given request.
	// The proxy will panic if this value is nil.
	Lookup func(*http.Request) *RouteTarget

	// UUID returns a unique id in uuid format.
	// If UUID is nil, uuid.NewUUID() is used.
	UUID func() string
}

func NewHTTPProxyHandle(cfg HTTPConfig, lookup func(*http.Request) *RouteTarget) http.Handler {
	return &HTTPProxyHandler{
		Config:            cfg,
		Transport:         newHTTPTransport(nil, cfg),
		InsecureTransport: newHTTPTransport(&tls.Config{InsecureSkipVerify: true}, cfg),
		Lookup:            lookup,
	}
}

func (p *HTTPProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.Lookup == nil {
		panic("no lookup function")
		return
	}

	log.Printf("[proxy] http/https req: %v, method:%v", r.URL.Path, r.Method)

	if p.Config.RequestID != "" {
		id := p.UUID
		if id == nil {
			id = uuid.NewUUID
		}
		r.Header.Set(p.Config.RequestID, id())
	}

	t := p.Lookup(r)

	if t == nil {
		status := p.Config.NoRouteStatus
		if status < 100 || status > 999 {
			status = http.StatusNotFound
		}
		w.WriteHeader(status)
		html := p.Config.NoRouteHTML
		if html != "" {
			io.WriteString(w, html)
		}
		return
	}

	// build the request url since r.URL will get modified
	// by the reverse proxy and contains only the RequestURI anyway
	requestURL := &url.URL{
		Scheme:   scheme(r),
		Host:     r.Host,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}
	_ = requestURL
	log.Println("[proxy] requestURL:", requestURL)

	if t.RedirectCode != 0 && t.RedirectURL != nil {
		http.Redirect(w, r, t.RedirectURL.String(), t.RedirectCode)
		return
	}

	// build the real target url that is passed to the proxy
	targetURL := &url.URL{
		Scheme: t.URL.Scheme,
		Host:   t.URL.Host,
		Path:   r.URL.Path,
	}
	if t.URL.RawQuery == "" || r.URL.RawQuery == "" {
		targetURL.RawQuery = t.URL.RawQuery + r.URL.RawQuery
	} else {
		targetURL.RawQuery = t.URL.RawQuery + "&" + r.URL.RawQuery
	}
	log.Println("[proxy] targetURL:", targetURL)

	if t.Host == "dst" {
		r.Host = targetURL.Host
	} else if t.Host != "" {
		r.Host = t.Host
	}

	// TODO(fs): The HasPrefix check seems redundant since the lookup function should
	// TODO(fs): have found the target based on the prefix but there may be other
	// TODO(fs): matchers which may have different rules. I'll keep this for
	// TODO(fs): a defensive approach.
	if t.StripPath != "" && strings.HasPrefix(r.URL.Path, t.StripPath) {
		targetURL.Path = targetURL.Path[len(t.StripPath):]
	}

	if err := addHeaders(r, p.Config, t.StripPath); err != nil {
		http.Error(w, "cannot parse "+r.RemoteAddr, http.StatusInternalServerError)
		return
	}

	if err := addResponseHeaders(w, r, p.Config); err != nil {
		http.Error(w, "cannot add response headers", http.StatusInternalServerError)
		return
	}

	upgrade, accept := r.Header.Get("Upgrade"), r.Header.Get("Accept")

	tr := p.Transport
	if t.TLSSkipVerify {
		tr = p.InsecureTransport
	}

	var h http.Handler
	switch {
	case upgrade == "websocket" || upgrade == "Websocket":
		r.URL = targetURL
		if targetURL.Scheme == "https" || targetURL.Scheme == "wss" {
			h = newWSHandler(targetURL.Host, func(network, address string) (net.Conn, error) {
				return tls.Dial(network, address, tr.(*http.Transport).TLSClientConfig)
			})
		} else {
			h = newWSHandler(targetURL.Host, net.Dial)
		}
	case accept == "text/event-stream":
		// use the flush interval for SSE (server-sent events)
		// must be > 0s to be effective
		h = newHTTPProxy(targetURL, tr, p.Config.FlushInterval)
	default:
		h = newHTTPProxy(targetURL, tr, p.Config.GlobalFlushInterval)
	}

	if p.Config.GZIPContentTypes != nil {
		h = gziph.NewGzipHandler(h, p.Config.GZIPContentTypes)
	}

	log.Println("[proxy] http proxy begin")
	rw := &responseWriter{w: w}
	h.ServeHTTP(rw, r)
	log.Println("[proxy] http proxy ret=", rw.code)
	if rw.code <= 0 {
		return
	}
}

// responseWriter wraps an http.ResponseWriter to capture the status code and
// the size of the response. It also implements http.Hijacker to forward
// hijacking the connection to the wrapped writer if supported.
type responseWriter struct {
	w    http.ResponseWriter
	code int
	size int
}

func (rw *responseWriter) Header() http.Header {
	return rw.w.Header()
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.w.Write(b)
	rw.size += n
	return n, err
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.w.WriteHeader(statusCode)
	rw.code = statusCode
}

var errNoHijacker = errors.New("not a hijacker")

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.w.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errNoHijacker
}
