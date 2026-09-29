//go:build windows

package search

import (
	"io/fs"
	"syscall"
)

// inspect clasifica una entrada usando los atributos Win32 que Windows ya
// devolvió al listar el directorio, sin llamadas adicionales al sistema.
func inspect(_ string, e fs.DirEntry) entryKind {
	// Los archivos normales se descartan sin consultar sus atributos. Go
	// marca las uniones (junctions) y los enlaces simbólicos como
	// ModeIrregular/ModeSymlink en lugar de ModeDir, por eso también se
	// examinan esos tipos.
	if !e.IsDir() && e.Type()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
		return entryKind{}
	}

	info, err := e.Info()
	if err != nil {
		return entryKind{}
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return entryKind{dir: e.IsDir(), hidden: isDotName(e.Name())}
	}

	attrs := data.FileAttributes
	if attrs&syscall.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return entryKind{}
	}

	return entryKind{
		dir: true,
		// Windows marca como directorio los enlaces y uniones que apuntan a
		// carpetas, pero Go (siguiendo la semántica POSIX) no los considera
		// directorios. Las carpetas de OneDrive (reparse points de nube)
		// sí se reportan como directorios y por tanto se recorren.
		link:   !e.IsDir(),
		hidden: attrs&(syscall.FILE_ATTRIBUTE_HIDDEN|syscall.FILE_ATTRIBUTE_SYSTEM) != 0 || isDotName(e.Name()),
	}
}
