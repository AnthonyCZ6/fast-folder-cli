package jumplist

import (
	"os"
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

// Las rutas de una unidad de red (Z: conectada a \servidor\carpeta) no son
// locales: comprobar si existen podría bloquear la búsqueda si el servidor no
// responde.
func TestLocalPathRemoteDrive(t *testing.T) {
	prev := driveType
	t.Cleanup(func() { driveType = prev })
	var asked []string
	driveType = func(root *uint16) uint32 {
		asked = append(asked, windows.UTF16PtrToString(root))
		if windows.UTF16PtrToString(root) == `Z:\` {
			return windows.DRIVE_REMOTE
		}
		return windows.DRIVE_FIXED
	}
	for path, want := range map[string]string{
		`Z:\Equipo\informe.docx`: "",
		`C:\Tesis\capítulo.docx`: `C:\Tesis\capítulo.docx`,
	} {
		if got := (Entry{Path: path}).LocalPath(); got != want {
			t.Errorf("LocalPath(%q) = %q, want %q", path, got, want)
		}
	}
	if len(asked) != 2 {
		t.Errorf("se consultaron las unidades %q", asked)
	}
}

// La unidad del sistema no es de red.
func TestRemoteDriveSystem(t *testing.T) {
	if remoteDrive(os.Getenv("SystemDrive") + `\Windows`) {
		t.Error("la unidad del sistema no debería ser de red")
	}
}
