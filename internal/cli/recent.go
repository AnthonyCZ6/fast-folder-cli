package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/humanize"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/pathutil"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/recent"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
)

// historyOff dice por qué Windows no guarda el historial de recientes ("" si
// lo guarda) y appNames da el nombre de los programas de cada jump list. Las
// pruebas los reemplazan para no depender del equipo.
var (
	historyOff = jumplist.HistoryOff
	appNames   = recent.AppNames
)

// runRecent muestra las carpetas usadas hace poco que coinciden con q, de la
// más reciente a la más antigua, y con --open abre la primera. Devuelve el
// código de salida.
func runRecent(cfg config, q query.Query, stdout, stderr io.Writer) int {
	start := time.Now()
	opts := recent.Options{
		Match:         q.Match,
		After:         q.Period.After,
		Before:        q.Period.Before,
		Exclude:       cfg.exclude,
		IncludeHidden: cfg.all,
	}
	if cfg.rootSet {
		root, err := pathutil.Resolve(cfg.root)
		if err != nil {
			return usageError(stderr, err)
		}
		opts.Within = root
	}

	dir, err := jumplist.Dir()
	if err != nil {
		fmt.Fprintf(stderr, "error: no se encontró el historial de Windows: %v\n", err)
		return exitUsage
	}
	lists, damaged, err := jumplist.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(stderr, "error: no se pudo leer el historial de Windows (%s): %v\n", dir, err)
		return exitUsage
	}
	names := appNames()
	desc := q.Describe()
	if cfg.with != "" {
		ids, label, err := appsNamed(cfg.with, names, lists)
		if err != nil {
			return usageError(stderr, err)
		}
		opts.AppIDs = ids
		desc += " (" + label + ")"
	}
	if reason := historyOff(); reason != "" {
		fmt.Fprintf(stderr, "aviso: Windows no está guardando las carpetas recientes: %s. "+
			"Solo aparece lo que se guardó antes de desactivarlo.\n", reason)
	}
	folders := recent.Find(lists, opts)

	out := bufio.NewWriter(stdout)
	p := newPrinter(out, supportsColor(stdout) && !cfg.json)
	p.json = cfg.json
	p.recentHeader(desc, opts.Within, cfg.all)
	now := time.Now()
	for i, f := range folders {
		p.recentFolder(int64(i+1), f, appsOf(f, names), now)
	}

	sum := summary{query: q, found: int64(len(folders)), lists: len(lists), damaged: damaged}
	if cfg.open && len(folders) > 0 {
		sum.opened = folders[0].Path
		sum.openErr = openExplorer(folders[0].Path)
	}
	sum.elapsed = time.Since(start)
	p.summary(sum)
	out.Flush()

	if sum.openErr != nil {
		fmt.Fprintf(stderr, "error: no se pudo abrir el Explorador: %v\n", sum.openErr)
	}
	return exitCode(sum)
}

// appsNamed devuelve las jump lists de los programas cuyo nombre coincide
// con term (--con) y esos nombres, para la cabecera. Si no coincide ninguno,
// el error dice qué programas tienen historial.
func appsNamed(term string, names map[uint64]recent.AppName, lists []jumplist.List) (map[uint64]bool, string, error) {
	m, err := search.NewMatcher(term)
	if err != nil {
		return nil, "", err
	}
	ids := map[uint64]bool{}
	var matched []string
	for id, n := range names {
		if n.Matches(m.Match) {
			ids[id] = true
			matched = append(matched, n.Name)
		}
	}
	if len(ids) == 0 {
		var withHistory []string
		for _, l := range lists {
			if n, ok := names[l.AppID]; ok && len(l.Entries) > 0 {
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

// appsOf devuelve los nombres de los programas con los que se abrió algo en
// la carpeta f, sin repetir y por orden alfabético. Los que no se pueden
// identificar no aparecen.
func appsOf(f recent.Folder, names map[uint64]recent.AppName) []string {
	var out []string
	for _, id := range f.AppIDs {
		if n, ok := names[id]; ok {
			out = append(out, n.Name)
		}
	}
	return sortedUnique(out)
}

func sortedUnique(s []string) []string {
	slices.Sort(s)
	return slices.Compact(s)
}

// jsonRecent es una carpeta reciente en la salida --json.
type jsonRecent struct {
	Path     string   `json:"path"`
	Name     string   `json:"name"`
	LastUsed string   `json:"last_used,omitempty"` // RFC 3339, hora local
	Files    int      `json:"files"`
	Apps     []string `json:"apps,omitempty"` // programas con los que se abrió algo en ella
}

// recentHeader imprime qué carpetas recientes se buscan (desc) y, si se
// indicó con -p, dentro de qué carpeta.
func (p *printer) recentHeader(desc, within string, all bool) {
	if p.json {
		return
	}
	where := "en el historial de Windows"
	if within != "" {
		where += " dentro de " + within
	}
	hidden := "excluidas"
	if all {
		hidden = "incluidas"
	}
	fmt.Fprintf(p.w, "%s %s %s %s\n\n",
		p.paint("Buscando", ansiBold, ansiCyan),
		p.paint(desc, ansiBold),
		where,
		p.paint("(ocultas/sistema: "+hidden+")", ansiDim),
	)
}

// maxAppNames es cuántos programas se nombran como mucho junto a una carpeta.
const maxAppNames = 3

// recentFolder imprime una carpeta reciente: el índice, la ruta con el
// nombre resaltado, cuándo se usó, cuántos archivos se abrieron en ella y con
// qué programas (apps).
func (p *printer) recentFolder(n int64, f recent.Folder, apps []string, now time.Time) {
	if p.json {
		j := jsonRecent{Path: f.Path, Name: filepath.Base(f.Path), Files: f.Items, Apps: apps}
		if !f.LastUsed.IsZero() {
			j.LastUsed = f.LastUsed.In(now.Location()).Format(time.RFC3339)
		}
		p.jsonLine(j)
		return
	}
	when := humanize.Ago(f.LastUsed, now)
	if f.Items > 0 {
		when += " · " + humanize.Count(int64(f.Items), "archivo", "archivos")
	}
	if len(apps) > maxAppNames {
		apps = append(apps[:maxAppNames:maxAppNames], "…")
	}
	if len(apps) > 0 {
		when += " · " + strings.Join(apps, ", ")
	}
	fmt.Fprintf(p.w, "  %s %s%s\n",
		p.paint("["+strconv.FormatInt(n, 10)+"]", ansiGreen),
		p.path(f.Path),
		p.tag(when),
	)
}
