// Package funcparser provides helper functions for defining functional parsers.
package funcparser

type Empty struct{}

type Input interface{}

type Output interface{}

type Result interface {
	Valid() bool
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
type Parser[I Input, O Output, R Result] func(a I) (O, I, R)

// func FlattenParser[A any, V any]

// LazyParser allows for the creation of the parser to be delayed to avoid infinite recursion.
func LazyParser[P Parser[I, O, R], I Input, O Output, R Result](f func() Parser[I, O, R]) Parser[I, O, R] {
	return func(bb I) (O, I, R) {
		return f()(bb)
	}
}
