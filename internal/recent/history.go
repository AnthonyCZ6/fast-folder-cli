package recent

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/humanize"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
)

// History es el historial de Windows ya leído: las jump lists y lo que hace
// falta para filtrarlas por programa y mostrarlas. Lo comparten la CLI y el
// modo interactivo.
type History struct {
	Lists   []jumplist.List
	Damaged int                // jump lists dañadas o ilegibles, omitidas
	Off     string             // por qué Windows no guarda el historial; "" si lo guarda
	Names   map[uint64]AppName // el programa de cada jump list que se identifica
}

// Load lee el historial de Windows (ver LoadLists), si Windows lo está
// guardando y el nombre de los programas, que se consulta al registro y al
// menú Inicio.
func Load() (History, error) {
	h, err := LoadLists()
	if err != nil {
		return History{}, err
	}
	h.Off, h.Names = jumplist.HistoryOff(), AppNames()
	return h, nil
}

// LoadLists lee solo las jump lists, de jumplist.Dir. Que no exista la
// carpeta (un Windows que nunca guardó nada) no es un error: el historial
// queda vacío.
func LoadLists() (History, error) {
	dir, err := jumplist.Dir()
	if err != nil {
		return History{}, fmt.Errorf("no se encontró el historial de Windows: %w", err)
	}
	lists, damaged, err := jumplist.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return History{}, fmt.Errorf("no se pudo leer el historial de Windows (%s): %w", dir, err)
	}
	return History{Lists: lists, Damaged: damaged}, nil
}

// AppsNamed devuelve las jump lists de los programas cuyo nombre o alias
// coincide con term (--con) y esos nombres, para las cabeceras. Si no
// coincide ninguno, el error dice qué programas tienen historial.
func (h History) AppsNamed(term string) (map[uint64]bool, string, error) {
	m, err := search.NewMatcher(term)
	if err != nil {
		return nil, "", err
	}
	ids := map[uint64]bool{}
	var matched []string
	for id, n := range h.Names {
		if n.Matches(m.Match) {
			ids[id] = true
			matched = append(matched, n.Name)
		}
	}
	if len(ids) == 0 {
		var withHistory []string
		for _, l := range h.Lists {
			if n, ok := h.Names[l.AppID]; ok && len(l.Entries) > 0 {
				withHistory = append(withHistory, n.Name)
			}
		}
		msg := fmt.Sprintf("no se reconoce el programa %q", term)
		if withHistory = sortedUnique(withHistory); len(withHistory) > 0 {
			msg += ". Tienen historial: " + strings.Join(withHistory, ", ")
		}
		return nil, "", errors.New(msg)
	}
	return ids, strings.Join(sortedUnique(matched), ", "), nil
}

// AppsOf devuelve los nombres de los programas con los que se abrió algo en
// la carpeta f, sin repetir y por orden alfabético. Los que no se pueden
// identificar no aparecen.
func (h History) AppsOf(f Folder) []string {
	var out []string
	for _, id := range f.AppIDs {
		if n, ok := h.Names[id]; ok {
			out = append(out, n.Name)
		}
	}
	return sortedUnique(out)
}

func sortedUnique(s []string) []string {
	slices.Sort(s)
	return slices.Compact(s)
}

// maxAppNames es cuántos programas se nombran como mucho en Summary.
const maxAppNames = 3

// Summary resume la carpeta para mostrarla junto a su ruta: cuándo se usó,
// cuántos archivos se abrieron en ella y con qué programas (apps, de AppsOf):
// "hace 10 min · 2 archivos · Word".
func (f Folder) Summary(apps []string, now time.Time) string {
	parts := []string{humanize.Ago(f.LastUsed, now)}
	if f.Items > 0 {
		parts = append(parts, humanize.Count(int64(f.Items), "archivo", "archivos"))
	}
	if len(apps) > maxAppNames {
		apps = append(apps[:maxAppNames:maxAppNames], "…")
	}
	if len(apps) > 0 {
		parts = append(parts, strings.Join(apps, ", "))
	}
	return strings.Join(parts, " · ")
}
