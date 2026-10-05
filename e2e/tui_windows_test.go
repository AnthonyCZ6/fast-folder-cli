package e2e

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/charmbracelet/x/xpty"
)

// Estas pruebas ejecutan el modo interactivo en una pseudoconsola de Windows
// (ConPTY), como en una terminal de verdad: se le envían teclas y se lee la
// pantalla con un emulador de terminal.

// waitTimeout es lo máximo que se espera a que aparezca algo en la pantalla o
// a que termine el programa. Es generoso porque Windows PowerShell 5.1 puede
// tardar mucho en arrancar en un runner ocupado; normalmente la espera acaba
// en menos de un segundo.
const waitTimeout = 60 * time.Second

// console es un programa ejecutándose en una pseudoconsola.
type console struct {
	cmd *exec.Cmd
	emu *vt.SafeEmulator
}

// errNoConsole indica que este Windows no tiene pseudoconsola (ConPTY).
var errNoConsole = errors.New("no hay pseudoconsola disponible")

// openConsole ejecuta cmd en una pseudoconsola de 120×30. release termina el
// programa si sigue en marcha y libera la consola. Lo usan estas pruebas y la
// prueba larga (soak), que registra los errores en lugar de detenerse.
func openConsole(cmd *exec.Cmd) (c *console, release func(), err error) {
	pty, err := xpty.NewPty(120, 30)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", errNoConsole, err)
	}
	emu := vt.NewSafeEmulator(120, 30)
	if err := pty.Start(cmd); err != nil {
		pty.Close()
		emu.Close()
		return nil, nil, fmt.Errorf("no se pudo iniciar %v: %w", cmd.Args, err)
	}
	go func() { _, _ = io.Copy(emu, pty) }() // lo que dibuja el programa
	go func() { _, _ = io.Copy(pty, emu) }() // teclas y respuestas del terminal
	release = func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
		}
		pty.Close()
		emu.Close()
	}
	return &console{cmd: cmd, emu: emu}, release, nil
}

// startConsole ejecuta cmd en una pseudoconsola de 120×30.
func startConsole(t *testing.T, cmd *exec.Cmd) *console {
	t.Helper()
	c, release, err := openConsole(cmd)
	switch {
	case errors.Is(err, errNoConsole):
		t.Skip(err)
	case err != nil:
		t.Fatal(err)
	}
	t.Cleanup(release)
	return c
}

// screen devuelve el texto de la pantalla, sin colores.
func (c *console) screen() string {
	return ansi.Strip(c.emu.Render())
}

// await espera a que la pantalla muestre want.
func (c *console) await(want string) error {
	for deadline := time.Now().Add(waitTimeout); time.Now().Before(deadline); {
		if strings.Contains(c.screen(), want) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("la pantalla no muestra %q", want)
}

// waitFor espera a que la pantalla muestre want.
func (c *console) waitFor(t *testing.T, want string) {
	t.Helper()
	if err := c.await(want); err != nil {
		t.Fatalf("%v:\n%s", err, c.screen())
	}
}

// press escribe keys como si se pulsaran en el teclado.
func (c *console) press(keys string) error {
	_, err := io.WriteString(c.emu.InputPipe(), keys)
	return err
}

// send escribe keys como si se pulsaran en el teclado.
func (c *console) send(t *testing.T, keys string) {
	t.Helper()
	if err := c.press(keys); err != nil {
		t.Fatal(err)
	}
}

// exitCode espera a que el programa termine y devuelve su código de salida.
func (c *console) exitCode() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	err := xpty.WaitProcess(ctx, c.cmd)
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		return exitErr.ExitCode(), nil
	case err != nil:
		return -1, fmt.Errorf("el programa no terminó: %w", err)
	}
	return c.cmd.ProcessState.ExitCode(), nil
}

// wait espera a que el programa termine y devuelve su código de salida.
func (c *console) wait(t *testing.T) int {
	t.Helper()
	code, err := c.exitCode()
	if err != nil {
		t.Fatalf("%v\n%s", err, c.screen())
	}
	return code
}

// Formulario, búsqueda con Enter y salida con q, en una consola real.
func TestInteractiveSearch(t *testing.T) {
	if exe == "" {
		t.Skip("con -short no se compila el programa")
	}
	root := makeTree(t, []string{"docs/Proyecto-A", "src/proyecto-b", "otros"}, nil)
	c := startConsole(t, exec.Command(exe, "-p", root))

	c.waitFor(t, "Buscar")
	c.waitFor(t, "Carpeta elegida")
	c.send(t, "proyecto\r")
	c.waitFor(t, "2 carpetas encontradas")
	c.send(t, "q")
	if code := c.wait(t); code != 0 {
		t.Errorf("código de salida = %d, want 0", code)
	}
}

// fcd de punta a punta, con el programa real: busca, se elige con Enter y la
// terminal queda en la carpeta, aunque su nombre lleve acentos y &.
func TestFcdInteractive(t *testing.T) {
	if exe == "" {
		t.Skip("con -short no se compila el programa")
	}
	root := makeTree(t, []string{"Música/Canción & co", "otros"}, nil)
	want := filepath.Join(root, "Música", "Canción & co")
	dir := filepath.Dir(exe)
	for _, script := range []string{"fcd.ps1", "fcd.cmd"} {
		copyScript(t, script, dir)
	}

	tests := []struct {
		name string
		cmd  func(pwd string) *exec.Cmd
	}{
		{"powershell.exe", func(pwd string) *exec.Cmd { return fcdPowerShell("powershell.exe", dir, root, pwd) }},
		{"pwsh.exe", func(pwd string) *exec.Cmd { return fcdPowerShell("pwsh.exe", dir, root, pwd) }},
		{"cmd.exe", func(pwd string) *exec.Cmd { return fcdCmd(dir, root, pwd) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := exec.LookPath(tt.name); err != nil {
				t.Skipf("%s no está instalado", tt.name)
			}
			pwd := filepath.Join(t.TempDir(), "pwd.txt")
			c := startConsole(t, tt.cmd(pwd))
			c.waitFor(t, "1 carpeta encontrada")
			c.send(t, "\r")
			if code := c.wait(t); code != 0 {
				t.Fatalf("código de salida = %d, want 0\n%s", code, c.screen())
			}
			assertSameDir(t, readPwd(t, pwd), want)
		})
	}
}

// copyScript copia el script de fcd junto al programa compilado, como lo
// instala el asistente.
func copyScript(t *testing.T, name, dir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "shell", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// fcdPowerShell ejecuta "fcd cancion -p root" en shell y guarda en pwd la
// carpeta en la que queda la sesión.
func fcdPowerShell(shell, dir, root, pwd string) *exec.Cmd {
	return fcdPowerShellArgs(shell, dir, "cancion -p '"+root+"'", pwd)
}

// fcdPowerShellArgs ejecuta "fcd args" en shell (args con la sintaxis de
// PowerShell) y guarda en pwd la carpeta en la que queda la sesión.
func fcdPowerShellArgs(shell, dir, args, pwd string) *exec.Cmd {
	script := "& '" + filepath.Join(dir, "fcd.ps1") + "' " + args + "; " +
		"(Get-Location).Path | Out-File -LiteralPath '" + pwd + "' -Encoding utf8"
	return exec.Command(shell, "-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
}

// fcdCmd hace lo mismo en cmd.exe.
func fcdCmd(dir, root, pwd string) *exec.Cmd {
	return fcdCmdArgs(dir, `cancion -p "`+root+`"`, pwd)
}

// fcdCmdArgs ejecuta "fcd args" en cmd.exe (args con la sintaxis de cmd).
// Con la página de códigos UTF-8, "cd" escribe la carpeta en UTF-8.
func fcdCmdArgs(dir, args, pwd string) *exec.Cmd {
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /c chcp 65001 >nul & call "` +
		filepath.Join(dir, "fcd.cmd") + `" ` + args + ` & cd > "` + pwd + `"`}
	return cmd
}

// pwdOf lee la carpeta que guardó la terminal, sin la marca BOM que añade
// Windows PowerShell 5.1.
func pwdOf(pwd string) (string, error) {
	data, err := os.ReadFile(pwd)
	if err != nil {
		return "", fmt.Errorf("la terminal no guardó su carpeta: %w", err)
	}
	return strings.TrimSpace(strings.TrimPrefix(string(data), "\ufeff")), nil
}

func readPwd(t *testing.T, pwd string) string {
	t.Helper()
	dir, err := pwdOf(pwd)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// sameDir comprueba que got y want son la misma carpeta.
func sameDir(got, want string) error {
	gotInfo, err := os.Stat(got)
	if err != nil {
		return fmt.Errorf("la terminal no quedó en una carpeta válida: %q", got)
	}
	wantInfo, err := os.Stat(want)
	if err != nil {
		return err
	}
	if !os.SameFile(gotInfo, wantInfo) {
		return fmt.Errorf("la terminal quedó en %q, want %q", got, want)
	}
	return nil
}

func assertSameDir(t *testing.T, got, want string) {
	t.Helper()
	if err := sameDir(got, want); err != nil {
		t.Error(err)
	}
}
