package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mattpgray/go-genjson"
)

func main() {
	var (
		indent   = flag.Int("indent", 4, "The indent of the json. If 0, there will be not newlines in the output.")
		prefix   = flag.Int("prefix", 0, "The prefix of the json. This can be useful if the output json is being injected into another json file.")
		keyGap   = flag.Bool("key-gap", true, "Whether to include a space between keys and values in objects.")
		sortKeys = flag.Bool("sort-keys", true, "Whether to sort keys in the output json")
	)
	flag.Parse()
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
		Indent:      *indent,
		KeyValueGap: *keyGap,
		SortKeys:    *sortKeys,
		Prefix:      *prefix,
	}
	data2 := s.Serialize(js)
	fmt.Printf("%s\n", data2)
}
