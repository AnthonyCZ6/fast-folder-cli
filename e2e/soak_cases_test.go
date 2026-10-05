//go:build soak

package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist/jumplisttest"
)

// Lo que se espera del árbol de benchtree: 10 áreas de 20 proyectos (la
// mitad de Go y la mitad de Node.js), cada uno con src, docs/Canción y un
// node_modules de 25 paquetes con 2 archivos de 512 bytes.
const (
	treeProjects     = 200
	treePackages     = 5000 // carpetas pkg-NN en todo el árbol
	areaNodeModules  = 20   // node_modules en un área
	nodeModulesBytes = 25600
	nodeModulesFiles = 50
	// foundAll es el resumen de una búsqueda que encuentra una carpeta por
	// proyecto (src, Canción...).
	foundAll = "Resultados : 200 carpetas encontradas"
	// maxRecentFolders es el máximo de carpetas recientes (maxRecent en
	// internal/config).
	maxRecentFolders = 20
)

// commonScenarios son los casos de la línea de comandos, en todos los
// sistemas.
func commonScenarios() []scenario {
	return []scenario{
		{"version", (*soak).caseVersion},
		{"acentos", (*soak).caseAccents},
		{"muchos-resultados", (*soak).caseManyResults},
		{"proyectos", (*soak).caseProjects},
		{"comodines", (*soak).caseWildcard},
		{"excluir", (*soak).caseExclude},
		{"modificadas", (*soak).caseModified},
		{"sin-resultados", (*soak).caseNoMatch},
		{"json", (*soak).caseJSON},
		{"tamaño", (*soak).caseSize},
		{"errores-de-uso", (*soak).caseUsageErrors},
		{"configuración", (*soak).caseConfig},
		{"recientes", (*soak).caseRecent},
	}
}

func (s *soak) caseVersion() outcome {
	o := s.cli("--version").expect(0, "fast-folder-cli ")
	if got := strings.TrimSpace(o.stdout); s.version != "" && got != "fast-folder-cli "+s.version {
		o = o.fail("--version muestra %q, se esperaba la versión %s", got, s.version)
	}
	return o
}

// La salida redirigida no lleva colores, y los nombres con acentos llegan en
// UTF-8 aunque se busquen sin acentos.
func (s *soak) caseAccents() outcome {
	o := s.cli("cancion", "-p", s.tree).expect(0,
		foundAll,
		filepath.Join(s.tree, "area-00", "Proyecto-000", "docs", "Canción"),
	)
	if strings.Contains(o.stdout, "\x1b[") {
		o = o.fail("la salida redirigida lleva colores")
	}
	return o
}

// Miles de resultados seguidos: no se pierde ni se repite ninguna línea.
func (s *soak) caseManyResults() outcome {
	o := s.cli("pkg", "-p", s.tree).expect(0, "Resultados : 5,000 carpetas encontradas")
	if n := strings.Count(o.stdout, string(filepath.Separator)+"pkg-"); n != treePackages {
		o = o.fail("se mostraron %d carpetas, se esperaban %d", n, treePackages)
	}
	return o
}

func (s *soak) caseProjects() outcome {
	return s.cli("--projects", "-p", s.tree).expect(0,
		"Resultados : 200 proyectos encontrados", "  (Go)", "  (Node.js)")
}

func (s *soak) caseWildcard() outcome {
	return s.cli("Proyecto-00?", "-p", s.tree).expect(0, "Resultados : 10 carpetas encontradas")
}

func (s *soak) caseExclude() outcome {
	return steps(
		func() outcome {
			return s.cli("src", "--exclude", "area-00,area-01", "-p", s.tree).expect(0, "Resultados : 160 carpetas encontradas")
		},
		func() outcome {
			return s.cli("pkg-01", "--exclude", "node_modules", "-p", s.tree).expect(1, "sin coincidencias")
		},
	)
}

// El árbol se crea al empezar la prueba: todas sus carpetas son de los
// últimos 3 días, aunque la prueba pase de un día al siguiente.
func (s *soak) caseModified() outcome {
	return s.cli("src", "-m", "3d", "-p", s.tree).expect(0, foundAll)
}

func (s *soak) caseNoMatch() outcome {
	return s.cli("nada-coincide-xyz", "-p", s.tree).expect(1, "sin coincidencias")
}

// --json: una línea JSON válida por carpeta, con su tipo de proyecto.
func (s *soak) caseJSON() outcome {
	return steps(
		func() outcome {
			o := s.cli("proyecto", "--json", "-p", s.tree).expect(0)
			items, err := jsonFolders(o.stdout)
			switch {
			case err != nil:
				return o.fail("%v", err)
			case len(items) != treeProjects:
				return o.fail("%d líneas JSON, se esperaban %d", len(items), treeProjects)
			}
			for _, it := range items {
				if !strings.HasPrefix(it.Name, "Proyecto-") || !strings.HasPrefix(it.Path, s.tree) {
					return o.fail("carpeta inesperada: %+v", it)
				}
			}
			return o
		},
		func() outcome {
			o := s.cli("--projects", "--json", "-p", s.tree).expect(0)
			items, err := jsonFolders(o.stdout)
			if err != nil {
				return o.fail("%v", err)
			}
			kinds := map[string]int{}
			for _, it := range items {
				kinds[it.Project]++
			}
			if len(items) != treeProjects || kinds["Go"] != treeProjects/2 || kinds["Node.js"] != treeProjects/2 {
				return o.fail("tipos de proyecto %v, se esperaban %d de Go y %d de Node.js", kinds, treeProjects/2, treeProjects/2)
			}
			return o
		},
	)
}

// --size mide cada node_modules de un área, con y sin --json.
func (s *soak) caseSize() outcome {
	area := filepath.Join(s.tree, "area-00")
	return steps(
		func() outcome {
			o := s.cli("node_modules", "--size", "--json", "-p", area).expect(0)
			items, err := jsonFolders(o.stdout)
			switch {
			case err != nil:
				return o.fail("%v", err)
			case len(items) != areaNodeModules:
				return o.fail("%d líneas JSON, se esperaban %d", len(items), areaNodeModules)
			}
			for _, it := range items {
				if it.Bytes == nil || *it.Bytes != nodeModulesBytes || it.Files == nil || *it.Files != nodeModulesFiles {
					return o.fail("%s: %v bytes y %v archivos, se esperaban %d y %d",
						it.Path, deref(it.Bytes), deref(it.Files), nodeModulesBytes, nodeModulesFiles)
				}
			}
			return o
		},
		func() outcome {
			return s.cli("node_modules", "--size", "-p", area).expect(0, "500 KB en total (1,000 archivos)")
		},
	)
}

// Opciones no válidas: código 2 y un mensaje por stderr. La ayuda, con 0.
func (s *soak) caseUsageErrors() outcome {
	tests := []struct {
		args []string
		code int
	}{
		{[]string{"-x"}, 2},
		{[]string{"-a"}, 2},
		{[]string{"-n", "x", "-p", filepath.Join(s.tree, "no-existe")}, 2},
		{[]string{"-m", "mañana", "-p", s.tree}, 2},
		{[]string{"--apps", "-p", s.tree}, 2},
		{[]string{"--help"}, 0},
		{nil, 0}, // sin terminal: la ayuda
	}
	fns := make([]func() outcome, len(tests))
	for i, tt := range tests {
		fns[i] = func() outcome {
			o := s.cli(tt.args...).expect(tt.code)
			switch {
			case tt.code == 0 && !strings.Contains(o.stdout, "Uso:"):
				o = o.fail("no mostró la ayuda")
			case tt.code != 0 && strings.TrimSpace(o.stderr) == "":
				o = o.fail("terminó con %d sin explicar el error", tt.code)
			}
			return o
		}
	}
	return steps(fns...)
}

// --config crea la plantilla una sola vez; la ubicación y las carpetas
// excluidas del archivo se aplican, y un archivo no válido se avisa y se
// ignora.
func (s *soak) caseConfig() outcome {
	dir := filepath.Join(s.work, "config-"+strconv.Itoa(s.next()))
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "config.toml")
	env := []string{"FFC_CONFIG=" + path}
	run := func(args ...string) outcome { return s.exec(s.exe, env, args...) }
	write := func(content string) func() outcome {
		return func() outcome {
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				return outcome{code: -1, problem: fmt.Sprintf("no se pudo escribir %s: %v", path, err)}
			}
			return outcome{}
		}
	}
	return steps(
		func() outcome { return run("--config").expect(0, "Configuración creada con la plantilla: "+path) },
		func() outcome { return run("--config").expect(0, "Configuración: "+path) },
		func() outcome { return run("src", "-p", s.tree).expect(0, foundAll) },
		write("ubicacion = '"+s.tree+"'\nexcluir = [\"node_modules\"]\n"),
		func() outcome { return run("src").expect(0, foundAll) },
		func() outcome { return run("pkg-01").expect(1, "sin coincidencias") },
		write("terminal = \"konsole\"\n"),
		func() outcome {
			o := run("src", "-p", s.tree).expect(0, foundAll)
			if !strings.Contains(o.stderr, "aviso: se ignora la configuración") {
				o = o.fail("no avisó de que la configuración no es válida")
			}
			return o
		},
	)
}

// Historial sintético de la prueba larga: Word abrió un documento en la
// carpeta docs de los primeros soakWordDocs proyectos y el Explorador abrió
// la carpeta src de los soakExplorerDirs siguientes.
const (
	soakWordDocs     = 20
	soakExplorerDirs = 10
)

// writeSoakHistory escribe en dir las jump lists del historial sintético,
// con rutas del árbol tree de benchtree.
func writeSoakHistory(dir, tree string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	project := func(n int) string {
		return filepath.Join(tree, fmt.Sprintf("area-%02d", n/20), fmt.Sprintf("Proyecto-%03d", n))
	}
	now := time.Now()
	var word, explorer []jumplisttest.Entry
	for i := range soakWordDocs {
		word = append(word, jumplisttest.Entry{
			Path: filepath.Join(project(i), "docs", "informe.docx"), LastUsed: now.Add(-time.Duration(i) * time.Minute),
		})
	}
	for i := range soakExplorerDirs {
		explorer = append(explorer, jumplisttest.Entry{
			Path: filepath.Join(project(soakWordDocs+i), "src"), LastUsed: now.Add(-time.Duration(i) * time.Hour),
		})
	}
	if err := jumplisttest.Write(dir, jumplist.AppID("Microsoft.Office.WINWORD.EXE.15"), 6, word); err != nil {
		return err
	}
	return jumplisttest.Write(dir, jumplist.AppID("Microsoft.Windows.Explorer"), 4, explorer)
}

// --recientes y --con con el historial sintético: todas las carpetas, las
// de un nombre y, en JSON, las de Word con el nombre del programa.
func (s *soak) caseRecent() outcome {
	env := []string{jumplist.EnvDir + "=" + s.history}
	run := func(args ...string) outcome { return s.exec(s.exe, env, args...) }
	all := fmt.Sprintf("Resultados : %d carpetas encontradas", soakWordDocs+soakExplorerDirs)
	return steps(
		func() outcome { return run("--recientes", "-p", s.tree).expect(0, all, "· Word)") },
		func() outcome {
			return run("--recientes", "src", "-p", s.tree).expect(0, fmt.Sprintf("Resultados : %d carpetas encontradas", soakExplorerDirs))
		},
		func() outcome {
			o := run("--con", "word", "--json", "-p", s.tree).expect(0)
			lines := nonEmptyLines(o.stdout)
			if len(lines) != soakWordDocs {
				return o.fail("%d líneas JSON, se esperaban %d", len(lines), soakWordDocs)
			}
			for _, line := range lines {
				var f recentJSON
				if err := json.Unmarshal([]byte(line), &f); err != nil {
					return o.fail("línea JSON no válida (%v): %s", err, line)
				}
				if filepath.Base(f.Path) != "docs" || f.Files != 1 || len(f.Apps) != 1 || f.Apps[0] != "Word" {
					return o.fail("carpeta inesperada: %+v", f)
				}
			}
			return o
		},
		func() outcome {
			o := run("--con", "photoshop").expect(2)
			if !strings.Contains(o.stderr, `no se reconoce el programa "photoshop"`) {
				o = o.fail("no dijo que no reconoce el programa")
			}
			return o
		},
	)
}

// jsonFolder es una línea de la salida --json de una búsqueda de carpetas.
type jsonFolder struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Project string `json:"project"`
	Bytes   *int64 `json:"bytes"`
	Files   *int64 `json:"files"`
}

// jsonFolders interpreta la salida --json: una carpeta por línea.
func jsonFolders(out string) ([]jsonFolder, error) {
	var items []jsonFolder
	for i, line := range nonEmptyLines(out) {
		var f jsonFolder
		if err := json.Unmarshal([]byte(line), &f); err != nil {
			return nil, fmt.Errorf("la línea %d no es JSON válido (%v): %s", i+1, err, line)
		}
		items = append(items, f)
	}
	return items, nil
}

// nonEmptyLines devuelve las líneas de out que no están vacías.
func nonEmptyLines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func deref(p *int64) any {
	if p == nil {
		return "(ninguno)"
	}
	return *p
}

// checkRecent comprueba que la lista de carpetas recientes, si existe, es
// válida y no pasa del máximo, por muchas carpetas que se elijan.
func checkRecent(path string) string {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		return fmt.Sprintf("no se pudo leer %s: %v", path, err)
	}
	var dirs []string
	if err := json.Unmarshal(data, &dirs); err != nil {
		return fmt.Sprintf("recientes.json está dañado: %v", err)
	}
	if len(dirs) > maxRecentFolders {
		return fmt.Sprintf("recientes.json tiene %d carpetas (máximo %d)", len(dirs), maxRecentFolders)
	}
	return ""
}
