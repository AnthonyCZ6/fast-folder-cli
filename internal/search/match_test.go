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
		// Sin distinguir acentos, diéresis ni la tilde de la ñ.
		{"cancion", "Canción", true},
		{"CANCIÓN", "cancion", true},
		{"ano", "Año 2024", true},
		{"pinguino", "Pingüino", true},
		{"cafe*", "Café-Internet", true},
		{"cafe", "Cafe\u0301", true}, // forma descompuesta
		{"tesis", "Tésis", true},
		{"cancion", "Canasta", false},
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

// FuzzMatcher comprueba que ningún término ni nombre provoca un pánico y que
// fold es idempotente: plegar un nombre ya plegado no lo cambia, así que
// Match da lo mismo con el nombre original que con su forma plegada.
func FuzzMatcher(f *testing.F) {
	for _, seed := range [][2]string{
		{"cancion", "Canción"},
		{"proy*", "Proyectos"},
		{"[ab]ackup", "Backup"},
		{"cafe", "Café"},
		{"ñ", "AÑO"},
		{"[", "x"},
		{"x", "\xff"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, term, name string) {
		folded := fold(name)
		if again := fold(folded); again != folded {
			t.Errorf("fold no es idempotente: fold(%q) = %q, pero fold(%q) = %q", name, folded, folded, again)
		}
		m, err := NewMatcher(term)
		if err != nil {
			return
		}
		if m.Match(name) != m.Match(folded) {
			t.Errorf("NewMatcher(%q): Match(%q) != Match(%q)", term, name, folded)
		}
	})
}
