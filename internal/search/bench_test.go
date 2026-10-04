package search

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/benchtree"
)

// Las pruebas de rendimiento recorren el árbol sintético de benchtree. Este
// archivo no depende de otros archivos de prueba: el workflow bench.yml lo
// copia a la versión del código con la que se compara.

func TestMain(m *testing.M) {
	code := m.Run()
	benchtree.Cleanup()
	os.Exit(code)
}

func benchRoot(b *testing.B) string {
	b.Helper()
	root, err := benchtree.Root()
	if err != nil {
		b.Fatal(err)
	}
	return root
}

func benchMatcher(b *testing.B, term string) *Matcher {
	b.Helper()
	m, err := NewMatcher(term)
	if err != nil {
		b.Fatal(err)
	}
	return m
}

// BenchmarkStart mide una búsqueda completa en cada modo.
func BenchmarkStart(b *testing.B) {
	root := benchRoot(b)
	cases := []struct {
		name string
		opts Options
	}{
		{"termino", Options{Matcher: benchMatcher(b, "proyecto")}},
		{"comodin", Options{Matcher: benchMatcher(b, "pkg-1*")}},
		{"acentos", Options{Matcher: benchMatcher(b, "cancion")}},
		{"fecha", Options{ModifiedAfter: time.Now().AddDate(0, 0, -7)}},
		{"proyectos", Options{Projects: true}},
	}
	for _, c := range cases {
		opts := c.opts
		opts.Root = root
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				results, _ := Start(context.Background(), opts)
				for range results {
				}
			}
		})
	}
}

// BenchmarkSize mide el tamaño de todo el árbol.
func BenchmarkSize(b *testing.B) {
	root := benchRoot(b)
	for b.Loop() {
		Size(context.Background(), root)
	}
}

// BenchmarkMatch compara un término con nombres habituales, con mayúsculas y
// acentos, sin tocar el disco.
func BenchmarkMatch(b *testing.B) {
	names := []string{"node_modules", "Proyecto-042", "src", "Documentos", "Canción", "pkg-17", "README", "Música"}
	for _, c := range []struct{ name, term string }{{"subcadena", "proyecto"}, {"comodin", "proy*"}} {
		m := benchMatcher(b, c.term)
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				for _, name := range names {
					m.Match(name)
				}
			}
		})
	}
}
