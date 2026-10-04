//go:build windows

package launch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

// Explorer abre path en una nueva ventana del Explorador de Windows.
//
// explorer.exe devuelve el código de salida 1 incluso cuando funciona, así que
// solo se lanza el proceso sin esperar ni interpretar su resultado.
func Explorer(path string) error {
	return start(exec.Command("explorer", path))
}

// Reveal abre en el Explorador la carpeta que contiene file, con file
// seleccionado.
//
// explorer.exe no separa sus argumentos como los demás programas: lee
// "/select," y la ruta como uno solo, y una coma en la ruta lo confunde si no
// va entre comillas. Por eso la línea de órdenes se escribe a mano en lugar de
// dejar que Go ponga las comillas. Un nombre de archivo de Windows no puede
// contener comillas, así que la ruta no necesita más protección.
func Reveal(file string) error {
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: revealCmdLine(file)}
	return start(cmd)
}

// revealCmdLine devuelve la línea de órdenes de explorer.exe que muestra file
// seleccionado.
func revealCmdLine(file string) string {
	return `explorer.exe /select,"` + file + `"`
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
		cmd := exec.Command(shell, "-NoLogo")
		cmd.Dir = path
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
		if err = start(cmd); err == nil {
			return nil
		}
	}
	return err
}
