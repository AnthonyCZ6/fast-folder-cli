package humanize

import (
	"math"
	"testing"
	"time"
)

func TestIntExtremes(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{math.MaxInt64, "9,223,372,036,854,775,807"},
		// Cambiar el signo de MinInt64 desborda: no puede formatearse como -(-n).
		{math.MinInt64, "-9,223,372,036,854,775,808"},
	}
	for _, tt := range tests {
		if got := Int(tt.n); got != tt.want {
			t.Errorf("Int(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

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

func TestBytes(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 bytes"},
		{1, "1 byte"},
		{1023, "1,023 bytes"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{10 << 20, "10.0 MB"},
		{150 << 20, "150 MB"},
		{3584 << 20, "3.5 GB"},
		{2 << 40, "2.0 TB"},
	}
	for _, tt := range tests {
		if got := Bytes(tt.n); got != tt.want {
			t.Errorf("Bytes(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestAgo(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 30, 0, 0, time.Local)
	tests := []struct {
		t    time.Time
		want string
	}{
		{time.Time{}, "fecha desconocida"},
		{now.Add(10 * time.Second), "hace un momento"}, // reloj adelantado
		{now.Add(-30 * time.Second), "hace un momento"},
		{now.Add(-5 * time.Minute), "hace 5 min"},
		{now.Add(-3 * time.Hour), "hace 3 h"},
		{time.Date(2026, 10, 4, 18, 20, 0, 0, time.Local), "ayer a las 18:20"},
		{time.Date(2026, 10, 4, 23, 59, 0, 0, time.Local), "ayer a las 23:59"}, // menos de 24 h
		{time.Date(2026, 10, 1, 9, 0, 0, 0, time.Local), "hace 4 días"},
		{time.Date(2026, 9, 12, 9, 0, 0, 0, time.Local), "el 12/09/2026"},
	}
	for _, tt := range tests {
		if got := Ago(tt.t, now); got != tt.want {
			t.Errorf("Ago(%v) = %q, want %q", tt.t, got, tt.want)
		}
	}
}
