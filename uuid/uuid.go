// Package uuid allows to generate uuid with string.
//
// Based-on https://github.com/fabiolb/fabio/proxy/uuid.
package uuid

import (
	"github.com/rogpeppe/fastuuid"
)

var generator = fastuuid.MustNewGenerator()

// NewUUID return UUID in string fromat
func NewUUID() string {
	return ToString(generator.Next())
}
