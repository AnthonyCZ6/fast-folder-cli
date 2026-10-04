package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeConfig escribe content en un config.toml temporal y devuelve su ruta.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFileMissingIsEmpty(t *testing.T) {
	c, err := LoadFile(filepath.Join(t.TempDir(), "no-existe.toml"))
	if err != nil || !reflect.DeepEqual(c, Config{}) {
		t.Errorf("LoadFile = %+v, %v; want una Config vacía sin error", c, err)
	}
}

func TestLoadFile(t *testing.T) {
	path := writeConfig(t, `
ubicacion = '%USERPROFILE%\Documents'
excluir = ["node_modules", "venv"]
editor = "cursor"
terminal = "WT"
ocultas = true
recientes = 3

[[ubicaciones]]
nombre = "Proyectos"
ruta = 'D:\dev'
`)
	three := 3
	want := Config{
		Root:      `%USERPROFILE%\Documents`,
		Exclude:   []string{"node_modules", "venv"},
		Editor:    "cursor",
		Terminal:  "wt",
		Hidden:    true,
		Recent:    &three,
		Locations: []Location{{Name: "Proyectos", Path: `D:\dev`}},
	}
	c, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("LoadFile =\n%+v\nwant\n%+v", c, want)
	}
}

func TestLoadFileErrors(t *testing.T) {
	tests := []struct {
		name, content, want string
	}{
		{"sintaxis", "ubicacion = 'sin cerrar\n", "line 1"},
		{"clave desconocida", "ubicaciom = 'C:\\'\n", `clave desconocida "ubicaciom"`},
		{"tipo", "excluir = \"node_modules\"\n", "excluir"},
		{"terminal", "terminal = \"konsole\"\n", `terminal "konsole" no válida`},
		{"ubicación sin ruta", "[[ubicaciones]]\nnombre = \"X\"\n", "la ubicación 1 necesita nombre y ruta"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.content)
			c, err := LoadFile(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), path) {
				t.Errorf("error = %v; want uno con la ruta y %q", err, tt.want)
			}
			if !reflect.DeepEqual(c, Config{}) {
				t.Errorf("con error debería devolver una Config vacía: %+v", c)
			}
		})
	}
}

// La plantilla tiene todas las claves comentadas: equivale a no tener archivo.
func TestTemplateIsEmpty(t *testing.T) {
	c, err := LoadFile(writeConfig(t, Template))
	if err != nil || !reflect.DeepEqual(c, Config{}) {
		t.Errorf("la plantilla da %+v, %v; want una Config vacía", c, err)
	}
	for _, key := range []string{"ubicacion", "excluir", "ocultas", "editor", "terminal", "recientes", "[[ubicaciones]]"} {
		if !strings.Contains(Template, key) {
			t.Errorf("la plantilla no documenta %s", key)
		}
	}
}

// Create crea la carpeta y la plantilla, y nunca sobrescribe un archivo.
func TestCreate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fast-folder-cli", "config.toml")
	created, err := Create(path)
	if err != nil || !created {
		t.Fatalf("Create = %v, %v; want true", created, err)
	}
	if err := os.WriteFile(path, []byte("ocultas = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err = Create(path)
	if err != nil || created {
		t.Errorf("Create sobre un archivo existente = %v, %v; want false", created, err)
	}
	if data, _ := os.ReadFile(path); string(data) != "ocultas = true\n" {
		t.Errorf("Create no debería sobrescribir: %q", data)
	}
}

func TestPath(t *testing.T) {
	t.Setenv(EnvVar, `C:\otra\config.toml`)
	if got, err := Path(); err != nil || got != `C:\otra\config.toml` {
		t.Errorf("con %s: Path = %q, %v", EnvVar, got, err)
	}
	t.Setenv(EnvVar, "")
	got, err := Path()
	if err != nil || !strings.HasSuffix(got, filepath.Join("fast-folder-cli", "config.toml")) {
		t.Errorf("Path = %q, %v; want …\\fast-folder-cli\\config.toml", got, err)
	}
}

func TestRecentLimit(t *testing.T) {
	n := func(v int) *int { return &v }
	tests := []struct {
		recent *int
		want   int
	}{
		{nil, DefaultRecent},
		{n(0), 0},
		{n(-2), 0},
		{n(8), 8},
	}
	for _, tt := range tests {
		if got := (Config{Recent: tt.recent}).RecentLimit(); got != tt.want {
			t.Errorf("RecentLimit(%v) = %d, want %d", tt.recent, got, tt.want)
		}
	}
}
