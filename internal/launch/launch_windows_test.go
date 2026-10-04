package launch

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/sys/windows"
)

func TestFindVSCode(t *testing.T) {
	// Rutas relativas a la carpeta de cada prueba.
	const (
		codeCmd  = "vscode/bin/code.cmd"
		fromPath = "vscode/Code.exe"
		user     = "local/Programs/Microsoft VS Code/Code.exe"
		system   = "pf/Microsoft VS Code/Code.exe"
	)
	tests := []struct {
		name  string
		files []string
		want  string // vacío: no se encuentra
	}{
		{"junto a code del PATH", []string{codeCmd, fromPath}, fromPath},
		{"instalación de usuario", []string{user}, user},
		{"instalación para todos", []string{system}, system},
		{"el PATH tiene prioridad", []string{codeCmd, fromPath, user}, fromPath},
		{"code.cmd sin Code.exe al lado", []string{codeCmd, system}, system},
		{"no instalado", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			// findVSCode solo debe ver las carpetas de la prueba.
			t.Setenv("PATH", filepath.Join(base, "vscode", "bin"))
			t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
			t.Setenv("LOCALAPPDATA", filepath.Join(base, "local"))
			t.Setenv("ProgramFiles", filepath.Join(base, "pf"))
			for _, f := range tt.files {
				path := filepath.Join(base, filepath.FromSlash(f))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got, err := findVSCode()
			if tt.want == "" {
				if err == nil || err.Error() != "no se encontró Visual Studio Code" {
					t.Errorf("findVSCode() = %q, %v; se esperaba que no lo encontrara", got, err)
				}
				return
			}
			want := filepath.Join(base, filepath.FromSlash(tt.want))
			if err != nil || filepath.Clean(got) != want {
				t.Errorf("findVSCode() = %q, %v; want %q", got, err, want)
			}
		})
	}
}

// Estas pruebas comprueban las órdenes que se lanzarían, sin abrir ventanas.

func TestExplorerCmd(t *testing.T) {
	dir := `C:\Mis cosas\A&B`
	if got := explorerCmd(dir).Args; !slices.Equal(got, []string{"explorer", dir}) {
		t.Errorf("argumentos = %q", got)
	}
}

// La ruta va entre comillas pegada a "/select,": con espacios o comas en la
// ruta, explorer.exe abriría otra carpeta.
func TestRevealCmd(t *testing.T) {
	cmd := revealCmd(`C:\Program Files\Mi App, Inc\app.exe`)
	want := `explorer.exe /select,"C:\Program Files\Mi App, Inc\app.exe"`
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CmdLine != want {
		t.Errorf("línea de órdenes = %+v, want %s", cmd.SysProcAttr, want)
	}
}

// Con ejecutable se abre su carpeta con él seleccionado; sin él, la carpeta.
func TestShowAppCmd(t *testing.T) {
	withExe := showAppCmd(`C:\App`, `C:\App\bin\app.exe`)
	if withExe.SysProcAttr == nil || withExe.SysProcAttr.CmdLine != `explorer.exe /select,"C:\App\bin\app.exe"` {
		t.Errorf("con ejecutable: %+v", withExe.SysProcAttr)
	}
	if got := showAppCmd(`C:\App`, "").Args; !slices.Equal(got, []string{"explorer", `C:\App`}) {
		t.Errorf("sin ejecutable: argumentos = %q", got)
	}
}

// La terminal se abre en una consola propia y en la carpeta elegida.
func TestTerminalCmd(t *testing.T) {
	dir := `C:\Mis cosas\A&B`
	cmd := terminalCmd("powershell.exe", dir)
	if !slices.Equal(cmd.Args, []string{"powershell.exe", "-NoLogo"}) || cmd.Dir != dir {
		t.Errorf("argumentos = %q, carpeta = %q", cmd.Args, cmd.Dir)
	}
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_NEW_CONSOLE == 0 {
		t.Errorf("debería abrirse en una consola nueva: %+v", cmd.SysProcAttr)
	}
}

// Cada terminal se abre en la carpeta como carpeta de trabajo, sin pasar su
// ruta como argumento.
func TestTerminalKindCmd(t *testing.T) {
	dir := `C:\a;b & c`
	tests := []struct {
		kind    string
		args    []string
		console bool
	}{
		{"wt", []string{"wt.exe", "-d", "."}, false},
		{"cmd", []string{"cmd.exe"}, true},
		{"pwsh", []string{"pwsh.exe", "-NoLogo"}, true},
		{"powershell", []string{"powershell.exe", "-NoLogo"}, true},
	}
	for _, tt := range tests {
		cmd := terminalKindCmd(tt.kind, dir)
		if !slices.Equal(cmd.Args, tt.args) || cmd.Dir != dir {
			t.Errorf("%s: argumentos = %q, carpeta = %q", tt.kind, cmd.Args, cmd.Dir)
		}
		console := cmd.SysProcAttr != nil && cmd.SysProcAttr.CreationFlags&windows.CREATE_NEW_CONSOLE != 0
		if console != tt.console {
			t.Errorf("%s: consola nueva = %v, want %v", tt.kind, console, tt.console)
		}
	}
}

// Un editor que no es VS Code recibe "." (o el nombre del archivo) y la
// carpeta como carpeta de trabajo; admite %VARIABLES% en su ruta.
func TestEditorCmd(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "A&B ^ 100%")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FFC_EDITOR_DIR", `C:\Editores`)
	// Una ruta relativa se resuelve contra la carpeta actual, nunca contra la
	// carpeta encontrada (que podría contener un ejecutable ajeno).
	relative, err := filepath.Abs(`tools\ed.exe`)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		command, target string
		args            []string
		dir             string
	}{
		{"cursor", dir, []string{"cursor", "."}, dir},
		{"notepad++", file, []string{"notepad++", "config.toml"}, dir},
		{`%FFC_EDITOR_DIR%\ed.exe`, dir, []string{`C:\Editores\ed.exe`, "."}, dir},
		{`tools\ed.exe`, dir, []string{relative, "."}, dir},
	}
	for _, tt := range tests {
		cmd, err := editorCmd(tt.command, tt.target)
		if err != nil {
			t.Fatalf("%s: %v", tt.command, err)
		}
		if !slices.Equal(cmd.Args, tt.args) || cmd.Dir != tt.dir {
			t.Errorf("%s: argumentos = %q, carpeta = %q; want %q, %q", tt.command, cmd.Args, cmd.Dir, tt.args, tt.dir)
		}
	}
}
