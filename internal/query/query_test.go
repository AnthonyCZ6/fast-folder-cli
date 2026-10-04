package query

import (
	"errors"
	"testing"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/period"
)

var now = time.Date(2026, 10, 2, 15, 30, 0, 0, time.Local)

func mustNew(t *testing.T, term string, projects bool, modified string) Query {
	t.Helper()
	q, err := New(term, projects, modified, now)
	if err != nil {
		t.Fatalf("New(%q, %v, %q): %v", term, projects, modified, err)
	}
	return q
}

func TestNewErrors(t *testing.T) {
	tests := []struct {
		term     string
		projects bool
		modified string
		want     error
	}{
		{"", false, "", ErrEmpty},
		{"informe", false, "mañana", period.ErrInvalid},
		{"", true, "0d", period.ErrInvalid},
		// Un periodo con solo espacios no es "cualquier fecha": es inválido.
		{"informe", false, " ", period.ErrInvalid},
	}
	for _, tt := range tests {
		if _, err := New(tt.term, tt.projects, tt.modified, now); !errors.Is(err, tt.want) {
			t.Errorf("New(%q, %v, %q) error = %v, want %v", tt.term, tt.projects, tt.modified, err, tt.want)
		}
	}

	// Errores del término: un patrón mal formado y un término con solo
	// espacios, que no cuenta como vacío.
	for _, term := range []string{"[roto", "   "} {
		_, err := New(term, false, "", now)
		if err == nil || errors.Is(err, ErrEmpty) || errors.Is(err, period.ErrInvalid) {
			t.Errorf("New(%q): error = %v, want el error del término", term, err)
		}
	}
}

func TestDescribe(t *testing.T) {
	tests := []struct {
		term     string
		projects bool
		modified string
		want     string
	}{
		{"tesis", false, "", `"tesis"`},
		{"", true, "", "proyectos"},
		{"api", true, "", `proyectos "api"`},
		{"", false, "hoy", "carpetas modificadas hoy"},
		{"informe", false, "semana", `carpetas "informe" modificadas en los últimos 7 días`},
		{"", true, "ayer", "proyectos modificados ayer"},
	}
	for _, tt := range tests {
		if got := mustNew(t, tt.term, tt.projects, tt.modified).Describe(); got != tt.want {
			t.Errorf("New(%q, %v, %q).Describe() = %q, want %q", tt.term, tt.projects, tt.modified, got, tt.want)
		}
	}
}

func TestFound(t *testing.T) {
	folders := mustNew(t, "x", false, "")
	projects := mustNew(t, "", true, "")
	tests := []struct {
		got, want string
	}{
		{folders.Found(1), "1 carpeta encontrada"},
		{folders.Found(1500), "1,500 carpetas encontradas"},
		{projects.Found(1), "1 proyecto encontrado"},
		{projects.Found(0), "0 proyectos encontrados"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("Found = %q, want %q", tt.got, tt.want)
		}
	}
}

func TestOptions(t *testing.T) {
	opts := mustNew(t, "cancion", true, "ayer").Options("raíz", true)
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

	opts = mustNew(t, "", true, "").Options("raíz", false)
	if opts.Matcher != nil || !opts.ModifiedAfter.IsZero() || !opts.ModifiedBefore.IsZero() {
		t.Errorf("sin término ni fecha no debería filtrar: %+v", opts)
	}
}
