// Package tui implementa el modo interactivo de fast-folder-cli: un formulario
// de búsqueda y una lista de resultados que se manejan con las flechas, sin
// escribir banderas.
package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/humanize"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/launch"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/period"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
)

// Options configura el estado inicial del modo interactivo.
type Options struct {
	Term     string // búsqueda inicial
	Root     string // ubicación inicial (ruta ya resuelta); vacía: la primera
	Hidden   bool   // incluir las carpetas ocultas y de sistema
	Projects bool   // buscar proyectos en lugar de carpetas
	Modified string // periodo de modificación, como en --modified
	CDFile   string // si no está vacío, Enter escribe aquí la carpeta elegida y sale (fcd)
}

// Run abre la interfaz interactiva y bloquea hasta que el usuario sale. Si
// opts incluye un término, proyectos o una fecha, la búsqueda empieza al abrir.
func Run(version string, opts Options) error {
	m := newModel(version, defaultLocations(), defaultActions())
	m = m.apply(opts)
	final, err := tea.NewProgram(m).Run()
	if fm, ok := final.(model); ok {
		fm.cancelSearch()
		fm.cancelDetails()
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

// actions agrupa lo que se puede hacer con una carpeta de los resultados. En
// las pruebas se reemplazan por funciones que solo registran la llamada.
type actions struct {
	explorer func(string) error
	code     func(string) error
	terminal func(string) error
	copyPath func(string) error
}

func defaultActions() actions {
	return actions{
		explorer: launch.Explorer,
		code:     launch.VSCode,
		terminal: launch.Terminal,
		copyPath: launch.CopyPath,
	}
}

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
	gen     int
	results []search.Result
	done    bool
}

// detailsMsg trae el tamaño y la fecha de una carpeta, calculados en segundo
// plano al pulsar d.
type detailsMsg struct {
	gen  int
	path string
	info search.SizeInfo
	mod  time.Time
	err  error
}

type model struct {
	version       string
	width, height int
	screen        screen
	acts          actions
	cdFile        string  // modo fcd: archivo donde se escribe la carpeta elegida
	initCmd       tea.Cmd // comando de la búsqueda iniciada al abrir

	// Formulario.
	focus     field
	input     textinput.Model
	locations []location
	locIndex  int
	projects  bool
	dates     []dateOption
	dateIndex int
	hidden    bool
	formErr   string

	// Resultados.
	gen       int // identifica la búsqueda en curso para descartar lotes viejos
	pending   <-chan search.Result
	cancel    context.CancelFunc
	stats     *search.Stats
	query     query.Query // lo que se busca, para la cabecera y el recuento
	root      string
	results   []search.Result
	cursor    int
	offset    int
	searching bool
	started   time.Time
	elapsed   time.Duration
	spinner   spinner.Model
	ticking   bool
	status    string
	statusErr bool

	// Tamaño y fecha de la carpeta seleccionada (tecla d).
	detailsGen    int
	detailsCancel context.CancelFunc
}

func newModel(version string, locations []location, acts actions) model {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "nombre o parte del nombre (ej. proyecto, tesis*)"
	in.CharLimit = 200
	in.Focus()

	return model{
		version:   version,
		width:     80,
		height:    24,
		acts:      acts,
		input:     in,
		locations: locations,
		dates:     dateOptions,
		spinner:   spinner.New(spinner.WithSpinner(spinner.Line), spinner.WithStyle(styleFocus)),
	}
}

// apply aplica las opciones iniciales y, si hay algo que buscar, empieza la
// búsqueda.
func (m model) apply(opts Options) model {
	m.input.SetValue(opts.Term)
	m.input.CursorEnd()
	m.hidden = opts.Hidden
	m.projects = opts.Projects
	m.cdFile = opts.CDFile

	if opts.Root != "" {
		m.locIndex = -1
		for i, loc := range m.locations {
			if samePath(loc.Path, opts.Root) {
				m.locIndex = i
				break
			}
		}
		if m.locIndex < 0 {
			chosen := location{Label: "Carpeta elegida", Path: opts.Root}
			m.locations = append([]location{chosen}, m.locations...)
			m.locIndex = 0
		}
	}

	if opts.Modified != "" {
		m.dateIndex = -1
		for i, d := range m.dates {
			if d.value == strings.ToLower(opts.Modified) {
				m.dateIndex = i
				break
			}
		}
		if m.dateIndex < 0 {
			m.dates = append(append([]dateOption(nil), m.dates...), dateOption{opts.Modified, opts.Modified})
			m.dateIndex = len(m.dates) - 1
		}
	}

	if opts.Term != "" || opts.Projects || opts.Modified != "" {
		next, cmd := m.startSearch()
		m = next.(model)
		m.initCmd = cmd
	}
	return m
}

func samePath(a, b string) bool {
	return strings.EqualFold(strings.TrimRight(a, `\/`), strings.TrimRight(b, `\/`))
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.initCmd)
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
		m.results = append(m.results, msg.results...)
		if msg.done {
			m.searching = false
			m.elapsed = time.Since(m.started)
			return m, nil
		}
		return m, waitForResults(msg.gen, m.pending)

	case detailsMsg:
		if msg.gen != m.detailsGen {
			return m, nil
		}
		m.showDetails(msg)
		return m, nil

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
			m.cancelDetails()
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

func (m model) startSearch() (tea.Model, tea.Cmd) {
	q, err := query.New(m.input.Value(), m.projects, m.dates[m.dateIndex].value, time.Now())
	switch {
	case errors.Is(err, query.ErrEmpty):
		m.formErr = "Escribe el nombre (o parte del nombre) de la carpeta que buscas, o elige Proyectos o una fecha."
		return m.setFocus(fieldTerm)
	case errors.Is(err, period.ErrInvalid):
		m.formErr = err.Error()
		return m.setFocus(fieldDate)
	case err != nil:
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
	ch, stats := search.Start(ctx, q.Options(loc.Path, m.hidden))

	m.gen++
	m.pending = ch
	m.cancel = cancel
	m.stats = stats
	m.query = q
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

func (m *model) cancelDetails() {
	if m.detailsCancel != nil {
		m.detailsCancel()
		m.detailsCancel = nil
	}
}

// waitForResults espera la siguiente carpeta encontrada y agrupa las que
// lleguen en los 30 ms siguientes, para no redibujar la pantalla por cada una.
func waitForResults(gen int, ch <-chan search.Result) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return resultsMsg{gen: gen, done: true}
		}
		batch := []search.Result{r}
		timeout := time.After(30 * time.Millisecond)
		for len(batch) < 500 {
			select {
			case r, ok := <-ch:
				if !ok {
					return resultsMsg{gen: gen, results: batch, done: true}
				}
				batch = append(batch, r)
			case <-timeout:
				return resultsMsg{gen: gen, results: batch}
			}
		}
		return resultsMsg{gen: gen, results: batch}
	}
}

// ------------------------------------------------------------------ resultados

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

// requestDetails empieza a calcular en segundo plano el tamaño de la carpeta
// seleccionada. Si había otro cálculo en curso, se cancela.
func (m model) requestDetails() (tea.Model, tea.Cmd) {
	path, ok := m.selected()
	if !ok {
		return m, nil
	}
	m.cancelDetails()
	ctx, cancel := context.WithCancel(context.Background())
	m.detailsCancel = cancel
	m.detailsGen++
	m.status, m.statusErr = "Calculando el tamaño de "+filepath.Base(path)+"...", false

	gen := m.detailsGen
	return m, func() tea.Msg {
		info, err := os.Stat(path)
		if err != nil {
			return detailsMsg{gen: gen, path: path, err: err}
		}
		return detailsMsg{gen: gen, path: path, info: search.Size(ctx, path), mod: info.ModTime()}
	}
}

func (m *model) showDetails(msg detailsMsg) {
	name := filepath.Base(msg.path)
	if msg.err != nil {
		m.status, m.statusErr = "No se pudo leer "+name+": "+msg.err.Error(), true
		return
	}
	status := fmt.Sprintf("%s: %s en %s · modificada el %s", name,
		humanize.Bytes(msg.info.Bytes),
		humanize.Count(msg.info.Files, "archivo", "archivos"),
		msg.mod.Format("02/01/2006 15:04"))
	if msg.info.Denied > 0 {
		status += " · " + humanize.Int(msg.info.Denied) + " carpetas sin acceso"
	}
	m.status, m.statusErr = status, false
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
// el alto total menos la cabecera (2 líneas) y el pie (5 líneas).
func (m model) listHeight() int {
	return max(m.height-7, 1)
}
