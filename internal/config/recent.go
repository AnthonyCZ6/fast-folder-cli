package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Carpetas recientes: las últimas elegidas en el modo interactivo. Se guardan
// en recientes.json, junto al archivo de configuración (así FFC_CONFIG
// también las aparta, por ejemplo en las pruebas).

// maxRecent es cuántas carpetas se guardan como máximo, aunque se ofrezcan
// menos.
const maxRecent = 20

// RecentPath devuelve la ruta del archivo de carpetas recientes.
func RecentPath() (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "recientes.json"), nil
}

// LoadRecent devuelve hasta limit carpetas recientes que aún existen, de la
// más reciente a la más antigua. Si el archivo no existe o está dañado
// devuelve nil: perder las recientes no impide usar el programa.
func LoadRecent(limit int) []string {
	if limit <= 0 {
		return nil
	}
	dirs := readRecent()
	var out []string
	for _, d := range dirs {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			out = append(out, d)
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

// AddRecent pone dir la primera de las carpetas recientes, sin repetirla, y
// guarda el archivo. Un archivo dañado se reemplaza.
func AddRecent(dir string) error {
	path, err := RecentPath()
	if err != nil {
		return err
	}
	dirs := slices.DeleteFunc(readRecent(), func(d string) bool { return strings.EqualFold(d, dir) })
	dirs = append([]string{dir}, dirs...)
	if len(dirs) > maxRecent {
		dirs = dirs[:maxRecent]
	}
	data, err := json.MarshalIndent(dirs, "", "  ")
	if err != nil {
		return err
	}
	// Solo para el usuario: la lista dice por qué carpetas se mueve.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// Se escribe aparte y se renombra: nunca queda un archivo a medias.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// readRecent lee la lista guardada; nil si no existe o está dañada.
func readRecent() []string {
	path, err := RecentPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var dirs []string
	if json.Unmarshal(data, &dirs) != nil {
		return nil
	}
	return dirs
}
