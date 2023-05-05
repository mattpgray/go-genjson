package genjson

import (
	"errors"
	"strconv"
	"unicode"

	. "github.com/mattpgray/go-genjson/internal/funcparser"
)

var (
	ErrUnmatchedQuote       = errors.New("unmatched quote")
	ErrUnexpectedEndOfInput = errors.New("unexpected end of input")
	ErrDuplicateKey         = errors.New("duplicate key in json input")
)

type InvalidTokenError struct {
	Token byte
}

func (ie InvalidTokenError) Error() string {
	return "invalid token '" + string(ie.Token) + "'"
}

func Deserialize(b []byte) (Value, error) {
	_, v, er := jsonParserE()(b)
	if er.Err != nil {
		return nil, er.Err
	}

	return v, nil
}

type (
	parser[I Input, R Result] Parser[[]byte, I, R]

	parserB[I Input] parser[I, *BoolResult]
	parserE[I Input] parser[I, *ErrResult]
	parserC[I Input] parser[I, *CombineResult]
)

func jsonParserC() parser[Value, *CombineResult] {
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

func jsonParserE() parser[Value, *ErrResult] {
	return trimSpaceParser(
		MapR(
			jsonParserC(),
			func(bb []byte, r *CombineResult) *ErrResult {
				if r.Valid() {
					return Err(nil)
				}
				if r.Err != nil {
					return r.ToE()
				}
				return Err(errNoMatch(bb))
			},
		),
	)
}

func objectParser() parserC[Value] {
	type keyValue struct {
		key   string
		value Value
	}

	elemParser := MapO(
		Chain(
			surroundParser[keyValue]()(
				MapO(
					stringParser(),
					func(s string) keyValue { return keyValue{key: s} },
				),
			)(
				Discard(
					trimSpaceParser(
						MapR(byteParser(':'), errBoolResult),
					),
				),
			),
			MapO(
				ToC(LazyP(jsonParserE)),
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
			Discard(byteParser('{')),
			Discard(trimSpaceParser(byteParser('}'))),
			Discard(trimSpaceParser(byteParser(','))),
			trimSpaceParser(elemParser),
		),
		func(kvs []keyValue) (Value, *CombineResult) {
			m := map[string]Value{}
			for _, kv := range kvs {
				if _, ok := m[kv.key]; ok {
					return nil, CErr(ErrDuplicateKey)
				}
				m[kv.key] = kv.value
			}
			return Object(m), COK(true)
		},
	)
}

func compositeParser[V any](start, end, sep parser[Empty, *BoolResult], elem parser[V, *CombineResult]) parser[[]V, *CombineResult] {
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

func arrayParser() parser[Value, *CombineResult] {
	return MapO(
		compositeParser(
			Discard(byteParser('[')),
			Discard(trimSpaceParser(byteParser(']'))),
			Discard(trimSpaceParser(byteParser(','))),
			LazyP(jsonParserC),
		),
		func(val []Value) Value {
			return Array(val)
		},
	)
}

func listParser[V any](p parser[V, *CombineResult], sep, endParser parser[Empty, *BoolResult]) parser[[]V, *CombineResult] {
	return func(bb []byte) ([]byte, []V, *CombineResult) {
		var vs []V
		bb2, _, br := endParser(bb)
		if br.Valid() {
			return bb2, vs, COK(true)
		}
		bb2, v, cr := p(bb2)
		if !cr.Valid() {
			return bb, nil, cr
		}
		vs = append(vs, v)

		for {
			bb3, _, br := endParser(bb2)
			if br.Valid() {
				return bb3, vs, COK(true)
			}
			bb3, _, br = sep(bb2)
			if !br.Valid() {
				return bb, nil, CErr(errNoMatch(bb3))
			}
			// An invalid match here is
			bb3, v, cr := p(bb3)
			if cr.Err != nil {
				return bb, nil, cr
			}
			// An invalid match here is still an error
			if !cr.OK {
				return bb, nil, CErr(errNoMatch(bb3))
			}

			vs = append(vs, v)
			bb2 = bb3
		}
	}
}

func trimSpaceParser[V any, R Result](p parser[V, R]) parser[V, R] {
	return func(bb []byte) ([]byte, V, R) {
		for i := range bb {
			if !unicode.IsSpace(rune(bb[i])) {
				bb = bb[i:]
				break
			}
		}
		return p(bb)
	}
}

func surroundParser[V any](before ...parser[Empty, *BoolResult]) func(p parser[V, *CombineResult]) func(after ...parser[Empty, *CombineResult]) parser[V, *CombineResult] {
	return func(p parser[V, *CombineResult]) func(after ...parser[Empty, *CombineResult]) parser[V, *CombineResult] {
		return func(after ...parser[Empty, *CombineResult]) parser[V, *CombineResult] {
			return func(bb []byte) ([]byte, V, *CombineResult) {
				bb2, _, ok := ChainP(before...)(bb)
				if !ok.Valid() {
					var v2 V
					return bb, v2, COK(ok.OK)
				}
				bb2, v2, cr := p(bb2)
				if !cr.Valid() {
					var v2 V
					return bb, v2, cr
				}
				bb2, _, cr = ChainP(after...)(bb2)
				if !cr.Valid() {
					var v2 V
					return bb, v2, cr
				}
				return bb2, v2, COK(true)
			}
		}
	}
}

func jsonStringParser() parserC[Value] {
	return MapO(
		stringParser(),
		func(s string) Value {
			return String(s)
		},
	)
}

func stringParser() parser[string, *CombineResult] {
	return Validate(
		Flatten(
			ToC(Chain(byteParser('"'))),
			func(bb []byte) ([]byte, []byte, *CombineResult) {
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
						return bb[i+1:], bb[:i+1], COK(true)
					}
				}
				return bb, nil, CErr(ErrUnmatchedQuote)
			},
		),
		func(b []byte) (string, *CombineResult) {
			s, err := strconv.Unquote(string(b))
			if err != nil {
				return "", CErr(err)
			}
			return s, COK(true)
		},
	)
}

func numberParser() parserC[Value] {
	return Try(
		MapO(floatParser(), func(i float64) Value { return Number{Float: i, IsFloat: true} }),
		MapO(intParser(), func(i int64) Value { return Number{Integer: i} }),
	)
}

func floatParser() parserC[float64] {
	return Validate(
		ToC(
			Flatten(
				digitsParser(),
				Chain(byteParser('.')),
				digitsParser(),
			),
		),
		func(bb []byte) (float64, *CombineResult) {
			f, err := strconv.ParseFloat(string(bb), 64)
			if err != nil {
				return 0, CErr(err)
			}
			return f, COK(true)
		},
	)
}

func intParser() parserC[int64] {
	return Validate(
		ToC(digitsParser()),
		func(bb []byte) (int64, *CombineResult) {
			i, err := strconv.ParseInt(string(bb), 10, 64)
			if err != nil {
				return 0, CErr(err)
			}
			return i, COK(true)
		},
	)
}

func digitsParser() parserB[[]byte] {
	return predicateParser(func(b byte) bool {
		return b >= '0' && b <= '9'
	})
}

func predicateParser(predicate func(b byte) bool) parserB[[]byte] {
	return func(bb []byte) ([]byte, []byte, *BoolResult) {
		for i := range bb {
			if !predicate(bb[i]) {
				ret := bb[:i]
				if len(ret) > 0 {
					return bb[i:], ret, OK(len(ret) > 0)
				}
				return bb, ret, OK(false)
			}
		}
		return nil, bb, OK(true)
	}
}

func nullParser() parserC[Value] {
	return ToC(
		MapO(
			Chain(
				byteParser('n'),
				byteParser('u'),
				byteParser('l'),
				byteParser('l'),
			),
			func([]byte) Value {
				return Null{}
			},
		),
	)
}

func boolParser() parserC[Value] {
	return ToC(
		MapO(
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
		),
	)
}

func byteParser(b byte) parser[byte, *BoolResult] {
	return func(bb []byte) ([]byte, byte, *BoolResult) {
		if len(bb) > 0 && bb[0] == b {
			return bb[1:], b, OK(true)
		}
		return bb, 0, OK(false)
	}
}

func errBoolResult(ii []byte, br *BoolResult) *CombineResult {
	if br.OK {
		return COK(true)
	}
	return CErr(errNoMatch(ii))
}

func errNoMatch(ii []byte) error {
	if len(ii) == 0 {
		return ErrUnexpectedEndOfInput
	}
	return InvalidTokenError{Token: ii[0]}
}
