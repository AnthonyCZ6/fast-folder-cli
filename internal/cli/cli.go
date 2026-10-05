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

	"github.com/AnthonyCZ6/fast-folder-cli/internal/apps"
	prefs "github.com/AnthonyCZ6/fast-folder-cli/internal/config"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/launch"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/pathutil"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/period"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
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

// openExplorer abre una carpeta en el Explorador (--open). Las pruebas lo
// reemplazan para no abrir ventanas.
var openExplorer = launch.Explorer

// findApps busca las aplicaciones instaladas (--apps) y showApp abre la
// ubicación de una (--apps --open). Las pruebas los reemplazan para no
// depender de lo instalado ni abrir ventanas.
var (
	findApps = apps.Find
	showApp  = launch.ShowApp
)

// folderSizer mide las carpetas encontradas con --size (search.Sizer): cada
// una empieza a medirse en cuanto aparece, mientras sigue la búsqueda.
type folderSizer interface {
	Add(path string)
	Wait() []search.SizeInfo
}

// newSizer crea el folderSizer de una búsqueda. Las pruebas lo reemplazan
// para simular una interrupción mientras se mide.
var newSizer = func(ctx context.Context) folderSizer { return search.NewSizer(ctx) }

// config contiene las opciones ya interpretadas de la línea de comandos.
type config struct {
	term     string
	root     string
	all      bool
	open     bool
	version  bool
	modified string
	projects bool
	apps     bool
	recent   bool   // --recientes: carpetas usadas hace poco
	with     string // --con: solo las de lo abierto con este programa (implica --recientes)
	size     bool
	cdFile   string
	exclude  []string // carpetas excluidas: las de --exclude y las del archivo
	editConf bool     // --config: crear y abrir el archivo de configuración
	json     bool     // --json: una línea JSON por resultado
	rootSet  bool     // se indicó -p, aunque sea con el valor por defecto

	// Preferencias del archivo de configuración (no vienen de las banderas).
	prefs prefs.Config
}

// loadPrefs lee el archivo de configuración. Las pruebas lo reemplazan para
// no depender del archivo de quien las ejecuta.
var loadPrefs = prefs.Load

// openConfigFile abre el archivo de configuración con el editor de las
// preferencias o, si no hay, con el Bloc de notas. Las pruebas lo reemplazan.
var openConfigFile = func(editor, path string) error {
	if editor == "" {
		return launch.Notepad(path)
	}
	return launch.Editor(editor, path)
}

// withPrefs aplica las preferencias p a lo que no indican las banderas: la
// ubicación, las ocultas y las carpetas excluidas. A las apps no se aplican.
func (cfg config) withPrefs(p prefs.Config) config {
	cfg.prefs = p
	if cfg.apps {
		return cfg
	}
	if !cfg.rootSet && p.Root != "" {
		cfg.root = p.Root
	}
	cfg.all = cfg.all || p.Hidden
	cfg.exclude = append(slices.Clip(p.Exclude), cfg.exclude...)
	return cfg
}

// readPrefs lee el archivo de configuración. Si tiene errores, lo avisa por
// stderr y sigue con los valores de siempre.
func readPrefs(stderr io.Writer) prefs.Config {
	p, err := loadPrefs()
	if err != nil {
		fmt.Fprintf(stderr, "aviso: se ignora la configuración: %v\n", err)
	}
	if p.Root != "" {
		if _, err := pathutil.Resolve(p.Root); err != nil {
			fmt.Fprintf(stderr, "aviso: se ignora la ubicación de la configuración: %v\n", err)
			p.Root = ""
		}
	}
	return p
}

// kind devuelve qué se busca según las opciones.
func (cfg config) kind() query.Kind {
	switch {
	case cfg.apps:
		return query.Apps
	case cfg.recent || cfg.with != "":
		return query.Recent
	case cfg.projects:
		return query.Projects
	}
	return query.Folders
}

// checkKinds rechaza las combinaciones de opciones que no tienen sentido.
func (cfg config) checkKinds() error {
	if err := cfg.checkRecent(); err != nil {
		return err
	}
	return cfg.checkApps()
}

// checkRecent rechaza las opciones que no tienen sentido con --recientes,
// que no recorre carpetas sino el historial de Windows.
func (cfg config) checkRecent() error {
	if !cfg.recent && cfg.with == "" {
		return nil
	}
	flag := "--recientes"
	if !cfg.recent {
		flag = "--con"
	}
	switch {
	case cfg.apps:
		return errors.New(flag + " y --apps no se pueden usar juntas")
	case cfg.projects:
		return errors.New(flag + " y --projects no se pueden usar juntas")
	case cfg.size:
		return errors.New("--size no se aplica a las carpetas recientes")
	}
	return nil
}

// checkApps rechaza las opciones que no tienen sentido al buscar apps, que
// no se buscan en una carpeta sino en la lista de programas instalados.
func (cfg config) checkApps() error {
	if !cfg.apps {
		return nil
	}
	switch {
	case cfg.projects:
		return errors.New("--apps y --projects no se pueden usar juntas")
	case cfg.modified != "":
		return errors.New("--modified no se aplica a las apps")
	case cfg.size:
		return errors.New("--size no se aplica a las apps")
	case cfg.all:
		return errors.New("--all no se aplica a las apps")
	case len(cfg.exclude) > 0:
		return errors.New("--exclude no se aplica a las apps")
	case cfg.rootSet:
		return errors.New("--path no se aplica a las apps: se buscan entre los programas instalados")
	}
	return nil
}

// Run ejecuta la CLI con los argumentos indicados (sin el nombre del programa)
// y devuelve el código de salida del proceso.
func Run(args []string, stdout, stderr io.Writer, version string) int {
	// Sin argumentos y en una terminal se abre el modo interactivo (más
	// abajo, porque no hay nada que buscar).
	if len(args) == 0 && !isInteractive(stdout) {
		// Sin argumentos y sin terminal (por ejemplo, al comprobar que el
		// programa quedó instalado) se muestra la ayuda y se termina bien.
		printUsage(stdout)
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
	case cfg.editConf:
		return runConfig(stdout, stderr)
	}
	cfg = cfg.withPrefs(readPrefs(stderr))

	// Sin término (ni proyectos ni fecha) no hay nada que buscar: en una
	// terminal se abre el modo interactivo con las opciones indicadas (así
	// funciona "fast -p D:" y el menú contextual del Explorador). --cd-file,
	// que usan los scripts fcd, también lo abre siempre.
	q, err := query.New(cfg.term, cfg.kind(), cfg.modified, time.Now())
	empty := errors.Is(err, query.ErrEmpty)
	if cfg.cdFile != "" || (empty && isInteractive(stdout)) {
		return runInteractive(cfg, stdout, stderr, version)
	}
	switch {
	case empty:
		printUsage(stderr)
		return exitUsage
	case err != nil:
		return usageError(stderr, err)
	}
	switch q.Kind {
	case query.Apps:
		return runApps(cfg, q, stdout, stderr)
	case query.Recent:
		return runRecent(cfg, q, stdout, stderr)
	}

	root, err := pathutil.Resolve(cfg.root)
	if err != nil {
		return usageError(stderr, err)
	}

	// Ctrl+C cancela la búsqueda (o el cálculo de tamaños) y muestra lo
	// encontrado hasta ese momento.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return runSearch(ctx, cfg, q, root, stdout, stderr)
}

// runSearch busca q en root, muestra cada carpeta en cuanto aparece y termina
// con un resumen. Si ctx se cancela, termina con lo encontrado hasta ese
// momento. Devuelve el código de salida.
func runSearch(ctx context.Context, cfg config, q query.Query, root string, stdout, stderr io.Writer) int {
	start := time.Now()

	opts := q.Options(root, cfg.all)
	opts.Prune = cfg.size
	opts.Exclude = cfg.exclude
	results, stats := search.Start(ctx, opts)

	out := bufio.NewWriter(stdout)
	p := newPrinter(out, supportsColor(stdout) && !cfg.json)
	p.json = cfg.json
	p.header(q.Describe(), root, cfg.all)

	var sizer folderSizer
	if cfg.size {
		sizer = newSizer(ctx)
	}
	sum, found := collect(cfg, p, nextResult(results, out), sizer)
	sum.query = q
	if cfg.size && len(found) > 0 {
		sum.sized, sum.bytes, sum.files = showSized(ctx, p, found, sizer.Wait())
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
	return exitCode(sum)
}

// runConfig crea el archivo de configuración con la plantilla comentada, si
// no existe, muestra su ruta y, en una terminal, lo abre para editarlo.
func runConfig(stdout, stderr io.Writer) int {
	path, err := prefs.Path()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitUsage
	}
	created, err := prefs.Create(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: no se pudo crear %s: %v\n", path, err)
		return exitUsage
	}
	if created {
		fmt.Fprintf(stdout, "Configuración creada con la plantilla: %s\n", path)
	} else {
		fmt.Fprintf(stdout, "Configuración: %s\n", path)
	}
	if isInteractive(stdout) {
		p, _ := loadPrefs()
		if err := openConfigFile(p.Editor, path); err != nil {
			fmt.Fprintf(stderr, "error: no se pudo abrir el archivo: %v\n", err)
		}
	}
	return exitFound
}

// runApps busca q entre las aplicaciones instaladas, las muestra y, con
// --open, abre en el Explorador la ubicación de la primera. Devuelve el
// código de salida.
func runApps(cfg config, q query.Query, stdout, stderr io.Writer) int {
	start := time.Now()
	list, err := findApps(q.Match)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitUsage
	}

	out := bufio.NewWriter(stdout)
	p := newPrinter(out, supportsColor(stdout) && !cfg.json)
	p.json = cfg.json
	p.appsHeader(q.Describe())
	for i, a := range list {
		p.app(int64(i+1), a)
	}

	sum := summary{query: q, found: int64(len(list))}
	if cfg.open && len(list) > 0 {
		sum.opened = list[0].Dir
		sum.openErr = showApp(list[0].Dir, list[0].Exe)
	}
	sum.elapsed = time.Since(start)
	p.summary(sum)
	out.Flush()

	if sum.openErr != nil {
		fmt.Fprintf(stderr, "error: no se pudo abrir el Explorador: %v\n", sum.openErr)
	}
	return exitCode(sum)
}

// nextResult devuelve una función que lee el siguiente resultado. La salida
// se agrupa en el búfer out, que se vacía cada vez que no hay más resultados
// inmediatamente disponibles: los resultados aparecen en cuanto se
// encuentran, pero miles de coincidencias seguidas no se escriben línea a
// línea en la consola (que es lenta).
func nextResult(results <-chan search.Result, out *bufio.Writer) func() (search.Result, bool) {
	return func() (search.Result, bool) {
		select {
		case r, ok := <-results:
			return r, ok
		default:
			out.Flush()
			r, ok := <-results
			return r, ok
		}
	}
}

// collect lee todos los resultados con next. Los muestra en cuanto llegan
// (con --size los guarda para mostrarlos al final, ordenados por tamaño, y
// empieza a medirlos con sizer) y, con --open, abre el primero. Devuelve el
// recuento en el resumen y las carpetas guardadas.
func collect(cfg config, p *printer, next func() (search.Result, bool), sizer folderSizer) (summary, []search.Result) {
	var sum summary
	var found []search.Result
	for r, ok := next(); ok; r, ok = next() {
		sum.found++
		if cfg.size {
			found = append(found, r)
			sizer.Add(r.Path)
		} else {
			p.match(sum.found, r)
		}

		if cfg.open && sum.found == 1 {
			// Se abre en cuanto aparece, sin esperar al final del recorrido.
			sum.opened = r.Path
			sum.openErr = openExplorer(r.Path)
		}
	}
	return sum, found
}

// exitCode devuelve el código de salida que corresponde al resumen.
func exitCode(sum summary) int {
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
		Kind:     cfg.kind(),
		Modified: cfg.modified,
		CDFile:   cfg.cdFile,
		Exclude:  cfg.exclude,
		With:     cfg.with,
		Prefs:    cfg.prefs,
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

// bySize junta cada carpeta con su tamaño (infos, en el mismo orden) y las
// ordena de mayor a menor; a igual tamaño, en el orden en que aparecieron.
func bySize(found []search.Result, infos []search.SizeInfo) []sized {
	list := make([]sized, len(found))
	for i, r := range found {
		list[i] = sized{Result: r, SizeInfo: infos[i]}
	}
	slices.SortStableFunc(list, func(a, b sized) int {
		return cmp.Compare(b.Bytes, a.Bytes)
	})
	return list
}

// showSized muestra al final las carpetas guardadas con --size, de mayor a
// menor tamaño según infos, y devuelve los totales. Si Ctrl+C llegó antes de
// medirlas todas, sus tamaños están incompletos: entonces las muestra sin
// tamaño, en el orden en que aparecieron, y devuelve measured = false.
func showSized(ctx context.Context, p *printer, found []search.Result, infos []search.SizeInfo) (measured bool, size, files int64) {
	if ctx.Err() != nil {
		for i, r := range found {
			p.match(int64(i+1), r)
		}
		return false, 0, 0
	}
	list := bySize(found, infos)
	p.sizes(list)
	size, files = totals(list)
	return true, size, files
}

// totals suma el tamaño y el número de archivos de las carpetas de list.
func totals(list []sized) (size, files int64) {
	for _, item := range list {
		size += item.Bytes
		files += item.Files
	}
	return size, files
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
	fs.BoolVar(&cfg.apps, "apps", false, "")
	fs.BoolVar(&cfg.recent, "recientes", false, "")
	fs.StringVar(&cfg.with, "con", "", "")
	fs.BoolVar(&cfg.size, "s", false, "")
	fs.BoolVar(&cfg.size, "size", false, "")
	fs.StringVar(&cfg.cdFile, "cd-file", "", "")
	fs.BoolVar(&cfg.editConf, "config", false, "")
	fs.BoolVar(&cfg.json, "json", false, "")
	var exclude string
	fs.StringVar(&exclude, "exclude", "", "")

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
	emptyWith := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "p", "path":
			cfg.rootSet = true
		case "con":
			emptyWith = strings.TrimSpace(cfg.with) == ""
		}
	})
	if emptyWith {
		return cfg, errors.New("--con necesita el nombre de un programa (por ejemplo, --con word)")
	}
	for _, name := range strings.Split(exclude, ",") {
		if name = strings.TrimSpace(name); name != "" {
			cfg.exclude = append(cfg.exclude, name)
		}
	}
	return cfg, cfg.checkKinds()
}

// flagErrors asocia el comienzo de cada error del paquete flag con su
// traducción.
var flagErrors = []struct{ prefix, es string }{
	{"flag provided but not defined: ", "opción desconocida: "},
	{"flag needs an argument: ", "falta el valor de la opción "},
}

// translateFlagError traduce al español los errores del paquete flag.
func translateFlagError(err error) error {
	if errors.Is(err, flag.ErrHelp) {
		return err
	}
	msg := err.Error()
	for _, t := range flagErrors {
		if rest, ok := strings.CutPrefix(msg, t.prefix); ok {
			return errors.New(t.es + rest)
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
  fast-folder-cli --recientes [término] [--con <programa>] [opciones]
  fast-folder-cli --apps [término] [-o]

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
      --recientes           Busca entre las carpetas en las que trabajaste hace
                            poco (las de lo que abriste en cualquier programa,
                            según el historial de Windows), de la usada más
                            recientemente a la más antigua. Con -m, por cuándo
                            las usaste; con -p, solo dentro de esa carpeta.
      --con <programa>      Solo las carpetas de lo que abriste con ese
                            programa: word, excel, code, fotos... (implica
                            --recientes).
      --apps                Busca aplicaciones instaladas por su nombre (las de
                            Configuración → Aplicaciones). Con -o abre su
                            ubicación con el ejecutable seleccionado.
  -s, --size                Calcula cuánto ocupa cada carpeta encontrada y las
                            ordena de mayor a menor.
  -a, --all                 Incluye carpetas ocultas y de sistema.
      --exclude <a,b>       No muestra ni recorre las carpetas con esos nombres
                            (node_modules, venv...). Se suman a las del archivo
                            de configuración.
      --json                Una línea JSON por resultado, para scripts, sin
                            cabecera ni resumen.
      --config              Crea, si no existe, el archivo de configuración
                            (%APPDATA%\fast-folder-cli\config.toml) y lo abre.
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
  fast-folder-cli --recientes -m ayer
  fast-folder-cli --con word -m semana
  fast-folder-cli --apps chrome -o

Códigos de salida: 0 = hay resultados, 1 = sin resultados, 2 = error de uso,
130 = interrumpido con Ctrl+C.
`)
}
