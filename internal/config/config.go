// Package config lee las preferencias de fast-folder-cli: un archivo TOML
// opcional (por defecto %APPDATA%\fast-folder-cli\config.toml) que cambia los
// valores de siempre. Sin archivo, todo funciona como si no existiera.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// EnvVar es la variable de entorno que indica otro archivo de configuración
// (por ejemplo, en las pruebas).
const EnvVar = "FFC_CONFIG"

// DefaultRecent es cuántas carpetas recientes se ofrecen si el archivo no
// dice otra cosa.
const DefaultRecent = 5

// terminals son los valores válidos de la clave terminal.
var terminals = []string{"wt", "pwsh", "powershell", "cmd"}

// Config son las preferencias del usuario. El valor cero de cada campo deja
// el comportamiento de siempre.
type Config struct {
	Root      string     `toml:"ubicacion"`   // carpeta por defecto, como -p
	Exclude   []string   `toml:"excluir"`     // carpetas que no se muestran ni se recorren
	Editor    string     `toml:"editor"`      // orden de la tecla v; vacía: VS Code
	Terminal  string     `toml:"terminal"`    // terminal de la tecla t; vacía: pwsh o powershell
	Hidden    bool       `toml:"ocultas"`     // incluir las carpetas ocultas y de sistema
	Recent    *int       `toml:"recientes"`   // carpetas recientes del formulario; nil: DefaultRecent
	Locations []Location `toml:"ubicaciones"` // ubicaciones propias del formulario
}

// Location es una ubicación propia del formulario.
type Location struct {
	Name string `toml:"nombre"`
	Path string `toml:"ruta"`
}

// RecentLimit devuelve cuántas carpetas recientes se ofrecen (0: ninguna).
func (c Config) RecentLimit() int {
	if c.Recent == nil {
		return DefaultRecent
	}
	return max(*c.Recent, 0)
}

// Path devuelve la ruta del archivo de configuración: la de FFC_CONFIG o, si
// no está definida, config.toml en la carpeta de configuración del usuario
// (%APPDATA%\fast-folder-cli en Windows).
func Path() (string, error) {
	if p := os.Getenv(EnvVar); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("no se encontró la carpeta de configuración: %w", err)
	}
	return filepath.Join(dir, "fast-folder-cli", "config.toml"), nil
}

// Load lee el archivo de configuración de Path (ver LoadFile).
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	return LoadFile(path)
}

// LoadFile lee y valida el archivo path. Si no existe, devuelve una Config
// vacía sin error. Si no se puede leer o tiene errores, devuelve una Config
// vacía junto con el error: quien llama avisa y sigue con los valores de
// siempre, porque un archivo mal escrito no debe dejar el programa inservible.
func LoadFile(path string) (Config, error) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	var c Config
	md, err := toml.DecodeFile(path, &c)
	if err == nil {
		err = validate(c, md)
	}
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	c.Terminal = strings.ToLower(c.Terminal)
	return c, nil
}

// validate comprueba lo que TOML no puede: claves desconocidas (casi siempre
// una errata), la terminal y las ubicaciones propias.
func validate(c Config, md toml.MetaData) error {
	if keys := md.Undecoded(); len(keys) > 0 {
		return fmt.Errorf("clave desconocida %q", keys[0].String())
	}
	if c.Terminal != "" && !slices.Contains(terminals, strings.ToLower(c.Terminal)) {
		return fmt.Errorf("terminal %q no válida: usa %s", c.Terminal, strings.Join(terminals, ", "))
	}
	for i, l := range c.Locations {
		if strings.TrimSpace(l.Name) == "" || strings.TrimSpace(l.Path) == "" {
			return fmt.Errorf("la ubicación %d necesita nombre y ruta", i+1)
		}
	}
	return nil
}

// Template es la plantilla comentada que crea fast --config: con todas las
// claves desactivadas, equivale a no tener archivo.
//
//go:embed plantilla.toml
var Template string

// Create escribe la plantilla en path, creando su carpeta, si el archivo no
// existe. Devuelve true si lo creó; un archivo que ya existe no se toca.
func Create(path string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := f.WriteString(Template); err != nil {
		f.Close()
		return false, err
	}
	return true, f.Close()
}
