package main

import (
	"fmt"
	"os"

	"github.com/mattpgray/go-genjson"
)

func main() {
	f := os.Args[1]
	data, err := os.ReadFile(f)
	if err != nil {
		fmt.Printf("ERROR: Could not open file %q, %v", f, err)
	}
	js, err := genjson.Deserialize(data)
	if err != nil {
		fmt.Printf("ERROR: %v", err)
	}
	s := genjson.Serializer{
		Indent:      4,
		KeyValueGap: true,
		SortKeys:    true,
	}
	data2 := s.Serialize(js)
	if err != nil {
		fmt.Printf("ERROR: %v", err)
	}
	fmt.Printf("%s\n", data2)
}
