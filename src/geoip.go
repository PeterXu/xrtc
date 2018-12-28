package webrtc

import (
	"net"

	log "github.com/PeterXu/xrtc/util"
	"github.com/oschwald/geoip2-golang"
)

var gGeoDB *geoip2.Reader

func init() {
	db, err := geoip2.Open(kGeoLite2File)
	if err != nil {
		log.Warnln(err)
	} else {
		log.Println("[geoip]", "load geo db success")
		gGeoDB = db
	}
}

// return false, default (srcIP->dstIP)
// return true, use (srcIP->midIP->dstIP) when midIP is more optimal than dstIP
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

	if srcCN == midCN && srcCN != dstCN {
		// cross country:
		// change from (srcCN -> dstCN) to (srcCN -> midCN -> dstCN).
		return true
	}

	return false
}
