// Package shell contiene las pruebas de los scripts del comando fcd.
package shell

import (
	"encoding/base64"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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

// fakePwdEnv hace que, si no recibe --cd-file, escriba la carpeta actual en
// ese archivo: así la prueba ve dónde quedó la consola después de fcd, sin
// depender de su página de códigos.
const fakePwdEnv = "FCD_FAKE_PWD"

func TestMain(m *testing.M) {
	if code, ok := os.LookupEnv(fakeExitEnv); ok {
		n, err := strconv.Atoi(code)
		if err != nil {
			os.Exit(99)
		}
		os.Exit(n)
	}
	if file, ok := os.LookupEnv(fakePwdEnv); ok && !slices.Contains(os.Args[1:], "--cd-file") {
		os.Exit(fakePwd(file))
	}
	if dir, ok := os.LookupEnv(fakeChooseEnv); ok {
		os.Exit(fakeChoose(dir))
	}
	os.Exit(m.Run())
}

// fakePwd escribe la carpeta actual en file, en UTF-8.
func fakePwd(file string) int {
	dir, err := os.Getwd()
	if err != nil {
		return 96
	}
	if err := os.WriteFile(file, []byte(dir), 0o600); err != nil {
		return 95
	}
	return 0
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

// setupScript copia el script name (fcd.cmd o fcd.ps1) y el
// fast-folder-cli.exe falso a una carpeta temporal y devuelve la ruta de la
// copia del script.
func setupScript(t *testing.T, name string) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip(name + " solo funciona en Windows")
	}
	dir := t.TempDir()
	copyFile(t, name, filepath.Join(dir, name))
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	copyFile(t, self, filepath.Join(dir, "fast-folder-cli.exe"))
	return filepath.Join(dir, name)
}

// trickyNames son nombres de carpeta con caracteres que cmd.exe o
// PowerShell podrían interpretar: acentos, espacios, &, ', %, !, ^ y
// corchetes.
var trickyNames = []string{"Canción", "con espacios", "A&B", "it's", "100%", "¡hola!", "a^b", "[corchetes]"}

// makeTrickyDirs crea las carpetas de trickyNames en un directorio temporal
// y devuelve sus rutas.
func makeTrickyDirs(t *testing.T) []string {
	t.Helper()
	base := t.TempDir()
	var dirs []string
	for _, name := range trickyNames {
		dir := filepath.Join(base, name)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, dir)
	}
	return dirs
}

// assertSameDir comprueba que got y want son la misma carpeta.
func assertSameDir(t *testing.T, got, want string) {
	t.Helper()
	gotInfo, err := os.Stat(got)
	if err != nil {
		t.Fatalf("la consola no quedó en una carpeta válida: %q", got)
	}
	wantInfo, err := os.Stat(want)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Errorf("la consola quedó en %q, want %q", got, want)
	}
}

// Si no se elige ninguna carpeta, fcd.cmd termina con el código de
// fast-folder-cli: 0 al salir sin elegir y 2 si las opciones no son válidas.
func TestFcdCmdExitCode(t *testing.T) {
	fcd := setupScript(t, "fcd.cmd")
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
	fcd := setupScript(t, "fcd.cmd")
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

// fcd.cmd entra en carpetas cuyo nombre lleva caracteres especiales para
// cmd.exe (&, %, !, ^) o acentos, que llegan en UTF-8.
func TestFcdCmdEntersTrickyFolders(t *testing.T) {
	fcd := setupScript(t, "fcd.cmd")
	fake := filepath.Join(filepath.Dir(fcd), "fast-folder-cli.exe")
	for _, want := range makeTrickyDirs(t) {
		t.Run(filepath.Base(want), func(t *testing.T) {
			// Tras fcd, el ejecutable falso escribe dónde quedó la consola.
			pwd := filepath.Join(t.TempDir(), "pwd.txt")
			cmd := exec.Command("cmd.exe", "/c", "call", fcd, "x", "&&", fake)
			cmd.Env = append(os.Environ(), fakeChooseEnv+"="+want, fakePwdEnv+"="+pwd)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("fcd.cmd: %v\n%s", err, out)
			}
			got, err := os.ReadFile(pwd)
			if err != nil {
				t.Fatal(err)
			}
			assertSameDir(t, string(got), want)
		})
	}
}

// powerShells devuelve las PowerShell instaladas: Windows PowerShell 5.1
// (powershell.exe) y PowerShell 7 (pwsh.exe).
func powerShells(t *testing.T) []string {
	t.Helper()
	var shells []string
	for _, s := range []string{"powershell.exe", "pwsh.exe"} {
		if _, err := exec.LookPath(s); err == nil {
			shells = append(shells, s)
		}
	}
	if len(shells) == 0 {
		t.Skip("no hay PowerShell instalado")
	}
	return shells
}

// powerShell ejecuta script en shell con la directiva de ejecución policy y
// con FCD_FAKE_CHOOSE=choose, y devuelve su salida.
func powerShell(shell, policy, choose, script string) ([]byte, error) {
	cmd := exec.Command(shell, "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", policy, "-Command", script)
	cmd.Env = append(os.Environ(), fakeChooseEnv+"="+choose)
	return cmd.CombinedOutput()
}

// fcd.ps1 deja la sesión de PowerShell (5.1 y 7) en la carpeta elegida,
// también con acentos, espacios, comillas simples o corchetes en el nombre.
func TestFcdPs1EntersTrickyFolders(t *testing.T) {
	fcd := setupScript(t, "fcd.ps1")
	dirs := makeTrickyDirs(t)
	for _, shell := range powerShells(t) {
		for _, want := range dirs {
			t.Run(shell+"/"+filepath.Base(want), func(t *testing.T) {
				// La carpeta final se imprime en Base64 para no depender de la
				// codificación de la consola.
				script := "& '" + fcd + "' x; [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes((Get-Location).Path))"
				out, err := powerShell(shell, "Bypass", want, script)
				if err != nil {
					t.Fatalf("fcd.ps1: %v\n%s", err, out)
				}
				lines := strings.Fields(string(out))
				if len(lines) == 0 {
					t.Fatal("PowerShell no mostró nada")
				}
				got, err := base64.StdEncoding.DecodeString(lines[len(lines)-1])
				if err != nil {
					t.Fatalf("salida inesperada: %q", out)
				}
				assertSameDir(t, string(got), want)
			})
		}
	}
}

// Con la directiva de ejecución Restricted, PowerShell no ejecuta fcd.ps1 y
// muestra el error de scripts deshabilitados del que habla el README.
// UnauthorizedAccess es el identificador del error, igual en cualquier idioma.
func TestFcdPs1RestrictedPolicy(t *testing.T) {
	fcd := setupScript(t, "fcd.ps1")
	for _, shell := range powerShells(t) {
		out, _ := powerShell(shell, "Restricted", t.TempDir(), "$ErrorView = 'NormalView'; & '"+fcd+"' x")
		if !strings.Contains(string(out), "UnauthorizedAccess") {
			t.Errorf("%s: se esperaba el error de directiva de ejecución:\n%s", shell, out)
		}
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
