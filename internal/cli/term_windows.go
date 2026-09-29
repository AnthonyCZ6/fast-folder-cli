//go:build windows

package cli

import (
	"os"
	"syscall"
)

const enableVirtualTerminalProcessing = 0x0004

var procSetConsoleMode = syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode")

// enableANSI activa el procesamiento de secuencias ANSI en la consola de
// Windows. Devuelve false si f no es una consola (salida redirigida a un
// archivo o tubería) o si la consola no las admite (anterior a Windows 10).
func enableANSI(f *os.File) bool {
	h := syscall.Handle(f.Fd())
	var mode uint32
	if err := syscall.GetConsoleMode(h, &mode); err != nil {
		return false
	}
	if mode&enableVirtualTerminalProcessing != 0 {
		return true
	}
	ok, _, _ := procSetConsoleMode.Call(uintptr(h), uintptr(mode|enableVirtualTerminalProcessing))
	return ok != 0
}
