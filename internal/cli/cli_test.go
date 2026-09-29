package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestFormatInt(t *testing.T) {
	tests := map[int64]string{0: "0", 7: "7", 999: "999", 1000: "1,000", 52341: "52,341", 1234567: "1,234,567", -4200: "-4,200"}
	for in, want := range tests {
		if got := formatInt(in); got != want {
			t.Errorf("formatInt(%d) = %q, want %q", in, got, want)
		}
	}
}
