package apps

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/lnk/lnktest"
)

// readShortcuts recorre las subcarpetas, ignora lo que no es .lnk y expande
// las variables de entorno del destino.
func TestReadShortcuts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FFC_LNK_DIR", `C:\Herramientas`)
	files := map[string][]byte{
		"Editor.lnk":         lnktest.File(`C:\Editor\`, "editor.exe", false, ""),
		"Carpeta/Tool.lnk":   lnktest.File("", "", false, `%FFC_LNK_DIR%\tool.exe`),
		"Carpeta/leeme.txt":  []byte("no es un acceso directo"),
		"Carpeta/Office.lnk": lnktest.File("", "", false, ""),
		"Carpeta/dañado.lnk": []byte("dañado"),
		"Carpeta/Otra/X.LNK": lnktest.File(`D:\x\`, "x.exe", true, ""),
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
