package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		args []string
		want config
	}{
		{[]string{"-n", "proyecto"}, config{term: "proyecto", root: defaultRoot}},
		{[]string{"--name=proyecto", "--path", "D:\\", "--all", "--open"}, config{term: "proyecto", root: "D:\\", all: true, open: true}},
		{[]string{"-a", "-o", "-p", "%APPDATA%", "-n", "x"}, config{term: "x", root: "%APPDATA%", all: true, open: true}},
		// Término posicional con opciones antes y después.
		{[]string{"-a", "mis", "proyectos", "-o"}, config{term: "mis proyectos", root: defaultRoot, all: true, open: true}},
		// Tras "--" todo es parte del término.
		{[]string{"--", "-raro"}, config{term: "-raro", root: defaultRoot}},
		{[]string{"-v"}, config{root: defaultRoot, version: true}},
		{[]string{"--projects", "api"}, config{term: "api", root: defaultRoot, projects: true}},
		{[]string{"-m", "hoy", "-s", "x"}, config{term: "x", root: defaultRoot, modified: "hoy", size: true}},
		{[]string{"--modified=semana", "--size", "--cd-file", "elegida.txt"}, config{root: defaultRoot, modified: "semana", size: true, cdFile: "elegida.txt"}},
	}
	for _, tt := range tests {
		got, err := parseArgs(tt.args)
		if err != nil {
			t.Errorf("parseArgs(%q): %v", tt.args, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseArgs(%q) = %+v, want %+v", tt.args, got, tt.want)
		}
	}
}

func TestParseArgsErrors(t *testing.T) {
	tests := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"-x"}, "opción desconocida: -x"},
		{[]string{"-n"}, "falta el valor de la opción -n"},
		{[]string{"-n", "uno", "dos"}, "argumentos inesperados: dos"},
	}
	for _, tt := range tests {
		_, err := parseArgs(tt.args)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("parseArgs(%q) error = %v, want %q", tt.args, err, tt.wantErr)
		}
	}
}

func TestRunEndToEnd(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"docs/Proyecto-A", "src/proyecto-b", "otros"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-n", "proyecto", "-p", root}, &stdout, &stderr, "test")
	if code != exitFound {
		t.Fatalf("código de salida = %d, want %d; stderr: %s", code, exitFound, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		filepath.Join(root, "docs", "Proyecto-A"),
		filepath.Join(root, "src", "proyecto-b"),
		"Resultados : 2 carpetas encontradas",
		"Analizadas : 6 carpetas", // raíz, docs, src, otros y las dos coincidencias
		"Tiempo     : ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("la salida no contiene %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("la salida redirigida no debe contener colores ANSI:\n%s", out)
	}
}

func TestRunExitCodes(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		args []string
		want int
	}{
		{[]string{"-n", "nada-coincide", "-p", root}, exitNoMatch},
		{[]string{"-n", "x", "-p", filepath.Join(root, "no-existe")}, exitUsage},
		{[]string{"-n", "[roto", "-p", root}, exitUsage},
		{[]string{}, exitUsage},
		{[]string{"-a"}, exitUsage}, // sin término y sin terminal no hay modo interactivo
		{[]string{"-m", "mañana", "-p", root}, exitUsage},
		{[]string{"--cd-file", "elegida.txt"}, exitUsage},
		{[]string{"--projects", "-p", root}, exitNoMatch},
		{[]string{"--help"}, exitFound},
		{[]string{"--version"}, exitFound},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		if got := Run(tt.args, &stdout, &stderr, "test"); got != tt.want {
			t.Errorf("Run(%q) = %d, want %d\nstdout: %s\nstderr: %s", tt.args, got, tt.want, stdout.String(), stderr.String())
		}
	}
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
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(f)), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// runOK ejecuta la CLI, comprueba el código de salida y devuelve la salida.
func runOK(t *testing.T, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := Run(args, &stdout, &stderr, "test"); code != exitFound {
		t.Fatalf("Run(%q) = %d, want %d\nstdout: %s\nstderr: %s", args, code, exitFound, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func TestRunIgnoresAccents(t *testing.T) {
	root := makeTree(t, []string{"Música/Canción", "otros"}, nil)
	out := runOK(t, "cancion", "-p", root)
	if !strings.Contains(out, filepath.Join(root, "Música", "Canción")) {
		t.Errorf("no encontró la carpeta con acentos:\n%s", out)
	}
}

func TestRunProjects(t *testing.T) {
	root := makeTree(t, []string{"web/src", "api", "notas"}, map[string]int{
		"web/package.json": 0,
		"api/go.mod":       0,
	})
	out := runOK(t, "--projects", "-p", root)
	for _, want := range []string{
		filepath.Join(root, "web") + "  (Node.js)",
		filepath.Join(root, "api") + "  (Go)",
		"Buscando proyectos en ",
		"Resultados : 2 proyectos encontrados",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("la salida no contiene %q:\n%s", want, out)
		}
	}
}

func TestRunSize(t *testing.T) {
	root := makeTree(t,
		[]string{"app/node_modules/lib/node_modules", "web/node_modules"},
		map[string]int{"app/node_modules/lib/index.js": 1000, "web/node_modules/big.js": 3000},
	)
	out := runOK(t, "node_modules", "--size", "-p", root)

	web := strings.Index(out, "2.9 KB  "+filepath.Join(root, "web", "node_modules"))
	app := strings.Index(out, "1,000 bytes  "+filepath.Join(root, "app", "node_modules"))
	if web < 0 || app < 0 || web > app {
		t.Errorf("se esperaban web (2.9 KB) y luego app (1,000 bytes):\n%s", out)
	}
	if strings.Contains(out, filepath.Join("lib", "node_modules")) {
		t.Errorf("no debería entrar en las carpetas encontradas:\n%s", out)
	}
	if !strings.Contains(out, "Tamaño     : 3.9 KB en total (2 archivos)") {
		t.Errorf("falta el total:\n%s", out)
	}
}

func TestRunModified(t *testing.T) {
	root := makeTree(t, []string{"viejo/informe", "nuevo/informe"}, nil)
	old := time.Now().AddDate(0, 0, -30)
	if err := os.Chtimes(filepath.Join(root, "viejo", "informe"), old, old); err != nil {
		t.Fatal(err)
	}

	out := runOK(t, "informe", "-m", "semana", "-p", root)
	if !strings.Contains(out, filepath.Join(root, "nuevo", "informe")) || strings.Contains(out, filepath.Join(root, "viejo", "informe")) {
		t.Errorf("solo debería aparecer nuevo/informe:\n%s", out)
	}
	if !strings.Contains(out, `Buscando carpetas "informe" modificadas en los últimos 7 días en `) {
		t.Errorf("cabecera inesperada:\n%s", out)
	}
}
