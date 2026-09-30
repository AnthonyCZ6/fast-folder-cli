<#
.SYNOPSIS
    Instala, actualiza o desinstala fast-folder-cli en Windows.

.DESCRIPTION
    Descarga el ejecutable de fast-folder-cli adecuado para tu equipo (x64 o
    ARM64) desde los releases de GitHub, verifica su suma SHA-256, lo copia en
    %LOCALAPPDATA%\Programs\fast-folder-cli y agrega esa carpeta al PATH de tu
    usuario. No requiere permisos de administrador.

    Volver a ejecutarlo actualiza fast-folder-cli a la última versión.

.PARAMETER Version
    Versión a instalar (por ejemplo, v1.0.0). Por defecto, la más reciente.

.PARAMETER InstallDir
    Carpeta de instalación. Por defecto, %LOCALAPPDATA%\Programs\fast-folder-cli.

.PARAMETER Uninstall
    Elimina fast-folder-cli y quita su carpeta del PATH.

.EXAMPLE
    irm https://raw.githubusercontent.com/AnthonyCZ6/fast-folder-cli/main/install.ps1 | iex

    Instala o actualiza a la última versión.

.EXAMPLE
    & ([scriptblock]::Create((irm https://raw.githubusercontent.com/AnthonyCZ6/fast-folder-cli/main/install.ps1))) -Version v1.0.0

    Instala una versión concreta.

.EXAMPLE
    & ([scriptblock]::Create((irm https://raw.githubusercontent.com/AnthonyCZ6/fast-folder-cli/main/install.ps1))) -Uninstall

    Desinstala fast-folder-cli.
#>
# Nota: el archivo se guarda en UTF-8 sin BOM. Con BOM, "irm | iex" falla porque
# Invoke-RestMethod no lo elimina y el bloque param deja de reconocerse.
param(
    [string]$Version = 'latest',
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\fast-folder-cli",
    [switch]$Uninstall
)

# Todo se ejecuta en un bloque propio para no dejar funciones ni preferencias
# modificadas en la sesión del usuario al usar "irm ... | iex".
& {
    $ErrorActionPreference = 'Stop'
    # La barra de progreso hace muy lento Invoke-WebRequest en PowerShell 5.1.
    $ProgressPreference = 'SilentlyContinue'

    $repo = 'AnthonyCZ6/fast-folder-cli'
    $exe = Join-Path $InstallDir 'fast-folder-cli.exe'

    function Write-Step([string]$Message) { Write-Host "  $Message" }
    function Write-Done([string]$Message) { Write-Host "  $Message" -ForegroundColor Green }

    function Get-Arch {
        $arch = ''
        try { $arch = [string][System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture } catch { }
        if (-not $arch) {
            $arch = $env:PROCESSOR_ARCHITEW6432
            if (-not $arch) { $arch = $env:PROCESSOR_ARCHITECTURE }
        }
        switch ($arch) {
            { $_ -in 'X64', 'AMD64' } { return 'amd64' }
            'Arm64' { return 'arm64' }
        }
        throw "Arquitectura no compatible ($arch): fast-folder-cli requiere Windows de 64 bits (x64 o ARM64)."
    }

    # Lee el PATH del usuario desde el registro sin expandir sus variables
    # (%USERPROFILE%, %JAVA_HOME%...) para no alterar las demás entradas al
    # reescribirlo, algo que sí ocurre con [Environment]::SetEnvironmentVariable.
    function Get-UserPath {
        $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
        try {
            $kind = [Microsoft.Win32.RegistryValueKind]::ExpandString
            if ($key.GetValueNames() -contains 'Path') { $kind = $key.GetValueKind('Path') }
            [pscustomobject]@{
                Value = [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
                Kind  = $kind
            }
        } finally {
            $key.Close()
        }
    }

    function Set-UserPath([string]$Value, [Microsoft.Win32.RegistryValueKind]$Kind) {
        $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
        try { $key.SetValue('Path', $Value, $Kind) } finally { $key.Close() }
        # Notifica el cambio a Windows (WM_SETTINGCHANGE) para que las
        # terminales que se abran a partir de ahora usen el PATH nuevo.
        [Environment]::SetEnvironmentVariable('FAST_FOLDER_CLI_TMP', '1', 'User')
        [Environment]::SetEnvironmentVariable('FAST_FOLDER_CLI_TMP', $null, 'User')
    }

    function Test-SameDir([string]$Entry) {
        $expanded = [Environment]::ExpandEnvironmentVariables($Entry).TrimEnd('\')
        $expanded -ne '' -and $expanded -ieq $InstallDir.TrimEnd('\')
    }

    # Descarga un archivo reintentando ante fallos transitorios de red (por
    # ejemplo, conexiones reutilizadas que el servidor ya cerró). Un 404 indica
    # que la versión no existe y no se reintenta.
    function Save-Url([string]$Uri, [string]$OutFile) {
        for ($attempt = 1; ; $attempt++) {
            try {
                Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $OutFile
                return
            } catch {
                $status = 0
                if ($_.Exception.Response) { $status = [int]$_.Exception.Response.StatusCode }
                if ($status -eq 404) {
                    throw "No se encontró la versión '$Version'. Consulta las disponibles en https://github.com/$repo/releases"
                }
                if ($attempt -ge 3) {
                    throw "No se pudo descargar fast-folder-cli: $($_.Exception.Message)"
                }
                Start-Sleep -Seconds $attempt
            }
        }
    }

    function Get-InstalledVersion {
        try { ((& $exe --version) -replace '^fast-folder-cli\s+', '').Trim() } catch { $null }
    }

    # ---------------------------------------------------------------- desinstalar
    if ($Uninstall) {
        Write-Host ''
        Write-Host 'Desinstalando fast-folder-cli...' -ForegroundColor Cyan

        if (Test-Path -LiteralPath $exe) {
            Remove-Item -LiteralPath $exe -Force
            Write-Step "Eliminado $exe"
        } else {
            Write-Step "No se encontró $exe"
        }
        # Por seguridad, la carpeta solo se borra si quedó vacía.
        if ((Test-Path -LiteralPath $InstallDir) -and -not (Get-ChildItem -LiteralPath $InstallDir -Force)) {
            Remove-Item -LiteralPath $InstallDir -Force
        }

        $userPath = Get-UserPath
        $entries = $userPath.Value -split ';'
        $kept = @($entries | Where-Object { -not (Test-SameDir $_) })
        if ($kept.Count -ne $entries.Count) {
            Set-UserPath ($kept -join ';') $userPath.Kind
            Write-Step "Se quitó $InstallDir del PATH de tu usuario."
        }
        $env:Path = @($env:Path -split ';' | Where-Object { -not (Test-SameDir $_) }) -join ';'

        Write-Host ''
        Write-Done 'fast-folder-cli se desinstaló correctamente.'
        return
    }

    # ------------------------------------------------------------------ instalar
    $arch = Get-Arch
    if ($Version -ne 'latest' -and $Version -notlike 'v*') { $Version = "v$Version" }
    if ($Version -eq 'latest') {
        $baseUrl = "https://github.com/$repo/releases/latest/download"
        $label = 'última versión'
    } else {
        $baseUrl = "https://github.com/$repo/releases/download/$Version"
        $label = $Version
    }
    $asset = "fast-folder-cli-windows-$arch.exe"

    Write-Host ''
    Write-Host 'Instalando fast-folder-cli...' -ForegroundColor Cyan

    # GitHub exige TLS 1.2, que Windows PowerShell 5.1 no siempre activa.
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $previous = $null
    if (Test-Path -LiteralPath $exe) { $previous = Get-InstalledVersion }

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ('fast-folder-cli-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        $tmpExe = Join-Path $tmp $asset
        $tmpSums = Join-Path $tmp 'checksums.txt'

        Write-Step "Descargando $asset ($label)..."
        Save-Url "$baseUrl/$asset" $tmpExe
        Save-Url "$baseUrl/checksums.txt" $tmpSums

        Write-Step 'Verificando la suma SHA-256...'
        $line = Get-Content -LiteralPath $tmpSums |
            Where-Object { ($_ -split '\s+')[-1].TrimStart('*') -eq $asset } |
            Select-Object -First 1
        if (-not $line) { throw "checksums.txt no contiene la suma de $asset." }
        $expected = ($line -split '\s+')[0]
        $actual = (Get-FileHash -LiteralPath $tmpExe -Algorithm SHA256).Hash
        if ($expected -ne $actual) {
            throw "La suma SHA-256 de $asset no coincide (esperada $expected, obtenida $actual). La descarga está dañada o fue alterada; no se instaló nada."
        }

        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
        try {
            Copy-Item -LiteralPath $tmpExe -Destination $exe -Force
        } catch {
            throw "No se pudo escribir $exe. Si fast-folder-cli se está ejecutando, ciérralo y vuelve a intentarlo."
        }
        Write-Step "Copiado en $exe"
    } finally {
        Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }

    $userPath = Get-UserPath
    if (-not @($userPath.Value -split ';' | Where-Object { Test-SameDir $_ }).Count) {
        if ($userPath.Value -eq '' -or $userPath.Value.EndsWith(';')) {
            $newValue = $userPath.Value + $InstallDir
        } else {
            $newValue = $userPath.Value + ';' + $InstallDir
        }
        Set-UserPath $newValue $userPath.Kind
        Write-Step "Se agregó $InstallDir al PATH de tu usuario."
    }
    # Disponible de inmediato en esta misma terminal.
    if (-not @($env:Path -split ';' | Where-Object { Test-SameDir $_ }).Count) {
        $env:Path = $env:Path.TrimEnd(';') + ';' + $InstallDir
    }

    $installed = Get-InstalledVersion
    $found = Get-Command fast-folder-cli -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($found -and -not (Test-SameDir (Split-Path -Parent $found.Source))) {
        Write-Warning "Hay otra copia de fast-folder-cli en $($found.Source) que tiene prioridad en el PATH."
    }

    Write-Host ''
    if ($previous -and $previous -ne $installed) {
        Write-Done "fast-folder-cli se actualizó de $previous a $installed."
    } elseif ($previous) {
        Write-Done "fast-folder-cli $installed ya estaba instalado y se reinstaló."
    } else {
        Write-Done "fast-folder-cli $installed se instaló correctamente."
    }
    Write-Host ''
    Write-Host '  Pruébalo:  ' -NoNewline
    Write-Host 'fast-folder-cli --help' -ForegroundColor Yellow
    Write-Host '  Si otra terminal no reconoce el comando, ciérrala y ábrela de nuevo.' -ForegroundColor DarkGray
    Write-Host ''
}
