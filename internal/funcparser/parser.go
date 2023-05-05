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

func Lazy[P ~func(I) (I, O, R), I Input, O Output, R Result](f func() P) P {
	return func(i I) (I, O, R) {
		return f()(i)
	}
}
