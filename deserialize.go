package genjson

import (
	"errors"
	"strconv"
	"unicode"

	. "github.com/mattpgray/go-genjson/internal/funcparser"
)

func Deserialize(b []byte) (Value, error) {
	_, v, ok := jsonParser()(b)
	if !ok.Valid() {
		return nil, errors.New("could not deserialize json")
	}

	return v, nil
}

type empty struct{}

type parser[V any] Parser[[]byte, V, *BoolResult]

func jsonParser() parser[Value] {
	return trimSpaceParser(
		Try(
			nullParser(),
			boolParser(),
			numberParser(),
			jsonStringParser(),
			arrayParser(),
			objectParser(),
		),
	)
}

func objectParser() parser[Value] {
	type keyValue struct {
		key   string
		value Value
	}

	elemParser := Map(
		Chain(
			surroundParser[keyValue]()(
				Map(
					stringParser(),
					func(s string) keyValue { return keyValue{key: s} },
				),
			)(
				discardParser(trimSpaceParser(byteParser(':'))),
			),
			Map(
				trimSpaceParser(LazyP(jsonParser)),
				func(value Value) keyValue {
					return keyValue{value: value}
				},
			),
		),
		func(kvs []keyValue) keyValue {
			return keyValue{
				key:   kvs[0].key,
				value: kvs[1].value,
			}
		},
	)
	return Validate(
		compositeParser(
			discardParser(byteParser('{')),
			discardParser(trimSpaceParser(byteParser('}'))),
			discardParser(trimSpaceParser(byteParser(','))),
			trimSpaceParser(elemParser),
		),
		func(kvs []keyValue) (Value, *BoolResult) {
			m := map[string]Value{}
			for _, kv := range kvs {
				if _, ok := m[kv.key]; ok {
					return nil, OK(false)
				}
				m[kv.key] = kv.value
			}
			return Object(m), OK(true)
		},
	)
}

func compositeParser[V any](start, end, sep parser[empty], elem parser[V]) parser[[]V] {
	return surroundParser[[]V](
		start,
	)(
		listParser(
			elem,
			sep,
			end,
		),
	)()
}

func arrayParser() parser[Value] {
	return Map(
		compositeParser(
			discardParser(byteParser('[')),
			discardParser(trimSpaceParser(byteParser(']'))),
			discardParser(trimSpaceParser(byteParser(','))),
			LazyP(jsonParser),
		),
		func(val []Value) Value {
			return Array(val)
		},
	)
}

func listParser[V any](p parser[V], sep parser[empty], endParser parser[empty]) parser[[]V] {
	return func(bb []byte) ([]byte, []V, *BoolResult) {
		var vs []V
		bb2, _, ok := endParser(bb)
		if ok.Valid() {
			return bb2, vs, ok
		}
		bb2, v, ok := p(bb2)
		if !ok.Valid() {
			return bb, nil, OK(false)
		}
		vs = append(vs, v)

		for {
			bb3, _, ok := endParser(bb2)
			if ok.Valid() {
				return bb3, vs, ok
			}
			bb3, _, ok = sep(bb2)
			if !ok.Valid() {
				return bb, nil, ok
			}
			bb3, v, ok := p(bb3)
			if !ok.Valid() {
				return bb, nil, ok
			}

			vs = append(vs, v)
			bb2 = bb3
		}
	}
}

func trimSpaceParser[V any](p parser[V]) parser[V] {
	return func(bb []byte) ([]byte, V, *BoolResult) {
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
	return func(bb []byte) ([]byte, empty, *BoolResult) {
		bb, _, ok := p(bb)
		return bb, empty{}, ok
	}
}

func surroundParser[V any](before ...parser[empty]) func(p parser[V]) func(after ...parser[empty]) parser[V] {
	return func(p parser[V]) func(after ...parser[empty]) parser[V] {
		return func(after ...parser[empty]) parser[V] {
			return func(bb []byte) ([]byte, V, *BoolResult) {
				bb2, _, ok := ChainP(before...)(bb)
				if !ok.Valid() {
					var v2 V
					return bb, v2, ok
				}
				bb2, v2, ok := p(bb2)
				if !ok.Valid() {
					var v2 V
					return bb, v2, ok
				}
				bb2, _, ok = ChainP(after...)(bb2)
				if !ok.Valid() {
					var v2 V
					return bb, v2, ok
				}
				return bb2, v2, OK(true)
			}
		}
	}
}

func jsonStringParser() parser[Value] {
	return Map(
		stringParser(),
		func(s string) Value {
			return String(s)
		},
	)
}

func stringParser() parser[string] {
	return Validate(
		Flatten(
			Chain(byteParser('"')),
			func(bb []byte) ([]byte, []byte, *BoolResult) {
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
						return bb[i+1:], bb[:i+1], OK(true)
					}
				}
				return bb, nil, OK(false)
			},
		),
		func(b []byte) (string, *BoolResult) {
			s, err := strconv.Unquote(string(b))
			if err != nil {
				return "", OK(false)
			}
			return s, OK(true)
		},
	)
}

func numberParser() parser[Value] {
	return Try(
		Map(floatParser(), func(i float64) Value { return Number{Float: i, IsFloat: true} }),
		Map(intParser(), func(i int64) Value { return Number{Integer: i} }),
	)
}

func floatParser() parser[float64] {
	floatBytesParser := Flatten(
		digitsParser(),
		Chain(byteParser('.')),
		digitsParser(),
	)
	return func(bb []byte) ([]byte, float64, *BoolResult) {
		floatBytes, bb2, ok := floatBytesParser(bb)
		if !ok.Valid() {
			return bb, 0, ok
		}
		f, err := strconv.ParseFloat(string(floatBytes), 64)
		if err != nil {
			return bb, 0, OK(false)
		}
		return bb2, f, OK(true)
	}
}

func intParser() parser[int64] {
	digitsParser := predicateParser(func(b byte) bool {
		return b >= '0' && b <= '9'
	})
	return func(bb []byte) ([]byte, int64, *BoolResult) {
		intBytes, bb2, ok := digitsParser(bb)
		if !ok.Valid() {
			return bb, 0, ok
		}
		i, err := strconv.ParseInt(string(intBytes), 10, 64)
		if err != nil {
			return bb, 0, OK(false)
		}
		return bb2, i, OK(true)
	}
}

func digitsParser() parser[[]byte] {
	return predicateParser(func(b byte) bool {
		return b >= '0' && b <= '9'
	})
}

func predicateParser(predicate func(b byte) bool) parser[[]byte] {
	return func(bb []byte) ([]byte, []byte, *BoolResult) {
		for i := range bb {
			if !predicate(bb[i]) {
				ret := bb[:i]
				return ret, bb[i:], OK(len(ret) > 0)
			}
		}
		return bb, nil, OK(true)
	}
}

func nullParser() parser[Value] {
	return Map(
		Chain(
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
	return Map(
		Try(
			Chain(
				byteParser('t'),
				byteParser('r'),
				byteParser('u'),
				byteParser('e'),
			),
			Chain(
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

func byteParser(b byte) parser[byte] {
	return func(bb []byte) ([]byte, byte, *BoolResult) {
		if len(bb) > 0 && bb[0] == b {
			return bb[1:], b, OK(true)
		}
		return bb, 0, OK(false)
	}
}
