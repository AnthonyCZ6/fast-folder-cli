// Package cli implementa la interfaz de línea de comandos de fast-folder-cli.
package cli

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/launch"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/pathutil"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/period"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/tui"
)

// Códigos de salida (compatibles con la convención de grep).
const (
	exitFound       = 0   // se encontró al menos una carpeta
	exitNoMatch     = 1   // la búsqueda terminó sin coincidencias
	exitUsage       = 2   // argumentos o ruta inválidos
	exitInterrupted = 130 // cancelada con Ctrl+C
)

const defaultRoot = "%USERPROFILE%"

// config contiene las opciones ya interpretadas de la línea de comandos.
type config struct {
	term     string
	root     string
	all      bool
	open     bool
	version  bool
	modified string
	projects bool
	size     bool
	cdFile   string
}

// Run ejecuta la CLI con los argumentos indicados (sin el nombre del programa)
// y devuelve el código de salida del proceso.
func Run(args []string, stdout, stderr io.Writer, version string) int {
	start := time.Now()

	// Sin argumentos y en una terminal se abre el modo interactivo, que se
	// maneja con las flechas. Si la salida está redirigida se muestra la ayuda.
	if len(args) == 0 && isInteractive(stdout) {
		return runInteractive(config{root: defaultRoot}, stdout, stderr, version)
	}

	cfg, err := parseArgs(args)
	switch {
	case errors.Is(err, flag.ErrHelp):
		printUsage(stdout)
		return exitFound
	case err != nil:
		return usageError(stderr, err)
	case cfg.version:
		fmt.Fprintf(stdout, "fast-folder-cli %s\n", version)
		return exitFound
	}

	// Sin término (ni proyectos ni fecha) no hay nada que buscar: en una
	// terminal se abre el modo interactivo con las opciones indicadas (así
	// funciona "fast -p D:" y el menú contextual del Explorador). --cd-file,
	// que usan los scripts fcd, también lo abre siempre.
	needsTerm := cfg.term == "" && !cfg.projects && cfg.modified == ""
	if cfg.cdFile != "" || (needsTerm && isInteractive(stdout)) {
		return runInteractive(cfg, stdout, stderr, version)
	}
	if needsTerm {
		printUsage(stderr)
		return exitUsage
	}

	var matcher *search.Matcher
	if cfg.term != "" {
		if matcher, err = search.NewMatcher(cfg.term); err != nil {
			return usageError(stderr, err)
		}
	}
	root, err := pathutil.Resolve(cfg.root)
	if err != nil {
		return usageError(stderr, err)
	}
	var when period.Range
	if cfg.modified != "" {
		if when, err = period.Parse(cfg.modified, time.Now()); err != nil {
			return usageError(stderr, err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	results, stats := search.Start(ctx, search.Options{
		Root:           root,
		Matcher:        matcher,
		IncludeHidden:  cfg.all,
		Projects:       cfg.projects,
		ModifiedAfter:  when.After,
		ModifiedBefore: when.Before,
		Prune:          cfg.size,
	})

	// La salida se agrupa en un búfer que se vacía cada vez que no hay más
	// resultados inmediatamente disponibles: los resultados aparecen en
	// cuanto se encuentran, pero miles de coincidencias seguidas no se
	// escriben línea a línea en la consola (que es lenta).
	out := bufio.NewWriter(stdout)
	p := newPrinter(out, supportsColor(stdout))
	p.header(describe(cfg, when), root, cfg.all)

	next := func() (search.Result, bool) {
		select {
		case r, ok := <-results:
			return r, ok
		default:
			out.Flush()
			r, ok := <-results
			return r, ok
		}
	}

	sum := summary{projects: cfg.projects}
	var found []search.Result
	for r, ok := next(); ok; r, ok = next() {
		sum.found++
		if cfg.size {
			// Con --size se muestran al final, ordenadas por tamaño.
			found = append(found, r)
		} else {
			p.match(sum.found, r)
		}

		if cfg.open && sum.found == 1 {
			// Se abre en cuanto aparece, sin esperar al final del recorrido.
			sum.opened = r.Path
			sum.openErr = launch.Explorer(r.Path)
		}
	}

	if cfg.size && len(found) > 0 && ctx.Err() == nil {
		p.sizes(measure(ctx, found), &sum)
	}

	sum.scanned = stats.Scanned()
	sum.denied = stats.Denied()
	sum.interrupted = ctx.Err() != nil
	sum.elapsed = time.Since(start)
	p.summary(sum)
	out.Flush()

	if sum.openErr != nil {
		fmt.Fprintf(stderr, "error: no se pudo abrir el Explorador: %v\n", sum.openErr)
	}

	switch {
	case sum.interrupted:
		return exitInterrupted
	case sum.found == 0:
		return exitNoMatch
	default:
		return exitFound
	}
}

// runInteractive abre el modo interactivo con los valores de cfg.
func runInteractive(cfg config, stdout, stderr io.Writer, version string) int {
	if !isInteractive(stdout) {
		return usageError(stderr, errors.New("el modo interactivo requiere una terminal"))
	}
	opts := tui.Options{
		Term:     cfg.term,
		Hidden:   cfg.all,
		Projects: cfg.projects,
		Modified: cfg.modified,
		CDFile:   cfg.cdFile,
	}
	if cfg.root != defaultRoot {
		root, err := pathutil.Resolve(cfg.root)
		if err != nil {
			return usageError(stderr, err)
		}
		opts.Root = root
	}
	if cfg.modified != "" {
		if _, err := period.Parse(cfg.modified, time.Now()); err != nil {
			return usageError(stderr, err)
		}
	}

	if err := tui.Run(version, opts); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitUsage
	}
	return exitFound
}

// sized es una carpeta encontrada junto con su tamaño.
type sized struct {
	search.Result
	search.SizeInfo
}

// measure calcula el tamaño de cada carpeta y las ordena de mayor a menor.
func measure(ctx context.Context, found []search.Result) []sized {
	list := make([]sized, len(found))
	for i, r := range found {
		list[i] = sized{Result: r, SizeInfo: search.Size(ctx, r.Path)}
	}
	slices.SortStableFunc(list, func(a, b sized) int {
		return cmp.Compare(b.Bytes, a.Bytes)
	})
	return list
}

// describe resume lo que se busca para la cabecera: `"tesis"`,
// `proyectos "api"`, `carpetas modificadas hoy`...
func describe(cfg config, when period.Range) string {
	var parts []string
	switch {
	case cfg.projects:
		parts = append(parts, "proyectos")
	case cfg.term == "" || when.Label != "":
		parts = append(parts, "carpetas")
	}
	if cfg.term != "" {
		parts = append(parts, `"`+cfg.term+`"`)
	}
	if when.Label != "" {
		adjective := "modificadas"
		if cfg.projects {
			adjective = "modificados"
		}
		parts = append(parts, adjective+" "+when.Label)
	}
	return strings.Join(parts, " ")
}

// parseArgs interpreta los argumentos. Cada opción admite su forma corta y
// larga (-n / --name). El término también puede pasarse como argumento
// posicional, y las opciones pueden ir antes o después de él:
//
//	fast-folder-cli proyecto -a -o
func parseArgs(args []string) (config, error) {
	cfg := config{root: defaultRoot}

	fs := flag.NewFlagSet("fast-folder-cli", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // los errores y la ayuda se muestran en Run
	fs.StringVar(&cfg.term, "n", "", "")
	fs.StringVar(&cfg.term, "name", "", "")
	fs.StringVar(&cfg.root, "p", defaultRoot, "")
	fs.StringVar(&cfg.root, "path", defaultRoot, "")
	fs.BoolVar(&cfg.all, "a", false, "")
	fs.BoolVar(&cfg.all, "all", false, "")
	fs.BoolVar(&cfg.open, "o", false, "")
	fs.BoolVar(&cfg.open, "open", false, "")
	fs.BoolVar(&cfg.version, "v", false, "")
	fs.BoolVar(&cfg.version, "version", false, "")
	fs.StringVar(&cfg.modified, "m", "", "")
	fs.StringVar(&cfg.modified, "modified", "", "")
	fs.BoolVar(&cfg.projects, "projects", false, "")
	fs.BoolVar(&cfg.size, "s", false, "")
	fs.BoolVar(&cfg.size, "size", false, "")
	fs.StringVar(&cfg.cdFile, "cd-file", "", "")

	// El paquete flag deja de interpretar opciones en el primer argumento
	// posicional; se reanuda el análisis tras cada uno para permitir
	// intercalarlos.
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return cfg, translateFlagError(err)
		}
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		if consumed := len(args) - len(rest); consumed > 0 && args[consumed-1] == "--" {
			// Tras "--" todo es posicional (útil para términos que empiezan por "-").
			positional = append(positional, rest...)
			break
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}

	if len(positional) > 0 {
		if cfg.term != "" {
			return cfg, fmt.Errorf("argumentos inesperados: %s", strings.Join(positional, " "))
		}
		cfg.term = strings.Join(positional, " ")
	}
	return cfg, nil
}

// translateFlagError traduce al español los errores del paquete flag.
func translateFlagError(err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return err
	}
	msg := err.Error()
	for prefix, es := range map[string]string{
		"flag provided but not defined: ": "opción desconocida: ",
		"flag needs an argument: ":        "falta el valor de la opción ",
	} {
		if rest, ok := strings.CutPrefix(msg, prefix); ok {
			return errors.New(es + rest)
		}
	}
	return errors.New("argumentos inválidos: " + msg)
}

// isInteractive indica si la entrada y la salida son una terminal real, en la
// que se puede usar el modo interactivo.
func isInteractive(stdout io.Writer) bool {
	f, ok := stdout.(*os.File)
	return ok && term.IsTerminal(int(f.Fd())) && term.IsTerminal(int(os.Stdin.Fd()))
}

func usageError(w io.Writer, err error) int {
	fmt.Fprintf(w, "error: %v\n", err)
	fmt.Fprintln(w, "Ejecuta 'fast-folder-cli --help' para ver las opciones disponibles.")
	return exitUsage
}

func printUsage(w io.Writer) {
	io.WriteString(w, `fast-folder-cli — búsqueda ultrarrápida de carpetas para Windows

Uso:
  fast-folder-cli                        modo interactivo (se maneja con las flechas)
  fast-folder-cli -n <término> [-p <ruta>] [opciones]
  fast-folder-cli <término> [opciones]
  fast-folder-cli --projects [término] [opciones]

Si lo instalaste con el asistente o el script, también puedes escribir "fast",
y "fcd <término>" para buscar una carpeta y entrar en ella desde la terminal.

Opciones:
  -n, --name <término>      Término o patrón a buscar. No distingue mayúsculas
                            ni acentos ("cancion" encuentra "Canción"). Sin
                            comodines busca coincidencias parciales; con * ? [ ]
                            el patrón debe cubrir el nombre completo.
  -p, --path <ruta>         Carpeta raíz de la búsqueda (por defecto: %USERPROFILE%).
                            Acepta %APPDATA%, %LOCALAPPDATA%, %PROGRAMDATA%, ~, D:, ...
  -m, --modified <periodo>  Solo carpetas modificadas en ese periodo: hoy, ayer,
                            semana, mes, año, un número de días (3d) o una fecha
                            (2026-09-01).
      --projects            Busca carpetas de proyectos (Git, Node.js, Python,
                            Go, .NET, Java, Unity...) en lugar de cualquier carpeta.
  -s, --size                Calcula cuánto ocupa cada carpeta encontrada y las
                            ordena de mayor a menor.
  -a, --all                 Incluye carpetas ocultas y de sistema.
  -o, --open                Abre la primera coincidencia en el Explorador de Windows.
  -v, --version             Muestra la versión.
  -h, --help                Muestra esta ayuda.

Ejemplos:
  fast-folder-cli -n proyecto
  fast-folder-cli cancion
  fast-folder-cli -n "tesis*" -p D:\ -o
  fast-folder-cli --projects
  fast-folder-cli -m ayer -p %USERPROFILE%\Documents
  fast-folder-cli node_modules --size -p C:\dev

Códigos de salida: 0 = hay resultados, 1 = sin resultados, 2 = error de uso,
130 = interrumpido con Ctrl+C.
`)
}
