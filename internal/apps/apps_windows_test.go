//go:build windows

package apps

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// testKey es la clave de HKCU donde las pruebas crean un registro falso. Se
// borra al terminar.
const testKey = `Software\fast-folder-cli-test`

// setValues crea la clave path de HKCU con los valores de texto strs, los
// de texto con variables expandibles exps y los números dwords.
func setValues(t *testing.T, path string, strs, exps map[string]string, dwords map[string]uint32) {
	t.Helper()
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		t.Fatalf("no se pudo crear %s: %v", path, err)
	}
	defer key.Close()
	for name, v := range strs {
		if err := key.SetStringValue(name, v); err != nil {
			t.Fatal(err)
		}
	}
	for name, v := range exps {
		if err := key.SetExpandStringValue(name, v); err != nil {
			t.Fatal(err)
		}
	}
	for name, v := range dwords {
		if err := key.SetDWordValue(name, v); err != nil {
			t.Fatal(err)
		}
	}
}

// deleteTree borra la clave path de HKCU con todas sus subclaves.
func deleteTree(t *testing.T, path string) {
	t.Helper()
	if key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS); err == nil {
		names, _ := key.ReadSubKeyNames(0)
		key.Close()
		for _, name := range names {
			deleteTree(t, path+`\`+name)
		}
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil && !errors.Is(err, registry.ErrNotExist) {
		t.Errorf("no se pudo borrar %s: %v", path, err)
	}
}

func TestReadRegistry(t *testing.T) {
	deleteTree(t, testKey) // restos de una ejecución interrumpida
	t.Cleanup(func() { deleteTree(t, testKey) })

	base := makeFiles(t, "App/app.exe", "Herramienta/tool.exe")
	t.Setenv("FFC_TEST_DIR", base)
	exe := filepath.Join(base, "App", "app.exe")

	uninstall := testKey + `\Uninstall`
	setValues(t, uninstall+`\MiApp`,
		map[string]string{"DisplayName": "Mi App", "InstallLocation": filepath.Join(base, "App"), "DisplayVersion": "1.0"},
		map[string]string{"DisplayIcon": `"%FFC_TEST_DIR%\App\app.exe",0`},
		nil)
	setValues(t, uninstall+`\Componente`,
		map[string]string{"DisplayName": "Componente", "InstallLocation": filepath.Join(base, "App")},
		nil, map[string]uint32{"SystemComponent": 1})
	setValues(t, uninstall+`\Parche`,
		map[string]string{"DisplayName": "Parche de Mi App", "ParentKeyName": "MiApp"},
		nil, nil)
	setValues(t, testKey+`\App Paths\tool.exe`,
		map[string]string{"": filepath.Join(base, "Herramienta", "tool.exe")},
		nil, nil)

	entries, err := readUninstall([]regKey{
		{registry.CURRENT_USER, uninstall},
		{registry.CURRENT_USER, testKey + `\NoExiste`}, // se omite
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("entradas = %+v, want 3", entries)
	}
	for _, e := range entries {
		switch e.name {
		case "Mi App":
			if e.displayIcon != `"`+exe+`",0` || e.version != "1.0" || e.hidden {
				t.Errorf("Mi App = %+v; el icono debería tener la variable expandida", e)
			}
		case "Componente", "Parche de Mi App":
			if !e.hidden {
				t.Errorf("%s debería estar oculta", e.name)
			}
		}
	}

	paths := readAppPaths([]regKey{{registry.CURRENT_USER, testKey + `\App Paths`}})
	if want := []string{filepath.Join(base, "Herramienta", "tool.exe")}; !slices.Equal(paths, want) {
		t.Errorf("App Paths = %q, want %q", paths, want)
	}

	want := []App{
		{Name: "Mi App", Dir: filepath.Join(base, "App"), Exe: exe, Version: "1.0"},
		{Name: "tool", Dir: filepath.Join(base, "Herramienta"), Exe: filepath.Join(base, "Herramienta", "tool.exe")},
	}
	if got := build(entries, paths, nil); !slices.Equal(got, want) {
		t.Errorf("build =\n%+v\nwant\n%+v", got, want)
	}
}

func TestReadUninstallFailsWithoutKeys(t *testing.T) {
	if _, err := readUninstall([]regKey{{registry.CURRENT_USER, testKey + `\NoExiste`}}); err == nil {
		t.Error("debería fallar si no puede leer ninguna clave")
	}
}

// Find lee el registro real. En cualquier Windows hay programas instalados, y
// cada uno debe tener una carpeta que exista.
func TestFindReadsTheRealRegistry(t *testing.T) {
	list, err := Find(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) == 0 {
		t.Fatal("no se encontró ninguna aplicación instalada")
	}
	for _, a := range list {
		if info, err := os.Stat(a.Dir); err != nil || !info.IsDir() {
			t.Errorf("%s: la carpeta %q no existe", a.Name, a.Dir)
		}
	}
	t.Logf("%d aplicaciones; la primera: %+v", len(list), list[0])
}
