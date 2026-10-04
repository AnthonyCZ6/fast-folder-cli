// Package query representa lo que el usuario quiere encontrar —un término,
// si busca proyectos y un periodo de modificación— de la misma forma en la
// línea de comandos y en el modo interactivo: lo valida, lo describe y lo
// convierte en las opciones de una búsqueda.
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

// Query es una búsqueda ya validada.
type Query struct {
	Term     string       // término tal como se recibió; vacío: cualquier nombre
	Projects bool         // buscar proyectos en lugar de carpetas
	Period   period.Range // intervalo de modificación; cero: cualquier fecha

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
// Devuelve ErrEmpty si no hay nada que buscar, un error que cumple
// errors.Is(err, period.ErrInvalid) si el periodo no es válido y, si no, el
// error del término (por ejemplo, un patrón mal formado).
func New(term string, projects bool, modified string, now time.Time) (Query, error) {
	if term == "" && !projects && modified == "" {
		return Query{}, ErrEmpty
	}

	q := Query{Term: term, Projects: projects}
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
		Projects:       q.Projects,
		ModifiedAfter:  q.Period.After,
		ModifiedBefore: q.Period.Before,
	}
}

// Describe resume lo que se busca, en minúsculas, para las cabeceras:
// `"tesis"`, `proyectos "api"`, `carpetas modificadas hoy`...
func (q Query) Describe() string {
	var parts []string
	switch {
	case q.Projects:
		parts = append(parts, "proyectos")
	case q.Term == "" || q.Period.Label != "":
		parts = append(parts, "carpetas")
	}
	if q.Term != "" {
		parts = append(parts, `"`+q.Term+`"`)
	}
	if q.Period.Label != "" {
		adjective := "modificadas"
		if q.Projects {
			adjective = "modificados"
		}
		parts = append(parts, adjective+" "+q.Period.Label)
	}
	return strings.Join(parts, " ")
}

// Found nombra n resultados de esta búsqueda: "1 carpeta encontrada",
// "3 proyectos encontrados"...
func (q Query) Found(n int64) string {
	if q.Projects {
		return humanize.Count(n, "proyecto encontrado", "proyectos encontrados")
	}
	return humanize.Count(n, "carpeta encontrada", "carpetas encontradas")
}
