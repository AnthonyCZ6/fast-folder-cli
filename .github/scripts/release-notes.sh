#!/usr/bin/env bash
# Genera en Markdown las notas de una versión a partir de los mensajes de
# commit (Conventional Commits) entre la etiqueta indicada y la anterior.
#
# Uso: release-notes.sh <etiqueta> <url-del-repositorio>
#   bash .github/scripts/release-notes.sh v1.1.0 https://github.com/AnthonyCZ6/fast-folder-cli
set -euo pipefail

tag="$1"
repo_url="$2"
module=$(awk '/^module /{print $2}' go.mod)

if prev=$(git describe --tags --abbrev=0 "$tag^" 2>/dev/null); then
  range="$prev..$tag"
  changes_url="$repo_url/compare/$prev...$tag"
else
  # Primera versión: se incluye todo el historial.
  range="$tag"
  changes_url="$repo_url/commits/$tag"
fi

subjects=$(git log --no-merges --pretty='%s (%h)' "$range")
types='feat|fix|perf|docs|build|ci|chore|refactor|test|style|revert'

# commits imprime como lista los commits cuyo tipo coincide con el patrón.
# Con -v imprime los que NO coinciden.
commits() {
  grep -E "$@" <<<"$subjects" | sed 's/^/- /' || true
}

# section imprime una sección con título si contiene algún commit.
section() {
  local title="$1" lines
  shift
  lines=$(commits "$@")
  if [[ -n "$lines" ]]; then
    printf '### %s\n\n%s\n\n' "$title" "$lines"
  fi
}

type_re() {
  echo "^($1)(\([^)]*\))?!?: "
}

echo "## 📝 Cambios"
echo
section "✨ Nuevas funciones" "$(type_re feat)"
section "🐛 Correcciones" "$(type_re fix)"
section "⚡ Rendimiento" "$(type_re perf)"
section "📚 Documentación" "$(type_re docs)"
section "🔧 Mantenimiento" "$(type_re 'build|ci|chore|refactor|test|style|revert')"
section "Otros cambios" -v -e "$(type_re "$types")" -e '^$'

sed -e "s|{{TAG}}|$tag|g" \
  -e "s|{{REPO_URL}}|$repo_url|g" \
  -e "s|{{MODULE}}|$module|g" \
  -e "s|{{CHANGES_URL}}|$changes_url|g" <<'EOF'
## 📥 Descarga

| Equipo | Archivo |
| --- | --- |
| Windows x64 (Intel o AMD, la mayoría de los equipos) | [fast-folder-cli-windows-amd64.exe]({{REPO_URL}}/releases/download/{{TAG}}/fast-folder-cli-windows-amd64.exe) |
| Windows ARM64 (Snapdragon, Surface Pro X) | [fast-folder-cli-windows-arm64.exe]({{REPO_URL}}/releases/download/{{TAG}}/fast-folder-cli-windows-arm64.exe) |

1. Descarga el archivo de tu equipo y renómbralo a `fast-folder-cli.exe`.
2. Agrégalo al PATH siguiendo [las instrucciones del README]({{REPO_URL}}#agregar-al-path-de-windows).

¿Tienes Go 1.23 o superior? Instálalo en un solo paso:

```powershell
go install {{MODULE}}@{{TAG}}
```

## 🔒 Verificación

Los ejecutables no están firmados digitalmente, así que Windows SmartScreen o tu antivirus pueden mostrar una advertencia la primera vez. Comprueba que el archivo es el original comparando su SHA-256 con `checksums.txt`:

```powershell
Get-FileHash .\fast-folder-cli-windows-amd64.exe
```

**Cambios completos**: {{CHANGES_URL}}
EOF
