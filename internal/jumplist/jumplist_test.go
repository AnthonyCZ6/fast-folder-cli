package jumplist

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist/jumplisttest"
)

// Los valores se comprobaron con las jump lists que crea Windows 10 y 11 (el
// nombre del archivo de cada programa).
func TestAppID(t *testing.T) {
	tests := []struct {
		id   string
		want uint64
	}{
		{"Microsoft.Windows.Explorer", 0xf01b4d95cf55d32a},
		{"microsoft.windows.explorer", 0xf01b4d95cf55d32a}, // no distingue mayúsculas
		{"Microsoft.Office.WINWORD.EXE.15", 0xfb3b0dbfee58fac8},
		{"Microsoft.VisualStudioCode", 0x1ced32d74a95c7bc},
		{"Chrome", 0x5d696d521de238c3},
		{`{6D809377-6AF0-444B-8957-A3773F02200E}\QuickCPU\QuickCPU.exe`, 0x72c603164537e056},
		{"FastFolderCLI.Prueba", 0x33d189571799a8da},
	}
	for _, tt := range tests {
		if got := AppID(tt.id); got != tt.want {
			t.Errorf("AppID(%q) = %x, want %x", tt.id, got, tt.want)
		}
	}
}

// entriesAt crea n entradas con rutas difíciles y fechas distintas.
func entriesAt(n int) []jumplisttest.Entry {
	base := time.Date(2026, 10, 5, 8, 30, 15, 123456700, time.UTC)
	var out []jumplisttest.Entry
	for i := range n {
		out = append(out, jumplisttest.Entry{
			Path:     fmt.Sprintf(`C:\Música\Canción & co %d\letra.txt`, i),
			LastUsed: base.Add(-time.Duration(i) * time.Hour),
			Pinned:   i == 0,
			Uses:     uint32(i + 1),
		})
	}
	return out
}

func toEntries(in []jumplisttest.Entry) []Entry {
	out := []Entry{}
	for _, e := range in {
		out = append(out, Entry{Path: e.Path, LastUsed: e.LastUsed, Pinned: e.Pinned, Uses: e.Uses})
	}
	return out
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		version uint32
		n       int
	}{
		{"Windows 10, pocas (mini stream)", 4, 3},
		{"Windows 11, pocas (mini stream)", 6, 3},
		{"Windows 11, muchas (sectores normales)", 6, 40},
		{"vacía", 6, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := entriesAt(tt.n)
			got, err := Parse(jumplisttest.File(tt.version, want))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, toEntries(want)) {
				t.Errorf("Parse =\n%+v\nwant\n%+v", got, toEntries(want))
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	full := jumplisttest.File(6, entriesAt(40))
	for name, data := range map[string][]byte{
		"no es un compound file": []byte("hola"),
		"recortado":              full[:1024],
		"sin datos":              nil,
	} {
		if _, err := Parse(data); err == nil {
			t.Errorf("%s: Parse no devolvió error", name)
		}
	}
}

func TestParseDestListErrors(t *testing.T) {
	header := func(version, n uint32) []byte {
		b := make([]byte, destHeaderSize)
		b[0], b[4] = byte(version), byte(n)
		b[5] = byte(n >> 8)
		return b
	}
	tests := map[string][]byte{
		"demasiado corto":      {1, 2, 3},
		"versión de Windows 7": header(1, 0),
		"demasiadas entradas":  header(4, 0xFFFF),
	}
	for name, b := range tests {
		if _, err := parseDestList(b); err == nil {
			t.Errorf("%s: parseDestList no devolvió error", name)
		}
	}
}

// Una entrada recortada (por ejemplo, de una lista que Windows está
// escribiendo) termina la lista, sin perder las entradas completas
// anteriores.
func TestParseDestListTruncated(t *testing.T) {
	c, err := openCFB(jumplisttest.File(6, entriesAt(3)))
	if err != nil {
		t.Fatal(err)
	}
	dl, err := c.stream("DestList")
	if err != nil {
		t.Fatal(err)
	}
	want := toEntries(entriesAt(3))
	lastPath := 2 * len(utf16.Encode([]rune(want[2].Path))) // bytes de la última ruta
	for name, cut := range map[string]int{
		"ruta recortada":    len(dl) - 10,
		"entrada recortada": len(dl) - 4 - lastPath - destFixedSize/2,
	} {
		got, err := parseDestList(dl[:cut])
		if err != nil || !reflect.DeepEqual(got, want[:2]) {
			t.Errorf("%s: parseDestList = %+v, %v; want las 2 primeras", name, got, err)
		}
	}
}

// Un bucle en la FAT no debe colgar la lectura ni reservar memoria sin fin.
func TestChainLoop(t *testing.T) {
	c := &cfb{sectorSize: 512, data: make([]byte, 512*4)}
	fat := []uint32{1, 2, 0} // 0 → 1 → 2 → 0
	if _, err := c.chain(0, fat, c.sector); err == nil || !strings.Contains(err.Error(), "bucle") {
		t.Errorf("chain con un bucle: %v", err)
	}
	if _, err := c.chain(7, fat, c.sector); err == nil {
		t.Error("chain fuera de la tabla no devolvió error")
	}
}

func TestReadDir(t *testing.T) {
	dir := t.TempDir()
	word := entriesAt(2)
	if err := jumplisttest.Write(dir, AppID("Microsoft.Office.WINWORD.EXE.15"), 6, word); err != nil {
		t.Fatal(err)
	}
	if err := jumplisttest.Write(dir, AppID("Chrome"), 4, nil); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"1234" + Ext:          "dañado",
		"no-es-hex" + Ext:     "se ignora",
		"leeme.txt":           "se ignora",
		"abcd.customDest-ms":  "se ignora",
		"ffff" + Ext + ".bak": "se ignora",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	lists, damaged, err := ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if damaged != 1 {
		t.Errorf("damaged = %d, want 1", damaged)
	}
	got := map[uint64][]Entry{}
	for _, l := range lists {
		got[l.AppID] = l.Entries
	}
	want := map[uint64][]Entry{
		AppID("Microsoft.Office.WINWORD.EXE.15"): toEntries(word),
		AppID("Chrome"):                          {},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadDir =\n%+v\nwant\n%+v", got, want)
	}

	if _, _, err := ReadDir(filepath.Join(dir, "no-existe")); err == nil {
		t.Error("ReadDir de una carpeta que no existe no devolvió error")
	}
}

func TestDir(t *testing.T) {
	t.Setenv(EnvDir, `D:\otras`)
	if got, err := Dir(); err != nil || got != `D:\otras` {
		t.Errorf("con %s: Dir = %q, %v", EnvDir, got, err)
	}
}

func TestLocalPath(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "Canción")
	tests := []struct {
		path, want string
	}{
		{abs, abs},
		{"https://ejemplo.com/informe.xlsx", ""},
		{`\\servidor\compartida\informe.docx`, ""},
		{"//servidor/compartida/informe.docx", ""},
		{`docs\relativa.txt`, ""},
		{"::{26EE0668-A00A-44D7-9371-BEB064C98683}", ""},
		{"knownfolder:{00000000-0000-0000-0000-000000000000}", ""},
		{"knownfolder:no-es-un-guid", ""},
	}
	for _, tt := range tests {
		if got := (Entry{Path: tt.path}).LocalPath(); got != tt.want {
			t.Errorf("LocalPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// FuzzParse comprueba que ningún archivo, por dañado o malicioso que sea,
// hace fallar a Parse ni le hace devolver más entradas de las permitidas.
func FuzzParse(f *testing.F) {
	f.Add(jumplisttest.File(4, entriesAt(3)))
	f.Add(jumplisttest.File(6, entriesAt(40)))
	f.Add([]byte("hola"))
	f.Fuzz(func(t *testing.T, data []byte) {
		entries, err := Parse(data)
		if err == nil && len(entries) > maxEntries {
			t.Errorf("Parse devolvió %d entradas", len(entries))
		}
	})
}
