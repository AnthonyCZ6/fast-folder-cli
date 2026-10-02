package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/humanize"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
)

// Secuencias ANSI de color.
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
)

// summary reúne los datos que se muestran al terminar la búsqueda.
type summary struct {
	found       int64
	scanned     int64
	denied      int64
	opened      string
	openErr     error
	interrupted bool
	elapsed     time.Duration
	projects    bool  // se buscaban proyectos en lugar de carpetas
	sized       bool  // se calculó el tamaño de los resultados (--size)
	bytes       int64 // tamaño total de los resultados
	files       int64 // archivos en total dentro de los resultados
}

// printer da formato a la salida en consola. Con color desactivado (salida
// redirigida a un archivo o NO_COLOR definido) tampoco usa caracteres de
// dibujo de cajas, para que el texto sea fácil de procesar con otras
// herramientas.
type printer struct {
	w     io.Writer
	color bool
}

func newPrinter(w io.Writer, color bool) *printer {
	return &printer{w: w, color: color}
}

func (p *printer) paint(s string, codes ...string) string {
	if !p.color || s == "" {
		return s
	}
	return strings.Join(codes, "") + s + ansiReset
}

// header imprime qué se busca (desc, por ejemplo `"tesis"` o `proyectos`) y
// dónde.
func (p *printer) header(desc, root string, all bool) {
	hidden := "excluidas"
	if all {
		hidden = "incluidas"
	}
	fmt.Fprintf(p.w, "%s %s en %s %s\n\n",
		p.paint("Buscando", ansiBold, ansiCyan),
		p.paint(desc, ansiBold),
		root,
		p.paint("(ocultas/sistema: "+hidden+")", ansiDim),
	)
}

// match imprime una coincidencia: el índice, la ruta de la carpeta padre, el
// nombre de la carpeta encontrada resaltado y, en el modo proyectos, su tipo.
func (p *printer) match(n int64, r search.Result) {
	fmt.Fprintf(p.w, "  %s %s%s\n",
		p.paint("["+strconv.FormatInt(n, 10)+"]", ansiGreen),
		p.path(r.Path),
		p.project(r.Project),
	)
}

// sizes imprime las carpetas con su tamaño, ya ordenadas, y suma el total.
func (p *printer) sizes(list []sized, s *summary) {
	s.sized = true
	for _, item := range list {
		fmt.Fprintf(p.w, "  %s  %s%s\n",
			p.paint(fmt.Sprintf("%10s", humanize.Bytes(item.Bytes)), ansiBold, ansiCyan),
			p.path(item.Path),
			p.project(item.Project),
		)
		s.bytes += item.Bytes
		s.files += item.Files
	}
}

// path devuelve la ruta con el nombre de la carpeta resaltado.
func (p *printer) path(path string) string {
	parent, name := filepath.Split(path)
	return parent + p.paint(name, ansiBold, ansiGreen)
}

// project devuelve el tipo de proyecto entre paréntesis, o "" si no lo hay.
func (p *printer) project(kind string) string {
	if kind == "" {
		return ""
	}
	return "  " + p.paint("("+kind+")", ansiCyan)
}

func (p *printer) summary(s summary) {
	rule := strings.Repeat("-", 64)
	if p.color {
		rule = strings.Repeat("─", 64)
	}
	if s.found > 0 {
		fmt.Fprintln(p.w)
	}
	fmt.Fprintln(p.w, p.paint(rule, ansiDim))

	one, many := "carpeta encontrada", "carpetas encontradas"
	if s.projects {
		one, many = "proyecto encontrado", "proyectos encontrados"
	}
	if s.found == 0 {
		p.field("Resultados", p.paint("sin coincidencias", ansiYellow))
	} else {
		p.field("Resultados", p.paint(humanize.Count(s.found, one, many), ansiBold, ansiGreen))
	}
	if s.sized {
		total := humanize.Bytes(s.bytes) + " en total"
		p.field("Tamaño", p.paint(total, ansiBold)+" ("+humanize.Count(s.files, "archivo", "archivos")+")")
	}

	scanned := humanize.Count(s.scanned, "carpeta", "carpetas")
	if s.denied > 0 {
		scanned += p.paint(" ("+humanize.Int(s.denied)+" sin acceso, omitidas)", ansiDim)
	}
	p.field("Analizadas", scanned)

	if s.opened != "" && s.openErr == nil {
		p.field("Explorador", s.opened)
	}
	if s.interrupted {
		p.field("Estado", p.paint("búsqueda interrumpida (resultados parciales)", ansiYellow))
	}
	p.field("Tiempo", p.paint(humanize.Int(s.elapsed.Milliseconds())+" ms", ansiBold, ansiCyan))
}

func (p *printer) field(label, value string) {
	fmt.Fprintf(p.w, "%s %s\n", p.paint(fmt.Sprintf("%-11s:", label), ansiDim), value)
}

// supportsColor indica si w es una consola capaz de mostrar colores ANSI.
// Respeta la convención NO_COLOR (https://no-color.org).
func supportsColor(w io.Writer) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	f, ok := w.(*os.File)
	return ok && enableANSI(f)
}
