package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/humanize"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/period"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
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

func (m model) startSearch() (tea.Model, tea.Cmd) {
	// Un término con solo espacios cuenta como vacío: se pide escribir algo.
	term := strings.TrimSpace(m.input.Value())
	q, err := query.New(term, m.projects, m.dates[m.dateIndex].value, time.Now())
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

// Los resultados llegan a la pantalla en lotes: el primero que se encuentra y
// los que lleguen durante batchWindow, hasta un máximo de batchMax.
const (
	batchWindow = 30 * time.Millisecond
	batchMax    = 500
)

// waitForResults espera la siguiente carpeta encontrada y la entrega junto con
// las que lleguen poco después (ver batchWindow), para no redibujar la
// pantalla por cada una.
func waitForResults(gen int, ch <-chan search.Result) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return resultsMsg{gen: gen, done: true}
		}
		batch := []search.Result{r}
		timeout := time.After(batchWindow)
		for len(batch) < batchMax {
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
