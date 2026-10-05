package recent

import (
	"path/filepath"
	"strings"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
)

// AppName es el nombre de un programa con jump list.
type AppName struct {
	Name    string   // como se muestra: "Word", "Visual Studio Code"
	Aliases []string // otros nombres por los que se puede buscar: "WINWORD", "code"
}

// Matches indica si match acepta el nombre o alguno de los alias.
func (a AppName) Matches(match func(string) bool) bool {
	if match(a.Name) {
		return true
	}
	for _, alias := range a.Aliases {
		if match(alias) {
			return true
		}
	}
	return false
}

// wellKnown son programas cuyo AppUserModelID se conoce de antemano: las
// apps de Windows (que no tienen acceso directo .lnk) y los programas que no
// siempre lo guardan en el suyo, como VS Code. Van primero, así que su
// nombre gana al de un acceso directo con el mismo AppUserModelID.
var wellKnown = []candidate{
	{"Microsoft.Windows.Explorer", AppName{"Explorador de archivos", []string{"explorer"}}},
	{"Microsoft.Office.WINWORD.EXE.15", AppName{"Word", []string{"winword"}}},
	{"Microsoft.Office.EXCEL.EXE.15", AppName{"Excel", nil}},
	{"Microsoft.Office.POWERPNT.EXE.15", AppName{"PowerPoint", []string{"powerpnt"}}},
	{"Microsoft.Office.ONENOTE.EXE.15", AppName{"OneNote", nil}},
	{"Microsoft.Office.OUTLOOK.EXE.15", AppName{"Outlook", nil}},
	{"Microsoft.Office.MSACCESS.EXE.15", AppName{"Access", nil}},
	{"Microsoft.Office.MSPUB.EXE.15", AppName{"Publisher", nil}},
	{"Microsoft.VisualStudioCode", AppName{"Visual Studio Code", []string{"vscode", "code"}}},
	{"Chrome", AppName{"Google Chrome", nil}},
	{"MSEdge", AppName{"Microsoft Edge", nil}},
	{"308046B0AF4A39CB", AppName{"Firefox", nil}},
	{"Microsoft.WindowsNotepad_8wekyb3d8bbwe!App", AppName{"Bloc de notas", []string{"notepad"}}},
	{`{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\notepad.exe`, AppName{"Bloc de notas", []string{"notepad"}}},
	{"Microsoft.Windows.Photos_8wekyb3d8bbwe!App", AppName{"Fotos", []string{"photos"}}},
	{"Microsoft.Paint_8wekyb3d8bbwe!App", AppName{"Paint", []string{"mspaint"}}},
	{`{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\mspaint.exe`, AppName{"Paint", []string{"mspaint"}}},
	{"Microsoft.ZuneMusic_8wekyb3d8bbwe!Microsoft.ZuneMusic", AppName{"Reproductor multimedia", []string{"media player"}}},
	{"Microsoft.ScreenSketch_8wekyb3d8bbwe!App", AppName{"Recortes", []string{"snipping tool"}}},
	{"Microsoft.WindowsTerminal_8wekyb3d8bbwe!App", AppName{"Terminal", []string{"windows terminal"}}},
	{"windows.immersivecontrolpanel_cw5n1h2txyewy!microsoft.windows.immersivecontrolpanel", AppName{"Configuración", []string{"settings"}}},
	{"Microsoft.XboxGamingOverlay_8wekyb3d8bbwe!App", AppName{"Xbox Game Bar", nil}},
}

// candidate es un identificador posible de un programa: su AppUserModelID o
// la ruta de su ejecutable.
type candidate struct {
	id   string
	name AppName
}

// knownFolder es una carpeta conocida que Windows sustituye por su GUID en
// las rutas de los ejecutables antes de calcular su AppID:
// C:\Program Files\App\app.exe → {6D809377-...}\App\app.exe.
type knownFolder struct {
	guid string // "{6D809377-6AF0-444B-8957-A3773F02200E}"
	path string // "C:\Program Files"
}

// AppNames devuelve el nombre de cada programa cuya jump list se puede
// identificar, por su AppID: los de la tabla, los de los accesos directos
// del menú Inicio y los de los programas instalados.
func AppNames() map[uint64]AppName {
	return buildNames(append(wellKnown[:len(wellKnown):len(wellKnown)], systemCandidates()...), knownFolders())
}

// buildNames calcula el AppID de cada candidato y, si es una ruta, también el
// de sus formas con una carpeta conocida. Si dos candidatos dan el mismo
// AppID, gana el primero.
func buildNames(cands []candidate, folders []knownFolder) map[uint64]AppName {
	names := map[uint64]AppName{}
	add := func(id string, n AppName) {
		if n.Name == "" {
			return
		}
		if h := jumplist.AppID(id); names[h].Name == "" {
			names[h] = n
		}
	}
	for _, c := range cands {
		add(c.id, c.name)
		for _, kf := range folders {
			if rest, ok := cutDir(c.id, kf.path); ok {
				add(kf.guid+`\`+rest, c.name)
			}
		}
	}
	return names
}

// cutDir devuelve lo que sigue a dir en path si path está dentro de dir.
// Windows no distingue mayúsculas.
func cutDir(path, dir string) (string, bool) {
	dir = strings.TrimRight(dir, `\/`)
	if dir == "" || len(path) <= len(dir)+1 || !strings.EqualFold(path[:len(dir)], dir) {
		return "", false
	}
	if c := path[len(dir)]; c != '\\' && c != '/' {
		return "", false
	}
	return path[len(dir)+1:], true
}

// exeName devuelve el nombre de un ejecutable sin la extensión: "WINWORD".
func exeName(exe string) string {
	base := filepath.Base(strings.ReplaceAll(exe, `\`, "/"))
	return strings.TrimSuffix(base, filepath.Ext(base))
}
