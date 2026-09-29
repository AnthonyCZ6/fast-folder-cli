// fast-folder-cli: búsqueda ultrarrápida de carpetas para Windows.
package main

import (
	"os"

	"fast-folder-cli/internal/cli"
)

// version se sobrescribe al compilar con:
//
//	go build -ldflags "-X main.version=1.0.0"
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr, version))
}
