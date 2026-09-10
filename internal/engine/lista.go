package engine

import (
	"sync"
	"time"
)

// Lista es el ClipSource más simple que existe: los mismos clips, una vuelta
// y otra, cada uno entero. No mira el reloj y no fija cortes.
//
// Es lo que usa el arnés de f0, donde el plan no existe porque lo que se mide
// es el motor y no la parrilla (§22.1). El de verdad —el que lee el plan— vive
// en internal/app.
type Lista struct {
	mu      sync.Mutex
	clips   []Clip
	i       int
	relleno Clip
}

// NuevaLista devuelve la fuente de una lista circular. Con la lista vacía
// todo lo que sale es el relleno.
func NuevaLista(clips []Clip, relleno Clip) *Lista {
	return &Lista{clips: clips, relleno: relleno}
}

// Next devuelve el siguiente clip de la vuelta. El instante de corte es cero:
// cada clip sale entero, dure lo que diga el archivo.
func (l *Lista) Next(time.Time) (Clip, time.Time, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.clips) == 0 {
		return l.relleno, time.Time{}, nil
	}
	c := l.clips[l.i%len(l.clips)]
	l.i++
	return c, time.Time{}, nil
}

// Filler es el relleno de la lista.
func (l *Lista) Filler() Clip {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.relleno
}
