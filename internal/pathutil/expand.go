// Package pathutil resuelve las rutas que el usuario escribe en la línea de
// comandos: variables de entorno al estilo Windows (%APPDATA%), el atajo "~"
// y unidades sin barra final ("D:").
package pathutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// envRef captura referencias del tipo %NOMBRE%.
var envRef = regexp.MustCompile(`%([^%\s]+)%`)

// ExpandEnv sustituye cada %VARIABLE% por su valor, igual que cmd.exe:
// %APPDATA%, %LOCALAPPDATA%, %USERPROFILE%, %PROGRAMDATA%, etc. En Windows los
// nombres no distinguen mayúsculas. Las variables inexistentes se dejan tal
// cual para que el error posterior muestre qué no se pudo resolver.
//
// Esto es necesario porque PowerShell, a diferencia de cmd.exe, no expande
// la sintaxis %VAR% antes de pasar los argumentos al programa.
func ExpandEnv(s string) string {
	return envRef.ReplaceAllStringFunc(s, func(ref string) string {
		name := ref[1 : len(ref)-1]
		if v, ok := os.LookupEnv(name); ok {
			return v
		}
		// Fuera de Windows no existe USERPROFILE: se usa el directorio
		// personal para que el valor por defecto de --path funcione igual.
		if strings.EqualFold(name, "USERPROFILE") {
			if home, err := os.UserHomeDir(); err == nil {
				return home
			}
		}
		return ref
	})
}

// Resolve convierte la ruta indicada por el usuario en una ruta absoluta y
// limpia, y comprueba que exista y sea una carpeta.
func Resolve(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	// En cmd.exe, "C:\Program Files\" llega como `C:\Program Files"` porque
	// la barra final escapa la comilla. Se eliminan las comillas sobrantes.
	p = strings.Trim(p, `"'`)
	if p == "" {
		return "", errors.New("la ruta está vacía")
	}

	p = ExpandEnv(p)
	p = expandHome(p)

	// "D:" significa "directorio actual de la unidad D", no su raíz.
	if vol := filepath.VolumeName(p); vol != "" && vol == p {
		p += string(filepath.Separator)
	}

	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("ruta inválida %q: %w", raw, err)
	}

	info, err := os.Stat(abs)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if ref := envRef.FindString(p); ref != "" {
			return "", fmt.Errorf("variable de entorno no definida: %s", ref)
		}
		return "", fmt.Errorf("la ruta no existe: %s", abs)
	case err != nil:
		return "", fmt.Errorf("no se puede acceder a %s: %w", abs, err)
	case !info.IsDir():
		return "", fmt.Errorf("la ruta no es una carpeta: %s", abs)
	}
	return abs, nil
}

// expandHome reemplaza un "~" inicial por el directorio personal.
func expandHome(p string) string {
	if p != "~" && !strings.HasPrefix(p, `~\`) && !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return home + p[1:]
}
