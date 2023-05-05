package main

import (
	"fmt"

	"github.com/mattpgray/go-genjson"
)

func main() {
	tests := [][]byte{
		[]byte(`null`),
		[]byte(`true`),
		[]byte(`false`),
		[]byte(`123`),
		[]byte(`0123`),
		[]byte(`01221344423452345234523456345634567456745673`),
		[]byte(`1.0`),
		[]byte(`0.12`),
		[]byte(`12.3`),
		[]byte(`"asdf"`),
		[]byte(`"\""`),
		[]byte(`   123  `),
		[]byte(`[]`),
		[]byte(`[ 1 ]`),
		[]byte(`[ 1 , 3 ]`),
		[]byte(`[ 1 , ]`),
		[]byte(`[ , 1 ]`),
		[]byte(`[ 1 `),
		[]byte(`{ 1 }`),
		[]byte(`{ 1: 1 }`),
		[]byte(`{ "key": "value" }`),
		[]byte(`{}`),
		[]byte(`{ "key": "value", "key2": "value2"}`),
		[]byte(`{ "key" : "value" , "key2" : { "key" : "value" } }`),
	}
	for _, tt := range tests {
		v, err := genjson.Deserialize(tt)
		if err != nil {
			fmt.Printf("ERROR %q: %v\n", tt, err)
		} else {
			fmt.Printf("%q => %#v\n", tt, v)
		}
	}
}
