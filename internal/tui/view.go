package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/humanize"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
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

	valueW := ansi.StringWidth("Proyectos")
	for _, loc := range m.locations {
		valueW = max(valueW, ansi.StringWidth(loc.Label))
	}
	for _, d := range m.dates {
		valueW = max(valueW, ansi.StringWidth(d.label))
	}

	b.WriteString(m.line(m.formRow(fieldTerm, "Buscar", m.input.View(), "")) + "\n")
	if len(m.locations) > 0 {
		loc := m.locations[m.locIndex]
		value := selector(loc.Label, m.focus == fieldLocation, valueW)
		b.WriteString(m.line(m.formRow(fieldLocation, "Ubicación", value, loc.Path)) + "\n")
	}

	kind, kindHint := "Carpetas", "cualquier carpeta cuyo nombre coincida"
	if m.projects {
		kind, kindHint = "Proyectos", "carpetas con .git, package.json, go.mod... (el nombre es opcional)"
	}
	value := selector(kind, m.focus == fieldKind, valueW)
	b.WriteString(m.line(m.formRow(fieldKind, "Tipo", value, kindHint)) + "\n")

	value = selector(m.dates[m.dateIndex].label, m.focus == fieldDate, valueW)
	b.WriteString(m.line(m.formRow(fieldDate, "Modificada", value, "fecha de modificación de la carpeta")) + "\n")

	hidden := "No"
	if m.hidden {
		hidden = "Sí"
	}
	value = selector(hidden, m.focus == fieldHidden, valueW)
	b.WriteString(m.line(m.formRow(fieldHidden, "Ocultas", value, "carpetas ocultas y de sistema")) + "\n")

	b.WriteString("\n")
	if m.formErr != "" {
		b.WriteString(m.line("   "+styleErr.Render(m.formErr)) + "\n\n")
	}
	b.WriteString(m.line(help("↑↓", "moverse", "←→", "cambiar opción", "Enter", "buscar", "Esc", "salir")))
	return b.String()
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
	where := styleName.Render(m.desc) + " en " + m.root
	if m.hidden {
		where += styleDim.Render(" (con ocultas)")
	}
	b.WriteString(m.line(m.header(where)) + "\n\n")

	h := m.listHeight()
	lines := 0
	if len(m.results) == 0 {
		if m.searching {
			b.WriteString(m.line("   "+styleDim.Render("Buscando...")) + "\n")
			lines = 1
		} else {
			empty := "No se encontró ninguna carpeta con ese nombre."
			if m.searchProjects {
				empty = "No se encontró ningún proyecto."
			}
			b.WriteString(m.line("   "+styleWarn.Render(empty)) + "\n")
			b.WriteString(m.line("   "+styleDim.Render("Pulsa ← para cambiar la búsqueda: prueba otra ubicación, otra fecha o incluir las ocultas.")) + "\n")
			lines = 2
		}
	} else {
		nameW := min(max(m.width*2/5, 12), 40)
		end := min(m.offset+h, len(m.results))
		for i := m.offset; i < end; i++ {
			b.WriteString(m.resultLine(m.results[i], i == m.cursor, nameW) + "\n")
			lines++
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

func (m model) resultLine(r search.Result, selected bool, nameW int) string {
	name := ansi.Truncate(filepath.Base(r.Path), nameW, "...")
	name += strings.Repeat(" ", max(nameW-ansi.StringWidth(name), 0))
	kind := ""
	if r.Project != "" {
		kind = r.Project + "  "
	}
	parent := truncateLeft(filepath.Dir(r.Path), m.width-nameW-6-ansi.StringWidth(kind))
	if selected {
		line := " ► " + name + "  " + kind + parent
		return styleCursor.Render(padRight(line, m.width))
	}
	if kind != "" {
		kind = styleOK.Render(kind)
	}
	return m.line("   " + styleName.Render(name) + "  " + kind + styleDim.Render(parent))
}

func (m model) statsLine() string {
	one, many := "carpeta encontrada", "carpetas encontradas"
	if m.searchProjects {
		one, many = "proyecto encontrado", "proyectos encontrados"
	}
	found := humanize.Count(int64(len(m.results)), one, many)
	var scanned, denied int64
	if m.stats != nil {
		scanned, denied = m.stats.Scanned(), m.stats.Denied()
	}
	details := " · " + humanize.Int(scanned) + " analizadas"
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
