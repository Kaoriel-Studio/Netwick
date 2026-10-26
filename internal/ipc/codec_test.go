package ipc

import (
	"bytes"
	"strings"
	"testing"
)

type msg struct {
	A string `json:"a"`
	N int    `json:"n"`
}

func TestRoundTrip(t *testing.T) {
	var b bytes.Buffer
	if err := Write(&b, msg{A: "x", N: 3}); err != nil {
		t.Fatal(err)
	}
	var m msg
	if err := Read(&b, &m); err != nil || m.A != "x" || m.N != 3 {
		t.Fatalf("%v %+v", err, m)
	}
}

func TestRejects(t *testing.T) {
	cases := []string{
		`{"a":"x","evil":1}` + "\n",
		`{"a":"x"}{"a":"y"}` + "\n",
		`not json` + "\n",
		`{"a":"` + strings.Repeat("x", MaxMessage) + `"}` + "\n",
		"",
	}
	for _, c := range cases {
		var m msg
		if err := Read(strings.NewReader(c), &m); err == nil {
			t.Errorf("accepted %.40q", c)
		}
	}
	if err := Write(&bytes.Buffer{}, msg{A: strings.Repeat("y", MaxMessage)}); err == nil {
		t.Error("wrote oversized message")
	}
}
