package search

import (
	"context"
	"io/fs"
	"sync/atomic"
)

// SizeInfo es el resultado de Size.
type SizeInfo struct {
	Bytes  int64 // suma del tamaño de los archivos
	Files  int64 // número de archivos
	Denied int64 // carpetas que no se pudieron leer
}

// Size calcula cuánto ocupan los archivos de root y de todas sus subcarpetas,
// incluidas las ocultas. Los enlaces y uniones no se siguen, para no contar
// dos veces el mismo contenido. Si ctx se cancela, devuelve lo sumado hasta
// ese momento.
func Size(ctx context.Context, root string) SizeInfo {
	s := NewSizer(ctx)
	s.Add(root)
	return s.Wait()[0]
}

// Sizer calcula el tamaño de varias carpetas a la vez con un único grupo de
// goroutines (ver pool). Cada carpeta empieza a medirse en cuanto se añade,
// así que se pueden ir añadiendo mientras sigue una búsqueda, y las carpetas
// pequeñas no dejan núcleos ociosos.
type Sizer struct {
	ctx     context.Context
	pool    *pool
	folders []*sizer
}

// NewSizer crea un Sizer. Si ctx se cancela, las mediciones en curso se
// detienen y Wait devuelve lo sumado hasta ese momento.
func NewSizer(ctx context.Context) *Sizer {
	return &Sizer{ctx: ctx, pool: newPool(DefaultWorkers())}
}

// Add empieza a medir path, con las mismas reglas que Size. Si todas las
// goroutines están ocupadas, lo mide en la goroutine que llama. No debe
// llamarse desde varias goroutines a la vez ni después de Wait.
func (s *Sizer) Add(path string) {
	f := &sizer{ctx: s.ctx, pool: s.pool}
	s.folders = append(s.folders, f)
	s.pool.do(func() { f.walk(path) })
}

// Wait espera a que terminen todas las mediciones y devuelve el tamaño de
// cada carpeta, en el orden en que se añadieron.
func (s *Sizer) Wait() []SizeInfo {
	s.pool.wait()
	infos := make([]SizeInfo, len(s.folders))
	for i, f := range s.folders {
		infos[i] = SizeInfo{Bytes: f.bytes.Load(), Files: f.files.Load(), Denied: f.denied.Load()}
	}
	return infos
}

// sizer suma el tamaño de una carpeta; sus subcarpetas se reparten en el pool
// compartido.
type sizer struct {
	ctx    context.Context
	pool   *pool
	bytes  atomic.Int64
	files  atomic.Int64
	denied atomic.Int64
}

func (s *sizer) walk(dir string) {
	if s.ctx.Err() != nil {
		return
	}

	entries, ok := readDir(dir)
	if !ok {
		s.denied.Add(1)
		return
	}

	for _, e := range entries {
		if kind := inspect(dir, e); kind.dir {
			if !kind.link {
				path := join(dir, e.Name())
				s.pool.do(func() { s.walk(path) })
			}
			continue
		}
		// Los enlaces a archivos no se cuentan. Los archivos de OneDrive
		// solo en la nube sí: se suma su tamaño lógico.
		if e.Type()&fs.ModeSymlink != 0 {
			continue
		}
		if info, err := e.Info(); err == nil {
			s.bytes.Add(info.Size())
			s.files.Add(1)
		}
	}
}
