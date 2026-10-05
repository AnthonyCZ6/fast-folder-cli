//go:build soak && !windows

package e2e

import (
	"os"
	"runtime"
	"strings"
)

// platformScenarios no añade casos: las apps, el modo interactivo en ConPTY
// y fcd son propios de Windows.
func (s *soak) platformScenarios() []scenario {
	s.notes = append(s.notes,
		"Fuera de Windows solo se prueba la línea de comandos: las apps, el modo interactivo y fcd son propios de Windows.",
		"En Linux no se mide la memoria: Go lanza los procesos con vfork y el kernel atribuye al hijo la memoria máxima del proceso de prueba.")
	return nil
}

func (s *soak) platformChecks() {}

// watchMemory no mide nada fuera de Windows. En Linux, el Maxrss de un hijo
// lanzado con vfork (como hace Go) incluye la memoria máxima del padre, así
// que no diría cuánto usa el programa.
func watchMemory(*os.Process) func(*os.ProcessState) uint64 {
	return func(*os.ProcessState) uint64 { return 0 }
}

// osDescription devuelve el nombre del sistema según /etc/os-release.
func osDescription() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}
	for _, line := range strings.Split(string(data), "\n") {
		if name, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(name, `"`)
		}
	}
	return runtime.GOOS
}
