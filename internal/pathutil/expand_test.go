package pathutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExpandEnv(t *testing.T) {
	t.Setenv("FFC_BASE", filepath.FromSlash("/datos/base"))
	t.Setenv("FFC_SUB", "sub")

	tests := []struct{ in, want string }{
		{`%FFC_BASE%`, filepath.FromSlash("/datos/base")},
		{`%FFC_BASE%\%FFC_SUB%\x`, filepath.FromSlash("/datos/base") + `\sub\x`},
		{`%FFC_NO_EXISTE%\x`, `%FFC_NO_EXISTE%\x`},
		{`100% seguro`, `100% seguro`},
		{`sin variables`, `sin variables`},
	}
	for _, tt := range tests {
		if got := ExpandEnv(tt.in); got != tt.want {
			t.Errorf("ExpandEnv(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExpandEnvIsCaseInsensitiveOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("solo en Windows las variables no distinguen mayúsculas")
	}
	t.Setenv("FFC_MIXTA", "valor")
	if got := ExpandEnv("%ffc_Mixta%"); got != "valor" {
		t.Errorf("got %q, want %q", got, "valor")
	}
}

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FFC_DIR", dir)

	for _, in := range []string{"%FFC_DIR%", `"` + dir + `"`, "  " + dir + "  "} {
		got, err := Resolve(in)
		if err != nil {
			t.Errorf("Resolve(%q): %v", in, err)
			continue
		}
		if got != dir {
			t.Errorf("Resolve(%q) = %q, want %q", in, got, dir)
		}
	}
}

func TestResolveErrors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "archivo.txt")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct{ in, wantErr string }{
		{"", "vacía"},
		{filepath.Join(dir, "no-existe"), "no existe"},
		{file, "no es una carpeta"},
		{`%FFC_VARIABLE_INEXISTENTE%\x`, "variable de entorno no definida: %FFC_VARIABLE_INEXISTENTE%"},
	}
	for _, tt := range tests {
		_, err := Resolve(tt.in)
		if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("Resolve(%q) error = %v, want que contenga %q", tt.in, err, tt.wantErr)
		}
	}
}

func TestResolveHomeAndDefault(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("sin directorio personal")
	}
	for _, in := range []string{"~", "%USERPROFILE%"} {
		got, err := Resolve(in)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", in, err)
		}
		if !strings.EqualFold(got, filepath.Clean(home)) {
			t.Errorf("Resolve(%q) = %q, want %q", in, got, home)
		}
	}
}

func TestResolveDriveRoot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("las letras de unidad solo existen en Windows")
	}
	drive := filepath.VolumeName(os.Getenv("SystemRoot")) // normalmente "C:"
	got, err := Resolve(drive)
	if err != nil {
		t.Fatal(err)
	}
	if want := drive + `\`; got != want {
		t.Errorf("Resolve(%q) = %q, want %q", drive, got, want)
	}
}
