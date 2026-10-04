package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// isolate hace que la configuración y las recientes vivan en una carpeta
// temporal, y la devuelve.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(EnvVar, filepath.Join(dir, "config.toml"))
	return dir
}

func mkdirs(t *testing.T, base string, names ...string) []string {
	t.Helper()
	var dirs []string
	for _, n := range names {
		d := filepath.Join(base, n)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, d)
	}
	return dirs
}

// AddRecent pone la carpeta la primera y sin repetir; LoadRecent devuelve las
// que aún existen, hasta el límite.
func TestRecent(t *testing.T) {
	base := isolate(t)
	if got := LoadRecent(5); got != nil {
		t.Errorf("sin archivo: %q, want nil", got)
	}
	d := mkdirs(t, base, "uno", "dos", "tres", "borrada")
	for _, dir := range []string{d[0], d[1], d[3], d[2], d[0]} {
		if err := AddRecent(dir); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(d[3]); err != nil {
		t.Fatal(err)
	}

	if got, want := LoadRecent(5), []string{d[0], d[2], d[1]}; !slices.Equal(got, want) {
		t.Errorf("LoadRecent(5) = %q, want %q", got, want)
	}
	if got := LoadRecent(2); len(got) != 2 {
		t.Errorf("LoadRecent(2) = %q, want 2", got)
	}
	if got := LoadRecent(0); got != nil {
		t.Errorf("LoadRecent(0) = %q, want nil", got)
	}
}

// Se guardan como mucho maxRecent carpetas, las más nuevas.
func TestRecentKeepsTheNewest(t *testing.T) {
	isolate(t)
	for i := range maxRecent + 5 {
		if err := AddRecent(filepath.Join("carpeta", string(rune('a'+i)))); err != nil {
			t.Fatal(err)
		}
	}
	got := readRecent()
	if len(got) != maxRecent || !strings.HasSuffix(got[0], string(rune('a'+maxRecent+4))) {
		t.Errorf("se guardaron %d (la primera %q), want %d con la más nueva primero", len(got), got, maxRecent)
	}
}

// Un archivo dañado no rompe nada: se ignora al leer y se reemplaza al guardar.
func TestRecentDamagedFile(t *testing.T) {
	base := isolate(t)
	path, err := RecentPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{no es json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LoadRecent(5); got != nil {
		t.Errorf("con el archivo dañado: %q, want nil", got)
	}
	d := mkdirs(t, base, "nueva")
	if err := AddRecent(d[0]); err != nil {
		t.Fatal(err)
	}
	if got := LoadRecent(5); !slices.Equal(got, d) {
		t.Errorf("después de guardar: %q, want %q", got, d)
	}
}
