package e2e

import (
	"os/exec"
	"testing"
)

// --apps lee la lista real de programas instalados de este Windows.
func TestAppsOnThisPC(t *testing.T) {
	r := mustRun(t, 0, "--apps")
	assertContains(t, r.stdout, "Buscando apps entre los programas instalados", "apps encontradas")

	// Git está instalado en los runners de Windows de GitHub. En otro PC
	// puede no estarlo.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git no está instalado")
	}
	r = mustRun(t, 0, "--apps", "git")
	assertContains(t, r.stdout, `Buscando apps "git"`, "] Git  ")
}
