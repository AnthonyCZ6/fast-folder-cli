package query

import (
	"errors"
	"testing"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/period"
)

var now = time.Date(2026, 10, 2, 15, 30, 0, 0, time.Local)

func mustNew(t *testing.T, term string, kind Kind, modified string) Query {
	t.Helper()
	q, err := New(term, kind, modified, now)
	if err != nil {
		t.Fatalf("New(%q, %v, %q): %v", term, kind, modified, err)
	}
	return q
}

func TestNewErrors(t *testing.T) {
	tests := []struct {
		term     string
		kind     Kind
		modified string
		want     error
	}{
		{"", Folders, "", ErrEmpty},
		{"informe", Folders, "mañana", period.ErrInvalid},
		{"", Projects, "0d", period.ErrInvalid},
		// Un periodo con solo espacios no es "cualquier fecha": es inválido.
		{"informe", Folders, " ", period.ErrInvalid},
		// Las apps no tienen fecha de modificación.
		{"chrome", Apps, "hoy", errAppsPeriod},
	}
	for _, tt := range tests {
		if _, err := New(tt.term, tt.kind, tt.modified, now); !errors.Is(err, tt.want) {
			t.Errorf("New(%q, %v, %q) error = %v, want %v", tt.term, tt.kind, tt.modified, err, tt.want)
		}
	}

	// Errores del término: un patrón mal formado y un término con solo
	// espacios, que no cuenta como vacío.
	for _, term := range []string{"[roto", "   "} {
		_, err := New(term, Folders, "", now)
		if err == nil || errors.Is(err, ErrEmpty) || errors.Is(err, period.ErrInvalid) {
			t.Errorf("New(%q): error = %v, want el error del término", term, err)
		}
	}
}

func TestDescribe(t *testing.T) {
	tests := []struct {
		term     string
		kind     Kind
		modified string
		want     string
	}{
		{"tesis", Folders, "", `"tesis"`},
		{"", Projects, "", "proyectos"},
		{"api", Projects, "", `proyectos "api"`},
		{"", Folders, "hoy", "carpetas modificadas hoy"},
		{"informe", Folders, "semana", `carpetas "informe" modificadas en los últimos 7 días`},
		{"", Projects, "ayer", "proyectos modificados ayer"},
		{"", Apps, "", "apps"},
		{"chrome", Apps, "", `apps "chrome"`},
		{"", Recent, "", "carpetas recientes"},
		{"tesis", Recent, "ayer", `carpetas recientes "tesis" usadas ayer`},
	}
	for _, tt := range tests {
		if got := mustNew(t, tt.term, tt.kind, tt.modified).Describe(); got != tt.want {
			t.Errorf("New(%q, %v, %q).Describe() = %q, want %q", tt.term, tt.kind, tt.modified, got, tt.want)
		}
	}
}

func TestFound(t *testing.T) {
	folders := mustNew(t, "x", Folders, "")
	projects := mustNew(t, "", Projects, "")
	apps := mustNew(t, "", Apps, "")
	tests := []struct {
		got, want string
	}{
		{folders.Found(1), "1 carpeta encontrada"},
		{folders.Found(1500), "1,500 carpetas encontradas"},
		{projects.Found(1), "1 proyecto encontrado"},
		{projects.Found(0), "0 proyectos encontrados"},
		{apps.Found(1), "1 app encontrada"},
		{apps.Found(3), "3 apps encontradas"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("Found = %q, want %q", tt.got, tt.want)
		}
	}
}

func TestMatch(t *testing.T) {
	if q := mustNew(t, "cancion", Apps, ""); !q.Match("Canción Studio") || q.Match("Google Chrome") {
		t.Error("Match debería aplicar el término sin distinguir acentos")
	}
	if q := mustNew(t, "", Apps, ""); !q.Match("Cualquier app") {
		t.Error("sin término, todos los nombres deberían coincidir")
	}
}

func TestOptions(t *testing.T) {
	opts := mustNew(t, "cancion", Projects, "ayer").Options("raíz", true)
	if opts.Root != "raíz" || !opts.IncludeHidden || !opts.Projects {
		t.Errorf("opciones = %+v", opts)
	}
	if opts.Matcher == nil || !opts.Matcher.Match("Canción") {
		t.Error("el término debería coincidir con Canción")
	}
	yesterday := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
	if !opts.ModifiedAfter.Equal(yesterday) || !opts.ModifiedBefore.Equal(yesterday.AddDate(0, 0, 1)) {
		t.Errorf("intervalo = [%v, %v), want ayer", opts.ModifiedAfter, opts.ModifiedBefore)
	}

	opts = mustNew(t, "", Projects, "").Options("raíz", false)
	if opts.Matcher != nil || !opts.ModifiedAfter.IsZero() || !opts.ModifiedBefore.IsZero() {
		t.Errorf("sin término ni fecha no debería filtrar: %+v", opts)
	}
}
