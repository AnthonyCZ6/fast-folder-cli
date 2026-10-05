// Package lnk lee accesos directos de Windows (.lnk, formato MS-SHLLINK): solo
// lo que necesita fast-folder-cli, la ruta del destino. Los accesos directos
// vienen de fuera (el menú Inicio, las jump lists), así que un archivo dañado
// nunca hace fallar la lectura: como mucho no se obtiene nada.
package lnk

import (
	"encoding/binary"
	"unicode/utf16"
)

// Constantes del formato MS-SHLLINK.
const (
	headerSize     = 0x4C       // tamaño de ShellLinkHeader
	hasIDList      = 1 << 0     // LinkFlags: HasLinkTargetIDList
	hasLinkInfo    = 1 << 1     // LinkFlags: HasLinkInfo
	isUnicode      = 1 << 7     // LinkFlags: IsUnicode (StringData en UTF-16)
	localBasePath  = 1          // LinkInfoFlags: VolumeIDAndLocalBasePath
	envBlock       = 0xA0000001 // firma de EnvironmentVariableDataBlock
	envBlockSize   = 0x314      // tamaño de EnvironmentVariableDataBlock
	stringDataBits = 5          // HasName, HasRelativePath, HasWorkingDir, HasArguments, HasIconLocation
)

// MaxSize es el tamaño máximo de un .lnk que vale la pena leer: uno real
// ocupa unos pocos KB.
const MaxSize = 1 << 16

// Target devuelve la ruta a la que apunta un acceso directo: la ruta local de
// LinkInfo o, si no la tiene, la del bloque de variables de entorno (sin
// expandir). Devuelve "" si no apunta a una ruta local (como los accesos
// "anunciados" de los instaladores MSI) o si el archivo está dañado.
func Target(data []byte) string {
	if u32(data, 0) != headerSize || len(data) < headerSize {
		return ""
	}
	flags := u32(data, 20)
	pos := headerSize
	if flags&hasIDList != 0 {
		pos += 2 + int(u16(data, pos))
	}
	if flags&hasLinkInfo != 0 {
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

// Bloque de propiedades (PropertyStoreDataBlock, MS-SHLLINK 2.5.7) y la
// propiedad System.AppUserModel.ID (formato MS-PROPSTORE).
const (
	propStoreBlock = 0xA0000009
	propStoreSPS1  = 0x53505331 // versión de una serialized property storage
	vtLPWSTR       = 0x1F
	appIDPropID    = 5
	maxAppIDChars  = 260
)

// appIDFormat es el FMTID de System.AppUserModel.ID,
// {9F4C2855-9F79-4B39-A8D0-E1D42DE1D5F3}, tal como se guarda en el archivo.
var appIDFormat = [16]byte{0x55, 0x28, 0x4C, 0x9F, 0x79, 0x9F, 0x39, 0x4B, 0xA8, 0xD0, 0xE1, 0xD4, 0x2D, 0xE1, 0xD5, 0xF3}

// AppUserModelID devuelve el identificador de aplicación (AppUserModelID)
// que guarda un acceso directo, como "Microsoft.Office.WINWORD.EXE.15" en el
// de Word. Es el que usa Windows para agrupar las ventanas y nombrar las jump
// lists de ese programa. Devuelve "" si no lo tiene o si el archivo está
// dañado.
func AppUserModelID(data []byte) string {
	if u32(data, 0) != headerSize || len(data) < headerSize {
		return ""
	}
	flags := u32(data, 20)
	pos := headerSize
	if flags&hasIDList != 0 {
		pos += 2 + int(u16(data, pos))
	}
	if flags&hasLinkInfo != 0 {
		pos += int(u32(data, pos))
	}
	for pos = skipStringData(data, pos, flags); pos+8 <= len(data); {
		size := int(u32(data, pos))
		if size < 8 || pos+size > len(data) {
			return ""
		}
		if u32(data, pos+4) == propStoreBlock {
			if id := propStoreAppID(data[pos+8 : pos+size]); id != "" {
				return id
			}
		}
		pos += size
	}
	return ""
}

// propStoreAppID busca System.AppUserModel.ID en un property store: una
// serie de storages (tamaño, "1SPS", FMTID y valores) que acaba en un tamaño 0.
func propStoreAppID(store []byte) string {
	for pos := 0; pos+24 <= len(store); {
		size := int(u32(store, pos))
		if size < 24 || pos+size > len(store) {
			return ""
		}
		storage := store[pos : pos+size]
		if u32(storage, 4) == propStoreSPS1 && [16]byte(storage[8:24]) == appIDFormat {
			return storageAppID(storage[24:])
		}
		pos += size
	}
	return ""
}

// storageAppID busca el valor con el identificador 5 (de tipo VT_LPWSTR)
// entre los valores de un storage con nombres numéricos.
func storageAppID(values []byte) string {
	for pos := 0; pos+13 <= len(values); {
		size := int(u32(values, pos))
		if size < 13 || pos+size > len(values) {
			return ""
		}
		v := values[pos : pos+size]
		if u32(v, 4) == appIDPropID && u16(v, 9) == vtLPWSTR {
			chars := int(u32(v, 13))
			if chars <= 0 || chars > maxAppIDChars || 17+2*chars > len(v) {
				return ""
			}
			return utf16z(v[:17+2*chars], 17)
		}
		pos += size
	}
	return ""
}

// linkInfoPath devuelve LocalBasePath + CommonPathSuffix de una estructura
// LinkInfo, en UTF-16 si la tiene y si no en ANSI.
func linkInfoPath(info []byte) string {
	if u32(info, 8)&localBasePath == 0 {
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
	for bit := range stringDataBits {
		if flags&(1<<(2+bit)) == 0 {
			continue
		}
		n := int(u16(data, pos))
		if flags&isUnicode != 0 {
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
		if u32(data, pos+4) == envBlock && size >= envBlockSize {
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
