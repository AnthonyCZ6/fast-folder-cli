package recent

import (
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
)

// Una carpeta dentro de otra con el atributo de oculta no se muestra sin
// IncludeHidden, como en la búsqueda sin --all.
func TestFindHiddenAttribute(t *testing.T) {
	root, at := tree(t, []string{"Datos/proyecto", "Visible"})
	p, err := syscall.UTF16PtrFromString(at("Datos"))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_HIDDEN); err != nil {
		t.Fatal(err)
	}
	lists := []jumplist.List{{AppID: 1, Entries: []jumplist.Entry{
		{Path: at("Datos/proyecto"), LastUsed: ago(time.Hour)},
		{Path: at("Visible"), LastUsed: ago(2 * time.Hour)},
	}}}
	if got := paths(Find(lists, Options{Within: root})); !reflect.DeepEqual(got, []string{at("Visible")}) {
		t.Errorf("sin IncludeHidden: %q", got)
	}
	got := paths(Find(lists, Options{Within: root, IncludeHidden: true}))
	if !reflect.DeepEqual(got, []string{at("Datos/proyecto"), at("Visible")}) {
		t.Errorf("con IncludeHidden: %q", got)
	}
}
