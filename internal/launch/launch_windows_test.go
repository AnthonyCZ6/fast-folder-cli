package launch

import (
	"os"
	"path/filepath"
	"testing"
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

// La ruta va entre comillas pegada a "/select,": con espacios o comas en la
// ruta, explorer.exe abriría otra carpeta.
func TestRevealCmdLine(t *testing.T) {
	got := revealCmdLine(`C:\Program Files\Mi App, Inc\app.exe`)
	want := `explorer.exe /select,"C:\Program Files\Mi App, Inc\app.exe"`
	if got != want {
		t.Errorf("revealCmdLine = %s, want %s", got, want)
	}
}
