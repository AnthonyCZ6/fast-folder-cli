//go:build windows

package jumplist

import (
	"path/filepath"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// defaultDir devuelve %APPDATA%\Microsoft\Windows\Recent\AutomaticDestinations.
func defaultDir() (string, error) {
	recent, err := windows.KnownFolderPath(windows.FOLDERID_Recent, 0)
	if err != nil {
		return "", err
	}
	return filepath.Join(recent, "AutomaticDestinations"), nil
}

// knownFolder devuelve la ruta de la carpeta conocida guid ("{374DE290-...}",
// Descargas), o "" si no es una carpeta conocida de este equipo.
func knownFolder(guid string) string {
	g, err := windows.GUIDFromString(guid)
	if err != nil {
		return ""
	}
	id := windows.KNOWNFOLDERID(g)
	path, err := windows.KnownFolderPath(&id, 0)
	if err != nil {
		return ""
	}
	return path
}

// driveType es windows.GetDriveType; las pruebas lo reemplazan para simular
// una unidad de red.
var driveType = windows.GetDriveType

// remoteDrive indica si path está en una unidad de red (Z: conectada a
// \\servidor\carpeta). Windows lo sabe sin conectarse al servidor.
func remoteDrive(path string) bool {
	vol := filepath.VolumeName(path)
	if len(vol) != 2 || vol[1] != ':' {
		return false
	}
	root, err := windows.UTF16PtrFromString(vol + `\`)
	return err == nil && driveType(root) == windows.DRIVE_REMOTE
}

// HistoryOff explica por qué Windows no guarda el historial de archivos
// recientes, o devuelve "" si lo guarda. Sin historial, las jump lists no
// reciben nada nuevo.
func HistoryOff() string {
	const policies = `Software\Microsoft\Windows\CurrentVersion\Policies\Explorer`
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		if dword(root, policies, "NoRecentDocsHistory") == 1 {
			return "una directiva del sistema desactiva el historial de archivos recientes"
		}
	}
	if dword(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\Advanced`, "Start_TrackDocs") == 0 {
		return `está desactivado "Mostrar elementos abiertos recientemente en Inicio, las listas de accesos directos y el Explorador de archivos" ` +
			"(Configuración → Personalización → Inicio)"
	}
	return ""
}

// dword devuelve el valor DWORD name de la clave path, o -1 si no existe.
func dword(root registry.Key, path, name string) int64 {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return -1
	}
	defer key.Close()
	v, _, err := key.GetIntegerValue(name)
	if err != nil {
		return -1
	}
	return int64(v)
}
