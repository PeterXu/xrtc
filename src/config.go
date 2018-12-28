package webrtc

import (
	"fmt"
	"net"
	"strings"

	"github.com/PeterXu/xrtc/util"
	log "github.com/PeterXu/xrtc/util"
	"github.com/PeterXu/xrtc/yaml"
)

const (
	uTAG               = "[CONFIG]"
	kDefaultServerName = "_"
	kDefaultServerRoot = "/tmp"
	kDefaultConfig     = "/tmp/etc/routes.yml"
	kTlsCrtFile        = "/tmp/etc/cert.pem"
	kTlsKeyFile        = "/tmp/etc/cert.key"
	kGeoLite2File      = "/tmp/etc/GeoLite2-City.mmdb"
	kCandidateIpMark   = "candidate_host_ip"
)

// Config contains all services(udp/tcp/http)
type Config struct {
	Servers []*NetConfig
}

func NewConfig() *Config {
	return &Config{}
}

// Load loads all service from config file.
func (c *Config) Load(fname string) bool {
	ycfg, err := yaml.ReadFile(fname)
	if err != nil {
		log.Error(uTAG, "read failed, err=", err)
		return false
	}

	var services yaml.Map

	// Check root and services
	if root, err := yaml.ToMap(ycfg.Root); err != nil {
		log.Error(uTAG, "check root, err=", err)
		return false
	} else {
		if services, err = yaml.ToMap(root.Key("services")); err != nil {
			log.Error(uTAG, "check services, err=", err)
			return false
		}
	}

	// Check services
	for _, key := range yaml.Keys(services) {
		service, err := yaml.ToMap(services.Key(key))
		if err != nil {
			log.Warn(uTAG, "check service [", key, "], err=", err)
			continue
		}

		var proto yaml.Scalar
		if proto, err = yaml.ToScalar(service.Key("proto")); err != nil {
			log.Warn(uTAG, "check service proto, err=", err)
			continue
		}

		log.Println(uTAG, "parse service:", key, ", proto:", proto)

		var netp yaml.Map
		if netp, err = yaml.ToMap(service.Key("net")); err != nil {
			log.Warn(uTAG, "check service net, err=", err)
			continue
		}

		netProto := strings.ToLower(proto.String())
		switch netProto {
		case "udp":
			udpsvr := NewUDPConfig(key, netp)
			c.Servers = append(c.Servers, udpsvr)
		case "tcp":
			tcpsvr := NewTCPConfig(key, netp)
			enableHttp := yaml.ToString(service.Key("enable_http"))
			//log.Println(uTAG, "check tcp's enable_http=", enableHttp)
			tcpsvr.EnableHttp = (enableHttp == "true")
			if tcpsvr.EnableHttp {
				if httpp, err := yaml.ToMap(service.Key("http")); err == nil {
					tcpsvr.Http.Load(httpp)
				}
			}
			c.Servers = append(c.Servers, tcpsvr)
		case "http":
			httpsvr := NewHTTPConfig(key, netp)
			enableHttp := yaml.ToString(service.Key("enable_http"))
			httpsvr.EnableHttp = (enableHttp == "true") // desperated
			if httpp, err := yaml.ToMap(service.Key("http")); err == nil {
				httpsvr.Http.Load(httpp)
			}
			c.Servers = append(c.Servers, httpsvr)
		default:
			log.Warn(uTAG, "unsupported proto=", proto)
		}
		fmt.Println()
	}
	fmt.Println()

	return true
}

// Net basic params
type NetParams struct {
	Addr       string   // "host:port"
	TlsCrtFile string   // crt file
	TlsKeyFile string   // key file
	EnableIce  bool     // enable ice
	Candidates []string // ice candidates(check EnableIce)
}

// Load the "net:" parameters under one service.
func (n *NetParams) Load(node yaml.Map, proto string) {
	n.Addr = yaml.ToString(node.Key("addr"))
	n.TlsCrtFile = yaml.ToString(node.Key("tls_crt_file"))
	n.TlsKeyFile = yaml.ToString(node.Key("tls_key_file"))

	n.EnableIce = (yaml.ToString(node.Key("enable_ice")) == "true")
	for n.EnableIce {
		var port string
		var err error
		if _, port, err = net.SplitHostPort(n.Addr); err != nil {
			log.Warnln(uTAG, "wrong net addr:", n.Addr, err)
			break
		}
		var ips yaml.List
		if ips, err = yaml.ToList(node.Key("candidate_ips")); err != nil {
			break
		}
		for idx, ip := range ips {
			szip0 := yaml.ToString(ip)
			if len(szip0) == 0 {
				continue
			}
			var szip string
			if szip0 == kCandidateIpMark {
				szip = util.LocalIPString()
			} else {
				szip = util.LookupIP(szip0)
			}
			log.Println(uTAG, "net candidate_ip: ", szip0, szip)

			var candidate string
			if proto == "udp" {
				candidate = fmt.Sprintf("a=candidate:%d 1 udp 2013266431 %s %s typ host",
					(idx + 1), szip, port)
			} else if proto == "tcp" {
				candidate = fmt.Sprintf("a=candidate:%d 1 tcp 1010827775 %s %s typ host tcptype passive",
					(idx + 1), szip, port)
			} else {
				continue
			}
			n.Candidates = append(n.Candidates, candidate)
		}
		break
	}
	log.Println(uTAG, "net params:", n)
}

/// net config

type NetConfig struct {
	Name       string
	Proto      string
	Net        NetParams
	EnableHttp bool       // for tcp/http
	Http       HttpParams // for tcp/http
}

func NewUDPConfig(name string, netp yaml.Map) *NetConfig {
	cfg := &NetConfig{Name: name, Proto: "udp"}
	cfg.Net.Load(netp, cfg.Proto)
	return cfg
}

func NewTCPConfig(name string, netp yaml.Map) *NetConfig {
	cfg := &NetConfig{Name: name, Proto: "tcp"}
	cfg.Net.Load(netp, cfg.Proto)
	cfg.Http = kDefaultHttpParams
	return cfg
}

func NewHTTPConfig(name string, netp yaml.Map) *NetConfig {
	cfg := &NetConfig{Name: name, Proto: "http"}
	cfg.Net.Load(netp, cfg.Proto)
	cfg.Http = kDefaultHttpParams
	return cfg
}

/// HttpParams

type HttpParams struct {
	Servername string // server name
	Root       string // static root dir
	RequestID  string
}

var kDefaultHttpParams = HttpParams{
	RequestID: "X-Request-Id",
}

// Load loads the http parameters(routes/..) under a service.
func (h *HttpParams) Load(node yaml.Map) {
	h.Servername = yaml.ToString(node.Key("servername"))
	if len(h.Servername) == 0 {
		h.Servername = kDefaultServerName
	}

	h.Root = yaml.ToString(node.Key("root"))
	if len(h.Root) == 0 {
		h.Root = kDefaultServerRoot
	}

	log.Println(uTAG, "http parameters:", h)
}
