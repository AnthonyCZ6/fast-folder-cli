// Package humanize da formato legible a números para mostrarlos en pantalla.
package humanize

import (
	"strconv"
	"strings"
	"time"
)

// Int formatea un entero con separador de miles: 52341 -> "52,341".
func Int(n int64) string {
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	if n < 0 {
		// Se separa el signo de los dígitos en lugar de formatear -n, que
		// desborda con math.MinInt64.
		b.WriteByte('-')
		s = s[1:]
	}
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Count devuelve n junto al sustantivo en singular o plural:
// Count(1, "carpeta", "carpetas") -> "1 carpeta".
func Count(n int64, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return Int(n) + " " + many
}

// Bytes formatea un tamaño con la unidad más adecuada, en múltiplos de 1024
// como el Explorador de Windows: 1536 -> "1.5 KB", 157286400 -> "150 MB".
func Bytes(n int64) string {
	if n < 1024 {
		return Count(n, "byte", "bytes")
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	v := float64(n) / 1024
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	decimals := 1
	if v >= 100 {
		decimals = 0
	}
	return strconv.FormatFloat(v, 'f', decimals, 64) + " " + units[i]
}

// Ago describe cuándo fue t respecto a now, como se diría en una
// conversación: "hace un momento", "hace 5 min", "hace 3 h" (si fue hoy),
// "ayer a las 18:20", "hace 4 días" (en la última semana) o "el 12/09/2026".
func Ago(t, now time.Time) string {
	if t.IsZero() {
		return "fecha desconocida"
	}
	t = t.In(now.Location())
	d := now.Sub(t)
	days := calendarDays(t, now)
	switch {
	case d < time.Minute: // también si t es posterior a now (reloj adelantado)
		return "hace un momento"
	case d < time.Hour:
		return "hace " + strconv.Itoa(int(d.Minutes())) + " min"
	case days == 0:
		return "hace " + strconv.Itoa(int(d.Hours())) + " h"
	case days == 1:
		return "ayer a las " + t.Format("15:04")
	case days < 7:
		return "hace " + strconv.Itoa(days) + " días"
	}
	return "el " + t.Format("02/01/2006")
}

// calendarDays cuenta los cambios de día entre t y now (0 si son del mismo
// día), sin que importen las horas.
func calendarDays(t, now time.Time) int {
	y1, m1, d1 := t.Date()
	y2, m2, d2 := now.Date()
	a := time.Date(y1, m1, d1, 0, 0, 0, 0, time.UTC)
	b := time.Date(y2, m2, d2, 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours() / 24)
}
