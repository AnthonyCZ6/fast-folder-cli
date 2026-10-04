//go:build !windows

package apps

import "errors"

// Find solo funciona en Windows, que es donde se registran las aplicaciones
// instaladas.
func Find(func(string) bool) ([]App, error) {
	return nil, errors.New("la búsqueda de apps solo está disponible en Windows")
}
