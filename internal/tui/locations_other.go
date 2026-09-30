//go:build !windows

package tui

import (
	"os"
	"path/filepath"
)

func platformLocations() (primary, secondary []location) {
	home, err := os.UserHomeDir()
	if err == nil {
		primary = []location{
			{Label: "Carpeta personal", Path: home},
			{Label: "Escritorio", Path: filepath.Join(home, "Desktop")},
			{Label: "Documentos", Path: filepath.Join(home, "Documents")},
			{Label: "Descargas", Path: filepath.Join(home, "Downloads")},
		}
	}
	secondary = []location{{Label: "Raíz del sistema", Path: "/"}}
	return primary, secondary
}
