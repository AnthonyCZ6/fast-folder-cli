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
# https://github.com/usuario/repo -> https://raw.githubusercontent.com/usuario/repo
raw_url="https://raw.githubusercontent.com/${repo_url#https://github.com/}"

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
  -e "s|{{RAW_URL}}|$raw_url|g" \
  -e "s|{{MODULE}}|$module|g" \
  -e "s|{{CHANGES_URL}}|$changes_url|g" <<'EOF'
## 📥 Instalación

**Asistente de instalación:** descarga
[fast-folder-cli-setup.exe]({{REPO_URL}}/releases/download/{{TAG}}/fast-folder-cli-setup.exe),
ábrelo y sigue los pasos. No requiere permisos de administrador, instala la versión
adecuada para tu equipo (x64 o ARM64), agrega `fast-folder-cli` al PATH con el atajo
`fast` y se puede desinstalar desde **Configuración → Aplicaciones**. Después, abre una
terminal nueva y escribe `fast --help`.

**Desde la terminal:** abre PowerShell y ejecuta (instala o actualiza):

```powershell
irm {{RAW_URL}}/main/install.ps1 | iex
```

El script detecta si tu equipo es x64 o ARM64, verifica la suma SHA-256 y agrega
`fast-folder-cli` al PATH con el atajo `fast`. Para instalar exactamente esta versión:

```powershell
& ([scriptblock]::Create((irm {{RAW_URL}}/main/install.ps1))) -Version {{TAG}}
```

¿Tienes Go 1.26 o superior? `go install {{MODULE}}@{{TAG}}`

<details>
<summary>Descarga manual</summary>

| Equipo | Archivo |
| --- | --- |
| Windows x64 (Intel o AMD, la mayoría de los equipos) | [fast-folder-cli-windows-amd64.exe]({{REPO_URL}}/releases/download/{{TAG}}/fast-folder-cli-windows-amd64.exe) |
| Windows ARM64 (Snapdragon, Surface Pro X) | [fast-folder-cli-windows-arm64.exe]({{REPO_URL}}/releases/download/{{TAG}}/fast-folder-cli-windows-arm64.exe) |

Renómbralo a `fast-folder-cli.exe` y agrégalo al PATH siguiendo
[las instrucciones del README]({{REPO_URL}}#agregar-al-path-de-windows).

</details>

## 🔒 Verificación

Los ejecutables no están firmados digitalmente, así que Windows SmartScreen puede mostrar "Windows protegió su PC" al abrir el asistente: pulsa **Más información → Ejecutar de todas formas**. Con Smart App Control activado (Windows 11), Windows bloquea los ejecutables sin esa opción: consulta [Si Windows bloquea el programa]({{REPO_URL}}#si-windows-bloquea-el-programa). El script de PowerShell comprueba la suma SHA-256 automáticamente; si descargas un archivo a mano, compáralo con `checksums.txt`:

```powershell
Get-FileHash .\fast-folder-cli-windows-amd64.exe
```

## 🔏 Code signing policy

Free code signing provided by [SignPath.io](https://about.signpath.io), certificate by [SignPath Foundation](https://signpath.org). Consulta la [política de firma de código completa]({{REPO_URL}}#code-signing-policy).

**Cambios completos**: {{CHANGES_URL}}
EOF
