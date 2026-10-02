package search

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// Tipos de proyecto, en el orden en que se muestran cuando una carpeta tiene
// indicios de varios.
const (
	kindGo = iota
	kindRust
	kindNode
	kindPython
	kindDotNet
	kindCpp
	kindJava
	kindKotlin
	kindPHP
	kindRuby
	kindFlutter
	kindUnity
	kindGit
	kindCount
)

var kindNames = [kindCount]string{
	"Go", "Rust", "Node.js", "Python", ".NET", "C/C++", "Java", "Kotlin",
	"PHP", "Ruby", "Flutter", "Unity", "Git",
}

// detectProject devuelve el tipo de proyecto de la carpeta cuyo contenido es
// entries ("Go", "Node.js, Python"...), o "" si no parece un proyecto. Una
// carpeta .git solo se muestra como "Git" si no hay indicios de un lenguaje.
func detectProject(entries []fs.DirEntry) string {
	var found uint32
	for _, e := range entries {
		if k := markerKind(strings.ToLower(e.Name())); k >= 0 {
			found |= 1 << k
		}
	}
	if found == 0 {
		return ""
	}
	if found != 1<<kindGit {
		found &^= 1 << kindGit
	}

	var kinds []string
	for k := range kindCount {
		if found&(1<<k) != 0 {
			kinds = append(kinds, kindNames[k])
		}
	}
	return strings.Join(kinds, ", ")
}

// markerKind devuelve el tipo de proyecto que indica un archivo o carpeta con
// ese nombre (en minúsculas), o -1 si no indica ninguno.
func markerKind(name string) int {
	switch name {
	case "go.mod":
		return kindGo
	case "cargo.toml":
		return kindRust
	case "package.json":
		return kindNode
	case "pyproject.toml", "requirements.txt", "setup.py", "pipfile":
		return kindPython
	case "cmakelists.txt":
		return kindCpp
	case "pom.xml", "build.gradle":
		return kindJava
	case "build.gradle.kts":
		return kindKotlin
	case "composer.json":
		return kindPHP
	case "gemfile":
		return kindRuby
	case "pubspec.yaml":
		return kindFlutter
	case "projectsettings":
		return kindUnity
	case ".git":
		return kindGit
	}
	switch filepath.Ext(name) {
	case ".sln", ".csproj":
		return kindDotNet
	case ".vcxproj":
		return kindCpp
	}
	return -1
}
