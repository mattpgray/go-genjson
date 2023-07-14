// Package genjson allows for json encoding and decoding. Unlike the standard library, genjson works
// by first deserializing a byte slice into a Value type. This is less efficient, but allows for
// perfectly describing json data without having any compromises for go specific implementation
// details.
package genjson

import (
	"container/list"
	"reflect"
)

type Type int8

const (
	TypeNull   Type = iota
	TypeBool   Type = iota
	TypeNumber Type = iota
	TypeString Type = iota
	TypeArray  Type = iota
	TypeObject Type = iota
)

func (t Type) String() string {
	switch t {
	case TypeNull:
		return "null"
	case TypeBool:
		return "bool"
	case TypeNumber:
		return "number"
	case TypeString:
		return "string"
	case TypeArray:
		return "array"
	case TypeObject:
		return "object"
	}
	return ""
}

type (
	// Value describes a json value. It is only implemented by types in this package. Picture it
	// as a set type from other languages.
	Value interface {
		isValue()
		append(*Serializer, int, []byte) []byte
		unmarshal(s *UnmarshalState, v reflect.Value) error
	}

	// Null represents a null json value.
	Null struct{}
	// Bool represents a boolean json value.
	Bool bool
	// Number represents a numeric json value
	Number struct {
		Float   float64
		Integer uint64
		IsFloat bool
		IsNeg   bool
	}
	// String represents a string json value.
	String string
	// Array represents an array json value.
	Array []Value
	// Object represents an object json value.
	Object struct {
		m *orderedDuplicateMap[string, Value]
	}
)

func integer(i uint64) Number {
	return Number{Integer: i}
}

func float(i float64) Number {
	return Number{Float: i, IsFloat: true}
}

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
	_ Value = Object{}
)

func (o *Object) init() {
	if o.m == nil {
		o.m = &orderedDuplicateMap[string, Value]{
			keys: list.New(),
			m:    make(map[string][]orderedDuplicateMapEntry[Value]),
		}
	}
}

// Get returns the first match of the key in the object.
func (o Object) Get(key string) (Value, bool) {
	return o.m.get(key)
}

// GetAll returns all entries matching the provided key.
func (o Object) GetAll(key string) ([]Value, bool) {
	return o.m.getAll(key)
}

// Set sets the value in the object, overwriting any previous values.
func (o *Object) Set(key string, value Value) {
	o.init()
	o.m.set(key, value)
}

// Len returns the length of the object.
func (o *Object) Len() int {
	return o.m.len()
}

// Set adds the value in the object.
func (o *Object) Add(key string, value Value) {
	o.init()
	o.m.add(key, value)
}

// Delete removes any entries matching the key from the object.
func (o Object) Delete(key string) {
	o.m.remove(key)
}

// Delete removes any entries matching the key from the object.
func (o Object) Iter() *ObjectIterator {
	return &ObjectIterator{iter: o.m.iter()}
}

type ObjectIterator struct {
	iter *orderedDuplicateMapIterator[string, Value]
}

func (o *ObjectIterator) Next() (string, Value, bool) {
	return o.iter.next()
}

type orderedDuplicateMap[K comparable, V any] struct {
	// Linked list of keys in insertion order.
	keys *list.List
	// The values of the map.
	m map[K][]orderedDuplicateMapEntry[V]
}

func (o *orderedDuplicateMap[K, V]) len() int {
	if o == nil {
		return 0
	}
	n := 0
	for _, e := range o.m {
		n += len(e)
	}
	return n
}

func (o *orderedDuplicateMap[K, V]) iter() *orderedDuplicateMapIterator[K, V] {
	iter := orderedDuplicateMapIterator[K, V]{}
	if o != nil && o.keys != nil {
		iter.e = o.keys.Front()
		iter.m = o.m
	}
	return &iter
}

func (o *orderedDuplicateMap[K, V]) getAll(k K) ([]V, bool) {
	e := o.m[k]
	if len(e) == 0 {
		return nil, false
	}
	values := make([]V, len(e))
	for i, ee := range e {
		values[i] = ee.value
	}
	return values, true
}

func (o *orderedDuplicateMap[K, V]) get(k K) (V, bool) {
	e := o.m[k]
	if len(e) == 0 {
		var empty V
		return empty, false
	}
	return e[0].value, true
}

// add appends the element to the map.
func (o *orderedDuplicateMap[K, V]) add(k K, v V) {
	o.m[k] = append(o.m[k], orderedDuplicateMapEntry[V]{
		key:   o.keys.PushBack(k),
		value: v,
	})
}

// set overwrites the element in the map.
func (o *orderedDuplicateMap[K, V]) set(k K, v V) {
	o.remove(k)
	o.add(k, v)
}

// remove removes all entries matching the key from the map
func (o *orderedDuplicateMap[K, V]) remove(k K) {
	for _, e := range o.m[k] {
		o.keys.Remove(e.key)
	}
	delete(o.m, k)
}

type orderedDuplicateMapEntry[V any] struct {
	key   *list.Element
	value V
}

type orderedDuplicateMapIterator[K comparable, V any] struct {
	e *list.Element
	m map[K][]orderedDuplicateMapEntry[V]
}

func (o *orderedDuplicateMapIterator[K, V]) next() (K, V, bool) {
	if o.e == nil {
		var emptyK K
		var emptyV V
		return emptyK, emptyV, false
	}

	key := o.e.Value.(K)
	es := o.m[key]
	for _, e := range es {
		if o.e == e.key {
			o.e = o.e.Next()
			return key, e.value, true
		}
	}
	panic("illegal map state")
}
