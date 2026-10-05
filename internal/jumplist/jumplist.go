// Package jumplist lee las jump lists de Windows: la lista que guarda Windows
// de lo último que se abrió con cada programa (la que muestra la barra de
// tareas al hacer clic derecho en él) y la de los archivos recientes en
// general. Cada lista es un archivo <AppID>.automaticDestinations-ms en
// %APPDATA%\Microsoft\Windows\Recent\AutomaticDestinations: un compound file
// (formato OLE) con un índice, el stream DestList, que dice qué se abrió,
// cuándo y si está anclado.
//
// Solo se lee: el paquete nunca modifica las listas. Los archivos vienen de
// fuera, así que uno dañado se descarta sin hacer fallar la lectura.
package jumplist

import (
	"errors"
	"fmt"
	"hash/crc64"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

// Ext es la extensión de las jump lists automáticas.
const Ext = ".automaticDestinations-ms"

// MaxSize es el tamaño máximo de una jump list que se lee: las reales ocupan
// desde unos pocos KB hasta un par de MB.
const MaxSize = 16 << 20

// EnvDir es la variable de entorno que indica otra carpeta de jump lists (por
// ejemplo, en las pruebas).
const EnvDir = "FFC_JUMPLIST_DIR"

// Entry es un elemento de una jump list: un archivo o una carpeta que se
// abrió con el programa.
type Entry struct {
	Path     string    // tal como lo guarda Windows: una ruta, knownfolder:{GUID} o una URL
	LastUsed time.Time // última vez que se abrió (UTC)
	Pinned   bool      // anclado en la jump list
	Uses     uint32    // veces que se abrió
}

// List es la jump list de un programa.
type List struct {
	AppID   uint64 // ver AppID: el nombre del archivo
	Entries []Entry
}

// appIDTable es la tabla del CRC-64 con el que Windows nombra las jump lists.
var appIDTable = crc64.MakeTable(0x92C64265D32139A4)

// AppID devuelve el número con el que Windows nombra la jump list del
// programa id: su AppUserModelID ("Microsoft.Office.WINWORD.EXE.15") o la
// ruta de su ejecutable. Es un CRC-64 (reflejado, polinomio 0x92C64265D32139A4,
// valor inicial ~0 y sin XOR final) del identificador en mayúsculas y
// UTF-16LE. Microsoft no lo documenta: se comprobó con las listas que crea
// Windows 10 y 11.
func AppID(id string) uint64 {
	units := utf16.Encode([]rune(strings.ToUpper(id)))
	b := make([]byte, 0, 2*len(units))
	for _, u := range units {
		b = append(b, byte(u), byte(u>>8))
	}
	// crc64.Checksum invierte el valor inicial y el final; Windows solo el
	// inicial.
	return ^crc64.Checksum(b, appIDTable)
}

// Parse lee las entradas de una jump list (el contenido de un archivo
// .automaticDestinations-ms), de la más reciente a la más antigua tal como
// las ordena Windows.
func Parse(data []byte) ([]Entry, error) {
	if len(data) > MaxSize {
		return nil, fmt.Errorf("demasiado grande (%d bytes)", len(data))
	}
	c, err := openCFB(data)
	if err != nil {
		return nil, err
	}
	dl, err := c.stream("DestList")
	if errors.Is(err, errNoStream) {
		return nil, nil // una lista recién creada aún no tiene índice
	}
	if err != nil {
		return nil, err
	}
	return parseDestList(dl)
}

// Formato del DestList (versiones 3 y siguientes: Windows 10 y 11): una
// cabecera de 32 bytes con la versión y el número de entradas, y cada entrada
// con una parte fija de 130 bytes, la ruta en UTF-16 y 4 bytes más.
const (
	destHeaderSize = 32
	destFixedSize  = 130
	destMinVersion = 3
	maxEntries     = 10000 // las listas reales tienen unas decenas
	filetimeEpoch  = 116444736000000000
)

// parseDestList lee las entradas de un stream DestList. Una entrada recortada
// (por ejemplo, de una lista que Windows está escribiendo) termina la lista:
// se devuelven las completas anteriores.
func parseDestList(b []byte) ([]Entry, error) {
	if len(b) < destHeaderSize {
		return nil, errors.New("DestList demasiado corto")
	}
	if v := le32(b, 0); v < destMinVersion {
		return nil, fmt.Errorf("DestList de la versión %d, anterior a Windows 10", v)
	}
	n := le32(b, 4)
	if n > maxEntries {
		return nil, fmt.Errorf("DestList con %d entradas", n)
	}
	entries := make([]Entry, 0, n)
	for i, o := uint32(0), destHeaderSize; i < n; i++ {
		if o+destFixedSize > len(b) {
			break
		}
		end := o + destFixedSize + 2*int(le16(b, o+128))
		if end > len(b) {
			break
		}
		units := make([]uint16, 0, (end-o-destFixedSize)/2)
		for j := o + destFixedSize; j < end; j += 2 {
			units = append(units, le16(b, j))
		}
		entries = append(entries, Entry{
			Path:     string(utf16.Decode(units)),
			LastUsed: filetime(uint64(le32(b, o+100)) | uint64(le32(b, o+104))<<32),
			Pinned:   int32(le32(b, o+108)) != -1,
			Uses:     le32(b, o+116),
		})
		o = end + 4
	}
	return entries, nil
}

// filetime convierte un FILETIME (centenas de nanosegundos desde 1601) a
// time.Time en UTC; cero si no es una fecha posterior a 1970.
func filetime(v uint64) time.Time {
	if v <= filetimeEpoch {
		return time.Time{}
	}
	ticks := v - filetimeEpoch
	return time.Unix(int64(ticks/10_000_000), int64(ticks%10_000_000)*100).UTC()
}

// ReadDir lee las jump lists de dir. Las que están dañadas o no se pueden
// leer se saltan y se cuentan en damaged; solo devuelve un error si no puede
// leer la carpeta.
func ReadDir(dir string) (lists []List, damaged int, err error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, err
	}
	for _, f := range files {
		name := f.Name()
		hex, ok := strings.CutSuffix(name, Ext)
		if !ok || f.IsDir() {
			continue
		}
		id, err := strconv.ParseUint(hex, 16, 64)
		if err != nil {
			continue
		}
		entries, err := readFile(filepath.Join(dir, name))
		if err != nil {
			damaged++
			continue
		}
		lists = append(lists, List{AppID: id, Entries: entries})
	}
	return lists, damaged, nil
}

// readFile lee y analiza una jump list, sin cargar las que superan MaxSize.
func readFile(path string) ([]Entry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > MaxSize {
		return nil, fmt.Errorf("%s: demasiado grande", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Dir devuelve la carpeta de las jump lists: la de EnvDir o, si no está
// definida, la de Windows.
func Dir() (string, error) {
	if dir := os.Getenv(EnvDir); dir != "" {
		return dir, nil
	}
	return defaultDir()
}

// LocalPath devuelve la ruta local de la entrada: la propia ruta, o la de la
// carpeta conocida si es knownfolder:{GUID}. Devuelve "" si no es una ruta
// local: una URL, un elemento del Panel de control, una carpeta conocida que
// este equipo no tiene o una ruta de red (\\servidor\... o una unidad de red
// como Z:), porque comprobar si existe podría bloquear la búsqueda o
// conectarse a ese servidor.
func (e Entry) LocalPath() string {
	if guid, ok := strings.CutPrefix(e.Path, "knownfolder:"); ok {
		return knownFolder(guid)
	}
	p := e.Path
	if strings.Contains(p, "://") || strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//") || !filepath.IsAbs(p) || remoteDrive(p) {
		return ""
	}
	return p
}
