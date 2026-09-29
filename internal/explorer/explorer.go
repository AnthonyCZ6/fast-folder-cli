// Package explorer abre carpetas en el Explorador de Windows.
package explorer

import (
	"errors"
	"os/exec"
	"runtime"
)

// Open abre path en una nueva ventana del Explorador de Windows.
//
// explorer.exe devuelve el código de salida 1 incluso cuando funciona, así que
// solo se lanza el proceso (Start) sin esperar ni interpretar su resultado.
func Open(path string) error {
	if runtime.GOOS != "windows" {
		return errors.New("--open solo está disponible en Windows")
	}
	cmd := exec.Command("explorer", path)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
