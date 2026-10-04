package launch

import (
	"testing"

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
	if got, err := clipboard.ReadAll(); err != nil || got != want {
		t.Errorf("portapapeles = %q, %v; want %q", got, err, want)
	}
}
