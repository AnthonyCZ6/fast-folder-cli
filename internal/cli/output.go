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

func (p *printer) header(term, root string, all bool) {
	hidden := "excluidas"
	if all {
		hidden = "incluidas"
	}
	fmt.Fprintf(p.w, "%s %s en %s %s\n\n",
		p.paint("Buscando", ansiBold, ansiCyan),
		p.paint(`"`+term+`"`, ansiBold),
		root,
		p.paint("(ocultas/sistema: "+hidden+")", ansiDim),
	)
}

// match imprime una coincidencia: el índice, la ruta de la carpeta padre y el
// nombre de la carpeta encontrada resaltado.
func (p *printer) match(n int64, path string) {
	parent, name := filepath.Split(path)
	fmt.Fprintf(p.w, "  %s %s%s\n",
		p.paint("["+strconv.FormatInt(n, 10)+"]", ansiGreen),
		parent,
		p.paint(name, ansiBold, ansiGreen),
	)
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

	if s.found == 0 {
		p.field("Resultados", p.paint("sin coincidencias", ansiYellow))
	} else {
		p.field("Resultados", p.paint(humanize.Count(s.found, "carpeta encontrada", "carpetas encontradas"), ansiBold, ansiGreen))
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
