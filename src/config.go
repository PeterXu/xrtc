package webrtc

import (
	"errors"
	"fmt"

	"github.com/PeterXu/xrtc/log"
	"github.com/PeterXu/xrtc/util"
	"github.com/PeterXu/xrtc/yaml"
)

const (
	uTAG                = "[CONFIG]"
	kDefaultServerName  = "_"
	kDefaultServerRoot  = "/tmp"
	kDefaultConfig      = "/tmp/etc/routes.yml"
	kDefaultCrtFile     = "/tmp/etc/cert.pem"
	kDefaultKeyFile     = "/tmp/etc/cert.key"
	kDefaultLogPath     = "/var/log/xrtc"
	kDefaultGeoLiteFile = "/tmp/etc/GeoLite2-City.mmdb"

	// marks
	kRoutePublicHostMark = "route_public_host_ip"
	kRouteInitAddrMark   = "route_init_addr"
	kCandidateHostMark   = "candidate_host_ip"
)

/// load config parameters.
func LoadConfig(fname string) *Config {
	config := NewConfig()
	if !config.Load(fname) {
		log.Fatal(uTAG, "read config failed:", fname)
		return nil
	}
	return config
}

/// hase config

type CommonConfig struct {
	Id          string
	Name        string
	CrtFile     string
	KeyFile     string
	GeoLiteFile string
	LogPath     string
}

func NewCommonConfig() *CommonConfig {
	return &CommonConfig{}
}

func (c *CommonConfig) Load(service yaml.Map) error {
	c.Id = getYamlStringEx(service, "id", util.SysUniqueId())
	c.Name = getYamlStringEx(service, "name", util.SysHostname())
	c.CrtFile = getYamlStringEx(service, "crt_file", kDefaultCrtFile)
	c.KeyFile = getYamlStringEx(service, "key_file", kDefaultKeyFile)
	c.GeoLiteFile = getYamlStringEx(service, "geolite_file", kDefaultGeoLiteFile)
	c.LogPath = getYamlStringEx(service, "log_path", kDefaultLogPath)
	return nil
}

/// mod config

type ModConfig struct {
	Common *CommonConfig
	Mod    string
	Name   string
	Addrs  []string        // "proto://host:port"
	Route  *RouteNetParams // for srt/..
	Ice    *IceNetParams   // for udp/tcp
	Rest   *RestNetParams  // for http/ws
}

func NewModConfig(mod string) *ModConfig {
	return &ModConfig{
		Mod: mod,
	}
}

func (mc *ModConfig) Load(service yaml.Map) error {
	mc.Name = getYamlString(service, "name")
	if len(mc.Name) == 0 {
		return errors.New("no mod name")
	}
	mc.Addrs = getYamlListString(service, "addrs")
	if len(mc.Addrs) == 0 {
		return errors.New("no mod addrs")
	}

	var err error
	switch mc.Mod {
	case "route":
		mc.Route = &RouteNetParams{
			Location: NewGeoLocation(),
		}
		err = mc.Route.Load(service, mc.Addrs)
	case "ice":
		mc.Ice = &IceNetParams{}
		err = mc.Ice.Load(service, mc.Addrs)
	case "rest":
		mc.Rest = &RestNetParams{}
		err = mc.Rest.Load(service)
	default:
		err = errors.New("unsupported mod=" + mc.Mod)
	}
	return err
}

func (mc ModConfig) String() string {
	mod := fmt.Sprintf("{Name: %s, Mod: %s, Addrs: %v}",
		mc.Name, mc.Mod, mc.Addrs)
	var extend string
	switch mc.Mod {
	case "route":
		extend = fmt.Sprintf(" Route{%s}", mc.Route)
	case "ice":
		extend = fmt.Sprintf(" Ice{%s}", mc.Ice)
	case "rest":
		extend = fmt.Sprintf(" Rest{%s}", mc.Rest)
	}
	return mod + extend
}

/// Config

type Config struct {
	common  *CommonConfig
	configs []*ModConfig
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
		if services, err = getYamlMap(root, "services"); err != nil {
			log.Error(uTAG, "check services, err=", err)
			return false
		}
	}

	// Check services
	for _, mod := range yaml.Keys(services) {
		service, err := getYamlMap(services, mod)
		if err != nil {
			log.Warn2f(uTAG, "check service [%s], err=%v", mod, err)
			return false
		}

		log.Print2f(uTAG, ">>>parse service mod [%s]", mod)

		if mod == "common" {
			cfg := NewCommonConfig()
			if err := cfg.Load(service); err != nil {
				log.Warn2f(uTAG, "check service [%s], err=%v", mod, err)
				return false
			}
			c.common = cfg
			continue
		}

		cfg := NewModConfig(mod)
		if err := cfg.Load(service); err != nil {
			log.Warn2f(uTAG, "check service mod [%s], err=[%v]", mod, err)
			return false
		}

		c.configs = append(c.configs, cfg)
	}
	fmt.Println()

	if c.common != nil {
		log.Println(uTAG, "detail:", c.common)
	}
	for _, cfg := range c.configs {
		log.Println(uTAG, "detail:", cfg)
		cfg.Common = c.common
		fmt.Println()
	}
	fmt.Println()

	return true
}

// Net params

type RouteNetParams struct {
	Location     *GeoLocation
	Capacity     uint32
	_PublicHosts []string // "host"
	PublicAddrs  []string // TODO: "proto://host:port"
	_InitAddrs   []string //
	InitAddrs    []string // TODO: "proto://host:port"
}

type IceNetParams struct {
	_CandidateHosts []string
	Candidates      []string // TODO:ice candidates
}

type RestNetParams struct {
	Servername string // server name
	Root       string // static root dir
	RequestID  string
}

var kDefaultHttpParams = RestNetParams{
	RequestID: "X-Request-Id",
}

// load the "route:" parameters
func (n *RouteNetParams) Load(service yaml.Map, addrs []string) error {
	node, err := getYamlMap(service, "route")
	if err != nil {
		return err
	}
	loc := getYamlString(node, "location")
	if len(loc) > 0 {
		n.Location.MergeFrom(loc)
		log.Println(uTAG, "location:", loc, n.Location)
	}
	n.Capacity = uint32(yaml.ToInt(node.Key("capacity"), 0))

	n._InitAddrs = getYamlListString(node, "init_addrs")
	if len(n._InitAddrs) == 0 {
		return errors.New("no init_addrs")
	}
	for _, val := range n._InitAddrs {
		addr := val
		if val == kRouteInitAddrMark {
			szip := util.LocalIPString()
			addr = fmt.Sprintf("srt://%s:9528", szip)
		}
		n.InitAddrs = append(n.InitAddrs, addr)
		log.Println(uTAG, "init addr: ", val, addr)
	}

	n._PublicHosts = getYamlListString(node, "public_hosts")
	if len(n._PublicHosts) == 0 {
		return errors.New("no public_hosts")
	}
	for _, val := range n._PublicHosts {
		var szip string
		if val == kRoutePublicHostMark {
			szip = util.LocalIPString()
		} else {
			szip = util.LookupIP(val)
		}
		for _, addr := range addrs {
			proto, _, port := util.ParseUriAll(addr)
			uri := proto + "://" + szip
			if port != 0 {
				uri = uri + ":" + util.Itoa(port)
			}
			log.Println(uTAG, "pub addr: ", val, uri)
			n.PublicAddrs = append(n.PublicAddrs, uri)
		}
	}
	return nil
}

// load the "ice:" parameters
func (n *IceNetParams) Load(service yaml.Map, addrs []string) error {
	node, err := getYamlMap(service, "ice")
	if err != nil {
		return err
	}
	n._CandidateHosts = getYamlListString(node, "candidate_hosts")
	if len(n._CandidateHosts) == 0 {
		return errors.New("no candidate_hosts")
	}
	for idx, val := range n._CandidateHosts {
		var szip string
		if val == kCandidateHostMark {
			szip = util.LocalIPString()
		} else {
			szip = util.LookupIP(val)
		}
		log.Println(uTAG, "net candidate host: ", val, szip)
		for _, addr := range addrs {
			proto, _, port := util.ParseUriAll(addr)
			if port == 0 {
				return errors.New("wrong net addr:" + addr)
			}
			var candidate string
			switch proto {
			case "udp":
				candidate = fmt.Sprintf("a=candidate:%d 1 udp 2013266431 %s %d typ host",
					(idx + 1), szip, port)
			case "tcp":
				candidate = fmt.Sprintf("a=candidate:%d 1 tcp 1010827775 %s %d typ host tcptype passive",
					(idx + 1), szip, port)
			default:
				return errors.New("wrong proto=" + proto)
			}
			if len(candidate) > 0 {
				n.Candidates = append(n.Candidates, candidate)
			}
		}
	}
	return nil
}

// loads the "rest:" parameters
func (h *RestNetParams) Load(service yaml.Map) error {
	node, err := getYamlMap(service, "rest")
	if err != nil {
		return err
	}

	h.Servername = getYamlStringEx(node, "servername", kDefaultServerName)
	h.Root = getYamlStringEx(node, "root", kDefaultServerRoot)
	return nil
}

/// misc tools

func getYamlMap(node yaml.Map, key string) (yaml.Map, error) {
	return yaml.ToMap(node.Key(key))
}

func getYamlString(node yaml.Map, key string) string {
	return yaml.ToString(node.Key(key))
}

func getYamlStringEx(node yaml.Map, key, value string) string {
	tmp := getYamlString(node, key)
	if len(tmp) == 0 {
		tmp = value
	}
	return tmp
}

func getYamlListString(node yaml.Map, key string) []string {
	var results []string
	if items, err := yaml.ToList(node.Key(key)); err == nil {
		for _, item := range items {
			tmp := yaml.ToString(item)
			if len(tmp) > 0 {
				results = append(results, tmp)
			}
		}
	}
	return results
}
