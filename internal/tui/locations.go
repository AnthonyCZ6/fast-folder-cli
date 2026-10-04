package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/config"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/pathutil"
)

// location es una carpeta raíz que se puede elegir con las flechas.
type location struct {
	Label string
	Path  string
}

// defaultLocations devuelve las ubicaciones ofrecidas en el formulario: las
// propias del archivo de configuración (custom), las carpetas habituales del
// usuario, la carpeta actual, las elegidas hace poco (recent) y el resto
// (AppData, unidades...). Se omiten las que no existen y las repetidas.
func defaultLocations(custom []config.Location, recent []string) []location {
	primary, secondary := platformLocations()
	var all []location
	for _, c := range custom {
		if path, err := pathutil.Resolve(c.Path); err == nil {
			all = append(all, location{Label: c.Name, Path: path})
		}
	}
	all = append(all, primary...)
	if wd, err := os.Getwd(); err == nil {
		all = append(all, location{Label: "Carpeta actual", Path: wd})
	}
	for _, dir := range recent {
		name := filepath.Base(dir)
		if strings.Trim(name, `\/`) == "" {
			name = dir // la raíz de una unidad
		}
		all = append(all, location{Label: "Reciente: " + name, Path: dir})
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
