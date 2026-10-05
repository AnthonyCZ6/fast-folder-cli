//go:build !windows

package recent

// hiddenAttr: fuera de Windows no hay atributo de oculta; basta con el punto
// inicial del nombre, que ya comprueba isHidden.
func hiddenAttr(string) bool { return false }
