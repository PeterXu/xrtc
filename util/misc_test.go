package util

import (
	"log"
	"testing"
)

func TestMisc_1(t *testing.T) {
	str := SysUniqueId()
	log.Println("SysUniqueId:", str)
	if len(str) == 0 {
		t.Error("invalid")
	}
}
