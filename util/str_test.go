package util

import (
	"testing"
)

func TestStr_1(t *testing.T) {
	str1 := "1234567890"
	str2 := "abcdefghijklmnopqrstuvwxyz"

	if SubString(str1, 0, 5) != "12345" {
		t.Error("invalid")
	}
	if SubString(str1, 0, 11) != "1234567890" {
		t.Error("invalid")
	}
	if SubString(str1, 5, 2) != "67" {
		t.Error("invalid")
	}
	if SubString(str1, 11, 4) != "" {
		t.Error("invalid")
	}

	str3 := SubString(str1, 0, 3) + "-" + SubString(str2, 0, 3)
	if str3 != "123-abc" {
		t.Error("invalid")
	}
}
