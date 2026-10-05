// Package query representa lo que el usuario quiere encontrar —un término,
// si busca carpetas, proyectos o apps y un periodo de modificación— de la
// misma forma en la línea de comandos y en el modo interactivo: lo valida, lo
// describe y lo convierte en las opciones de una búsqueda.
package query

import (
	"errors"
	"strings"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/humanize"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/period"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
)

// ErrEmpty indica que no hay nada que buscar: ni término, ni proyectos, ni
// periodo de modificación.
var ErrEmpty = errors.New("no hay nada que buscar")

// Kind es el tipo de lo que se busca.
type Kind int

const (
	// Folders busca carpetas cuyo nombre coincide con el término.
	Folders Kind = iota
	// Projects busca carpetas de proyectos (.git, go.mod, package.json...).
	Projects
	// Apps busca aplicaciones instaladas por su nombre (paquete apps).
	Apps
	// Recent busca entre las carpetas usadas hace poco (paquete recent): el
	// periodo se refiere a cuándo se usaron, no a cuándo se modificaron.
	Recent
)

// errAppsPeriod indica que se pidió filtrar apps por fecha de modificación.
var errAppsPeriod = errors.New("las apps no se pueden filtrar por fecha de modificación")

// Query es una búsqueda ya validada.
type Query struct {
	Term   string       // término tal como se recibió; vacío: cualquier nombre
	Kind   Kind         // qué se busca
	Period period.Range // intervalo de modificación; cero: cualquier fecha

	matcher *search.Matcher // nil si no hay término
}

// New valida una búsqueda. modified es un periodo como los de --modified
// ("hoy", "semana", "7d"...) o "" para cualquier fecha, y now la referencia de
// los periodos relativos.
//
// No recorta espacios: un término o un periodo formado solo por espacios no
// cuenta como vacío, sino que es un error (el modo interactivo recorta el
// término antes de llamar a New).
//
// Devuelve ErrEmpty si no hay nada que buscar, un error si se piden apps con
// un periodo (las apps no tienen fecha de modificación), un error que cumple
// errors.Is(err, period.ErrInvalid) si el periodo no es válido y, si no, el
// error del término (por ejemplo, un patrón mal formado).
func New(term string, kind Kind, modified string, now time.Time) (Query, error) {
	if term == "" && kind == Folders && modified == "" {
		return Query{}, ErrEmpty
	}
	if kind == Apps && modified != "" {
		return Query{}, errAppsPeriod
	}

	q := Query{Term: term, Kind: kind}
	if term != "" {
		m, err := search.NewMatcher(term)
		if err != nil {
			return Query{}, err
		}
		q.matcher = m
	}
	if modified != "" {
		r, err := period.Parse(modified, now)
		if err != nil {
			return Query{}, err
		}
		q.Period = r
	}
	return q, nil
}

// Options devuelve las opciones para ejecutar la búsqueda en root.
func (q Query) Options(root string, includeHidden bool) search.Options {
	return search.Options{
		Root:           root,
		Matcher:        q.matcher,
		IncludeHidden:  includeHidden,
		Projects:       q.Kind == Projects,
		ModifiedAfter:  q.Period.After,
		ModifiedBefore: q.Period.Before,
	}
}

// Describe resume lo que se busca, en minúsculas, para las cabeceras:
// `"tesis"`, `proyectos "api"`, `carpetas modificadas hoy`...
func (q Query) Describe() string {
	var parts []string
	switch {
	case q.Kind == Projects:
		parts = append(parts, "proyectos")
	case q.Kind == Apps:
		parts = append(parts, "apps")
	case q.Kind == Recent:
		parts = append(parts, "carpetas recientes")
	case q.Term == "" || q.Period.Label != "":
		parts = append(parts, "carpetas")
	}
	if q.Term != "" {
		parts = append(parts, `"`+q.Term+`"`)
	}
	if q.Period.Label != "" {
		adjective := "modificadas"
		switch q.Kind {
		case Projects:
			adjective = "modificados"
		case Recent:
			adjective = "usadas"
		}
		parts = append(parts, adjective+" "+q.Period.Label)
	}
	return strings.Join(parts, " ")
}

// Found nombra n resultados de esta búsqueda: "1 carpeta encontrada",
// "3 proyectos encontrados"...
func (q Query) Found(n int64) string {
	switch q.Kind {
	case Projects:
		return humanize.Count(n, "proyecto encontrado", "proyectos encontrados")
	case Apps:
		return humanize.Count(n, "app encontrada", "apps encontradas")
	}
	return humanize.Count(n, "carpeta encontrada", "carpetas encontradas")
}

// Match indica si name coincide con el término; sin término, todos coinciden.
// Sirve para filtrar las apps por su nombre.
func (q Query) Match(name string) bool {
	return q.matcher == nil || q.matcher.Match(name)
}
