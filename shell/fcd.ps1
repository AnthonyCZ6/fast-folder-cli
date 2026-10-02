<#
.SYNOPSIS
    Busca una carpeta con fast-folder-cli y entra en ella.

.DESCRIPTION
    Abre el modo interactivo de fast-folder-cli. Al elegir una carpeta con
    Enter, la terminal se cambia a esa carpeta. Acepta las mismas opciones que
    fast-folder-cli: fcd tesis, fcd -p D: proyecto, fcd --projects...

.EXAMPLE
    fcd tesis
#>
# Se instala junto a fast-folder-cli.exe. Al ejecutarse como script (y no
# como función), Set-Location cambia la carpeta de la sesión de PowerShell que
# lo llamó. La carpeta elegida llega en un archivo temporal en UTF-8, así no
# importa la codificación de la consola y se conservan los acentos.

$exe = Join-Path $PSScriptRoot 'fast-folder-cli.exe'
$file = New-TemporaryFile
try {
    & $exe --cd-file $file.FullName @args
    $dir = Get-Content -LiteralPath $file.FullName -Raw -Encoding UTF8
    if ($dir) {
        Set-Location -LiteralPath $dir
    }
} finally {
    Remove-Item -LiteralPath $file.FullName -Force -ErrorAction SilentlyContinue
}
