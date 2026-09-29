//go:build !windows

package search

import (
	"io/fs"
	"os"
)

// inspect clasifica una entrada en sistemas no Windows. Allí no existen los
// atributos oculto/sistema, así que solo se consideran ocultos los nombres que
// empiezan por punto.
func inspect(dir string, e fs.DirEntry) entryKind {
	hidden := isDotName(e.Name())
	if e.IsDir() {
		return entryKind{dir: true, hidden: hidden}
	}
	if e.Type()&fs.ModeSymlink != 0 {
		if info, err := os.Stat(join(dir, e.Name())); err == nil && info.IsDir() {
			return entryKind{dir: true, link: true, hidden: hidden}
		}
	}
	return entryKind{}
}
