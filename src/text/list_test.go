package text

import (
	"reflect"
	"testing"
)

func TestList(t *testing.T) {
	a, b := Text("Alice"), Text("Bob")
	sep := Text(", ").Colored(Gray)

	cases := []struct {
		parts []Component
		want  Component
	}{
		{nil, Component{}},
		{[]Component{a}, a},
		{[]Component{a, b}, Component{Extra: []Component{a, sep, b}}},
	}

	for _, c := range cases {
		if got := List(c.parts); !reflect.DeepEqual(got, c.want) {
			t.Errorf("List(%v) = %+v, want %+v", c.parts, got, c.want)
		}
	}

	if got := List([]Component{a, b}).PlainText(); got != "Alice, Bob" {
		t.Errorf("plain text is %q", got)
	}
}
