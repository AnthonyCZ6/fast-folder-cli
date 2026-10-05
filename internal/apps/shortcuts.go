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

// shortcut es un acceso directo: su nombre (sin .lnk) y su destino.
type shortcut struct {
	name   string
	target string
}

// readShortcuts lee los accesos directos que hay bajo dir y devuelve los que
// apuntan a una ruta local, con las variables de entorno expandidas.
func readShortcuts(dir string) []shortcut {
	var links []shortcut
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
		if target := lnk.Target(data); target != "" {
			name := strings.TrimSpace(strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())))
			links = append(links, shortcut{name: name, target: pathutil.ExpandEnv(target)})
		}
		return nil
	})
	return links
}
