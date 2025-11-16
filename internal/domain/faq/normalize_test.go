package faq

import "testing"

func TestNormalizeQuestion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
	}{
		{name: "trims whitespace", in: "  Hello World  ", out: "hello world"},
		{name: "removes punctuation", in: "What's, the distance?", out: "what s the distance"},
	}

	for _, tc := range cases {
		if got := normalizeQuestion(tc.in); got != tc.out {
			t.Fatalf("%s: expected %q got %q", tc.name, tc.out, got)
		}
	}
}
