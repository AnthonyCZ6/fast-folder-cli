package tui

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
)

var specialKeys = map[string]rune{
	"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
	"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "backspace": tea.KeyBackspace,
	"home": tea.KeyHome, "end": tea.KeyEnd, "pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
}

// key construye la pulsación de una tecla especial ("down") o de un carácter.
func key(s string) tea.KeyPressMsg {
	if code, ok := specialKeys[s]; ok {
		return tea.KeyPressMsg{Code: code}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func keys(names ...string) []tea.Msg {
	msgs := make([]tea.Msg, len(names))
	for i, n := range names {
		msgs[i] = key(n)
	}
	return msgs
}

func typeText(s string) []tea.Msg {
	var msgs []tea.Msg
	for _, r := range s {
		msgs = append(msgs, key(string(r)))
	}
	return msgs
}

func send(m model, msgs ...tea.Msg) model {
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(model)
	}
	return m
}

// recorder registra las acciones ejecutadas sobre las carpetas, como
// "explorer:C:\ruta".
type recorder struct {
	calls []string
}

func (r *recorder) actions() actions {
	record := func(name string) func(string) error {
		return func(path string) error {
			r.calls = append(r.calls, name+":"+path)
			return nil
		}
	}
	return actions{
		explorer: record("explorer"),
		code:     record("code"),
		terminal: record("terminal"),
		copyPath: record("copy"),
	}
}

func newTestModel(locs ...location) (model, *recorder) {
	rec := &recorder{}
	m := newModel("test", locs, rec.actions())
	m = send(m, tea.WindowSizeMsg{Width: 100, Height: 20})
	return m, rec
}

// withResults pone el modelo en la pantalla de resultados con esas carpetas.
func withResults(m model, paths ...string) model {
	m.screen, m.searching, m.gen = screenResults, true, 1
	results := make([]search.Result, len(paths))
	for i, p := range paths {
		results[i] = search.Result{Path: p}
	}
	return send(m, resultsMsg{gen: 1, results: results, done: true})
}

var testLocations = []location{
	{Label: "Uno", Path: `C:\uno`},
	{Label: "Dos", Path: `C:\dos`},
	{Label: "Tres", Path: `C:\tres`},
}

func TestFormArrowNavigation(t *testing.T) {
	m, _ := newTestModel(testLocations...)
	if m.focus != fieldTerm {
		t.Fatalf("foco inicial = %v, want campo de búsqueda", m.focus)
	}

	m = send(m, keys("down")...)
	if m.focus != fieldLocation {
		t.Fatalf("↓: foco = %v, want ubicación", m.focus)
	}
	m = send(m, keys("right")...)
	if m.locIndex != 1 {
		t.Errorf("→: ubicación = %d, want 1", m.locIndex)
	}
	m = send(m, keys("right", "right")...) // da la vuelta
	if m.locIndex != 0 {
		t.Errorf("→→: ubicación = %d, want 0", m.locIndex)
	}
	m = send(m, keys("left")...)
	if m.locIndex != 2 {
		t.Errorf("←: ubicación = %d, want 2", m.locIndex)
	}

	m = send(m, keys("down", "right")...)
	if m.focus != fieldKind || !m.projects {
		t.Errorf("↓→: foco = %v proyectos = %v, want proyectos activados", m.focus, m.projects)
	}
	m = send(m, keys("left")...)
	if m.projects {
		t.Error("←: debería volver a buscar carpetas")
	}

	m = send(m, keys("down", "right", "right")...)
	if m.focus != fieldDate || m.dates[m.dateIndex].value != "ayer" {
		t.Errorf("↓→→: foco = %v fecha = %+v, want Ayer", m.focus, m.dates[m.dateIndex])
	}
	m = send(m, keys("left", "left", "left")...) // da la vuelta
	if m.dateIndex != len(dateOptions)-1 {
		t.Errorf("←←←: fecha = %d, want la última opción", m.dateIndex)
	}

	m = send(m, keys("down", "right")...)
	if m.focus != fieldHidden || !m.hidden {
		t.Errorf("↓→: foco = %v ocultas = %v, want ocultas activadas", m.focus, m.hidden)
	}
	m = send(m, keys("left")...)
	if m.hidden {
		t.Error("←: las ocultas deberían desactivarse")
	}

	m = send(m, keys("down")...) // da la vuelta al primer campo
	if m.focus != fieldTerm {
		t.Errorf("↓ desde el último campo: foco = %v, want campo de búsqueda", m.focus)
	}
	m = send(m, keys("up")...)
	if m.focus != fieldHidden {
		t.Errorf("↑ desde el primer campo: foco = %v, want ocultas", m.focus)
	}
}

func TestTypingGoesToSearchBox(t *testing.T) {
	m, _ := newTestModel(testLocations...)
	m = send(m, keys("down")...)
	m = send(m, typeText("proy")...)
	if m.focus != fieldTerm || m.input.Value() != "proy" {
		t.Errorf("foco = %v, texto = %q; want campo de búsqueda con \"proy\"", m.focus, m.input.Value())
	}
}

func TestFormErrors(t *testing.T) {
	m, _ := newTestModel(testLocations...)
	m = send(m, keys("enter")...)
	if m.screen != screenForm || m.formErr == "" {
		t.Errorf("Enter sin término: pantalla = %v, error = %q; want error en el formulario", m.screen, m.formErr)
	}

	m = send(m, typeText("[abc")...)
	if m.formErr != "" {
		t.Errorf("escribir debería limpiar el error, queda %q", m.formErr)
	}
	m = send(m, keys("enter")...)
	if !strings.Contains(m.formErr, "patrón inválido") || m.focus != fieldTerm {
		t.Errorf("patrón inválido: error = %q, foco = %v", m.formErr, m.focus)
	}

	// Un periodo no válido se señala en el campo Fecha.
	m, _ = newTestModel(testLocations...)
	m = m.apply(Options{Term: "informe", Modified: "mañana"})
	if m.screen != screenForm || m.focus != fieldDate || !strings.Contains(m.formErr, "periodo no válido") {
		t.Errorf("periodo no válido: pantalla = %v, foco = %v, error = %q", m.screen, m.focus, m.formErr)
	}
}

func TestEscQuits(t *testing.T) {
	m, _ := newTestModel(testLocations...)
	_, cmd := m.Update(key("esc"))
	if cmd == nil {
		t.Fatal("Esc no devolvió ningún comando")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("Esc debería salir del programa")
	}
}

// drain ejecuta cmd y los comandos que genere, como haría el bucle de Bubble
// Tea, hasta que la búsqueda termina.
func drain(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	queue := []tea.Cmd{cmd}
	deadline := time.Now().Add(10 * time.Second)
	for m.searching {
		if time.Now().After(deadline) {
			t.Fatal("la búsqueda no terminó a tiempo")
		}
		if len(queue) == 0 {
			t.Fatal("la búsqueda sigue activa pero no quedan comandos pendientes")
		}
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case resultsMsg:
			next, cmd := m.Update(msg)
			m = next.(model)
			queue = append(queue, cmd)
		}
	}
	return m
}

// runSearch pulsa Enter y espera a que la búsqueda termine.
func runSearch(t *testing.T, m model) model {
	t.Helper()
	next, cmd := m.Update(key("enter"))
	return drain(t, next.(model), cmd)
}

func makeTree(t *testing.T, dirs ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestSearchAndOpenWithArrows(t *testing.T) {
	root := makeTree(t, "docs/Proyecto-A", "src/proyecto-b", "src/otro/proyecto-c", "nada")

	m, rec := newTestModel(location{Label: "Temporal", Path: root})
	m = send(m, typeText("proyecto")...)
	m = runSearch(t, m)

	if m.screen != screenResults {
		t.Fatalf("pantalla = %v, want resultados", m.screen)
	}
	var names []string
	for _, r := range m.results {
		names = append(names, filepath.Base(r.Path))
	}
	slices.Sort(names)
	if want := []string{"Proyecto-A", "proyecto-b", "proyecto-c"}; !slices.Equal(names, want) {
		t.Fatalf("resultados = %v, want %v", names, want)
	}
	if !strings.Contains(m.View().Content, "3 carpetas encontradas") {
		t.Errorf("la vista no muestra el total:\n%s", m.View().Content)
	}

	m = send(m, keys("down", "down", "down")...) // no pasa del último
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2", m.cursor)
	}
	m = send(m, keys("up")...)
	m = send(m, keys("enter")...)
	if want := []string{"explorer:" + m.results[1].Path}; !slices.Equal(rec.calls, want) {
		t.Errorf("Enter: acciones = %v, want %v", rec.calls, want)
	}
	m = send(m, keys("home", "right")...) // → también abre
	if len(rec.calls) != 2 || rec.calls[1] != "explorer:"+m.results[0].Path {
		t.Errorf("→: acciones = %v, want el primer resultado", rec.calls)
	}

	m = send(m, keys("left")...)
	if m.screen != screenForm || m.focus != fieldTerm || m.input.Value() != "proyecto" {
		t.Errorf("← debería volver al formulario conservando la búsqueda (pantalla %v, foco %v, texto %q)",
			m.screen, m.focus, m.input.Value())
	}
}

func TestProjectsWithoutTerm(t *testing.T) {
	root := makeTree(t, "web/src", "notas")
	if err := os.WriteFile(filepath.Join(root, "web", "package.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	m, _ := newTestModel(location{Label: "Temporal", Path: root})
	m = send(m, keys("down", "down", "right")...) // Tipo: Proyectos
	m = runSearch(t, m)

	if len(m.results) != 1 || filepath.Base(m.results[0].Path) != "web" || m.results[0].Project != "Node.js" {
		t.Fatalf("resultados = %+v, want el proyecto web (Node.js)", m.results)
	}
	view := m.View().Content
	for _, want := range []string{"1 proyecto encontrado", "Node.js", "Proyectos"} {
		if !strings.Contains(view, want) {
			t.Errorf("la vista no contiene %q:\n%s", want, view)
		}
	}
}

func TestActionKeys(t *testing.T) {
	m, rec := newTestModel(testLocations...)
	m = withResults(m, `C:\a\uno`, `C:\a\dos`)

	m = send(m, keys("down", "c", "v", "t", "e")...)
	want := []string{`copy:C:\a\dos`, `code:C:\a\dos`, `terminal:C:\a\dos`, `explorer:C:\a\dos`}
	if !slices.Equal(rec.calls, want) {
		t.Errorf("acciones = %v, want %v", rec.calls, want)
	}
	if m.statusErr || !strings.Contains(m.status, "Abierto en el Explorador") {
		t.Errorf("estado = %q", m.status)
	}

	m.acts.code = func(string) error { return errors.New("no está instalado") }
	m = send(m, keys("v")...)
	if !m.statusErr || !strings.Contains(m.status, "No se pudo abrir VS Code: no está instalado") {
		t.Errorf("estado tras un error = %q (error: %v)", m.status, m.statusErr)
	}
}

func TestCDFileWritesSelectionAndQuits(t *testing.T) {
	file := filepath.Join(t.TempDir(), "elegida.txt")
	m, rec := newTestModel(testLocations...)
	m.cdFile = file
	m = withResults(m, `C:\a\uno`, `C:\a\Canción`)

	if view := m.View().Content; !strings.Contains(view, "entrar en la carpeta") {
		t.Errorf("la ayuda debería indicar que Enter entra en la carpeta:\n%s", view)
	}

	m = send(m, keys("down")...)
	_, cmd := m.Update(key("enter"))
	if cmd == nil {
		t.Fatal("Enter no devolvió ningún comando")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("Enter debería salir del programa")
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != `C:\a\Canción` {
		t.Errorf("archivo = %q (%v), want la carpeta elegida", data, err)
	}
	if len(rec.calls) != 0 {
		t.Errorf("no debería abrir el Explorador: %v", rec.calls)
	}
}

func TestDetailsKey(t *testing.T) {
	root := makeTree(t, "datos")
	if err := os.WriteFile(filepath.Join(root, "datos", "a.bin"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	m, _ := newTestModel(testLocations...)
	m = withResults(m, filepath.Join(root, "datos"))

	next, cmd := m.Update(key("d"))
	m = next.(model)
	if !strings.Contains(m.status, "Calculando") || cmd == nil {
		t.Fatalf("d: estado = %q, sin comando = %v", m.status, cmd == nil)
	}
	m = send(m, cmd())
	if !strings.Contains(m.status, "datos: 2.0 KB en 1 archivo · modificada el ") {
		t.Errorf("estado = %q", m.status)
	}

	before := m.status
	m = send(m, detailsMsg{gen: m.detailsGen - 1, path: "vieja", err: errors.New("vieja")})
	if m.status != before {
		t.Errorf("un cálculo anterior no debería cambiar el estado: %q", m.status)
	}
}

func TestApplyOptions(t *testing.T) {
	root := makeTree(t, "docs/Informe-final", "otros")

	// Una carpeta que no está en la lista se añade al principio y se elige.
	m, _ := newTestModel(testLocations...)
	m = m.apply(Options{Root: root, Hidden: true})
	if m.locIndex != 0 || m.locations[0].Path != root || m.locations[0].Label != "Carpeta elegida" || len(m.locations) != 4 {
		t.Errorf("ubicaciones = %+v (índice %d), want la carpeta elegida primero", m.locations, m.locIndex)
	}
	if !m.hidden || m.screen != screenForm {
		t.Errorf("ocultas = %v, pantalla = %v; want ocultas y el formulario (no hay término)", m.hidden, m.screen)
	}

	// Una carpeta que ya está en la lista se elige sin repetirla.
	m, _ = newTestModel(testLocations...)
	m = m.apply(Options{Root: `C:\dos\`})
	if m.locIndex != 1 || len(m.locations) != 3 {
		t.Errorf("índice = %d, ubicaciones = %d; want 1 y 3", m.locIndex, len(m.locations))
	}

	// Con término, la búsqueda empieza al abrir; un periodo que no está en
	// la lista se añade como opción.
	m, _ = newTestModel(location{Label: "Temporal", Path: root})
	m = m.apply(Options{Term: "informe", Modified: "3d"})
	if m.dates[m.dateIndex].value != "3d" {
		t.Errorf("fecha = %+v, want 3d", m.dates[m.dateIndex])
	}
	m = drain(t, m, m.initCmd)
	if len(m.results) != 1 || filepath.Base(m.results[0].Path) != "Informe-final" {
		t.Errorf("resultados = %+v, want Informe-final", m.results)
	}
	if view := m.View().Content; !strings.Contains(view, `Carpetas "informe" modificadas en los últimos 3 días`) {
		t.Errorf("cabecera inesperada:\n%s", view)
	}
	if len(dateOptions) != 5 {
		t.Errorf("las opciones de fecha por defecto no deberían cambiar: %+v", dateOptions)
	}
}

func TestResultsScrolling(t *testing.T) {
	m, _ := newTestModel(testLocations...)
	m = send(m, tea.WindowSizeMsg{Width: 80, Height: 9}) // 2 filas de lista
	m = withResults(m, `C:\a\uno`, `C:\a\dos`, `C:\a\tres`, `C:\a\cuatro`, `C:\a\cinco`)

	m = send(m, keys("down", "down", "down")...)
	if m.cursor != 3 || m.offset != 2 {
		t.Errorf("cursor = %d offset = %d, want 3 y 2", m.cursor, m.offset)
	}
	view := m.View().Content
	if !strings.Contains(view, "tres") || !strings.Contains(view, "cuatro") || strings.Contains(view, "cinco") {
		t.Errorf("la vista debería mostrar solo las filas 3 y 4:\n%s", view)
	}
	m = send(m, keys("end")...)
	if m.cursor != 4 || m.offset != 3 {
		t.Errorf("Fin: cursor = %d offset = %d, want 4 y 3", m.cursor, m.offset)
	}
	m = send(m, keys("pgup")...)
	if m.cursor != 2 || m.offset != 2 {
		t.Errorf("RePág: cursor = %d offset = %d, want 2 y 2", m.cursor, m.offset)
	}
}

func TestStaleResultsAreIgnored(t *testing.T) {
	m, _ := newTestModel(testLocations...)
	m.screen, m.searching, m.gen = screenResults, true, 2
	m = send(m, resultsMsg{gen: 1, results: []search.Result{{Path: `C:\vieja`}}, done: true})
	if len(m.results) != 0 || !m.searching {
		t.Errorf("un lote de otra búsqueda no debería afectar a la actual: %+v", m.results)
	}
}

func TestFormView(t *testing.T) {
	m, _ := newTestModel(testLocations...)
	m = send(m, keys("down", "right")...)
	view := m.View()
	if !view.AltScreen {
		t.Error("la interfaz debería usar la pantalla alternativa")
	}
	for _, want := range []string{"Buscar", "Ubicación", "Dos", `C:\dos`, "Tipo", "Carpetas", "Modificada", "Cualquiera", "Ocultas", "Enter", "buscar"} {
		if !strings.Contains(view.Content, want) {
			t.Errorf("la vista no contiene %q:\n%s", want, view.Content)
		}
	}
}

func TestDefaultLocations(t *testing.T) {
	locs := defaultLocations()
	if len(locs) == 0 {
		t.Fatal("no hay ubicaciones")
	}
	seen := map[string]bool{}
	for _, loc := range locs {
		key := strings.ToLower(strings.TrimRight(loc.Path, `\/`))
		if seen[key] {
			t.Errorf("ubicación repetida: %s", loc.Path)
		}
		seen[key] = true
		if info, err := os.Stat(loc.Path); err != nil || !info.IsDir() {
			t.Errorf("la ubicación %q (%s) no es una carpeta existente", loc.Label, loc.Path)
		}
	}
}
