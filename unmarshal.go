package genjson

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

var (
	ErrInvalidValue = errors.New("supplied value must be a non-nil pointer")
	ErrCannotSet    = errors.New("cannot set supplied value for an unknown reason")
)

// TODO: This should contain the unmarsaling options. Things such as required fields, custom
// unmarshalers etc. should go here.
type Unmarshaler struct {
}

// TODO: Circular references should be disallowed as they are not valid json.
type UnmarshalState struct {
	u    *Unmarshaler
	node *node // Optional. Used for location data.
	key  []string
}

type From interface {
	FromJSON(UnmarshalState, Value) error
}

var defaultUnmarshaler Unmarshaler

func Unmarshal(data []byte, v any) error {
	return defaultUnmarshaler.Unmarshal(data, v)
}

func (u *Unmarshaler) Unmarshal(data []byte, v any) error {
	d, err := deserialize(data)
	if err != nil {
		return err
	}
	return u.unmarshal(d.value, &d.node, v)
}

func (u *Unmarshaler) UnmarshalValue(value Value, v any) error {
	return u.unmarshal(value, nil, v)
}

func (u *Unmarshaler) unmarshal(value Value, node *node, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return ErrInvalidValue
	}
	s := &UnmarshalState{
		u:    u,
		node: node,
	}
	return value.unmarshal(s, rv)
}

func unmarshal(s *UnmarshalState, value Value, v reflect.Value) error {
	if !v.CanSet() {
		return unmarshalError(s, ErrCannotSet)
	}
	return value.unmarshal(s, v)
}

func (n Null) unmarshal(s *UnmarshalState, v reflect.Value) error {
	// TODO: Allow nulls for any valid json values as a unmarshal option.
	switch v.Kind() {
	case reflect.Pointer,
		reflect.Slice,
		reflect.Array,
		reflect.Map,
		reflect.Interface:
		return nil
	default:
		return unmarshalInvalidTypeError(s, v.Type(), TypeNull)
	}
}

func (b Bool) unmarshal(s *UnmarshalState, v reflect.Value) error {
	rv := reflect.Indirect(v)
	switch rv.Kind() {
	case reflect.Bool:
		return set(rv, bool(b))
	default:
		return unmarshalInvalidTypeError(s, v.Type(), TypeBool)
	}
}

func (n Number) unmarshal(s *UnmarshalState, v reflect.Value) error {
	rv := reflect.Indirect(v)
	switch rv.Kind() {
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		// TODO: Loose casting of floats should be an unmarshaler option.
		u, err := n.looseUint64(rv.Type())
		if err != nil {
			return unmarshalError(s, err)
		}
		if u > math.MaxInt64 {
			return unmarshalError(s, overflowError(rv.Type(), n))
		}
		i := int64(u)
		if n.IsNeg {
			i = -i
		}
		if rv.OverflowInt(i) {
			return unmarshalError(s, overflowError(rv.Type(), n))
		}
		return set(rv, i)

	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		if n.IsNeg {
			return unmarshalError(s, negativeUintError(rv.Type(), n))
		}
		// TODO: Loose casting of floats should be an unmarshaler option.
		u, err := n.looseUint64(rv.Type())
		if err != nil {
			return unmarshalError(s, err)
		}
		if rv.OverflowUint(u) {
			return unmarshalError(s, overflowError(rv.Type(), n))
		}
		return set(rv, u)

	case reflect.Float32, reflect.Float64:
		f := n.float64()
		if rv.OverflowFloat(f) {
			return unmarshalError(s, overflowError(rv.Type(), n))
		}
		return set(rv, f)

	default:
		return unmarshalInvalidTypeError(s, v.Type(), TypeNumber)
	}
}

func (n Number) looseUint64(t reflect.Type) (uint64, error) {
	if !n.IsFloat {
		return n.Integer, nil
	}
	if n.Float > float64(math.MaxInt64) {
		return 0, overflowError(t, n)
	}
	u := uint64(n.Float)
	fmt.Printf("%d, %.1f\n", u, n.Float)
	if n.Float != float64(u) {
		fmt.Printf("%d != %.1f\n", u, n.Float)
		return 0, fractionalFloatError(t, n)
	}
	return u, nil
}

func (n Number) float64() float64 {
	if !n.IsFloat {
		return float64(n.Integer)
	}
	return n.Float
}

func (st String) unmarshal(s *UnmarshalState, v reflect.Value) error {
	rv := reflect.Indirect(v)
	switch rv.Kind() {
	// TODO: Byte slice (as a compiler option). Maybe as a hex string? Maybe not hard coded but
	// one the of the default custom unmarshal types, similar to how we will handle time.Time?
	case reflect.String:
		return set(rv, string(st))
	default:
		return unmarshalInvalidTypeError(s, v.Type(), TypeNull)
	}
}

func (a Array) unmarshal(s *UnmarshalState, v reflect.Value) error {
	rv := reflect.Indirect(v)
	switch rv.Kind() {
	case reflect.Slice:
		out := reflect.New(rv.Type()).Elem()
		elemType := rv.Type().Elem()
		for i, v := range a {
			elem := reflect.New(elemType).Elem()

			// new state "frame"
			ss := *s
			if s.node != nil {
				ss.node = &s.node.arrayNodes[i]
			}
			ss.key = append(cloneStrings(s.key), strconv.Itoa(i))

			if err := unmarshal(s, v, elem); err != nil {
				return err
			}

			out = reflect.Append(out, elem)
		}

		rv.Set(out)
		return nil
	case reflect.Array:
		panic("unmarshaling into arrays is not implemented yet")
	default:
		return unmarshalInvalidTypeError(s, v.Type(), TypeNull)
	}
}

func (Object) unmarshal(s *UnmarshalState, v reflect.Value) error {
	panic("not implemented")
}

// ---------------- helpers start ----------------

func set[V any](r reflect.Value, v V) error {
	r.Set(reflect.ValueOf(v).Convert(r.Type()))
	return nil
}

// ---------------- helpers end ----------------

// ---------------- errors ----------------

type UnmarshalError struct {
	Cause error
	// Field is set if the value was part of a nested field e.g. a struct or map.
	Field []string
	// Loc is set if location information is available to the Unmarshaler. This is the case if
	// Unmarshal was used.
	Loc *Loc
}

func unmarshalError(s *UnmarshalState, e error) UnmarshalError {
	var loc *Loc
	if s.node != nil {
		l := s.node.start
		loc = &l
	}
	return UnmarshalError{
		Cause: e,
		Field: cloneStrings(s.key),
		Loc:   loc,
	}
}

func (ue UnmarshalError) Error() string {
	sb := strings.Builder{}
	sb.WriteString("unmarshal error")
	if len(ue.Field) > 0 {
		sb.WriteString(" ")
		sb.WriteString(strings.Join(ue.Field, "."))
	}
	if ue.Loc != nil {
		sb.WriteString(" ")
		sb.WriteString(locString(ue.Loc))
	}
	sb.WriteString(": ")
	sb.WriteString(ue.Cause.Error())
	return sb.String()
}

func locString(l *Loc) string {
	return fmt.Sprintf("%d:%d", l.Row, l.Col)
}

type InvalidTypeError struct {
	ValueType reflect.Type
	JSONType  Type
}

func (e InvalidTypeError) Error() string {
	return fmt.Sprintf("invalid go type %s for json value of type %s", e.ValueType, e.JSONType)
}

func unmarshalInvalidTypeError(s *UnmarshalState, t reflect.Type, jt Type) UnmarshalError {
	return unmarshalError(s, InvalidTypeError{t, jt})
}

type OverflowError struct {
	ValueType reflect.Type
	Number    Number
}

func (e OverflowError) Error() string {
	return fmt.Sprintf("number %s cannot be represented by go type %s as it is would overflow",
		e.Number.append(&Serializer{}, 0, make([]byte, 0, 64)),
		e.ValueType)
}

func overflowError(t reflect.Type, number Number) OverflowError {
	return OverflowError{t, number}
}

type FractionalFloatError struct {
	ValueType reflect.Type
	Number    Number
}

func (e FractionalFloatError) Error() string {
	return fmt.Sprintf("number %s cannot be represented by go type %s as it has a fractional part",
		e.Number.append(&Serializer{}, 0, make([]byte, 0, 64)),
		e.ValueType)
}

func fractionalFloatError(t reflect.Type, number Number) FractionalFloatError {
	return FractionalFloatError{t, number}
}

type NegativeUintError struct {
	ValueType reflect.Type
	Number    Number
}

func (e NegativeUintError) Error() string {
	return fmt.Sprintf("number %s cannot be represented by go type %s as it is negative",
		e.Number.append(&Serializer{}, 0, make([]byte, 0, 64)),
		e.ValueType)
}

func negativeUintError(t reflect.Type, number Number) NegativeUintError {
	return NegativeUintError{t, number}
}

func cloneStrings(strs []string) []string {
	return append([]string{}, strs...)
}

// ---------------- errors end ----------------
