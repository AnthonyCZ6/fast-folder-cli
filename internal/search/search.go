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
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Options configura una búsqueda.
type Options struct {
	// Root es el directorio desde el que comienza el recorrido. Debe ser una
	// ruta ya resuelta (sin variables de entorno).
	Root string
	// Matcher decide qué nombres de carpeta coinciden. Si es nil coinciden
	// todas, lo que sirve para listar proyectos o filtrar solo por fecha.
	Matcher *Matcher
	// IncludeHidden incluye carpetas ocultas y de sistema.
	IncludeHidden bool
	// Projects busca carpetas de proyectos (las que contienen .git, go.mod,
	// package.json...) en lugar de carpetas cualesquiera. El interior de un
	// proyecto no se recorre, y la raíz nunca cuenta como proyecto.
	Projects bool
	// ModifiedAfter y ModifiedBefore, si no son cero, limitan los resultados
	// a las carpetas modificadas en el intervalo [ModifiedAfter,
	// ModifiedBefore).
	ModifiedAfter, ModifiedBefore time.Time
	// Prune evita recorrer el interior de las carpetas encontradas; por
	// ejemplo, para no contar dos veces los node_modules anidados.
	Prune bool
	// Workers es el máximo de goroutines de recorrido simultáneas.
	// Si es <= 0 se usa DefaultWorkers().
	Workers int
}

// Result es una carpeta encontrada.
type Result struct {
	Path string
	// Project es el tipo de proyecto ("Go", "Node.js, Git"...) en el modo
	// proyectos; en el resto de búsquedas está vacío.
	Project string
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
// emite cada carpeta encontrada. El canal se cierra al terminar el recorrido o
// al cancelarse ctx.
func Start(ctx context.Context, opts Options) (<-chan Result, *Stats) {
	workers := opts.Workers
	if workers <= 0 {
		workers = DefaultWorkers()
	}

	out := make(chan Result, 64)
	w := &walker{
		ctx:   ctx,
		opts:  opts,
		sem:   make(chan struct{}, workers),
		out:   out,
		stats: &Stats{},
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.walk(opts.Root, true)
	}()
	go func() {
		w.wg.Wait()
		close(out)
	}()

	return out, w.stats
}

type walker struct {
	ctx   context.Context
	opts  Options
	sem   chan struct{}
	wg    sync.WaitGroup
	out   chan<- Result
	stats *Stats
}

// walk lee dir, emite las subcarpetas que coinciden y desciende en ellas.
func (w *walker) walk(dir string, root bool) {
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

	// En el modo proyectos, una carpeta con indicios de proyecto se reporta
	// y no se recorre: su interior (node_modules, bin, .git...) no interesa.
	// La raíz se recorre siempre, porque un package.json suelto en la carpeta
	// personal ocultaría todos los proyectos.
	if w.opts.Projects && !root {
		if kind := detectProject(entries); kind != "" {
			if w.accept(dir, filepath.Base(dir)) {
				w.emit(Result{Path: dir, Project: kind})
			}
			return
		}
	}

	for _, e := range entries {
		kind := inspect(dir, e)
		if !kind.dir || (kind.hidden && !w.opts.IncludeHidden) {
			continue
		}

		path := join(dir, e.Name())
		if !w.opts.Projects && w.accept(path, e.Name()) {
			if !w.emit(Result{Path: path}) {
				return
			}
			if w.opts.Prune {
				continue
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

// accept aplica el término de búsqueda y el filtro de fecha a una carpeta.
func (w *walker) accept(path, name string) bool {
	if w.opts.Matcher != nil && !w.opts.Matcher.Match(name) {
		return false
	}
	after, before := w.opts.ModifiedAfter, w.opts.ModifiedBefore
	if after.IsZero() && before.IsZero() {
		return true
	}
	// La fecha se consulta solo para las carpetas que ya coinciden con el
	// término. Es una llamada más al sistema, pero da la fecha exacta: en
	// NTFS, la que devuelve el listado del directorio padre puede estar
	// desactualizada.
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	mod := info.ModTime()
	if !after.IsZero() && mod.Before(after) {
		return false
	}
	return before.IsZero() || mod.Before(before)
}

// emit envía r por el canal de resultados. Devuelve false si la búsqueda se
// canceló mientras esperaba.
func (w *walker) emit(r Result) bool {
	select {
	case w.out <- r:
		return true
	case <-w.ctx.Done():
		return false
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
			w.walk(path, false)
		}()
	default:
		w.walk(path, false)
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
