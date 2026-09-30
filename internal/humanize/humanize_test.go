package humanize

import "testing"

func TestInt(t *testing.T) {
	tests := map[int64]string{0: "0", 7: "7", 999: "999", 1000: "1,000", 52341: "52,341", 1234567: "1,234,567", -4200: "-4,200"}
	for in, want := range tests {
		if got := Int(in); got != want {
			t.Errorf("Int(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 carpetas"},
		{1, "1 carpeta"},
		{2, "2 carpetas"},
		{12345, "12,345 carpetas"},
	}
	for _, tt := range tests {
		if got := Count(tt.n, "carpeta", "carpetas"); got != tt.want {
			t.Errorf("Count(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
