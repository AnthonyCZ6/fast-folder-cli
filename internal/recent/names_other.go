//go:build !windows

package recent

// Fuera de Windows no hay menú Inicio ni carpetas conocidas: solo se usan los
// nombres de la tabla wellKnown.

func systemCandidates() []candidate { return nil }

func knownFolders() []knownFolder { return nil }
