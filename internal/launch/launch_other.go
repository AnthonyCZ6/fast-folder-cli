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

// ShowApp solo funciona en Windows.
func ShowApp(string, string) error { return errWindowsOnly }

// Terminal solo funciona en Windows.
func Terminal(string) error { return errWindowsOnly }

// TerminalWith solo funciona en Windows.
func TerminalWith(string, string) error { return errWindowsOnly }

// Notepad solo funciona en Windows.
func Notepad(string) error { return errWindowsOnly }

// Editor abre target con el editor command (vacío: code).
func Editor(command, target string) error {
	if command == "" {
		command = "code"
	}
	return start(exec.Command(command, target))
}

// VSCode abre path en Visual Studio Code.
func VSCode(path string) error {
	return start(exec.Command("code", path))
}
