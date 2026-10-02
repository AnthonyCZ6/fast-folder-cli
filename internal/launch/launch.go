// Package launch abre carpetas con otros programas (el Explorador de Windows,
// Visual Studio Code o una terminal nueva) y copia rutas al portapapeles.
package launch

import (
	"os/exec"

	"github.com/atotto/clipboard"
)

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
