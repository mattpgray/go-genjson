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
