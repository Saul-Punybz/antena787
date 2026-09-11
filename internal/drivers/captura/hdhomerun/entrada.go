package hdhomerun

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Señal es el nivel de recepción del sintonizador, en las mismas unidades
// que documenta la Guía de Desarrollo de SiliconDust para
// "hdhomerun_config <id> get /tuner<n>/status": tres números de 0 a 100.
// Ninguno es un dBm ni un LUFS, son el porcentaje de fábrica del equipo.
type Señal struct {
	// Bloqueo es la modulación que el sintonizador detectó de verdad
	// ("qam256", "8vsb"…); vacío si no hay canal sintonizado.
	Bloqueo string
	// FuerzaPct es "ss": la fuerza de la señal, 0-100.
	FuerzaPct int
	// RuidoPct es "snq": la calidad de la relación señal/ruido, 0-100.
	RuidoPct int
	// SimboloPct es "seq": la calidad de símbolo —errores digitales
	// corregibles—, 0-100. 100 es "sin errores".
	SimboloPct int
}

// Estado es lo que la pantalla necesita de esta entrada, en el mismo
// vocabulario que model.Output (estado_conexion, reintentos, ultimo_error;
// F2-48). Se lee con Retorno.Estado mientras Abrir mantiene la conexión sola.
type Estado struct {
	Conectado   bool
	Reintentos  int
	UltimoError string
	Señal       *Señal
}

// Retorno es una entrada de retorno de aire por HDHomeRun: guarda el estado
// que la pantalla lee mientras Abrir reconecta sola y Medir mide.
type Retorno struct {
	Cliente *Cliente
	// HostAPI es dónde vive discover.json/lineup.json/status.json de este
	// dispositivo —normalmente Dispositivo.BaseURL—. Medir lo usa para pedir
	// la señal; vacío, intenta con el propio host de la URL del canal (que
	// en la mayoría de los equipos es un puerto de streaming distinto al de
	// la API, así que puede no encontrar nada — no es un error, ver Medir).
	HostAPI string

	mu     sync.Mutex
	estado Estado
}

// NuevoRetorno crea un Retorno listo para Abrir o Medir. cliente nil usa uno
// de fábrica (http.DefaultClient, espera progresiva 1→60 s).
func NuevoRetorno(cliente *Cliente) *Retorno {
	if cliente == nil {
		cliente = &Cliente{}
	}
	return &Retorno{Cliente: cliente}
}

// Estado devuelve una copia del estado actual: segura de leer desde otra
// goroutine mientras Abrir sigue corriendo.
func (r *Retorno) Estado() Estado {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.estado
}

func (r *Retorno) marcar(f func(*Estado)) {
	r.mu.Lock()
	f(&r.estado)
	r.mu.Unlock()
}

// Abrir entrega el TS crudo de una URL de canal —el campo URL de Lineup,
// con forma http://<ip>:5004/auto/v6.1— y reconecta sola si la conexión se
// cae: espera progresiva 1, 2, 4… hasta el tope de Cliente.EsperaMaxima
// (F2-48), sin que quien lee el io.ReadCloser tenga que hacer nada.
//
// El primer intento sí se informa como error: quien pide Abrir quiere saber
// si la URL ni siquiera contesta la primera vez, antes de comprometerse a
// leer de ella. Una vez abierto, Read no devuelve el error de una caída a
// mitad de stream — reconecta y sigue —, salvo que ctx se cancele o se
// llame a Close.
func (r *Retorno) Abrir(ctx context.Context, url string) (io.ReadCloser, error) {
	l := &lector{r: r, c: r.Cliente, ctx: ctx, url: url, espera: r.Cliente.esperaInicial()}
	if err := l.conectar(); err != nil {
		return nil, err
	}
	return l, nil
}

// lector es el io.ReadCloser que Abrir entrega.
type lector struct {
	r      *Retorno
	c      *Cliente
	ctx    context.Context
	url    string
	espera time.Duration

	mu      sync.Mutex
	resp    *http.Response
	cerrado bool
}

// conectar hace un único intento de conexión y actualiza el Estado del
// Retorno dueño según el resultado.
func (l *lector) conectar() error {
	req, err := http.NewRequestWithContext(l.ctx, http.MethodGet, l.url, nil)
	if err != nil {
		return err
	}
	resp, err := l.c.http().Do(req)
	if err != nil {
		l.r.marcar(func(e *Estado) { e.Conectado = false; e.UltimoError = err.Error() })
		return err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		err := fmt.Errorf("%s respondió %s", l.url, resp.Status)
		l.r.marcar(func(e *Estado) { e.Conectado = false; e.UltimoError = err.Error() })
		return err
	}

	l.mu.Lock()
	l.resp = resp
	l.mu.Unlock()
	l.espera = l.c.esperaInicial()
	l.r.marcar(func(e *Estado) { e.Conectado = true; e.UltimoError = "" })
	return nil
}

// reconectarConEspera reintenta conectar() con espera progresiva hasta que
// alguno de los dos gane: se conecta, o el contexto se cancela / lo cierran.
func (l *lector) reconectarConEspera() error {
	l.r.marcar(func(e *Estado) { e.Conectado = false })
	for {
		if err := l.ctx.Err(); err != nil {
			return err
		}
		l.mu.Lock()
		cerrado := l.cerrado
		l.mu.Unlock()
		if cerrado {
			return io.ErrClosedPipe
		}

		// Reintentos cuenta cada intento de reconexión, gane o pierda: es
		// lo que la pantalla enseña como "van N reintentos", y por eso sube
		// también en el intento que por fin funciona.
		l.r.marcar(func(e *Estado) { e.Reintentos++ })
		if err := l.conectar(); err == nil {
			return nil
		}

		if err := l.c.dormir(l.ctx, l.espera); err != nil {
			return err
		}
		l.espera *= 2
		if max := l.c.esperaMaxima(); l.espera > max {
			l.espera = max
		}
	}
}

func (l *lector) Read(p []byte) (int, error) {
	for {
		l.mu.Lock()
		if l.cerrado {
			l.mu.Unlock()
			return 0, io.ErrClosedPipe
		}
		resp := l.resp
		l.mu.Unlock()

		if resp != nil {
			n, err := resp.Body.Read(p)
			if err == nil || (err == io.EOF && n > 0) {
				return n, err
			}
			resp.Body.Close()
			l.mu.Lock()
			if l.resp == resp {
				l.resp = nil
			}
			l.mu.Unlock()
			if err == nil {
				// EOF sin datos: probar otra vez antes de contarlo como una
				// caída de verdad.
				continue
			}
		}

		if err := l.reconectarConEspera(); err != nil {
			return 0, err
		}
	}
}

func (l *lector) Close() error {
	l.mu.Lock()
	l.cerrado = true
	resp := l.resp
	l.resp = nil
	l.mu.Unlock()
	if resp != nil {
		return resp.Body.Close()
	}
	return nil
}
