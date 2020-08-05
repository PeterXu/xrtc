package webrtc

import (
	"fmt"
	"net"

	log "github.com/PeterXu/xrtc/util"
	"github.com/oschwald/geoip2-golang"
)

var gGeoDB *geoip2.Reader

func init() {
	db, err := geoip2.Open(kGeoLite2File)
	if err == nil {
		gGeoDB = db
	} else {
		log.Warnln(err)
		fmt.Println()
	}
}

// The src is client which exists in anywhere.
// The dst is server which deployed in data-center.
// The mid is proxy which deployed in data-center.
//  return false: default and optimal connection: src->dst
//  return true: change and optimal connections: src->mid->dst
func checkGeoOptimal(srcIP, midIP, dstIP string) bool {
	db := gGeoDB
	if db == nil {
		return false
	}
	srcRd, err := db.City(net.ParseIP(srcIP))
	if err != nil {
		return false
	}
	srcCN := srcRd.Country.Names["en"]

	midRd, err := db.City(net.ParseIP(midIP))
	if err != nil {
		return false
	}
	midCN := midRd.Country.Names["en"]

	dstRd, err := db.City(net.ParseIP(dstIP))
	if err != nil {
		return false
	}
	dstCN := dstRd.Country.Names["en"]

	log.Println("[geoip]", srcCN, midCN, dstCN)

	if len(srcCN) == 0 || len(dstCN) == 0 {
		// maybe src or dst is local ip.
		// That means: src euqal-to mid, or mid euqal-to dst.
		// Donot need to change.
		return false
	}

	if srcCN != dstCN && srcCN == midCN {
		// a) different country between src and dst,
		// b) the same country for src and mid,
		// And the connection of (mid-dst) works better than (src-dst) in general.
		// Then change from (src -> dst) to (src -> mid -> dst).
		return true
	}

	return false
}

func checkGeoHops(srcIP, midIP, dstIP string) bool {
	return false
}
