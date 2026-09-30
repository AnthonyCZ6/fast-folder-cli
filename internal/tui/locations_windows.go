//go:build windows

package tui

import "golang.org/x/sys/windows"

// platformLocations usa las carpetas conocidas de Windows, que respetan las
// redirecciones (por ejemplo, Escritorio o Documentos movidos a OneDrive).
func platformLocations() (primary, secondary []location) {
	known := func(label string, id *windows.KNOWNFOLDERID) location {
		path, err := windows.KnownFolderPath(id, 0)
		if err != nil {
			return location{}
		}
		return location{Label: label, Path: path}
	}

	primary = []location{
		known("Perfil de usuario", windows.FOLDERID_Profile),
		known("Escritorio", windows.FOLDERID_Desktop),
		known("Documentos", windows.FOLDERID_Documents),
		known("Descargas", windows.FOLDERID_Downloads),
	}
	secondary = []location{
		known("AppData (Roaming)", windows.FOLDERID_RoamingAppData),
		known("AppData (Local)", windows.FOLDERID_LocalAppData),
		known("ProgramData", windows.FOLDERID_ProgramData),
	}

	// Unidades locales y extraíbles. Las de red se omiten porque recorrerlas
	// puede ser muy lento.
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return primary, secondary
	}
	for i := 0; i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		p, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		switch windows.GetDriveType(p) {
		case windows.DRIVE_FIXED, windows.DRIVE_REMOVABLE:
			secondary = append(secondary, location{Label: "Unidad " + root[:2], Path: root})
		}
	}
	return primary, secondary
}
