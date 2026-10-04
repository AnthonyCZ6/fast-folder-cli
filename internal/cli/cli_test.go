package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/apps"
	prefs "github.com/AnthonyCZ6/fast-folder-cli/internal/config"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/search"
)

// Las pruebas no leen el archivo de configuración de quien las ejecuta: cada
// prueba que lo necesita define el suyo con stubPrefs. (TestMain está en
// bench_test.go, que no puede depender de este archivo.)
func init() {
	loadPrefs = func() (prefs.Config, error) { return prefs.Config{}, nil }
}

// stubPrefs hace que el archivo de configuración contenga p (o falle con err).
func stubPrefs(t *testing.T, p prefs.Config, err error) {
	t.Helper()
	prev := loadPrefs
	loadPrefs = func() (prefs.Config, error) { return p, err }
	t.Cleanup(func() { loadPrefs = prev })
}

// La ubicación, las ocultas y las exclusiones del archivo se aplican si las
// banderas no dicen otra cosa; --exclude se suma a las del archivo.
func TestRunAppliesPrefs(t *testing.T) {
	root := makeTree(t, []string{"informe", "node_modules/informe", "venv/informe", ".oculta/informe"}, nil)
	stubPrefs(t, prefs.Config{Root: root, Exclude: []string{"node_modules"}, Hidden: true}, nil)

	out := runOK(t, "informe", "--exclude", "venv")
	for _, want := range []string{
		filepath.Join(root, "informe"),
		filepath.Join(root, ".oculta", "informe"),
		"Resultados : 2 carpetas encontradas",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("la salida no contiene %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "node_modules") || strings.Contains(out, "venv") {
		t.Errorf("las carpetas excluidas no deberían aparecer:\n%s", out)
	}

	// -p tiene prioridad sobre la ubicación del archivo.
	other := makeTree(t, []string{"informe-b"}, nil)
	if out := runOK(t, "informe", "-p", other); !strings.Contains(out, filepath.Join(other, "informe-b")) {
		t.Errorf("-p debería mandar sobre el archivo:\n%s", out)
	}
}

// Un archivo de configuración con errores se avisa y se ignora.
func TestRunWarnsAboutBadPrefs(t *testing.T) {
	stubPrefs(t, prefs.Config{}, errors.New(`config.toml: clave desconocida "x"`))
	root := makeTree(t, []string{"informe"}, nil)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"informe", "-p", root}, &stdout, &stderr, "test"); code != exitFound {
		t.Fatalf("código = %d; stderr: %s", code, stderr.String())
	}
	if want := `aviso: se ignora la configuración: config.toml: clave desconocida "x"`; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

// --config crea la plantilla la primera vez y después solo muestra la ruta.
// Sin terminal no intenta abrir el editor.
func TestRunConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fast-folder-cli", "config.toml")
	t.Setenv(prefs.EnvVar, path)
	opened := false
	prev := openConfigFile
	openConfigFile = func(string, string) error { opened = true; return nil }
	t.Cleanup(func() { openConfigFile = prev })

	if out := runOK(t, "--config"); out != "Configuración creada con la plantilla: "+path+"\n" {
		t.Errorf("primera vez: %q", out)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != prefs.Template {
		t.Errorf("no se escribió la plantilla: %v", err)
	}
	if out := runOK(t, "--config"); out != "Configuración: "+path+"\n" {
		t.Errorf("segunda vez: %q", out)
	}
	if opened {
		t.Error("sin terminal no debería abrir el editor")
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		args []string
		want config
	}{
		{[]string{"-n", "proyecto"}, config{term: "proyecto", root: defaultRoot}},
		{[]string{"--name=proyecto", "--path", "D:\\", "--all", "--open"}, config{term: "proyecto", root: "D:\\", all: true, open: true}},
		{[]string{"-a", "-o", "-p", "%APPDATA%", "-n", "x"}, config{term: "x", root: "%APPDATA%", all: true, open: true}},
		// Término posicional con opciones antes y después.
		{[]string{"-a", "mis", "proyectos", "-o"}, config{term: "mis proyectos", root: defaultRoot, all: true, open: true}},
		// Tras "--" todo es parte del término.
		{[]string{"--", "-raro"}, config{term: "-raro", root: defaultRoot}},
		{[]string{"-v"}, config{root: defaultRoot, version: true}},
		{[]string{"--projects", "api"}, config{term: "api", root: defaultRoot, projects: true}},
		{[]string{"--apps", "chrome", "-o"}, config{term: "chrome", root: defaultRoot, apps: true, open: true}},
		{[]string{"x", "--exclude", "node_modules, venv,,"}, config{term: "x", root: defaultRoot, exclude: []string{"node_modules", "venv"}}},
		{[]string{"--config"}, config{root: defaultRoot, editConf: true}},
		{[]string{"-m", "hoy", "-s", "x"}, config{term: "x", root: defaultRoot, modified: "hoy", size: true}},
		{[]string{"--modified=semana", "--size", "--cd-file", "elegida.txt"}, config{root: defaultRoot, modified: "semana", size: true, cdFile: "elegida.txt"}},
	}
	for _, tt := range tests {
		got, err := parseArgs(tt.args)
		if err != nil {
			t.Errorf("parseArgs(%q): %v", tt.args, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parseArgs(%q) = %+v, want %+v", tt.args, got, tt.want)
		}
	}
}

func TestParseArgsErrors(t *testing.T) {
	tests := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"-x"}, "opción desconocida: -x"},
		{[]string{"-n"}, "falta el valor de la opción -n"},
		{[]string{"-n", "uno", "dos"}, "argumentos inesperados: dos"},
		// Opciones que no se aplican a las apps.
		{[]string{"--apps", "--projects"}, "--apps y --projects no se pueden usar juntas"},
		{[]string{"--apps", "-m", "hoy"}, "--modified no se aplica a las apps"},
		{[]string{"--apps", "x", "-s"}, "--size no se aplica a las apps"},
		{[]string{"--apps", "-a"}, "--all no se aplica a las apps"},
		{[]string{"--apps", "--exclude", "x"}, "--exclude no se aplica a las apps"},
		{[]string{"--apps", "-p", "D:"}, "--path no se aplica a las apps: se buscan entre los programas instalados"},
	}
	for _, tt := range tests {
		_, err := parseArgs(tt.args)
		if err == nil || err.Error() != tt.wantErr {
			t.Errorf("parseArgs(%q) error = %v, want %q", tt.args, err, tt.wantErr)
		}
	}
}

func TestRunEndToEnd(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"docs/Proyecto-A", "src/proyecto-b", "otros"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-n", "proyecto", "-p", root}, &stdout, &stderr, "test")
	if code != exitFound {
		t.Fatalf("código de salida = %d, want %d; stderr: %s", code, exitFound, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		filepath.Join(root, "docs", "Proyecto-A"),
		filepath.Join(root, "src", "proyecto-b"),
		"Resultados : 2 carpetas encontradas",
		"Analizadas : 6 carpetas", // raíz, docs, src, otros y las dos coincidencias
		"Tiempo     : ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("la salida no contiene %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("la salida redirigida no debe contener colores ANSI:\n%s", out)
	}
}

func TestRunExitCodes(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		args []string
		want int
	}{
		{[]string{"-n", "nada-coincide", "-p", root}, exitNoMatch},
		{[]string{"-n", "x", "-p", filepath.Join(root, "no-existe")}, exitUsage},
		{[]string{"-n", "[roto", "-p", root}, exitUsage},
		{[]string{}, exitUsage},
		{[]string{"-a"}, exitUsage}, // sin término y sin terminal no hay modo interactivo
		{[]string{"-m", "mañana", "-p", root}, exitUsage},
		{[]string{"--cd-file", "elegida.txt"}, exitUsage},
		{[]string{"--projects", "-p", root}, exitNoMatch},
		{[]string{"--help"}, exitFound},
		{[]string{"--version"}, exitFound},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		if got := Run(tt.args, &stdout, &stderr, "test"); got != tt.want {
			t.Errorf("Run(%q) = %d, want %d\nstdout: %s\nstderr: %s", tt.args, got, tt.want, stdout.String(), stderr.String())
		}
	}
}

// makeTree crea las carpetas indicadas (rutas relativas con "/") y los
// archivos de files (con el tamaño indicado) bajo un directorio temporal.
func makeTree(t *testing.T, dirs []string, files map[string]int) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for f, size := range files {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(f)), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// runOK ejecuta la CLI, comprueba el código de salida y devuelve la salida.
func runOK(t *testing.T, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := Run(args, &stdout, &stderr, "test"); code != exitFound {
		t.Fatalf("Run(%q) = %d, want %d\nstdout: %s\nstderr: %s", args, code, exitFound, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func TestRunIgnoresAccents(t *testing.T) {
	root := makeTree(t, []string{"Música/Canción", "otros"}, nil)
	out := runOK(t, "cancion", "-p", root)
	if !strings.Contains(out, filepath.Join(root, "Música", "Canción")) {
		t.Errorf("no encontró la carpeta con acentos:\n%s", out)
	}
}

func TestRunProjects(t *testing.T) {
	root := makeTree(t, []string{"web/src", "api", "notas"}, map[string]int{
		"web/package.json": 0,
		"api/go.mod":       0,
	})
	out := runOK(t, "--projects", "-p", root)
	for _, want := range []string{
		filepath.Join(root, "web") + "  (Node.js)",
		filepath.Join(root, "api") + "  (Go)",
		"Buscando proyectos en ",
		"Resultados : 2 proyectos encontrados",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("la salida no contiene %q:\n%s", want, out)
		}
	}
}

func TestRunSize(t *testing.T) {
	root := makeTree(t,
		[]string{"app/node_modules/lib/node_modules", "web/node_modules"},
		map[string]int{"app/node_modules/lib/index.js": 1000, "web/node_modules/big.js": 3000},
	)
	out := runOK(t, "node_modules", "--size", "-p", root)

	web := strings.Index(out, "2.9 KB  "+filepath.Join(root, "web", "node_modules"))
	app := strings.Index(out, "1,000 bytes  "+filepath.Join(root, "app", "node_modules"))
	if web < 0 || app < 0 || web > app {
		t.Errorf("se esperaban web (2.9 KB) y luego app (1,000 bytes):\n%s", out)
	}
	if strings.Contains(out, filepath.Join("lib", "node_modules")) {
		t.Errorf("no debería entrar en las carpetas encontradas:\n%s", out)
	}
	if !strings.Contains(out, "Tamaño     : 3.9 KB en total (2 archivos)") {
		t.Errorf("falta el total:\n%s", out)
	}
}

func TestRunModified(t *testing.T) {
	root := makeTree(t, []string{"viejo/informe", "nuevo/informe"}, nil)
	old := time.Now().AddDate(0, 0, -30)
	if err := os.Chtimes(filepath.Join(root, "viejo", "informe"), old, old); err != nil {
		t.Fatal(err)
	}

	out := runOK(t, "informe", "-m", "semana", "-p", root)
	if !strings.Contains(out, filepath.Join(root, "nuevo", "informe")) || strings.Contains(out, filepath.Join(root, "viejo", "informe")) {
		t.Errorf("solo debería aparecer nuevo/informe:\n%s", out)
	}
	if !strings.Contains(out, `Buscando carpetas "informe" modificadas en los últimos 7 días en `) {
		t.Errorf("cabecera inesperada:\n%s", out)
	}
}

// Un término o un periodo formados solo por espacios no cuentan como vacíos:
// son un error de uso, con el mensaje del término o del periodo.
func TestRunRejectsBlankTermAndPeriod(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		args    []string
		wantErr string
	}{
		{[]string{"   ", "-p", root}, "error: el término de búsqueda está vacío\n"},
		{[]string{"x", "-m", " ", "-p", root}, "error: periodo no válido "},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		if code := Run(tt.args, &stdout, &stderr, "test"); code != exitUsage {
			t.Errorf("Run(%q) = %d, want %d", tt.args, code, exitUsage)
		}
		if !strings.HasPrefix(stderr.String(), tt.wantErr) {
			t.Errorf("Run(%q) stderr = %q, want que empiece por %q", tt.args, stderr.String(), tt.wantErr)
		}
	}
}

// stubExplorer sustituye el Explorador durante la prueba: cada llamada
// devuelve err y queda registrada en la lista que se devuelve. Cambia una
// variable del paquete, así que estas pruebas no pueden usar t.Parallel.
func stubExplorer(t *testing.T, err error) *[]string {
	t.Helper()
	var opened []string
	prev := openExplorer
	openExplorer = func(path string) error {
		opened = append(opened, path)
		return err
	}
	t.Cleanup(func() { openExplorer = prev })
	return &opened
}

func TestRunOpenOpensOnlyTheFirstMatch(t *testing.T) {
	root := makeTree(t, []string{"a/informe", "b/informe"}, nil)
	opened := stubExplorer(t, nil)

	out := runOK(t, "informe", "-o", "-p", root)
	if len(*opened) != 1 {
		t.Fatalf("se abrieron %d carpetas, want 1: %q", len(*opened), *opened)
	}
	first := (*opened)[0]
	if first != filepath.Join(root, "a", "informe") && first != filepath.Join(root, "b", "informe") {
		t.Errorf("se abrió %q, que no es una coincidencia", first)
	}
	if !strings.Contains(out, "Explorador : "+first+"\n") {
		t.Errorf("el resumen no indica la carpeta abierta:\n%s", out)
	}
}

func TestRunOpenReportsErrors(t *testing.T) {
	root := makeTree(t, []string{"informe"}, nil)
	stubExplorer(t, errors.New("sin Explorador"))

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"informe", "-o", "-p", root}, &stdout, &stderr, "test"); code != exitFound {
		t.Fatalf("Run = %d, want %d; stderr: %s", code, exitFound, stderr.String())
	}
	if want := "error: no se pudo abrir el Explorador: sin Explorador\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
	if strings.Contains(stdout.String(), "Explorador :") {
		t.Errorf("el resumen no debe indicar una carpeta abierta si falló:\n%s", stdout.String())
	}
}

func TestRunWithoutOpenDoesNotOpen(t *testing.T) {
	root := makeTree(t, []string{"informe"}, nil)
	opened := stubExplorer(t, nil)

	runOK(t, "informe", "-p", root)
	if len(*opened) != 0 {
		t.Errorf("sin -o no debe abrirse nada; se abrió %q", *opened)
	}
}

// runInterrupted busca node_modules en root con runSearch y ctx, que la
// prueba cancela a mitad de camino. Comprueba que termina como interrumpida y
// sin mostrar tamaños, y devuelve la salida.
func runInterrupted(t *testing.T, ctx context.Context, cfg config, root string) string {
	t.Helper()
	q, err := query.New("node_modules", query.Folders, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runSearch(ctx, cfg, q, root, &stdout, &stderr); code != exitInterrupted {
		t.Fatalf("runSearch = %d, want %d; stderr: %s", code, exitInterrupted, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "bytes") || strings.Contains(out, "Tamaño") {
		t.Errorf("interrumpida, no debe mostrar tamaños (estarían incompletos):\n%s", out)
	}
	return out
}

// Con --size, si Ctrl+C llega durante la búsqueda, se listan sin tamaño las
// carpetas ya encontradas en lugar de ninguna.
func TestRunSizeInterruptedWhileSearching(t *testing.T) {
	root := makeTree(t, []string{"a/node_modules", "b/node_modules"}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// --open llama a openExplorer en cuanto aparece la primera carpeta: ahí
	// se simula Ctrl+C.
	prev := openExplorer
	openExplorer = func(string) error { cancel(); return nil }
	t.Cleanup(func() { openExplorer = prev })

	out := runInterrupted(t, ctx, config{size: true, open: true}, root)
	if !strings.Contains(out, "  [1] ") {
		t.Errorf("no se listó ninguna de las carpetas encontradas:\n%s", out)
	}
}

// interruptedSizer simula Ctrl+C mientras se miden las carpetas: la primera
// queda a medias (100 bytes) y las demás sin medir, como con search.Sizer.
type interruptedSizer struct {
	cancel context.CancelFunc
	added  int
}

func (s *interruptedSizer) Add(string) { s.added++ }

func (s *interruptedSizer) Wait() []search.SizeInfo {
	s.cancel()
	infos := make([]search.SizeInfo, s.added)
	if s.added > 0 {
		infos[0] = search.SizeInfo{Bytes: 100, Files: 1}
	}
	return infos
}

// Con --size, si Ctrl+C llega mientras se miden las carpetas, se listan todas
// sin tamaño en lugar de con tamaños a medias o a cero.
func TestRunSizeInterruptedWhileMeasuring(t *testing.T) {
	root := makeTree(t, []string{"a/node_modules", "b/node_modules"}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	prev := newSizer
	newSizer = func(context.Context) folderSizer { return &interruptedSizer{cancel: cancel} }
	t.Cleanup(func() { newSizer = prev })

	out := runInterrupted(t, ctx, config{size: true}, root)
	for _, dir := range []string{"a", "b"} {
		if !strings.Contains(out, "] "+filepath.Join(root, dir, "node_modules")+"\n") {
			t.Errorf("falta %s/node_modules en la lista:\n%s", dir, out)
		}
	}
}

// FuzzParseArgs comprueba que ninguna combinación de argumentos provoca un
// pánico: cualquier entrada debe terminar en una configuración o en un error.
func FuzzParseArgs(f *testing.F) {
	for _, seed := range []string{
		"-n proyecto",
		"-a mis proyectos -o",
		"-- -raro",
		"--projects -m semana -s",
		"--path= -n",
		"--apps chrome -o",
		"-x",
		"--",
		"",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, line string) {
		_, _ = parseArgs(strings.Fields(line))
	})
}

var testApps = []apps.App{
	{
		Name:    "Google Chrome",
		Dir:     `C:\Program Files\Google\Chrome\Application`,
		Exe:     `C:\Program Files\Google\Chrome\Application\chrome.exe`,
		Version: "120.0",
	},
	{Name: "Paint.NET", Dir: `C:\Program Files\paint.net`},
}

// stubApps hace que --apps encuentre las aplicaciones de list (o falle con
// err) y devuelve las ubicaciones que se abren, como "carpeta|ejecutable".
func stubApps(t *testing.T, list []apps.App, err error) *[]string {
	t.Helper()
	prevFind, prevShow := findApps, showApp
	findApps = func(match func(string) bool) ([]apps.App, error) {
		var found []apps.App
		for _, a := range list {
			if match(a.Name) {
				found = append(found, a)
			}
		}
		return found, err
	}
	var shown []string
	showApp = func(dir, exe string) error {
		shown = append(shown, dir+"|"+exe)
		return nil
	}
	t.Cleanup(func() { findApps, showApp = prevFind, prevShow })
	return &shown
}

func TestRunApps(t *testing.T) {
	shown := stubApps(t, testApps, nil)

	out := runOK(t, "--apps", "chrome", "-o")
	for _, want := range []string{
		`Buscando apps "chrome" entre los programas instalados`,
		`  [1] Google Chrome  C:\Program Files\Google\Chrome\Application  (120.0)` + "\n",
		"Resultados : 1 app encontrada",
		`Explorador : C:\Program Files\Google\Chrome\Application` + "\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("la salida no contiene %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Analizadas") || strings.Contains(out, "Paint.NET") {
		t.Errorf("salida inesperada:\n%s", out)
	}
	if want := []string{testApps[0].Dir + "|" + testApps[0].Exe}; !slices.Equal(*shown, want) {
		t.Errorf("se abrió %q, want %q", *shown, want)
	}
}

func TestRunAppsWithoutTermListsAll(t *testing.T) {
	shown := stubApps(t, testApps, nil)
	out := runOK(t, "--apps")
	if !strings.Contains(out, "[2] Paint.NET") || !strings.Contains(out, "Resultados : 2 apps encontradas") {
		t.Errorf("deberían aparecer todas las apps:\n%s", out)
	}
	if len(*shown) != 0 {
		t.Errorf("sin -o no debería abrirse nada: %q", *shown)
	}
}

func TestRunAppsExitCodes(t *testing.T) {
	stubApps(t, testApps, nil)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--apps", "nada"}, &stdout, &stderr, "test"); code != exitNoMatch {
		t.Errorf("sin coincidencias: código = %d, want %d", code, exitNoMatch)
	}

	stubApps(t, nil, errors.New("no se pudo leer el registro"))
	stdout.Reset()
	if code := Run([]string{"--apps", "chrome"}, &stdout, &stderr, "test"); code != exitUsage {
		t.Errorf("con error: código = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "error: no se pudo leer el registro") {
		t.Errorf("el error no se mostró: %q", stderr.String())
	}
}
