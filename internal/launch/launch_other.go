//go:build !windows

package launch

import (
	"errors"
	"os/exec"
)

var errWindowsOnly = errors.New("solo está disponible en Windows")

// Explorer solo funciona en Windows.
func Explorer(string) error { return errWindowsOnly }

// Reveal solo funciona en Windows.
func Reveal(string) error { return errWindowsOnly }

// Terminal solo funciona en Windows.
func Terminal(string) error { return errWindowsOnly }

// VSCode abre path en Visual Studio Code.
func VSCode(path string) error {
	return start(exec.Command("code", path))
}
