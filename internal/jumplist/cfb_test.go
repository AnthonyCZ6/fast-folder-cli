package jumplist

import (
	"encoding/binary"
	"strings"
	"testing"
)

// cfbFile arma un compound file con sectores de 512 bytes: la cabecera, sin
// sectores de FAT ni DIFAT, y sectors sectores vacíos. put escribe un valor
// de 32 bits en una posición del archivo.
func cfbFile(sectors int) (data []byte, put func(off int, v uint32)) {
	data = make([]byte, 512*(1+sectors))
	put = func(off int, v uint32) { binary.LittleEndian.PutUint32(data[off:], v) }
	binary.LittleEndian.PutUint64(data, cfbSignature)
	binary.LittleEndian.PutUint16(data[0x1E:], 9)
	binary.LittleEndian.PutUint16(data[0x20:], 6)
	put(0x38, miniStreamCutoff)
	put(0x44, endOfChain)
	for i := range headerDIFAT {
		put(0x4C+4*i, freeSect)
	}
	return data, put
}

// sectorAt es la posición en el archivo del sector n.
func sectorAt(n int) int { return 512 * (n + 1) }

// Si la DIFAT nombra muchas veces el mismo sector de la FAT, la FAT ocuparía
// en memoria muchas veces el tamaño del archivo (un archivo de 16 MB podría
// pedir varios GB): se rechaza.
func TestOpenCFBRepeatedFATSector(t *testing.T) {
	data, put := cfbFile(2) // sector 0: FAT; sector 1: DIFAT
	put(0x4C, 0)
	put(0x44, 1)
	for i := range 512/4 - 1 {
		put(sectorAt(1)+4*i, 0) // vuelve a nombrar el sector 0
	}
	put(sectorAt(1)+512-4, endOfChain)
	if _, err := openCFB(data); err == nil || !strings.Contains(err.Error(), "se repite") {
		t.Errorf("openCFB = %v, want que el sector de la FAT se repite", err)
	}

	// También si se repite en la propia cabecera.
	data, put = cfbFile(1)
	put(0x4C, 0)
	put(0x50, 0)
	if _, err := openCFB(data); err == nil || !strings.Contains(err.Error(), "se repite") {
		t.Errorf("openCFB con la cabecera repetida = %v", err)
	}
}

func TestOpenCFBFATSectorOutsideFile(t *testing.T) {
	data, put := cfbFile(1)
	put(0x4C, 1000)
	if _, err := openCFB(data); err == nil || !strings.Contains(err.Error(), "fuera del archivo") {
		t.Errorf("openCFB = %v, want sector fuera del archivo", err)
	}
}

func TestOpenCFBDIFATLoop(t *testing.T) {
	data, put := cfbFile(1) // sector 0: DIFAT que se enlaza a sí mismo
	put(0x44, 0)
	for i := range 512/4 - 1 {
		put(sectorAt(0)+4*i, freeSect)
	}
	put(sectorAt(0)+512-4, 0)
	if _, err := openCFB(data); err == nil || !strings.Contains(err.Error(), "bucle") {
		t.Errorf("openCFB = %v, want bucle en la DIFAT", err)
	}
}

// El corte del mini stream es siempre 4096 (MS-CFB); otro valor mandaría
// streams grandes al mini stream.
func TestOpenCFBMiniStreamCutoff(t *testing.T) {
	data, put := cfbFile(1)
	put(0x38, 1<<30)
	if _, err := openCFB(data); err == nil || !strings.Contains(err.Error(), "mini stream") {
		t.Errorf("openCFB = %v, want corte del mini stream no válido", err)
	}
}
