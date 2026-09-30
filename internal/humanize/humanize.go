// Package humanize da formato legible a números para mostrarlos en pantalla.
package humanize

import (
	"strconv"
	"strings"
)

// Int formatea un entero con separador de miles: 52341 -> "52,341".
func Int(n int64) string {
	if n < 0 {
		return "-" + Int(-n)
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
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
