//go:build soak

package e2e

// Prueba larga (soak): durante FFC_SOAK_DURATION ejecuta sin parar, en un
// orden aleatorio que cambia en cada ronda, una mezcla de búsquedas sobre un
// árbol de unas 6.000 carpetas, errores de uso, la configuración y, en
// Windows, las apps, el modo interactivo y fcd. Registra el código de salida,
// el tiempo y la memoria máxima de cada ejecución y, al terminar, escribe en
// FFC_SOAK_REPORT_DIR un informe en markdown (informe.md) y un resumen
// (resumen.json) que junta .github/scripts/soak-report.sh. Solo se compila
// con la etiqueta soak:
//
//	go test -tags soak -run '^TestSoak$' -v -timeout 90m ./e2e
//
// Variables de entorno (todas opcionales):
//
//	FFC_SOAK_DURATION    duración, con el formato de Go (60m, 90s); por defecto, 2m
//	FFC_SOAK_EXE         programa a probar (el instalado); por defecto, el que compila TestMain
//	FFC_SOAK_VERSION     versión que debe mostrar --version (1.4.0)
//	FFC_SOAK_REPORT_DIR  carpeta del informe; por defecto, una temporal
//	FFC_SOAK_MACHINE     nombre de la máquina en el informe
//	FFC_SOAK_INSTALL     cómo se instaló el programa
//	FFC_SOAK_ARCH        arquitectura que debe tener el ejecutable (X64 o ARM64, como runner.arch)
//	FFC_SOAK_SEED        semilla del orden aleatorio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/benchtree"
)

const (
	// procTimeout es lo máximo que puede tardar una ejecución por línea de
	// comandos: más, y se da por colgada.
	procTimeout = 2 * time.Minute
	// progressEvery es cada cuánto se muestra el avance en el log.
	progressEvery = 5 * time.Minute
	// maxDetails es cuántos fallos de cada caso se guardan con su salida; del
	// resto solo se guarda el minuto y el motivo.
	maxDetails = 5
)

// soakConfig son las opciones de la prueba larga (ver el comentario inicial).
type soakConfig struct {
	duration  time.Duration
	exe       string
	installed bool // el programa viene de FFC_SOAK_EXE, no de TestMain
	version   string
	reportDir string
	machine   string
	install   string
	arch      string
	seed      uint64
}

func loadSoakConfig(t *testing.T) soakConfig {
	t.Helper()
	cfg := soakConfig{
		duration:  2 * time.Minute,
		exe:       exe,
		version:   os.Getenv("FFC_SOAK_VERSION"),
		reportDir: os.Getenv("FFC_SOAK_REPORT_DIR"),
		machine:   os.Getenv("FFC_SOAK_MACHINE"),
		install:   os.Getenv("FFC_SOAK_INSTALL"),
		arch:      os.Getenv("FFC_SOAK_ARCH"),
		seed:      uint64(time.Now().UnixNano()),
	}
	if v := os.Getenv("FFC_SOAK_DURATION"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			t.Fatalf("FFC_SOAK_DURATION no válida: %q", v)
		}
		cfg.duration = d
	}
	if v := os.Getenv("FFC_SOAK_EXE"); v != "" {
		cfg.exe, cfg.installed = v, true
	}
	if v := os.Getenv("FFC_SOAK_SEED"); v != "" {
		seed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			t.Fatalf("FFC_SOAK_SEED no válida: %q", v)
		}
		cfg.seed = seed
	}
	if cfg.machine == "" {
		cfg.machine = runtime.GOOS + "-" + runtime.GOARCH + " (local)"
	}
	if cfg.install == "" {
		cfg.install = "compilado por la prueba"
		if cfg.installed {
			cfg.install = "FFC_SOAK_EXE"
		}
	}
	if cfg.reportDir == "" {
		dir, err := os.MkdirTemp("", "ffc-soak-informe-")
		if err != nil {
			t.Fatal(err)
		}
		cfg.reportDir = dir
	}
	return cfg
}

// soak es el estado de la prueba larga.
type soak struct {
	soakConfig
	tree    string // árbol de unas 6.000 carpetas (benchtree)
	small   string // árbol pequeño con nombres especiales, para fcd
	work    string // carpeta de trabajo: configuraciones, archivos de fcd
	tempDir string // TEMP de los procesos, para ver si dejan archivos
	program string // lo que muestra --version al empezar
	start   time.Time
	end     time.Time
	rounds  int
	runs    []runRecord
	fails   []failure
	notes   []string // lo que no se pudo probar en esta máquina, y por qué
	seq     int      // contador para nombres de archivo únicos
}

// runRecord es una ejecución de un caso.
type runRecord struct {
	scenario string
	at       time.Duration // desde el comienzo de la prueba
	elapsed  time.Duration
	peakMem  uint64 // bytes; 0 si no se midió
	failed   bool
}

// failure es una ejecución de un caso que no dio lo esperado.
type failure struct {
	scenario string
	at       time.Duration
	detailed bool // se guardó con su salida (ver maxDetails)
	outcome
}

// outcome es lo que deja un caso: la orden ejecutada, su salida y, si falló,
// por qué.
type outcome struct {
	command string
	code    int // -1 si el programa no llegó a terminar
	stdout  string
	stderr  string
	screen  string // pantalla del modo interactivo
	problem string // vacío si todo fue bien
	peakMem uint64
}

// scenario es un caso de la prueba larga.
type scenario struct {
	name string
	run  func(s *soak) outcome
}

func TestSoak(t *testing.T) {
	if exe == "" {
		t.Skip("con -short no se compila el programa")
	}
	s := newSoak(t, loadSoakConfig(t))
	cases := append(commonScenarios(), s.platformScenarios()...)
	t.Logf("%s: %d casos durante %v (semilla %d), programa %s", s.machine, len(cases), s.duration, s.seed, s.exe)

	s.initialChecks()
	rng := rand.New(rand.NewPCG(s.seed, s.seed))
	deadline := s.start.Add(s.duration)
	nextLog := s.start.Add(progressEvery)
	for time.Now().Before(deadline) {
		s.rounds++
		rng.Shuffle(len(cases), func(i, j int) { cases[i], cases[j] = cases[j], cases[i] })
		for _, c := range cases {
			if !time.Now().Before(deadline) {
				break
			}
			s.runCase(t, c)
		}
		if now := time.Now(); now.After(nextLog) {
			t.Logf("minuto %s: %d ejecuciones, %d fallos", minute(now.Sub(s.start)), len(s.runs), len(s.fails))
			nextLog = nextLog.Add(progressEvery)
		}
	}
	s.end = time.Now()
	s.finalChecks()

	path, err := s.writeReport()
	if err != nil {
		t.Fatalf("no se pudo escribir el informe: %v", err)
	}
	t.Logf("%d ejecuciones en %d rondas, %d fallos; informe: %s", len(s.runs), s.rounds, len(s.fails), path)
	if len(s.fails) > 0 {
		t.Errorf("%d ejecuciones fallaron; el detalle está en %s", len(s.fails), path)
	}
}

// newSoak prepara los árboles de carpetas y la carpeta de trabajo.
func newSoak(t *testing.T, cfg soakConfig) *soak {
	t.Helper()
	tree, err := benchtree.Root()
	if err != nil {
		t.Fatalf("no se pudo crear el árbol de carpetas: %v", err)
	}
	t.Cleanup(benchtree.Cleanup)
	s := &soak{
		soakConfig: cfg,
		tree:       tree,
		small:      makeTree(t, []string{"Música/Canción & co", "otros", "docs/Proyecto-A"}, nil),
		work:       t.TempDir(),
	}
	s.tempDir = filepath.Join(s.work, "temp")
	for _, dir := range []string{s.tempDir, filepath.Join(s.work, "config")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s.start = time.Now()
	return s
}

// runCase ejecuta una vez el caso c y lo registra.
func (s *soak) runCase(t *testing.T, c scenario) {
	at := time.Since(s.start)
	start := time.Now()
	o := c.run(s)
	rec := runRecord{scenario: c.name, at: at, elapsed: time.Since(start), peakMem: o.peakMem, failed: o.problem != ""}
	s.runs = append(s.runs, rec)
	if rec.failed {
		t.Logf("FALLO minuto %s, %s: %s", minute(at), c.name, o.problem)
		s.recordFailure(c.name, at, o)
	}
}

// recordFailure guarda un fallo. Solo los primeros maxDetails de cada caso
// conservan la salida, para que un caso que falle siempre no llene la
// memoria ni el informe.
func (s *soak) recordFailure(name string, at time.Duration, o outcome) {
	n := 0
	for _, f := range s.fails {
		if f.scenario == name {
			n++
		}
	}
	f := failure{scenario: name, at: at, detailed: n < maxDetails, outcome: o}
	if f.detailed {
		f.stdout, f.stderr = excerpt(o.stdout), excerpt(o.stderr)
	} else {
		f.stdout, f.stderr, f.screen = "", "", ""
	}
	s.fails = append(s.fails, f)
}

// check registra como fallo de name un problema encontrado fuera de los
// casos (en las comprobaciones inicial y final).
func (s *soak) check(name, problem string) {
	if problem == "" {
		return
	}
	at := time.Since(s.start)
	s.runs = append(s.runs, runRecord{scenario: name, at: at, failed: true})
	s.recordFailure(name, at, outcome{code: -1, problem: problem})
}

// env devuelve el entorno de los procesos: un archivo de configuración que
// no existe (el de quien ejecuta la prueba no debe influir) y una carpeta
// temporal propia (TEMP y TMP en Windows, TMPDIR en Linux). Si una variable
// se repite, exec usa el último valor: extra manda.
func (s *soak) env(extra ...string) []string {
	env := append(os.Environ(),
		"FFC_CONFIG="+filepath.Join(s.work, "config", "sin-config.toml"),
		"TEMP="+s.tempDir,
		"TMP="+s.tempDir,
		"TMPDIR="+s.tempDir,
	)
	return append(env, extra...)
}

// noteOnce añade una nota al informe, si no estaba ya.
func (s *soak) noteOnce(note string) {
	if !slices.Contains(s.notes, note) {
		s.notes = append(s.notes, note)
	}
}

// next devuelve un número distinto en cada llamada, para nombres de archivo.
func (s *soak) next() int {
	s.seq++
	return s.seq
}

// cli ejecuta el programa con args.
func (s *soak) cli(args ...string) outcome {
	return s.exec(s.exe, nil, args...)
}

// exec ejecuta prog con args y las variables extra env, y devuelve su
// resultado. Si no termina en procTimeout, lo detiene y lo da por colgado.
func (s *soak) exec(prog string, env []string, args ...string) outcome {
	ctx, cancel := context.WithTimeout(context.Background(), procTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, prog, args...)
	cmd.Env = s.env(env...)
	cmd.WaitDelay = 5 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	o := outcome{command: commandLine(prog, args), code: -1}
	if len(env) > 0 {
		o.command = strings.Join(env, " ") + " " + o.command
	}
	if err := cmd.Start(); err != nil {
		o.problem = fmt.Sprintf("no se pudo ejecutar: %v", err)
		return o
	}
	peak := watchMemory(cmd.Process)
	err := cmd.Wait()
	o.peakMem = peak(cmd.ProcessState)
	o.stdout, o.stderr = stdout.String(), stderr.String()
	var exitErr *exec.ExitError
	switch {
	case ctx.Err() != nil:
		o.problem = fmt.Sprintf("no terminó en %v: se dio por colgado", procTimeout)
	case errors.As(err, &exitErr):
		o.code = exitErr.ExitCode()
	case err != nil:
		o.problem = fmt.Sprintf("error al esperar al programa: %v", err)
	default:
		o.code = 0
	}
	return o
}

// expect comprueba que el programa terminó con code y que stdout contiene
// wants. Si ya había un problema, lo deja como estaba.
func (o outcome) expect(code int, wants ...string) outcome {
	if o.problem != "" {
		return o
	}
	if o.code != code {
		return o.fail("terminó con el código %d, se esperaba %d", o.code, code)
	}
	for _, want := range wants {
		if !strings.Contains(o.stdout, want) {
			return o.fail("la salida no contiene %q", want)
		}
	}
	return o
}

// fail anota el problema, si no había otro.
func (o outcome) fail(format string, args ...any) outcome {
	if o.problem == "" {
		o.problem = fmt.Sprintf(format, args...)
	}
	return o
}

// steps ejecuta los pasos en orden y se detiene en el primero que falla.
// Devuelve ese paso o, si todos van bien, el último, con la memoria máxima
// de todos.
func steps(fns ...func() outcome) outcome {
	var last outcome
	var peak uint64
	for _, fn := range fns {
		last = fn()
		peak = max(peak, last.peakMem)
		if last.problem != "" {
			break
		}
	}
	last.peakMem = peak
	return last
}

// commandLine escribe la orden como se escribiría en una terminal.
func commandLine(prog string, args []string) string {
	parts := []string{filepath.Base(prog)}
	for _, a := range args {
		if a == "" || strings.ContainsAny(a, " &?*") {
			a = `"` + a + `"`
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

// excerpt recorta una salida larga para el informe.
func excerpt(s string) string {
	const maxLines, maxBytes = 40, 4000
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	extra := ""
	if len(lines) > maxLines {
		extra = fmt.Sprintf("\n… (%d líneas más)", len(lines)-maxLines)
		lines = lines[:maxLines]
	}
	out := strings.Join(lines, "\n")
	if len(out) > maxBytes {
		out = strings.ToValidUTF8(out[:maxBytes], "") + "\n… (recortado)"
	}
	return out + extra
}

// minute da formato mm:ss a un instante de la prueba.
func minute(d time.Duration) string {
	d = d.Round(time.Second)
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

// initialChecks anota la versión del programa y hace las comprobaciones
// propias del sistema (en Windows, la instalación).
func (s *soak) initialChecks() {
	s.program = strings.TrimSpace(s.cli("--version").stdout)
	s.platformChecks()
}

// finalChecks comprueba lo que podría acumularse con el tiempo: archivos
// temporales sin borrar y la lista de carpetas recientes.
func (s *soak) finalChecks() {
	s.check("comprobación-final", leftoverTemp(s.tempDir))
	s.check("comprobación-final", checkRecent(filepath.Join(s.work, "config", "recientes.json")))
}

// leftoverTemp busca archivos temporales que fcd debería haber borrado:
// fcd-*.txt (fcd.cmd) y tmp*.tmp (New-TemporaryFile, en fcd.ps1).
func leftoverTemp(dir string) string {
	var left []string
	for _, pattern := range []string{"fcd-*", "tmp*.tmp"} {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, m := range matches {
			left = append(left, filepath.Base(m))
		}
	}
	if len(left) == 0 {
		return ""
	}
	total := len(left)
	if total > 10 {
		left = append(left[:10], "…")
	}
	return fmt.Sprintf("quedaron %d archivos temporales sin borrar en TEMP: %s", total, strings.Join(left, ", "))
}
