package genjson

import (
	"errors"
	"fmt"
	"strconv"
	"unicode"

	. "github.com/mattpgray/go-genjson/internal/funcparser"
)

var (
	ErrUnmatchedQuote       = errors.New("unmatched quote")
	ErrUnexpectedEndOfInput = errors.New("unexpected end of input")
)

type InvalidTokenError struct {
	Token byte
	Row   int
	Col   int
}

func (ie InvalidTokenError) Error() string {
	return fmt.Sprintf("%d:%d: invalid token '%s'", ie.Row, ie.Col, string(ie.Token))
}

func Deserialize(b []byte) (Value, error) {
	d, err := deserialize(b)
	if err != nil {
		return nil, err
	}
	return d.value, nil
}

func deserialize(b []byte) (output, error) {
	d := deserializer{
		b:   b,
		idx: 0,
		row: 1,
		col: 1,
	}
	_, v, er := jsonParserE()(d)
	if er.Err != nil {
		return output{}, er.Err
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

type Loc struct {
	Row int
	Col int
}

type nodeKeyValue struct {
	key      string
	keyStart Loc
	keyEnd   Loc
	node
}

// nodes contains location and order information about the json. It does not contain the value
// itself and must be kept consistent in a separate structure.
type node struct {
	objectNodes []nodeKeyValue
	arrayNodes  []node
	start       Loc
	end         Loc
}

type (
	parser[I Input, R Result] Parser[deserializer, I, R]

	parserB[I Input] parser[I, *BoolResult]
	parserE[I Input] parser[I, *ErrResult]
	parserC[I Input] parser[I, *CombineResult]
)

type output struct {
	value Value
	node  node
}

type locV[V any] struct {
	start Loc
	end   Loc
	v     V
}

func locParser[O Output, R Result](p parser[O, R]) parser[locV[O], R] {
	return func(d deserializer) (deserializer, locV[O], R) {
		start := Loc{Row: d.row, Col: d.col}
		d, o, r := p(d)
		end := Loc{Row: d.row, Col: d.col}
		return d, locV[O]{v: o, start: start, end: end}, r
	}
}

func outputParser(p parser[Value, *CombineResult]) parserC[output] {
	return MapO(
		locParser(p),
		func(loc locV[Value]) output {
			return output{value: loc.v, node: node{start: loc.start, end: loc.end}}
		},
	)
}

func jsonParserC() parser[output, *CombineResult] {
	return trimSpaceParser(
		Try(
			nullParser(),
			boolParser(),
			numberParser(),
			stringParser(),
			arrayParser(),
			objectParser(),
		),
	)
}

func jsonParserE() parser[output, *ErrResult] {
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

func nullParser() parserC[output] {
	return outputParser(
		ToC(
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
		),
	)
}

func boolParser() parserC[output] {
	return outputParser(
		ToC(
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
		),
	)
}

func numberParser() parserC[output] {
	return outputParser(
		MapO(
			Try(
				MapO(
					surroundParser[Number](
						Discard(byteParser('-')),
					)(
						positiveNumberParser(),
					)(),
					func(n Number) Number {
						n.IsNeg = true
						return n
					},
				),
				positiveNumberParser(),
			),
			func(n Number) Value {
				return n
			},
		),
	)
}
func positiveNumberParser() parser[Number, *CombineResult] {
	return Try(
		MapO(floatParser(), func(i float64) Number { return Number{Float: i, IsFloat: true} }),
		MapO(intParser(), func(i uint64) Number { return Number{Integer: i} }),
	)
}

func stringParser() parserC[output] {
	return outputParser(
		MapO(
			rawStringParser(),
			func(s string) Value {
				return String(s)
			},
		),
	)
}

func arrayParser() parser[output, *CombineResult] {
	return MapO(
		locParser(
			compositeParser(
				Discard(byteParser('[')),
				Discard(trimSpaceParser(byteParser(']'))),
				Discard(trimSpaceParser(byteParser(','))),
				LazyP(jsonParserC),
			),
		),
		func(val locV[[]output]) output {
			var vals []Value
			var nodes []node
			for _, o := range val.v {
				vals = append(vals, o.value)
				nodes = append(nodes, o.node)
			}
			return output{
				value: Array(vals),
				node: node{
					arrayNodes: nodes,
					start:      val.start,
					end:        val.end,
				},
			}
		},
	)
}

func objectParser() parserC[output] {
	type keyValue struct {
		key   locV[string]
		value output
	}

	elemParser := MapO(
		Chain(
			surroundParser[keyValue]()(
				MapO(
					locParser(rawStringParser()),
					func(s locV[string]) keyValue { return keyValue{key: s} },
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
				func(o output) keyValue {
					return keyValue{value: o}
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
		locParser(
			compositeParser(
				Discard(byteParser('{')),
				Discard(trimSpaceParser(byteParser('}'))),
				Discard(trimSpaceParser(byteParser(','))),
				trimSpaceParser(elemParser),
			),
		),
		func(kvs locV[[]keyValue]) (output, *CombineResult) {
			var o Object
			nodes := []nodeKeyValue{}
			for _, kv := range kvs.v {
				nodes = append(nodes, nodeKeyValue{
					node:     kv.value.node,
					keyStart: kv.key.start,
					keyEnd:   kv.key.end,
				})
				o.Add(kv.key.v, kv.value.value)
			}
			return output{
				value: o,
				node: node{
					start:       kvs.start,
					end:         kvs.end,
					objectNodes: nodes,
				},
			}, COK(true)
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

func rawStringParser() parser[string, *CombineResult] {
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

func intParser() parserC[uint64] {
	return Validate(
		ToC(digitsParser()),
		func(bb []byte) (uint64, *CombineResult) {
			i, err := strconv.ParseUint(string(bb), 10, 64)
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
