#!/usr/bin/env bash
# Junta en un solo informe en Markdown (por stdout) los informes de las
# máquinas de la prueba larga (soak.yml). Cada máquina deja informe.md y
# resumen.json en <carpeta>/<artefacto>/. jobs.tsv, opcional, tiene el nombre
# y la conclusión de cada job de máquina (de la API de GitHub): sirve para
# señalar las máquinas que no llegaron a dejar informe.
#
# Uso: soak-report.sh <carpeta> <versión> <minutos> <url-de-la-ejecución> [jobs.tsv]
#   bash .github/scripts/soak-report.sh informes v1.4.0 60 https://github.com/... jobs.tsv
set -euo pipefail

dir="$1"
tag="$2"
minutes="$3"
run_url="$4"
jobs="${5:-}"

shopt -s nullglob
summaries=("$dir"/*/resumen.json)
reports=("$dir"/*/informe.md)

# Todos los resúmenes en un solo array JSON, ordenados por máquina.
all=$(if ((${#summaries[@]})); then jq -s 'sort_by(.machine)' "${summaries[@]}"; else echo '[]'; fi)

# Máquinas con job pero sin informe: no llegaron a arrancar, se cancelaron...
missing=()
if [[ -n $jobs && -f $jobs ]]; then
  while IFS=$'\t' read -r name conclusion; do
    [[ -z $name ]] && continue
    if ! jq -e --arg n "$name" 'any(.[]; .machine == $n)' <<<"$all" >/dev/null; then
      missing+=("$name"$'\t'"$conclusion")
    fi
  done <"$jobs"
fi

count() { jq --arg s "$1" '[.[] | select(.status == $s)] | length' <<<"$all"; }
ok=$(count "sin fallos")
failed=$(count "con fallos")
untested=$(count "sin prueba")

echo "# Prueba de ${minutes} minutos en varias máquinas"
echo
echo "- **Fecha:** $(date -u +%Y-%m-%d) (UTC)"
echo "- **Versión probada:** ${tag}"
echo "- **Ejecución en GitHub Actions:** ${run_url}"
echo "- **Máquinas:** sin fallos: ${ok} · con fallos: ${failed} · sin prueba: ${untested} · no disponibles: ${#missing[@]}"
echo
echo "## Resumen"
echo
echo "| Máquina | Sistema | Instalación | Estado | Ejecuciones | Fallos | Avisos |"
echo "|---|---|---|---|---:|---:|---:|"
jq -r '.[] | "| \(.machine) | \(.system) | \(.install) | \(
  {"sin fallos": "✅ sin fallos", "con fallos": "❌ con fallos"}[.status] // "⛔ \(.status)"
) | \(.runs) | \(.failures) | \(.warnings) |"' <<<"$all"
for m in "${missing[@]}"; do
  echo "| ${m%%$'\t'*} | — | — | ⚪ no disponible (${m#*$'\t'}) | — | — | — |"
done

echo
echo "## Fallos por caso"
echo
jq -r '
  [.[] as $m | $m.failed[] | . + {machine: $m.machine}]
  | if length == 0 then "Ninguna máquina registró fallos."
    else group_by(.scenario)[]
      | (length) as $n
      | ([.[].machine] | unique | length) as $machines
      | "### \(.[0].scenario): \($n) \(if $n == 1 then "fallo" else "fallos" end) en \($machines) \(if $machines == 1 then "máquina" else "máquinas" end)\n\n"
        + ([group_by(.machine)[]
            | "- **\(.[0].machine)** (\(length)), minutos \([.[].minute][:10] | join(", "))\(if length > 10 then ", …" else "" end): \(.[0].problem)"]
           | join("\n"))
        + "\n"
    end' <<<"$all"
if ((${#missing[@]})); then
  echo "Las máquinas no disponibles no dejaron informe: el runner no arrancó o el job se canceló. Ver la ejecución en GitHub Actions."
fi

# El informe de cada máquina, con su propio título de nivel 2.
for report in "${reports[@]}"; do
  echo
  echo "---"
  echo
  cat "$report"
done
