package kratosapi

import "testing"

func TestBoundedInt32(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int32
	}{
		{name: "negative", input: -1, want: 0},
		{name: "normal", input: 42, want: 42},
		{name: "maximum", input: 2147483647, want: 2147483647},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := boundedInt32(test.input); got != test.want {
				t.Fatalf("boundedInt32(%d) = %d, want %d", test.input, got, test.want)
			}
		})
	}
}
