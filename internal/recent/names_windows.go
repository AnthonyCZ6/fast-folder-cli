//go:build windows

package recent

import (
	"golang.org/x/sys/windows"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/apps"
)

// systemCandidates devuelve los identificadores de los programas de este
// equipo: el AppUserModelID y el ejecutable de los accesos directos del menú
// Inicio, y el ejecutable de los programas instalados.
func systemCandidates() []candidate {
	var out []candidate
	for _, s := range apps.StartMenu() {
		n := AppName{Name: s.Name}
		if s.Target != "" {
			n.Aliases = []string{exeName(s.Target)}
		}
		if s.AppUserModelID != "" {
			out = append(out, candidate{s.AppUserModelID, n})
		}
		if s.Target != "" {
			out = append(out, candidate{s.Target, n})
		}
	}
	// Sin la lista de programas instalados solo faltarán algunos nombres.
	if list, err := apps.Find(nil); err == nil {
		for _, a := range list {
			if a.Exe != "" {
				out = append(out, candidate{a.Exe, AppName{a.Name, []string{exeName(a.Exe)}}})
			}
		}
	}
	return out
}

// substituted son las carpetas conocidas que Windows sustituye por su GUID
// en la ruta de un ejecutable al calcular su AppID.
var substituted = []*windows.KNOWNFOLDERID{
	windows.FOLDERID_ProgramFilesX64,
	windows.FOLDERID_ProgramFilesX86,
	windows.FOLDERID_ProgramFilesCommonX64,
	windows.FOLDERID_ProgramFilesCommonX86,
	windows.FOLDERID_System,
	windows.FOLDERID_SystemX86,
	windows.FOLDERID_Windows,
}

// knownFolders devuelve la ruta en este equipo de las carpetas de substituted.
func knownFolders() []knownFolder {
	var out []knownFolder
	for _, id := range substituted {
		if path, err := windows.KnownFolderPath(id, 0); err == nil {
			out = append(out, knownFolder{guid: windows.GUID(*id).String(), path: path})
		}
	}
	return out
}
