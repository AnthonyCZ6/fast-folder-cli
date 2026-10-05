package lnk

import (
	"encoding/binary"
	"testing"
	"unicode/utf8"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/lnk/lnktest"
)

func TestTarget(t *testing.T) {
	full := lnktest.File(`C:\Program Files\Mí App\`, "app.exe", false, "")
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"ANSI con acentos", full, `C:\Program Files\Mí App\app.exe`},
		{"Unicode", lnktest.File(`C:\Apps\日本\`, "app.exe", true, ""), `C:\Apps\日本\app.exe`},
		{"solo variables de entorno", lnktest.File("", "", false, `%ProgramFiles%\Tool\tool.exe`), `%ProgramFiles%\Tool\tool.exe`},
		{"anunciado (sin destino)", lnktest.File("", "", false, ""), ""},
		{"recortado", full[:lnktest.HeaderSize+10], ""},
		{"no es un .lnk", []byte("hola"), ""},
		{"solo la cabecera, con LinkInfo", lnktest.File(`C:\a\`, "b.exe", false, "")[:lnktest.HeaderSize], ""},
		{"bloque de tamaño 7", binary.LittleEndian.AppendUint32(lnktest.File("", "", false, "")[:lnktest.HeaderSize], 7), ""},
	}
	for _, tt := range tests {
		if got := Target(tt.data); got != tt.want {
			t.Errorf("%s: Target = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestAppUserModelID(t *testing.T) {
	word := lnktest.File(`C:\Program Files\Microsoft Office\root\Office16\`, "WINWORD.EXE", true, "",
		lnktest.AppIDBlock("Microsoft.Office.WINWORD.EXE.15"))
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"con LinkInfo", word, "Microsoft.Office.WINWORD.EXE.15"},
		{"anunciado, sin destino", lnktest.File("", "", false, "", lnktest.AppIDBlock("Chrome")), "Chrome"},
		{"tras el bloque de variables", lnktest.File("", "", false, `%LOCALAPPDATA%\x.exe`, lnktest.AppIDBlock("Microsoft.VisualStudioCode")), "Microsoft.VisualStudioCode"},
		{"sin AppUserModelID", lnktest.File(`C:\a\`, "b.exe", false, ""), ""},
		{"recortado", word[:len(word)-40], ""},
		{"no es un .lnk", []byte("hola"), ""},
	}
	for _, tt := range tests {
		if got := AppUserModelID(tt.data); got != tt.want {
			t.Errorf("%s: AppUserModelID = %q, want %q", tt.name, got, tt.want)
		}
	}
	if got := Target(word); got != `C:\Program Files\Microsoft Office\root\Office16\WINWORD.EXE` {
		t.Errorf("el bloque de propiedades no debe cambiar el destino: %q", got)
	}
}

// FuzzTarget comprueba que ningún archivo, por dañado o malicioso que sea,
// hace fallar a Target ni a AppUserModelID: o dan texto en UTF-8 válido, o "".
func FuzzTarget(f *testing.F) {
	f.Add(lnktest.File(`C:\Program Files\Mí App\`, "app.exe", false, ""))
	f.Add(lnktest.File(`C:\Apps\日本\`, "app.exe", true, ""))
	f.Add(lnktest.File("", "", false, `%ProgramFiles%\Tool\tool.exe`))
	f.Add(lnktest.File(`C:\Office\`, "WINWORD.EXE", true, "", lnktest.AppIDBlock("Microsoft.Office.WINWORD.EXE.15")))
	f.Add([]byte("hola"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if got := Target(data); !utf8.ValidString(got) {
			t.Errorf("Target devolvió UTF-8 no válido: %q", got)
		}
		if got := AppUserModelID(data); !utf8.ValidString(got) {
			t.Errorf("AppUserModelID devolvió UTF-8 no válido: %q", got)
		}
	})
}
