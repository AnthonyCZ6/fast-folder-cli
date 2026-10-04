package tui

import (
	"os"

	tea "charm.land/bubbletea/v2"
)

func (m model) updateResults(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	page := m.listHeight()
	switch msg.String() {
	case "esc", "q":
		m.cancelSearch()
		m.cancelDetails()
		return m, tea.Quit
	case "left", "backspace":
		m.cancelSearch()
		m.searching = false
		m.screen = screenForm
		return m.setFocus(fieldTerm)
	case "up":
		m.moveCursor(-1)
	case "down":
		m.moveCursor(1)
	case "pgup":
		m.moveCursor(-page)
	case "pgdown":
		m.moveCursor(page)
	case "home":
		m.moveCursor(-len(m.results))
	case "end":
		m.moveCursor(len(m.results))
	case "enter", "right":
		return m.choose()
	case "e":
		return m.act(m.acts.explorer, "Abierto en el Explorador: ", "No se pudo abrir el Explorador: ")
	case "c":
		return m.act(m.acts.copyPath, "Ruta copiada: ", "No se pudo copiar la ruta: ")
	case "v":
		return m.act(m.acts.code, "Abierto en VS Code: ", "No se pudo abrir VS Code: ")
	case "t":
		return m.act(m.acts.terminal, "Terminal abierta en ", "No se pudo abrir la terminal: ")
	case "d":
		return m.requestDetails()
	}
	return m, nil
}

// selected devuelve la ruta de la carpeta seleccionada, si hay resultados.
func (m model) selected() (string, bool) {
	if len(m.results) == 0 {
		return "", false
	}
	return m.results[m.cursor].Path, true
}

// choose ejecuta la acción principal sobre la carpeta seleccionada: en el modo
// fcd, guardarla y salir para que el script entre en ella; si no, abrirla en
// el Explorador.
func (m model) choose() (tea.Model, tea.Cmd) {
	path, ok := m.selected()
	if !ok {
		return m, nil
	}
	if m.cdFile == "" {
		return m.act(m.acts.explorer, "Abierto en el Explorador: ", "No se pudo abrir el Explorador: ")
	}
	if err := os.WriteFile(m.cdFile, []byte(path), 0o600); err != nil {
		m.status, m.statusErr = "No se pudo guardar la carpeta elegida: "+err.Error(), true
		return m, nil
	}
	m.cancelSearch()
	m.cancelDetails()
	return m, tea.Quit
}

// act ejecuta fn con la carpeta seleccionada y muestra el resultado en la
// línea de estado.
func (m model) act(fn func(string) error, done, failed string) (tea.Model, tea.Cmd) {
	path, ok := m.selected()
	if !ok {
		return m, nil
	}
	if err := fn(path); err != nil {
		m.status, m.statusErr = failed+err.Error(), true
	} else {
		m.status, m.statusErr = done+path, false
	}
	return m, nil
}

func (m *model) moveCursor(delta int) {
	if len(m.results) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+delta, 0), len(m.results)-1)
	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	} else if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
}

// Filas que ocupan la cabecera y el pie de la pantalla de resultados.
const (
	headerLines = 2
	footerLines = 5
)

// listHeight es el número de filas disponibles para la lista de resultados:
// el alto total menos la cabecera y el pie.
func (m model) listHeight() int {
	return max(m.height-headerLines-footerLines, 1)
}
