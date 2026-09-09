package app

import (
	"sync"
	"time"
)

// Event es lo que sale por el WebSocket cuando algo pasa: un incidente, un
// plan que cambió, un archivo que entró. El texto está escrito para que lo
// lea una persona.
type Event struct {
	Type   string    `json:"tipo"`   // siempre "evento"
	Kind   string    `json:"clase"`  // incidente | plan | material | goroutine | ingest
	Name   string    `json:"nombre"` // el tipo de incidente, el nombre de la goroutine…
	Detail string    `json:"detalle"`
	At     time.Time `json:"instante"`
}

// bus reparte eventos a quien esté mirando. Un suscriptor lento no atrasa a
// nadie: su evento se descarta y el aire sigue.
type bus struct {
	mu     sync.Mutex
	subs   map[int]chan Event
	next   int
	closed bool
}

func newBus() *bus { return &bus{subs: map[int]chan Event{}} }

// Subscribe devuelve un canal de eventos y la función para soltarlo.
func (a *App) Subscribe() (<-chan Event, func()) { return a.bus.subscribe() }

func (b *bus) subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Event, 32)
	if b.closed {
		close(ch)
		return ch, func() {}
	}
	id := b.next
	b.next++
	b.subs[id] = ch
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if c, ok := b.subs[id]; ok {
			delete(b.subs, id)
			close(c)
		}
	}
}

func (b *bus) publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

func (b *bus) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for id, ch := range b.subs {
		delete(b.subs, id)
		close(ch)
	}
}

// Publish empuja un evento a todo el que esté mirando.
func (a *App) Publish(kind, name, detail string) {
	a.bus.publish(Event{Type: "evento", Kind: kind, Name: name, Detail: detail, At: a.Now()})
}
