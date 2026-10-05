// Prueba de viabilidad (no se une a main): registra archivos como abiertos
// recientemente con un AppID propio y comprueba que la jump list que escribe
// Windows se lee bien: el nombre del archivo (CRC-64 del AppID), el formato
// compound file y las entradas del DestList con sus rutas.
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const appID = "FastFolderCLI.Prueba"

var (
	shell32         = syscall.NewLazyDLL("shell32.dll")
	setAppID        = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	addToRecentDocs = shell32.NewProc("SHAddToRecentDocs")
)

const sHARDPathW = 3

// testFiles son los archivos que se registran como abiertos recientemente.
var testFiles = []string{
	`Música\Canción & co\letra.txt`,
	`Universidad\Tesis final (2)\capítulo 1.docx`,
	`proyectos\api\README.md`,
	`proyectos\api\notas.txt`,
	`Año 2024\Fotos\resumen.txt`,
}

// Se ejecuta dos veces: "escribir" registra los archivos y termina (Windows
// guarda la jump list al terminar el proceso); "leer" la lee después.
func main() {
	var err error
	switch mode := os.Args[len(os.Args)-1]; mode {
	case "escribir":
		err = write()
	case "leer":
		err = read()
	default:
		err = fmt.Errorf("uso: escribir | leer (recibido %q)", mode)
	}
	if err != nil {
		fmt.Println("FALLO:", err)
		os.Exit(1)
	}
}

func testPaths() []string {
	root := filepath.Join(os.TempDir(), "jl-prueba")
	var paths []string
	for _, f := range testFiles {
		paths = append(paths, filepath.Join(root, f))
	}
	return paths
}

func write() error {
	p, _ := syscall.UTF16PtrFromString(appID)
	if r, _, _ := setAppID.Call(uintptr(unsafe.Pointer(p))); r != 0 {
		return fmt.Errorf("SetCurrentProcessExplicitAppUserModelID: 0x%X", r)
	}
	for _, path := range testPaths() {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte("prueba"), 0o644); err != nil {
			return err
		}
		ptr, _ := syscall.UTF16PtrFromString(path)
		addToRecentDocs.Call(sHARDPathW, uintptr(unsafe.Pointer(ptr)))
		time.Sleep(1100 * time.Millisecond) // fechas distintas para ver el orden
	}
	time.Sleep(5 * time.Second)
	fmt.Printf("Registrados %d archivos con el AppID %s\n", len(testFiles), appID)
	return nil
}

func read() error {
	want := testPaths()
	name := fmt.Sprintf("%x.automaticDestinations-ms", crc64AppID(appID))
	list := filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Recent\AutomaticDestinations`, name)
	fmt.Println("Jump list esperada:", list)

	var entries []destEntry
	var version uint32
	var lastErr error
	for deadline := time.Now().Add(45 * time.Second); time.Now().Before(deadline); time.Sleep(time.Second) {
		data, err := os.ReadFile(list)
		if err != nil {
			lastErr = err
			continue
		}
		c, err := openCFB(data)
		if err != nil {
			lastErr = err
			continue
		}
		dl, err := c.stream("DestList")
		if err != nil {
			lastErr = err
			continue
		}
		version, entries, lastErr = destList(dl)
		if lastErr == nil && len(entries) >= len(want) {
			fmt.Printf("Compound file: %d entradas de directorio; streams: %s\n", len(c.entries), c.names())
			fmt.Printf("DestList: versión %d, %d elementos, %d bytes\n", version, len(entries), len(dl))
			fmt.Printf("Primera entrada (cabecera, 136 bytes):\n%s\n", hexdump(dl[32:min(len(dl), 32+136)]))
			break
		}
	}
	if len(entries) < len(want) {
		listDir(filepath.Dir(list))
		return fmt.Errorf("la jump list no tiene las %d entradas (tiene %d; último error: %v)", len(want), len(entries), lastErr)
	}

	got := map[string]bool{}
	for _, e := range entries {
		pin := ""
		if e.pinned {
			pin = " [anclado]"
		}
		fmt.Printf("  #%-3x %s  usos=%d%s  %s\n", e.number, e.last.Format(time.RFC3339), e.count, pin, e.path)
		got[strings.ToLower(e.path)] = true
		if e.last.IsZero() || time.Since(e.last) > time.Hour || time.Since(e.last) < -time.Minute {
			return fmt.Errorf("fecha inesperada en %q: %v", e.path, e.last)
		}
	}
	for _, w := range want {
		if !got[strings.ToLower(w)] {
			return fmt.Errorf("falta %q en el DestList", w)
		}
	}
	fmt.Println("CORRECTO: AppID, compound file y DestList coinciden con lo registrado.")
	listDir(filepath.Dir(list))
	return nil
}

// listDir muestra cada jump list de dir con sus primeras entradas.
func listDir(dir string) {
	entries, _ := os.ReadDir(dir)
	fmt.Printf("Contenido de %s (%d archivos):\n", dir, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		fmt.Printf("  %s %d B", e.Name(), info.Size())
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			fmt.Println(" error:", err)
			continue
		}
		c, err := openCFB(data)
		if err != nil {
			fmt.Println(" error:", err)
			continue
		}
		dl, err := c.stream("DestList")
		if err != nil {
			fmt.Printf(" sin DestList (%v); streams: %s\n", err, c.names())
			continue
		}
		v, items, err := destList(dl)
		fmt.Printf(" DestList v%d, %d elementos, %d bytes, error=%v\n", v, len(items), len(dl), err)
		for i, it := range items {
			if i == 5 {
				break
			}
			fmt.Printf("      %s  %s\n", it.last.Format(time.RFC3339), it.path)
		}
	}
}

func hexdump(b []byte) string {
	var sb strings.Builder
	for i := 0; i < len(b); i += 16 {
		fmt.Fprintf(&sb, "  %04x  % x\n", i, b[i:min(i+16, len(b))])
	}
	return sb.String()
}

// crc64AppID es el nombre de la jump list de un AppID: CRC-64 reflejado con
// el polinomio 0x92C64265D32139A4 y valor inicial ~0, en mayúsculas UTF-16LE.
func crc64AppID(s string) uint64 {
	c := ^uint64(0)
	for _, u := range utf16.Encode([]rune(strings.ToUpper(s))) {
		for _, b := range []byte{byte(u), byte(u >> 8)} {
			c ^= uint64(b)
			for i := 0; i < 8; i++ {
				if c&1 != 0 {
					c = c>>1 ^ 0x92C64265D32139A4
				} else {
					c >>= 1
				}
			}
		}
	}
	return c
}

// --- Lector mínimo de compound files (OLE/CFB) ---

const (
	endOfChain = 0xFFFFFFFE
	freeSect   = 0xFFFFFFFF
)

type cfb struct {
	data       []byte
	sectorSize int
	miniSize   int
	cutoff     uint32
	fat        []uint32
	miniFat    []uint32
	ministream []byte
	entries    []dirEntry
}

type dirEntry struct {
	name  string
	kind  byte
	start uint32
	size  uint64
}

func le32(b []byte, o int) uint32 { return binary.LittleEndian.Uint32(b[o:]) }

func (c *cfb) names() string {
	var n []string
	for _, e := range c.entries {
		if e.kind == 2 {
			n = append(n, e.name)
		}
	}
	return strings.Join(n, ",")
}

func (c *cfb) sector(n uint32) ([]byte, error) {
	off := (int(n) + 1) * c.sectorSize
	if off < 0 || off+c.sectorSize > len(c.data) {
		return nil, fmt.Errorf("sector %d fuera del archivo", n)
	}
	return c.data[off : off+c.sectorSize], nil
}

func (c *cfb) chain(start uint32, fat []uint32, read func(uint32) ([]byte, error)) ([]byte, error) {
	var out []byte
	for n, steps := start, 0; n != endOfChain; steps++ {
		if int(n) >= len(fat) || steps > len(fat) {
			return nil, errors.New("cadena rota o en bucle")
		}
		b, err := read(n)
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
		n = fat[n]
	}
	return out, nil
}

func openCFB(data []byte) (*cfb, error) {
	if len(data) < 512 || binary.LittleEndian.Uint64(data) != 0xE11AB1A1E011CFD0 {
		return nil, errors.New("no es un compound file")
	}
	c := &cfb{data: data, sectorSize: 1 << binary.LittleEndian.Uint16(data[0x1E:]), miniSize: 1 << binary.LittleEndian.Uint16(data[0x20:]), cutoff: le32(data, 0x38)}
	if c.sectorSize != 512 && c.sectorSize != 4096 {
		return nil, errors.New("tamaño de sector no válido")
	}
	var fatSectors []uint32
	for i := 0; i < 109; i++ {
		if s := le32(data, 0x4C+4*i); s != freeSect {
			fatSectors = append(fatSectors, s)
		}
	}
	for d, n := le32(data, 0x44), 0; d != endOfChain && d != freeSect && n < 1000; n++ {
		b, err := c.sector(d)
		if err != nil {
			return nil, err
		}
		for i := 0; i < c.sectorSize/4-1; i++ {
			if s := le32(b, 4*i); s != freeSect {
				fatSectors = append(fatSectors, s)
			}
		}
		d = le32(b, c.sectorSize-4)
	}
	for _, s := range fatSectors {
		b, err := c.sector(s)
		if err != nil {
			return nil, err
		}
		for i := 0; i < c.sectorSize/4; i++ {
			c.fat = append(c.fat, le32(b, 4*i))
		}
	}
	dir, err := c.chain(le32(data, 0x30), c.fat, c.sector)
	if err != nil {
		return nil, fmt.Errorf("directorio: %w", err)
	}
	for o := 0; o+128 <= len(dir); o += 128 {
		e := dir[o : o+128]
		nameLen := min(int(binary.LittleEndian.Uint16(e[64:])), 64)
		var u []uint16
		for i := 0; i+1 < nameLen-1; i += 2 {
			u = append(u, binary.LittleEndian.Uint16(e[i:]))
		}
		size := binary.LittleEndian.Uint64(e[120:])
		if c.sectorSize == 512 {
			size &= 0xFFFFFFFF
		}
		c.entries = append(c.entries, dirEntry{name: string(utf16.Decode(u)), kind: e[66], start: le32(e, 116), size: size})
	}
	if len(c.entries) == 0 || c.entries[0].kind != 5 {
		return nil, errors.New("sin entrada raíz")
	}
	if c.entries[0].start != endOfChain {
		if c.ministream, err = c.chain(c.entries[0].start, c.fat, c.sector); err != nil {
			return nil, fmt.Errorf("mini stream: %w", err)
		}
	}
	if mf := le32(data, 0x3C); mf != endOfChain {
		b, err := c.chain(mf, c.fat, c.sector)
		if err != nil {
			return nil, fmt.Errorf("mini FAT: %w", err)
		}
		for i := 0; i+4 <= len(b); i += 4 {
			c.miniFat = append(c.miniFat, le32(b, i))
		}
	}
	return c, nil
}

func (c *cfb) stream(name string) ([]byte, error) {
	for _, e := range c.entries {
		if e.kind != 2 || e.name != name {
			continue
		}
		var b []byte
		var err error
		if e.size < uint64(c.cutoff) {
			b, err = c.chain(e.start, c.miniFat, func(n uint32) ([]byte, error) {
				off := int(n) * c.miniSize
				if off+c.miniSize > len(c.ministream) {
					return nil, errors.New("mini sector fuera")
				}
				return c.ministream[off : off+c.miniSize], nil
			})
		} else {
			b, err = c.chain(e.start, c.fat, c.sector)
		}
		if err != nil {
			return nil, err
		}
		if uint64(len(b)) < e.size {
			return nil, errors.New("stream más corto que su tamaño")
		}
		return b[:e.size], nil
	}
	return nil, os.ErrNotExist
}

// --- DestList ---

type destEntry struct {
	number uint32
	path   string
	last   time.Time
	pinned bool
	count  uint32
}

func filetime(v uint64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(0, int64(v-116444736000000000)*100).UTC()
}

func destList(b []byte) (version uint32, out []destEntry, err error) {
	if len(b) < 32 {
		return 0, nil, errors.New("DestList demasiado corto")
	}
	version = le32(b, 0)
	n := int(le32(b, 4))
	o := 32
	for i := 0; i < n; i++ {
		if o+130 > len(b) {
			return version, out, fmt.Errorf("entrada %d recortada", i)
		}
		chars := int(binary.LittleEndian.Uint16(b[o+128:]))
		end := o + 130 + 2*chars
		if end > len(b) {
			return version, out, fmt.Errorf("ruta %d recortada", i)
		}
		var u []uint16
		for j := o + 130; j < end; j += 2 {
			u = append(u, binary.LittleEndian.Uint16(b[j:]))
		}
		out = append(out, destEntry{
			number: le32(b, o+88),
			last:   filetime(binary.LittleEndian.Uint64(b[o+100:])),
			pinned: int32(le32(b, o+108)) != -1,
			count:  le32(b, o+116),
			path:   string(utf16.Decode(u)),
		})
		o = end
		if version >= 3 {
			o += 4
		}
	}
	return version, out, nil
}
