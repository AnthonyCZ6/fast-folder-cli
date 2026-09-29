package search

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// makeTree crea las carpetas indicadas (rutas relativas con "/") bajo un
// directorio temporal y devuelve su ruta.
func makeTree(t *testing.T, dirs ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// run ejecuta una búsqueda y devuelve las coincidencias relativas a root,
// ordenadas y con "/" como separador.
func run(t *testing.T, root, term string, hidden bool, workers int) []string {
	t.Helper()
	m, err := NewMatcher(term)
	if err != nil {
		t.Fatal(err)
	}
	results, _ := Start(context.Background(), Options{
		Root: root, Matcher: m, IncludeHidden: hidden, Workers: workers,
	})
	var got []string
	for p := range results {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, filepath.ToSlash(rel))
	}
	slices.Sort(got)
	return got
}

func TestSearchFindsNestedFolders(t *testing.T) {
	root := makeTree(t,
		"alpha/Proyecto-Final/src",
		"alpha/docs/mi-proyecto",
		"beta/other/deep/deeper/PROYECTO",
		"gamma",
	)
	// Un archivo con nombre coincidente no debe aparecer.
	if err := os.WriteFile(filepath.Join(root, "gamma", "proyecto.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"alpha/Proyecto-Final",
		"alpha/docs/mi-proyecto",
		"beta/other/deep/deeper/PROYECTO",
	}
	// Con un solo worker se fuerza el procesamiento en línea; con muchos, el
	// reparto entre goroutines. El resultado debe ser idéntico.
	for _, workers := range []int{1, 2, 64} {
		if got := run(t, root, "proyecto", false, workers); !slices.Equal(got, want) {
			t.Errorf("workers=%d: got %v, want %v", workers, got, want)
		}
	}
}

func TestSearchGlob(t *testing.T) {
	root := makeTree(t, "proyectos", "mi-proyecto", "proyecto-2024/proyecto-x")

	got := run(t, root, "proyecto*", false, 0)
	want := []string{"proyecto-2024", "proyecto-2024/proyecto-x", "proyectos"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSearchSkipsDotFoldersUnlessAll(t *testing.T) {
	root := makeTree(t, ".cache/config-app", "visible/config")

	if got, want := run(t, root, "config", false, 0), []string{"visible/config"}; !slices.Equal(got, want) {
		t.Errorf("sin --all: got %v, want %v", got, want)
	}
	want := []string{".cache/config-app", "visible/config"}
	if got := run(t, root, "config", true, 0); !slices.Equal(got, want) {
		t.Errorf("con --all: got %v, want %v", got, want)
	}
}

func TestSearchStats(t *testing.T) {
	root := makeTree(t, "a/b/c", "d")
	m, _ := NewMatcher("zzz")
	results, stats := Start(context.Background(), Options{Root: root, Matcher: m})
	for range results {
		t.Error("no se esperaban resultados")
	}
	// root, a, a/b, a/b/c y d.
	if got := stats.Scanned(); got != 5 {
		t.Errorf("Scanned() = %d, want 5", got)
	}
	if got := stats.Denied(); got != 0 {
		t.Errorf("Denied() = %d, want 0", got)
	}
}

func TestSearchMissingRootCountsAsDenied(t *testing.T) {
	m, _ := NewMatcher("x")
	results, stats := Start(context.Background(), Options{
		Root: filepath.Join(t.TempDir(), "no-existe"), Matcher: m,
	})
	for range results {
	}
	if stats.Denied() != 1 || stats.Scanned() != 0 {
		t.Errorf("Denied=%d Scanned=%d, want 1 y 0", stats.Denied(), stats.Scanned())
	}
}

func TestSearchCancel(t *testing.T) {
	root := makeTree(t, "x/x/x", "x/y/x")
	m, _ := NewMatcher("x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results, _ := Start(ctx, Options{Root: root, Matcher: m})
	for p := range results {
		t.Errorf("resultado inesperado tras cancelar: %s", p)
	}
}
