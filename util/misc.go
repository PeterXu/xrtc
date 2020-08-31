package util

import (
	"net"
	"os"
)

func SysHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		LogWarnln("fail to get hostname")
		hostname = RandomString(10)
	}
	return hostname
}

func SysUniqueId() string {
	hostname, err := os.Hostname()
	if err != nil {
		LogWarnln("fail to get hostname")
		hostname = RandomString(10)
	}
	var macaddr string
	if nicList, err := net.Interfaces(); err == nil {
		if len(nicList) > 0 {
			nic := nicList[0]
			for _, one := range nicList {
				if (one.Flags&net.FlagLoopback) == 0 &&
					(one.Flags&net.FlagUp) != 0 &&
					(one.Flags&net.FlagBroadcast) != 0 {
					nic = one
					break
				}
			}
			macaddr = nic.HardwareAddr.String()
		}
	}
	if len(macaddr) == 0 {
		LogWarnln("no hwmac addr")
		macaddr = RandomString(20)
	}

	return SubString(hostname, 0, 9) + "-" + SubString(macaddr, 0, 19)
}
