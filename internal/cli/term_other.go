//go:build !windows

package cli

import "os"

// enableANSI indica si f es una terminal; en sistemas tipo Unix los colores
// ANSI funcionan sin configuración adicional.
func enableANSI(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
