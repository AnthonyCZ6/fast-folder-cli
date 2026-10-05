package recent

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist/jumplisttest"
)

// IDs de las jump lists de las pruebas de History.
var (
	wordID     = jumplist.AppID("Microsoft.Office.WINWORD.EXE.15")
	explorerID = jumplist.AppID("Microsoft.Windows.Explorer")
	unknownID  = jumplist.AppID("Programa.Desconocido")
)

func testHistory() History {
	return History{
		Lists: []jumplist.List{
			{AppID: wordID, Entries: []jumplist.Entry{{Path: `C:\Tesis\capítulo 1.docx`}}},
			{AppID: explorerID}, // sin historial
			{AppID: unknownID, Entries: []jumplist.Entry{{Path: `C:\Otro\x.txt`}}},
		},
		Names: map[uint64]AppName{
			wordID:     {Name: "Word", Aliases: []string{"WINWORD"}},
			explorerID: {Name: "Explorador de archivos", Aliases: []string{"explorer"}},
		},
	}
}

func TestLoadLists(t *testing.T) {
	dir := t.TempDir()
	if err := jumplisttest.Write(dir, wordID, 6, []jumplisttest.Entry{
		{Path: `C:\Tesis\capítulo 1.docx`, LastUsed: now},
	}); err != nil {
		t.Fatal(err)
	}
	damaged := filepath.Join(dir, "1234"+jumplist.Ext)
	if err := os.WriteFile(damaged, []byte("no es un compound file"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(jumplist.EnvDir, dir)

	h, err := LoadLists()
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Lists) != 1 || h.Lists[0].AppID != wordID || len(h.Lists[0].Entries) != 1 {
		t.Errorf("Lists = %+v", h.Lists)
	}
	if h.Damaged != 1 {
		t.Errorf("Damaged = %d, want 1", h.Damaged)
	}
	if h.Off != "" || h.Names != nil {
		t.Errorf("LoadLists no debe consultar el sistema: Off = %q, Names = %v", h.Off, h.Names)
	}
}

// Un Windows que nunca guardó nada no tiene la carpeta: no es un error.
func TestLoadListsWithoutDir(t *testing.T) {
	t.Setenv(jumplist.EnvDir, filepath.Join(t.TempDir(), "no-existe"))
	h, err := LoadLists()
	if err != nil || len(h.Lists) != 0 {
		t.Errorf("LoadLists() = %+v, %v; want vacío y sin error", h, err)
	}
}

func TestLoad(t *testing.T) {
	t.Setenv(jumplist.EnvDir, t.TempDir())
	h, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if h.Names == nil {
		t.Error("Load debe dar el nombre de los programas")
	}
}

func TestAppsNamed(t *testing.T) {
	h := testHistory()
	for _, term := range []string{"word", "winword", "WOR*"} {
		ids, label, err := h.AppsNamed(term)
		if err != nil || !reflect.DeepEqual(ids, map[uint64]bool{wordID: true}) || label != "Word" {
			t.Errorf("AppsNamed(%q) = %v, %q, %v", term, ids, label, err)
		}
	}

	// Varios programas: la etiqueta los nombra a todos, por orden alfabético.
	_, label, err := h.AppsNamed("*r*")
	if err != nil || label != "Explorador de archivos, Word" {
		t.Errorf("AppsNamed(*r*) = %q, %v", label, err)
	}
}

// Si no se reconoce el programa, el error dice cuáles tienen historial (no
// los que tienen la lista vacía ni los que no se identifican).
func TestAppsNamedUnknown(t *testing.T) {
	_, _, err := testHistory().AppsNamed("photoshop")
	if err == nil {
		t.Fatal("AppsNamed(photoshop) no devolvió error")
	}
	want := `no se reconoce el programa "photoshop". Tienen historial: Word`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}

	_, _, err = History{}.AppsNamed("word")
	if err == nil || strings.Contains(err.Error(), "Tienen historial") {
		t.Errorf("sin historial: error = %v", err)
	}
	if _, _, err := testHistory().AppsNamed("[a-"); err == nil {
		t.Error("un patrón mal formado debe dar error")
	}
}

func TestAppsOf(t *testing.T) {
	f := Folder{AppIDs: []uint64{wordID, unknownID, explorerID}}
	got := testHistory().AppsOf(f)
	if want := []string{"Explorador de archivos", "Word"}; !reflect.DeepEqual(got, want) {
		t.Errorf("AppsOf = %q, want %q", got, want)
	}
	if got := testHistory().AppsOf(Folder{AppIDs: []uint64{unknownID}}); len(got) != 0 {
		t.Errorf("AppsOf(desconocido) = %q, want vacío", got)
	}
}

func TestSummary(t *testing.T) {
	tests := []struct {
		f    Folder
		apps []string
		want string
	}{
		{Folder{LastUsed: ago(10 * time.Minute), Items: 2}, []string{"Word"}, "hace 10 min · 2 archivos · Word"},
		{Folder{LastUsed: ago(10 * time.Minute), Items: 1}, nil, "hace 10 min · 1 archivo"},
		{Folder{LastUsed: ago(10 * time.Minute)}, []string{"Explorador de archivos"}, "hace 10 min · Explorador de archivos"},
		{Folder{}, nil, "fecha desconocida"},
		{Folder{LastUsed: ago(time.Hour + time.Minute)}, []string{"A", "B", "C", "D"}, "hace 1 h · A, B, C, …"},
	}
	for _, tt := range tests {
		if got := tt.f.Summary(tt.apps, now); got != tt.want {
			t.Errorf("Summary(%+v, %q) = %q, want %q", tt.f, tt.apps, got, tt.want)
		}
	}

	// No modifica la lista que recibe.
	apps := []string{"A", "B", "C", "D"}
	Folder{}.Summary(apps[:3], now)
	if apps[3] != "D" {
		t.Errorf("Summary modificó la lista: %q", apps)
	}
}
