package recent

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
)

// tree crea las carpetas (rutas relativas con "/") y los archivos vacíos de
// files bajo un directorio temporal. Devuelve ese directorio y una función
// que da la ruta completa de un elemento. Las opciones de las pruebas usan
// Within con ese directorio: en Windows está dentro de AppData, que es una
// carpeta oculta.
func tree(t *testing.T, dirs []string, files ...string) (string, func(string) string) {
	t.Helper()
	root := t.TempDir()
	at := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }
	for _, d := range dirs {
		if err := os.MkdirAll(at(d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(at(f)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at(f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, at
}

var now = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func ago(d time.Duration) time.Time { return now.Add(-d) }

func paths(folders []Folder) []string {
	var out []string
	for _, f := range folders {
		out = append(out, f.Path)
	}
	return out
}

func TestFindGroupsByFolder(t *testing.T) {
	root, at := tree(t, []string{"Proyectos/api"}, "Tesis/capítulo 1.docx", "Tesis/capítulo 2.docx", "Música/Canción & co/letra.txt")
	lists := []jumplist.List{
		{AppID: 1, Entries: []jumplist.Entry{
			{Path: at("Tesis/capítulo 1.docx"), LastUsed: ago(3 * time.Hour)},
			{Path: at("Tesis/capítulo 2.docx"), LastUsed: ago(time.Hour)},
			{Path: at("Tesis/borrado.docx"), LastUsed: ago(2 * time.Hour)}, // ya no existe: cuenta su carpeta
			{Path: at("NoExiste/x.txt"), LastUsed: ago(time.Minute)},       // su carpeta tampoco existe
			{Path: "https://ejemplo.com/informe.xlsx", LastUsed: ago(time.Minute)},
		}},
		{AppID: 2, Entries: []jumplist.Entry{
			{Path: at("Proyectos/api"), LastUsed: ago(30 * time.Minute)}, // una carpeta abierta
			{Path: at("Tesis/capítulo 1.docx"), LastUsed: ago(5 * time.Hour)},
			{Path: at("Música/Canción & co/letra.txt"), LastUsed: ago(48 * time.Hour)},
		}},
	}
	want := []Folder{
		{Path: at("Proyectos/api"), LastUsed: ago(30 * time.Minute), Items: 0, AppIDs: []uint64{2}},
		{Path: at("Tesis"), LastUsed: ago(time.Hour), Items: 3, AppIDs: []uint64{1, 2}},
		{Path: at("Música/Canción & co"), LastUsed: ago(48 * time.Hour), Items: 1, AppIDs: []uint64{2}},
	}
	if got := Find(lists, Options{Within: root}); !reflect.DeepEqual(got, want) {
		t.Errorf("Find =\n%+v\nwant\n%+v", got, want)
	}
}

func TestFindFilters(t *testing.T) {
	root, at := tree(t, []string{"Clientes/Año 2024/informes", "Clientes/node_modules/pkg", ".oculta/notas", "Otros/tesis"})
	entries := []jumplist.Entry{
		{Path: at("Clientes/Año 2024/informes"), LastUsed: ago(time.Hour)},
		{Path: at("Clientes/node_modules/pkg"), LastUsed: ago(2 * time.Hour)},
		{Path: at(".oculta/notas"), LastUsed: ago(3 * time.Hour)},
		{Path: at("Otros/tesis"), LastUsed: ago(72 * time.Hour)},
	}
	lists := []jumplist.List{{AppID: 7, Entries: entries}}
	anio, err := search.NewMatcher("ano")
	if err != nil {
		t.Fatal(err)
	}
	isTesis := func(n string) bool { return n == "tesis" }
	tests := []struct {
		name string
		opts Options
		want []string
	}{
		{"sin filtros: las ocultas no", Options{}, []string{at("Clientes/Año 2024/informes"), at("Clientes/node_modules/pkg"), at("Otros/tesis")}},
		{"con las ocultas", Options{IncludeHidden: true}, []string{at("Clientes/Año 2024/informes"), at("Clientes/node_modules/pkg"), at(".oculta/notas"), at("Otros/tesis")}},
		{"nombre de la carpeta", Options{Match: isTesis}, []string{at("Otros/tesis")}},
		{"el término no mira las superiores", Options{Match: anio.Match}, nil},
		{"periodo", Options{After: ago(24 * time.Hour)}, []string{at("Clientes/Año 2024/informes"), at("Clientes/node_modules/pkg")}},
		{"periodo cerrado", Options{After: ago(150 * time.Minute), Before: ago(90 * time.Minute)}, []string{at("Clientes/node_modules/pkg")}},
		{"dentro de una carpeta", Options{Within: at("Clientes")}, []string{at("Clientes/Año 2024/informes"), at("Clientes/node_modules/pkg")}},
		{"excluida una superior, sin acentos", Options{Exclude: []string{"NODE_MODULES", "ano 2024"}}, []string{at("Otros/tesis")}},
		{"dentro de una carpeta oculta", Options{Within: at(".oculta")}, []string{at(".oculta/notas")}},
		{"otra jump list", Options{AppIDs: map[uint64]bool{8: true}}, nil},
		{"esta jump list", Options{AppIDs: map[uint64]bool{7: true}, Match: isTesis}, []string{at("Otros/tesis")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opts.Within == "" {
				tt.opts.Within = root
			}
			if got := paths(Find(lists, tt.opts)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Find =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

// Con un periodo, cuenta cuándo se abrió cada elemento: una carpeta usada
// ayer y hoy aparece en "ayer" aunque su último uso sea de hoy.
func TestFindPeriodPerItem(t *testing.T) {
	root, at := tree(t, nil, "Tesis/a.docx", "Tesis/b.docx")
	lists := []jumplist.List{{AppID: 1, Entries: []jumplist.Entry{
		{Path: at("Tesis/a.docx"), LastUsed: ago(time.Hour)},
		{Path: at("Tesis/b.docx"), LastUsed: ago(26 * time.Hour)},
	}}}
	got := Find(lists, Options{Within: root, After: ago(48 * time.Hour), Before: ago(24 * time.Hour)})
	want := []Folder{{Path: at("Tesis"), LastUsed: ago(26 * time.Hour), Items: 1, AppIDs: []uint64{1}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Find =\n%+v\nwant\n%+v", got, want)
	}
}

// Una entrada sin fecha (algunas carpetas ancladas) cuenta sin periodo y no
// con él.
func TestFindNoDate(t *testing.T) {
	root, at := tree(t, []string{"Anclada"})
	lists := []jumplist.List{{AppID: 1, Entries: []jumplist.Entry{{Path: at("Anclada"), Pinned: true}}}}
	if got := paths(Find(lists, Options{Within: root})); !reflect.DeepEqual(got, []string{at("Anclada")}) {
		t.Errorf("sin periodo: %q", got)
	}
	if got := Find(lists, Options{Within: root, After: ago(time.Hour)}); got != nil {
		t.Errorf("con periodo: %+v", got)
	}
}
