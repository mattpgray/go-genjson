package genjson

import (
	"testing"
)

func TestDeserialize(t *testing.T) {
	tests := []struct {
		input   []byte
		want    Value
		wantErr bool
	}{
		{
			input:   []byte(`null`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`true`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`false`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`123`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`0123`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`01221344423452345234523456345634567456745673`),
			want:    nil,
			wantErr: true,
		},
		{
			input:   []byte(`1.0`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`0.12`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`12.3`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`"asdf"`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`"\""`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`   123  `),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`[]`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`[ 1 ]`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`[ 1 , 3 ]`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`[ 1 , ]`),
			want:    nil,
			wantErr: true,
		},
		{
			input:   []byte(`[ , 1 ]`),
			want:    nil,
			wantErr: true,
		},
		{
			input:   []byte(`[ 1 `),
			want:    nil,
			wantErr: true,
		},
		{
			input:   []byte(`{ 1 }`),
			want:    nil,
			wantErr: true,
		},
		{
			input:   []byte(`{ 1: 1 }`),
			want:    nil,
			wantErr: true,
		},
		{
			input:   []byte(`{ "key": "value" }`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`{}`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`{ "key": "value", "key2": "value2"}`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`{ "key": "value", "key": "value2"}`),
			want:    nil,
			wantErr: false,
		},
		{
			input:   []byte(`{ "key" : "value" , "key2" : { "key" : "value" } }`),
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			v, err := Deserialize(tt.input)
			if tt.wantErr != (err != nil) {
				t.Errorf("unexpected error %v", err)
			}
			t.Logf("%#v", v)
		})
	}
}
