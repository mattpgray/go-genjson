package main

import (
	"fmt"
	"io"
	"os"

	"github.com/mattpgray/go-genjson"
)

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Could not read from stdin %v\n", err)
		os.Exit(1)
	}
	js, err := genjson.Deserialize(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	s := genjson.Serializer{
		Indent:      4,
		KeyValueGap: true,
		SortKeys:    true,
	}
	data2 := s.Serialize(js)
	fmt.Printf("%s\n", data2)
}
