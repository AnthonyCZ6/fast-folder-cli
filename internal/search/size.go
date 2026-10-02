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
	s := &sizer{ctx: ctx, pool: newPool(DefaultWorkers())}
	s.walk(root)
	s.pool.wait()
	return SizeInfo{Bytes: s.bytes.Load(), Files: s.files.Load(), Denied: s.denied.Load()}
}

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
