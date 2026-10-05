// Package recent encuentra las carpetas en las que se trabajó hace poco a
// partir de las jump lists de Windows (paquete jumplist): la carpeta de cada
// archivo que se abrió y las carpetas que se abrieron, con la última vez que
// se usó cada una. Así se encuentra una carpeta sin recordar su nombre.
package recent

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
)

// Folder es una carpeta usada hace poco.
type Folder struct {
	Path     string
	LastUsed time.Time // la última vez que se abrió algo en ella
	Items    int       // archivos que se abrieron en ella (0 si solo se abrió la carpeta)
	AppIDs   []uint64  // jump lists en las que aparece, de menor a mayor
}

// Options elige qué carpetas se devuelven. El valor cero las devuelve todas,
// salvo las ocultas.
type Options struct {
	Match         func(name string) bool // nombre de la carpeta; nil: cualquiera
	After, Before time.Time              // cuándo se abrió algo en ella; cero: sin límite
	Within        string                 // solo carpetas dentro de esta (o ella misma); "": cualquiera
	Exclude       []string               // carpetas excluidas por nombre, como --exclude
	IncludeHidden bool                   // incluir las que están dentro de carpetas ocultas o de sistema
	AppIDs        map[uint64]bool        // solo estas jump lists; nil: todas
}

// Find agrupa por carpeta las entradas de lists y devuelve las carpetas que
// existen y cumplen opts, de la usada más recientemente a la más antigua.
func Find(lists []jumplist.List, opts Options) []Folder {
	type group struct {
		folder Folder
		items  map[string]bool
		apps   map[uint64]bool
	}
	groups := map[string]*group{}
	f := newFilter(opts)
	for _, l := range lists {
		if opts.AppIDs != nil && !opts.AppIDs[l.AppID] {
			continue
		}
		for _, e := range l.Entries {
			if !inPeriod(e.LastUsed, opts) {
				continue
			}
			path := e.LocalPath()
			if path == "" {
				continue
			}
			dir := f.folderOf(path)
			k := key(dir)
			g := groups[k]
			if g == nil {
				g = &group{folder: Folder{Path: dir}, items: map[string]bool{}, apps: map[uint64]bool{}}
				groups[k] = g
			}
			if key(path) != k { // un archivo, no la propia carpeta
				g.items[key(path)] = true
			}
			g.apps[l.AppID] = true
			if e.LastUsed.After(g.folder.LastUsed) {
				g.folder.LastUsed = e.LastUsed
			}
		}
	}

	var out []Folder
	for _, g := range groups {
		if !f.accepts(g.folder.Path) {
			continue
		}
		g.folder.Items = len(g.items)
		for id := range g.apps {
			g.folder.AppIDs = append(g.folder.AppIDs, id)
		}
		slices.Sort(g.folder.AppIDs)
		out = append(out, g.folder)
	}
	slices.SortFunc(out, func(a, b Folder) int {
		return cmp.Or(b.LastUsed.Compare(a.LastUsed), cmp.Compare(key(a.Path), key(b.Path)))
	})
	return out
}

// inPeriod indica si se usó en el periodo de opts. Sin periodo, cualquier
// fecha vale (también la desconocida).
func inPeriod(t time.Time, opts Options) bool {
	if !opts.After.IsZero() && t.Before(opts.After) {
		return false
	}
	return opts.Before.IsZero() || t.Before(opts.Before)
}

// filter decide qué carpetas se muestran. Guarda lo que ya consultó al
// sistema de archivos: muchas entradas comparten carpeta.
type filter struct {
	opts    Options
	exclude map[string]bool // opts.Exclude normalizado con search.Fold
	isDir   map[string]bool
	hidden  map[string]bool
}

func newFilter(opts Options) *filter {
	f := &filter{opts: opts, isDir: map[string]bool{}, hidden: map[string]bool{}}
	for _, name := range opts.Exclude {
		if name = strings.TrimSpace(name); name != "" {
			if f.exclude == nil {
				f.exclude = map[string]bool{}
			}
			f.exclude[search.Fold(name)] = true
		}
	}
	return f
}

// folderOf devuelve la carpeta de una entrada: la propia ruta si es una
// carpeta y, si no (un archivo, o algo que ya no existe), la que la contiene.
func (f *filter) folderOf(path string) string {
	if f.dir(path) {
		return filepath.Clean(path)
	}
	return filepath.Dir(path)
}

// accepts indica si dir se muestra: existe, su nombre coincide, está dentro
// de Within y ni ella ni las carpetas que la contienen (hasta Within, sin
// incluirla, o hasta la raíz de la unidad) están excluidas u ocultas.
func (f *filter) accepts(dir string) bool {
	if !f.dir(dir) || (f.opts.Match != nil && !f.opts.Match(filepath.Base(dir))) {
		return false
	}
	stop := ""
	if f.opts.Within != "" {
		rel, err := filepath.Rel(f.opts.Within, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return false
		}
		stop = key(f.opts.Within)
	}
	for d := dir; key(d) != stop && filepath.Dir(d) != d; d = filepath.Dir(d) {
		if f.exclude[search.Fold(filepath.Base(d))] {
			return false
		}
		if !f.opts.IncludeHidden && f.isHidden(d) {
			return false
		}
	}
	return true
}

// dir indica si path es una carpeta que existe.
func (f *filter) dir(path string) bool {
	k := key(path)
	ok, seen := f.isDir[k]
	if !seen {
		info, err := os.Stat(path)
		ok = err == nil && info.IsDir()
		f.isDir[k] = ok
	}
	return ok
}

// isHidden indica si la carpeta d está oculta o es de sistema.
func (f *filter) isHidden(d string) bool {
	k := key(d)
	h, seen := f.hidden[k]
	if !seen {
		h = strings.HasPrefix(filepath.Base(d), ".") || hiddenAttr(d)
		f.hidden[k] = h
	}
	return h
}

// key normaliza una ruta para agrupar y comparar: Windows no distingue
// mayúsculas.
func key(path string) string {
	return strings.ToLower(filepath.Clean(path))
}
