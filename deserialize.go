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
	Row   int
	Col   int
}

func (ie InvalidTokenError) Error() string {
	return "invalid token '" + string(ie.Token) + "'"
}

func Deserialize(b []byte) (Value, error) {
	d := deserializer{
		b:   b,
		idx: 0,
		row: 1,
		col: 1,
	}
	_, v, er := jsonParserE()(d)
	if er.Err != nil {
		return nil, er.Err
	}

	return v, nil
}

type deserializer struct {
	b   []byte
	idx int
	row int
	col int
}

func read(d deserializer) (deserializer, byte, *BoolResult) {
	if d.idx < len(d.b) {
		b := d.b[d.idx]
		row := d.row
		col := d.col
		if b == '\n' {
			row++
			col = 1
		} else {
			col++
		}
		return deserializer{
			b:   d.b,
			idx: d.idx + 1,
			row: row,
			col: col,
		}, b, OK(true)
	}
	return d, 0, OK(false)
}

type (
	parser[I Input, R Result] Parser[deserializer, I, R]

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
			func(d deserializer, r *CombineResult) *ErrResult {
				if r.Valid() {
					return Err(nil)
				}
				if r.Err != nil {
					return r.ToE()
				}
				return Err(errNoMatch(d))
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
	return func(d deserializer) (deserializer, []V, *CombineResult) {
		var vs []V
		bb2, _, br := endParser(d)
		if br.Valid() {
			return bb2, vs, COK(true)
		}
		bb2, v, cr := p(bb2)
		if !cr.Valid() {
			return d, nil, cr
		}
		vs = append(vs, v)

		for {
			bb3, _, br := endParser(bb2)
			if br.Valid() {
				return bb3, vs, COK(true)
			}
			bb3, _, br = sep(bb2)
			if !br.Valid() {
				return d, nil, CErr(errNoMatch(bb3))
			}
			// An invalid match here is
			bb3, v, cr := p(bb3)
			if cr.Err != nil {
				return d, nil, cr
			}
			// An invalid match here is still an error
			if !cr.OK {
				return d, nil, CErr(errNoMatch(bb3))
			}

			vs = append(vs, v)
			bb2 = bb3
		}
	}
}

func trimSpaceParser[V any, R Result](p parser[V, R]) parser[V, R] {
	return func(d deserializer) (deserializer, V, R) {
		for {
			d2, b, br := read(d)
			if !br.OK {
				break
			}
			if !unicode.IsSpace(rune(b)) {
				break
			}
			d = d2
		}
		return p(d)
	}
}

func surroundParser[V any](before ...parser[Empty, *BoolResult]) func(p parser[V, *CombineResult]) func(after ...parser[Empty, *CombineResult]) parser[V, *CombineResult] {
	return func(p parser[V, *CombineResult]) func(after ...parser[Empty, *CombineResult]) parser[V, *CombineResult] {
		return func(after ...parser[Empty, *CombineResult]) parser[V, *CombineResult] {
			return func(d deserializer) (deserializer, V, *CombineResult) {
				bb2, _, ok := ChainP(before...)(d)
				if !ok.Valid() {
					var v2 V
					return d, v2, COK(ok.OK)
				}
				bb2, v2, cr := p(bb2)
				if !cr.Valid() {
					var v2 V
					return d, v2, cr
				}
				bb2, _, cr = ChainP(after...)(bb2)
				if !cr.Valid() {
					var v2 V
					return d, v2, cr
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
			func(d deserializer) (deserializer, []byte, *CombineResult) {
				var buf []byte
				inEscape := false
				for {
					var (
						b  byte
						br *BoolResult
					)
					d, b, br = read(d)
					if !br.OK {
						return d, nil, CErr(ErrUnmatchedQuote)
					}
					buf = append(buf, b)
					if inEscape {
						inEscape = false
						continue
					}
					switch b {
					case '\\':
						inEscape = true
					case '"':
						return d, buf, COK(true)
					}
				}
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
	return func(d deserializer) (deserializer, []byte, *BoolResult) {
		var buf []byte
		for {
			d2, b, br := read(d)
			if br.OK && predicate(b) {
				buf = append(buf, b)
				d = d2
			} else {
				return d, buf, OK(len(buf) > 0)
			}
		}
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
	return func(d deserializer) (deserializer, byte, *BoolResult) {
		if d2, bb, br := read(d); br.OK && bb == b {
			return d2, b, OK(true)
		}
		return d, 0, OK(false)
	}
}

func errBoolResult(d deserializer, br *BoolResult) *CombineResult {
	if br.OK {
		return COK(true)
	}
	return CErr(errNoMatch(d))
}

func errNoMatch(d deserializer) error {
	_, b, br := read(d)
	if !br.OK {
		return ErrUnexpectedEndOfInput
	}
	return InvalidTokenError{Token: b, Row: d.row, Col: d.col}
}
