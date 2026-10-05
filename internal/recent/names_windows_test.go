package recent

import (
	"slices"
	"testing"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
)

// Cuántas jump lists de este equipo se identifican. No falla por las que no
// (apps desinstaladas, por ejemplo): lo anota para ver la cobertura real.
func TestAppNamesThisPC(t *testing.T) {
	names := AppNames()
	if _, ok := names[jumplist.AppID("Microsoft.Windows.Explorer")]; !ok {
		t.Error("el Explorador siempre debería identificarse")
	}
	dir, err := jumplist.Dir()
	if err != nil {
		t.Skip(err)
	}
	lists, _, err := jumplist.ReadDir(dir)
	if err != nil || len(lists) == 0 {
		t.Skip("este equipo no tiene jump lists")
	}
	var known []string
	for _, l := range lists {
		if n, ok := names[l.AppID]; ok {
			known = append(known, n.Name)
		}
	}
	slices.Sort(known)
	t.Logf("%d de %d jump lists identificadas: %v", len(known), len(lists), known)
}
