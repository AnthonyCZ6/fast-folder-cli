package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
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

func newTestModel(locs ...location) (model, *[]string) {
	opened := &[]string{}
	m := newModel("test", locs, func(p string) error {
		*opened = append(*opened, p)
		return nil
	})
	m = send(m, tea.WindowSizeMsg{Width: 100, Height: 20})
	return m, opened
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
	if !strings.Contains(m.formErr, "patrón inválido") {
		t.Errorf("patrón inválido: error = %q", m.formErr)
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

// runSearch pulsa Enter y ejecuta los comandos resultantes hasta que la
// búsqueda termina, como haría el bucle de Bubble Tea.
func runSearch(t *testing.T, m model) model {
	t.Helper()
	next, cmd := m.Update(key("enter"))
	m = next.(model)
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

func TestSearchAndOpenWithArrows(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"docs/Proyecto-A", "src/proyecto-b", "src/otro/proyecto-c", "nada"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	m, opened := newTestModel(location{Label: "Temporal", Path: root})
	m = send(m, typeText("proyecto")...)
	m = runSearch(t, m)

	if m.screen != screenResults {
		t.Fatalf("pantalla = %v, want resultados", m.screen)
	}
	var names []string
	for _, p := range m.results {
		names = append(names, filepath.Base(p))
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
	if len(*opened) != 1 || (*opened)[0] != m.results[1] {
		t.Errorf("Enter abrió %v, want [%s]", *opened, m.results[1])
	}
	m = send(m, keys("home", "right")...) // → también abre
	if len(*opened) != 2 || (*opened)[1] != m.results[0] {
		t.Errorf("→ abrió %v, want el primer resultado", *opened)
	}

	m = send(m, keys("left")...)
	if m.screen != screenForm || m.focus != fieldTerm || m.input.Value() != "proyecto" {
		t.Errorf("← debería volver al formulario conservando la búsqueda (pantalla %v, foco %v, texto %q)",
			m.screen, m.focus, m.input.Value())
	}
}

func TestResultsScrolling(t *testing.T) {
	m, _ := newTestModel(testLocations...)
	m = send(m, tea.WindowSizeMsg{Width: 80, Height: 8}) // 2 filas de lista
	m.screen, m.searching, m.gen = screenResults, true, 1
	paths := []string{`C:\a\uno`, `C:\a\dos`, `C:\a\tres`, `C:\a\cuatro`, `C:\a\cinco`}
	m = send(m, resultsMsg{gen: 1, paths: paths, done: true})

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
	m = send(m, resultsMsg{gen: 1, paths: []string{`C:\vieja`}, done: true})
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
	for _, want := range []string{"Buscar", "Ubicación", "Dos", `C:\dos`, "Ocultas", "Enter", "buscar"} {
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
