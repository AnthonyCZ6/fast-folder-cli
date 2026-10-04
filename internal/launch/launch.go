// Package launch abre carpetas con otros programas (el Explorador de Windows,
// Visual Studio Code o una terminal nueva) y copia rutas al portapapeles.
package launch

import (
	"os/exec"

	"github.com/atotto/clipboard"
)

// ShowApp abre en el Explorador la ubicación de un programa: la carpeta de
// exe con exe seleccionado o, si no se conoce el ejecutable (exe vacío), la
// carpeta dir.
func ShowApp(dir, exe string) error {
	if exe != "" {
		return Reveal(exe)
	}
	return Explorer(dir)
}

// CopyPath copia path al portapapeles.
func CopyPath(path string) error {
	return clipboard.WriteAll(path)
}

// start lanza cmd sin esperar a que termine.
func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
