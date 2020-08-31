package webrtc

import (
	"errors"
	"fmt"
	"net"

	log "github.com/PeterXu/xrtc/util"
	"github.com/oschwald/geoip2-golang"
)

var gGeoDB *geoip2.Reader

func init() {
	if db, err := geoip2.Open(kGeoLite2File); err == nil {
		gGeoDB = db
	} else {
		log.Warnln(err)
		fmt.Println()
	}
}

type GeoLocation struct {
	City    string
	Country string
}

func (loc *GeoLocation) String() string {
	return fmt.Sprintf("city=%s;country=%s", loc.City, loc.Country)
}

func parseGeoLocation(ipstr string) (*GeoLocation, error) {
	if db := gGeoDB; db != nil {
		if rd, err := db.City(net.ParseIP(ipstr)); err == nil {
			ct, _ := rd.City.Names["en"]
			cn, _ := rd.Country.Names["en"]
			return &GeoLocation{ct, cn}, nil
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

	if len(srcLoc.Country) == 0 || len(dstLoc.Country) == 0 {
		// maybe src or dst is local ip.
		// That means: src euqal-to mid, or mid euqal-to dst.
		// Donot need to change.
		return false
	}

	if srcLoc.Country != dstLoc.Country && srcLoc.Country == proxyLoc.Country {
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
