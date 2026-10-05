package recent

import (
	"reflect"
	"strings"
	"testing"

	"github.com/AnthonyCZ6/fast-folder-cli/internal/jumplist"
)

func TestBuildNames(t *testing.T) {
	word := AppName{"Word", []string{"winword"}}
	tool := AppName{"Mi Herramienta", []string{"tool"}}
	cands := []candidate{
		{"Microsoft.Office.WINWORD.EXE.15", word},
		{"Microsoft.Office.WINWORD.EXE.15", AppName{"Otro nombre", nil}}, // el primero gana
		{`C:\Program Files\Herramienta\tool.exe`, tool},
		{`C:\Users\ana\AppData\Local\Programs\Editor\editor.exe`, AppName{"Editor", nil}},
		{"Sin.Nombre", AppName{}}, // sin nombre: no cuenta
	}
	folders := []knownFolder{
		{"{6D809377-6AF0-444B-8957-A3773F02200E}", `C:\Program Files`},
		{"{F38BF404-1D43-42F2-9305-67DE0B28FC23}", `C:\Windows\`},
	}
	names := buildNames(cands, folders)
	want := map[uint64]AppName{
		jumplist.AppID("Microsoft.Office.WINWORD.EXE.15"):                             word,
		jumplist.AppID(`C:\Program Files\Herramienta\tool.exe`):                       tool,
		jumplist.AppID(`{6D809377-6AF0-444B-8957-A3773F02200E}\Herramienta\tool.exe`): tool,
		jumplist.AppID(`C:\Users\ana\AppData\Local\Programs\Editor\editor.exe`):       {"Editor", nil},
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("buildNames =\n%v\nwant\n%v", names, want)
	}
}

// La tabla va antes que los programas del equipo: su nombre gana.
func TestWellKnownFirst(t *testing.T) {
	cands := append(wellKnown[:len(wellKnown):len(wellKnown)], candidate{"Microsoft.Windows.Explorer", AppName{"File Explorer", nil}})
	names := buildNames(cands, nil)
	if got := names[jumplist.AppID("Microsoft.Windows.Explorer")].Name; got != "Explorador de archivos" {
		t.Errorf("Explorador = %q", got)
	}
	if got := names[0xf01b4d95cf55d32a].Name; got != "Explorador de archivos" {
		t.Errorf("la jump list f01b4d95cf55d32a es la del Explorador, no %q", got)
	}
}

func TestCutDir(t *testing.T) {
	tests := []struct {
		path, dir, rest string
		ok              bool
	}{
		{`C:\Program Files\App\app.exe`, `C:\Program Files`, `App\app.exe`, true},
		{`c:\program files\App\app.exe`, `C:\Program Files\`, `App\app.exe`, true},
		{`C:\Program Files (x86)\App\app.exe`, `C:\Program Files`, "", false},
		{`C:\Program Files`, `C:\Program Files`, "", false},
		{`Microsoft.Office.WINWORD.EXE.15`, `C:\Program Files`, "", false},
		{`C:\x.exe`, "", "", false},
	}
	for _, tt := range tests {
		if rest, ok := cutDir(tt.path, tt.dir); rest != tt.rest || ok != tt.ok {
			t.Errorf("cutDir(%q, %q) = %q, %v; want %q, %v", tt.path, tt.dir, rest, ok, tt.rest, tt.ok)
		}
	}
}

func TestAppNameMatches(t *testing.T) {
	code := AppName{"Visual Studio Code", []string{"vscode", "code"}}
	contains := func(term string) func(string) bool {
		return func(s string) bool { return strings.Contains(strings.ToLower(s), term) }
	}
	for term, want := range map[string]bool{"studio": true, "vscode": true, "code": true, "word": false} {
		if got := code.Matches(contains(term)); got != want {
			t.Errorf("Matches(%q) = %v, want %v", term, got, want)
		}
	}
}

func TestExeName(t *testing.T) {
	for exe, want := range map[string]string{
		`C:\Program Files\Microsoft Office\WINWORD.EXE`: "WINWORD",
		`C:\Apps\mi.app.exe`:                            "mi.app",
		"code":                                          "code",
	} {
		if got := exeName(exe); got != want {
			t.Errorf("exeName(%q) = %q, want %q", exe, got, want)
		}
	}
}
