//go:build soak && windows

package e2e

import (
	"debug/pe"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// platformScenarios devuelve los casos propios de Windows: las apps, el modo
// interactivo, fcd en cada terminal instalada y, si el programa está
// instalado, el atajo fast.exe.
func (s *soak) platformScenarios() []scenario {
	cases := []scenario{
		{"apps", (*soak).caseApps},
		{"interactivo", (*soak).caseInteractive},
	}
	for _, shell := range []string{"powershell.exe", "pwsh.exe", "cmd.exe"} {
		if _, err := exec.LookPath(shell); err != nil {
			s.notes = append(s.notes, shell+" no está instalado: no se prueba fcd con él.")
			continue
		}
		name := "fcd-" + strings.TrimSuffix(shell, ".exe")
		cases = append(cases, scenario{name, func(s *soak) outcome { return s.caseFcd(shell) }})
	}
	if s.installed {
		cases = append(cases, scenario{"atajo-fast", (*soak).caseAlias})
	}
	return cases
}

// platformChecks comprueba la instalación: la arquitectura del ejecutable y
// que fast.exe y los scripts de fcd están a su lado. Con el programa que
// compila la prueba, copia los scripts a su lado, como el asistente.
func (s *soak) platformChecks() {
	s.check("instalación", checkArch(s.exe, s.arch))
	dir := filepath.Dir(s.exe)
	if s.installed {
		for _, name := range []string{"fast.exe", "fcd.ps1", "fcd.cmd"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				s.check("instalación", fmt.Sprintf("falta %s junto al programa", name))
			}
		}
		return
	}
	for _, name := range []string{"fcd.ps1", "fcd.cmd"} {
		data, err := os.ReadFile(filepath.Join("..", "shell", name))
		if err == nil {
			err = os.WriteFile(filepath.Join(dir, name), data, 0o644)
		}
		if err != nil {
			s.check("preparación", fmt.Sprintf("no se pudo copiar %s: %v", name, err))
		}
	}
}

// checkArch comprueba que el ejecutable es para want: X64 o ARM64, como
// runner.arch en GitHub Actions. Con otro valor (o vacío) no comprueba nada.
func checkArch(path, want string) string {
	machines := map[string]uint16{"X64": pe.IMAGE_FILE_MACHINE_AMD64, "ARM64": pe.IMAGE_FILE_MACHINE_ARM64}
	machine, ok := machines[want]
	if !ok {
		return ""
	}
	f, err := pe.Open(path)
	if err != nil {
		return fmt.Sprintf("no se pudo leer el ejecutable: %v", err)
	}
	defer f.Close()
	if f.Machine != machine {
		return fmt.Sprintf("el ejecutable es para la máquina 0x%X; en %s debería ser 0x%X", f.Machine, want, machine)
	}
	return ""
}

// --apps lee los programas instalados de este Windows: una línea JSON válida
// por app y, si Git está instalado (como en los runners de GitHub), lo
// busca por su nombre. Que git esté en el PATH no garantiza que figure entre
// las apps (depende de cómo lo instaló la imagen): si no aparece, se anota
// en el informe en lugar de contarlo como fallo en cada ronda.
func (s *soak) caseApps() outcome {
	return steps(
		func() outcome {
			o := s.cli("--apps", "--json").expect(0)
			lines := nonEmptyLines(o.stdout)
			for i, line := range lines {
				var app struct {
					Name string `json:"name"`
					Dir  string `json:"dir"`
				}
				if err := json.Unmarshal([]byte(line), &app); err != nil {
					return o.fail("la línea %d no es JSON válido (%v): %s", i+1, err, line)
				}
				if app.Name == "" || app.Dir == "" {
					return o.fail("app sin nombre o sin carpeta en la línea %d: %s", i+1, line)
				}
			}
			if len(lines) == 0 {
				return o.fail("--apps no encontró ninguna app")
			}
			return o
		},
		func() outcome {
			if _, err := exec.LookPath("git"); err != nil {
				return outcome{}
			}
			o := s.cli("--apps", "git")
			if o.problem == "" && o.code == 1 {
				s.noteOnce("git está en el PATH pero no figura entre las apps instaladas: no se comprueba que --apps encuentre Git.")
				return o.expect(1, `Buscando apps "git"`, "sin coincidencias")
			}
			return o.expect(0, `Buscando apps "git"`, "] Git  ")
		},
	)
}

// Formulario, búsqueda con Enter y salida con q, en una pseudoconsola.
func (s *soak) caseInteractive() outcome {
	cmd := exec.Command(s.exe, "-p", s.tree)
	cmd.Env = s.env()
	return s.console(commandLine(s.exe, cmd.Args[1:]), cmd, func(c *console) error {
		for _, want := range []string{"Buscar", "Carpeta elegida"} {
			if err := c.await(want); err != nil {
				return err
			}
		}
		if err := c.press("cancion\r"); err != nil {
			return err
		}
		if err := c.await("200 carpetas encontradas"); err != nil {
			return err
		}
		if err := c.press("q"); err != nil {
			return err
		}
		return expectExit(c, 0)
	})
}

// fcd en shell, con los scripts instalados: busca, se elige con Enter y la
// terminal queda en la carpeta, aunque su nombre lleve acentos y &.
func (s *soak) caseFcd(shell string) outcome {
	dir := filepath.Dir(s.exe)
	want := filepath.Join(s.small, "Música", "Canción & co")
	pwd := filepath.Join(s.work, fmt.Sprintf("pwd-%d.txt", s.next()))
	defer os.Remove(pwd)
	cmd := fcdPowerShell(shell, dir, s.small, pwd)
	if shell == "cmd.exe" {
		cmd = fcdCmd(dir, s.small, pwd)
	}
	cmd.Env = s.env()
	return s.console(shell+": fcd cancion -p "+s.small, cmd, func(c *console) error {
		if err := c.await("1 carpeta encontrada"); err != nil {
			return err
		}
		if err := c.press("\r"); err != nil {
			return err
		}
		if err := expectExit(c, 0); err != nil {
			return err
		}
		got, err := pwdOf(pwd)
		if err != nil {
			return err
		}
		return sameDir(got, want)
	})
}

// El atajo fast.exe que instalan el asistente e install.ps1 busca igual.
func (s *soak) caseAlias() outcome {
	alias := filepath.Join(filepath.Dir(s.exe), "fast.exe")
	return s.exec(alias, nil, "cancion", "-p", s.tree).expect(0, foundAll)
}

// console ejecuta cmd en una pseudoconsola y lo maneja con drive. Si algo
// falla, el resultado lleva la pantalla de ese momento.
func (s *soak) console(command string, cmd *exec.Cmd, drive func(*console) error) outcome {
	o := outcome{command: command, code: -1}
	c, release, err := openConsole(cmd)
	if err != nil {
		o.problem = err.Error()
		return o
	}
	defer release()
	if err := drive(c); err != nil {
		o.problem = err.Error()
		o.screen = c.screen()
		return o
	}
	o.code = 0
	return o
}

// expectExit espera a que el programa de la consola termine con want.
func expectExit(c *console, want int) error {
	code, err := c.exitCode()
	if err != nil {
		return err
	}
	if code != want {
		return fmt.Errorf("terminó con el código %d, se esperaba %d", code, want)
	}
	return nil
}

// processMemoryCounters es PROCESS_MEMORY_COUNTERS de psapi.h.
type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

var procGetProcessMemoryInfo = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

// watchMemory abre el proceso p para leer, cuando termine, su memoria máxima
// (PeakWorkingSetSize). Hay que abrirlo antes de esperarlo: al esperarlo, Go
// cierra su handle y el proceso deja de existir. Devuelve 0 si no se puede
// medir.
func watchMemory(p *os.Process) func(*os.ProcessState) uint64 {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(p.Pid))
	if err != nil {
		return func(*os.ProcessState) uint64 { return 0 }
	}
	return func(*os.ProcessState) uint64 {
		defer windows.CloseHandle(h)
		var c processMemoryCounters
		c.cb = uint32(unsafe.Sizeof(c))
		ok, _, _ := procGetProcessMemoryInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&c)), uintptr(c.cb))
		if ok == 0 {
			return 0
		}
		return uint64(c.peakWorkingSetSize)
	}
}

// osDescription devuelve la versión de Windows.
func osDescription() string {
	v := windows.RtlGetVersion()
	return fmt.Sprintf("Windows %d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
}
