package apps

import (
	"encoding/binary"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/pathutil"
)

// Lectura de accesos directos (.lnk) del menú Inicio, para encontrar apps que
// no se registran en Configuración → Aplicaciones (las portables, por
// ejemplo). Solo se lee lo necesario del formato MS-SHLLINK: la ruta del
// destino.

// shortcut es un acceso directo: su nombre (sin .lnk) y su destino.
type shortcut struct {
	name   string
	target string
}

// Constantes del formato MS-SHLLINK.
const (
	lnkHeaderSize     = 0x4C       // tamaño de ShellLinkHeader
	lnkHasIDList      = 1 << 0     // LinkFlags: HasLinkTargetIDList
	lnkHasLinkInfo    = 1 << 1     // LinkFlags: HasLinkInfo
	lnkIsUnicode      = 1 << 7     // LinkFlags: IsUnicode (StringData en UTF-16)
	lnkLocalBasePath  = 1          // LinkInfoFlags: VolumeIDAndLocalBasePath
	lnkEnvBlock       = 0xA0000001 // firma de EnvironmentVariableDataBlock
	lnkEnvBlockSize   = 0x314      // tamaño de EnvironmentVariableDataBlock
	lnkStringDataBits = 5          // HasName, HasRelativePath, HasWorkingDir, HasArguments, HasIconLocation
	maxLnkSize        = 1 << 16    // un .lnk real ocupa unos pocos KB
)

// readShortcuts lee los accesos directos que hay bajo dir y devuelve los que
// apuntan a una ruta local, con las variables de entorno expandidas.
func readShortcuts(dir string) []shortcut {
	var links []shortcut
	// Las carpetas que no se pueden leer se saltan: solo faltarán sus apps.
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".lnk") {
			return nil
		}
		if info, err := d.Info(); err != nil || info.Size() > maxLnkSize {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if target := lnkTarget(data); target != "" {
			name := strings.TrimSpace(strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())))
			links = append(links, shortcut{name: name, target: pathutil.ExpandEnv(target)})
		}
		return nil
	})
	return links
}

// lnkTarget devuelve la ruta a la que apunta un acceso directo: la ruta local
// de LinkInfo o, si no la tiene, la del bloque de variables de entorno (sin
// expandir). Devuelve "" si no apunta a una ruta local (como los accesos
// "anunciados" de los instaladores MSI) o si el archivo está dañado.
func lnkTarget(data []byte) string {
	if u32(data, 0) != lnkHeaderSize || len(data) < lnkHeaderSize {
		return ""
	}
	flags := u32(data, 20)
	pos := lnkHeaderSize
	if flags&lnkHasIDList != 0 {
		pos += 2 + int(u16(data, pos))
	}
	if flags&lnkHasLinkInfo != 0 {
		size := int(u32(data, pos))
		if size < 0x1C || pos+size > len(data) {
			return ""
		}
		if target := linkInfoPath(data[pos : pos+size]); target != "" {
			return target
		}
		pos += size
	}
	return envBlockTarget(data, skipStringData(data, pos, flags))
}

// linkInfoPath devuelve LocalBasePath + CommonPathSuffix de una estructura
// LinkInfo, en UTF-16 si la tiene y si no en ANSI.
func linkInfoPath(info []byte) string {
	if u32(info, 8)&lnkLocalBasePath == 0 {
		return ""
	}
	if u32(info, 4) >= 0x24 {
		if base := utf16z(info, int(u32(info, 28))); base != "" {
			return base + utf16z(info, int(u32(info, 32)))
		}
	}
	return ansiz(info, int(u32(info, 16))) + ansiz(info, int(u32(info, 24)))
}

// skipStringData salta las cadenas de StringData que indican los flags y
// devuelve dónde empieza ExtraData.
func skipStringData(data []byte, pos int, flags uint32) int {
	for bit := range lnkStringDataBits {
		if flags&(1<<(2+bit)) == 0 {
			continue
		}
		n := int(u16(data, pos))
		if flags&lnkIsUnicode != 0 {
			n *= 2
		}
		pos += 2 + n
	}
	return pos
}

// envBlockTarget busca en los bloques de ExtraData, desde pos, el de
// variables de entorno y devuelve su destino.
func envBlockTarget(data []byte, pos int) string {
	for pos+8 <= len(data) {
		size := int(u32(data, pos))
		if size < 8 || pos+size > len(data) {
			return ""
		}
		if u32(data, pos+4) == lnkEnvBlock && size >= lnkEnvBlockSize {
			block := data[pos : pos+size]
			if target := utf16z(block[:8+260+520], 8+260); target != "" {
				return target
			}
			return ansiz(block[:8+260], 8)
		}
		pos += size
	}
	return ""
}

// u32 y u16 leen enteros little-endian; fuera de los límites devuelven 0.
func u32(b []byte, off int) uint32 {
	if off < 0 || off+4 > len(b) {
		return 0
	}
	return binary.LittleEndian.Uint32(b[off:])
}

func u16(b []byte, off int) uint16 {
	if off < 0 || off+2 > len(b) {
		return 0
	}
	return binary.LittleEndian.Uint16(b[off:])
}

// utf16z lee una cadena UTF-16 terminada en cero desde off.
func utf16z(b []byte, off int) string {
	if off <= 0 || off >= len(b) {
		return ""
	}
	var units []uint16
	for i := off; i+1 < len(b); i += 2 {
		u := binary.LittleEndian.Uint16(b[i:])
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return string(utf16.Decode(units))
}

// ansiz lee una cadena ANSI terminada en cero desde off. Se interpreta como
// Latin-1, que coincide con la página de códigos 1252 en las letras con
// acento; una ruta mal leída no existirá y se descartará después.
func ansiz(b []byte, off int) string {
	if off <= 0 || off >= len(b) {
		return ""
	}
	var r []rune
	for _, c := range b[off:] {
		if c == 0 {
			break
		}
		r = append(r, rune(c))
	}
	return string(r)
}
