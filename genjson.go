// Package genjson allows for json encoding and decoding. Unlike the standard library, genjson works
// by first deserializing a byte slice into a Value type. This is less efficient, but allows for
// perfectly describing json data without having any compromises for go specific implementation
// details.
package genjson

import (
	"fmt"
	"strconv"
)

type (
	// Value describes a json value. It is only implemented by types in this package. Picture it
	// as a set type from other languages.
	Value interface {
		isValue()
		append(*Serializer, int, []byte) []byte
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

var (
	_ Value = Null{}
	_ Value = Bool(false)
	_ Value = Number{}
	_ Value = String("")
	_ Value = Array(nil)
	_ Value = Object(nil)
)

func (b Bool) GoString() string { return "genjson.Bool{" + strconv.FormatBool(bool(b)) + "}" }

func (n Number) GoString() string {
	if n.IsFloat {
		return fmt.Sprintf("genjson.Number{%v})", n.Float)
	}
	return fmt.Sprintf("genjson.Number{%v})", n.Integer)
}

func (s String) GoString() string {
	return fmt.Sprintf("genjson.String{%#v})", string(s))
}
