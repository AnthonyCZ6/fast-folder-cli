// fast-folder-cli: búsqueda ultrarrápida de carpetas para Windows.
package main

import (
	"os"
	"runtime/debug"
	"strings"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/cli"
)

// version se sobrescribe al compilar con:
//
//	go build -ldflags "-X main.version=1.0.0"
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, resolveVersion()))
}

// resolveVersion devuelve la versión definida con -ldflags o, si no se definió
// (por ejemplo al instalar con "go install ...@latest"), la versión del módulo
// que Go registra en el ejecutable.
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return version
}
