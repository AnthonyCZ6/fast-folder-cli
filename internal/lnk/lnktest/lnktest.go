// Package lnktest arma accesos directos (.lnk) mínimos para las pruebas de los
// paquetes que los leen. Solo lo usan las pruebas, como benchtree.
package lnktest

import (
	"encoding/binary"
	"unicode/utf16"
)

// Constantes del formato MS-SHLLINK (las mismas que lee el paquete lnk).
const (
	HeaderSize   = 0x4C
	hasLinkInfo  = 1 << 1
	isUnicode    = 1 << 7
	envBlock     = 0xA0000001
	envBlockSize = 0x314
)

// File arma un acceso directo. Con base, lleva un LinkInfo que apunta a
// base+suffix (en ANSI y, si unicode, también en UTF-16). Con env, un bloque
// de variables de entorno que apunta a env. blocks se añaden al final de
// ExtraData (por ejemplo, AppIDBlock).
func File(base, suffix string, unicode bool, env string, blocks ...[]byte) []byte {
	var flags uint32 = isUnicode
	if base != "" {
		flags |= hasLinkInfo
	}
	b := make([]byte, HeaderSize)
	binary.LittleEndian.PutUint32(b[0:], HeaderSize)
	binary.LittleEndian.PutUint32(b[20:], flags)
	if base != "" {
		b = append(b, linkInfo(base, suffix, unicode)...)
	}
	if env != "" {
		block := make([]byte, envBlockSize)
		binary.LittleEndian.PutUint32(block[0:], envBlockSize)
		binary.LittleEndian.PutUint32(block[4:], envBlock)
		copy(block[8:], latin1z(env))
		copy(block[8+260:], utf16z(env))
		b = append(b, block...)
	}
	for _, block := range blocks {
		b = append(b, block...)
	}
	return binary.LittleEndian.AppendUint32(b, 0) // TerminalBlock
}

// AppIDBlock arma un PropertyStoreDataBlock con la propiedad
// System.AppUserModel.ID igual a aumid, como el de los accesos directos de
// Office o VS Code. Antes lleva otro storage (System.Link.TargetParsingPath)
// para comprobar que el lector lo salta.
func AppIDBlock(aumid string) []byte {
	other := storage([16]byte{0xB9, 0xB4, 0xB3, 0xB4, 0x75, 0x31, 0x9E, 0x4D, 0x8D, 0xC5, 0x8D, 0x10, 0x9F, 0xE8, 0x2E, 0xBA}, 2, `C:\otro.exe`)
	appID := storage([16]byte{0x55, 0x28, 0x4C, 0x9F, 0x79, 0x9F, 0x39, 0x4B, 0xA8, 0xD0, 0xE1, 0xD4, 0x2D, 0xE1, 0xD5, 0xF3}, 5, aumid)
	store := append(append(other, appID...), 0, 0, 0, 0) // un storage de tamaño 0 cierra la lista
	block := binary.LittleEndian.AppendUint32(nil, uint32(8+len(store)))
	block = binary.LittleEndian.AppendUint32(block, 0xA0000009)
	return append(block, store...)
}

// storage arma una serialized property storage con un único valor de texto
// (VT_LPWSTR) con el identificador id.
func storage(fmtid [16]byte, id uint32, value string) []byte {
	text := utf16z(value)
	v := binary.LittleEndian.AppendUint32(nil, 0) // tamaño, se rellena después
	v = binary.LittleEndian.AppendUint32(v, id)
	v = append(v, 0)                              // reservado
	v = binary.LittleEndian.AppendUint16(v, 0x1F) // VT_LPWSTR
	v = binary.LittleEndian.AppendUint16(v, 0)    // relleno
	v = binary.LittleEndian.AppendUint32(v, uint32(len(text)/2))
	v = append(v, text...)
	for len(v)%4 != 0 {
		v = append(v, 0)
	}
	binary.LittleEndian.PutUint32(v, uint32(len(v)))
	values := append(v, 0, 0, 0, 0) // un valor de tamaño 0 cierra la lista

	s := binary.LittleEndian.AppendUint32(nil, uint32(24+len(values)))
	s = binary.LittleEndian.AppendUint32(s, 0x53505331) // "1SPS"
	s = append(s, fmtid[:]...)
	return append(s, values...)
}

func linkInfo(base, suffix string, unicode bool) []byte {
	headerSize := 0x1C
	if unicode {
		headerSize = 0x24
	}
	volume := make([]byte, 0x11) // VolumeID con la etiqueta vacía
	binary.LittleEndian.PutUint32(volume[0:], 0x11)
	binary.LittleEndian.PutUint32(volume[12:], 0x10)
	parts := [][]byte{volume, latin1z(base), latin1z(suffix)}
	if unicode {
		parts = append(parts, utf16z(base), utf16z(suffix))
	}
	offsets := make([]int, len(parts))
	size := headerSize
	for i, p := range parts {
		offsets[i] = size
		size += len(p)
	}
	info := make([]byte, headerSize)
	put := func(off, v int) { binary.LittleEndian.PutUint32(info[off:], uint32(v)) }
	put(0, size)
	put(4, headerSize)
	put(8, 1) // VolumeIDAndLocalBasePath
	put(12, offsets[0])
	put(16, offsets[1])
	put(24, offsets[2])
	if unicode {
		put(28, offsets[3])
		put(32, offsets[4])
	}
	for _, p := range parts {
		info = append(info, p...)
	}
	return info
}

func latin1z(s string) []byte {
	var b []byte
	for _, r := range s {
		b = append(b, byte(r))
	}
	return append(b, 0)
}

func utf16z(s string) []byte {
	var b []byte
	for _, u := range utf16.Encode([]rune(s)) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return append(b, 0, 0)
}
