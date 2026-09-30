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
| 🔤 **Búsqueda flexible** | Coincidencia parcial sin distinguir mayúsculas, o patrones con comodines `*`, `?` y `[ ]`. |
| 🌐 **Variables de entorno** | Entiende `%APPDATA%`, `%LOCALAPPDATA%`, `%USERPROFILE%`, `%PROGRAMDATA%`, `~` y `D:` incluso desde PowerShell. |
| 🛡️ **Tolerante a errores** | Las carpetas sin permisos se omiten y se contabilizan; la búsqueda nunca se detiene. |
| 🔁 **Seguro con enlaces** | Las uniones (*junctions*) y enlaces simbólicos se reportan pero no se recorren, evitando ciclos infinitos. |
| 👻 **Ocultas y de sistema** | Se ignoran por defecto (más rápido y menos ruido); `--all` las incluye. |
| 📂 **Integración con el Explorador** | `--open` abre la primera coincidencia en cuanto se encuentra. |
| ⌨️ **Modo interactivo** | Escribe `fast` sin argumentos y busca, elige ubicación y abre carpetas usando solo las flechas. |
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
- Se registra en **Configuración → Aplicaciones → Aplicaciones instaladas**, desde donde se
  desinstala. Para actualizar, basta con ejecutar el asistente de una versión nueva.

> ⚠️ El asistente no está firmado digitalmente, así que la primera vez Windows SmartScreen
> puede mostrar **"Windows protegió su PC"**: pulsa **Más información → Ejecutar de todas
> formas**.

### ⚡ Desde la terminal, con un solo comando

Abre **PowerShell** y ejecuta:

```powershell
irm https://raw.githubusercontent.com/AnthonyCZ6/fast-folder-cli/main/install.ps1 | iex
```

Eso es todo: ya puedes usar `fast-folder-cli` en esa misma ventana. El
[script](install.ps1):

- Detecta si tu equipo es x64 o ARM64 y descarga el ejecutable de la última versión.
- Comprueba su suma SHA-256 antes de instalarlo.
- Lo copia en `%LOCALAPPDATA%\Programs\fast-folder-cli`, junto con el atajo **`fast`**, y
  agrega esa carpeta al PATH de tu usuario, **sin permisos de administrador** y sin alterar
  el resto del PATH.

| Para... | Ejecuta |
| --- | --- |
| **Actualizar** a la última versión | El mismo comando de instalación. |
| Instalar una **versión concreta** | `& ([scriptblock]::Create((irm https://raw.githubusercontent.com/AnthonyCZ6/fast-folder-cli/main/install.ps1))) -Version v1.0.0` |
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
> antivirus pueden mostrar una advertencia la primera vez. Para comprobar que el archivo es
> el original, compara `Get-FileHash .\fast-folder-cli.exe` con el valor de `checksums.txt`
> publicado en la versión. El instalador de un solo comando hace esta comprobación por ti.

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

> 💡 **Atajo `fast`:** el asistente y el script de instalación lo crean automáticamente. Si
> instalas con Go o a mano, copia `fast-folder-cli.exe` como `fast.exe` en la misma carpeta.

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
 fast-folder-cli   v1.1.0

 ► Buscar     proyecto
   Ubicación  ◄ Documentos ►  C:\Users\usuario\Documents
   Ocultas      No            carpetas ocultas y de sistema

 ↑↓ moverse   ←→ cambiar opción   Enter buscar   Esc salir
```

1. Escribe el nombre (o parte del nombre) de la carpeta.
2. Con **↓** baja a **Ubicación** y elige dónde buscar con **← →**: tu perfil, Escritorio,
   Documentos, Descargas, la carpeta actual, AppData, ProgramData o cualquier unidad (C:, D:...).
3. Con **↓** baja a **Ocultas** y actívalas con **← →** si también quieres buscar en
   carpetas ocultas y de sistema.
4. Pulsa **Enter**. Los resultados aparecen conforme se encuentran:

```text
 fast-folder-cli   "proyecto" en C:\Users\usuario\Documents

 ► Proyecto-Final          C:\Users\usuario\Documents
   mi-proyecto             C:\Users\usuario\Documents\escuela

 2 carpetas encontradas · 1,284 analizadas · 1/2 · 38 ms
 ↑↓ moverse   Enter abrir en el Explorador   ← nueva búsqueda   Esc salir
```

| Tecla | En la búsqueda | En los resultados |
| --- | --- | --- |
| <kbd>↑</kbd> <kbd>↓</kbd> | Cambiar de campo | Moverse por la lista (<kbd>RePág</kbd> <kbd>AvPág</kbd> <kbd>Inicio</kbd> <kbd>Fin</kbd> para saltar) |
| <kbd>←</kbd> <kbd>→</kbd> | Cambiar la ubicación o las ocultas | <kbd>→</kbd> abre la carpeta · <kbd>←</kbd> vuelve a la búsqueda |
| <kbd>Enter</kbd> | Buscar | Abrir la carpeta en el Explorador |
| <kbd>Esc</kbd> | Salir | Salir |

### Con comandos (para scripts)

```text
fast-folder-cli -n <término> [-p <ruta>] [-a] [-o]
fast-folder-cli <término> [opciones]
```

Con argumentos, la herramienta funciona como un comando normal: imprime los resultados y
termina, así que se puede usar en scripts y redirigir su salida.

> Si tienes otro programa llamado `fast` (por ejemplo, el medidor de velocidad de npm
> `fast-cli`), el que aparezca primero en el PATH tendrá prioridad; el nombre completo
> `fast-folder-cli` siempre funciona.

### Banderas

| Corta | Larga | Descripción | Por defecto |
| --- | --- | --- | --- |
| `-n` | `--name` | Término o patrón a buscar en los nombres de carpeta. También puede indicarse como argumento posicional. | *(obligatorio)* |
| `-p` | `--path` | Carpeta raíz desde la que comienza la búsqueda. Admite variables de entorno. | `%USERPROFILE%` |
| `-a` | `--all` | Incluye carpetas ocultas y de sistema. | desactivado |
| `-o` | `--open` | Abre la primera carpeta encontrada en el Explorador de Windows. | desactivado |
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
```

### Reglas de coincidencia

Las búsquedas **no distinguen mayúsculas de minúsculas**, igual que Windows.

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
| `0` | Se encontró al menos una carpeta. |
| `1` | La búsqueda terminó sin coincidencias. |
| `2` | Error de uso: bandera desconocida, ruta inexistente o patrón inválido. |
| `130` | Búsqueda interrumpida con <kbd>Ctrl</kbd> + <kbd>C</kbd> (se muestran los resultados parciales). |

---

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
│   ├── workflows/release.yml      # Pruebas, compilación y publicación de versiones
│   └── scripts/release-notes.sh   # Notas de versión a partir de los commits
├── installer/fast-folder-cli.iss  # Asistente de instalación (Inno Setup)
├── install.ps1                    # Instalador de un solo comando para Windows
├── main.go                        # Punto de entrada
├── internal/
│   ├── cli/                       # Banderas, formato de salida y colores de consola
│   ├── tui/                       # Modo interactivo con flechas (Bubble Tea)
│   ├── search/                    # Motor de búsqueda concurrente y coincidencia de nombres
│   ├── pathutil/                  # Resolución de %VARIABLES%, ~ y unidades
│   ├── explorer/                  # Apertura de carpetas en el Explorador de Windows
│   └── humanize/                  # Formato de números ("52,341") y plurales
├── go.mod
├── LICENSE
└── README.md
```

## 🧪 Desarrollo

```powershell
go test ./...     # pruebas unitarias (incluye pruebas específicas de Windows: atributos y junctions)
go vet ./...      # análisis estático
gofmt -l .        # comprobar formato
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
   versión tomada de la etiqueta.
3. Genera el asistente `fast-folder-cli-setup.exe` con [Inno Setup](https://jrsoftware.org/isinfo.php)
   (versión y SHA-256 fijados en el workflow) y comprueba en Windows que se instala y
   desinstala correctamente en modo silencioso.
4. Crea el release en GitHub con los tres ejecutables, `checksums.txt` (SHA-256) y notas
   generadas a partir de los mensajes de commit, agrupados por tipo (`feat`, `fix`, `docs`...).
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

## 📬 Contacto

**Anthony** — autor y mantenedor

- GitHub: [@AnthonyCZ6](https://github.com/AnthonyCZ6)
- Correo: [20243ds055@utez.edu.mx](mailto:20243ds055@utez.edu.mx)
- Reporta errores o propone mejoras en
  [Issues](https://github.com/AnthonyCZ6/fast-folder-cli/issues).

Si esta herramienta te ahorra tiempo, ¡considera darle una ⭐ al repositorio!
