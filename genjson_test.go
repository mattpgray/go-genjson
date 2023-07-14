package genjson

import (
	"bytes"
	_ "embed"
	"testing"
)

//go:embed testdata/test.json
var testData []byte

func TestRoundTrip(t *testing.T) {
	tes := bytes.TrimSpace(testData)
	v, err := Deserialize(testData)
	if err != nil {
		t.Fatalf("unexpected error during deserialization %v", err)
	}
	s := Serializer{
		Indent:      2,
		KeyValueGap: 1,
	}
	data := s.Serialize(v)
	if !bytes.Equal(data, tes) {
		t.Errorf("json round trip error %q != %q", tes, data)
	}
}
