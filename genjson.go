// Package genjson allows for json encoding and decoding. Unlike the standard library, genjson works
// by first deserializing a byte slice into a Value type. This is less efficient, but allows for
// perfectly describing json data without having any compromises for go specific implementation
// details.
package genjson

import (
	"errors"
	"strconv"
	"unicode"
)

type (
	// Value describes a json value. It is only implemented by types in this package. Picture it
	// as a set type from other languages.
	Value interface {
		isValue()
	}

	Null   struct{}
	Bool   bool
	Number struct {
		Float   float64
		Integer int64
		IsFloat bool
	}
	String string
	Array  []Value
	Object map[string]Value
)

func (Null) isValue()   {}
func (Bool) isValue()   {}
func (Number) isValue() {}
func (String) isValue() {}
func (Array) isValue()  {}
func (Object) isValue() {}

func Deserialize(b []byte) (Value, error) {
	v, _, ok := jsonParser()(b)
	if !ok {
		return nil, errors.New("could not deserialize json")
	}

	return v, nil
}

type empty struct{}

type parser[V any] func(bb []byte) (V, []byte, bool)

func jsonParser() parser[Value] {
	return trimSpaceParser(
		tryParsers(
			nullParser(),
			boolParser(),
			numberParser(),
			jsonStringParser(),
		),
	)
}

func trimSpaceParser[V any](p parser[V]) parser[V] {
	return func(bb []byte) (V, []byte, bool) {
		for i := range bb {
			if !unicode.IsSpace(rune(bb[i])) {
				bb = bb[i:]
				break
			}
		}
		return p(bb)
	}
}

func discardParser[V any](p parser[V]) parser[empty] {
	return func(bb []byte) (empty, []byte, bool) {
		_, bb, ok := p(bb)
		return empty{}, bb, ok
	}
}

func surroundParser[V any](before ...parser[empty]) func(p parser[V]) func(after ...parser[empty]) parser[V] {
	return func(p parser[V]) func(after ...parser[empty]) parser[V] {
		return func(after ...parser[empty]) parser[V] {
			return func(bb []byte) (V, []byte, bool) {
				_, bb2, ok := chainParsers(before...)(bb)
				if !ok {
					var v2 V
					return v2, bb, false
				}
				v2, bb2, ok := p(bb2)
				if !ok {
					var v2 V
					return v2, bb, false
				}
				_, bb2, ok = chainParsers(after...)(bb2)
				if !ok {
					var v2 V
					return v2, bb, false
				}
				return v2, bb2, true
			}
		}
	}
}

func jsonStringParser() parser[Value] {
	return mapParser(
		stringParser(),
		func(s string) Value {
			return String(s)
		},
	)
}

func stringParser() parser[string] {
	return parseParser(
		flattenParser(
			chainParsers(byteParser('"')),
			func(bb []byte) ([]byte, []byte, bool) {
				inEscape := false
				for i := range bb {
					if inEscape {
						inEscape = false
						continue
					}
					switch bb[i] {
					case '\\':
						inEscape = true
					case '"':
						return bb[:i+1], bb[i+1:], true
					}
				}
				return nil, bb, false
			},
		),
		func(b []byte) (string, bool) {
			s, err := strconv.Unquote(string(b))
			if err != nil {
				return "", false
			}
			return s, true
		},
	)
}

func flattenParser[V any](parsers ...parser[[]V]) parser[[]V] {
	p := chainParsers(parsers...)
	return func(bb []byte) ([]V, []byte, bool) {
		res, bb, ok := p(bb)
		if !ok {
			return nil, bb, false
		}
		cap := 0
		for _, r := range res {
			cap += len(r)
		}
		out := make([]V, 0, cap)
		for _, r := range res {
			out = append(out, r...)
		}
		return out, bb, true
	}
}

func numberParser() parser[Value] {
	return tryParsers(
		mapParser(floatParser(), func(i float64) Value { return Number{Float: i, IsFloat: true} }),
		mapParser(intParser(), func(i int64) Value { return Number{Integer: i} }),
	)
}

func floatParser() parser[float64] {
	floatBytesParser := flattenParser(
		digitsParser(),
		chainParsers(byteParser('.')),
		digitsParser(),
	)
	return func(bb []byte) (float64, []byte, bool) {
		floatBytes, bb2, ok := floatBytesParser(bb)
		if !ok {
			return 0, bb, false
		}
		f, err := strconv.ParseFloat(string(floatBytes), 64)
		if err != nil {
			return 0, bb, false
		}
		return f, bb2, true
	}
}

func intParser() parser[int64] {
	digitsParser := predicateParser(func(b byte) bool {
		return b >= '0' && b <= '9'
	})
	return func(bb []byte) (int64, []byte, bool) {
		intBytes, bb2, ok := digitsParser(bb)
		if !ok {
			return 0, bb, false
		}
		i, err := strconv.ParseInt(string(intBytes), 10, 64)
		if err != nil {
			return 0, bb, false
		}
		return i, bb2, true
	}
}

func digitsParser() parser[[]byte] {
	return predicateParser(func(b byte) bool {
		return b >= '0' && b <= '9'
	})
}

func predicateParser(predicate func(b byte) bool) parser[[]byte] {
	return func(bb []byte) ([]byte, []byte, bool) {
		for i := range bb {
			if !predicate(bb[i]) {
				ret := bb[:i]
				return ret, bb[i:], len(ret) > 0
			}
		}
		return bb, nil, true
	}
}

func nullParser() parser[Value] {
	return mapParser(
		chainParsers(
			byteParser('n'),
			byteParser('u'),
			byteParser('l'),
			byteParser('l'),
		),
		func([]byte) Value {
			return Null{}
		},
	)
}

func boolParser() parser[Value] {
	return mapParser(
		tryParsers(
			chainParsers(
				byteParser('t'),
				byteParser('r'),
				byteParser('u'),
				byteParser('e'),
			),
			chainParsers(
				byteParser('f'),
				byteParser('a'),
				byteParser('l'),
				byteParser('s'),
				byteParser('e'),
			),
		),
		func(v []byte) Value {
			return Bool(string(v) == "true")
		},
	)
}

func tryParsers[V any](parsers ...parser[V]) parser[V] {
	return func(bb []byte) (V, []byte, bool) {
		for _, p := range parsers {
			v, bb2, ok := p(bb)
			if ok {
				return v, bb2, true
			}
		}
		var v V
		return v, bb, false
	}
}

func chainParsers[V any](parsers ...parser[V]) parser[[]V] {
	return func(bb []byte) ([]V, []byte, bool) {
		res := make([]V, 0, len(parsers))
		bb2 := bb
		for _, p := range parsers {
			v, bb3, ok := p(bb2)
			if !ok {
				return nil, bb, false
			}
			res = append(res, v)
			bb2 = bb3
		}
		return res, bb2, true
	}
}

func byteParser(b byte) parser[byte] {
	return func(bb []byte) (byte, []byte, bool) {
		if len(bb) > 0 && bb[0] == b {
			return b, bb[1:], true
		}
		return 0, bb, false
	}
}

func mapParser[V1 any, V2 any](parser parser[V1], f func(V1) V2) parser[V2] {
	return func(bb []byte) (V2, []byte, bool) {
		v1, bb, ok := parser(bb)
		if !ok {
			var v2 V2
			return v2, bb, false
		}
		return f(v1), bb, true
	}
}

func parseParser[V1 any, V2 any](parser parser[V1], f func(V1) (V2, bool)) parser[V2] {
	return func(bb []byte) (V2, []byte, bool) {
		v1, bb2, ok := parser(bb)
		if !ok {
			var v2 V2
			return v2, bb, false
		}
		v2, ok := f(v1)
		if !ok {
			var v2 V2
			return v2, bb, false
		}
		return v2, bb2, true
	}
}

type deserializer struct {
	data []byte
}
