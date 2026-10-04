package launch

import (
	"testing"
	"time"

	"github.com/atotto/clipboard"
)

// CopyPath deja la ruta en el portapapeles tal cual, con acentos y símbolos.
// Sin portapapeles (un Linux sin xclip, por ejemplo) la prueba se salta. Lo
// que hubiera en el portapapeles se restaura al terminar.
func TestCopyPath(t *testing.T) {
	prev, prevErr := clipboard.ReadAll()
	want := `C:\Música\A&B 100%`
	if err := CopyPath(want); err != nil {
		t.Skipf("no hay portapapeles disponible: %v", err)
	}
	if prevErr == nil {
		t.Cleanup(func() { _ = clipboard.WriteAll(prev) })
	}
	// Otro programa (el historial del portapapeles, por ejemplo) puede tenerlo
	// abierto un instante justo después de escribir: se reintenta un poco.
	var got string
	var err error
	for range 10 {
		if got, err = clipboard.ReadAll(); err == nil && got == want {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("portapapeles = %q, %v; want %q", got, err, want)
}
