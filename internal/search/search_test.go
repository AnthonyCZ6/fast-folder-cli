package search

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
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

// collect ejecuta una búsqueda en root y devuelve los resultados relativos a
// root, ordenados, con "/" como separador y el tipo de proyecto (si lo hay)
// entre paréntesis.
func collect(t *testing.T, root string, opts Options) []string {
	t.Helper()
	opts.Root = root
	results, _ := Start(context.Background(), opts)
	var got []string
	for r := range results {
		rel, err := filepath.Rel(root, r.Path)
		if err != nil {
			t.Fatal(err)
		}
		s := filepath.ToSlash(rel)
		if r.Project != "" {
			s += " (" + r.Project + ")"
		}
		got = append(got, s)
	}
	slices.Sort(got)
	return got
}

// run busca term en root y devuelve las coincidencias como collect.
func run(t *testing.T, root, term string, hidden bool, workers int) []string {
	t.Helper()
	return collect(t, root, Options{Matcher: mustMatcher(t, term), IncludeHidden: hidden, Workers: workers})
}

func mustMatcher(t *testing.T, term string) *Matcher {
	t.Helper()
	m, err := NewMatcher(term)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// writeFiles crea archivos vacíos (rutas relativas con "/") bajo root.
func writeFiles(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(f)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
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
	for r := range results {
		t.Errorf("resultado inesperado tras cancelar: %s", r.Path)
	}
}

func TestSearchIgnoresAccents(t *testing.T) {
	root := makeTree(t, "Música/Canción favorita", "Fotos/Año 2024", "Cafe\u0301")

	if got, want := run(t, root, "cancion", false, 0), []string{"Música/Canción favorita"}; !slices.Equal(got, want) {
		t.Errorf("cancion: got %v, want %v", got, want)
	}
	if got, want := run(t, root, "ano 2024", false, 0), []string{"Fotos/Año 2024"}; !slices.Equal(got, want) {
		t.Errorf("ano 2024: got %v, want %v", got, want)
	}
	// Nombre guardado en forma descompuesta (e + acento combinado).
	if got := run(t, root, "café", false, 0); len(got) != 1 {
		t.Errorf("café: got %v, want 1 resultado", got)
	}
}

func TestSearchProjects(t *testing.T) {
	root := makeTree(t,
		"escuela/web/src",
		"escuela/web/node_modules/lib",
		"escuela/api",
		"escuela/notas",
		"juegos/plataformas/Assets",
		"juegos/plataformas/ProjectSettings",
		"juegos/consola",
		"dotfiles/.git",
	)
	writeFiles(t, root,
		"package.json", // en la raíz: no debe ocultar los proyectos de dentro
		"escuela/web/package.json",
		"escuela/web/node_modules/lib/package.json",
		"escuela/api/go.mod",
		"escuela/api/requirements.txt",
		"escuela/notas/apuntes.txt",
		"juegos/consola/Consola.sln",
	)

	want := []string{
		"dotfiles (Git)",
		"escuela/api (Go, Python)",
		"escuela/web (Node.js)",
		"juegos/consola (.NET)",
		"juegos/plataformas (Unity)",
	}
	if got := collect(t, root, Options{Projects: true}); !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Con término, solo los proyectos cuyo nombre coincide.
	got := collect(t, root, Options{Projects: true, Matcher: mustMatcher(t, "web")})
	if want := []string{"escuela/web (Node.js)"}; !slices.Equal(got, want) {
		t.Errorf("con término: got %v, want %v", got, want)
	}
}

func setModTime(t *testing.T, path string, mod time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestSearchModifiedRange(t *testing.T) {
	root := makeTree(t, "viejo/informe", "ayer/informe", "nuevo/informe")
	now := time.Now()
	setModTime(t, filepath.Join(root, "viejo", "informe"), now.AddDate(0, 0, -30))
	setModTime(t, filepath.Join(root, "ayer", "informe"), now.AddDate(0, 0, -1))
	m := mustMatcher(t, "informe")

	got := collect(t, root, Options{Matcher: m, ModifiedAfter: now.AddDate(0, 0, -7)})
	if want := []string{"ayer/informe", "nuevo/informe"}; !slices.Equal(got, want) {
		t.Errorf("últimos 7 días: got %v, want %v", got, want)
	}

	got = collect(t, root, Options{Matcher: m, ModifiedAfter: now.AddDate(0, 0, -2), ModifiedBefore: now.Add(-time.Hour)})
	if want := []string{"ayer/informe"}; !slices.Equal(got, want) {
		t.Errorf("intervalo: got %v, want %v", got, want)
	}

	// Sin término se filtra solo por fecha (las carpetas padre son nuevas).
	got = collect(t, root, Options{ModifiedBefore: now.AddDate(0, 0, -7)})
	if want := []string{"viejo/informe"}; !slices.Equal(got, want) {
		t.Errorf("solo fecha: got %v, want %v", got, want)
	}
}

func TestSearchPrune(t *testing.T) {
	root := makeTree(t, "app/node_modules/lib/node_modules/dep", "web/node_modules")
	m := mustMatcher(t, "node_modules")

	got := collect(t, root, Options{Matcher: m, Prune: true})
	if want := []string{"app/node_modules", "web/node_modules"}; !slices.Equal(got, want) {
		t.Errorf("con Prune: got %v, want %v", got, want)
	}
	got = collect(t, root, Options{Matcher: m})
	if want := []string{"app/node_modules", "app/node_modules/lib/node_modules", "web/node_modules"}; !slices.Equal(got, want) {
		t.Errorf("sin Prune: got %v, want %v", got, want)
	}
}

func TestSize(t *testing.T) {
	root := makeTree(t, "a/b", "c", ".oculta")
	for path, size := range map[string]int{"a/uno.txt": 10, "a/b/dos.txt": 20, "c/tres.txt": 5, ".oculta/cuatro.txt": 7} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	info := Size(context.Background(), root)
	if info.Bytes != 42 || info.Files != 4 || info.Denied != 0 {
		t.Errorf("Size = %+v, want 42 bytes en 4 archivos", info)
	}
}
