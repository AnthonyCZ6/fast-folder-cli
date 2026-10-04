package tui

import tea "charm.land/bubbletea/v2"

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
		return m.setFocus((m.focus + fieldCount - 1) % fieldCount)
	case "down", "tab":
		return m.setFocus((m.focus + 1) % fieldCount)
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
			m.projects = !m.projects
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

func (m model) setFocus(f field) (tea.Model, tea.Cmd) {
	m.focus = f
	if f == fieldTerm {
		return m, m.input.Focus()
	}
	m.input.Blur()
	return m, nil
}
