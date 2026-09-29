package search

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Matcher compara nombres de carpeta con un término de búsqueda, sin
// distinguir mayúsculas de minúsculas (igual que el sistema de archivos de
// Windows).
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
	pattern := strings.ToLower(strings.TrimSpace(term))
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
	name = strings.ToLower(name)
	if m.glob {
		ok, _ := filepath.Match(m.pattern, name)
		return ok
	}
	return strings.Contains(name, m.pattern)
}
