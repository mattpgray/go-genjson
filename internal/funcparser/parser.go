// Package funcparser provides helper functions for defining functional parsers.
package funcparser

type Empty struct{}

type Input interface{}

type Output interface{}

type result interface {
	Valid() bool
	Fatal() bool
	toC() *CombineResult
}

type Result interface {
	*ErrResult | *BoolResult | *CombineResult
	result
}

type TryResult interface {
	*BoolResult | *CombineResult
	result
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
	case *CombineResult:
		validAny = COK(true)
	default:
		panic("unreachable")
	}
	return validAny.(R)
}

func ok[R TryResult](ok bool) R {
	var r R
	// unsafe casting to get around the type system.
	var a any
	switch any(r).(type) {
	case *BoolResult:
		a = OK(ok)
	case *CombineResult:
		a = COK(ok)
	default:
		panic("unreachable")
	}
	return a.(R)
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
	return er.Err == nil
}

func (er *ErrResult) Fatal() bool {
	return !er.Valid()
}

func (er *ErrResult) toC() *CombineResult {
	return CErr(er.Err)
}

type BoolResult struct {
	OK bool
}

func (br *BoolResult) Valid() bool {
	return br == nil || br.OK
}

func (br *BoolResult) Fatal() bool {
	return false
}

func (br *BoolResult) toC() *CombineResult {
	return COK(br.OK)
}

// TODO: The  correct name for this type.
// Combine an error and ok type for differentiating between when a parser is not valid and when
// there is an actual error.
type CombineResult struct {
	OK  bool
	Err error
}

func (cr *CombineResult) Valid() bool {
	return cr.OK && cr.Err == nil
}

func (cr *CombineResult) Fatal() bool {
	return cr.Err != nil
}

func (cr *CombineResult) toC() *CombineResult {
	return cr
}

func (cr *CombineResult) ToE() *ErrResult {
	return &ErrResult{Err: cr.Err}
}

func (cr *CombineResult) ToB() *BoolResult {
	return &BoolResult{OK: cr.OK}
}

func CErr(err error) *CombineResult {
	return &CombineResult{Err: err, OK: err == nil}
}

func COK(ok bool) *CombineResult {
	return &CombineResult{Err: nil, OK: ok}
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

func Try[I Input, O Output, R TryResult](parsers ...func(I) (I, O, R)) func(I) (I, O, R) {
	return TryP(parsers...)
}

func TryP[P ~func(I) (I, O, R), I Input, O Output, R TryResult](parsers ...P) func(I) (I, O, R) {
	return func(ii I) (I, O, R) {
		for _, p := range parsers {
			ii2, v, r := p(ii)
			if r.Fatal() {
				var o O
				return ii, o, r
			}
			if r.Valid() {
				return ii2, v, r
			}
		}
		var o O
		return ii, o, ok[R](false)
	}
}

func MapO[I Input, O1 Output, O2 Output, R Result](parser func(I) (I, O1, R), f func(O1) O2) func(I) (I, O2, R) {
	return func(ii I) (I, O2, R) {
		ii, o1, ok := parser(ii)
		if !ok.Valid() {
			var o2 O2
			return ii, o2, ok
		}
		return ii, f(o1), ok
	}
}

func MapR[I Input, O Output, R1 Result, R2 Result](parser func(I) (I, O, R1), f func(I, R1) R2) func(I) (I, O, R2) {
	return func(ii I) (I, O, R2) {
		ii, o1, r := parser(ii)
		return ii, o1, f(ii, r)
	}
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

func ToC[I Input, O Output, R Result](parser func(I) (I, O, R)) func(I) (I, O, *CombineResult) {
	return func(ii I) (I, O, *CombineResult) {
		ii, o, r := parser(ii)
		return ii, o, r.toC()
	}
}

func Discard[I Input, O Output, R Result](p func(I) (I, O, R)) func(I) (I, Empty, R) {
	return func(ii I) (I, Empty, R) {
		ii, _, ok := p(ii)
		return ii, Empty{}, ok
	}
}

func DiscardP[P2 ~func(I) (I, Empty, R), P ~func(I) (I, O, R), I Input, O Output, R Result](p P) P2 {
	return Discard(p)
}

func Left[P2 ~func(I) (I, Empty, R), P ~func(I) (I, O, R), I Input, O Output, R Result](p P) P2 {
	return Discard(p)
}

func Flatten[I Input, O Output, R Result](parsers ...func(I) (I, []O, R)) func(I) (I, []O, R) {
	return FlattenP(parsers...)
}

func FlattenP[P ~func(I) (I, []O, R), I Input, O Output, R Result](parsers ...P) P {
	return MapO(
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
