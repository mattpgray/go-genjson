package genjson

import (
	"sort"
	"strconv"
	"strings"
)

func (Null) append(s *Serializer, level int, bb []byte) []byte {
	return append(bb, "null"...)
}

func (b Bool) append(s *Serializer, level int, bb []byte) []byte {
	return append(bb, strconv.FormatBool(bool(b))...)
}

func (n Number) append(s *Serializer, level int, bb []byte) []byte {
	if n.IsFloat {
		s := strconv.FormatFloat(n.Float, 'f', -1, 64)
		if !strings.Contains(s, ".") {
			s += ".0"
		}
		return append(bb, s...)
	}
	return append(bb, strconv.FormatUint(n.Integer, 10)...)
}

func (s String) append(_ *Serializer, level int, bb []byte) []byte {
	return appendString(bb, string(s))
}

func appendString(bb []byte, s string) []byte {
	return append(bb, strconv.Quote(s)...)
}

func (a Array) append(s *Serializer, level int, bb []byte) []byte {
	bb = append(bb, "["...)
	for i, v := range a {
		if i > 0 {
			bb = append(bb, ","...)
		}
		bb = appendIndent(s, level+1, bb)
		bb = v.append(s, level+1, bb)
	}
	bb = appendIndent(s, level, bb)
	return append(bb, "]"...)
}

func (o Object) append(s *Serializer, level int, bb []byte) []byte {
	bb = append(bb, "{"...)
	i := 0
	keys := make([]string, 0, len(o))
	for k := range o {
		keys = append(keys, k)
	}
	if s.SortKeys {
		sort.Strings(keys)
	}
	for _, k := range keys {
		v := o[k]
		if i > 0 {
			bb = append(bb, ","...)
		}

		i++
		bb = appendIndent(s, level+1, bb)
		bb = appendString(bb, k)
		bb = append(bb, ":"...)
		if s.KeyValueGap {
			bb = append(bb, " "...)
		}
		bb = v.append(s, level+1, bb)
	}

	bb = appendIndent(s, level, bb)
	return append(bb, "}"...)
}

func appendIndent(s *Serializer, level int, bb []byte) []byte {
	if s.Indent != 0 {
		bb = append(bb, "\n"...)
		bb = append(bb, strings.Repeat(" ", s.Prefix)...)
		bb = append(bb, strings.Repeat(" ", s.Indent*level)...)
	}
	return bb
}

type Serializer struct {
	Indent      int
	Prefix      int
	KeyValueGap bool
	SortKeys    bool
}

var defSerializer Serializer

func (s *Serializer) Serialize(v Value) []byte {
	buf := make([]byte, 0, 1024)
	buf = append(buf, strings.Repeat(" ", s.Prefix)...)
	buf = v.append(s, 0, buf)
	buf = buf[:len(buf):len(buf)]
	return buf
}

func Serialize(v Value) []byte {
	return defSerializer.Serialize(v)
}
