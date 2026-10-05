# ⚡ fast-folder-cli

[![Release](https://img.shields.io/github/v/release/AnthonyCZ6/fast-folder-cli?sort=semver)](https://github.com/AnthonyCZ6/fast-folder-cli/releases/latest)
[![Workflow de release](https://github.com/AnthonyCZ6/fast-folder-cli/actions/workflows/release.yml/badge.svg)](https://github.com/AnthonyCZ6/fast-folder-cli/actions/workflows/release.yml)
![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white)
![Windows](https://img.shields.io/badge/Windows-10%20%7C%2011-0078D6?logo=windows&logoColor=white)
![Licencia](https://img.shields.io/badge/licencia-MIT-yellow)

**Encuentra cualquier carpeta de tu PC en segundos, directamente desde la terminal.**

`fast-folder-cli` es una herramienta de línea de comandos escrita en Go que recorre el disco
de forma concurrente para localizar carpetas por nombre, pensada para reemplazar la lenta
búsqueda del Explorador de archivos de Windows.

```text
> fast-folder-cli -n proyecto

Buscando "proyecto" en C:\Users\usuario (ocultas/sistema: excluidas)

  [1] C:\Users\usuario\Documents\Proyecto-Final
  [2] C:\Users\usuario\Desktop\escuela\mi-proyecto
  [3] C:\Users\usuario\source\repos\proyecto-api

────────────────────────────────────────────────────────────────
Resultados : 3 carpetas encontradas
Analizadas : 27,972 carpetas
Tiempo     : 854 ms
```

## ✨ Lo que lo hace diferente

| | |
| --- | --- |
| 🔤 **Sin acentos** | `cancion` encuentra `Canción`, `ano` encuentra `Año 2024`: escribe rápido, sin preocuparte por las tildes. |
| 📂 **Entra en la carpeta** | `fcd tesis` busca, eliges con las flechas y la terminal **queda dentro** de la carpeta. |
| 🧑‍💻 **Tus proyectos** | `fast --projects` lista todos tus proyectos (Git, Node.js, Python, Go, .NET, Java, Unity...) aunque estén regados por Descargas, el Escritorio o Documentos. |
| 📦 **¿Dónde se instaló?** | `fast --apps chrome -o` encuentra una app instalada y abre su carpeta con el ejecutable seleccionado. |
| 🕒 **¿Dónde guardé eso?** | `fast --recientes` muestra las carpetas de lo que abriste hace poco, en cualquier programa; `fast --con word`, solo las de lo que abriste con Word. Sin recordar el nombre. |
| 📅 **"¿En qué trabajé ayer?"** | `fast -m ayer` muestra las carpetas modificadas ayer; también `hoy`, `semana`, `mes`... |
| 💾 **¿Qué ocupa tanto?** | `fast node_modules --size` dice cuánto pesa cada carpeta, de mayor a menor. |
| 🖱️ **Desde el Explorador** | Clic derecho en una carpeta → **Buscar carpetas aquí**. |
| ⚡ **Acciones rápidas** | En el modo interactivo: copia la ruta, ábrela en VS Code o en una terminal con una tecla. |

---

## 📌 El problema

Buscar una carpeta con el cuadro de búsqueda del Explorador de Windows suele ser una
experiencia frustrante:

- **Depende del índice de Windows Search.** Fuera de las ubicaciones indexadas (otras
  unidades, `AppData`, `ProgramData`, carpetas de proyectos...) el Explorador recorre el disco
  de forma secuencial y los resultados aparecen a cuentagotas.
- **Busca de más.** Examina archivos, contenido y propiedades aunque solo quieras una carpeta.
- **No es automatizable.** No puedes usarlo desde un script ni encadenarlo con otras
  herramientas.
- **Las alternativas en consola son lentas.** `dir /s /b /ad` y `Get-ChildItem -Recurse`
  recorren el árbol en un único hilo.

## ✅ La solución

| Característica | Detalle |
| --- | --- |
| 🚀 **Recorrido concurrente** | Varias goroutines leen directorios en paralelo con un límite de concurrencia (4 × núcleos). |
| 🔤 **Búsqueda flexible** | Coincidencia parcial sin distinguir mayúsculas ni acentos, o patrones con comodines `*`, `?` y `[ ]`. |
| 🔎 **Filtros** | Por tipo (`--projects`), por fecha de modificación (`-m hoy`) y con el tamaño de cada carpeta (`--size`). |
| 🌐 **Variables de entorno** | Entiende `%APPDATA%`, `%LOCALAPPDATA%`, `%USERPROFILE%`, `%PROGRAMDATA%`, `~` y `D:` incluso desde PowerShell. |
| 🛡️ **Tolerante a errores** | Las carpetas sin permisos se omiten y se contabilizan; la búsqueda nunca se detiene. |
| 🔁 **Seguro con enlaces** | Las uniones (*junctions*) y enlaces simbólicos se reportan pero no se recorren, evitando ciclos infinitos. |
| 👻 **Ocultas y de sistema** | Se ignoran por defecto (más rápido y menos ruido); `--all` las incluye. |
| 📂 **Integración con el Explorador** | `--open` abre la primera coincidencia en cuanto se encuentra, y el menú contextual del Explorador abre la búsqueda en cualquier carpeta. |
| ⌨️ **Modo interactivo** | Escribe `fast` sin argumentos y busca, elige ubicación y abre carpetas usando solo las flechas. |
| 🚪 **`fcd`** | Busca una carpeta y deja la terminal (PowerShell o Símbolo del sistema) dentro de ella. |
| 📦 **Un solo ejecutable** | Sin instalar nada más: un único `.exe` para x64 o ARM64. |

## 📊 Rendimiento

Búsqueda de una carpeta en todo el perfil de usuario (`%USERPROFILE%`, incluyendo carpetas
ocultas: ~149,000 carpetas), en Windows 11 con un procesador de 8 núcleos y SSD:

| Herramienta | Comando | Tiempo |
| --- | --- | ---: |
| PowerShell | `Get-ChildItem $env:USERPROFILE -Recurse -Directory -Force -Filter *fast-folder*` | 44.2 s |
| **fast-folder-cli** | `fast-folder-cli -n fast-folder -a` | **5.1 s** |

Ambas encontraron las mismas 4 carpetas, pero fast-folder-cli fue **~8.7 veces más rápido**. Sin carpetas ocultas
(el modo por defecto) el mismo perfil se recorre en menos de 1 segundo. Los tiempos dependen
del disco y de la caché del sistema de archivos: la segunda búsqueda suele ser más rápida.

---

## 🛠️ Instalación

Requiere Windows 10 u 11 de 64 bits (x64 o ARM64).

> ⚠️ Si tu Windows 11 tiene **Smart App Control** activado, no podrás ejecutar
> fast-folder-cli. Consulta [Si Windows bloquea el programa](#si-windows-bloquea-el-programa)
> antes de instalar.

### 🧙 Asistente de instalación (recomendado)

1. Descarga **[fast-folder-cli-setup.exe](https://github.com/AnthonyCZ6/fast-folder-cli/releases/latest/download/fast-folder-cli-setup.exe)**.
2. Ábrelo y sigue los pasos: licencia → carpeta → opciones → **Instalar**.
3. Abre una terminal **nueva** y escribe `fast --help`. En la última pantalla puedes marcar
   **Abrir una terminal para probar fast-folder-cli** para hacerlo directamente.

El asistente:

- Instala la versión adecuada para tu equipo (x64 o ARM64) en
  `%LOCALAPPDATA%\Programs\fast-folder-cli`, **sin permisos de administrador**.
- Agrega esa carpeta al PATH de tu usuario (opción marcada por defecto) sin alterar el resto
  de entradas.
- Crea el atajo **`fast`** (opción marcada por defecto), para escribir `fast -n proyecto` en
  lugar de `fast-folder-cli -n proyecto`.
- Instala el comando [**`fcd`**](#-entrar-en-una-carpeta-con-fcd) para buscar una carpeta y
  entrar en ella.
- Agrega **Buscar carpetas aquí** al [menú contextual del Explorador](#️-menú-contextual-del-explorador)
  (opción marcada por defecto).
- Se registra en **Configuración → Aplicaciones → Aplicaciones instaladas**, desde donde se
  desinstala. Para actualizar, basta con ejecutar el asistente de una versión nueva.

> ⚠️ El asistente no está firmado digitalmente, así que la primera vez Windows SmartScreen
> puede mostrar **"Windows protegió su PC"**: pulsa **Más información → Ejecutar de todas
> formas**. Si en cambio Windows bloquea el archivo sin darte esa opción, es Smart App
> Control: consulta [Si Windows bloquea el programa](#si-windows-bloquea-el-programa).

### ⚡ Desde la terminal, con un solo comando

Abre **PowerShell** y ejecuta:

```powershell
irm https://raw.githubusercontent.com/AnthonyCZ6/fast-folder-cli/main/install.ps1 | iex
```

Eso es todo: ya puedes usar `fast-folder-cli` en esa misma ventana. El
[script](install.ps1):

- Detecta si tu equipo es x64 o ARM64 y descarga el ejecutable de la última versión.
- Comprueba las sumas SHA-256 antes de instalar.
- Lo copia en `%LOCALAPPDATA%\Programs\fast-folder-cli`, junto con el atajo **`fast`** y el
  comando **`fcd`**, y agrega esa carpeta al PATH de tu usuario, **sin permisos de
  administrador** y sin alterar el resto del PATH.
- Agrega **Buscar carpetas aquí** al menú contextual del Explorador.

| Para... | Ejecuta |
| --- | --- |
| **Actualizar** a la última versión | El mismo comando de instalación. |
| Instalar una **versión concreta** | `& ([scriptblock]::Create((irm https://raw.githubusercontent.com/AnthonyCZ6/fast-folder-cli/main/install.ps1))) -Version v1.0.0` |
| Instalar **sin el menú contextual** | `& ([scriptblock]::Create((irm https://raw.githubusercontent.com/AnthonyCZ6/fast-folder-cli/main/install.ps1))) -NoContextMenu` |
| Instalar **sin conexión** (con los archivos del release ya descargados en una carpeta) | `.\install.ps1 -SourceDir C:\Descargas\fast-folder-cli` |
| **Desinstalar** | `& ([scriptblock]::Create((irm https://raw.githubusercontent.com/AnthonyCZ6/fast-folder-cli/main/install.ps1))) -Uninstall` |

### Con Go

Si tienes [Go 1.26 o superior](https://go.dev/dl/) (`winget install GoLang.Go`):

```powershell
go install github.com/AnthonyCZ6/fast-folder-cli@latest
```

El ejecutable se instala en `%USERPROFILE%\go\bin`, carpeta que el instalador de Go ya
agrega al PATH.

### Descarga manual

1. Descarga el `.exe` de tu equipo desde la
   [última versión](https://github.com/AnthonyCZ6/fast-folder-cli/releases/latest):

   | Equipo | Archivo |
   | --- | --- |
   | Windows x64 (procesadores Intel o AMD, la mayoría de los equipos) | [fast-folder-cli-windows-amd64.exe](https://github.com/AnthonyCZ6/fast-folder-cli/releases/latest/download/fast-folder-cli-windows-amd64.exe) |
   | Windows ARM64 (procesadores Snapdragon, Surface Pro X) | [fast-folder-cli-windows-arm64.exe](https://github.com/AnthonyCZ6/fast-folder-cli/releases/latest/download/fast-folder-cli-windows-arm64.exe) |

2. Renómbralo a `fast-folder-cli.exe` y guárdalo en una carpeta permanente, por ejemplo
   `%LOCALAPPDATA%\Programs\fast-folder-cli`.
3. Agrega esa carpeta al PATH (siguiente apartado).

> ⚠️ Los ejecutables no están firmados digitalmente, así que Windows SmartScreen o tu
> antivirus pueden mostrar una advertencia la primera vez, y Smart App Control los bloquea
> (consulta [Si Windows bloquea el programa](#si-windows-bloquea-el-programa)). Para
> comprobar que el archivo es el original, compara `Get-FileHash .\fast-folder-cli.exe` con
> el valor de `checksums.txt` publicado en la versión. El instalador de un solo comando hace
> esta comprobación por ti.

#### Agregar al PATH de Windows

1. Pulsa <kbd>Win</kbd> + <kbd>R</kbd>, escribe `SystemPropertiesAdvanced` y pulsa
   <kbd>Enter</kbd>.
2. Haz clic en **Variables de entorno...**.
3. En **Variables de usuario**, selecciona `Path` → **Editar** → **Nuevo** y pega la ruta
   de la carpeta.
4. Acepta todas las ventanas y abre una terminal nueva (el PATH solo se lee al iniciarla).

Comprueba que funciona con:

```powershell
fast-folder-cli --version
```

### Compilar desde el código

Requiere [Go 1.26 o superior](https://go.dev/dl/):

```powershell
git clone https://github.com/AnthonyCZ6/fast-folder-cli.git
cd fast-folder-cli
go build -trimpath -ldflags "-s -w -X main.version=1.0.0" -o fast-folder-cli.exe .
```

| Opción | Efecto |
| --- | --- |
| `-trimpath` | Elimina las rutas locales de tu equipo del binario. |
| `-ldflags "-s -w"` | Quita la información de depuración (ejecutable más pequeño). |
| `-X main.version=1.0.0` | Define la versión que muestra `--version`. |

**Compilación cruzada** desde Linux o macOS (también compila allí, con funciones limitadas):

```bash
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o fast-folder-cli.exe .
GOOS=windows GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o fast-folder-cli-arm64.exe .
```

> 💡 **Atajo `fast` y comando `fcd`:** el asistente y el script de instalación los crean
> automáticamente. Si instalas con Go o a mano, copia `fast-folder-cli.exe` como `fast.exe`
> y [`shell/fcd.ps1`](shell/fcd.ps1) y [`shell/fcd.cmd`](shell/fcd.cmd) en la misma carpeta.

### Si Windows bloquea el programa

Los ejecutables de fast-folder-cli todavía no están firmados digitalmente. Con **Smart App
Control** activado, Windows 11 no deja ejecutar programas sin firma ni reputación, así que la
instalación termina sin errores, pero al escribir `fast` aparece un error como este:

```text
Program 'fast-folder-cli.exe' failed to run: An Application Control policy has blocked this file
```

En Windows en español, el mensaje indica que una directiva de Control de aplicaciones bloqueó
el archivo. Ocurre con cualquier método de instalación, incluso si lo compilas tú mismo.

Para saber si Smart App Control está activado, abre **Seguridad de Windows → Control de
aplicaciones y navegador → Configuración de Smart App Control**, o ejecuta en PowerShell:

```powershell
(Get-MpComputerStatus).SmartAppControlState   # On = bloquea; Eval u Off = no bloquea
```

A diferencia de SmartScreen, Smart App Control no ofrece **Ejecutar de todas formas** ni
permite excepciones para un programa concreto. Mientras no haya versiones firmadas, la única
forma de usar fast-folder-cli es desactivarlo. Antes de hacerlo, ten en cuenta que protege a
todo el sistema. Desde la actualización de abril de 2026 (KB5083769) se puede volver a activar
sin reinstalar Windows; en versiones anteriores de Windows 11, una vez desactivado, solo se
podía recuperar reinstalando.

---

## 🚀 Uso

Si instalaste con el asistente o el script, puedes usar el atajo **`fast`** en lugar del
nombre completo `fast-folder-cli`. Tras instalar, abre una terminal **nueva**: las que ya
estaban abiertas no ven el PATH actualizado.

### ⌨️ Modo interactivo (con las flechas)

*Disponible desde la versión 1.1.0.* Escribe solo:

```powershell
fast
```

y se abre una pantalla de búsqueda que se maneja sin escribir comandos:

```text
 fast-folder-cli   v1.5.0

 ► Buscar     cancion
   Ubicación    Documentos           C:\Users\usuario\Documents
   Tipo         Carpetas             cualquier carpeta cuyo nombre coincida
   Modificada   Cualquiera           fecha de modificación de la carpeta
   Ocultas      No                   carpetas ocultas y de sistema

 ↑↓ moverse   ←→ cambiar opción   Enter buscar   Esc salir
```

1. Escribe el nombre (o parte del nombre) de la carpeta. No hace falta poner acentos.
2. Con **↓** baja a **Ubicación** y elige dónde buscar con **← →**: tu perfil, Escritorio,
   Documentos, Descargas, la carpeta actual, AppData, ProgramData o cualquier unidad (C:, D:...).
3. En **Tipo** elige **Proyectos** para buscar solo [carpetas de proyectos](#buscar-proyectos),
   **Apps** para buscar [aplicaciones instaladas](#buscar-apps-instaladas) o **Recientes**
   para ver las [carpetas de lo que abriste hace poco](#carpetas-recientes); así el nombre
   es opcional y, sin él, aparecen todos. Con **Apps**, Enter abre la ubicación de la app y
   los campos Ubicación, Modificada y Ocultas no se aplican. Con **Recientes**, Ubicación no
   se aplica y Modificada pasa a ser **Usada**: cuándo abriste algo en la carpeta.
4. En **Modificada** elige **Hoy**, **Ayer**, **Últimos 7 días** o **Últimos 30 días** para
   ver solo las carpetas modificadas en ese periodo.
5. En **Ocultas** actívalas si también quieres buscar en carpetas ocultas y de sistema.
6. Pulsa **Enter**. Los resultados aparecen conforme se encuentran:

```text
 fast-folder-cli   "cancion" en C:\Users\usuario\Documents

 ► Canción final           C:\Users\usuario\Documents\música
   Canciones-2024          C:\Users\usuario\Documents\escuela

 2 carpetas encontradas · 1,284 analizadas · 1/2 · 38 ms

 ↑↓ moverse   Enter abrir en el Explorador   ← nueva búsqueda   Esc salir
 c copiar ruta   v VS Code   t terminal   d tamaño y fecha
```

| Tecla | En la búsqueda | En los resultados |
| --- | --- | --- |
| <kbd>↑</kbd> <kbd>↓</kbd> | Cambiar de campo | Moverse por la lista (<kbd>RePág</kbd> <kbd>AvPág</kbd> <kbd>Inicio</kbd> <kbd>Fin</kbd> para saltar) |
| <kbd>←</kbd> <kbd>→</kbd> | Cambiar la opción del campo | <kbd>→</kbd> abre la carpeta · <kbd>←</kbd> vuelve a la búsqueda |
| <kbd>Enter</kbd> | Buscar | Abrir la carpeta en el Explorador (con `fcd`, entrar en ella) |
| <kbd>C</kbd> | | Copiar la ruta al portapapeles |
| <kbd>V</kbd> | | Abrir la carpeta en Visual Studio Code |
| <kbd>T</kbd> | | Abrir una terminal nueva en la carpeta |
| <kbd>D</kbd> | | Mostrar cuánto ocupa y cuándo se modificó |
| <kbd>E</kbd> | | Abrir la carpeta en el Explorador |
| <kbd>Esc</kbd> | Salir | Salir |

### 🚪 Entrar en una carpeta con `fcd`

*Disponible desde la versión 1.2.0.* Un programa no puede cambiar la carpeta de la terminal
que lo abrió, así que el asistente y el script de instalación agregan el comando **`fcd`**
(para PowerShell y para el Símbolo del sistema), que sí puede:

```powershell
fcd tesis
```

Abre el modo interactivo con la búsqueda de `tesis`; al elegir una carpeta con
<kbd>Enter</kbd>, la terminal queda dentro de ella. `fcd` acepta las mismas opciones que
`fast-folder-cli`: `fcd -p D: proyecto`, `fcd --projects`, `fcd -m ayer`... Sin argumentos
abre el formulario vacío. Si no eliges ninguna carpeta, `fcd` termina con el mismo código de
salida que `fast-folder-cli` (por ejemplo, 2 si una opción no es válida).

> Si PowerShell dice que *la ejecución de scripts está deshabilitada en este sistema*, `fcd`
> no puede funcionar con la directiva de ejecución actual. Puedes permitir los scripts
> locales solo para tu usuario con `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned`, o
> usar `fcd` desde el Símbolo del sistema.

### 🖱️ Menú contextual del Explorador

*Disponible desde la versión 1.2.0.* Haz clic derecho en el fondo de una carpeta (o sobre
una carpeta) y elige **Buscar carpetas aquí (fast-folder-cli)**: se abre el modo
interactivo con esa carpeta como ubicación. En Windows 11 la opción está en **Mostrar más
opciones** (o pulsando <kbd>Mayús</kbd> + <kbd>F10</kbd>).

El asistente la agrega si dejas marcada su casilla y el script de instalación la agrega
salvo que uses `-NoContextMenu`. Al desinstalar se quita. Por dentro ejecuta
`fast-folder-cli --path "<carpeta>"`, que sin término abre el modo interactivo.

### Con comandos (para scripts)

```text
fast-folder-cli -n <término> [-p <ruta>] [opciones]
fast-folder-cli <término> [opciones]
fast-folder-cli --projects [término] [opciones]
```

Con un término (o con `--projects` o `-m`), la herramienta funciona como un comando normal:
imprime los resultados y termina, así que se puede usar en scripts y redirigir su salida.
Si solo indicas opciones, como `fast -p D:`, abre el modo interactivo con ellas.

> Si tienes otro programa llamado `fast` (por ejemplo, el medidor de velocidad de npm
> `fast-cli`), el que aparezca primero en el PATH tendrá prioridad; el nombre completo
> `fast-folder-cli` siempre funciona.

### Banderas

| Corta | Larga | Descripción | Por defecto |
| --- | --- | --- | --- |
| `-n` | `--name` | Término o patrón a buscar en los nombres de carpeta. También puede indicarse como argumento posicional. | *(obligatorio, salvo con `--projects`, `--apps`, `--recientes` o `-m`)* |
| `-p` | `--path` | Carpeta raíz desde la que comienza la búsqueda. Admite variables de entorno. | `%USERPROFILE%` |
| `-m` | `--modified` | Solo carpetas [modificadas](#filtrar-por-fecha-de-modificación) en ese periodo: `hoy`, `ayer`, `semana`, `mes`, `año`, un número de días (`3d`) o una fecha (`2026-09-01`). | cualquier fecha |
| | `--projects` | Busca [carpetas de proyectos](#buscar-proyectos) en lugar de cualquier carpeta. | desactivado |
| | `--apps` | Busca [aplicaciones instaladas](#buscar-apps-instaladas) por su nombre en lugar de carpetas. Con `-o` abre su ubicación. | desactivado |
| | `--recientes` | Busca entre las [carpetas de lo que abriste hace poco](#carpetas-recientes), de la más reciente a la más antigua. Con `-m`, por cuándo se usaron. | desactivado |
| | `--con` | Solo las carpetas recientes de lo que abriste con ese programa (`--con word`, `--con code`). Implica `--recientes`. | cualquier programa |
| `-s` | `--size` | Calcula [cuánto ocupa](#cuánto-ocupa-cada-carpeta) cada carpeta encontrada y las ordena de mayor a menor. | desactivado |
| `-a` | `--all` | Incluye carpetas ocultas y de sistema. | desactivado |
| `-o` | `--open` | Abre la primera carpeta encontrada en el Explorador de Windows. | desactivado |
| | `--exclude` | Carpetas que no se muestran ni se recorren, separadas por comas (`--exclude node_modules,venv`). Se suman a las de la [configuración](#personalización). | ninguna |
| | `--json` | Una línea JSON por resultado, para scripts, sin cabecera ni resumen. | desactivado |
| | `--config` | Crea, si no existe, el [archivo de configuración](#personalización) y lo abre. | |
| `-v` | `--version` | Muestra la versión instalada. | |
| `-h` | `--help` | Muestra la ayuda. | |

Las banderas pueden escribirse en cualquier orden, antes o después del término, y con
`=` o espacio: `--name=proyecto` equivale a `--name proyecto`.

### Ejemplos

```powershell
# Buscar en tu perfil de usuario
fast-folder-cli -n proyecto

# Término como argumento posicional (con espacios, entre comillas)
fast-folder-cli "mis documentos escolares"

# Buscar en AppData incluyendo carpetas ocultas
fast-folder-cli -n discord -p %APPDATA% -a

# Buscar en otra unidad y abrir el primer resultado en el Explorador
fast-folder-cli -n "tesis*" -p D: -o

# Buscar datos de aplicaciones en ProgramData
fast-folder-cli --name nvidia --path %PROGRAMDATA% --all

# Guardar los resultados en un archivo
fast-folder-cli -n node_modules -p C:\dev > resultados.txt

# Sin acentos: encuentra "Canción", "canciones"...
fast-folder-cli cancion

# Todos tus proyectos, y los de Python modificados esta semana
fast-folder-cli --projects
fast-folder-cli --projects -m semana

# Las carpetas de Documentos que tocaste ayer
fast-folder-cli -m ayer -p %USERPROFILE%\Documents

# Cuánto ocupa cada node_modules de C:\dev
fast-folder-cli node_modules --size -p C:\dev

# Dónde se instaló Chrome: abre su carpeta con chrome.exe seleccionado
fast-folder-cli --apps chrome -o

# Las carpetas de lo que abriste esta semana con Word
fast-folder-cli --con word -m semana
```

### Buscar proyectos

`--projects` (o **Tipo: Proyectos** en el modo interactivo) busca carpetas que contienen
alguno de estos archivos o carpetas, y muestra el tipo junto a cada una:

| Tipo | Se reconoce por |
| --- | --- |
| Go | `go.mod` |
| Rust | `Cargo.toml` |
| Node.js | `package.json` |
| Python | `pyproject.toml`, `requirements.txt`, `setup.py`, `Pipfile` |
| .NET | `*.sln`, `*.csproj` |
| C/C++ | `CMakeLists.txt`, `*.vcxproj` |
| Java / Kotlin | `pom.xml`, `build.gradle` / `build.gradle.kts` |
| PHP, Ruby, Flutter | `composer.json`, `Gemfile`, `pubspec.yaml` |
| Unity | `ProjectSettings` |
| Git | `.git` (solo si no hay otro indicio) |

```text
> fast --projects -p C:\Users\usuario\Documents

  [1] C:\Users\usuario\Documents\escuela\web  (Node.js)
  [2] C:\Users\usuario\Documents\escuela\api  (Go, Python)
  [3] C:\Users\usuario\Documents\juegos\plataformas  (Unity)
```

Con un término, solo aparecen los proyectos cuyo nombre coincide (`--projects api`). El
interior de un proyecto no se recorre (`node_modules`, `bin`, `.git`...), lo que hace la
búsqueda rápida; por eso un proyecto dentro de otro no aparece. La carpeta raíz de la
búsqueda nunca cuenta como proyecto, para que un `package.json` suelto en tu carpeta
personal no oculte todo lo demás.

### Buscar apps instaladas

`--apps` (o **Tipo: Apps** en el modo interactivo) busca entre los programas instalados
(los mismos que muestra **Configuración → Aplicaciones**) en lugar de recorrer carpetas, así
que responde al instante. Con `-o` (o Enter en el modo interactivo) abre su ubicación en el
Explorador con el ejecutable seleccionado:

```text
> fast --apps chrome -o

  [1] Google Chrome  C:\Program Files\Google\Chrome\Application  (120.0.6099.130)
```

- La ubicación es la carpeta del ejecutable principal o, si el programa no lo indica, la
  carpeta donde se instaló.
- También aparecen los accesos directos del menú Inicio (apps portables como Figma, o Git
  Bash) y los programas que solo registran su ejecutable, como Excel o Word de Microsoft 365.
- No se muestran los componentes del sistema ni las actualizaciones, ni las apps de la
  Microsoft Store, cuya carpeta (`WindowsApps`) está protegida.
- Sin término, `fast --apps` las lista todas. `-p`, `-m`, `-s`, `-a` y `--projects` no se
  aplican a las apps.
- `fcd --apps steam` te deja en la carpeta de la app elegida, por ejemplo para ejecutar
  sus herramientas desde la terminal.

### Carpetas recientes

¿Dónde guardaste el documento de ayer? `--recientes` (o **Tipo: Recientes** en el modo
interactivo) muestra las carpetas de lo que abriste hace poco, en cualquier programa, de la
más reciente a la más antigua, con cuándo, cuántos archivos y con qué programas:

```text
> fast --recientes

Buscando carpetas recientes en el historial de Windows (ocultas/sistema: excluidas)

  [1] C:\Users\usuario\Documents\escuela\tesis  (hace 10 min · 2 archivos · Word)
  [2] C:\Users\usuario\Downloads  (hace 2 h · 1 archivo · Microsoft Edge, Fotos)
  [3] C:\Users\usuario\Documents\proyectos\api  (ayer a las 18:20 · Visual Studio Code)
```

- `--con word` deja solo las de lo que abriste con Word; también `excel`, `code`, `fotos`,
  `explorer`... Si no reconoce el programa, dice cuáles tienen historial.
- Se combina con un término (`fast --recientes tesis`), con `-m` (por cuándo las usaste:
  `fast --con word -m semana`), con `-p` (solo dentro de esa carpeta), con `--json` (añade
  `last_used`, `files` y `apps`) y con `-o` (abre la más reciente).
- `fcd --recientes` y `fcd --con code` te dejan en la carpeta elegida.
- Solo aparecen carpetas que siguen existiendo, en este equipo: las URL y las rutas de red
  (`\\servidor\...` y las unidades de red, como `Z:`) se ignoran, porque comprobarlas podría
  dejar la búsqueda esperando a un servidor. Las carpetas ocultas (como AppData) solo salen
  con `-a`.

**De dónde sale.** Windows anota lo que abres en sus *jump lists*: las listas de
**Recientes** del Explorador y las que aparecen al hacer clic derecho en un programa de la
barra de tareas. fast-folder-cli **solo las lee**, en tu equipo
(`%APPDATA%\Microsoft\Windows\Recent\AutomaticDestinations`): no las modifica ni envía nada
a ningún sitio. El programa de cada lista se reconoce por su identificador, con una tabla de
los más comunes, los accesos directos del menú Inicio y los programas instalados; los que no
se reconocen aparecen sin nombre.

**Si no aparece nada nuevo**, puede que el historial esté desactivado. Actívalo en
**Configuración → Personalización → Inicio → "Mostrar elementos abiertos recientemente en
Inicio, las listas de accesos directos y el Explorador de archivos"**; desde entonces Windows
vuelve a anotar lo que abres. fast-folder-cli avisa cuando está desactivado (también si lo
desactiva una directiva de la empresa, que no se puede cambiar).

### Filtrar por fecha de modificación

`-m` (o el campo **Modificada** del modo interactivo) deja solo las carpetas modificadas en
un periodo. Se puede usar con o sin término:

| Valor | Carpetas modificadas... |
| --- | --- |
| `hoy` | hoy |
| `ayer` | ayer (no hoy) |
| `semana` | en los últimos 7 días, incluido hoy |
| `mes` | en los últimos 30 días |
| `año` (o `ano`) | en los últimos 365 días |
| `3d`, `15d`... | en ese número de días |
| `2026-09-01` | desde esa fecha |

Windows cambia la fecha de una carpeta cuando se crea, borra o renombra algo **directamente
dentro de ella** (también al guardar archivos con muchos editores, que reemplazan el archivo
al guardar), pero no al editar un archivo de una subcarpeta.

### Cuánto ocupa cada carpeta

`--size` suma el tamaño de los archivos de cada carpeta encontrada (incluidas sus
subcarpetas ocultas, sin seguir enlaces) y las muestra ordenadas de mayor a menor, con el
total al final:

```text
> fast node_modules --size -p C:\dev

     1.2 GB  C:\dev\tienda\node_modules
     350 MB  C:\dev\blog\node_modules
      48 MB  C:\dev\api\node_modules

────────────────────────────────────────────────────────────────
Resultados : 3 carpetas encontradas
Tamaño     : 1.6 GB en total (98,412 archivos)
```

Con `--size` no se busca dentro de las carpetas encontradas, para no contar dos veces los
`node_modules` anidados. Si pulsas <kbd>Ctrl</kbd>+<kbd>C</kbd> antes de que termine, se
muestran las carpetas encontradas hasta ese momento, sin tamaño (estaría incompleto). En el
modo interactivo, la tecla <kbd>D</kbd> muestra el tamaño de la carpeta seleccionada.

### Reglas de coincidencia

Las búsquedas **no distinguen mayúsculas de minúsculas**, igual que Windows, **ni acentos**:
`cancion` encuentra `Canción`, `pinguino` encuentra `Pingüino` y `ano` encuentra `Año 2024`
(la ñ se trata como n).

| Término | Tipo | Coincide con | No coincide con |
| --- | --- | --- | --- |
| `proy` | Subcadena | `Proyectos`, `mi-proyecto` | `Documentos` |
| `proy*` | Comodín | `Proyectos`, `proyecto-2024` | `mi-proyecto` |
| `*backup` | Comodín | `db-backup`, `Backup` | `backups` |
| `tesis-202?` | Comodín | `Tesis-2024`, `tesis-2025` | `tesis-20245` |
| `[ab]ackup` | Comodín | `Backup`, `aackup` | `Cackup` |

Sin comodines, el término puede aparecer en cualquier parte del nombre. Con comodines
(`*`, `?`, `[ ]`) el patrón debe cubrir **el nombre completo** de la carpeta.

### Variables de entorno y rutas especiales

| Valor de `--path` | Se resuelve como |
| --- | --- |
| `%USERPROFILE%` | `C:\Users\<usuario>` |
| `%APPDATA%` | `C:\Users\<usuario>\AppData\Roaming` |
| `%LOCALAPPDATA%` | `C:\Users\<usuario>\AppData\Local` |
| `%PROGRAMDATA%` | `C:\ProgramData` |
| `%APPDATA%\..` | `C:\Users\<usuario>\AppData` |
| `~` o `~\Documents` | Tu carpeta personal |
| `D:` | La raíz de la unidad `D:\` |

Funciona con cualquier variable definida en el sistema. PowerShell no expande la sintaxis
`%VAR%`, así que `fast-folder-cli` la resuelve por su cuenta; en PowerShell también puedes
usar la sintaxis nativa (`-p $env:APPDATA`).

### Carpetas ocultas y de sistema

Por defecto se omiten (ni se muestran ni se recorren):

- Carpetas con el atributo **Oculto** o **Sistema** de Windows (`AppData`, `$Recycle.Bin`,
  `System Volume Information`, ...).
- Carpetas cuyo nombre empieza por punto (`.git`, `.vscode`, `.cache`, ...).

Usa `-a` / `--all` para incluirlas. La carpeta raíz indicada con `--path` siempre se
recorre, aunque esté oculta: `-p %APPDATA%` funciona sin `--all`.

### Códigos de salida

Útiles para scripts, siguiendo la convención de `grep`:

| Código | Significado |
| ---: | --- |
| `0` | Se encontró al menos una carpeta. También sin argumentos fuera de una terminal: muestra la ayuda por la salida estándar. |
| `1` | La búsqueda terminó sin coincidencias. |
| `2` | Error de uso: bandera desconocida, ruta inexistente o patrón inválido (o, con `--apps`, no se pudo leer la lista de programas instalados; con `--recientes`, el historial de Windows; con `--con`, no se reconoce el programa). |
| `130` | Búsqueda interrumpida con <kbd>Ctrl</kbd> + <kbd>C</kbd> (se muestran los resultados parciales). |

---

## Personalización

`fast --config` crea (si no existe) y abre `%APPDATA%\fast-folder-cli\config.toml`, una
plantilla comentada: quita el `#` de las líneas que quieras activar. Sin archivo, todo funciona
como siempre; si el archivo tiene un error, se avisa y se ignora. Las banderas mandan sobre el
archivo.

```toml
ubicacion = '%USERPROFILE%\Documents'        # dónde buscar si no se indica -p
excluir = ["node_modules", "venv", ".venv"]  # carpetas que nunca se recorren
ocultas = true                               # como -a
editor = "cursor"                            # tecla v: code, cursor, codium, notepad++...
terminal = "wt"                              # tecla t: wt, pwsh, powershell o cmd
recientes = 5                                # carpetas elegidas hace poco (0: ninguna)

[[ubicaciones]]                              # ubicaciones propias, las primeras del formulario
nombre = "Proyectos"
ruta = 'D:\dev'
```

- Las carpetas que eliges con Enter en el modo interactivo aparecen después como
  **Reciente: …** en el campo Ubicación.
- La variable de entorno `FFC_CONFIG` permite usar otro archivo.

## ⚙️ Cómo funciona

```text
             ┌────────────────── semáforo (4 × núcleos) ──────────────────┐
  raíz ──►  walk(dir) ──► ¿hay un slot libre? ──sí──► nueva goroutine: walk(subdir)
                                   │
                                   └──no──► walk(subdir) en la goroutine actual
```

1. **Concurrencia acotada sin interbloqueos.** Cada subcarpeta se entrega a una goroutine
   nueva si hay capacidad libre en el semáforo; si no, la goroutine actual la procesa en
   profundidad. Nunca hay más goroutines activas que el límite y ninguna se queda bloqueada
   esperando a otra.
2. **Mínimas llamadas al sistema.** Los atributos (oculto, sistema, *reparse point*) se leen de
   los datos que Windows ya devuelve al listar el directorio, sin consultas adicionales por
   carpeta. Las entradas no se ordenan, ahorrando trabajo en carpetas grandes.
3. **Errores sin interrupciones.** Un "Acceso denegado" solo incrementa el contador
   `sin acceso`; el resto del árbol se sigue recorriendo.
4. **Resultados en tiempo real.** Cada coincidencia se imprime en cuanto se encuentra; la salida
   se agrupa en un búfer que se vacía cuando no hay más resultados pendientes, para no
   ralentizar la consola cuando hay miles de coincidencias.
5. **Enlaces seguros.** Las uniones heredadas de Windows (por ejemplo
   `AppData\Local\Application Data`, que apunta a sí misma) provocarían un bucle infinito.
   Los enlaces se muestran si coinciden, pero nunca se recorren. Las carpetas de OneDrive sí
   se recorren con normalidad.

## 📁 Estructura del proyecto

```text
fast-folder-cli/
├── .github/
│   ├── workflows/                 # Pruebas (ci.yml), publicación (release.yml) y benchmarks (bench.yml)
│   ├── scripts/release-notes.sh   # Notas de versión a partir de los commits
│   ├── ISSUE_TEMPLATE/            # Formularios para reportar errores y proponer mejoras
│   └── dependabot.yml             # Actualizaciones semanales de módulos de Go y acciones
├── e2e/                           # Pruebas de punta a punta: compilan el programa y lo ejecutan
├── installer/fast-folder-cli.iss  # Asistente de instalación (Inno Setup)
├── install.ps1                    # Instalador de un solo comando para Windows
├── shell/                         # Comando fcd para PowerShell (fcd.ps1) y cmd (fcd.cmd)
├── main.go                        # Punto de entrada
├── internal/
│   ├── cli/                       # Banderas, formato de salida y colores de consola
│   ├── tui/                       # Modo interactivo con flechas (Bubble Tea)
│   ├── query/                     # Qué se busca (término, proyectos, fecha): validación y descripción comunes a cli y tui
│   ├── search/                    # Búsqueda concurrente, coincidencia sin acentos, proyectos y tamaños
│   ├── period/                    # Periodos de fecha en español ("hoy", "semana", "7d")
│   ├── pathutil/                  # Resolución de %VARIABLES%, ~ y unidades
│   ├── apps/                      # Apps instaladas (registro de Windows y menú Inicio) y su carpeta
│   ├── jumplist/                  # Lector de las jump lists de Windows (compound files y DestList)
│   ├── recent/                    # Carpetas recientes a partir de las jump lists, y nombre de cada programa
│   ├── lnk/                       # Lector de accesos directos .lnk
│   ├── config/                    # Archivo de configuración (TOML) y carpetas recientes del modo interactivo
│   ├── launch/                    # Explorador, VS Code, terminal y portapapeles
│   └── humanize/                  # Formato de números ("52,341"), tamaños ("1.5 KB") y plurales
├── go.mod
├── LICENSE
├── SECURITY.md                    # Cómo reportar una vulnerabilidad
└── README.md
```

## 🧪 Desarrollo

```powershell
go test ./...         # todas las pruebas; las de e2e/ compilan el programa y lo ejecutan de punta a punta
go test -short ./...  # sin las pruebas de punta a punta (más rápido)
go vet ./...          # análisis estático
gofmt -l .            # comprobar formato
```

Para generar el asistente de instalación localmente, instala
[Inno Setup](https://jrsoftware.org/isdl.php) (6.7 o superior), deja los dos ejecutables en
`dist\` y ejecuta:

```powershell
ISCC.exe /DAppVersion=1.0.0 installer\fast-folder-cli.iss   # genera dist\fast-folder-cli-setup.exe
```

### Publicar una nueva versión

Las versiones se publican automáticamente con GitHub Actions
([`release.yml`](.github/workflows/release.yml)). Basta con crear y subir una etiqueta
[semántica](https://semver.org/lang/es/):

```powershell
git tag v1.1.0
git push origin v1.1.0
```

El workflow entonces:

1. Ejecuta las pruebas en Windows (con Go 1.26 y con la versión estable más reciente) y en
   Linux con el detector de condiciones de carrera (`-race`).
2. Compila `fast-folder-cli-windows-amd64.exe` y `fast-folder-cli-windows-arm64.exe` con la
   versión tomada de la etiqueta, e incrusta en ellos el nombre del producto y la versión
   (los que Windows muestra en **Propiedades → Detalles**) con
   [go-winres](https://github.com/tc-hib/go-winres). El workflow comprueba que esos datos
   coinciden con la versión en los tres ejecutables.
3. Genera el asistente `fast-folder-cli-setup.exe` con [Inno Setup](https://jrsoftware.org/isinfo.php)
   (versión y SHA-256 fijados en el workflow) y comprueba en Windows que se instala y
   desinstala correctamente en modo silencioso, igual que `install.ps1` con los archivos
   recién compilados (`-SourceDir`): ejecutables, `fcd` y menú contextual.
4. Crea el release en GitHub con los tres ejecutables, los scripts de `fcd`, `checksums.txt`
   (SHA-256) y notas generadas a partir de los mensajes de commit, agrupados por tipo (`feat`,
   `fix`, `docs`...).
5. Instala la versión recién publicada con [`install.ps1`](install.ps1) en un Windows limpio
   (Windows PowerShell 5.1 y PowerShell 7), la usa y la desinstala, para garantizar que el
   comando de instalación funciona.

Las etiquetas con sufijo (`v1.2.0-rc.1`) se publican como *prerelease*. Para comprobar que
todo compila sin publicar nada, ejecuta el workflow a mano desde la pestaña **Actions**
(**Release → Run workflow**): los ejecutables quedarán disponibles como artefacto y el
instalador se probará con la última versión publicada.

## 🤝 Contribuir

1. Haz un *fork* del repositorio y crea una rama: `git checkout -b feat/mi-mejora`.
2. Asegúrate de que `go test ./...` y `go vet ./...` pasan.
3. Usa [Conventional Commits](https://www.conventionalcommits.org/es/) en tus mensajes
   (`feat:`, `fix:`, `docs:`, `refactor:`...).
4. Abre un *pull request* describiendo el cambio.

## 📄 Licencia

Distribuido bajo la licencia MIT. Consulta el archivo [LICENSE](LICENSE) para más detalles.

## Code signing policy

Los ejecutables para Windows (`fast-folder-cli-windows-amd64.exe`, `fast-folder-cli-windows-arm64.exe`
y `fast-folder-cli-setup.exe`) todavía **no están firmados digitalmente**. Se compilan en
GitHub Actions a partir del código de este repositorio
([`release.yml`](.github/workflows/release.yml)), y cada versión publica sus sumas SHA-256 en
`checksums.txt`. Cuando haya versiones firmadas, la firma también se hará en GitHub Actions y
cada una requerirá la aprobación manual del mantenedor.

- **Committers and reviewers:** [@AnthonyCZ6](https://github.com/AnthonyCZ6)
- **Approvers:** [@AnthonyCZ6](https://github.com/AnthonyCZ6)

**Privacy policy:** This program will not transfer any information to other networked systems
unless specifically requested by the user or the person installing or operating it.

fast-folder-cli solo lee los nombres de las carpetas de tu equipo y no se conecta a internet. El
instalador de un solo comando descarga el ejecutable desde GitHub porque tú lo ejecutas.

## 📬 Contacto

**Anthony** — autor y mantenedor

- GitHub: [@AnthonyCZ6](https://github.com/AnthonyCZ6)
- Reporta errores o propone mejoras en
  [Issues](https://github.com/AnthonyCZ6/fast-folder-cli/issues).
- ¿Encontraste una vulnerabilidad? Repórtala en privado siguiendo la
  [política de seguridad](SECURITY.md).

Si esta herramienta te ahorra tiempo, ¡considera darle una ⭐ al repositorio!
