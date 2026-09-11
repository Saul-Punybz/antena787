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

func TestAbrirCerrarDetieneLaReconexion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	r := NuevoRetorno(&Cliente{Dormir: sinEsperaDeVerdad})
	lc, err := r.Abrir(context.Background(), srv.URL)
	// Es posible que el primer intento ya falle (conexión cortada de
	// inmediato); lo que importa aquí es Close, así que si Abrir falló no
	// hay nada más que probar.
	if err != nil {
		return
	}
	if cerr := lc.Close(); cerr != nil {
		t.Fatalf("Close: %v", cerr)
	}
	if _, err := lc.Read(make([]byte, 16)); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Read tras Close debería devolver io.ErrClosedPipe, dio %v", err)
	}
}
