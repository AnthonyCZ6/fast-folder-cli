// Package period interpreta los periodos de tiempo que el usuario escribe en
// español ("hoy", "ayer", "semana", "7d", "2026-09-01") para filtrar carpetas
// por su fecha de modificación.
package period

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrInvalid es el error que devuelve Parse cuando no reconoce el periodo.
var ErrInvalid = errors.New("periodo no válido")

// Range es un intervalo [After, Before). Un extremo cero significa "sin
// límite".
type Range struct {
	After, Before time.Time
	// Label describe el periodo para completar la frase "modificadas ...":
	// "hoy", "ayer", "en los últimos 7 días", "desde el 01/09/2026".
	Label string
}

// Parse interpreta s tomando now como referencia. Acepta:
//
//   - hoy, ayer
//   - semana, mes, año (o ano): los últimos 7, 30 o 365 días, incluido hoy
//   - un número de días: 3d son hoy y los dos días anteriores
//   - una fecha AAAA-MM-DD: desde ese día (incluido) hasta ahora
func Parse(s string, now time.Time) (Range, error) {
	value := strings.ToLower(strings.TrimSpace(s))
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch value {
	case "hoy":
		return Range{After: today, Label: "hoy"}, nil
	case "ayer":
		return Range{After: today.AddDate(0, 0, -1), Before: today, Label: "ayer"}, nil
	case "semana":
		return lastDays(today, 7), nil
	case "mes":
		return lastDays(today, 30), nil
	case "año", "ano":
		return lastDays(today, 365), nil
	}

	if n, ok := strings.CutSuffix(value, "d"); ok {
		if days, err := strconv.Atoi(n); err == nil && days > 0 {
			return lastDays(today, days), nil
		}
	}
	if date, err := time.ParseInLocation("2006-01-02", value, now.Location()); err == nil {
		return Range{After: date, Label: "desde el " + date.Format("02/01/2006")}, nil
	}
	return Range{}, fmt.Errorf("%w %q: usa hoy, ayer, semana, mes, año, "+
		"un número de días (7d) o una fecha (2026-09-01)", ErrInvalid, s)
}

// lastDays devuelve el intervalo formado por hoy y los days-1 días anteriores.
func lastDays(today time.Time, days int) Range {
	if days == 1 {
		return Range{After: today, Label: "hoy"}
	}
	return Range{
		After: today.AddDate(0, 0, -(days - 1)),
		Label: fmt.Sprintf("en los últimos %d días", days),
	}
}
