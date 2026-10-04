// Package e2e prueba fast-folder-cli de punta a punta: compila el programa y
// lo ejecuta como un proceso aparte, igual que desde una terminal, sobre
// carpetas temporales. Con -short no se compila y las pruebas se saltan.
package e2e

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// exe es la ruta del programa compilado; vacía con -short.
var exe string

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("", "fast-folder-cli-e2e")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// El programa no debe leer el archivo de configuración de quien ejecuta
	// las pruebas: FFC_CONFIG apunta a uno que no existe.
	os.Setenv("FFC_CONFIG", filepath.Join(dir, "sin-config.toml"))
	code := buildAndRun(m, dir)
	os.RemoveAll(dir)
	os.Exit(code)
}

// buildAndRun compila el programa en dir y ejecuta las pruebas.
func buildAndRun(m *testing.M, dir string) int {
	name := "fast-folder-cli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	exe = filepath.Join(dir, name)
	build := exec.Command("go", "build", "-o", exe, "-ldflags", "-X main.version=e2e", "github.com/AnthonyCZ6/fast-folder-cli")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "no se pudo compilar fast-folder-cli: %v\n%s", err, out)
		return 1
	}
	return m.Run()
}

// result es lo que deja una ejecución del programa.
type result struct {
	stdout, stderr string
	code           int
}

// run ejecuta el programa con args y devuelve su salida y su código.
func run(t *testing.T, args ...string) result {
	t.Helper()
	if exe == "" {
		t.Skip("con -short no se compila el programa")
	}
	cmd := exec.Command(exe, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := result{stdout: stdout.String(), stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		r.code = exitErr.ExitCode()
	case err != nil:
		t.Fatalf("no se pudo ejecutar %v: %v", args, err)
	}
	return r
}

// mustRun ejecuta el programa y comprueba que termina con el código want.
func mustRun(t *testing.T, want int, args ...string) result {
	t.Helper()
	r := run(t, args...)
	if r.code != want {
		t.Fatalf("%v terminó con %d, want %d\nstdout:\n%s\nstderr:\n%s", args, r.code, want, r.stdout, r.stderr)
	}
	return r
}

// makeTree crea las carpetas indicadas (rutas relativas con "/") y los
// archivos de files (con el tamaño indicado) bajo un directorio temporal.
func makeTree(t *testing.T, dirs []string, files map[string]int) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for f, size := range files {
		path := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func assertContains(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("la salida no contiene %q:\n%s", want, out)
		}
	}
}

func TestVersion(t *testing.T) {
	r := mustRun(t, 0, "--version")
	if r.stdout != "fast-folder-cli e2e\n" {
		t.Errorf("--version = %q", r.stdout)
	}
}

func TestExitCodes(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		args []string
		want int
	}{
		{[]string{"--help"}, 0},
		{[]string{"-n", "nada-coincide", "-p", root}, 1},
		{[]string{}, 2}, // sin terminal no se abre el modo interactivo
		{[]string{"-x"}, 2},
		{[]string{"-n", "x", "-p", filepath.Join(root, "no-existe")}, 2},
		{[]string{"-m", "mañana", "-p", root}, 2},
		{[]string{"--apps", "-p", root}, 2},
	}
	for _, tt := range tests {
		if r := run(t, tt.args...); r.code != tt.want {
			t.Errorf("%q terminó con %d, want %d\nstderr: %s", tt.args, r.code, tt.want, r.stderr)
		}
	}
}

// La salida redirigida no lleva colores, y los nombres con acentos llegan en
// UTF-8 aunque se busquen sin acentos.
func TestSearchIgnoresAccents(t *testing.T) {
	root := makeTree(t, []string{"Música/Canción", "Música/otra", "docs"}, nil)
	r := mustRun(t, 0, "cancion", "-p", root)
	assertContains(t, r.stdout,
		filepath.Join(root, "Música", "Canción"),
		"Resultados : 1 carpeta encontrada",
	)
	if strings.Contains(r.stdout, "\x1b[") {
		t.Errorf("la salida redirigida no debe llevar colores:\n%s", r.stdout)
	}
}

func TestProjects(t *testing.T) {
	root := makeTree(t, []string{"notas"}, map[string]int{
		"web/package.json": 0,
		"api/go.mod":       0,
	})
	r := mustRun(t, 0, "--projects", "-p", root)
	assertContains(t, r.stdout,
		filepath.Join(root, "web")+"  (Node.js)",
		filepath.Join(root, "api")+"  (Go)",
		"Resultados : 2 proyectos encontrados",
	)
}

func TestModifiedToday(t *testing.T) {
	root := makeTree(t, []string{"nueva", "vieja"}, nil)
	old := time.Now().AddDate(0, 0, -10)
	if err := os.Chtimes(filepath.Join(root, "vieja"), old, old); err != nil {
		t.Fatal(err)
	}
	r := mustRun(t, 0, "-m", "hoy", "-p", root)
	assertContains(t, r.stdout, filepath.Join(root, "nueva"), "Resultados : 1 carpeta encontrada")
	if strings.Contains(r.stdout, filepath.Join(root, "vieja")) {
		t.Errorf("la carpeta de hace 10 días no debería aparecer:\n%s", r.stdout)
	}
}

// --size ordena de mayor a menor y muestra el total.
func TestSizeSortsBySize(t *testing.T) {
	root := makeTree(t, nil, map[string]int{
		"chica/node_modules/a.js":  10,
		"grande/node_modules/b.js": 5000,
	})
	r := mustRun(t, 0, "node_modules", "--size", "-p", root)
	big := strings.Index(r.stdout, filepath.Join(root, "grande", "node_modules"))
	small := strings.Index(r.stdout, filepath.Join(root, "chica", "node_modules"))
	if big < 0 || small < 0 || big > small {
		t.Errorf("la carpeta grande debería ir antes que la chica:\n%s", r.stdout)
	}
	assertContains(t, r.stdout, "Tamaño     : ", "(2 archivos)")
}
