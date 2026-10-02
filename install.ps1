<#
.SYNOPSIS
    Instala, actualiza o desinstala fast-folder-cli en Windows.

.DESCRIPTION
    Descarga el ejecutable de fast-folder-cli adecuado para tu equipo (x64 o
    ARM64) desde los releases de GitHub, verifica su suma SHA-256, lo copia en
    %LOCALAPPDATA%\Programs\fast-folder-cli (también como "fast.exe", un atajo
    más corto) y agrega esa carpeta al PATH de tu usuario. Desde la versión
    1.2.0 instala también el comando fcd (buscar una carpeta y entrar en ella)
    y la opción "Buscar carpetas aquí" en el menú contextual del Explorador.
    No requiere permisos de administrador.

    Volver a ejecutarlo actualiza fast-folder-cli a la última versión.

.PARAMETER Version
    Versión a instalar (por ejemplo, v1.0.0). Por defecto, la más reciente.

.PARAMETER InstallDir
    Carpeta de instalación. Por defecto, %LOCALAPPDATA%\Programs\fast-folder-cli.

.PARAMETER SourceDir
    Instala desde una carpeta con los archivos del release ya descargados
    (fast-folder-cli-windows-*.exe, checksums.txt, fcd.ps1 y fcd.cmd) en lugar
    de descargarlos: sirve para instalar sin conexión o probar una compilación.

.PARAMETER NoContextMenu
    No agrega la opción "Buscar carpetas aquí" al menú contextual del
    Explorador (y la quita si ya estaba).

.PARAMETER Uninstall
    Elimina fast-folder-cli, quita su carpeta del PATH y su opción del menú
    contextual.

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
    [string]$SourceDir,
    [switch]$NoContextMenu,
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
    # Atajo "fast": copia del mismo ejecutable, válida en cualquier terminal.
    $alias = Join-Path $InstallDir 'fast.exe'
    # Comando fcd (buscar una carpeta y entrar en ella) para PowerShell y cmd.
    $scripts = 'fcd.ps1', 'fcd.cmd'
    # Opción "Buscar carpetas aquí" del menú contextual del Explorador: al
    # hacer clic derecho en el fondo de una carpeta y sobre una carpeta.
    $menuKeys = 'HKCU:\Software\Classes\Directory\Background\shell\fast-folder-cli',
        'HKCU:\Software\Classes\Directory\shell\fast-folder-cli'

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
    # que la versión no existe y no se reintenta; con -Optional (archivos que
    # las versiones antiguas no incluyen) devuelve $false en lugar de fallar.
    function Save-Url([string]$Uri, [string]$OutFile, [switch]$Optional) {
        if ($SourceDir) {
            $local = Join-Path $SourceDir (Split-Path -Leaf $Uri)
            if (Test-Path -LiteralPath $local) {
                Copy-Item -LiteralPath $local -Destination $OutFile
                return $true
            }
            if ($Optional) { return $false }
            throw "No se encontró $local."
        }
        for ($attempt = 1; ; $attempt++) {
            try {
                Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $OutFile
                return $true
            } catch {
                $status = 0
                if ($_.Exception.Response) { $status = [int]$_.Exception.Response.StatusCode }
                if ($status -eq 404) {
                    if ($Optional) { return $false }
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

    # Comprueba que el archivo descargado coincide con su suma en checksums.txt.
    function Assert-Checksum([string]$File, [string]$Asset, [string[]]$Sums) {
        $line = $Sums |
            Where-Object { ($_ -split '\s+')[-1].TrimStart('*') -eq $Asset } |
            Select-Object -First 1
        if (-not $line) { throw "checksums.txt no contiene la suma de $Asset." }
        $expected = ($line -split '\s+')[0]
        $actual = (Get-FileHash -LiteralPath $File -Algorithm SHA256).Hash
        if ($expected -ne $actual) {
            throw "La suma SHA-256 de $Asset no coincide (esperada $expected, obtenida $actual). La descarga está dañada o fue alterada; no se instaló nada."
        }
    }

    function Add-ContextMenu {
        foreach ($key in $menuKeys) {
            New-Item -Path "$key\command" -Force | Out-Null
            Set-Item -LiteralPath $key -Value 'Buscar carpetas aquí (fast-folder-cli)'
            Set-ItemProperty -LiteralPath $key -Name 'Icon' -Value "`"$exe`""
            Set-Item -LiteralPath "$key\command" -Value "`"$exe`" --path `"%V`""
        }
    }

    function Remove-ContextMenu {
        $removed = $false
        foreach ($key in $menuKeys) {
            if (Test-Path -LiteralPath $key) {
                Remove-Item -LiteralPath $key -Recurse -Force
                $removed = $true
            }
        }
        $removed
    }

    # ---------------------------------------------------------------- desinstalar
    if ($Uninstall) {
        Write-Host ''
        Write-Host 'Desinstalando fast-folder-cli...' -ForegroundColor Cyan

        if (-not (Test-Path -LiteralPath $exe)) { Write-Step "No se encontró $exe" }
        foreach ($file in @($exe, $alias) + @($scripts | ForEach-Object { Join-Path $InstallDir $_ })) {
            if (Test-Path -LiteralPath $file) {
                Remove-Item -LiteralPath $file -Force
                Write-Step "Eliminado $file"
            }
        }
        if (Remove-ContextMenu) {
            Write-Step 'Se quitó la opción "Buscar carpetas aquí" del menú contextual del Explorador.'
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
    if ($SourceDir) {
        $SourceDir = (Resolve-Path -LiteralPath $SourceDir).Path
        $baseUrl = $SourceDir
        $label = "desde $SourceDir"
    } elseif ($Version -eq 'latest') {
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
        $null = Save-Url "$baseUrl/$asset" $tmpExe
        $null = Save-Url "$baseUrl/checksums.txt" $tmpSums
        # Las versiones anteriores a la 1.2.0 no incluyen los scripts de fcd.
        $withScripts = $true
        foreach ($script in $scripts) {
            if (-not (Save-Url "$baseUrl/$script" (Join-Path $tmp $script) -Optional)) { $withScripts = $false }
        }

        Write-Step 'Verificando las sumas SHA-256...'
        $sums = Get-Content -LiteralPath $tmpSums
        Assert-Checksum $tmpExe $asset $sums
        if ($withScripts) {
            foreach ($script in $scripts) { Assert-Checksum (Join-Path $tmp $script) $script $sums }
        }

        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
        $copies = [ordered]@{ $exe = $tmpExe; $alias = $tmpExe }
        if ($withScripts) {
            foreach ($script in $scripts) { $copies[(Join-Path $InstallDir $script)] = Join-Path $tmp $script }
        }
        foreach ($file in $copies.Keys) {
            try {
                Copy-Item -LiteralPath $copies[$file] -Destination $file -Force
            } catch {
                throw "No se pudo escribir $file. Si fast-folder-cli se está ejecutando, ciérralo y vuelve a intentarlo."
            }
            Write-Step "Copiado en $file"
        }
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

    # El menú contextual abre el modo interactivo con "--path <carpeta>", que
    # las versiones anteriores a la 1.2.0 (sin scripts de fcd) no entienden.
    if ($withScripts -and -not $NoContextMenu) {
        Add-ContextMenu
        Write-Step 'Se agregó "Buscar carpetas aquí" al menú contextual del Explorador (en Windows 11, en "Mostrar más opciones").'
    } elseif (Remove-ContextMenu) {
        Write-Step 'Se quitó la opción "Buscar carpetas aquí" del menú contextual del Explorador.'
    }

    $installed = Get-InstalledVersion
    # Avisa si otro programa con el mismo nombre tiene prioridad en el PATH
    # (por ejemplo, el comando "fast" del paquete de npm fast-cli).
    foreach ($name in 'fast-folder-cli', 'fast') {
        $found = Get-Command $name -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($found -and -not (Test-SameDir (Split-Path -Parent $found.Source))) {
            Write-Warning "El comando '$name' ejecuta $($found.Source), que tiene prioridad en el PATH. Usa fast-folder-cli con su ruta completa o quita ese otro programa."
        }
    }

    Write-Host ''
    if ($previous -and $previous -ne $installed) {
        Write-Done "fast-folder-cli se actualizó de $previous a $installed."
    } elseif ($previous) {
        Write-Done "fast-folder-cli $installed ya estaba instalado y se reinstaló."
    } else {
        Write-Done "fast-folder-cli $installed se instaló correctamente."
    }
    if (-not $installed) {
        Write-Warning "fast-folder-cli se copió, pero Windows no permitió ejecutarlo. Si Smart App Control está activado, consulta https://github.com/$repo#si-windows-bloquea-el-programa"
    }
    Write-Host ''
    Write-Host '  Pruébalo:  ' -NoNewline
    Write-Host 'fast' -ForegroundColor Yellow -NoNewline
    Write-Host '           búsqueda con las flechas'
    if ($withScripts) {
        Write-Host '             ' -NoNewline
        Write-Host 'fcd tesis' -ForegroundColor Yellow -NoNewline
        Write-Host '      busca "tesis" y entra en la carpeta que elijas'
    }
    Write-Host '             ' -NoNewline
    Write-Host 'fast --help' -ForegroundColor Yellow -NoNewline
    Write-Host '    todas las opciones'
    Write-Host '  Si otra terminal no reconoce el comando, ciérrala y ábrela de nuevo.' -ForegroundColor DarkGray
    Write-Host ''
}
