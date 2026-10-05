package cli

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/pathutil"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/recent"
)

// loadHistory lee el historial de Windows. Las pruebas lo reemplazan para
// no depender del equipo.
var loadHistory = recent.Load

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

	h, err := loadHistory()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitUsage
	}
	desc := q.Describe()
	if cfg.with != "" {
		ids, label, err := h.AppsNamed(cfg.with)
		if err != nil {
			return usageError(stderr, err)
		}
		opts.AppIDs = ids
		desc += " (" + label + ")"
	}
	if h.Off != "" {
		fmt.Fprintf(stderr, "aviso: Windows no está guardando las carpetas recientes: %s. "+
			"Solo aparece lo que se guardó antes de desactivarlo.\n", h.Off)
	}
	folders := recent.Find(h.Lists, opts)

	out := bufio.NewWriter(stdout)
	p := newPrinter(out, supportsColor(stdout) && !cfg.json)
	p.json = cfg.json
	p.recentHeader(desc, opts.Within, cfg.all)
	now := time.Now()
	for i, f := range folders {
		p.recentFolder(int64(i+1), f, h.AppsOf(f), now)
	}

	sum := summary{query: q, found: int64(len(folders)), lists: len(h.Lists), damaged: h.Damaged}
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
	fmt.Fprintf(p.w, "  %s %s%s\n",
		p.paint("["+strconv.FormatInt(n, 10)+"]", ansiGreen),
		p.path(f.Path),
		p.tag(f.Summary(apps, now)),
	)
}
