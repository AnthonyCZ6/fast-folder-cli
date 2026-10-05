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

// FuzzTarget comprueba que ningún archivo, por dañado o malicioso que sea,
// hace fallar a Target: o da una ruta en UTF-8 válido, o "".
func FuzzTarget(f *testing.F) {
	f.Add(lnktest.File(`C:\Program Files\Mí App\`, "app.exe", false, ""))
	f.Add(lnktest.File(`C:\Apps\日本\`, "app.exe", true, ""))
	f.Add(lnktest.File("", "", false, `%ProgramFiles%\Tool\tool.exe`))
	f.Add([]byte("hola"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if got := Target(data); !utf8.ValidString(got) {
			t.Errorf("Target devolvió UTF-8 no válido: %q", got)
		}
	})
}
