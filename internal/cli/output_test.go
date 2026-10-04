package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/apps"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
)

// Estas pruebas fijan el texto exacto que imprime printer, con y sin color:
// es lo que leen los scripts que procesan la salida de la CLI.

func TestPrinterHeader(t *testing.T) {
	tests := []struct {
		color, all bool
		want       string
	}{
		{false, false, `Buscando "tesis" en D:\ (ocultas/sistema: excluidas)` + "\n\n"},
		{false, true, `Buscando "tesis" en D:\ (ocultas/sistema: incluidas)` + "\n\n"},
		{true, false, "\x1b[1m\x1b[36mBuscando\x1b[0m \x1b[1m\"tesis\"\x1b[0m en D:\\ \x1b[2m(ocultas/sistema: excluidas)\x1b[0m\n\n"},
	}
	for _, tt := range tests {
		var b strings.Builder
		newPrinter(&b, tt.color).header(`"tesis"`, `D:\`, tt.all)
		if got := b.String(); got != tt.want {
			t.Errorf("header (color %v, all %v) = %q, want %q", tt.color, tt.all, got, tt.want)
		}
	}
}

func TestPrinterMatch(t *testing.T) {
	parent := filepath.Join("dev", "proyectos") + string(filepath.Separator)
	r := search.Result{Path: parent + "api", Project: "Go"}
	tests := []struct {
		color bool
		want  string
	}{
		{false, "  [3] " + parent + "api  (Go)\n"},
		{true, "  \x1b[32m[3]\x1b[0m " + parent + "\x1b[1m\x1b[32mapi\x1b[0m  \x1b[36m(Go)\x1b[0m\n"},
	}
	for _, tt := range tests {
		var b strings.Builder
		newPrinter(&b, tt.color).match(3, r)
		if got := b.String(); got != tt.want {
			t.Errorf("match (color %v) = %q, want %q", tt.color, got, tt.want)
		}
	}
}

func TestPrinterApp(t *testing.T) {
	tests := []struct {
		app   apps.App
		color bool
		want  string
	}{
		{apps.App{Name: "Paint.NET", Dir: `C:\paint.net`}, false, `  [2] Paint.NET  C:\paint.net` + "\n"},
		{apps.App{Name: "Git", Dir: `C:\Git`, Version: "2.51"}, false, `  [2] Git  C:\Git  (2.51)` + "\n"},
		{apps.App{Name: "Git", Dir: `C:\Git`, Version: "2.51"}, true,
			"  \x1b[32m[2]\x1b[0m \x1b[1m\x1b[32mGit\x1b[0m  C:\\Git  \x1b[36m(2.51)\x1b[0m\n"},
	}
	for _, tt := range tests {
		var b strings.Builder
		newPrinter(&b, tt.color).app(2, tt.app)
		if got := b.String(); got != tt.want {
			t.Errorf("app (color %v) = %q, want %q", tt.color, got, tt.want)
		}
	}
}

func TestPrinterSummary(t *testing.T) {
	folders, err := query.New("informe", query.Folders, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	projects, err := query.New("", query.Projects, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	rule := strings.Repeat("-", 64) + "\n"

	tests := []struct {
		name string
		sum  summary
		want string
	}{
		{
			name: "con resultados y Explorador",
			sum:  summary{query: folders, found: 2, scanned: 1234, denied: 5, opened: `C:\informe`, elapsed: 1500 * time.Millisecond},
			want: "\n" + rule +
				"Resultados : 2 carpetas encontradas\n" +
				"Analizadas : 1,234 carpetas (5 sin acceso, omitidas)\n" +
				"Explorador : C:\\informe\n" +
				"Tiempo     : 1,500 ms\n",
		},
		{
			name: "interrumpida sin resultados",
			sum:  summary{query: projects, scanned: 1, interrupted: true},
			want: rule +
				"Resultados : sin coincidencias\n" +
				"Analizadas : 1 carpeta\n" +
				"Estado     : búsqueda interrumpida (resultados parciales)\n" +
				"Tiempo     : 0 ms\n",
		},
		{
			name: "con tamaños",
			sum:  summary{query: folders, found: 1, scanned: 3, sized: true, bytes: 3000, files: 2},
			want: "\n" + rule +
				"Resultados : 1 carpeta encontrada\n" +
				"Tamaño     : 2.9 KB en total (2 archivos)\n" +
				"Analizadas : 3 carpetas\n" +
				"Tiempo     : 0 ms\n",
		},
	}
	for _, tt := range tests {
		var b strings.Builder
		newPrinter(&b, false).summary(tt.sum)
		if got := b.String(); got != tt.want {
			t.Errorf("%s: summary =\n%q\nwant\n%q", tt.name, got, tt.want)
		}
	}
}

func TestPrinterSummaryColorRule(t *testing.T) {
	q, err := query.New("informe", query.Folders, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	newPrinter(&b, true).summary(summary{query: q})
	if want := "\x1b[2m" + strings.Repeat("─", 64) + "\x1b[0m\n"; !strings.HasPrefix(b.String(), want) {
		t.Errorf("con color, la línea separadora debe ser %q:\n%q", want, b.String())
	}
}
