package yaml

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

func Atoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		buf := []byte(s)
		for k := range buf {
			if buf[k] < '0' || buf[k] > '9' {
				i, _ = strconv.Atoi(string(buf[0:k]))
				break
			}
		}
	}
	return i
}

// Keys return the key list of one yaml.Map node.
func Keys(node Map) []string {
	var keys []string
	for k, _ := range node {
		keys = append(keys, k)
	}
	return keys
}

// ToMap convert yaml.Node to yaml.Map.
func ToMap(node Node) (Map, error) {
	if m, ok := node.(Map); ok {
		return m, nil
	}
	return nil, errors.New("Not yaml.Map")
}

// ToList convert yaml.Node to yaml.List.
func ToList(node Node) (List, error) {
	if l, ok := node.(List); ok {
		return l, nil
	}
	return nil, errors.New("Not yaml.List")
}

// ToScalar convert yaml.Node to a yaml.Scalar.
func ToScalar(node Node) (Scalar, error) {
	if s, ok := node.(Scalar); ok {
		return s, nil
	}
	return "", errors.New("Not yaml.Scalar")
}

// ToString convert yaml.Node(Scalar) to a string.
func ToString(node Node) string {
	if s, err := ToScalar(node); err != nil {
		return ""
	} else {
		return strings.TrimSpace(s.String())
	}
}

// ToInt convert yaml.Node(Scalar) to int
func ToInt(node Node, defaultValue int) int {
	if val := ToString(node); len(val) != 0 {
		return Atoi(val)
	}
	return defaultValue
}

// ToDuration convert yaml.Node(Scalar) to time.Duration
func ToDuration(node Node, defaultValue time.Duration) time.Duration {
	duration := defaultValue
	if szval := ToString(node); len(szval) != 0 {
		val := 0
		field := ""
		// [0-9]+s|ms
		findDigit := false
		for _, ch := range []byte(szval) {
			if ch >= '0' && ch <= '9' {
				val = val*10 + (int(ch) - 48)
				findDigit = true
			} else {
				if !findDigit {
					break
				}
				field += string(ch)
			}
		}

		switch strings.ToLower(strings.TrimSpace(field)) {
		case "s":
			duration = time.Duration(val) * time.Second
		case "ms":
			duration = time.Duration(val) * time.Millisecond
		}
	}
	return duration
}
