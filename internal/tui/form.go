package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
)

// Campos del formulario, en el orden en que se recorren con ↑ y ↓.
type field int

const (
	fieldTerm field = iota
	fieldLocation
	fieldKind
	fieldDate
	fieldHidden
	fieldCount
)

// kindOption es una opción del campo Tipo.
type kindOption struct {
	label string
	hint  string // qué encuentra, junto al valor
	kind  query.Kind
}

var kindOptions = []kindOption{
	{"Carpetas", "cualquier carpeta cuyo nombre coincida", query.Folders},
	{"Proyectos", "carpetas con .git, package.json, go.mod... (el nombre es opcional)", query.Projects},
	{"Apps", "aplicaciones instaladas: Enter abre su ubicación (el nombre es opcional)", query.Apps},
	{"Recientes", "carpetas de lo que abriste hace poco (el nombre es opcional)", query.Recent},
}

// dateOption es una opción del campo Fecha.
type dateOption struct {
	label string
	value string // como en --modified; vacío: cualquier fecha
}

var dateOptions = []dateOption{
	{"Cualquiera", ""},
	{"Hoy", "hoy"},
	{"Ayer", "ayer"},
	{"Últimos 7 días", "semana"},
	{"Últimos 30 días", "mes"},
}

func (m model) updateForm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, tea.Quit
	case "enter":
		return m.startSearch()
	case "up", "shift+tab":
		return m.setFocus(m.nextField(-1))
	case "down", "tab":
		return m.setFocus(m.nextField(1))
	case "left", "right":
		step := 1
		if msg.String() == "left" {
			step = -1
		}
		switch m.focus {
		case fieldLocation:
			if n := len(m.locations); n > 0 {
				m.locIndex = (m.locIndex + step + n) % n
			}
			return m, nil
		case fieldKind:
			n := len(kindOptions)
			m.kindIndex = (m.kindIndex + step + n) % n
			return m, nil
		case fieldDate:
			n := len(m.dates)
			m.dateIndex = (m.dateIndex + step + n) % n
			return m, nil
		case fieldHidden:
			m.hidden = !m.hidden
			return m, nil
		}
	}

	// Escribir en cualquier campo lleva el texto al campo de búsqueda.
	var focusCmd tea.Cmd
	if m.focus != fieldTerm && msg.Text != "" {
		m.focus = fieldTerm
		focusCmd = m.input.Focus()
	}
	if m.focus != fieldTerm {
		return m, nil
	}
	m.formErr = ""
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, tea.Batch(focusCmd, cmd)
}

// kind devuelve lo que se busca según el campo Tipo.
func (m model) kind() query.Kind {
	return kindOptions[m.kindIndex].kind
}

// applies indica si el campo f se aplica a lo que se busca: las apps no se
// buscan en una ubicación, ni por fecha, ni entre carpetas ocultas, y las
// carpetas recientes se buscan en todo el historial, no en una ubicación.
func (m model) applies(f field) bool {
	switch m.kind() {
	case query.Apps:
		return f == fieldTerm || f == fieldKind
	case query.Recent:
		return f != fieldLocation
	}
	return true
}

// nextField devuelve el campo al que lleva ↓ (step 1) o ↑ (step -1) desde el
// actual, saltando los que no se aplican.
func (m model) nextField(step int) field {
	f := m.focus
	for {
		f = (f + field(step) + fieldCount) % fieldCount
		if m.applies(f) {
			return f
		}
	}
}

func (m model) setFocus(f field) (tea.Model, tea.Cmd) {
	m.focus = f
	if f == fieldTerm {
		return m, m.input.Focus()
	}
	m.input.Blur()
	return m, nil
}
