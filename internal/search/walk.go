package search

import (
	"io/fs"
	"os"
	"strings"
	"sync"
)

// Piezas comunes a los dos recorridos del paquete: la búsqueda (Start) y el
// cálculo de tamaños (Size).

// pool reparte el recorrido de las subcarpetas entre un número acotado de
// goroutines: cada subcarpeta se procesa en una goroutine nueva si hay un
// "slot" libre en el semáforo; si no, la goroutine actual la procesa en línea
// (en profundidad). Así nunca hay más de workers goroutines activas, no puede
// producirse un interbloqueo y los núcleos se mantienen ocupados mientras
// quede trabajo pendiente.
type pool struct {
	sem chan struct{}
	wg  sync.WaitGroup
}

// newPool crea un pool de workers goroutines; si workers <= 0 se usa
// DefaultWorkers().
func newPool(workers int) *pool {
	if workers <= 0 {
		workers = DefaultWorkers()
	}
	return &pool{sem: make(chan struct{}, workers)}
}

// do ejecuta fn en una goroutine nueva si hay capacidad libre, o en la
// goroutine actual en caso contrario.
func (p *pool) do(fn func()) {
	select {
	case p.sem <- struct{}{}:
		p.wg.Add(1)
		go func() {
			defer func() {
				<-p.sem
				p.wg.Done()
			}()
			fn()
		}()
	default:
		fn()
	}
}

// wait espera a que terminen todas las goroutines lanzadas con do. Debe
// llamarse cuando ya ha vuelto el recorrido de la raíz: a partir de ahí solo
// las goroutines pendientes pueden lanzar otras.
func (p *pool) wait() {
	p.wg.Wait()
}

// readDir lee las entradas de dir. Devuelve false si no se pudo leer (acceso
// denegado, carpeta eliminada durante el recorrido...).
//
// File.ReadDir no ordena las entradas (os.ReadDir sí), lo que ahorra trabajo
// innecesario en carpetas grandes.
func readDir(dir string) ([]fs.DirEntry, bool) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, false
	}
	entries, err := f.ReadDir(-1)
	f.Close()
	return entries, err == nil || len(entries) > 0
}

// join concatena dir y name evitando el coste de filepath.Join (que limpia la
// ruta completa en cada llamada). dir siempre es una ruta ya limpia.
func join(dir, name string) string {
	if os.IsPathSeparator(dir[len(dir)-1]) {
		return dir + name
	}
	return dir + string(os.PathSeparator) + name
}

// entryKind describe una entrada de directorio desde el punto de vista del
// recorrido. inspect (específica de cada sistema) la obtiene.
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
