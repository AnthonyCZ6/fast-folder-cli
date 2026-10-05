package apps

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/lnk"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/pathutil"
)

// Accesos directos (.lnk) del menú Inicio, para encontrar apps que no se
// registran en Configuración → Aplicaciones (las portables, por ejemplo).

// Shortcut es un acceso directo del menú Inicio.
type Shortcut struct {
	Name           string // nombre del acceso directo, sin .lnk
	Target         string // destino, con las variables de entorno expandidas; vacío si no apunta a una ruta local
	AppUserModelID string // identificador de aplicación ("Microsoft.Office.WINWORD.EXE.15"); vacío si no lo tiene
}

// readShortcuts lee los accesos directos que hay bajo dir y devuelve los que
// apuntan a una ruta local (con las variables de entorno expandidas) o tienen
// un AppUserModelID, como los "anunciados" de Office, que no tienen destino.
func readShortcuts(dir string) []Shortcut {
	var links []Shortcut
	// Las carpetas que no se pueden leer se saltan: solo faltarán sus apps.
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".lnk") {
			return nil
		}
		if info, err := d.Info(); err != nil || info.Size() > lnk.MaxSize {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		s := Shortcut{
			Name:           strings.TrimSpace(strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))),
			AppUserModelID: lnk.AppUserModelID(data),
		}
		if target := lnk.Target(data); target != "" {
			s.Target = pathutil.ExpandEnv(target)
		}
		if s.Target != "" || s.AppUserModelID != "" {
			links = append(links, s)
		}
		return nil
	})
	return links
}
