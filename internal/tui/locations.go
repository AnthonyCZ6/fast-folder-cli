package tui

import (
	"os"
	"strings"
)

// location es una carpeta raíz que se puede elegir con las flechas.
type location struct {
	Label string
	Path  string
}

// defaultLocations devuelve las ubicaciones ofrecidas en el formulario: las
// carpetas habituales del usuario, la carpeta actual y el resto (AppData,
// unidades...). Se omiten las que no existen y las repetidas.
func defaultLocations() []location {
	primary, secondary := platformLocations()
	all := primary
	if wd, err := os.Getwd(); err == nil {
		all = append(all, location{Label: "Carpeta actual", Path: wd})
	}
	all = append(all, secondary...)

	seen := make(map[string]bool)
	var result []location
	for _, loc := range all {
		key := strings.ToLower(strings.TrimRight(loc.Path, `\/`))
		if loc.Path == "" || seen[key] {
			continue
		}
		if info, err := os.Stat(loc.Path); err != nil || !info.IsDir() {
			continue
		}
		seen[key] = true
		result = append(result, loc)
	}
	return result
}
