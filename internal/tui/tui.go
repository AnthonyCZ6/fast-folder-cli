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

	"github.com/AnthonyCZ6/fast-folder-cli/internal/launch"
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

// maxTermLength es la longitud máxima del término en el formulario.
const maxTermLength = 200

func newModel(version string, locations []location, acts actions) model {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "nombre o parte del nombre (ej. proyecto, tesis*)"
	in.CharLimit = maxTermLength
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
