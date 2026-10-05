package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/recent"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
)

// recentView es lo que se muestra de una búsqueda de carpetas recientes,
// además de las rutas.
type recentView struct {
	tags  []string // de cada resultado: cuándo se usó y con qué programas
	label string   // los programas de --con, para la cabecera
	off   string   // por qué Windows no guarda el historial; "" si lo guarda
	lists int      // jump lists leídas
}

// recentMsg trae las carpetas recientes que encontró la búsqueda gen.
type recentMsg struct {
	gen   int
	paths []string
	info  recentView
	err   error
}

// recentOptions devuelve qué carpetas recientes busca q con las opciones del
// formulario. La ubicación no se aplica: se busca en todo el historial.
func (m model) recentOptions(q query.Query) recent.Options {
	return recent.Options{
		Match:         q.Match,
		After:         q.Period.After,
		Before:        q.Period.Before,
		Exclude:       m.exclude,
		IncludeHidden: m.hidden,
	}
}

// findRecent lee en segundo plano el historial de Windows con load y busca
// en él las carpetas que cumplen opts. Si with no está vacío (fcd --con),
// solo las de lo que se abrió con ese programa.
func findRecent(gen int, load func() (recent.History, error), opts recent.Options, with string) tea.Cmd {
	return func() tea.Msg {
		h, err := load()
		if err != nil {
			return recentMsg{gen: gen, err: err}
		}
		info := recentView{off: h.Off, lists: len(h.Lists)}
		if with != "" {
			ids, label, err := h.AppsNamed(with)
			if err != nil {
				return recentMsg{gen: gen, info: info, err: err}
			}
			opts.AppIDs, info.label = ids, label
		}
		folders := recent.Find(h.Lists, opts)
		paths := make([]string, len(folders))
		info.tags = make([]string, len(folders))
		now := time.Now()
		for i, f := range folders {
			paths[i] = f.Path
			info.tags[i] = f.Summary(h.AppsOf(f), now)
		}
		return recentMsg{gen: gen, paths: paths, info: info}
	}
}

// showRecent muestra las carpetas recientes encontradas. Son resultados
// normales: Enter, c, v, t, d y fcd funcionan igual que con cualquier
// carpeta.
func (m *model) showRecent(msg recentMsg) {
	m.searching = false
	m.elapsed = time.Since(m.started)
	m.recents = msg.info
	if msg.err != nil {
		m.status, m.statusErr = capitalize(msg.err.Error()), true
		return
	}
	m.results = make([]search.Result, len(msg.paths))
	for i, p := range msg.paths {
		m.results[i] = search.Result{Path: p}
	}
}

// historyNotice son las líneas del aviso de que Windows no guarda el
// historial (ninguna si lo guarda), ajustadas al ancho de la pantalla.
func (m model) historyNotice() []string {
	if m.query.Kind != query.Recent || m.recents.off == "" {
		return nil
	}
	text := "Windows no está guardando las carpetas recientes: " + m.recents.off +
		". Solo aparece lo que se guardó antes de desactivarlo."
	var lines []string
	for _, l := range strings.Split(ansi.Wordwrap(text, max(m.width-4, 20), ""), "\n") {
		lines = append(lines, "   "+styleWarn.Render(l))
	}
	return lines
}
