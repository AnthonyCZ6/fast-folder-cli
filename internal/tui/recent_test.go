package tui

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/recent"
)

// AppID de las jump lists de las pruebas.
const (
	wordList     = 1
	explorerList = 2
)

// withHistory crea unas carpetas y hace que el historial de rec diga que
// Word abrió archivos en Tesis y en "Canción & co", y que el Explorador abrió
// Viejo hace 40 días. Devuelve la ruta de cada carpeta.
func withHistory(t *testing.T, rec *recorder) (tesis, cancion, viejo string) {
	t.Helper()
	root := makeTree(t, "Tesis", "Música/Canción & co", "Viejo")
	at := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }
	now := time.Now()
	rec.history = recent.History{
		Lists: []jumplist.List{
			{AppID: wordList, Entries: []jumplist.Entry{
				{Path: at("Tesis/capítulo 2.docx"), LastUsed: now.Add(-10 * time.Minute)},
				{Path: at("Tesis/capítulo 1.docx"), LastUsed: now.Add(-2 * time.Hour)},
				{Path: at("Música/Canción & co/letra.txt"), LastUsed: now.Add(-30 * time.Minute)},
			}},
			{AppID: explorerList, Entries: []jumplist.Entry{
				{Path: at("Viejo"), LastUsed: now.AddDate(0, 0, -40)},
			}},
		},
		Names: map[uint64]recent.AppName{
			wordList:     {Name: "Word", Aliases: []string{"WINWORD"}},
			explorerList: {Name: "Explorador de archivos"},
		},
	}
	return at("Tesis"), at("Música/Canción & co"), at("Viejo")
}

// selectRecent elige Tipo: Recientes (← desde Carpetas da la última opción)
// desde el campo de búsqueda, y deja el foco en Tipo.
func selectRecent(m model) model {
	return send(m, keys("down", "down", "left")...)
}

// withHidden pasa a Ocultas: Sí desde Tipo. Las pruebas lo necesitan porque
// en Windows la carpeta temporal está dentro de AppData, que es oculta, y con
// Recientes no hay ubicación que la incluya.
func withHidden(m model) model {
	return send(m, keys("down", "down", "right")...)
}

func resultPaths(m model) []string {
	var out []string
	for _, r := range m.results {
		out = append(out, r.Path)
	}
	return out
}

func TestRecentSearch(t *testing.T) {
	m, rec := newTestModel(testLocations...)
	tesis, cancion, viejo := withHistory(t, rec)
	m = selectRecent(m)
	if m.focus != fieldKind || m.kind() != query.Recent {
		t.Fatalf("foco = %v tipo = %v, want Tipo: Recientes", m.focus, m.kind())
	}
	m = runSearch(t, withHidden(m))

	if got, want := resultPaths(m), []string{tesis, cancion, viejo}; !slices.Equal(got, want) {
		t.Fatalf("resultados = %q, want %q (la más reciente primero)", got, want)
	}
	view := m.View().Content
	for _, want := range []string{
		"Carpetas recientes", "en el historial de Windows",
		"Tesis", "hace 10 min · 2 archivos · Word",
		"Canción & co", "hace 30 min · 1 archivo · Word",
		"Explorador de archivos",
		"3 carpetas encontradas", "2 listas de Windows",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("la vista no contiene %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "analizadas") || strings.Contains(view, "no está guardando") {
		t.Errorf("con el historial activado no se analizan carpetas ni hay aviso:\n%s", view)
	}

	// Enter abre la carpeta en el Explorador y la recuerda, como cualquier
	// carpeta.
	m = send(m, keys("enter")...)
	if want := []string{"explorer:" + tesis}; !slices.Equal(rec.calls, want) {
		t.Errorf("acciones = %v, want %v", rec.calls, want)
	}
	if want := []string{tesis}; !slices.Equal(rec.remembered, want) {
		t.Errorf("recordadas = %v, want %v", rec.remembered, want)
	}
}

// El término filtra por el nombre de la carpeta, y la fecha, por cuándo se
// usó.
func TestRecentSearchFilters(t *testing.T) {
	m, rec := newTestModel(testLocations...)
	tesis, cancion, viejo := withHistory(t, rec)

	m = runSearch(t, withHidden(selectRecent(send(m, typeText("can*")...))))
	if got := resultPaths(m); !slices.Equal(got, []string{cancion}) {
		t.Errorf("con can*: %q, want solo %q", got, cancion)
	}

	m = send(m, keys("left")...)
	m.input.SetValue("")
	m = send(m, keys("down", "down", "left", "left")...) // Usada: Últimos 7 días
	m = runSearch(t, m)
	if got := resultPaths(m); !slices.Equal(got, []string{tesis, cancion}) {
		t.Errorf("últimos 7 días: %q, want sin %q", got, viejo)
	}
	if view := m.View().Content; !strings.Contains(view, "Carpetas recientes usadas en los últimos 7 días") {
		t.Errorf("la cabecera debería decir el periodo:\n%s", view)
	}
}

// Con Recientes la ubicación no se aplica (se busca en todo el historial):
// se muestra atenuada y ↑↓ la saltan. La fecha pasa a ser la de uso.
func TestRecentFormSkipsLocation(t *testing.T) {
	m, _ := newTestModel(testLocations...)
	m = selectRecent(m)

	m = send(m, keys("up")...)
	if m.focus != fieldTerm {
		t.Errorf("↑ desde Tipo: foco = %v, want Buscar", m.focus)
	}
	m = send(m, keys("down")...)
	if m.focus != fieldKind {
		t.Errorf("↓ desde Buscar: foco = %v, want Tipo", m.focus)
	}
	view := m.View().Content
	for _, want := range []string{"Recientes", "no se aplica a las carpetas recientes", "Usada", "cuándo se abrió algo en ella"} {
		if !strings.Contains(view, want) {
			t.Errorf("la vista no contiene %q:\n%s", want, view)
		}
	}
	if strings.Count(view, "no se aplica") != 1 {
		t.Errorf("solo Ubicación debería verse atenuada:\n%s", view)
	}
}

// Si Windows no guarda el historial, se avisa debajo de los resultados (o en
// lugar de ellos) con el motivo.
func TestRecentHistoryOff(t *testing.T) {
	m, rec := newTestModel(testLocations...)
	withHistory(t, rec)
	rec.history.Off = "está desactivado en Configuración"
	m = runSearch(t, withHidden(selectRecent(m)))
	notice := "Windows no está guardando las carpetas recientes: está desactivado en Configuración"
	if view := m.View().Content; len(m.results) != 3 || !strings.Contains(view, notice) {
		t.Errorf("con resultados debería verse el aviso:\n%s", view)
	}

	rec.history.Lists = nil
	m = runSearch(t, send(m, keys("left")...))
	view := m.View().Content
	for _, want := range []string{"No se encontró ninguna carpeta reciente.", notice} {
		if !strings.Contains(view, want) {
			t.Errorf("sin resultados, la vista no contiene %q:\n%s", want, view)
		}
	}
}

// Con la lista llena no queda sitio para el aviso: los resultados mandan.
func TestRecentHistoryOffWithFullList(t *testing.T) {
	m, rec := newTestModel(testLocations...)
	rec.history.Off = "está desactivado"
	var entries []jumplist.Entry
	for range 30 {
		entries = append(entries, jumplist.Entry{Path: makeTree(t), LastUsed: time.Now()})
	}
	rec.history.Lists = []jumplist.List{{AppID: wordList, Entries: entries}}
	m = runSearch(t, withHidden(selectRecent(m)))
	view := m.View().Content
	if len(m.results) != 30 || strings.Contains(view, "no está guardando") {
		t.Errorf("%d resultados; el aviso no cabe y no debería verse:\n%s", len(m.results), view)
	}
	if lines := strings.Count(view, "\n") + 1; lines != m.height {
		t.Errorf("la pantalla tiene %d líneas, want %d", lines, m.height)
	}
}

func TestRecentLoadError(t *testing.T) {
	m, rec := newTestModel(testLocations...)
	rec.historyErr = errors.New(`no se pudo leer el historial de Windows (C:\x): acceso denegado`)
	m = runSearch(t, selectRecent(m))
	if m.searching || !m.statusErr || m.status != `No se pudo leer el historial de Windows (C:\x): acceso denegado` {
		t.Errorf("estado = %q (error %v, buscando %v), want el error", m.status, m.statusErr, m.searching)
	}
	if view := m.View().Content; !strings.Contains(view, "No se encontró ninguna carpeta reciente.") {
		t.Errorf("la vista debería quedar sin resultados:\n%s", view)
	}
}

// Si se vuelve al formulario con ← antes de que llegue el historial, la
// respuesta de esa búsqueda se descarta.
func TestAbandonedRecentSearchIsIgnored(t *testing.T) {
	m, rec := newTestModel(testLocations...)
	withHistory(t, rec)
	next, cmd := selectRecent(m).Update(key("enter"))
	m = next.(model)
	if !m.searching || cmd == nil {
		t.Fatal("la búsqueda de recientes debería estar en curso")
	}
	gen := m.gen
	m = send(m, keys("left")...)
	m = send(m, recentMsg{gen: gen, paths: []string{`C:\x`}})
	if len(m.results) != 0 || m.screen != screenForm {
		t.Errorf("la búsqueda abandonada no debería mostrar resultados: %+v", m.results)
	}
}

// Una etiqueta larga se recorta para que quepa en la línea dónde está la
// carpeta.
func TestRecentLongTagIsTruncated(t *testing.T) {
	m, rec := newTestModel(testLocations...)
	tesis, _, _ := withHistory(t, rec)
	rec.history.Lists = rec.history.Lists[:1]
	rec.history.Names[wordList] = recent.AppName{Name: strings.Repeat("Programa con un nombre larguísimo ", 3)}
	m = runSearch(t, withHidden(selectRecent(m)))

	// Cada carpeta, con el final de la carpeta que la contiene.
	parents := map[string]string{"Tesis": filepath.Base(filepath.Dir(tesis)), "Canción & co": "Música"}
	view := m.View().Content
	for name, parent := range parents {
		ok := false
		for _, line := range strings.Split(view, "\n") {
			if strings.Contains(line, name) && strings.Contains(line, "Programa") && strings.Contains(line, parent) {
				ok = true
			}
		}
		if !ok {
			t.Errorf("falta una línea con %s, el programa y %s:\n%s", name, parent, view)
		}
	}
}
