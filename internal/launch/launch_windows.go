//go:build windows

package launch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/pathutil"
)

// Explorer abre path en una nueva ventana del Explorador de Windows.
//
// explorer.exe devuelve el código de salida 1 incluso cuando funciona, así que
// solo se lanza el proceso sin esperar ni interpretar su resultado.
func Explorer(path string) error {
	return start(explorerCmd(path))
}

func explorerCmd(path string) *exec.Cmd {
	return exec.Command("explorer", path)
}

// Reveal abre en el Explorador la carpeta que contiene file, con file
// seleccionado.
func Reveal(file string) error {
	return start(revealCmd(file))
}

// revealCmd devuelve la orden de Reveal.
//
// explorer.exe no separa sus argumentos como los demás programas: lee
// "/select," y la ruta como uno solo, y una coma en la ruta lo confunde si no
// va entre comillas. Por eso la línea de órdenes se escribe a mano en lugar de
// dejar que Go ponga las comillas. Un nombre de archivo de Windows no puede
// contener comillas, así que la ruta no necesita más protección.
func revealCmd(file string) *exec.Cmd {
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + file + `"`}
	return cmd
}

// ShowApp abre en el Explorador la ubicación de un programa: la carpeta de
// exe con exe seleccionado o, si no se conoce el ejecutable (exe vacío), la
// carpeta dir.
func ShowApp(dir, exe string) error {
	return start(showAppCmd(dir, exe))
}

func showAppCmd(dir, exe string) *exec.Cmd {
	if exe != "" {
		return revealCmd(exe)
	}
	return explorerCmd(dir)
}

// VSCode abre path en Visual Studio Code.
//
// El comando "code" del PATH es un archivo por lotes (code.cmd), y cmd.exe
// interpretaría caracteres como & o ^ del nombre de la carpeta. Por eso se
// ejecuta directamente Code.exe, que está en la carpeta superior a code.cmd.
func VSCode(path string) error {
	exe, err := findVSCode()
	if err != nil {
		return err
	}
	return start(exec.Command(exe, path))
}

func findVSCode() (string, error) {
	var candidates []string
	if cmd, err := exec.LookPath("code"); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(cmd), "..", "Code.exe"))
	}
	for _, env := range []string{"LOCALAPPDATA", "ProgramFiles"} {
		if base := os.Getenv(env); base != "" {
			if env == "LOCALAPPDATA" {
				base = filepath.Join(base, "Programs")
			}
			candidates = append(candidates, filepath.Join(base, "Microsoft VS Code", "Code.exe"))
		}
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", errors.New("no se encontró Visual Studio Code")
}

// Terminal abre una ventana de PowerShell nueva en path (PowerShell 7 si está
// instalado). Si Windows Terminal es la terminal predeterminada, la ventana se
// abre en él.
func Terminal(path string) error {
	var err error
	for _, shell := range []string{"pwsh.exe", "powershell.exe"} {
		if err = start(terminalCmd(shell, path)); err == nil {
			return nil
		}
	}
	return err
}

// TerminalWith abre la terminal kind en una ventana nueva, en path: "wt"
// (Windows Terminal), "pwsh", "powershell" o "cmd". Si kind está vacía, o la
// elegida no se puede abrir (no está instalada), usa Terminal.
func TerminalWith(kind, path string) error {
	if kind != "" {
		if err := start(terminalKindCmd(kind, path)); err == nil {
			return nil
		}
	}
	return Terminal(path)
}

// terminalKindCmd devuelve la orden que abre la terminal kind en path. La
// carpeta se pasa como carpeta de trabajo, no como argumento: Windows
// Terminal separa sus órdenes con ";", que puede aparecer en el nombre.
func terminalKindCmd(kind, path string) *exec.Cmd {
	switch kind {
	case "wt":
		cmd := exec.Command("wt.exe", "-d", ".")
		cmd.Dir = path
		return cmd
	case "cmd":
		cmd := exec.Command("cmd.exe")
		cmd.Dir = path
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
		return cmd
	}
	return terminalCmd(kind+".exe", path)
}

// Editor abre target (una carpeta o un archivo) con el editor command: vacío
// o "code" es VS Code, buscado como en VSCode; si no, un programa del PATH
// ("cursor", "notepad++") o la ruta de un ejecutable (admite %VARIABLES%).
func Editor(command, target string) error {
	cmd, err := editorCmd(command, target)
	if err != nil {
		return err
	}
	return start(cmd)
}

// editorCmd devuelve la orden de Editor. Un editor que no es VS Code se
// ejecuta en la carpeta de target y recibe solo su nombre ("." si es una
// carpeta): así ninguna ruta pasa por la línea de órdenes, y un editor que
// sea un script .cmd (como cursor) no puede interpretar & o ^ del nombre.
func editorCmd(command, target string) (*exec.Cmd, error) {
	if command == "" || strings.EqualFold(command, "code") {
		exe, err := findVSCode()
		if err != nil {
			return nil, err
		}
		return exec.Command(exe, target), nil
	}
	dir, arg := target, "."
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		dir, arg = filepath.Dir(target), filepath.Base(target)
	}
	cmd := exec.Command(pathutil.ExpandEnv(command), arg)
	cmd.Dir = dir
	return cmd, nil
}

// Notepad abre file en el Bloc de notas.
func Notepad(file string) error {
	return start(exec.Command("notepad.exe", file))
}

// terminalCmd devuelve la orden que abre shell en una consola nueva, en la
// carpeta path.
func terminalCmd(shell, path string) *exec.Cmd {
	cmd := exec.Command(shell, "-NoLogo")
	cmd.Dir = path
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
	return cmd
}
