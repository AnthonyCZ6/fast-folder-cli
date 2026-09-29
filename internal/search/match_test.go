package search

import "testing"

func TestMatcher(t *testing.T) {
	tests := []struct {
		term, name string
		want       bool
	}{
		{"proy", "Proyectos", true},
		{"PROY", "mi-proyecto", true},
		{"proyecto", "proy", false},
		{"  node_modules ", "node_modules", true},
		{"Imágenes", "IMÁGENES", true},
		{"proy*", "Proyectos", true},
		{"proy*", "mi-proyecto", false},
		{"*proy*", "mi-proyecto", true},
		{"tesis-202?", "Tesis-2024", true},
		{"tesis-202?", "Tesis-20245", false},
		{"[ab]ackup", "Backup", true},
		{"[ab]ackup", "Cackup", false},
	}
	for _, tt := range tests {
		m, err := NewMatcher(tt.term)
		if err != nil {
			t.Fatalf("NewMatcher(%q): %v", tt.term, err)
		}
		if got := m.Match(tt.name); got != tt.want {
			t.Errorf("NewMatcher(%q).Match(%q) = %v, want %v", tt.term, tt.name, got, tt.want)
		}
	}
}

func TestMatcherErrors(t *testing.T) {
	for _, term := range []string{"", "   ", "[abc"} {
		if _, err := NewMatcher(term); err == nil {
			t.Errorf("NewMatcher(%q): se esperaba un error", term)
		}
	}
}
