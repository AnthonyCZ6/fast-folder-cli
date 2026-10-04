// Package humanize da formato legible a números para mostrarlos en pantalla.
package humanize

import (
	"strconv"
	"strings"
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
