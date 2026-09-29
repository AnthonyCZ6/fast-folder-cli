// Package search implementa el recorrido concurrente de directorios.
//
// El recorrido usa un número acotado de goroutines: cada subdirectorio se
// entrega a una goroutine nueva si hay un "slot" libre en el semáforo; si no,
// la goroutine actual lo procesa en línea (en profundidad). Así nunca hay más
// de Workers goroutines activas, no puede producirse un interbloqueo y los
// núcleos se mantienen ocupados mientras quede trabajo pendiente.
package search

import (
	"context"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// Options configura una búsqueda.
type Options struct {
	// Root es el directorio desde el que comienza el recorrido. Debe ser una
	// ruta ya resuelta (sin variables de entorno).
	Root string
	// Matcher decide qué nombres de carpeta coinciden.
	Matcher *Matcher
	// IncludeHidden incluye carpetas ocultas y de sistema.
	IncludeHidden bool
	// Workers es el máximo de goroutines de recorrido simultáneas.
	// Si es <= 0 se usa DefaultWorkers().
	Workers int
}

// Stats contiene los contadores de una búsqueda. Sus valores son definitivos
// una vez que el canal de resultados se ha cerrado.
type Stats struct {
	scanned atomic.Int64
	denied  atomic.Int64
}

// Scanned devuelve el número de carpetas cuyo contenido se ha leído.
func (s *Stats) Scanned() int64 { return s.scanned.Load() }

// Denied devuelve el número de carpetas que no se pudieron leer
// (normalmente por falta de permisos).
func (s *Stats) Denied() int64 { return s.denied.Load() }

// DefaultWorkers devuelve el grado de concurrencia por defecto. Leer
// directorios es una operación dominada por E/S, así que conviene tener más
// goroutines que núcleos.
func DefaultWorkers() int {
	return max(runtime.NumCPU()*4, 8)
}

// Start inicia la búsqueda en segundo plano y devuelve un canal por el que se
// emite la ruta completa de cada carpeta coincidente. El canal se cierra al
// terminar el recorrido o al cancelarse ctx.
func Start(ctx context.Context, opts Options) (<-chan string, *Stats) {
	workers := opts.Workers
	if workers <= 0 {
		workers = DefaultWorkers()
	}

	out := make(chan string, 64)
	w := &walker{
		ctx:           ctx,
		matcher:       opts.Matcher,
		includeHidden: opts.IncludeHidden,
		sem:           make(chan struct{}, workers),
		out:           out,
		stats:         &Stats{},
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.walk(opts.Root)
	}()
	go func() {
		w.wg.Wait()
		close(out)
	}()

	return out, w.stats
}

type walker struct {
	ctx           context.Context
	matcher       *Matcher
	includeHidden bool
	sem           chan struct{}
	wg            sync.WaitGroup
	out           chan<- string
	stats         *Stats
}

// walk lee dir, emite las subcarpetas que coinciden y desciende en ellas.
func (w *walker) walk(dir string) {
	if w.ctx.Err() != nil {
		return
	}

	f, err := os.Open(dir)
	if err != nil {
		// Acceso denegado, ruta eliminada durante el recorrido, etc.
		// Se contabiliza y se sigue con el resto del árbol.
		w.stats.denied.Add(1)
		return
	}
	// File.ReadDir no ordena las entradas (os.ReadDir sí), lo que ahorra
	// trabajo innecesario en carpetas grandes.
	entries, err := f.ReadDir(-1)
	f.Close()
	if err != nil && len(entries) == 0 {
		w.stats.denied.Add(1)
		return
	}
	w.stats.scanned.Add(1)

	for _, e := range entries {
		kind := inspect(dir, e)
		if !kind.dir || (kind.hidden && !w.includeHidden) {
			continue
		}

		path := join(dir, e.Name())
		if w.matcher.Match(e.Name()) {
			select {
			case w.out <- path:
			case <-w.ctx.Done():
				return
			}
		}

		// Los enlaces simbólicos y las uniones (junctions) se reportan si
		// coinciden, pero nunca se recorren: pueden apuntar fuera de la raíz
		// o formar ciclos infinitos (p. ej. "Application Data").
		if !kind.link {
			w.descend(path)
		}
	}
}

// descend procesa path en una goroutine nueva si hay capacidad libre, o en la
// goroutine actual en caso contrario.
func (w *walker) descend(path string) {
	select {
	case w.sem <- struct{}{}:
		w.wg.Add(1)
		go func() {
			defer func() {
				<-w.sem
				w.wg.Done()
			}()
			w.walk(path)
		}()
	default:
		w.walk(path)
	}
}

// join concatena dir y name evitando el coste de filepath.Join (que limpia la
// ruta completa en cada llamada). dir siempre es una ruta ya limpia.
func join(dir, name string) string {
	if os.IsPathSeparator(dir[len(dir)-1]) {
		return dir + name
	}
	return dir + string(os.PathSeparator) + name
}

// entryKind describe una entrada de directorio desde el punto de vista de la
// búsqueda.
type entryKind struct {
	dir    bool // es una carpeta (o un enlace a una carpeta)
	link   bool // es un enlace simbólico o junction: no se recorre
	hidden bool // está oculta o es de sistema
}

// isDotName indica si el nombre empieza por punto (.git, .vscode, .cache...),
// convención habitual para carpetas ocultas de herramientas de desarrollo.
func isDotName(name string) bool {
	return strings.HasPrefix(name, ".")
}
