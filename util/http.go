package util

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
)

func ReadHttpRawBody(reader io.ReadCloser) ([]byte, error) {
	if body, err := ioutil.ReadAll(reader); err == nil {
		err = reader.Close()
		return body, err
	} else {
		return nil, err
	}
}

func ReadHttpBody(reader io.ReadCloser, encoding string) ([]byte, error) {
	var body []byte
	var err error

	if body, err = ReadHttpRawBody(reader); err != nil {
		return nil, err
	}

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

// scheme derives the request scheme used on the initial
// request first from headers and then from the connection
// using the following heuristic:
//
// If either X-Forwarded-Proto or Forwarded is set then use
// its value to set the other header. If both headers are
// set do not modify the protocol. If none are set derive
// the protocol from the connection.
func ParseHttpScheme(r *http.Request) string {
	xfp := r.Header.Get("X-Forwarded-Proto")
	fwd := r.Header.Get("Forwarded")
	switch {
	case xfp != "" && fwd == "":
		return xfp

	case fwd != "" && xfp == "":
		p := strings.SplitAfterN(fwd, "proto=", 2)
		if len(p) == 1 {
			break
		}
		n := strings.IndexRune(p[1], ';')
		if n >= 0 {
			return p[1][:n]
		}
		return p[1]
	}

	ws := r.Header.Get("Upgrade") == "websocket"
	switch {
	case ws && r.TLS != nil:
		return "wss"
	case ws && r.TLS == nil:
		return "ws"
	case r.TLS != nil:
		return "https"
	default:
		return "http"
	}
}

func ParseHttpPort(r *http.Request) string {
	if r == nil {
		return ""
	}
	n := strings.Index(r.Host, ":")
	if n > 0 && n < len(r.Host)-1 {
		return r.Host[n+1:]
	}
	if r.TLS != nil {
		return "443"
	}
	return "80"
}

const (
	ChromeAgent  string = "chrome"
	FirefoxAgent string = "firefox"
	SafariAgent  string = "safari"
	UnknownAgent string = "unknown"
)

// parse browser long agent to short name
func ParseHttpAgent(userAgent string) string {
	userAgent = strings.ToLower(userAgent)
	if strings.Contains(userAgent, "firefox/") {
		return FirefoxAgent
	}
	if strings.Contains(userAgent, "chrome/") {
		return ChromeAgent
	}
	return UnknownAgent
}

// Http methods

var kHttpMethodGET = []byte{0x47, 0x45, 0x54}
var kHttpMethodPOST = []byte{0x50, 0x4F, 0x53}
var kHttpMethodOPTIONS = []byte{0x4F, 0x50, 0x54}
var kHttpMethodHEAD = []byte{0x48, 0x45, 0x41}
var kHttpMethodPUT = []byte{0x50, 0x55, 0x54}
var kHttpMethodDELETE = []byte{0x44, 0x45, 0x4C}

func CheckHttpRequest(data []byte) bool {
	if len(data) != 3 {
		return false
	}
	if bytes.Compare(data, kHttpMethodGET) == 0 ||
		bytes.Compare(data, kHttpMethodPOST) == 0 ||
		bytes.Compare(data, kHttpMethodHEAD) == 0 ||
		bytes.Compare(data, kHttpMethodPUT) == 0 ||
		bytes.Compare(data, kHttpMethodDELETE) == 0 ||
		bytes.Compare(data, kHttpMethodOPTIONS) == 0 {
		return true
	} else {
		return false
	}
}

// This wraps an http.ResponseWriter to capture the status code and
// the size of the response. It also implements http.Hijacker to forward
// hijacking the connection to the wrapped writer if supported.
type HttpResponseWriter struct {
	w    http.ResponseWriter
	code int
	size int
}

func (rw *HttpResponseWriter) Header() http.Header {
	return rw.w.Header()
}

func (rw *HttpResponseWriter) Write(b []byte) (int, error) {
	n, err := rw.w.Write(b)
	rw.size += n
	return n, err
}

func (rw *HttpResponseWriter) WriteHeader(statusCode int) {
	rw.w.WriteHeader(statusCode)
	rw.code = statusCode
}

var errNoHijacker = errors.New("not a hijacker")

func (rw *HttpResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.w.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errNoHijacker
}
