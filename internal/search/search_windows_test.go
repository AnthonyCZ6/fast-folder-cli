//go:build windows

package search

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
)

func setAttributes(t *testing.T, path string, attrs uint32) {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetFileAttributes(p, attrs); err != nil {
		t.Fatalf("SetFileAttributes(%s): %v", path, err)
	}
}

func TestSearchHiddenAndSystemAttributes(t *testing.T) {
	root := makeTree(t, "oculta/datos", "sistema/datos", "normal/datos")
	setAttributes(t, filepath.Join(root, "oculta"), syscall.FILE_ATTRIBUTE_HIDDEN)
	setAttributes(t, filepath.Join(root, "sistema"), syscall.FILE_ATTRIBUTE_SYSTEM)

	if got, want := run(t, root, "datos", false, 0), []string{"normal/datos"}; !slices.Equal(got, want) {
		t.Errorf("sin --all: got %v, want %v", got, want)
	}
	want := []string{"normal/datos", "oculta/datos", "sistema/datos"}
	if got := run(t, root, "datos", true, 0); !slices.Equal(got, want) {
		t.Errorf("con --all: got %v, want %v", got, want)
	}
}

func TestSearchReportsButDoesNotFollowJunctions(t *testing.T) {
	root := makeTree(t, "real/objetivo")

	// Unión (junction) que apunta a la raíz: seguirla produciría un ciclo
	// infinito real/enlace-objetivo/real/enlace-objetivo/...
	// mklink /J no requiere privilegios de administrador.
	link := filepath.Join(root, "real", "enlace-objetivo")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, root).CombinedOutput(); err != nil {
		t.Skipf("no se pudo crear la unión: %v: %s", err, out)
	}

	got := run(t, root, "objetivo", false, 0)
	want := []string{"real/enlace-objetivo", "real/objetivo"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSizeDoesNotFollowJunctions(t *testing.T) {
	root := makeTree(t, "datos")
	if err := os.WriteFile(filepath.Join(root, "datos", "archivo.bin"), make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "enlace")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, filepath.Join(root, "datos")).CombinedOutput(); err != nil {
		t.Skipf("no se pudo crear la unión: %v: %s", err, out)
	}

	info := Size(context.Background(), root)
	if info.Bytes != 100 || info.Files != 1 {
		t.Errorf("Size = %+v, want 100 bytes en 1 archivo (sin contar la unión)", info)
	}
}
