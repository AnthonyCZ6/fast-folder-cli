// Package tui implementa el modo interactivo de fast-folder-cli: un formulario
// de búsqueda y una lista de resultados que se manejan con las flechas, sin
// escribir banderas.
package tui

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/explorer"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
)

// Run abre la interfaz interactiva y bloquea hasta que el usuario sale.
func Run(version string) error {
	m := newModel(version, defaultLocations(), explorer.Open)
	final, err := tea.NewProgram(m).Run()
	if fm, ok := final.(model); ok {
		fm.cancelSearch()
	}
	return err
}

type screen int

const (
	screenForm screen = iota
	screenResults
)

// Campos del formulario, en el orden en que se recorren con ↑ y ↓.
type field int

const (
	fieldTerm field = iota
	fieldLocation
	fieldHidden
	fieldCount
)

// Estilos. Se usan los 16 colores básicos para respetar el tema de la terminal
// y funcionar también en la consola clásica de Windows.
var (
	accent      = lipgloss.Color("6")
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(accent).Padding(0, 1)
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleFocus  = lipgloss.NewStyle().Bold(true).Foreground(accent)
	styleName   = lipgloss.NewStyle().Bold(true)
	styleCursor = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(accent)
	styleOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleKey    = lipgloss.NewStyle().Bold(true)
)

// resultsMsg entrega un lote de carpetas encontradas por la búsqueda gen.
type resultsMsg struct {
	gen   int
	paths []string
	done  bool
}

type model struct {
	version       string
	width, height int
	screen        screen
	open          func(string) error // abre una carpeta; reemplazable en pruebas

	// Formulario.
	focus     field
	input     textinput.Model
	locations []location
	locIndex  int
	hidden    bool
	formErr   string

	// Resultados.
	gen       int // identifica la búsqueda en curso para descartar lotes viejos
	pending   <-chan string
	cancel    context.CancelFunc
	stats     *search.Stats
	term      string
	root      string
	results   []string
	cursor    int
	offset    int
	searching bool
	started   time.Time
	elapsed   time.Duration
	spinner   spinner.Model
	ticking   bool
	status    string
	statusErr bool
}

func newModel(version string, locations []location, open func(string) error) model {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "nombre o parte del nombre (ej. proyecto, tesis*)"
	in.CharLimit = 200
	in.Focus()

	return model{
		version:   version,
		width:     80,
		height:    24,
		open:      open,
		input:     in,
		locations: locations,
		spinner:   spinner.New(spinner.WithSpinner(spinner.Line), spinner.WithStyle(styleFocus)),
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.SetWidth(max(m.width-20, 10))
		return m, nil

	case resultsMsg:
		if msg.gen != m.gen {
			return m, nil // lote de una búsqueda anterior ya cancelada
		}
		m.results = append(m.results, msg.paths...)
		if msg.done {
			m.searching = false
			m.elapsed = time.Since(m.started)
			return m, nil
		}
		return m, waitForResults(msg.gen, m.pending)

	case spinner.TickMsg:
		if !m.searching {
			m.ticking = false
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.cancelSearch()
			return m, tea.Quit
		}
		if m.screen == screenForm {
			return m.updateForm(msg)
		}
		return m.updateResults(msg)
	}

	// Mensajes internos del campo de texto (parpadeo del cursor, etc.).
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// ------------------------------------------------------------------ formulario

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

func (m model) startSearch() (tea.Model, tea.Cmd) {
	term := strings.TrimSpace(m.input.Value())
	if term == "" {
		m.formErr = "Escribe el nombre (o parte del nombre) de la carpeta que buscas."
		return m.setFocus(fieldTerm)
	}
	matcher, err := search.NewMatcher(term)
	if err != nil {
		m.formErr = err.Error()
		return m.setFocus(fieldTerm)
	}
	if len(m.locations) == 0 {
		m.formErr = "No hay ubicaciones disponibles para buscar."
		return m, nil
	}

	m.cancelSearch()
	ctx, cancel := context.WithCancel(context.Background())
	loc := m.locations[m.locIndex]
	ch, stats := search.Start(ctx, search.Options{
		Root:          loc.Path,
		Matcher:       matcher,
		IncludeHidden: m.hidden,
	})

	m.gen++
	m.pending = ch
	m.cancel = cancel
	m.stats = stats
	m.term = term
	m.root = loc.Path
	m.results = nil
	m.cursor, m.offset = 0, 0
	m.searching = true
	m.started = time.Now()
	m.status = ""
	m.formErr = ""
	m.screen = screenResults
	m.input.Blur()

	cmds := []tea.Cmd{waitForResults(m.gen, ch)}
	if !m.ticking {
		m.ticking = true
		cmds = append(cmds, m.spinner.Tick)
	}
	return m, tea.Batch(cmds...)
}

func (m *model) cancelSearch() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

// waitForResults espera la siguiente carpeta encontrada y agrupa las que
// lleguen en los 30 ms siguientes, para no redibujar la pantalla por cada una.
func waitForResults(gen int, ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		path, ok := <-ch
		if !ok {
			return resultsMsg{gen: gen, done: true}
		}
		batch := []string{path}
		timeout := time.After(30 * time.Millisecond)
		for len(batch) < 500 {
			select {
			case path, ok := <-ch:
				if !ok {
					return resultsMsg{gen: gen, paths: batch, done: true}
				}
				batch = append(batch, path)
			case <-timeout:
				return resultsMsg{gen: gen, paths: batch}
			}
		}
		return resultsMsg{gen: gen, paths: batch}
	}
}

// ------------------------------------------------------------------ resultados

func (m model) updateResults(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	page := m.listHeight()
	switch msg.String() {
	case "esc", "q":
		m.cancelSearch()
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
		if len(m.results) == 0 {
			return m, nil
		}
		path := m.results[m.cursor]
		if err := m.open(path); err != nil {
			m.status, m.statusErr = "No se pudo abrir el Explorador: "+err.Error(), true
		} else {
			m.status, m.statusErr = "Abierto en el Explorador: "+path, false
		}
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

// listHeight es el número de filas disponibles para la lista de resultados:
// el alto total menos la cabecera (2 líneas) y el pie (4 líneas).
func (m model) listHeight() int {
	return max(m.height-6, 1)
}
