// Package funcparser provides helper functions for defining functional parsers.
package funcparser

type Empty struct{}

type Input interface{}

type Output interface{}

type Result interface {
	*ErrResult | *BoolResult
	Valid() bool
}

func valid[R Result]() R {
	var r R
	// unsafe casting to get around the type system.
	var validAny any
	switch any(r).(type) {
	case *BoolResult:
		validAny = OK(true)
	case *ErrResult:
		validAny = Err(nil)
	}
	return validAny.(R)
}

// Err is a helper for making valid or invalid ErrResults.
func Err(err error) *ErrResult {
	return &ErrResult{Err: err}
}

// OK is a helper for making valid or invalid BoolResults.
func OK(ok bool) *BoolResult {
	return &BoolResult{OK: ok}
}

type ErrResult struct{ Err error }

func (er *ErrResult) Valid() bool {
	return er == nil || er.Err == nil
}

type BoolResult struct {
	OK bool
}

func (br *BoolResult) Valid() bool {
	return br == nil || br.OK
}

// Parser is an abstract type that defines a function that is able to take some input, return some
// output and the remaining input and any result.
type Parser[I Input, O Output, R Result] func(I) (I, O, R)

func Lazy[I Input, O Output, R Result](f func() func(I) (I, O, R)) func(I) (I, O, R) {
	return LazyP(f)
}

func LazyP[P ~func(I) (I, O, R), I Input, O Output, R Result](f func() P) P {
	return func(i I) (I, O, R) {
		return f()(i)
	}
}

func Chain[I Input, O Output, R Result](parsers ...func(I) (I, O, R)) func(I) (I, []O, R) {
	return ChainP(parsers...)
}

func ChainP[P ~func(I) (I, O, R), I Input, O Output, R Result](parsers ...P) func(I) (I, []O, R) {
	return func(ii I) (I, []O, R) {
		res := make([]O, 0, len(parsers))
		ii2 := ii
		for _, p := range parsers {
			ii3, v, ok := p(ii2)
			if !ok.Valid() {
				return ii, nil, ok
			}
			res = append(res, v)
			ii2 = ii3
		}
		return ii2, res, valid[R]()
	}
}

func Try[I Input, O Output, R Result](parsers ...func(I) (I, O, R)) func(I) (I, O, *BoolResult) {
	return TryP(parsers...)
}

func TryP[P ~func(I) (I, O, R), I Input, O Output, R Result](parsers ...P) func(I) (I, O, *BoolResult) {
	return func(ii I) (I, O, *BoolResult) {
		for _, p := range parsers {
			ii2, v, ok := p(ii)
			if ok.Valid() {
				return ii2, v, OK(true)
			}
		}
		var o O
		return ii, o, OK(false)
	}
}

func TryP2[P2 ~func(I) (I, O, *BoolResult), P ~func(I) (I, O, R), I Input, O Output, R Result](parsers ...P) P2 {
	return TryP(parsers...)
}

func Map[I Input, O1 Output, O2 Output, R Result](parser func(I) (I, O1, R), f func(O1) O2) func(I) (I, O2, R) {
	return func(ii I) (I, O2, R) {
		ii, o1, ok := parser(ii)
		if !ok.Valid() {
			var o2 O2
			return ii, o2, ok
		}
		return ii, f(o1), ok
	}
}

func MapResult[I Input, O Output, R1 Result, R2 Result](parser func(I) (I, O, R1), f func(R1) R2) func(I) (I, O, R2) {
	return func(ii I) (I, O, R2) {
		ii, o, r := parser(ii)
		return ii, o, f(r)
	}
}

func Flatten[I Input, O Output, R Result](parsers ...func(I) (I, []O, R)) func(I) (I, []O, R) {
	return FlattenP(parsers...)
}

func FlattenP[P ~func(I) (I, []O, R), I Input, O Output, R Result](parsers ...P) P {
	return Map(
		ChainP(parsers...),
		func(oo [][]O) []O {
			cap := 0
			for _, o := range oo {
				cap += len(o)
			}
			out := make([]O, 0, cap)
			for _, o := range oo {
				out = append(out, o...)
			}
			return out
		},
	)
}

func Validate[I Input, O1 Output, O2 Output, R Result](parser func(I) (I, O1, R), f func(O1) (O2, R)) func(I) (I, O2, R) {
	return func(ii I) (I, O2, R) {
		ii2, o1, ok := parser(ii)
		if !ok.Valid() {
			var o2 O2
			return ii, o2, ok
		}
		o2, r := f(o1)
		if !r.Valid() {
			var o2 O2
			return ii, o2, r
		}
		return ii2, o2, ok
	}
}
