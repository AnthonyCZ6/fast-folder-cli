package search

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Matcher compara nombres de carpeta con un término de búsqueda, sin
// distinguir mayúsculas de minúsculas (igual que el sistema de archivos de
// Windows) ni acentos: "cancion" coincide con "Canción" y "ano" con "Año".
//
//   - Sin comodines, el término se busca como subcadena: "proy" coincide con
//     "Proyectos" y con "mi-proyecto".
//   - Con comodines (* ? [..]) el patrón debe cubrir el nombre completo:
//     "proy*" coincide con "Proyectos" pero no con "mi-proyecto".
type Matcher struct {
	pattern string
	glob    bool
}

// NewMatcher crea un Matcher y valida la sintaxis del patrón.
func NewMatcher(term string) (*Matcher, error) {
	pattern := fold(strings.TrimSpace(term))
	if pattern == "" {
		return nil, errors.New("el término de búsqueda está vacío")
	}

	glob := strings.ContainsAny(pattern, "*?[")
	if glob {
		if _, err := filepath.Match(pattern, ""); err != nil {
			return nil, fmt.Errorf("patrón inválido %q: %w", term, err)
		}
	}
	return &Matcher{pattern: pattern, glob: glob}, nil
}

// Match indica si name coincide con el término de búsqueda.
func (m *Matcher) Match(name string) bool {
	name = fold(name)
	if m.glob {
		ok, _ := filepath.Match(m.pattern, name)
		return ok
	}
	return strings.Contains(name, m.pattern)
}

// Fold normaliza un nombre de carpeta como lo compara la búsqueda: en
// minúsculas y sin acentos. Sirve para comparar nombres fuera del recorrido
// (por ejemplo, las carpetas excluidas de las carpetas recientes).
func Fold(s string) string { return fold(s) }

// fold pasa s a minúsculas y le quita los acentos, la diéresis y la tilde de
// la ñ, para comparar nombres como lo haría una persona al escribir rápido.
func fold(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return strings.ToLower(s)
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		// Marcas diacríticas combinadas: aparecen en nombres guardados en
		// forma descompuesta (por ejemplo, copiados desde macOS).
		if r >= 0x300 && r <= 0x36f {
			continue
		}
		if base, ok := accents[r]; ok {
			r = base
		}
		b.WriteRune(r)
	}
	return b.String()
}

// accents asocia cada letra minúscula acentuada con su letra base.
var accents = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ä': 'a', 'ã': 'a', 'å': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'ö': 'o', 'õ': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ñ': 'n', 'ç': 'c', 'ý': 'y', 'ÿ': 'y',
}
