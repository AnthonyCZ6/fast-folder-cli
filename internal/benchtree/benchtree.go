// Package benchtree crea el árbol de carpetas sintético que usan las pruebas
// de rendimiento (benchmarks) de search y cli. Es siempre el mismo, para que
// las medidas de dos versiones del código sean comparables.
package benchtree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Forma del árbol: areas carpetas de primer nivel con projects proyectos cada
// una. Cada proyecto tiene src, docs/Canción y un node_modules con packages
// paquetes de filesPerPackage archivos de fileSize bytes. En total, unas
// 6.000 carpetas y 10.200 archivos.
const (
	areas           = 10
	projects        = 20
	packages        = 25
	filesPerPackage = 2
	fileSize        = 512
)

// version identifica la forma del árbol: si cambia, un árbol ya creado en la
// carpeta de EnvDir se vuelve a crear.
const version = "1"

// EnvDir es la variable de entorno con la carpeta donde crear el árbol para
// reutilizarlo en varias ejecuciones.
const EnvDir = "FFC_BENCH_TREE"

var (
	once    sync.Once
	root    string
	rootErr error
	tempDir string // carpeta temporal que borra Cleanup
)

// Root devuelve la carpeta raíz del árbol y lo crea la primera vez. Con
// EnvDir definida, el árbol se crea allí y se reutiliza en las siguientes
// ejecuciones; si no, se crea en una carpeta temporal que borra Cleanup.
func Root() (string, error) {
	once.Do(func() {
		if dir := os.Getenv(EnvDir); dir != "" {
			root, rootErr = reuse(dir)
			return
		}
		tempDir, rootErr = os.MkdirTemp("", "ffc-bench-")
		if rootErr == nil {
			root, rootErr = tempDir, build(tempDir)
		}
	})
	return root, rootErr
}

// Cleanup borra el árbol si se creó en una carpeta temporal.
func Cleanup() {
	if tempDir != "" {
		os.RemoveAll(tempDir)
	}
}

// reuse devuelve dir si ya contiene un árbol completo de esta versión y, si
// no, lo crea de nuevo. La versión se guarda junto a dir, no dentro, para no
// alterar el árbol.
func reuse(dir string) (string, error) {
	marker := dir + ".version"
	if data, err := os.ReadFile(marker); err == nil && string(data) == version {
		return dir, nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := build(dir); err != nil {
		return "", err
	}
	return dir, os.WriteFile(marker, []byte(version), 0o644)
}

// build crea el árbol en dir.
func build(dir string) error {
	content := []byte(strings.Repeat("x", fileSize))
	for a := range areas {
		area := filepath.Join(dir, fmt.Sprintf("area-%02d", a))
		for p := range projects {
			project := filepath.Join(area, fmt.Sprintf("Proyecto-%03d", a*projects+p))
			if err := buildProject(project, p%2 == 0, content); err != nil {
				return err
			}
		}
	}
	return nil
}

// buildProject crea un proyecto de Go (goMod) o de Node.js.
func buildProject(project string, goMod bool, content []byte) error {
	for _, sub := range []string{"src", filepath.Join("docs", "Canción")} {
		if err := os.MkdirAll(filepath.Join(project, sub), 0o755); err != nil {
			return err
		}
	}
	marker := "package.json"
	if goMod {
		marker = "go.mod"
	}
	if err := os.WriteFile(filepath.Join(project, marker), nil, 0o644); err != nil {
		return err
	}
	for k := range packages {
		pkg := filepath.Join(project, "node_modules", fmt.Sprintf("pkg-%02d", k))
		if err := os.MkdirAll(pkg, 0o755); err != nil {
			return err
		}
		for f := range filesPerPackage {
			if err := os.WriteFile(filepath.Join(pkg, fmt.Sprintf("file-%d.js", f)), content, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
