package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist/jumplisttest"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
)

// recentFixture crea un árbol con carpetas y archivos y una jump list que los
// nombra, y deja FFC_JUMPLIST_DIR apuntando a ella. Devuelve la raíz del
// árbol: las pruebas la pasan con -p, porque en Windows la carpeta temporal
// está dentro de AppData, que es oculta.
func recentFixture(t *testing.T, historyReason string) string {
	t.Helper()
	root := makeTree(t, []string{"Tesis", "Música/Canción & co", "Viejo"}, map[string]int{
		"Tesis/capítulo 1.docx":         0,
		"Tesis/capítulo 2.docx":         0,
		"Música/Canción & co/letra.txt": 0,
	})
	at := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }
	now := time.Now()
	dir := t.TempDir()
	err := jumplisttest.Write(dir, jumplist.AppID("Microsoft.Office.WINWORD.EXE.15"), 6, []jumplisttest.Entry{
		{Path: at("Tesis/capítulo 2.docx"), LastUsed: now.Add(-10 * time.Minute)},
		{Path: at("Tesis/capítulo 1.docx"), LastUsed: now.Add(-2 * time.Hour)},
		{Path: at("Música/Canción & co/letra.txt"), LastUsed: now.Add(-30 * time.Minute)},
		{Path: at("Viejo"), LastUsed: now.AddDate(0, 0, -40)},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(jumplist.EnvDir, dir)
	prev := historyOff
	historyOff = func() string { return historyReason }
	t.Cleanup(func() { historyOff = prev })
	return root
}

func wantContains(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("la salida no contiene %q:\n%s", want, out)
		}
	}
}

func TestRunRecent(t *testing.T) {
	root := recentFixture(t, "")
	out := runOK(t, "--recientes", "-p", root)
	tesis := strings.Index(out, filepath.Join(root, "Tesis"))
	musica := strings.Index(out, filepath.Join(root, "Música", "Canción & co"))
	viejo := strings.Index(out, filepath.Join(root, "Viejo"))
	if tesis < 0 || musica < 0 || viejo < 0 || !(tesis < musica && musica < viejo) {
		t.Errorf("deberían salir de la más reciente a la más antigua:\n%s", out)
	}
	wantContains(t, out,
		"Buscando carpetas recientes en el historial de Windows dentro de "+root,
		"(hace 10 min · 2 archivos)",
		"Resultados : 3 carpetas encontradas",
		"Historial  : 1 lista de Windows",
	)
}

func TestRunRecentFilters(t *testing.T) {
	root := recentFixture(t, "")
	out := runOK(t, "cancion", "--recientes", "-p", root)
	wantContains(t, out, filepath.Join(root, "Música", "Canción & co"), "Resultados : 1 carpeta encontrada")

	out = runOK(t, "--recientes", "-m", "semana", "-p", root)
	if strings.Contains(out, filepath.Join(root, "Viejo")) {
		t.Errorf("-m semana no debería mostrar la carpeta de hace 40 días:\n%s", out)
	}
	wantContains(t, out, "carpetas recientes usadas en los últimos 7 días", "Resultados : 2 carpetas encontradas")

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"nada-coincide", "--recientes", "-p", root}, &stdout, &stderr, "test"); code != exitNoMatch {
		t.Errorf("sin coincidencias: código %d, want %d", code, exitNoMatch)
	}
}

func TestRunRecentJSON(t *testing.T) {
	root := recentFixture(t, "")
	objs := jsonLines(t, runOK(t, "tesis", "--recientes", "-p", root, "--json"))
	if len(objs) != 1 || objs[0]["path"] != filepath.Join(root, "Tesis") || objs[0]["name"] != "Tesis" || objs[0]["files"] != float64(2) {
		t.Fatalf("JSON = %v", objs)
	}
	if s, _ := objs[0]["last_used"].(string); s == "" {
		t.Errorf("falta last_used: %v", objs[0])
	} else if _, err := time.Parse(time.RFC3339, s); err != nil {
		t.Errorf("last_used no es RFC 3339: %q", s)
	}
}

func TestRunRecentOpen(t *testing.T) {
	root := recentFixture(t, "")
	opened := stubExplorer(t, nil)
	runOK(t, "--recientes", "-o", "-p", root)
	if len(*opened) != 1 || (*opened)[0] != filepath.Join(root, "Tesis") {
		t.Errorf("abrió %q, want solo la más reciente", *opened)
	}
}

// Si Windows no guarda el historial, se avisa por stderr (la salida sigue
// siendo válida para scripts) y se muestra lo que haya.
func TestRunRecentWarnsWhenHistoryIsOff(t *testing.T) {
	root := recentFixture(t, "está desactivado")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--recientes", "-p", root, "--json"}, &stdout, &stderr, "test"); code != exitFound {
		t.Fatalf("código %d\n%s", code, stderr.String())
	}
	wantContains(t, stderr.String(), "aviso: Windows no está guardando las carpetas recientes: está desactivado")
	if len(jsonLines(t, stdout.String())) != 3 {
		t.Errorf("stdout = %q", stdout.String())
	}
}

// Sin carpeta de jump lists (un Windows que nunca guardó nada) no hay
// resultados, pero no es un error.
func TestRunRecentWithoutLists(t *testing.T) {
	recentFixture(t, "")
	t.Setenv(jumplist.EnvDir, filepath.Join(t.TempDir(), "no-existe"))
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--recientes"}, &stdout, &stderr, "test"); code != exitNoMatch {
		t.Errorf("código %d, want %d\n%s", code, exitNoMatch, stderr.String())
	}
	wantContains(t, stdout.String(), "Historial  : 0 listas de Windows")
}

func TestParseArgsRecent(t *testing.T) {
	cfg, err := parseArgs([]string{"--recientes", "tesis", "-m", "ayer"})
	if err != nil || !cfg.recent || cfg.term != "tesis" || cfg.kind() != query.Recent {
		t.Errorf("parseArgs = %+v, %v", cfg, err)
	}
	for _, args := range [][]string{
		{"--recientes", "--apps"},
		{"--recientes", "--projects"},
		{"--recientes", "--size"},
	} {
		if _, err := parseArgs(args); err == nil {
			t.Errorf("parseArgs(%q) no devolvió error", args)
		}
	}
}
