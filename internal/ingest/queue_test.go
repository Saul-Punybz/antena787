package ingest

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ── el aire manda sobre la preparación (ADR 0008) ─────────────────────

// TestLaColaEsperaMientrasElAireSufre comprueba la opción 4: con el aire
// atrasado la cola no toca un archivo, y sigue sola en cuanto se recupera. Es
// la diferencia entre preparar la biblioteca y tumbarle la señal a alguien.
func TestLaColaEsperaMientrasElAireSufre(t *testing.T) {
	q := NewQueue()
	var sufriendo atomic.Bool
	sufriendo.Store(true)
	q.Permiso = func() (bool, string) {
		if sufriendo.Load() {
			return false, "el aire va atrasado"
		}
		return true, ""
	}
	var dm sync.Mutex
	var dichos []string
	q.Aviso = func(texto string) { dm.Lock(); dichos = append(dichos, texto); dm.Unlock() }

	hechos := make(chan int64, 4)
	q.Enqueue(7, time.Now())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = q.Run(ctx, nil, func(_ context.Context, j Job) (string, error) {
			hechos <- j.AssetID
			return "", nil
		})
	}()

	// Mientras el aire sufre, nadie toca el archivo.
	select {
	case id := <-hechos:
		t.Fatalf("la cola preparó el archivo %d con el aire atrasado: preparar no puede costarle la señal a nadie", id)
	case <-time.After(2 * PermisoPoll):
	}
	dm.Lock()
	n := len(dichos)
	dm.Unlock()
	if n == 0 {
		t.Fatal("la cola se paró sin decir por qué: en la bitácora tiene que quedar constancia")
	}

	// Y en cuanto el aire se pone al día, sigue sola, sin que nadie la empuje.
	sufriendo.Store(false)
	select {
	case id := <-hechos:
		if id != 7 {
			t.Fatalf("preparó el archivo %d y el que estaba en la fila era el 7", id)
		}
	case <-time.After(4 * PermisoPoll):
		t.Fatal("el aire se puso al día y la cola no siguió sola")
	}
}

// TestSinPermisoLaColaSeComportaComoSiempre: quien no ponga la puerta no nota
// que existe. Lo que ya funcionaba tiene que seguir funcionando igual.
func TestSinPermisoLaColaSeComportaComoSiempre(t *testing.T) {
	q := NewQueue()
	hecho := make(chan struct{})
	q.Enqueue(3, time.Now())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = q.Run(ctx, nil, func(context.Context, Job) (string, error) {
			close(hecho)
			return "", nil
		})
	}()
	select {
	case <-hecho:
	case <-time.After(3 * time.Second):
		t.Fatal("sin Permiso puesto la cola tiene que preparar sin esperar a nadie")
	}
}

// TestLaPuertaNoSePreguntaConLaFilaVacia: la cola espera permiso ANTES de
// sacar de la fila, así que lo que entre mientras el aire sufre se ordena
// igual por su hora de aire. Si se guardara el trabajo ya sacado, al reanudar
// saldría uno viejo delante de otro que corre más prisa.
func TestLoQueEntraMientrasElAireSufreSeOrdenaIgual(t *testing.T) {
	q := NewQueue()
	var sufriendo atomic.Bool
	sufriendo.Store(true)
	q.Permiso = func() (bool, string) {
		if sufriendo.Load() {
			return false, "el aire va atrasado"
		}
		return true, ""
	}

	ahora := time.Now()
	// El 1 sale la semana que viene; entra primero a la fila.
	q.Enqueue(1, ahora.Add(7*24*time.Hour))

	hechos := make(chan int64, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = q.Run(ctx, nil, func(_ context.Context, j Job) (string, error) {
			hechos <- j.AssetID
			return "", nil
		})
	}()

	// Mientras el aire sufre entra el 2, que sale dentro de una hora.
	time.Sleep(PermisoPoll / 2)
	q.Enqueue(2, ahora.Add(time.Hour))
	sufriendo.Store(false)

	select {
	case id := <-hechos:
		if id != 2 {
			t.Fatalf("salió primero el %d: tenía que salir el 2, que es el que sale al aire antes", id)
		}
	case <-time.After(4 * PermisoPoll):
		t.Fatal("la cola no siguió cuando el aire se puso al día")
	}
}
