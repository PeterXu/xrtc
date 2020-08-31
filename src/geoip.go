package webrtc

import (
	"errors"
	"fmt"
	"net"

	"github.com/PeterXu/xrtc/log"
	"github.com/oschwald/geoip2-golang"
)

var gGeoDB *geoip2.Reader

func initGeoLite(fname string) {
	if gGeoDB == nil {
		log.Println("[GEO] init geolite:", fname)
		if db, err := geoip2.Open(fname); err == nil {
			gGeoDB = db
		} else {
			log.Warnln(err)
			fmt.Println()
		}
	}
}

type GeoLocation struct {
	attrs map[string]string
}

func NewGeoLocation() *GeoLocation {
	return &GeoLocation{
		attrs: make(map[string]string),
	}
}

func (loc *GeoLocation) Add(key, value string) {
	loc.attrs[key] = value
}

func (loc *GeoLocation) Get(key string) string {
	if val, ok := loc.attrs[key]; ok {
		return val
	}
	return ""
}

func (loc *GeoLocation) String() string {
	var szline string
	if loc.attrs != nil {
		for key, value := range loc.attrs {
			attr := fmt.Sprintf("%s=%s", key, value)
			if len(szline) > 0 {
				szline = szline + ";"
			}
			szline = szline + attr
		}
	}
	return szline
}

func parseGeoLocation(ipstr string) (*GeoLocation, error) {
	if db := gGeoDB; db != nil {
		if rd, err := db.City(net.ParseIP(ipstr)); err == nil {
			ct, _ := rd.City.Names["en"]
			cn, _ := rd.Country.Names["en"]
			loc := NewGeoLocation()
			loc.Add("city", ct)
			loc.Add("country", cn)
			return loc, nil
		} else {
			return nil, err
		}
	}
	return nil, errors.New("no geo db")
}

// The src is client which exists in anywhere.
// The dst is server which deployed in data-center.
// The mid is proxy which deployed in data-center.
//  return false: default and optimal connection: src->dst
//  return true: change and optimal connections: src->mid->dst
func checkGeoOptimal(srcIP, proxyIP, dstIP string) bool {
	srcLoc, err := parseGeoLocation(srcIP)
	if err != nil {
		return false
	}

	proxyLoc, err := parseGeoLocation(proxyIP)
	if err != nil {
		return false
	}

	dstLoc, err := parseGeoLocation(dstIP)
	if err != nil {
		return false
	}

	log.Println("[geoip]", srcLoc, proxyLoc, dstLoc)
	srcCN := srcLoc.Get("country")
	proxyCN := proxyLoc.Get("country")
	dstCN := dstLoc.Get("country")

	if len(srcCN) == 0 || len(dstCN) == 0 {
		// maybe src or dst is local ip.
		// That means: src euqal-to mid, or mid euqal-to dst.
		// Donot need to change.
		return false
	}

	if srcCN != dstCN && srcCN == proxyCN {
		// a) different country between src and dst,
		// b) the same country for src and mid,
		// And the connection of (mid-dst) works better than (src-dst) in general.
		// Then change from (src -> dst) to (src -> mid -> dst).
		return true
	}

	return false
}

func checkGeoHops(srcIP, proxyIP, dstIP string) bool {
	return false
}
