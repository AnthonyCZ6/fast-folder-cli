package cli

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/benchtree"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/query"
)

// Las pruebas de rendimiento recorren el árbol sintético de benchtree. Este
// archivo no depende de otros archivos de prueba: el workflow bench.yml lo
// copia a la versión del código con la que se compara.

func TestMain(m *testing.M) {
	code := m.Run()
	benchtree.Cleanup()
	os.Exit(code)
}

// BenchmarkRun mide la CLI de principio a fin, sin la consola: una búsqueda
// por término y --size sobre los node_modules del árbol.
func BenchmarkRun(b *testing.B) {
	root, err := benchtree.Root()
	if err != nil {
		b.Fatal(err)
	}
	cases := []struct {
		name string
		term string
		cfg  config
	}{
		{"termino", "proyecto", config{}},
		{"size", "node_modules", config{size: true}},
	}
	for _, c := range cases {
		q, err := query.New(c.term, query.Folders, "", time.Now())
		if err != nil {
			b.Fatal(err)
		}
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				runSearch(context.Background(), c.cfg, q, root, io.Discard, io.Discard)
			}
		})
	}
}
