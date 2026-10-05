//go:build !windows

package jumplist

import "errors"

// Fuera de Windows no hay jump lists: solo se pueden leer las de EnvDir (en
// las pruebas).

func defaultDir() (string, error) {
	return "", errors.New("las jump lists solo existen en Windows")
}

func knownFolder(string) string { return "" }

// HistoryOff siempre devuelve "": fuera de Windows no hay ajuste que mirar.
func HistoryOff() string { return "" }
