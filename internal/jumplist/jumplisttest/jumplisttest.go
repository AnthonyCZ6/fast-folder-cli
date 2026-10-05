// Package jumplisttest arma jump lists (compound files con un DestList, como
// las que escribe Windows) para las pruebas de los paquetes que las leen. Solo
// lo usan las pruebas, como benchtree.
package jumplisttest

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"unicode/utf16"
)

// Entry es una entrada del DestList.
type Entry struct {
	Path     string
	LastUsed time.Time
	Pinned   bool
	Uses     uint32
}

// File arma una jump list con un DestList de la versión version (4 en Windows
// 10, 6 en Windows 11) y, como Windows, un stream por entrada con su número
// en hexadecimal como nombre.
func File(version uint32, entries []Entry) []byte {
	streams := []stream{{"DestList", destList(version, entries)}}
	for i := range entries {
		streams = append(streams, stream{strconv.FormatUint(uint64(i+1), 16), []byte("lnk")})
	}
	return compoundFile(streams)
}

// Write escribe en dir la jump list del AppID appID (el número que calcula
// jumplist.AppID), con el nombre que le daría Windows.
func Write(dir string, appID uint64, version uint32, entries []Entry) error {
	name := fmt.Sprintf("%x.automaticDestinations-ms", appID)
	return os.WriteFile(filepath.Join(dir, name), File(version, entries), 0o644)
}

// destList arma el stream DestList: una cabecera de 32 bytes y, por entrada,
// 130 bytes fijos, la ruta en UTF-16 y 4 bytes más.
func destList(version uint32, entries []Entry) []byte {
	b := make([]byte, 32)
	put32(b, 0, version)
	put32(b, 4, uint32(len(entries)))
	pinned := 0
	for _, e := range entries {
		if e.Pinned {
			pinned++
		}
	}
	put32(b, 8, math.Float32bits(float32(pinned)))
	put32(b, 16, uint32(len(entries))) // último número de entrada
	for i, e := range entries {
		h := make([]byte, 130)
		copy(h[72:88], "PRUEBA") // nombre NetBIOS del equipo
		put32(h, 88, uint32(i+1))
		ft := filetime(e.LastUsed)
		put32(h, 100, uint32(ft))
		put32(h, 104, uint32(ft>>32))
		pin := int32(-1)
		if e.Pinned {
			pin = 0
		}
		put32(h, 108, uint32(pin))
		put32(h, 116, e.Uses)
		units := utf16.Encode([]rune(e.Path))
		binary.LittleEndian.PutUint16(h[128:], uint16(len(units)))
		b = append(b, h...)
		for _, u := range units {
			b = binary.LittleEndian.AppendUint16(b, u)
		}
		b = append(b, 0, 0, 0, 0)
	}
	return b
}

func filetime(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}
	return uint64(t.UnixNano()/100) + 116444736000000000
}

// Compound file (MS-CFB, versión 3: sectores de 512 bytes). Los streams de
// menos de 4096 bytes van en el mini stream, como en los archivos reales.
const (
	sectorSize = 512
	miniSize   = 64
	cutoff     = 4096
	endOfChain = 0xFFFFFFFE
	freeSect   = 0xFFFFFFFF
	fatSect    = 0xFFFFFFFD
	noStream   = 0xFFFFFFFF
)

type stream struct {
	name string
	data []byte
}

// sectors reparte los datos en sectores encadenados en la FAT.
type sectors struct {
	list [][]byte
	fat  []uint32
}

// add guarda data en sectores nuevos y devuelve el primero.
func (s *sectors) add(data []byte) uint32 {
	if len(data) == 0 {
		return endOfChain
	}
	start := uint32(len(s.list))
	n := (len(data) + sectorSize - 1) / sectorSize
	for i := range n {
		sec := make([]byte, sectorSize)
		copy(sec, data[i*sectorSize:])
		s.list = append(s.list, sec)
		next := uint32(len(s.list))
		if i == n-1 {
			next = endOfChain
		}
		s.fat = append(s.fat, next)
	}
	return start
}

func compoundFile(streams []stream) []byte {
	var secs sectors
	var mini []byte
	var miniFat []uint32
	starts := make([]uint32, len(streams))
	for i, st := range streams {
		if len(st.data) >= cutoff {
			starts[i] = secs.add(st.data)
			continue
		}
		starts[i] = endOfChain
		n := (len(st.data) + miniSize - 1) / miniSize
		for j := range n {
			if j == 0 {
				starts[i] = uint32(len(mini) / miniSize)
			}
			chunk := make([]byte, miniSize)
			copy(chunk, st.data[j*miniSize:])
			mini = append(mini, chunk...)
			next := uint32(len(mini) / miniSize)
			if j == n-1 {
				next = endOfChain
			}
			miniFat = append(miniFat, next)
		}
	}
	rootStart := secs.add(mini)
	var miniFatBytes []byte
	for _, v := range miniFat {
		miniFatBytes = binary.LittleEndian.AppendUint32(miniFatBytes, v)
	}
	miniFatStart := secs.add(miniFatBytes)

	// Directorio: la raíz y, colgando de ella, los streams en fila.
	child := uint32(1)
	if len(streams) == 0 {
		child = noStream
	}
	dir := dirEntry("Root Entry", 5, rootStart, uint64(len(mini)), child, noStream)
	for i, st := range streams {
		right := uint32(i + 2)
		if i == len(streams)-1 {
			right = noStream
		}
		dir = append(dir, dirEntry(st.name, 2, starts[i], uint64(len(st.data)), noStream, right)...)
	}
	for len(dir)%sectorSize != 0 {
		dir = append(dir, dirEntry("", 0, 0, 0, noStream, noStream)...)
	}
	dirStart := secs.add(dir)

	// La FAT va al final: sus propios sectores también figuran en ella.
	n := len(secs.list)
	k := 1
	for (n+k+sectorSize/4-1)/(sectorSize/4) > k {
		k++
	}
	if k > 109 {
		panic("jumplisttest: archivo demasiado grande para la cabecera")
	}
	fat := secs.fat
	for range k {
		fat = append(fat, fatSect)
	}
	for len(fat) < k*sectorSize/4 {
		fat = append(fat, freeSect)
	}

	h := make([]byte, sectorSize)
	binary.LittleEndian.PutUint64(h[0:], 0xE11AB1A1E011CFD0)
	binary.LittleEndian.PutUint16(h[0x18:], 0x3E)
	binary.LittleEndian.PutUint16(h[0x1A:], 3)
	binary.LittleEndian.PutUint16(h[0x1C:], 0xFFFE)
	binary.LittleEndian.PutUint16(h[0x1E:], 9)
	binary.LittleEndian.PutUint16(h[0x20:], 6)
	put32(h, 0x2C, uint32(k))
	put32(h, 0x30, dirStart)
	put32(h, 0x38, cutoff)
	put32(h, 0x3C, miniFatStart)
	put32(h, 0x40, uint32((len(miniFatBytes)+sectorSize-1)/sectorSize))
	put32(h, 0x44, endOfChain)
	for i := range 109 {
		v := uint32(freeSect)
		if i < k {
			v = uint32(n + i)
		}
		put32(h, 0x4C+4*i, v)
	}

	out := h
	for _, sec := range secs.list {
		out = append(out, sec...)
	}
	for _, v := range fat {
		out = binary.LittleEndian.AppendUint32(out, v)
	}
	return out
}

func dirEntry(name string, kind byte, start uint32, size uint64, child, right uint32) []byte {
	e := make([]byte, 128)
	units := utf16.Encode([]rune(name))
	for i, u := range units {
		binary.LittleEndian.PutUint16(e[2*i:], u)
	}
	if kind != 0 {
		binary.LittleEndian.PutUint16(e[64:], uint16(2*(len(units)+1)))
	}
	e[66] = kind
	e[67] = 1 // negro
	put32(e, 68, noStream)
	put32(e, 72, right)
	put32(e, 76, child)
	put32(e, 116, start)
	binary.LittleEndian.PutUint64(e[120:], size)
	return e
}

func put32(b []byte, off int, v uint32) {
	binary.LittleEndian.PutUint32(b[off:], v)
}
