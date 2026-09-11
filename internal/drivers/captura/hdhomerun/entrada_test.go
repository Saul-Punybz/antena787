package hdhomerun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func sinEsperaDeVerdad(ctx context.Context, d time.Duration) error {
	// Las pruebas no tienen por qué esperar el segundo/dos segundos/etc. de
	// verdad: lo único que importa es que Abrir cuenta los reintentos y
	// dobla la espera, no cuánto dura un time.Sleep.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func TestAbrirPrimerIntentoFallidoDevuelveError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	r := NuevoRetorno(&Cliente{Dormir: sinEsperaDeVerdad})
	_, err := r.Abrir(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("se esperaba error contra un servidor que no sirve nada")
	}
	if r.Estado().Conectado {
		t.Fatal("Conectado no debería quedar en true tras un fallo")
	}
}

func TestAbrirLeeElTSMientrasLaConexionAguanta(t *testing.T) {
	cuerpo := construirTSSintetico(5)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(cuerpo)
	}))
	defer srv.Close()

	r := NuevoRetorno(&Cliente{Dormir: sinEsperaDeVerdad})
	lc, err := r.Abrir(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Abrir: %v", err)
	}
	defer lc.Close()

	leido, err := io.ReadAll(lc)
	if err != nil {
		t.Fatalf("leer del lector: %v", err)
	}
	if len(leido) != len(cuerpo) {
		t.Fatalf("se leyeron %d bytes, se esperaban %d", len(leido), len(cuerpo))
	}
	if !r.Estado().Conectado {
		t.Fatal("Conectado debería quedar en true tras una lectura buena")
	}
}

// TestAbrirReconectaConEsperaProgresiva simula un HDHomeRun que se cae a
// media transmisión (cierra la conexión sin avisar) y vuelve a estar
// disponible poco después: Abrir tiene que reconectar solo, contar el
// reintento en Estado, y seguir entregando bytes sin que quien llama a Read
// haga nada especial.
func TestAbrirReconectaConEsperaProgresiva(t *testing.T) {
	var intento int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		n := atomic.AddInt32(&intento, 1)
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("el ResponseWriter de la prueba no sabe hacer Hijack")
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			t.Fatalf("Hijack: %v", err)
		}
		defer conn.Close()

		if n == 1 {
			// Primer intento: promete el doble de lo que manda y corta la
			// conexión — el cliente ve un cuerpo más corto de lo prometido,
			// que es exactamente lo que hace una red que se cae a media
			// transmisión.
			cuerpo := construirTSSintetico(2)
			fmt.Fprintf(bufrw, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", len(cuerpo)*2)
			bufrw.Write(cuerpo)
			bufrw.Flush()
			return
		}
		// Segundo intento en adelante: sirve completo y cierra limpio.
		cuerpo := construirTSSintetico(3)
		fmt.Fprintf(bufrw, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", len(cuerpo))
		bufrw.Write(cuerpo)
		bufrw.Flush()
	}))
	defer srv.Close()

	r := NuevoRetorno(&Cliente{Dormir: sinEsperaDeVerdad})
	lc, err := r.Abrir(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Abrir: %v", err)
	}
	defer lc.Close()

	leido, err := io.ReadAll(lc)
	if err != nil {
		t.Fatalf("leer del lector tras la reconexión: %v", err)
	}
	if len(leido) == 0 {
		t.Fatal("no se leyó nada")
	}
	if atomic.LoadInt32(&intento) < 2 {
		t.Fatalf("se esperaban al menos dos intentos de conexión, hubo %d", intento)
	}
	if r.Estado().Reintentos < 1 {
		t.Fatalf("Estado.Reintentos debería contar al menos uno, quedó en %d", r.Estado().Reintentos)
	}
	if !r.Estado().Conectado {
		t.Fatal("Conectado debería quedar en true tras reconectar")
	}
}

// TestAbrirCerrarDetieneLaReconexion comprueba que Close manda: quien cierra
// la entrada no quiere que siga reconectando a espaldas de nadie.
//
// Esta prueba estaba escrita de forma que nunca llegaba a su propia aserción
// (auditoría del 11 sept 2026, §2): el servidor hacía Hijack y cerraba el
// socket sin contestar, así que el primer conectar() siempre daba error, Abrir
// devolvía error, y el `if err != nil { return }` salía antes de probar Close.
// Ahora el servidor contesta de verdad, Abrir tiene que funcionar —si no, es un
// fallo, no un motivo para irse—, y se comprueban las dos mitades de «Close
// detiene la reconexión».
func TestAbrirCerrarDetieneLaReconexion(t *testing.T) {
	cuerpo := construirTSSintetico(2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write(cuerpo)
	}))
	defer srv.Close()

	r := NuevoRetorno(&Cliente{Dormir: sinEsperaDeVerdad})
	lc, err := r.Abrir(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Abrir contra un servidor que sí contesta: %v", err)
	}
	if cerr := lc.Close(); cerr != nil {
		t.Fatalf("Close: %v", cerr)
	}
	if _, err := lc.Read(make([]byte, 16)); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Read tras Close debería devolver io.ErrClosedPipe, dio %v", err)
	}
	// Cerrar dos veces no puede reventar: la pantalla puede pedir cerrar una
	// entrada que el motor ya cerró.
	if cerr := lc.Close(); cerr != nil {
		t.Fatalf("segundo Close: %v", cerr)
	}
}

// Y la otra mitad, que es la que de verdad importa: si la conexión se cae y
// alguien cierra mientras el bucle de reconexión está esperando su turno, el
// bucle se rinde en vez de seguir golpeando al equipo para siempre. Sin esto,
// apagar una entrada dejaría un reintento eterno contra el HDHomeRun.
func TestCerrarDuranteLaEsperaCortaElBucleDeReconexion(t *testing.T) {
	var peticiones int32
	primerCuerpo := construirTSSintetico(2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if atomic.AddInt32(&peticiones, 1) != 1 {
			http.NotFound(w, req) // a partir de la segunda el equipo no contesta
			return
		}
		// La primera conexión promete el doble de lo que manda y se corta: es
		// una caída a media transmisión, no un final limpio. Un cuerpo que
		// termina bien no dispara la reconexión (Read devuelve io.EOF con los
		// últimos bytes y quien lee se para ahí), y entonces esta prueba no
		// probaría nada — que es de lo que trata la auditoría.
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("el ResponseWriter de la prueba no sabe hacer Hijack")
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			t.Errorf("Hijack: %v", err)
			return
		}
		defer conn.Close()
		fmt.Fprintf(bufrw, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", len(primerCuerpo)*2)
		bufrw.Write(primerCuerpo)
		bufrw.Flush()
	}))
	defer srv.Close()

	// El cierre llega desde dentro de la espera del backoff, que es el momento
	// exacto en que el bucle está «dormido» y podría no enterarse.
	var lc io.ReadCloser
	cerrarEnLaEspera := func(ctx context.Context, d time.Duration) error {
		if lc != nil {
			lc.Close()
		}
		return nil
	}

	r := NuevoRetorno(&Cliente{Dormir: cerrarEnLaEspera})
	lc, err := r.Abrir(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Abrir: %v", err)
	}

	// Leer hasta el final: el cuerpo se acaba, Read intenta reconectar, el
	// intento falla, y en la espera se cierra.
	_, err = io.ReadAll(lc)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("tras cerrar durante la espera, Read debía dar io.ErrClosedPipe; dio %v", err)
	}
	if n := atomic.LoadInt32(&peticiones); n != 2 {
		t.Fatalf("peticiones al equipo: %d; se esperaban 2 (la buena y un solo reintento antes de cerrar)", n)
	}
	if r.Estado().Conectado {
		t.Fatal("tras cerrar, Conectado no puede quedar en true")
	}
}
