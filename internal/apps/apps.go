// Package apps encuentra las aplicaciones instaladas en Windows a partir de lo
// que registran sus instaladores (lo mismo que muestra Configuración →
// Aplicaciones) y averigua en qué carpeta está cada una.
package apps

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// App es una aplicación instalada.
type App struct {
	Name      string // nombre visible, como en Configuración → Aplicaciones
	Dir       string // carpeta del ejecutable o, si no se conoce, la de instalación
	Exe       string // ejecutable principal; vacío si no se conoce
	Publisher string
	Version   string
}

// entry son los valores que se usan de una clave de desinstalación del
// registro (...\CurrentVersion\Uninstall\<programa>).
type entry struct {
	name            string // DisplayName
	installLocation string // InstallLocation
	displayIcon     string // DisplayIcon: `"C:\App\app.exe",0`, un .ico...
	uninstall       string // UninstallString: la orden que lo desinstala
	publisher       string // Publisher
	version         string // DisplayVersion
	hidden          bool   // componente del sistema o actualización de otro programa
}

// build convierte las entradas del registro y las rutas de App Paths en la
// lista de aplicaciones que acepta match (todas si match es nil; ver
// finish), ordenada por nombre y sin repetidas. Descarta las entradas
// ocultas, las que no tienen nombre y las que no tienen una carpeta que
// exista.
func build(entries []entry, appPaths []string, links []shortcut, match func(string) bool) []App {
	exes := existingExes(appPaths)
	linked := existingLinks(links)
	candidates := exes
	for _, l := range linked {
		candidates = append(candidates, l.target)
	}
	candidates = existingExes(candidates) // sin repetir

	var list []App
	used := map[string]bool{} // ejecutables que ya son de una aplicación
	add := func(a App) {
		if a.Exe != "" {
			used[pathKey(a.Exe)] = true
		}
		list = append(list, a)
	}
	for _, e := range entries {
		if a, ok := fromEntry(e); ok {
			add(withAppPath(a, candidates))
		}
	}
	for _, l := range linked {
		if !used[pathKey(l.target)] {
			add(App{Name: l.name, Dir: filepath.Dir(l.target), Exe: l.target})
		}
	}
	list = append(list, fromAppPaths(exes, used)...)
	return finish(list, match)
}

// existingLinks deja los accesos directos que apuntan a un .exe que existe y
// no es un desinstalador, sin repetir el destino.
func existingLinks(links []shortcut) []shortcut {
	seen := map[string]bool{}
	var out []shortcut
	for _, l := range links {
		exe := cleanPath(l.target)
		if seen[pathKey(exe)] || !isExe(exe) || isUninstaller(exe) || !isFile(exe) {
			continue
		}
		seen[pathKey(exe)] = true
		out = append(out, shortcut{name: l.name, target: exe})
	}
	return out
}

// existingExes limpia las rutas de App Paths y deja, sin repetir, las de los
// .exe que existen.
func existingExes(appPaths []string) []string {
	seen := map[string]bool{}
	var exes []string
	for _, p := range appPaths {
		exe := cleanPath(p)
		if seen[pathKey(exe)] || !isExe(exe) || !isFile(exe) {
			continue
		}
		seen[pathKey(exe)] = true
		exes = append(exes, exe)
	}
	return exes
}

// withAppPath completa una aplicación sin ejecutable conocido con el único
// ejecutable de exes (App Paths) que esté en su carpeta, como 7zFM.exe en la
// de 7-Zip. Si hay varios (Microsoft 365 registra EXCEL, WINWORD...), no
// elige ninguno y cada uno aparece como una aplicación aparte.
func withAppPath(a App, exes []string) App {
	if a.Exe != "" {
		return a
	}
	found := ""
	for _, exe := range exes {
		if !within(exe, a.Dir) {
			continue
		}
		if found != "" {
			return a
		}
		found = exe
	}
	if found != "" {
		a.Exe, a.Dir = found, filepath.Dir(found)
	}
	return a
}

// fromEntry averigua la carpeta y el ejecutable de una entrada del registro.
// Devuelve false si la entrada no es un programa cuya ubicación se pueda
// abrir.
func fromEntry(e entry) (App, bool) {
	name := strings.TrimSpace(e.name)
	if e.hidden || name == "" {
		return App{}, false
	}
	loc := cleanPath(e.installLocation)
	if !isDir(loc) {
		loc = ""
	}
	exe := mainExe(e.displayIcon, loc)
	dir := loc
	switch {
	case exe != "":
		dir = filepath.Dir(exe)
	case dir == "":
		dir = fallbackDir(e)
	}
	if dir == "" {
		return App{}, false
	}
	return App{
		Name:      name,
		Dir:       dir,
		Exe:       exe,
		Publisher: strings.TrimSpace(e.publisher),
		Version:   strings.TrimSpace(e.version),
	}, true
}

// mainExe devuelve el ejecutable del icono (DisplayIcon) si es un .exe que
// existe, no es el desinstalador y está dentro de loc (cuando se conoce loc).
func mainExe(displayIcon, loc string) string {
	icon := iconFile(displayIcon)
	if !isExe(icon) || isUninstaller(icon) || !isFile(icon) {
		return ""
	}
	if loc != "" && !within(icon, loc) {
		return ""
	}
	return icon
}

// fallbackDir busca la carpeta de una entrada sin InstallLocation ni
// ejecutable principal: la del icono o, si no, la del desinstalador.
func fallbackDir(e entry) string {
	if icon := iconFile(e.displayIcon); isFile(icon) {
		return filepath.Dir(icon)
	}
	if exe := commandExe(e.uninstall); isFile(exe) {
		return filepath.Dir(exe)
	}
	return ""
}

// fromAppPaths convierte en aplicaciones los ejecutables de App Paths (exes)
// que no son ya los de otra aplicación (used). Así aparecen, por ejemplo,
// Excel o Word, que el registro agrupa como "Microsoft 365". Su nombre es el
// del ejecutable.
func fromAppPaths(exes []string, used map[string]bool) []App {
	var list []App
	for _, exe := range exes {
		if !used[pathKey(exe)] {
			list = append(list, App{Name: exeName(exe), Dir: filepath.Dir(exe), Exe: exe})
		}
	}
	return list
}

// exeName devuelve el nombre del ejecutable exe sin la extensión: "EXCEL".
func exeName(exe string) string {
	return strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
}

// finish deja en list las aplicaciones cuyo nombre o el de su ejecutable
// acepta match ("excel" encuentra "Microsoft 365" si su ejecutable es
// EXCEL.EXE), quita las repetidas (mismo nombre y misma carpeta, como las que
// se registran a la vez para todos los usuarios y para el actual) y las
// ordena por nombre.
func finish(list []App, match func(string) bool) []App {
	seen := map[string]bool{}
	var out []App
	for _, a := range list {
		k := strings.ToLower(a.Name) + "|" + pathKey(a.Dir)
		if seen[k] || !accepts(match, a) {
			continue
		}
		seen[k] = true
		out = append(out, a)
	}
	slices.SortStableFunc(out, func(a, b App) int {
		return cmp.Or(
			cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
			cmp.Compare(pathKey(a.Dir), pathKey(b.Dir)),
		)
	})
	return out
}

// accepts indica si match acepta el nombre de a o el de su ejecutable. Sin
// match, las acepta todas.
func accepts(match func(string) bool, a App) bool {
	return match == nil || match(a.Name) || (a.Exe != "" && match(exeName(a.Exe)))
}

// cleanPath quita los espacios y las comillas que rodean una ruta del
// registro, y la barra final salvo en la raíz de una unidad (C:\).
func cleanPath(s string) string {
	s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), `"`))
	if len(s) > 3 {
		s = strings.TrimRight(s, `\/`)
	}
	return s
}

// iconFile extrae el archivo de un valor DisplayIcon, que puede llevar
// comillas y el número del icono: `"C:\App\app.exe",0`.
func iconFile(s string) string {
	s = strings.TrimSpace(s)
	if q, ok := quoted(s); ok {
		return cleanPath(q)
	}
	if i := strings.LastIndex(s, ","); i >= 0 {
		if _, err := strconv.Atoi(strings.TrimSpace(s[i+1:])); err == nil {
			s = s[:i]
		}
	}
	return cleanPath(s)
}

// commandExe extrae el ejecutable de una orden como UninstallString:
// `"C:\App\unins000.exe" /SILENT` o `C:\Mi App\uninstall.exe /S`.
func commandExe(s string) string {
	s = strings.TrimSpace(s)
	if q, ok := quoted(s); ok {
		return cleanPath(q)
	}
	if strings.HasPrefix(s, `"`) {
		return "" // comillas sin cerrar
	}
	if i := strings.Index(strings.ToLower(s), ".exe"); i >= 0 {
		return cleanPath(s[:i+len(".exe")])
	}
	return ""
}

// quoted devuelve el texto entre las comillas con las que empieza s, si s
// empieza por comillas y se cierran.
func quoted(s string) (string, bool) {
	if !strings.HasPrefix(s, `"`) {
		return "", false
	}
	end := strings.Index(s[1:], `"`)
	if end < 0 {
		return "", false
	}
	return s[1 : end+1], true
}

// rejected indica si path no puede ser la ubicación de una aplicación: la
// raíz de una unidad, la carpeta de Windows (allí están msiexec.exe y las
// copias de los instaladores MSI que algunos programas usan como icono), la
// caché de paquetes de los instaladores (Package Cache) o una carpeta de red
// (\\servidor\...): comprobar si existe podría bloquear la búsqueda o
// conectarse a un servidor que nombre el registro.
func rejected(path string) bool {
	p := strings.TrimRight(pathKey(path), "/")
	if len(p) <= 2 || strings.HasPrefix(p, "//") {
		return true
	}
	if p[1] == ':' && strings.HasPrefix(p[2:]+"/", "/windows/") {
		return true
	}
	return strings.Contains(p+"/", "/package cache/")
}

// usable indica si path es una ruta absoluta que puede ser la ubicación de
// una aplicación (ver rejected).
func usable(path string) bool {
	return path != "" && filepath.IsAbs(path) && !rejected(path)
}

// isDir indica si path es una carpeta usable que existe.
func isDir(path string) bool {
	if !usable(path) {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// isFile indica si path es un archivo usable que existe.
func isFile(path string) bool {
	if !usable(path) {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isExe(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".exe")
}

// isUninstaller indica si exe parece un desinstalador (unins000.exe,
// uninstall.exe...), que no sirve como ejecutable principal.
func isUninstaller(exe string) bool {
	base := strings.ToLower(filepath.Base(exe))
	return strings.HasPrefix(base, "unins") || strings.Contains(base, "uninstall")
}

// within indica si path está dentro de la carpeta dir o de una subcarpeta.
func within(path, dir string) bool {
	return strings.HasPrefix(pathKey(path), strings.TrimRight(pathKey(dir), "/")+"/")
}

// pathKey normaliza una ruta para compararla con otra: Windows no distingue
// mayúsculas ni el tipo de barra.
func pathKey(path string) string {
	return strings.ToLower(strings.ReplaceAll(path, `\`, "/"))
}
