package apps

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

// lnkFile arma un acceso directo mínimo (formato MS-SHLLINK). Con base, lleva
// un LinkInfo que apunta a base+suffix (en ANSI y, si unicode, también en
// UTF-16). Con env, un bloque de variables de entorno que apunta a env.
func lnkFile(base, suffix string, unicode bool, env string) []byte {
	var flags uint32 = lnkIsUnicode
	if base != "" {
		flags |= lnkHasLinkInfo
	}
	b := make([]byte, lnkHeaderSize)
	binary.LittleEndian.PutUint32(b[0:], lnkHeaderSize)
	binary.LittleEndian.PutUint32(b[20:], flags)
	if base != "" {
		b = append(b, linkInfo(base, suffix, unicode)...)
	}
	if env != "" {
		block := make([]byte, lnkEnvBlockSize)
		binary.LittleEndian.PutUint32(block[0:], lnkEnvBlockSize)
		binary.LittleEndian.PutUint32(block[4:], lnkEnvBlock)
		copy(block[8:], latin1z(env))
		copy(block[8+260:], utf16zBytes(env))
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
		parts = append(parts, utf16zBytes(base), utf16zBytes(suffix))
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
	put(8, lnkLocalBasePath)
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

func utf16zBytes(s string) []byte {
	var b []byte
	for _, u := range utf16.Encode([]rune(s)) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return append(b, 0, 0)
}

func TestLnkTarget(t *testing.T) {
	full := lnkFile(`C:\Program Files\Mí App\`, "app.exe", false, "")
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"ANSI con acentos", full, `C:\Program Files\Mí App\app.exe`},
		{"Unicode", lnkFile(`C:\Apps\日本\`, "app.exe", true, ""), `C:\Apps\日本\app.exe`},
		{"solo variables de entorno", lnkFile("", "", false, `%ProgramFiles%\Tool\tool.exe`), `%ProgramFiles%\Tool\tool.exe`},
		{"anunciado (sin destino)", lnkFile("", "", false, ""), ""},
		{"recortado", full[:lnkHeaderSize+10], ""},
		{"no es un .lnk", []byte("hola"), ""},
		{"solo la cabecera, con LinkInfo", lnkFile(`C:\a\`, "b.exe", false, "")[:lnkHeaderSize], ""},
		{"bloque de tamaño 7", binary.LittleEndian.AppendUint32(lnkFile("", "", false, "")[:lnkHeaderSize], 7), ""},
	}
	for _, tt := range tests {
		if got := lnkTarget(tt.data); got != tt.want {
			t.Errorf("%s: lnkTarget = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// readShortcuts recorre las subcarpetas, ignora lo que no es .lnk y expande
// las variables de entorno del destino.
func TestReadShortcuts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FFC_LNK_DIR", `C:\Herramientas`)
	files := map[string][]byte{
		"Editor.lnk":         lnkFile(`C:\Editor\`, "editor.exe", false, ""),
		"Carpeta/Tool.lnk":   lnkFile("", "", false, `%FFC_LNK_DIR%\tool.exe`),
		"Carpeta/leeme.txt":  []byte("no es un acceso directo"),
		"Carpeta/Office.lnk": lnkFile("", "", false, ""),
		"Carpeta/dañado.lnk": []byte("dañado"),
		"Carpeta/Otra/X.LNK": lnkFile(`D:\x\`, "x.exe", true, ""),
	}
	for name, data := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := readShortcuts(dir)
	slices.SortFunc(got, func(a, b shortcut) int { return strings.Compare(a.name, b.name) })
	want := []shortcut{
		{"Editor", `C:\Editor\editor.exe`},
		{"Tool", `C:\Herramientas\tool.exe`},
		{"X", `D:\x\x.exe`},
	}
	if !slices.Equal(got, want) {
		t.Errorf("readShortcuts =\n%+v\nwant\n%+v", got, want)
	}
}

// Los accesos directos añaden apps portables con su nombre; si apuntan al
// .exe de una app del registro no se repiten, y una app sin ejecutable adopta
// el único que haya en su carpeta.
func TestBuildWithShortcuts(t *testing.T) {
	base := makeFiles(t, "Portable/portable.exe", "Editor/editor.exe", "Chrome/chrome.exe", "Juego/unins000.exe")
	at := func(rel string) string { return filepath.Join(base, filepath.FromSlash(rel)) }
	entries := []entry{
		{name: "Google Chrome", displayIcon: at("Chrome/chrome.exe")},
		{name: "Editor", installLocation: at("Editor")},
	}
	links := []shortcut{
		{"Mi Portable", at("Portable/portable.exe")},
		{"Chrome", at("Chrome/chrome.exe")},       // ya es de Google Chrome
		{"Editor", at("Editor/editor.exe")},       // lo adopta la app Editor
		{"Desinstalar", at("Juego/unins000.exe")}, // un desinstalador no es una app
		{"Borrado", at("NoExiste/x.exe")},
	}
	want := []App{
		{Name: "Editor", Dir: at("Editor"), Exe: at("Editor/editor.exe")},
		{Name: "Google Chrome", Dir: at("Chrome"), Exe: at("Chrome/chrome.exe")},
		{Name: "Mi Portable", Dir: at("Portable"), Exe: at("Portable/portable.exe")},
	}
	if got := build(entries, nil, links, nil); !slices.Equal(got, want) {
		t.Errorf("build =\n%+v\nwant\n%+v", got, want)
	}
}

// Si App Paths nombra el ejecutable de una app, se adopta aunque un acceso
// directo apunte a otro de la misma carpeta (un actualizador, por ejemplo).
func TestBuildPrefersAppPaths(t *testing.T) {
	base := makeFiles(t, "7-Zip/7zFM.exe", "7-Zip/7zG.exe")
	at := func(rel string) string { return filepath.Join(base, filepath.FromSlash(rel)) }
	entries := []entry{{name: "7-Zip", installLocation: at("7-Zip")}}
	links := []shortcut{{"7-Zip GUI", at("7-Zip/7zG.exe")}}
	want := []App{
		{Name: "7-Zip", Dir: at("7-Zip"), Exe: at("7-Zip/7zFM.exe")},
		{Name: "7-Zip GUI", Dir: at("7-Zip"), Exe: at("7-Zip/7zG.exe")},
	}
	if got := build(entries, []string{at("7-Zip/7zFM.exe")}, links, nil); !slices.Equal(got, want) {
		t.Errorf("build =\n%+v\nwant\n%+v", got, want)
	}
}
