package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/humanize"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
)

// Nota: solo se usan símbolos presentes en las fuentes de la consola clásica
// de Windows (Consolas, Lucida Console): ► ◄ ↑ ↓ ← → y el spinner |/-\.

func (m model) View() tea.View {
	content := m.viewResults()
	if m.screen == screenForm {
		content = m.viewForm()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.WindowTitle = "fast-folder-cli"
	return v
}

func (m model) viewForm() string {
	var b strings.Builder
	version := m.version
	if version != "" && version[0] >= '0' && version[0] <= '9' {
		version = "v" + version
	}
	b.WriteString(m.line(m.header(styleDim.Render(version))) + "\n\n")

	valueW := m.valueWidth()
	b.WriteString(m.line(m.formRow(fieldTerm, "Buscar", m.input.View(), "")) + "\n")
	if len(m.locations) > 0 {
		loc := m.locations[m.locIndex]
		b.WriteString(m.optionRow(fieldLocation, "Ubicación", loc.Label, loc.Path, valueW) + "\n")
	}
	kind := kindOptions[m.kindIndex]
	if kind.kind == query.Recent && m.with != "" {
		kind.hint = "solo lo que abriste con " + m.with
	}
	b.WriteString(m.optionRow(fieldKind, "Tipo", kind.label, kind.hint, valueW) + "\n")
	date, dateLabel, dateHint := m.dates[m.dateIndex].label, "Modificada", "fecha de modificación de la carpeta"
	if m.kind() == query.Recent {
		dateLabel, dateHint = "Usada", "cuándo se abrió algo en ella por última vez"
	}
	b.WriteString(m.optionRow(fieldDate, dateLabel, date, dateHint, valueW) + "\n")
	hidden := "No"
	if m.hidden {
		hidden = "Sí"
	}
	b.WriteString(m.optionRow(fieldHidden, "Ocultas", hidden, "carpetas ocultas y de sistema", valueW) + "\n")

	b.WriteString("\n")
	if m.formErr != "" {
		b.WriteString(m.line("   "+styleErr.Render(m.formErr)) + "\n\n")
	}
	b.WriteString(m.line(help("↑↓", "moverse", "←→", "cambiar opción", "Enter", "buscar", "Esc", "salir")))
	return b.String()
}

// valueWidth es el ancho de la opción más larga de los campos, para que las
// sugerencias queden alineadas.
func (m model) valueWidth() int {
	w := 0
	for _, k := range kindOptions {
		w = max(w, ansi.StringWidth(k.label))
	}
	for _, loc := range m.locations {
		w = max(w, ansi.StringWidth(loc.Label))
	}
	for _, d := range m.dates {
		w = max(w, ansi.StringWidth(d.label))
	}
	return w
}

// optionRow muestra un campo que se cambia con ←→: su valor y una sugerencia.
// Un campo que no se aplica a lo que se busca (con Apps, todos salvo Buscar y
// Tipo; con Recientes, la ubicación) se muestra atenuado.
func (m model) optionRow(f field, label, value, hint string, valueW int) string {
	if !m.applies(f) {
		what := "las apps"
		if m.kind() == query.Recent {
			what = "las carpetas recientes"
		}
		row := fmt.Sprintf("%-11s", label) + selector(value, false, valueW) + "  no se aplica a " + what
		return m.line("   " + styleDim.Render(row))
	}
	return m.line(m.formRow(f, label, selector(value, m.focus == f, valueW), hint))
}

func (m model) formRow(f field, label, value, hint string) string {
	marker := "  "
	label = fmt.Sprintf("%-11s", label)
	if m.focus == f {
		marker = styleFocus.Render("► ")
		label = styleFocus.Render(label)
	}
	line := " " + marker + label + value
	if hint != "" {
		line += "  " + styleDim.Render(hint)
	}
	return line
}

// selector muestra el valor de una opción; con el foco añade ◄ ► para indicar
// que se cambia con las flechas.
func selector(value string, focused bool, width int) string {
	pad := strings.Repeat(" ", max(width-ansi.StringWidth(value), 0))
	if focused {
		return styleFocus.Render("◄ ") + styleName.Render(value) + pad + styleFocus.Render(" ►")
	}
	return "  " + value + pad + "  "
}

func (m model) viewResults() string {
	var b strings.Builder
	b.WriteString(m.line(m.header(m.resultsTitle())) + "\n\n")

	h := m.listHeight()
	lines := 0
	if len(m.results) == 0 {
		for _, l := range m.emptyLines() {
			b.WriteString(m.line(l) + "\n")
			lines++
		}
	} else {
		nameW := min(max(m.width*2/5, 12), 40)
		end := min(m.offset+h, len(m.results))
		for i := m.offset; i < end; i++ {
			b.WriteString(m.resultLine(i, nameW) + "\n")
			lines++
		}
		// El aviso de que no se guarda el historial, si cabe debajo.
		if notice := m.historyNotice(); len(notice) > 0 && lines+1+len(notice) <= h {
			b.WriteString("\n" + strings.Join(notice, "\n") + "\n")
			lines += 1 + len(notice)
		}
	}
	b.WriteString(strings.Repeat("\n", max(h-lines, 0)))

	b.WriteString("\n" + m.line(m.statsLine()) + "\n")
	status := ""
	if m.status != "" {
		style := styleOK
		if m.statusErr {
			style = styleErr
		}
		status = " " + style.Render(m.status)
	}
	b.WriteString(m.line(status) + "\n")
	if m.cdFile != "" {
		b.WriteString(m.line(help("↑↓", "moverse", "Enter", "entrar en la carpeta", "←", "nueva búsqueda", "Esc", "salir")) + "\n")
		b.WriteString(m.line(help("e", "Explorador", "c", "copiar ruta", "v", "VS Code", "t", "terminal", "d", "tamaño y fecha")))
	} else {
		b.WriteString(m.line(help("↑↓", "moverse", "Enter", "abrir en el Explorador", "←", "nueva búsqueda", "Esc", "salir")) + "\n")
		b.WriteString(m.line(help("c", "copiar ruta", "v", "VS Code", "t", "terminal", "d", "tamaño y fecha")))
	}
	return b.String()
}

// resultsTitle describe en la cabecera qué se busca y dónde.
func (m model) resultsTitle() string {
	desc := capitalize(m.query.Describe())
	if m.recents.label != "" {
		desc += " (" + m.recents.label + ")"
	}
	title := styleName.Render(desc)
	switch m.query.Kind {
	case query.Apps:
		return title + " entre los programas instalados"
	case query.Recent:
		title += " en el historial de Windows"
	default:
		title += " en " + m.root
	}
	if m.hidden {
		title += styleDim.Render(" (con ocultas)")
	}
	return title
}

// emptyLines son las líneas de una lista sin resultados: un aviso mientras se
// busca o, al terminar, qué se puede probar.
func (m model) emptyLines() []string {
	if m.searching {
		return []string{"   " + styleDim.Render("Buscando...")}
	}
	empty := "No se encontró ninguna carpeta con ese nombre."
	hint := "Pulsa ← para cambiar la búsqueda: prueba otra ubicación, otra fecha o incluir las ocultas."
	switch m.query.Kind {
	case query.Projects:
		empty = "No se encontró ningún proyecto."
	case query.Apps:
		empty, hint = "No se encontró ninguna app con ese nombre.", "Pulsa ← para cambiar la búsqueda."
	case query.Recent:
		empty = "No se encontró ninguna carpeta reciente."
		hint = "Pulsa ← para cambiar la búsqueda: prueba otro nombre, otra fecha o incluir las ocultas."
	}
	lines := []string{"   " + styleWarn.Render(empty), "   " + styleDim.Render(hint)}
	if notice := m.historyNotice(); len(notice) > 0 {
		lines = append(append(lines, ""), notice...)
	}
	return lines
}

// minWhereWidth es el ancho que se reserva, como mínimo, para mostrar dónde
// está cada resultado cuando su etiqueta es larga.
const minWhereWidth = 15

func (m model) resultLine(i, nameW int) string {
	name, tag, where := m.resultParts(i)
	name = ansi.Truncate(name, nameW, "...")
	name += strings.Repeat(" ", max(nameW-ansi.StringWidth(name), 0))
	if tag != "" {
		tag = ansi.Truncate(tag, max(m.width-nameW-6-minWhereWidth, 0), "...") + "  "
	}
	where = truncateLeft(where, m.width-nameW-6-ansi.StringWidth(tag))
	if i == m.cursor {
		line := " ► " + name + "  " + tag + where
		return styleCursor.Render(padRight(line, m.width))
	}
	if tag != "" {
		tag = styleOK.Render(tag)
	}
	return m.line("   " + styleName.Render(name) + "  " + tag + styleDim.Render(where))
}

// resultParts devuelve las columnas del resultado i: el nombre, la etiqueta
// (el tipo de un proyecto, o cuándo se usó una carpeta reciente) y dónde
// está. De una app se muestran su nombre y su carpeta.
func (m model) resultParts(i int) (name, tag, where string) {
	if i < len(m.appList) {
		a := m.appList[i]
		return a.Name, "", a.Dir
	}
	r := m.results[i]
	tag = r.Project
	if i < len(m.recents.tags) {
		tag = m.recents.tags[i]
	}
	return filepath.Base(r.Path), tag, filepath.Dir(r.Path)
}

func (m model) statsLine() string {
	found := m.query.Found(int64(len(m.results)))
	var scanned, denied int64
	if m.stats != nil {
		scanned, denied = m.stats.Scanned(), m.stats.Denied()
	}
	details := ""
	switch m.query.Kind {
	case query.Apps: // las apps no se buscan recorriendo carpetas
	case query.Recent: // ni las recientes, que salen del historial
		if !m.searching {
			details = " · " + humanize.Count(int64(m.recents.lists), "lista de Windows", "listas de Windows")
		}
	default:
		details = " · " + humanize.Int(scanned) + " analizadas"
	}
	if denied > 0 {
		details += " · " + humanize.Int(denied) + " sin acceso"
	}
	if len(m.results) > 0 {
		details += fmt.Sprintf(" · %d/%d", m.cursor+1, len(m.results))
	}

	if m.searching {
		return " " + m.spinner.View() + " Buscando... " + found + styleDim.Render(details)
	}
	details += " · " + humanize.Int(m.elapsed.Milliseconds()) + " ms"
	return " " + styleOK.Render(found) + styleDim.Render(details)
}

func (m model) header(info string) string {
	return " " + styleTitle.Render("fast-folder-cli") + "  " + info
}

// line recorta una línea al ancho de la terminal para que nunca salte de línea.
func (m model) line(s string) string {
	return ansi.Truncate(s, m.width, "")
}

// help compone la línea de ayuda a partir de pares tecla/descripción.
func help(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, styleKey.Render(pairs[i])+" "+styleDim.Render(pairs[i+1]))
	}
	return " " + strings.Join(parts, styleDim.Render("   "))
}

// truncateLeft acorta s por la izquierda para que quepa en width columnas,
// conservando el final de la ruta, que es la parte más informativa.
func truncateLeft(s string, width int) string {
	if width <= 3 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w <= width {
		return s
	}
	return ansi.TruncateLeft(s, w-width+3, "...")
}

func padRight(s string, width int) string {
	return s + strings.Repeat(" ", max(width-ansi.StringWidth(s), 0))
}

// capitalize pone en mayúscula la primera letra de s.
func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}
