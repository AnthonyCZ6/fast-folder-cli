package jumplist

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf16"
)

// Lector mínimo de compound files (formato OLE/CFB, MS-CFB), el contenedor
// de las jump lists: solo lo necesario para leer un stream por su nombre. El
// archivo viene de fuera, así que nada de lo que contenga puede hacer que se
// lea fuera de él ni que se reserve más memoria que su propio tamaño: cada
// sector se usa como mucho una vez en la FAT y en cada cadena.

const (
	cfbSignature     = 0xE11AB1A1E011CFD0
	endOfChain       = 0xFFFFFFFE
	freeSect         = 0xFFFFFFFF
	headerDIFAT      = 109  // entradas de la DIFAT que caben en la cabecera
	miniStreamCutoff = 4096 // los streams más pequeños están en el mini stream
	dirEntrySize     = 128
	kindStream       = 2
	kindRoot         = 5
)

var errNotCFB = errors.New("no es un compound file")

// cfb es un compound file abierto.
type cfb struct {
	data       []byte
	sectorSize int
	miniSize   int
	fat        []uint32
	miniFat    []uint32
	ministream []byte
	entries    []dirEntry
}

// dirEntry es una entrada del directorio: un stream o la raíz.
type dirEntry struct {
	name  string
	kind  byte
	start uint32
	size  uint64
}

// openCFB lee la cabecera, la FAT, el directorio y el mini stream de data.
func openCFB(data []byte) (*cfb, error) {
	if len(data) < 512 || binary.LittleEndian.Uint64(data) != cfbSignature {
		return nil, errNotCFB
	}
	c := &cfb{data: data}
	switch shift := le16(data, 0x1E); shift {
	case 9, 12: // 512 o 4096 bytes
		c.sectorSize = 1 << shift
	default:
		return nil, fmt.Errorf("tamaño de sector no válido (2^%d)", shift)
	}
	if shift := le16(data, 0x20); shift != 6 {
		return nil, fmt.Errorf("tamaño de mini sector no válido (2^%d)", shift)
	}
	c.miniSize = 64
	if cutoff := le32(data, 0x38); cutoff != miniStreamCutoff {
		return nil, fmt.Errorf("corte del mini stream no válido (%d)", cutoff)
	}
	if err := c.readFAT(); err != nil {
		return nil, err
	}
	if err := c.readDirectory(); err != nil {
		return nil, err
	}
	if err := c.readMini(); err != nil {
		return nil, err
	}
	return c, nil
}

// readFAT junta los sectores de la FAT: los que nombra la cabecera y los de
// la cadena DIFAT. Cada uno debe estar dentro del archivo y aparecer una sola
// vez; si no, un archivo pequeño que repitiera un sector miles de veces haría
// reservar varios GB.
func (c *cfb) readFAT() error {
	var sectors [][]byte
	listed := map[uint32]bool{}
	add := func(s uint32) error {
		if s == freeSect {
			return nil
		}
		if listed[s] {
			return fmt.Errorf("FAT: el sector %d se repite", s)
		}
		b, err := c.sector(s)
		if err != nil {
			return fmt.Errorf("FAT: %w", err)
		}
		listed[s] = true
		sectors = append(sectors, b)
		return nil
	}
	for i := range headerDIFAT {
		if err := add(le32(c.data, 0x4C+4*i)); err != nil {
			return err
		}
	}
	perSector := c.sectorSize/4 - 1 // el último valor enlaza el siguiente sector DIFAT
	seen := map[uint32]bool{}
	for d := le32(c.data, 0x44); d != endOfChain && d != freeSect; {
		if seen[d] {
			return errors.New("la DIFAT forma un bucle")
		}
		seen[d] = true
		b, err := c.sector(d)
		if err != nil {
			return fmt.Errorf("DIFAT: %w", err)
		}
		for i := range perSector {
			if err := add(le32(b, 4*i)); err != nil {
				return err
			}
		}
		d = le32(b, c.sectorSize-4)
	}
	c.fat = make([]uint32, 0, len(sectors)*c.sectorSize/4)
	for _, b := range sectors {
		for i := 0; i < c.sectorSize; i += 4 {
			c.fat = append(c.fat, le32(b, i))
		}
	}
	return nil
}

// readDirectory lee las entradas del directorio. La primera es la raíz.
func (c *cfb) readDirectory() error {
	dir, err := c.chain(le32(c.data, 0x30), c.fat, c.sector)
	if err != nil {
		return fmt.Errorf("directorio: %w", err)
	}
	for o := 0; o+dirEntrySize <= len(dir); o += dirEntrySize {
		e := dir[o : o+dirEntrySize]
		nameLen := min(int(le16(e, 64)), 64) // en bytes, con el cero final
		var units []uint16
		for i := 0; i+1 < nameLen-1; i += 2 {
			units = append(units, le16(e, i))
		}
		size := binary.LittleEndian.Uint64(e[120:])
		if c.sectorSize == 512 {
			size &= 0xFFFFFFFF // versión 3: los 4 bytes altos no cuentan
		}
		c.entries = append(c.entries, dirEntry{
			name:  string(utf16.Decode(units)),
			kind:  e[66],
			start: le32(e, 116),
			size:  size,
		})
	}
	if len(c.entries) == 0 || c.entries[0].kind != kindRoot {
		return errors.New("el directorio no empieza por la raíz")
	}
	return nil
}

// readMini lee el mini stream (los datos de la raíz) y la mini FAT.
func (c *cfb) readMini() error {
	if root := c.entries[0]; root.start != endOfChain && root.size > 0 {
		b, err := c.chain(root.start, c.fat, c.sector)
		if err != nil {
			return fmt.Errorf("mini stream: %w", err)
		}
		c.ministream = b[:min(uint64(len(b)), root.size)]
	}
	if first := le32(c.data, 0x3C); first != endOfChain && first != freeSect {
		b, err := c.chain(first, c.fat, c.sector)
		if err != nil {
			return fmt.Errorf("mini FAT: %w", err)
		}
		for i := 0; i+4 <= len(b); i += 4 {
			c.miniFat = append(c.miniFat, le32(b, i))
		}
	}
	return nil
}

// errNoStream indica que el compound file no tiene el stream pedido.
var errNoStream = errors.New("no existe el stream")

// stream devuelve el contenido del stream name.
func (c *cfb) stream(name string) ([]byte, error) {
	for _, e := range c.entries {
		if e.kind != kindStream || e.name != name {
			continue
		}
		var b []byte
		var err error
		if e.size < miniStreamCutoff {
			b, err = c.chain(e.start, c.miniFat, c.miniSector)
		} else {
			b, err = c.chain(e.start, c.fat, c.sector)
		}
		if err != nil {
			return nil, fmt.Errorf("stream %s: %w", name, err)
		}
		if uint64(len(b)) < e.size {
			return nil, fmt.Errorf("stream %s: más corto que su tamaño", name)
		}
		return b[:e.size], nil
	}
	return nil, fmt.Errorf("%w %s", errNoStream, name)
}

// chain sigue una cadena de fat desde start y junta sus sectores (que lee
// read). Un sector que se repite es un error: así una cadena nunca puede ser
// más larga que el propio archivo.
func (c *cfb) chain(start uint32, fat []uint32, read func(uint32) ([]byte, error)) ([]byte, error) {
	var out []byte
	seen := make(map[uint32]bool)
	for n := start; n != endOfChain; n = fat[n] {
		if int64(n) >= int64(len(fat)) {
			return nil, fmt.Errorf("sector %d fuera de la tabla", n)
		}
		if seen[n] {
			return nil, errors.New("la cadena de sectores forma un bucle")
		}
		seen[n] = true
		b, err := read(n)
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	return out, nil
}

// sector devuelve el sector n del archivo (el 0 empieza tras la cabecera).
func (c *cfb) sector(n uint32) ([]byte, error) {
	off := (int64(n) + 1) * int64(c.sectorSize)
	if off+int64(c.sectorSize) > int64(len(c.data)) {
		return nil, fmt.Errorf("sector %d fuera del archivo", n)
	}
	return c.data[off : off+int64(c.sectorSize)], nil
}

// miniSector devuelve el mini sector n del mini stream.
func (c *cfb) miniSector(n uint32) ([]byte, error) {
	off := int64(n) * int64(c.miniSize)
	if off+int64(c.miniSize) > int64(len(c.ministream)) {
		return nil, fmt.Errorf("mini sector %d fuera del mini stream", n)
	}
	return c.ministream[off : off+int64(c.miniSize)], nil
}

// le32 y le16 leen enteros little-endian; fuera de los límites devuelven 0.
func le32(b []byte, off int) uint32 {
	if off < 0 || off+4 > len(b) {
		return 0
	}
	return binary.LittleEndian.Uint32(b[off:])
}

func le16(b []byte, off int) uint16 {
	if off < 0 || off+2 > len(b) {
		return 0
	}
	return binary.LittleEndian.Uint16(b[off:])
}
