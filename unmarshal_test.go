package genjson

import (
	"math"
	"reflect"
	"testing"
)

type (
	tb bool
	ci int64
	cu uint64
)

type unmarshalTest[V any] struct {
	name    string
	value   Value
	want    V
	wantErr bool
}

func (ut unmarshalTest[V]) i() iUnmarshalTest {
	var v V
	return iUnmarshalTest{
		name:    ut.name,
		value:   ut.value,
		in:      &v,
		want:    &ut.want,
		wantErr: ut.wantErr,
	}
}

type iUnmarshalTest struct {
	name    string
	value   Value
	in      any
	want    any
	wantErr bool
}

func TestUnmarshal(t *testing.T) {
	tests := []iUnmarshalTest{
		unmarshalTest[bool]{
			name:  "bool-true",
			value: Bool(true),
			want:  true,
		}.i(),
		unmarshalTest[bool]{
			name:  "bool-false",
			value: Bool(false),
			want:  false,
		}.i(),
		unmarshalTest[tb]{
			name:  "custom-bool",
			value: Bool(true),
			want:  true,
		}.i(),
		unmarshalTest[int64]{
			name:  "positive-int64",
			value: integer(1),
			want:  1,
		}.i(),
		unmarshalTest[int64]{
			name:  "positive-int64-valid-float-value",
			value: float(1.0),
			want:  1,
		}.i(),
		unmarshalTest[int64]{
			name:    "positive-int64-fractional-float-value",
			value:   float(1.1),
			want:    0,
			wantErr: true,
		}.i(),
		unmarshalTest[int64]{
			name:    "positive-int64-overflow-float-value",
			value:   float(1e200),
			want:    0,
			wantErr: true,
		}.i(),
		unmarshalTest[int32]{
			name:    "positive-int32-overflow-int-value",
			value:   integer(math.MaxUint64),
			want:    0,
			wantErr: true,
		}.i(),
		unmarshalTest[ci]{
			name:  "custom-int64",
			value: integer(1),
			want:  1,
		}.i(),
		unmarshalTest[uint64]{
			name:  "uint64",
			value: integer(1),
			want:  1,
		}.i(),
		unmarshalTest[uint64]{
			name:    "uint64-fractional-float-value",
			value:   float(1.1),
			want:    0,
			wantErr: true,
		}.i(),
		unmarshalTest[uint32]{
			name:    "uint32-overflow",
			value:   integer(math.MaxUint64),
			want:    0,
			wantErr: true,
		}.i(),
		unmarshalTest[cu]{
			name:  "custom-uint64",
			value: integer(1),
			want:  1,
		}.i(),
		unmarshalTest[[]int]{
			name:  "slice",
			value: Array([]Value{integer(1), integer(2)}),
			want:  []int{1, 2},
		}.i(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("%#v", tt.value)
			data := Serialize(tt.value)
			t.Logf("%s", data)
			err := Unmarshal(data, tt.in)
			t.Logf("%v", err)
			if err != nil != (tt.wantErr) {
				t.Errorf("unexpected error %v", err)
			}
			if !reflect.DeepEqual(tt.want, tt.in) {
				t.Errorf("unexpected result %+v != %+v", indirect(tt.want), indirect(tt.in))
			}
		})
	}
}

func indirect(v any) any {
	return reflect.ValueOf(v).Elem().Interface()
}
