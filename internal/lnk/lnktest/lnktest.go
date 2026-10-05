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
// de variables de entorno que apunta a env.
func File(base, suffix string, unicode bool, env string) []byte {
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
	return binary.LittleEndian.AppendUint32(b, 0) // TerminalBlock
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
