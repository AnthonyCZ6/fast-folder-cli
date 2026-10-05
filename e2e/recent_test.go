package e2e

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist/jumplisttest"
)

// writeHistory escribe una jump list por programa (su AppUserModelID) en una
// carpeta temporal y hace que el programa la lea en lugar del historial de
// Windows (FFC_JUMPLIST_DIR).
func writeHistory(t *testing.T, lists map[string][]jumplisttest.Entry) {
	t.Helper()
	dir := t.TempDir()
	for app, entries := range lists {
		if err := jumplisttest.Write(dir, jumplist.AppID(app), 6, entries); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(jumplist.EnvDir, dir)
}

// recentJSON es una carpeta reciente de la salida --json.
type recentJSON struct {
	Path  string   `json:"path"`
	Files int      `json:"files"`
	Apps  []string `json:"apps"`
}

func recentFolders(t *testing.T, out string) []recentJSON {
	t.Helper()
	var folders []recentJSON
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var f recentJSON
		if err := json.Unmarshal([]byte(line), &f); err != nil {
			t.Fatalf("línea JSON no válida %q: %v", line, err)
		}
		folders = append(folders, f)
	}
	return folders
}

// --recientes y --con con un historial sintético: las carpetas de lo que se
// abrió, la más reciente primero, y solo las de Word con --con word.
func TestRecent(t *testing.T) {
	root := makeTree(t, []string{"Tesis", "Música/Canción & co"}, nil)
	tesis, cancion := filepath.Join(root, "Tesis"), filepath.Join(root, "Música", "Canción & co")
	now := time.Now()
	writeHistory(t, map[string][]jumplisttest.Entry{
		"Microsoft.Office.WINWORD.EXE.15": {
			{Path: filepath.Join(tesis, "capítulo 1.docx"), LastUsed: now.Add(-10 * time.Minute)},
		},
		"Microsoft.Windows.Explorer": {{Path: cancion, LastUsed: now.Add(-5 * time.Minute)}},
	})

	// -p limita a root: en Windows la carpeta temporal está dentro de
	// AppData, que es oculta.
	r := mustRun(t, 0, "--recientes", "-p", root)
	assertContains(t, r.stdout, "Buscando carpetas recientes", "hace 10 min · 1 archivo · Word", "Resultados : 2 carpetas encontradas")
	if i, j := strings.Index(r.stdout, cancion), strings.Index(r.stdout, tesis); i < 0 || j < i {
		t.Errorf("la más reciente (%s) debería salir primero:\n%s", cancion, r.stdout)
	}

	r = mustRun(t, 0, "--con", "word", "--json", "-p", root)
	folders := recentFolders(t, r.stdout)
	if len(folders) != 1 || folders[0].Path != tesis || folders[0].Files != 1 || !slices.Equal(folders[0].Apps, []string{"Word"}) {
		t.Errorf("--con word --json = %+v, want solo %s, de Word", folders, tesis)
	}

	r = mustRun(t, 2, "--con", "photoshop")
	assertContains(t, r.stderr, `no se reconoce el programa "photoshop". Tienen historial:`, "Word")
}
