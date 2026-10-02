package period

import (
	"errors"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	now := time.Date(2026, 10, 2, 15, 30, 0, 0, time.Local)
	day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.Local) }

	tests := []struct {
		input         string
		after, before time.Time
		label         string
	}{
		{"hoy", day(2), time.Time{}, "hoy"},
		{" HOY ", day(2), time.Time{}, "hoy"},
		{"ayer", day(1), day(2), "ayer"},
		{"semana", time.Date(2026, 9, 26, 0, 0, 0, 0, time.Local), time.Time{}, "en los últimos 7 días"},
		{"mes", time.Date(2026, 9, 3, 0, 0, 0, 0, time.Local), time.Time{}, "en los últimos 30 días"},
		{"Año", time.Date(2025, 10, 3, 0, 0, 0, 0, time.Local), time.Time{}, "en los últimos 365 días"},
		{"ano", time.Date(2025, 10, 3, 0, 0, 0, 0, time.Local), time.Time{}, "en los últimos 365 días"},
		{"3d", time.Date(2026, 9, 30, 0, 0, 0, 0, time.Local), time.Time{}, "en los últimos 3 días"},
		{"1d", day(2), time.Time{}, "hoy"},
		{"2026-09-01", time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local), time.Time{}, "desde el 01/09/2026"},
	}
	for _, tt := range tests {
		got, err := Parse(tt.input, now)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.input, err)
			continue
		}
		if !got.After.Equal(tt.after) || !got.Before.Equal(tt.before) || got.Label != tt.label {
			t.Errorf("Parse(%q) = {%v, %v, %q}, want {%v, %v, %q}",
				tt.input, got.After, got.Before, got.Label, tt.after, tt.before, tt.label)
		}
	}
}

func TestParseErrors(t *testing.T) {
	now := time.Now()
	for _, input := range []string{"", "mañana", "0d", "-3d", "d", "2026-13-01", "01/09/2026"} {
		if _, err := Parse(input, now); !errors.Is(err, ErrInvalid) {
			t.Errorf("Parse(%q) error = %v, want ErrInvalid", input, err)
		}
	}
}
