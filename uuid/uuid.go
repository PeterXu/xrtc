// Package uuid allows to generate uuid with string.
//
// Based-on https://github.com/fabiolb/fabio/proxy/uuid.
package uuid

var generator = MustNewGenerator()

// NewUUID return UUID in string fromat
func NewUUID() string {
	return ToString(generator.Next())
}
