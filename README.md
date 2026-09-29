# ⚡ fast-folder-cli

![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go&logoColor=white)
![Windows](https://img.shields.io/badge/Windows-10%20%7C%2011-0078D6?logo=windows&logoColor=white)
![Dependencias](https://img.shields.io/badge/dependencias-0-brightgreen)
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
| 📦 **Sin dependencias** | Solo la biblioteca estándar de Go: un único `.exe` de ~2.3 MB. |

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

### Requisitos

- Windows 10 u 11 (también compila en Linux y macOS, con funciones limitadas).
- [Go 1.23 o superior](https://go.dev/dl/) para compilar. Con winget:

  ```powershell
  winget install GoLang.Go
  ```

### Compilar el ejecutable `.exe`

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

**Compilación cruzada** desde Linux o macOS (o para equipos Windows ARM):

```bash
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o fast-folder-cli.exe .
GOOS=windows GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o fast-folder-cli-arm64.exe .
```

### Agregar al PATH de Windows

Así podrás ejecutar `fast-folder-cli` desde cualquier carpeta y cualquier terminal.

#### Opción A — PowerShell (recomendada, no requiere administrador)

Desde la carpeta del proyecto, después de compilar:

```powershell
$dest = "$env:LOCALAPPDATA\Programs\fast-folder-cli"
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Copy-Item .\fast-folder-cli.exe $dest -Force

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($userPath -split ";") -notcontains $dest) {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$dest", "User")
}
```

#### Opción B — Interfaz gráfica

1. Copia `fast-folder-cli.exe` a una carpeta permanente, por ejemplo
   `C:\Users\<usuario>\AppData\Local\Programs\fast-folder-cli`.
2. Pulsa <kbd>Win</kbd> + <kbd>R</kbd>, escribe `SystemPropertiesAdvanced` y pulsa
   <kbd>Enter</kbd>.
3. Haz clic en **Variables de entorno...**.
4. En **Variables de usuario**, selecciona `Path` → **Editar** → **Nuevo** y pega la ruta
   de la carpeta.
5. Acepta todas las ventanas.

#### Opción C — `go install`

```powershell
go install -trimpath -ldflags "-s -w" .
```

El ejecutable se instala en `%USERPROFILE%\go\bin`, que el instalador de Go ya agrega al PATH.

#### Verificar la instalación

Cierra y vuelve a abrir la terminal (el PATH solo se lee al iniciarla) y ejecuta:

```powershell
fast-folder-cli --version
```

> 💡 **Atajo:** renombra el ejecutable a `ff.exe` o crea un alias en tu perfil de PowerShell
> (`notepad $PROFILE`): `Set-Alias ff fast-folder-cli`

---

## 🚀 Uso

```text
fast-folder-cli -n <término> [-p <ruta>] [-a] [-o]
fast-folder-cli <término> [opciones]
```

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
├── main.go                        # Punto de entrada
├── internal/
│   ├── cli/                       # Banderas, formato de salida y colores de consola
│   ├── search/                    # Motor de búsqueda concurrente y coincidencia de nombres
│   ├── pathutil/                  # Resolución de %VARIABLES%, ~ y unidades
│   └── explorer/                  # Apertura de carpetas en el Explorador de Windows
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

## 🤝 Contribuir

1. Haz un *fork* del repositorio y crea una rama: `git checkout -b feat/mi-mejora`.
2. Asegúrate de que `go test ./...` y `go vet ./...` pasan.
3. Usa [Conventional Commits](https://www.conventionalcommits.org/es/) en tus mensajes
   (`feat:`, `fix:`, `docs:`, `refactor:`...).
4. Abre un *pull request* describiendo el cambio.

## 📄 Licencia

Distribuido bajo la licencia MIT. Consulta el archivo [LICENSE](LICENSE) para más detalles.

## 📬 Contacto

**Tony** — autor y mantenedor

- GitHub: [@AnthonyCZ6](https://github.com/AnthonyCZ6)
- Correo: [20243ds055@utez.edu.mx](mailto:20243ds055@utez.edu.mx)
- Reporta errores o propone mejoras en
  [Issues](https://github.com/AnthonyCZ6/fast-folder-cli/issues).

Si esta herramienta te ahorra tiempo, ¡considera darle una ⭐ al repositorio!
