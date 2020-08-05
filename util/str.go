package util

func SubString(str string, pos int, size int) string {
	if pos >= len(str) {
		return ""
	}
	s := []byte(str)[pos:]
	if len(s) <= size {
		return string(s)
	}
	return string(s[:size])
}

// like c++ std::pair
type StringPair struct {
	First  string
	Second string
}

type IntPair struct {
	First  int
	Second int
}

func (sp StringPair) ToStringBySpace() string {
	return sp.First + " " + sp.Second
}
