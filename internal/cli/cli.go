// Package cli implementa la interfaz de línea de comandos de fast-folder-cli.
package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/explorer"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/pathutil"
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
	term    string
	root    string
	all     bool
	open    bool
	version bool
}

// Run ejecuta la CLI con los argumentos indicados (sin el nombre del programa)
// y devuelve el código de salida del proceso.
func Run(args []string, stdout, stderr io.Writer, version string) int {
	start := time.Now()

	// Sin argumentos y en una terminal se abre el modo interactivo, que se
	// maneja con las flechas. Si la salida está redirigida se muestra la ayuda.
	if len(args) == 0 && isInteractive(stdout) {
		if err := tui.Run(version); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return exitUsage
		}
		return exitFound
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
	case cfg.term == "":
		printUsage(stderr)
		return exitUsage
	}

	matcher, err := search.NewMatcher(cfg.term)
	if err != nil {
		return usageError(stderr, err)
	}
	root, err := pathutil.Resolve(cfg.root)
	if err != nil {
		return usageError(stderr, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	results, stats := search.Start(ctx, search.Options{
		Root:          root,
		Matcher:       matcher,
		IncludeHidden: cfg.all,
	})

	// La salida se agrupa en un búfer que se vacía cada vez que no hay más
	// resultados inmediatamente disponibles: los resultados aparecen en
	// cuanto se encuentran, pero miles de coincidencias seguidas no se
	// escriben línea a línea en la consola (que es lenta).
	out := bufio.NewWriter(stdout)
	p := newPrinter(out, supportsColor(stdout))
	p.header(cfg.term, root, cfg.all)

	next := func() (string, bool) {
		select {
		case path, ok := <-results:
			return path, ok
		default:
			out.Flush()
			path, ok := <-results
			return path, ok
		}
	}

	var sum summary
	for path, ok := next(); ok; path, ok = next() {
		sum.found++
		p.match(sum.found, path)

		if cfg.open && sum.found == 1 {
			// Se abre en cuanto aparece, sin esperar al final del recorrido.
			sum.opened = path
			sum.openErr = explorer.Open(path)
		}
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
  fast-folder-cli -n <término> [-p <ruta>] [-a] [-o]
  fast-folder-cli <término> [opciones]

Si lo instalaste con el asistente o el script, también puedes escribir "fast".

Opciones:
  -n, --name <término>  Término o patrón a buscar (sin distinguir mayúsculas).
                        Sin comodines busca coincidencias parciales; con * ? [ ]
                        el patrón debe cubrir el nombre completo.
  -p, --path <ruta>     Carpeta raíz de la búsqueda (por defecto: %USERPROFILE%).
                        Acepta %APPDATA%, %LOCALAPPDATA%, %PROGRAMDATA%, ~, D:, ...
  -a, --all             Incluye carpetas ocultas y de sistema.
  -o, --open            Abre la primera coincidencia en el Explorador de Windows.
  -v, --version         Muestra la versión.
  -h, --help            Muestra esta ayuda.

Ejemplos:
  fast-folder-cli -n proyecto
  fast-folder-cli -n "node_modules" -p %APPDATA% -a
  fast-folder-cli -n "tesis*" -p D:\ -o

Códigos de salida: 0 = hay resultados, 1 = sin resultados, 2 = error de uso,
130 = interrumpido con Ctrl+C.
`)
}
