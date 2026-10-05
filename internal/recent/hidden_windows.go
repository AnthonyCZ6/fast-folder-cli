//go:build windows

package recent

import (
	"os"
	"syscall"
)

// hiddenAttr indica si la carpeta path tiene el atributo de oculta o de
// sistema, como las que se salta la búsqueda sin --all.
func hiddenAttr(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.FileAttributes&(syscall.FILE_ATTRIBUTE_HIDDEN|syscall.FILE_ATTRIBUTE_SYSTEM) != 0
}
