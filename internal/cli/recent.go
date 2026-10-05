package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strconv"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/humanize"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/pathutil"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/recent"
)

// historyOff dice por qué Windows no guarda el historial de recientes ("" si
// lo guarda). Las pruebas lo reemplazan para no depender del equipo.
var historyOff = jumplist.HistoryOff

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
	if reason := historyOff(); reason != "" {
		fmt.Fprintf(stderr, "aviso: Windows no está guardando las carpetas recientes: %s. "+
			"Solo aparece lo que se guardó antes de desactivarlo.\n", reason)
	}
	folders := recent.Find(lists, opts)

	out := bufio.NewWriter(stdout)
	p := newPrinter(out, supportsColor(stdout) && !cfg.json)
	p.json = cfg.json
	p.recentHeader(q.Describe(), opts.Within, cfg.all)
	now := time.Now()
	for i, f := range folders {
		p.recentFolder(int64(i+1), f, now)
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

// jsonRecent es una carpeta reciente en la salida --json.
type jsonRecent struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	LastUsed string `json:"last_used,omitempty"` // RFC 3339, hora local
	Files    int    `json:"files"`
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

// recentFolder imprime una carpeta reciente: el índice, la ruta con el
// nombre resaltado y cuándo se usó (y cuántos archivos se abrieron en ella).
func (p *printer) recentFolder(n int64, f recent.Folder, now time.Time) {
	if p.json {
		j := jsonRecent{Path: f.Path, Name: filepath.Base(f.Path), Files: f.Items}
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
	fmt.Fprintf(p.w, "  %s %s%s\n",
		p.paint("["+strconv.FormatInt(n, 10)+"]", ansiGreen),
		p.path(f.Path),
		p.tag(when),
	)
}
