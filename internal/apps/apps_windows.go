//go:build windows

package apps

import (
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// regKey es una clave del registro: una raíz (HKLM, HKCU) y una ruta.
type regKey struct {
	root registry.Key
	path string
}

// uninstallKeys son las listas de programas instalados que muestra
// Configuración → Aplicaciones: para todos los usuarios (64 y 32 bits) y para
// el usuario actual.
var uninstallKeys = []regKey{
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
}

// appPathKeys son las claves App Paths, donde los programas registran la ruta
// de su ejecutable para que Windows lo encuentre por su nombre (excel.exe).
var appPathKeys = []regKey{
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths`},
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\App Paths`},
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths`},
}

// Find devuelve las aplicaciones instaladas cuyo nombre acepta match (todas
// si match es nil), ordenadas por nombre.
func Find(match func(string) bool) ([]App, error) {
	entries, err := readUninstall(uninstallKeys)
	if err != nil {
		return nil, err
	}
	return build(entries, readAppPaths(appPathKeys), readStartMenu(), match), nil
}

// startMenuFolders son las carpetas de accesos directos del menú Inicio:
// para todos los usuarios y para el actual.
var startMenuFolders = []*windows.KNOWNFOLDERID{windows.FOLDERID_CommonPrograms, windows.FOLDERID_Programs}

// StartMenu devuelve los accesos directos (.lnk) del menú Inicio, de todos
// los usuarios y del actual, que apuntan a una ruta local o tienen un
// AppUserModelID.
func StartMenu() []Shortcut {
	return readStartMenu()
}

// readStartMenu lee los accesos directos (.lnk) del menú Inicio.
func readStartMenu() []Shortcut {
	var links []Shortcut
	for _, id := range startMenuFolders {
		if dir, err := windows.KnownFolderPath(id, 0); err == nil {
			links = append(links, readShortcuts(dir)...)
		}
	}
	return links
}

// readUninstall lee las entradas de desinstalación que hay bajo keys. Una
// clave que no existe (por ejemplo, WOW6432Node en un Windows de 32 bits) se
// omite; solo falla si no puede leer ninguna.
func readUninstall(keys []regKey) ([]entry, error) {
	var entries []entry
	var lastErr error
	read := false
	for _, k := range keys {
		names, err := subKeys(k)
		if err != nil {
			lastErr = err
			continue
		}
		read = true
		for _, name := range names {
			if e, ok := readEntry(regKey{k.root, k.path + `\` + name}); ok {
				entries = append(entries, e)
			}
		}
	}
	if !read {
		return nil, fmt.Errorf("no se pudo leer la lista de programas instalados: %w", lastErr)
	}
	return entries, nil
}

// readEntry lee los valores de una clave de desinstalación. Devuelve false
// si no se puede abrir.
func readEntry(k regKey) (entry, bool) {
	key, err := registry.OpenKey(k.root, k.path, registry.QUERY_VALUE)
	if err != nil {
		return entry{}, false
	}
	defer key.Close()
	return entry{
		name:            stringValue(key, "DisplayName"),
		installLocation: stringValue(key, "InstallLocation"),
		displayIcon:     stringValue(key, "DisplayIcon"),
		uninstall:       stringValue(key, "UninstallString"),
		publisher:       stringValue(key, "Publisher"),
		version:         stringValue(key, "DisplayVersion"),
		hidden:          isHidden(key),
	}, true
}

// isHidden indica si una entrada no aparece en Configuración → Aplicaciones:
// un componente del sistema o una actualización de otro programa.
func isHidden(key registry.Key) bool {
	if n, _, err := key.GetIntegerValue("SystemComponent"); err == nil && n == 1 {
		return true
	}
	if stringValue(key, "ParentKeyName") != "" {
		return true
	}
	switch stringValue(key, "ReleaseType") {
	case "Update", "Hotfix", "Security Update", "Service Pack":
		return true
	}
	return false
}

// readAppPaths devuelve las rutas de los ejecutables registrados en App
// Paths: el valor predeterminado de cada subclave de keys. Las claves que no
// existen o no se pueden leer se omiten.
func readAppPaths(keys []regKey) []string {
	var paths []string
	for _, k := range keys {
		names, err := subKeys(k)
		if err != nil {
			continue
		}
		for _, name := range names {
			if p := defaultValue(regKey{k.root, k.path + `\` + name}); p != "" {
				paths = append(paths, p)
			}
		}
	}
	return paths
}

// subKeys devuelve los nombres de las subclaves de k.
func subKeys(k regKey) ([]string, error) {
	key, err := registry.OpenKey(k.root, k.path, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil, err
	}
	defer key.Close()
	return key.ReadSubKeyNames(0)
}

// defaultValue devuelve el valor predeterminado de k, o "" si no existe.
func defaultValue(k regKey) string {
	key, err := registry.OpenKey(k.root, k.path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	return stringValue(key, "")
}

// stringValue devuelve el valor de texto name de key, con las variables de
// entorno expandidas (%ProgramFiles%...), o "" si no existe.
func stringValue(key registry.Key, name string) string {
	s, typ, err := key.GetStringValue(name)
	if err != nil {
		return ""
	}
	if typ == registry.EXPAND_SZ {
		if expanded, err := registry.ExpandString(s); err == nil {
			return expanded
		}
	}
	return s
}
