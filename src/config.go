package webrtc

import (
	"errors"
	"fmt"

	"github.com/PeterXu/xrtc/util"
	log "github.com/PeterXu/xrtc/util"
	"github.com/PeterXu/xrtc/yaml"
)

const (
	uTAG                 = "[CONFIG]"
	kDefaultServerName   = "_"
	kDefaultServerRoot   = "/tmp"
	kDefaultConfig       = "/tmp/etc/routes.yml"
	kTlsCrtFile          = "/tmp/etc/cert.pem"
	kTlsKeyFile          = "/tmp/etc/cert.key"
	kGeoLite2File        = "/tmp/etc/GeoLite2-City.mmdb"
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
	Id      string
	Name    string
	CrtFile string
	KeyFile string
}

func NewCommonConfig() *CommonConfig {
	return &CommonConfig{}
}

func (bc *CommonConfig) Load(service yaml.Map) error {
	bc.Id = getYamlString(service, "id")
	if len(bc.Id) == 0 {
		bc.Id = util.SysUniqueId()
	}
	bc.Name = getYamlString(service, "name")
	if len(bc.Name) == 0 {
		bc.Name = util.SysHostname()
	}
	bc.CrtFile = getYamlString(service, "crt_file")
	if len(bc.CrtFile) == 0 {
		bc.CrtFile = kTlsCrtFile
	}
	bc.KeyFile = getYamlString(service, "key_file")
	if len(bc.KeyFile) == 0 {
		bc.KeyFile = kTlsKeyFile
	}
	return nil
}

func (bc CommonConfig) String() string {
	return fmt.Sprintf("Common{Id: %s, Name: %s, CrtKey: {%s,%s}}",
		bc.Id, bc.Name, bc.CrtFile, bc.KeyFile)
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
		mc.Route = &RouteNetParams{}
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
	var commonCfg *CommonConfig
	for _, mod := range yaml.Keys(services) {
		service, err := getYamlMap(services, mod)
		if err != nil {
			log.Warn2f(uTAG, "check service [%s], err=%v", mod, err)
			return false
		}

		log.Print2f(uTAG, ">>>parse service mod [%s]", mod)

		if mod == "base" {
			cfg := NewCommonConfig()
			if err := cfg.Load(service); err != nil {
				log.Warn2f(uTAG, "check service [%s], err=%v", mod, err)
				return false
			}
			commonCfg = cfg
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

	if commonCfg != nil {
		log.Println(uTAG, "detail:", commonCfg)
	}
	for _, cfg := range c.configs {
		log.Println(uTAG, "detail:", cfg)
		cfg.Common = commonCfg
		fmt.Println()
	}
	fmt.Println()

	return true
}

// Net params

type RouteNetParams struct {
	Location     string
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
	n.Location = getYamlString(node, "location")
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

	h.Servername = getYamlString(node, "servername")
	if len(h.Servername) == 0 {
		h.Servername = kDefaultServerName
	}

	h.Root = getYamlString(node, "root")
	if len(h.Root) == 0 {
		h.Root = kDefaultServerRoot
	}
	return nil
}

/// misc tools

func getYamlMap(node yaml.Map, key string) (yaml.Map, error) {
	return yaml.ToMap(node.Key(key))
}

func getYamlString(node yaml.Map, key string) string {
	return yaml.ToString(node.Key(key))
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
