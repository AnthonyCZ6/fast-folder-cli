package e2e

import (
	"context"
	"errors"
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
// a que termine el programa.
const waitTimeout = 15 * time.Second

// console es un programa ejecutándose en una pseudoconsola.
type console struct {
	cmd *exec.Cmd
	emu *vt.SafeEmulator
}

// startConsole ejecuta cmd en una pseudoconsola de 120×30.
func startConsole(t *testing.T, cmd *exec.Cmd) *console {
	t.Helper()
	pty, err := xpty.NewPty(120, 30)
	if err != nil {
		t.Skipf("no hay pseudoconsola disponible: %v", err)
	}
	emu := vt.NewSafeEmulator(120, 30)
	if err := pty.Start(cmd); err != nil {
		pty.Close()
		t.Fatalf("no se pudo iniciar %v: %v", cmd.Args, err)
	}
	go func() { _, _ = io.Copy(emu, pty) }() // lo que dibuja el programa
	go func() { _, _ = io.Copy(pty, emu) }() // teclas y respuestas del terminal
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
		}
		pty.Close()
		emu.Close()
	})
	return &console{cmd: cmd, emu: emu}
}

// screen devuelve el texto de la pantalla, sin colores.
func (c *console) screen() string {
	return ansi.Strip(c.emu.Render())
}

// waitFor espera a que la pantalla muestre want.
func (c *console) waitFor(t *testing.T, want string) {
	t.Helper()
	for deadline := time.Now().Add(waitTimeout); time.Now().Before(deadline); {
		if strings.Contains(c.screen(), want) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("la pantalla no muestra %q:\n%s", want, c.screen())
}

// send escribe keys como si se pulsaran en el teclado.
func (c *console) send(t *testing.T, keys string) {
	t.Helper()
	if _, err := io.WriteString(c.emu.InputPipe(), keys); err != nil {
		t.Fatal(err)
	}
}

// wait espera a que el programa termine y devuelve su código de salida.
func (c *console) wait(t *testing.T) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	err := xpty.WaitProcess(ctx, c.cmd)
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		return exitErr.ExitCode()
	case err != nil:
		t.Fatalf("el programa no terminó: %v\n%s", err, c.screen())
	}
	return c.cmd.ProcessState.ExitCode()
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
	script := "& '" + filepath.Join(dir, "fcd.ps1") + "' cancion -p '" + root + "'; " +
		"(Get-Location).Path | Out-File -LiteralPath '" + pwd + "' -Encoding utf8"
	return exec.Command(shell, "-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
}

// fcdCmd hace lo mismo en cmd.exe. Con la página de códigos UTF-8, "cd"
// escribe la carpeta en UTF-8.
func fcdCmd(dir, root, pwd string) *exec.Cmd {
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /c chcp 65001 >nul & call "` +
		filepath.Join(dir, "fcd.cmd") + `" cancion -p "` + root + `" & cd > "` + pwd + `"`}
	return cmd
}

// readPwd lee la carpeta que guardó la terminal, sin la marca BOM que añade
// Windows PowerShell 5.1.
func readPwd(t *testing.T, pwd string) string {
	t.Helper()
	data, err := os.ReadFile(pwd)
	if err != nil {
		t.Fatalf("la terminal no guardó su carpeta: %v", err)
	}
	return strings.TrimSpace(strings.TrimPrefix(string(data), "\ufeff"))
}

// assertSameDir comprueba que got y want son la misma carpeta.
func assertSameDir(t *testing.T, got, want string) {
	t.Helper()
	gotInfo, err := os.Stat(got)
	if err != nil {
		t.Fatalf("la terminal no quedó en una carpeta válida: %q", got)
	}
	wantInfo, err := os.Stat(want)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Errorf("la terminal quedó en %q, want %q", got, want)
	}
}
