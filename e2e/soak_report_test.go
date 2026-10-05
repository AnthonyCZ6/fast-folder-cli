//go:build soak

package e2e

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Tendencias: se comparan las medianas de los primeros y los últimos minutos
// de cada caso (trendWindow, o la quinta parte de una prueba corta). Un
// aumento que supera los dos umbrales de una medida es un aviso: no es un
// fallo, porque el tiempo en un runner compartido varía, pero se informa.
const (
	trendWindow    = 10 * time.Minute
	trendMinRuns   = 3
	memTrendRatio  = 1.5
	memTrendBytes  = 16 << 20
	timeTrendRatio = 3.0
	timeTrendDelta = 500 * time.Millisecond
	// maxSummaryFailures es cuántos fallos se listan en resumen.json.
	maxSummaryFailures = 200
)

// caseStats resume las ejecuciones de un caso.
type caseStats struct {
	name                string
	runs, failures      int
	p50, p95, max       time.Duration
	maxMem              uint64
	trend               bool // hay ejecuciones suficientes al principio y al final
	timeFirst, timeLast time.Duration
	memFirst, memLast   uint64 // 0 si no se midió la memoria
}

// soakSummary es resumen.json: lo que junta soak-report.sh de cada máquina.
type soakSummary struct {
	Machine  string           `json:"machine"`
	Install  string           `json:"install"`
	System   string           `json:"system"`
	Program  string           `json:"program"`
	Status   string           `json:"status"` // "sin fallos" o "con fallos"
	Minutes  int              `json:"minutes"`
	Runs     int              `json:"runs"`
	Rounds   int              `json:"rounds"`
	Failures int              `json:"failures"`
	Warnings int              `json:"warnings"`
	Seed     string           `json:"seed"`
	Failed   []summaryFailure `json:"failed"`
}

type summaryFailure struct {
	Scenario string `json:"scenario"`
	Minute   string `json:"minute"`
	Problem  string `json:"problem"`
}

// writeReport escribe informe.md y resumen.json en la carpeta del informe y
// devuelve la ruta de informe.md.
func (s *soak) writeReport() (string, error) {
	if err := os.MkdirAll(s.reportDir, 0o755); err != nil {
		return "", err
	}
	stats := s.stats()
	var warnings []string
	for _, st := range stats {
		warnings = append(warnings, st.warnings(s.window())...)
	}
	path := filepath.Join(s.reportDir, "informe.md")
	if err := os.WriteFile(path, []byte(s.markdown(stats, warnings)), 0o644); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(s.summary(len(warnings)), "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(filepath.Join(s.reportDir, "resumen.json"), append(data, '\n'), 0o644)
}

// window es cuánto dura el principio y el final de la prueba con los que se
// calculan las tendencias.
func (s *soak) window() time.Duration {
	return min(trendWindow, s.end.Sub(s.start)/5)
}

// stats resume las ejecuciones de cada caso, por orden alfabético.
func (s *soak) stats() []caseStats {
	byName := map[string][]runRecord{}
	for _, r := range s.runs {
		byName[r.scenario] = append(byName[r.scenario], r)
	}
	total, window := s.end.Sub(s.start), s.window()
	var out []caseStats
	for name, runs := range byName {
		st := caseStats{name: name, runs: len(runs)}
		var times, firstT, lastT []time.Duration
		var firstM, lastM []uint64
		for _, r := range runs {
			times = append(times, r.elapsed)
			st.maxMem = max(st.maxMem, r.peakMem)
			switch {
			case r.failed:
				st.failures++
			case r.at < window:
				firstT, firstM = append(firstT, r.elapsed), appendPositive(firstM, r.peakMem)
			case r.at >= total-window:
				lastT, lastM = append(lastT, r.elapsed), appendPositive(lastM, r.peakMem)
			}
		}
		slices.Sort(times)
		n := len(times)
		st.p50, st.p95, st.max = times[(n-1)*50/100], times[(n-1)*95/100], times[n-1]
		if len(firstT) >= trendMinRuns && len(lastT) >= trendMinRuns {
			st.trend = true
			st.timeFirst, st.timeLast = median(firstT), median(lastT)
			if len(firstM) >= trendMinRuns && len(lastM) >= trendMinRuns {
				st.memFirst, st.memLast = median(firstM), median(lastM)
			}
		}
		out = append(out, st)
	}
	slices.SortFunc(out, func(a, b caseStats) int { return strings.Compare(a.name, b.name) })
	return out
}

func appendPositive(xs []uint64, x uint64) []uint64 {
	if x == 0 {
		return xs
	}
	return append(xs, x)
}

func median[T cmp.Ordered](xs []T) T {
	sorted := slices.Clone(xs)
	slices.Sort(sorted)
	return sorted[len(sorted)/2]
}

// warnings devuelve los avisos de tendencia del caso.
func (st caseStats) warnings(window time.Duration) []string {
	var out []string
	if st.memFirst > 0 && float64(st.memLast) > memTrendRatio*float64(st.memFirst) && st.memLast-st.memFirst > memTrendBytes {
		out = append(out, fmt.Sprintf("%s: la memoria máxima pasó de %s a %s (medianas de los primeros y los últimos %v).",
			st.name, fmtMem(st.memFirst), fmtMem(st.memLast), window.Round(time.Second)))
	}
	if st.trend && float64(st.timeLast) > timeTrendRatio*float64(st.timeFirst) && st.timeLast-st.timeFirst > timeTrendDelta {
		out = append(out, fmt.Sprintf("%s: el tiempo pasó de %s a %s (medianas de los primeros y los últimos %v).",
			st.name, fmtDur(st.timeFirst), fmtDur(st.timeLast), window.Round(time.Second)))
	}
	return out
}

// trendText describe la tendencia del caso: últimos minutos frente a los
// primeros.
func (st caseStats) trendText() string {
	if !st.trend {
		return "—"
	}
	text := fmt.Sprintf("tiempo ×%.2f", float64(st.timeLast)/float64(st.timeFirst))
	if st.memFirst > 0 {
		text = fmt.Sprintf("memoria ×%.2f · %s", float64(st.memLast)/float64(st.memFirst), text)
	}
	return text
}

// markdown devuelve el informe de esta máquina.
func (s *soak) markdown(stats []caseStats, warnings []string) string {
	var b strings.Builder
	p := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	status := "✅ sin fallos"
	if n := len(s.fails); n > 0 {
		status = "❌ " + plural(n, "fallo", "fallos")
	}
	p("## %s — %s\n\n", s.machine, status)
	p("| | |\n|---|---|\n")
	p("| Sistema | %s |\n", s.system())
	p("| Instalación | %s |\n", s.install)
	p("| Programa | %s (`%s`) |\n", cmp.Or(s.program, "—"), s.exe)
	p("| Duración | %s |\n", fmtTotal(s.end.Sub(s.start)))
	p("| Ejecuciones | %d en %d rondas (semilla %d) |\n", len(s.runs), s.rounds, s.seed)
	p("| Fallos | %d |\n", len(s.fails))
	p("| Avisos | %d |\n\n", len(warnings))

	p("### Casos\n\n")
	p("| Caso | Ejecuciones | Fallos | p50 | p95 | Máx. | Memoria máx. | Tendencia (final / inicio) |\n")
	p("|---|---:|---:|---:|---:|---:|---:|---|\n")
	for _, st := range stats {
		p("| %s | %d | %d | %s | %s | %s | %s | %s |\n", st.name, st.runs, st.failures,
			fmtDur(st.p50), fmtDur(st.p95), fmtDur(st.max), fmtMem(st.maxMem), st.trendText())
	}

	p("\n### Fallos\n\n")
	if len(s.fails) == 0 {
		p("Ninguna ejecución falló.\n")
	}
	s.writeFailures(&b)

	p("\n### Avisos\n\n")
	if len(warnings) == 0 {
		p("Ninguno: el tiempo y la memoria de cada caso se mantuvieron estables.\n")
	}
	for _, w := range warnings {
		p("- %s\n", w)
	}
	if len(s.notes) > 0 {
		p("\n### Notas\n\n")
		for _, n := range s.notes {
			p("- %s\n", n)
		}
	}
	return b.String()
}

// writeFailures escribe el detalle de los fallos guardados con su salida y,
// agrupados por caso, el minuto y el motivo de los demás.
func (s *soak) writeFailures(b *strings.Builder) {
	p := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }
	n := 0
	rest := map[string][]failure{}
	var restOrder []string
	for _, f := range s.fails {
		if !f.detailed {
			if len(rest[f.scenario]) == 0 {
				restOrder = append(restOrder, f.scenario)
			}
			rest[f.scenario] = append(rest[f.scenario], f)
			continue
		}
		n++
		p("#### %d. %s — minuto %s\n\n", n, f.scenario, minute(f.at))
		p("- **Motivo:** %s\n", f.problem)
		if f.command != "" {
			p("- **Orden:** `%s`\n", f.command)
		}
		if f.code >= 0 {
			p("- **Código de salida:** %d\n", f.code)
		}
		for _, block := range []struct{ title, text string }{
			{"stdout", f.stdout}, {"stderr", f.stderr}, {"Pantalla", f.screen},
		} {
			if strings.TrimSpace(block.text) != "" {
				p("\n%s:\n\n```text\n%s\n```\n", block.title, strings.TrimRight(block.text, "\n"))
			}
		}
		p("\n")
	}
	for _, name := range restOrder {
		fails := rest[name]
		var minutes []string
		for i, f := range fails {
			if i == 20 {
				minutes = append(minutes, "…")
				break
			}
			minutes = append(minutes, minute(f.at))
		}
		p("- **%s:** %s más, sin la salida (minutos %s). Primer motivo: %s\n",
			name, plural(len(fails), "fallo", "fallos"), strings.Join(minutes, ", "), fails[0].problem)
	}
}

// summary devuelve resumen.json.
func (s *soak) summary(warnings int) soakSummary {
	sum := soakSummary{
		Machine:  s.machine,
		Install:  s.install,
		System:   s.system(),
		Program:  s.program,
		Status:   "sin fallos",
		Minutes:  int(s.end.Sub(s.start).Round(time.Minute).Minutes()),
		Runs:     len(s.runs),
		Rounds:   s.rounds,
		Failures: len(s.fails),
		Warnings: warnings,
		Seed:     strconv.FormatUint(s.seed, 10),
		Failed:   []summaryFailure{},
	}
	if len(s.fails) > 0 {
		sum.Status = "con fallos"
	}
	for _, f := range s.fails[:min(len(s.fails), maxSummaryFailures)] {
		sum.Failed = append(sum.Failed, summaryFailure{Scenario: f.scenario, Minute: minute(f.at), Problem: f.problem})
	}
	return sum
}

// system describe el sistema, la arquitectura y, en GitHub Actions, la
// imagen del runner.
func (s *soak) system() string {
	text := osDescription() + " · " + runtime.GOARCH
	if image := os.Getenv("ImageOS"); image != "" {
		text += " · imagen " + strings.TrimSpace(image+" "+os.Getenv("ImageVersion"))
	}
	return text
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

func fmtDur(d time.Duration) string {
	if d < time.Second {
		return strconv.FormatInt(d.Milliseconds(), 10) + " ms"
	}
	return fmt.Sprintf("%.2f s", d.Seconds())
}

func fmtMem(n uint64) string {
	if n == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
}

func fmtTotal(d time.Duration) string {
	d = d.Round(time.Second)
	return fmt.Sprintf("%d min %02d s", int(d.Minutes()), int(d.Seconds())%60)
}
