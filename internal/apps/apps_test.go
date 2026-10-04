package apps

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestIconFile(t *testing.T) {
	tests := []struct{ in, want string }{
		{`"C:\App\app.exe",0`, `C:\App\app.exe`},
		{`C:\App\app.exe,0`, `C:\App\app.exe`},
		{`C:\App\app.exe,-101`, `C:\App\app.exe`},
		{`"C:\App\app.exe"`, `C:\App\app.exe`},
		{`C:\App\app.ico`, `C:\App\app.ico`},
		// Una coma que no va seguida del número del icono es parte de la ruta.
		{`C:\A,B\app.exe`, `C:\A,B\app.exe`},
		{"  ", ""},
	}
	for _, tt := range tests {
		if got := iconFile(tt.in); got != tt.want {
			t.Errorf("iconFile(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCommandExe(t *testing.T) {
	tests := []struct{ in, want string }{
		{`"C:\App\unins000.exe" /SILENT`, `C:\App\unins000.exe`},
		{`C:\Mi App\uninstall.exe /S`, `C:\Mi App\uninstall.exe`},
		{`MsiExec.exe /X{1234}`, `MsiExec.exe`},
		{`rundll32 algo.dll,Desinstalar`, ""},
		{`"sin cerrar.exe`, ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := commandExe(tt.in); got != tt.want {
			t.Errorf("commandExe(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCleanPath(t *testing.T) {
	tests := []struct{ in, want string }{
		{`"C:\App\"`, `C:\App`},
		{`  C:\App  `, `C:\App`},
		{`C:\`, `C:\`},
		{"", ""},
	}
	for _, tt := range tests {
		if got := cleanPath(tt.in); got != tt.want {
			t.Errorf("cleanPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRejected(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{`C:\`, true},
		{`D:`, true},
		{`c:\windows`, true},
		{`C:\Windows\Installer\{1234}\icono.exe`, true},
		{`C:\ProgramData\Package Cache\{1234}\setup.exe`, true},
		{`C:\Program Files\App`, false},
		{`D:\Juegos\Windows Tools`, false},
		{`C:\WindowsApps2\app`, false},
	}
	for _, tt := range tests {
		if got := rejected(tt.path); got != tt.want {
			t.Errorf("rejected(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestWithin(t *testing.T) {
	tests := []struct {
		path, dir string
		want      bool
	}{
		{`C:\App\bin\app.exe`, `c:\app`, true},
		{`C:\App\app.exe`, `C:\App\`, true},
		{`C:\Apple\app.exe`, `C:\App`, false},
		{`C:\App`, `C:\App`, false},
	}
	for _, tt := range tests {
		if got := within(tt.path, tt.dir); got != tt.want {
			t.Errorf("within(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.want)
		}
	}
}

// makeFiles crea archivos vacíos (rutas relativas con "/") bajo un directorio
// temporal y devuelve el directorio.
func makeFiles(t *testing.T, files ...string) string {
	t.Helper()
	base := t.TempDir()
	for _, f := range files {
		path := filepath.Join(base, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return base
}

func TestBuild(t *testing.T) {
	base := makeFiles(t,
		"Chrome/Application/chrome.exe",
		"Juego/unins000.exe",
		"Office/root/EXCEL.EXE",
		"Herramienta/tool.cmd",
	)
	at := func(rel string) string { return filepath.Join(base, filepath.FromSlash(rel)) }

	chrome := entry{
		name:            "Google Chrome",
		installLocation: at("Chrome") + `\`,
		displayIcon:     `"` + at("Chrome/Application/chrome.exe") + `",0`,
		publisher:       "Google LLC",
		version:         "120.0",
	}
	entries := []entry{
		chrome,
		chrome, // la misma app registrada dos veces (HKLM y HKCU)
		// Sin InstallLocation y con el desinstalador como icono: se usa su
		// carpeta, pero no se selecciona el desinstalador.
		{name: "Juego", displayIcon: at("Juego/unins000.exe"), uninstall: `"` + at("Juego/unins000.exe") + `" /SILENT`},
		// El icono es una copia del instalador en C:\Windows\Installer: se
		// usa la carpeta de instalación.
		{name: "Microsoft 365", installLocation: at("Office"), displayIcon: `C:\Windows\Installer\{1234}\icono.exe`},
		{name: "Componente", installLocation: at("Chrome"), hidden: true},
		{name: "   ", installLocation: at("Chrome")},
		{name: "Sin carpeta", installLocation: at("NoExiste")},
		{name: "Instalador MSI", uninstall: "MsiExec.exe /X{1234}"},
	}
	appPaths := []string{
		at("Chrome/Application/chrome.exe"), // ya es el ejecutable de Google Chrome
		`"` + at("Office/root/EXCEL.EXE") + `"`,
		at("NoExiste/otra.exe"),
		at("Herramienta/tool.cmd"), // no es un .exe
	}

	want := []App{
		{Name: "EXCEL", Dir: at("Office/root"), Exe: at("Office/root/EXCEL.EXE")},
		{Name: "Google Chrome", Dir: at("Chrome/Application"), Exe: at("Chrome/Application/chrome.exe"), Publisher: "Google LLC", Version: "120.0"},
		{Name: "Juego", Dir: at("Juego")},
		{Name: "Microsoft 365", Dir: at("Office")},
	}
	if got := build(entries, appPaths, nil); !slices.Equal(got, want) {
		t.Errorf("build =\n%+v\nwant\n%+v", got, want)
	}

	chromeOnly := func(name string) bool { return strings.Contains(strings.ToLower(name), "chrome") }
	if got := build(entries, appPaths, chromeOnly); len(got) != 1 || got[0].Name != "Google Chrome" {
		t.Errorf("build con filtro = %+v, want solo Google Chrome", got)
	}
}

// Si el icono es un .exe fuera de la carpeta de instalación (otro programa),
// se abre la carpeta de instalación sin seleccionar nada.
func TestBuildIgnoresIconOutsideInstallLocation(t *testing.T) {
	base := makeFiles(t, "App/datos.txt", "Comun/lanzador.exe")
	entries := []entry{{
		name:            "App",
		installLocation: filepath.Join(base, "App"),
		displayIcon:     filepath.Join(base, "Comun", "lanzador.exe"),
	}}
	want := []App{{Name: "App", Dir: filepath.Join(base, "App")}}
	if got := build(entries, nil, nil); !slices.Equal(got, want) {
		t.Errorf("build = %+v, want %+v", got, want)
	}
}
