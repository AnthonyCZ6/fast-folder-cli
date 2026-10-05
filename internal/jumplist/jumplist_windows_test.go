package jumplist

import (
	"testing"

	"golang.org/x/sys/windows"
)

// knownfolder:{GUID} se convierte en la carpeta de este equipo.
func TestLocalPathKnownFolder(t *testing.T) {
	want, err := windows.KnownFolderPath(windows.FOLDERID_Downloads, 0)
	if err != nil {
		t.Skip("este equipo no tiene carpeta de Descargas")
	}
	e := Entry{Path: "knownfolder:{374DE290-123F-4565-9164-39C4925E467B}"}
	if got := e.LocalPath(); got != want {
		t.Errorf("LocalPath = %q, want %q", got, want)
	}
}

// Las jump lists reales de este equipo se leen sin errores: un fallo aquí
// indica un formato que el lector no conoce.
func TestReadDirThisPC(t *testing.T) {
	dir, err := defaultDir()
	if err != nil {
		t.Skip(err)
	}
	lists, damaged, err := ReadDir(dir)
	if err != nil {
		t.Skipf("no se puede leer %s: %v", dir, err)
	}
	if damaged > 0 {
		t.Errorf("%d de %d jump lists de este equipo no se pudieron leer", damaged, len(lists)+damaged)
	}
	t.Logf("%d jump lists; historial: %q", len(lists), HistoryOff())
}
