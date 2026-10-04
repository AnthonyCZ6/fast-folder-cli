// Package shell contiene las pruebas de los scripts del comando fcd.
package shell

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// El binario de pruebas hace también de fast-folder-cli.exe falso: copiado
// junto a fcd.cmd, se comporta según una de estas variables de entorno.

// fakeExitEnv hace que termine nada más arrancar con ese código de salida.
const fakeExitEnv = "FCD_FAKE_EXIT"

// fakeChooseEnv hace que, como al elegir esa carpeta con Enter, la escriba en
// el archivo de --cd-file y termine con 0.
const fakeChooseEnv = "FCD_FAKE_CHOOSE"

func TestMain(m *testing.M) {
	if code, ok := os.LookupEnv(fakeExitEnv); ok {
		n, err := strconv.Atoi(code)
		if err != nil {
			os.Exit(99)
		}
		os.Exit(n)
	}
	if dir, ok := os.LookupEnv(fakeChooseEnv); ok {
		os.Exit(fakeChoose(dir))
	}
	os.Exit(m.Run())
}

// fakeChoose escribe dir en el archivo que sigue a --cd-file en los
// argumentos, como hace fast-folder-cli al elegir una carpeta.
func fakeChoose(dir string) int {
	args := os.Args[1:]
	for i, a := range args {
		if a == "--cd-file" && i+1 < len(args) {
			if err := os.WriteFile(args[i+1], []byte(dir), 0o600); err != nil {
				return 98
			}
			return 0
		}
	}
	return 97
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		t.Fatal(err)
	}
}

// setupFcd copia fcd.cmd y el fast-folder-cli.exe falso a una carpeta
// temporal y devuelve la ruta de la copia de fcd.cmd.
func setupFcd(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("fcd.cmd solo funciona en Windows")
	}
	dir := t.TempDir()
	copyFile(t, "fcd.cmd", filepath.Join(dir, "fcd.cmd"))
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	copyFile(t, self, filepath.Join(dir, "fast-folder-cli.exe"))
	return filepath.Join(dir, "fcd.cmd")
}

// Si no se elige ninguna carpeta, fcd.cmd termina con el código de
// fast-folder-cli: 0 al salir sin elegir y 2 si las opciones no son válidas.
func TestFcdCmdExitCode(t *testing.T) {
	fcd := setupFcd(t)
	for _, want := range []int{0, 2} {
		cmd := exec.Command("cmd.exe", "/c", fcd, "--opcion-desconocida")
		cmd.Env = append(os.Environ(), fakeExitEnv+"="+strconv.Itoa(want))
		cmd.Stdout = io.Discard
		got := 0
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatal(err)
			}
			got = exitErr.ExitCode()
		}
		if got != want {
			t.Errorf("fast-folder-cli terminó con %d y fcd.cmd con %d", want, got)
		}
	}
}

// Al elegir una carpeta, fcd.cmd entra en ella, en la misma consola, y
// termina con 0.
func TestFcdCmdChangesDirectory(t *testing.T) {
	fcd := setupFcd(t)
	want := filepath.Join(t.TempDir(), "elegida")
	if err := os.Mkdir(want, 0o755); err != nil {
		t.Fatal(err)
	}

	// Con "call", la consola sigue tras fcd.cmd; "cd" muestra dónde se quedó.
	cmd := exec.Command("cmd.exe", "/c", "call", fcd, "tesis", "&&", "cd")
	cmd.Env = append(os.Environ(), fakeChooseEnv+"="+want)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("fcd.cmd: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	got := strings.TrimSpace(lines[len(lines)-1])
	gotInfo, err := os.Stat(got)
	if err != nil {
		t.Fatalf("la consola no quedó en una carpeta válida: %q", out)
	}
	wantInfo, err := os.Stat(want)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Errorf("la consola quedó en %q, want %q", got, want)
	}
}

// fcd.cmd debe contener solo caracteres ASCII y finales de línea CRLF:
// cmd.exe lo vuelve a leer tras cambiar la página de códigos, y con saltos
// de línea sin CR puede leer mal las etiquetas de goto.
func TestFcdCmdIsASCIIWithCRLF(t *testing.T) {
	data, err := os.ReadFile("fcd.cmd")
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range data {
		if b >= 0x80 {
			t.Fatalf("fcd.cmd tiene un carácter no ASCII en la posición %d", i)
		}
		if b == '\n' && (i == 0 || data[i-1] != '\r') {
			t.Fatalf("fcd.cmd tiene un salto de línea sin CR en la posición %d", i)
		}
	}
}
