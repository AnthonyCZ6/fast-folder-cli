package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist/jumplisttest"
)

// fcd --con word de punta a punta, con un historial sintético: el modo
// interactivo muestra solo las carpetas de Word, se elige con Enter y la
// terminal queda en la carpeta.
func TestFcdRecent(t *testing.T) {
	if exe == "" {
		t.Skip("con -short no se compila el programa")
	}
	root := makeTree(t, []string{"Música/Canción & co", "otros"}, nil)
	want := filepath.Join(root, "Música", "Canción & co")
	writeHistory(t, map[string][]jumplisttest.Entry{
		"Microsoft.Office.WINWORD.EXE.15": {{Path: filepath.Join(want, "letra.docx"), LastUsed: time.Now()}},
		"Microsoft.Windows.Explorer":      {{Path: filepath.Join(root, "otros"), LastUsed: time.Now()}},
	})
	dir := filepath.Dir(exe)
	for _, script := range []string{"fcd.ps1", "fcd.cmd"} {
		copyScript(t, script, dir)
	}

	// -a: en Windows la carpeta temporal está dentro de AppData, que es
	// oculta.
	tests := []struct {
		name string
		cmd  func(pwd string) *exec.Cmd
	}{
		{"powershell.exe", func(pwd string) *exec.Cmd { return fcdPowerShellArgs("powershell.exe", dir, "--con word -a", pwd) }},
		{"pwsh.exe", func(pwd string) *exec.Cmd { return fcdPowerShellArgs("pwsh.exe", dir, "--con word -a", pwd) }},
		{"cmd.exe", func(pwd string) *exec.Cmd { return fcdCmdArgs(dir, "--con word -a", pwd) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := exec.LookPath(tt.name); err != nil {
				t.Skipf("%s no está instalado", tt.name)
			}
			pwd := filepath.Join(t.TempDir(), "pwd.txt")
			c := startConsole(t, tt.cmd(pwd))
			c.waitFor(t, "1 carpeta encontrada")
			c.waitFor(t, "(Word)")
			c.send(t, "\r")
			if code := c.wait(t); code != 0 {
				t.Fatalf("código de salida = %d, want 0\n%s", code, c.screen())
			}
			assertSameDir(t, readPwd(t, pwd), want)
		})
	}
}

// envAddRecent hace que el binario de estas pruebas, en lugar de ejecutarlas,
// registre como abiertos los archivos que nombra (uno por línea) y termine.
// Hace falta otro proceso: Windows guarda la jump list al terminar el
// proceso que la escribe.
const envAddRecent = "FFC_E2E_ADD_RECENT"

func init() {
	if files := os.Getenv(envAddRecent); files != "" {
		os.Exit(addRecent(strings.Split(files, "\n")))
	}
}

var (
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	procSetAppID      = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	procAddRecentDocs = shell32.NewProc("SHAddToRecentDocs")
)

// shardPathW indica a SHAddToRecentDocs que recibe una ruta en UTF-16.
const shardPathW = 3

// addRecent registra files como abiertos recientemente, como hace el
// Explorador al abrir un archivo, y devuelve el código de salida.
func addRecent(files []string) int {
	id, err := windows.UTF16PtrFromString("FastFolderCLI.E2E")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if r, _, _ := procSetAppID.Call(uintptr(unsafe.Pointer(id))); r != 0 {
		fmt.Fprintf(os.Stderr, "SetCurrentProcessExplicitAppUserModelID: 0x%X\n", r)
		return 1
	}
	for i, f := range files {
		path, err := windows.UTF16PtrFromString(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if i > 0 {
			time.Sleep(1100 * time.Millisecond) // fechas distintas, para comprobar el orden
		}
		procAddRecentDocs.Call(shardPathW, uintptr(unsafe.Pointer(path)))
	}
	time.Sleep(3 * time.Second) // da tiempo al shell a anotarlos
	return 0
}

// sameDirs indica si got son las carpetas de want, en el mismo orden. Compara
// con os.SameFile porque Windows puede guardar la ruta larga
// (C:\Users\runneradmin\...) aunque se le diera la corta (C:\Users\RUNNER~1\...,
// la de TEMP en los runners de GitHub).
func sameDirs(got []recentJSON, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i, f := range got {
		if sameDir(f.Path, want[i]) != nil {
			return false
		}
	}
	return true
}

// within devuelve las carpetas de folders que son alguna de dirs.
func within(folders []recentJSON, dirs ...string) []recentJSON {
	var out []recentJSON
	for _, f := range folders {
		for _, d := range dirs {
			if sameDir(f.Path, d) == nil {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// --recientes con el historial real de Windows: otro proceso registra dos
// archivos como abiertos y el programa encuentra sus carpetas, la más
// reciente primero, con las rutas exactas. Solo en el CI, porque escribe en el
// historial de quien ejecuta las pruebas, y solo si Windows lo guarda (no en
// Windows 11 ARM de GitHub).
func TestRecentWithRealHistory(t *testing.T) {
	if exe == "" {
		t.Skip("con -short no se compila el programa")
	}
	if os.Getenv("CI") == "" {
		t.Skip("solo en el CI: registraría archivos en tu historial de recientes")
	}
	root := makeTree(t, nil, map[string]int{
		"Tesis final (2)/capítulo 1.docx": 6,
		"Música/Canción & co/letra.txt":   6,
	})
	tesis, cancion := filepath.Join(root, "Tesis final (2)"), filepath.Join(root, "Música", "Canción & co")
	if r := run(t, "--recientes", "-p", root); strings.Contains(r.stderr, "no está guardando") {
		t.Skipf("este Windows no guarda el historial de recientes:\n%s", r.stderr)
	}

	helper := exec.Command(os.Args[0])
	helper.Env = append(os.Environ(),
		envAddRecent+"="+filepath.Join(tesis, "capítulo 1.docx")+"\n"+filepath.Join(cancion, "letra.txt"))
	if out, err := helper.CombinedOutput(); err != nil {
		t.Fatalf("no se pudieron registrar los archivos: %v\n%s", err, out)
	}

	// Windows escribe la jump list al terminar el proceso; puede tardar. Se
	// busca en todo el historial (-a: la carpeta temporal está dentro de
	// AppData, que es oculta) y se miran solo las carpetas de la prueba.
	var all []recentJSON
	for deadline := time.Now().Add(45 * time.Second); time.Now().Before(deadline); time.Sleep(time.Second) {
		all = recentFolders(t, run(t, "--recientes", "--json", "-a").stdout)
		if sameDirs(within(all, tesis, cancion), cancion, tesis) {
			return
		}
	}
	t.Errorf("--recientes -a no tiene %s ni %s, en ese orden; tiene %d carpetas:", cancion, tesis, len(all))
	for _, f := range all[:min(len(all), 15)] {
		t.Logf("  %s (%d archivos)", f.Path, f.Files)
	}
}
