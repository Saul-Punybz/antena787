package scte104

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// inyectorDePrueba es un inyector de SCTE-104 de verdad, en lo que importa:
// escucha TCP, lee marcos con el mismo encuadre del estándar, contesta el
// init_request con un init_response, el alive_request con un alive_response, y
// cada multiple_operation_message con su inject_response llevando el
// message_number que acusa. Con eso las pruebas recorren el diálogo entero, no
// solo la codificación.
//
// Los ganchos de abajo son para las pruebas que necesitan un inyector que se
// porta mal: uno que rechaza el inicio, uno que no contesta el latido, uno que
// cierra el socket. Son los tres modos en que un encoder de cabecera falla de
// verdad.
type inyectorDePrueba struct {
	ln net.Listener

	mu         sync.Mutex
	sencillos  []Sencillo
	multiples  []Multiple
	conexiones int
	numero     uint8

	// resultadoInicio es el `result` del init_response. ResultadoExito si es 0.
	resultadoInicio uint16
	// resultadoCorte es el `result` del inject_response. ResultadoExito si es 0.
	resultadoCorte uint16
	// calladoAlVivo hace que no conteste el alive_request: es un enlace zombi,
	// el socket abierto y nadie del otro lado.
	calladoAlVivo bool
	// cierraTrasInicio corta el socket en cuanto contesta el init_response.
	cierraTrasInicio bool
	// tambienCompleta manda además el inject_complete_response, como hace un
	// inyector que confirma la inyección y no solo la recepción.
	tambienCompleta bool
}

func nuevoInyector(t *testing.T) *inyectorDePrueba {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("no se pudo escuchar: %v", err)
	}
	i := &inyectorDePrueba{ln: ln}
	go i.aceptar()
	t.Cleanup(func() { _ = ln.Close() })
	return i
}

func (i *inyectorDePrueba) direccion() string { return i.ln.Addr().String() }

func (i *inyectorDePrueba) aceptar() {
	for {
		conn, err := i.ln.Accept()
		if err != nil {
			return
		}
		i.mu.Lock()
		i.conexiones++
		i.mu.Unlock()
		go i.atender(conn)
	}
}

func (i *inyectorDePrueba) atender(conn net.Conn) {
	defer conn.Close()
	for {
		marco, err := LeerMarco(conn)
		if err != nil {
			return
		}
		if EsMultiple(marco) {
			m, err := DecodificarMultiple(marco)
			if err != nil {
				return
			}
			i.mu.Lock()
			i.multiples = append(i.multiples, m)
			res := i.resultadoCorte
			completa := i.tambienCompleta
			i.numero++
			n := i.numero
			i.mu.Unlock()
			if res == 0 {
				res = ResultadoExito
			}
			if !i.escribir(conn, InyectaRespuesta(n, m.IndicePID, res, m.Numero)) {
				return
			}
			if completa {
				i.mu.Lock()
				i.numero++
				n = i.numero
				i.mu.Unlock()
				if !i.escribir(conn, InyectaCompleta(n, m.IndicePID, res, m.Numero, uint8(len(m.Operaciones)))) {
					return
				}
			}
			continue
		}

		s, err := DecodificarSencillo(marco)
		if err != nil {
			return
		}
		i.mu.Lock()
		i.sencillos = append(i.sencillos, s)
		resInicio := i.resultadoInicio
		callado := i.calladoAlVivo
		cierra := i.cierraTrasInicio
		i.mu.Unlock()

		switch s.OpID {
		case OpInicioPeticion:
			if resInicio == 0 {
				resInicio = ResultadoExito
			}
			if !i.escribir(conn, InicioRespuesta(s.Numero, s.IndicePID, resInicio)) {
				return
			}
			if cierra {
				return
			}
		case OpVivoPeticion:
			if callado {
				continue
			}
			hora, err := s.Tiempo()
			if err != nil {
				return
			}
			// Un inyector real devuelve la hora que recibió.
			if !i.escribir(conn, VivoRespuesta(s.Numero, s.IndicePID, hora)) {
				return
			}
		}
	}
}

func (i *inyectorDePrueba) escribir(conn net.Conn, m Sencillo) bool {
	b, err := m.Codificar()
	if err != nil {
		return false
	}
	_, err = conn.Write(b)
	return err == nil
}

func (i *inyectorDePrueba) recibidosSencillos() []Sencillo {
	i.mu.Lock()
	defer i.mu.Unlock()
	return append([]Sencillo(nil), i.sencillos...)
}

func (i *inyectorDePrueba) recibidosMultiples() []Multiple {
	i.mu.Lock()
	defer i.mu.Unlock()
	return append([]Multiple(nil), i.multiples...)
}

func (i *inyectorDePrueba) cuantasConexiones() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.conexiones
}

// esperarA aguanta hasta que la condición se cumple o se acaba el plazo. Las
// pruebas de red no pueden afirmar en el instante siguiente: hay dos goroutines
// y un socket en medio.
func esperarA(t *testing.T, plazo time.Duration, motivo string, cond func() bool) {
	t.Helper()
	hasta := time.Now().Add(plazo)
	for time.Now().Before(hasta) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("se acabó el plazo de %s esperando: %s", plazo, motivo)
}

// clienteDePrueba arma un cliente con plazos cortos, para que las pruebas de
// red no tarden segundos.
func clienteDePrueba(t *testing.T, opc Opciones) *Cliente {
	t.Helper()
	if opc.Latido == 0 {
		opc.Latido = time.Hour // que no lata salvo que la prueba lo pida
	}
	if opc.PlazoRespuesta == 0 {
		opc.PlazoRespuesta = 500 * time.Millisecond
	}
	if opc.Espera == nil {
		// La espera progresiva de verdad se prueba aparte, en
		// TestEsperaProgresiva; aquí solo hace falta que no tarde.
		opc.Espera = func(int) time.Duration { return time.Millisecond }
	}
	c := NuevoCliente(opc)
	t.Cleanup(func() { _ = c.Cerrar() })
	return c
}

// TestDialogoCompleto es la prueba que cubre el enlace de punta a punta: se
// abre, se presenta, se piden los tres cortes que sabe pedir el cliente, y se
// comprueba en el lado del inyector que llegaron los bytes que tenían que
// llegar.
func TestDialogoCompleto(t *testing.T) {
	iny := nuevoInyector(t)
	iny.mu.Lock()
	iny.tambienCompleta = true
	iny.mu.Unlock()

	c := clienteDePrueba(t, Opciones{IndicePID: 0x0040, IndiceAS: 3, VersionSCTE35: 0})

	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := c.Conectar(ctx, iny.direccion()); err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}

	// Lo primero que sale de un sistema de automatización es el init_request:
	// sin eso el inyector no atiende cortes.
	recibidos := iny.recibidosSencillos()
	if len(recibidos) != 1 || recibidos[0].OpID != OpInicioPeticion {
		t.Fatalf("el primer mensaje tenía que ser un init_request, llegaron %+v", recibidos)
	}
	if recibidos[0].IndicePID != 0x0040 || recibidos[0].IndiceAS != 3 {
		t.Errorf("el init_request no llevó el DPI_PID_index ni el AS_index configurados: %+v", recibidos[0])
	}
	if recibidos[0].Resultado != ResultadoNoUsado || recibidos[0].ResultadoExtra != ResultadoNoUsado {
		t.Errorf("en una petición result y result_extension van en 0xFFFF: %+v", recibidos[0])
	}

	// Los mismos números del vector externo, pero pedidos por la API: 4 s de
	// pre-roll y 240 s de corte tienen que salir como 4000 y 2400 en el cable.
	const evento = uint32(0x60C65C03)
	if err := c.IniciarCorte(evento, 4*time.Second, 240*time.Second, true); err != nil {
		t.Fatalf("no se pudo iniciar el corte: %v", err)
	}
	if err := c.TerminarCorte(evento); err != nil {
		t.Fatalf("no se pudo terminar el corte: %v", err)
	}
	if err := c.Cancelar(evento + 1); err != nil {
		t.Fatalf("no se pudo cancelar: %v", err)
	}

	esperarA(t, 2*time.Second, "tres mensajes múltiples en el inyector", func() bool {
		return len(iny.recibidosMultiples()) == 3
	})
	ms := iny.recibidosMultiples()

	t.Run("el inicio del corte", func(t *testing.T) {
		corte, err := ms[0].Operaciones[0].Corte()
		if err != nil {
			t.Fatalf("no se leyó como splice_request_data: %v", err)
		}
		quiero := Corte{Tipo: CorteEmpiezaNormal, EventoID: evento, PreRoll: 4000, Duracion: 2400, AutoReturn: 1}
		if corte != quiero {
			t.Errorf("salió %+v, quiero %+v", corte, quiero)
		}
		if ms[0].Marca.Tipo != TiempoNinguno {
			t.Errorf("el corte con pre-roll va sin marca de tiempo, salió %+v", ms[0].Marca)
		}
		if ms[0].IndicePID != 0x0040 {
			t.Errorf("el DPI_PID_index salió 0x%04X", ms[0].IndicePID)
		}
	})

	t.Run("el fin del corte es inmediato", func(t *testing.T) {
		corte, err := ms[1].Operaciones[0].Corte()
		if err != nil {
			t.Fatalf("no se leyó como splice_request_data: %v", err)
		}
		quiero := Corte{Tipo: CorteTerminaYa, EventoID: evento}
		if corte != quiero {
			t.Errorf("salió %+v, quiero %+v", corte, quiero)
		}
	})

	t.Run("la cancelación va por splice_event_id", func(t *testing.T) {
		corte, err := ms[2].Operaciones[0].Corte()
		if err != nil {
			t.Fatalf("no se leyó como splice_request_data: %v", err)
		}
		quiero := Corte{Tipo: CorteCancela, EventoID: evento + 1}
		if corte != quiero {
			t.Errorf("salió %+v, quiero %+v", corte, quiero)
		}
	})

	t.Run("cada mensaje múltiple llevó su propio message_number", func(t *testing.T) {
		vistos := map[uint8]bool{}
		for _, m := range ms {
			if vistos[m.Numero] {
				t.Errorf("el message_number %d salió dos veces", m.Numero)
			}
			vistos[m.Numero] = true
		}
	})

	est := c.Estado()
	if !est.Conectado {
		t.Error("el enlace tenía que estar en pie")
	}
	if est.CortesPedidos != 1 {
		t.Errorf("CortesPedidos salió %d, quiero 1 (solo IniciarCorte lo suma)", est.CortesPedidos)
	}
	if est.Reintentos != 0 {
		t.Errorf("sin caídas, Reintentos tenía que ser 0, salió %d", est.Reintentos)
	}
	if est.UltimoError != "" {
		t.Errorf("sin caídas, UltimoError tenía que estar vacío, salió %q", est.UltimoError)
	}
	if est.DesdeCuando.IsZero() {
		t.Error("DesdeCuando tenía que traer la hora en que se abrió el enlace")
	}
	if est.Direccion != iny.direccion() {
		t.Errorf("la dirección salió %q, quiero %q", est.Direccion, iny.direccion())
	}
}

// TestTerminarCorteEnAnticipado comprueba el spliceEnd_normal, para cuando el
// fin del corte se sabe con antelación.
func TestTerminarCorteEnAnticipado(t *testing.T) {
	iny := nuevoInyector(t)
	c := clienteDePrueba(t, Opciones{})
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := c.Conectar(ctx, iny.direccion()); err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}
	if err := c.TerminarCorteEn(7, 2*time.Second); err != nil {
		t.Fatalf("no se pudo terminar el corte: %v", err)
	}
	esperarA(t, 2*time.Second, "el mensaje en el inyector", func() bool {
		return len(iny.recibidosMultiples()) == 1
	})
	corte, err := iny.recibidosMultiples()[0].Operaciones[0].Corte()
	if err != nil {
		t.Fatal(err)
	}
	quiero := Corte{Tipo: CorteTerminaNormal, EventoID: 7, PreRoll: 2000}
	if corte != quiero {
		t.Errorf("salió %+v, quiero %+v", corte, quiero)
	}
}

// TestCorteInmediatoSinPreRoll comprueba que un pre-roll de cero se manda como
// spliceStart_immediate y no como un normal con pre-roll cero, que es otra cosa.
func TestCorteInmediatoSinPreRoll(t *testing.T) {
	iny := nuevoInyector(t)
	c := clienteDePrueba(t, Opciones{})
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := c.Conectar(ctx, iny.direccion()); err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}
	if err := c.IniciarCorte(5, 0, 30*time.Second, false); err != nil {
		t.Fatalf("no se pudo iniciar el corte: %v", err)
	}
	esperarA(t, 2*time.Second, "el mensaje en el inyector", func() bool {
		return len(iny.recibidosMultiples()) == 1
	})
	corte, err := iny.recibidosMultiples()[0].Operaciones[0].Corte()
	if err != nil {
		t.Fatal(err)
	}
	quiero := Corte{Tipo: CorteEmpiezaYa, EventoID: 5, PreRoll: 0, Duracion: 300, AutoReturn: 0}
	if corte != quiero {
		t.Errorf("salió %+v, quiero %+v", corte, quiero)
	}
}

// TestLatidoPeriodico comprueba que el alive_request sale solo, con su hora, y
// que la respuesta del inyector queda anotada para la pantalla.
func TestLatidoPeriodico(t *testing.T) {
	iny := nuevoInyector(t)
	c := clienteDePrueba(t, Opciones{Latido: 15 * time.Millisecond})
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := c.Conectar(ctx, iny.direccion()); err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}

	contarVivos := func() int {
		n := 0
		for _, s := range iny.recibidosSencillos() {
			if s.OpID == OpVivoPeticion {
				n++
			}
		}
		return n
	}
	esperarA(t, 3*time.Second, "al menos tres alive_request", func() bool { return contarVivos() >= 3 })

	for _, s := range iny.recibidosSencillos() {
		if s.OpID != OpVivoPeticion {
			continue
		}
		hora, err := s.Tiempo()
		if err != nil {
			t.Fatalf("el alive_request no traía una hora legible: %v", err)
		}
		// La cuenta arranca el 6 de enero de 1980, así que hoy son más de mil
		// millones de segundos. Si saliera cero, el reloj o la época estarían
		// mal.
		if hora.Segundos < 1_000_000_000 {
			t.Errorf("la hora del alive salió %+v: la época de GPS no cuadra", hora)
		}
		if hora.Microsegundos > 999_999 {
			t.Errorf("los microsegundos salieron %d", hora.Microsegundos)
		}
	}

	esperarA(t, 2*time.Second, "el último latido anotado", func() bool {
		return !c.Estado().UltimoLatido.IsZero()
	})
	if est := c.Estado(); est.UltimoResultado != ResultadoExito {
		t.Errorf("el último resultado salió %d, quiero %d", est.UltimoResultado, ResultadoExito)
	}
}

// TestEsperaProgresiva comprueba el 1, 2, 4… con tope de 60 s de F2-48 como
// función pura, sin red y sin esperar de verdad.
func TestEsperaProgresiva(t *testing.T) {
	quiero := []time.Duration{
		1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second,
		16 * time.Second, 32 * time.Second, 60 * time.Second, 60 * time.Second,
		60 * time.Second,
	}
	for i, q := range quiero {
		if d := EsperaProgresiva(i + 1); d != q {
			t.Errorf("el intento %d esperó %s, quiero %s", i+1, d, q)
		}
	}
	// El séptimo intento sería 64 s por la duplicación; el tope lo baja a 60.
	if d := EsperaProgresiva(7); d != 60*time.Second {
		t.Errorf("el séptimo intento esperó %s, quiero 60s (el tope)", d)
	}
	// Nunca cero: el backoff deja de crecer, no de intentar.
	for _, i := range []int{-5, 0, 1, 100, 1000} {
		if d := EsperaProgresiva(i); d <= 0 || d > 60*time.Second {
			t.Errorf("el intento %d esperó %s: fuera del rango 1s-60s", i, d)
		}
	}
}

// TestReconectaSiElInyectorNoEstaTodavia es el caso de una cabecera de verdad:
// el encoder arranca más despacio que el playout. El cliente no se rinde, y
// cada intento queda contado con su error.
func TestReconectaSiElInyectorNoEstaTodavia(t *testing.T) {
	iny := nuevoInyector(t)
	var intentos atomic.Int64
	dialer := &net.Dialer{Timeout: time.Second}
	c := clienteDePrueba(t, Opciones{
		Dial: func(ctx context.Context, red, dir string) (net.Conn, error) {
			if intentos.Add(1) <= 3 {
				return nil, fmt.Errorf("el inyector todavía no arrancó (intento %d)", intentos.Load())
			}
			return dialer.DialContext(ctx, red, dir)
		},
	})

	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := c.Conectar(ctx, iny.direccion()); err != nil {
		t.Fatalf("tenía que acabar conectando: %v", err)
	}
	est := c.Estado()
	if est.Reintentos < 3 {
		t.Errorf("Reintentos salió %d, quiero al menos 3", est.Reintentos)
	}
	if !strings.Contains(est.UltimoError, "todavía no arrancó") {
		t.Errorf("UltimoError tenía que traer el motivo, salió %q", est.UltimoError)
	}
	if !est.Conectado {
		t.Error("el enlace tenía que quedar en pie")
	}
	// Y con el enlace en pie los cortes salen, que es lo único que importa.
	if err := c.IniciarCorte(1, 4*time.Second, 30*time.Second, true); err != nil {
		t.Errorf("el corte tenía que salir: %v", err)
	}
}

// TestReconectaCuandoElInyectorCierraElSocket comprueba que una caída a mitad
// del enlace se levanta sola, con un init_request nuevo: el inyector no se
// acuerda de nosotros después de un reinicio.
func TestReconectaCuandoElInyectorCierraElSocket(t *testing.T) {
	iny := nuevoInyector(t)
	iny.mu.Lock()
	iny.cierraTrasInicio = true
	iny.mu.Unlock()

	c := clienteDePrueba(t, Opciones{})
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := c.Conectar(ctx, iny.direccion()); err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}

	esperarA(t, 3*time.Second, "una segunda conexión con su init_request", func() bool {
		return iny.cuantasConexiones() >= 2 && c.Estado().Reintentos >= 1
	})

	inicios := 0
	for _, s := range iny.recibidosSencillos() {
		if s.OpID == OpInicioPeticion {
			inicios++
		}
	}
	if inicios < 2 {
		t.Errorf("llegaron %d init_request, quiero al menos 2 (uno por conexión)", inicios)
	}
	if est := c.Estado(); est.UltimoError == "" {
		t.Error("una caída tenía que dejar su motivo en UltimoError")
	}

	// Y cuando el inyector deja de portarse mal, el enlace se queda en pie.
	iny.mu.Lock()
	iny.cierraTrasInicio = false
	iny.mu.Unlock()
	esperarA(t, 3*time.Second, "el enlace otra vez en pie", func() bool {
		return c.Estado().Conectado
	})
}

// TestInyectorMudoCortaLaSesion es el enlace zombi: el socket sigue abierto y
// del otro lado ya no hay nadie. Solo el latido lo detecta.
func TestInyectorMudoCortaLaSesion(t *testing.T) {
	iny := nuevoInyector(t)
	iny.mu.Lock()
	iny.calladoAlVivo = true
	iny.mu.Unlock()

	c := clienteDePrueba(t, Opciones{
		Latido:         10 * time.Millisecond,
		PlazoRespuesta: 30 * time.Millisecond,
	})
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := c.Conectar(ctx, iny.direccion()); err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}

	esperarA(t, 3*time.Second, "que el latido sin respuesta tire la sesión", func() bool {
		return iny.cuantasConexiones() >= 2
	})
	if est := c.Estado(); !strings.Contains(est.UltimoError, "alive_request") {
		t.Errorf("UltimoError tenía que decir que el inyector no contestó el latido, salió %q", est.UltimoError)
	}
}

// TestInicioRechazadoNoDaElEnlacePorAbierto: un socket abierto al que el
// inyector contesta «acceso denegado» no es un enlace. Conectar no vuelve, y el
// motivo queda para la pantalla.
func TestInicioRechazadoNoDaElEnlacePorAbierto(t *testing.T) {
	iny := nuevoInyector(t)
	iny.mu.Lock()
	iny.resultadoInicio = ResultadoAccesoDenegado
	iny.mu.Unlock()

	// Una espera larguísima deja exactamente un intento antes de que el
	// contexto se acabe, así que el error que se comprueba es el del rechazo y
	// no el de un reintento posterior.
	c := clienteDePrueba(t, Opciones{Espera: func(int) time.Duration { return time.Hour }})
	ctx, cancelar := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancelar()
	err := c.Conectar(ctx, iny.direccion())
	if err == nil {
		t.Fatal("con el init rechazado, Conectar no tenía que darse por bueno")
	}
	if !strings.Contains(err.Error(), "acceso denegado") {
		t.Errorf("el error tenía que decir qué contestó el inyector, dijo: %v", err)
	}
	if est := c.Estado(); est.Conectado {
		t.Error("el enlace no estaba en pie")
	}
}

// TestElInyectorRechazaElCorte comprueba que el `result` del inyector llega al
// que pidió el corte, con su frase. Es el camino del result 122
// (ResultadoPreRollMuyChico), que es la razón por la que este paquete no valida
// el pre-roll por su cuenta.
func TestElInyectorRechazaElCorte(t *testing.T) {
	iny := nuevoInyector(t)
	iny.mu.Lock()
	iny.resultadoCorte = ResultadoPreRollMuyChico
	iny.mu.Unlock()

	c := clienteDePrueba(t, Opciones{})
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := c.Conectar(ctx, iny.direccion()); err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}
	err := c.IniciarCorte(1, 500*time.Millisecond, 30*time.Second, true)
	if err == nil {
		t.Fatal("el corte rechazado tenía que devolver error")
	}
	if !strings.Contains(err.Error(), "pre-roll") {
		t.Errorf("el error tenía que traer la frase del result 122, dijo: %v", err)
	}
	if !strings.Contains(err.Error(), "splice_request_data") {
		t.Errorf("el error tenía que decir qué operación se rechazó, dijo: %v", err)
	}
	if est := c.Estado(); est.CortesPedidos != 0 {
		t.Errorf("un corte rechazado no se cuenta como pedido, salió %d", est.CortesPedidos)
	}
}

// TestSinEnlaceElCorteFalla: no se encola nada. Un corte que no se puede
// señalar ahora no se puede señalar más tarde, porque más tarde ya está
// saliendo otra cosa.
func TestSinEnlaceElCorteFalla(t *testing.T) {
	c := NuevoCliente(Opciones{Direccion: "127.0.0.1:1"})
	t.Cleanup(func() { _ = c.Cerrar() })

	if err := c.IniciarCorte(1, 4*time.Second, 30*time.Second, true); !errors.Is(err, ErrSinConexion) {
		t.Errorf("quiero ErrSinConexion, salió %v", err)
	}
	if err := c.TerminarCorte(1); !errors.Is(err, ErrSinConexion) {
		t.Errorf("quiero ErrSinConexion, salió %v", err)
	}
	if err := c.Cancelar(1); !errors.Is(err, ErrSinConexion) {
		t.Errorf("quiero ErrSinConexion, salió %v", err)
	}
	if _, err := c.Enviar((SenalDeHora{PreRoll: 4000}).Operacion()); !errors.Is(err, ErrSinConexion) {
		t.Errorf("quiero ErrSinConexion, salió %v", err)
	}
}

// TestEnviarOtrasOperaciones comprueba la puerta de entrada de todo lo que no
// es un corte, y que varias operaciones caben en un solo mensaje: así se señala
// un bloque entero de una vez.
func TestEnviarOtrasOperaciones(t *testing.T) {
	iny := nuevoInyector(t)
	c := clienteDePrueba(t, Opciones{})
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := c.Conectar(ctx, iny.direccion()); err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}

	dtmf, err := (DTMF{PreRoll: 40, Digitos: "*123#"}).Operacion()
	if err != nil {
		t.Fatal(err)
	}
	marca := Marca{Tipo: TiempoGPI, NumeroGPI: 2, FlancoGPI: FlancoCierra}
	if _, err := c.EnviarConMarca(marca, (SenalDeHora{PreRoll: 4000}).Operacion(), dtmf, CorteNulo()); err != nil {
		t.Fatalf("no se pudo enviar: %v", err)
	}

	esperarA(t, 2*time.Second, "el mensaje en el inyector", func() bool {
		return len(iny.recibidosMultiples()) == 1
	})
	m := iny.recibidosMultiples()[0]
	if m.Marca != marca {
		t.Errorf("la marca salió %+v, quiero %+v", m.Marca, marca)
	}
	if len(m.Operaciones) != 3 {
		t.Fatalf("llegaron %d operaciones, quiero 3", len(m.Operaciones))
	}
	if m.Operaciones[0].OpID != MopSenalDeHora || m.Operaciones[1].OpID != MopInsertaDTMF || m.Operaciones[2].OpID != MopCorteNulo {
		t.Errorf("las operaciones llegaron en otro orden: %+v", m.Operaciones)
	}
	vuelta, err := DecodificarDTMF(m.Operaciones[1].Datos)
	if err != nil || vuelta.Digitos != "*123#" {
		t.Errorf("el DTMF llegó como %+v, err %v", vuelta, err)
	}

	if _, err := c.Enviar(); err == nil {
		t.Error("un mensaje múltiple sin operaciones no dice nada: tenía que fallar")
	}
}

// TestCortesQueNoCaben comprueba que un descuido de unidades se ve antes de
// mandar nada, con un mensaje que dice cuál es el tope.
func TestCortesQueNoCaben(t *testing.T) {
	iny := nuevoInyector(t)
	c := clienteDePrueba(t, Opciones{})
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := c.Conectar(ctx, iny.direccion()); err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}

	// El pre_roll_time es de 16 bits en milisegundos: 65,5 s como mucho.
	if err := c.IniciarCorte(1, 70*time.Second, 30*time.Second, true); err == nil {
		t.Error("un pre-roll de 70 s no cabe en el pre_roll_time: tenía que fallar")
	} else if !strings.Contains(err.Error(), "pre_roll_time") {
		t.Errorf("el error tenía que nombrar el campo, dijo: %v", err)
	}
	// El break_duration es de 16 bits en décimas de segundo: 109 minutos.
	if err := c.IniciarCorte(1, 4*time.Second, 3*time.Hour, true); err == nil {
		t.Error("un corte de 3 h no cabe en el break_duration: tenía que fallar")
	} else if !strings.Contains(err.Error(), "break_duration") {
		t.Errorf("el error tenía que nombrar el campo, dijo: %v", err)
	}
	// Y un descuido que sí cabe pasa: 109 minutos justos.
	if err := c.IniciarCorte(1, 0, 6553*time.Second, false); err != nil {
		t.Errorf("6553 s sí caben: %v", err)
	}
	if len(iny.recibidosMultiples()) > 0 && len(iny.recibidosMultiples()) != 1 {
		t.Errorf("solo el corte que cabe tenía que salir, salieron %d", len(iny.recibidosMultiples()))
	}
}

// TestConPuerto comprueba que en Ajustes se pueda escribir solo la IP del
// encoder y que el puerto de referencia lo ponga el driver.
func TestConPuerto(t *testing.T) {
	casos := []struct{ entra, sale string }{
		{"192.168.1.50", "192.168.1.50:5167"},
		{"192.168.1.50:9000", "192.168.1.50:9000"},
		{"encoder.local", "encoder.local:5167"},
		{"  192.168.1.50  ", "192.168.1.50:5167"},
		{"[::1]:5167", "[::1]:5167"},
		{"", ""},
	}
	for _, c := range casos {
		if sale := conPuerto(c.entra); sale != c.sale {
			t.Errorf("%q salió %q, quiero %q", c.entra, sale, c.sale)
		}
	}
	if err := NuevoCliente(Opciones{}).Conectar(context.Background(), ""); err == nil {
		t.Error("sin dirección tenía que fallar")
	}
}

// TestCerrarEsIdempotente: cerrar dos veces no rompe nada, y cerrar sin haber
// conectado tampoco. Es lo que va a pasar cuando el canal se apague por
// cualquiera de los caminos que tiene un canal para apagarse.
func TestCerrarEsIdempotente(t *testing.T) {
	iny := nuevoInyector(t)
	c := clienteDePrueba(t, Opciones{})
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := c.Conectar(ctx, iny.direccion()); err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}
	if err := c.Cerrar(); err != nil {
		t.Errorf("el primer Cerrar devolvió %v", err)
	}
	if err := c.Cerrar(); err != nil {
		t.Errorf("el segundo Cerrar devolvió %v", err)
	}
	if est := c.Estado(); est.Conectado {
		t.Error("después de Cerrar el enlace no está en pie")
	}
	// Y ya no se vuelve a levantar solo.
	antes := iny.cuantasConexiones()
	time.Sleep(50 * time.Millisecond)
	if ahora := iny.cuantasConexiones(); ahora != antes {
		t.Errorf("después de Cerrar no se reconecta: había %d conexiones, ahora %d", antes, ahora)
	}

	sinConectar := NuevoCliente(Opciones{Direccion: "127.0.0.1:1"})
	if err := sinConectar.Cerrar(); err != nil {
		t.Errorf("cerrar sin conectar devolvió %v", err)
	}
}

// TestUnClienteEsDeUnSoloEnlace: llamar a Conectar dos veces es un error de
// programación, y se dice en vez de dejar dos supervisores compitiendo por el
// mismo socket.
func TestUnClienteEsDeUnSoloEnlace(t *testing.T) {
	iny := nuevoInyector(t)
	c := clienteDePrueba(t, Opciones{})
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := c.Conectar(ctx, iny.direccion()); err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}
	if err := c.Conectar(ctx, iny.direccion()); err == nil {
		t.Error("el segundo Conectar tenía que fallar")
	}
}

// TestAvisoDelEnlace comprueba el gancho para la bitácora: el paquete no
// escribe incidentes (no conoce la base), pero dice en palabras claras cuándo el
// enlace se abre y cuándo se cae.
func TestAvisoDelEnlace(t *testing.T) {
	iny := nuevoInyector(t)
	iny.mu.Lock()
	iny.cierraTrasInicio = true
	iny.mu.Unlock()

	var mu sync.Mutex
	var avisos []string
	c := clienteDePrueba(t, Opciones{
		Aviso: func(texto string) {
			mu.Lock()
			avisos = append(avisos, texto)
			mu.Unlock()
		},
	})
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := c.Conectar(ctx, iny.direccion()); err != nil {
		t.Fatalf("no se pudo conectar: %v", err)
	}

	tiene := func(trozo string) bool {
		mu.Lock()
		defer mu.Unlock()
		for _, a := range avisos {
			if strings.Contains(a, trozo) {
				return true
			}
		}
		return false
	}
	esperarA(t, 3*time.Second, "el aviso de enlace abierto", func() bool { return tiene("enlace abierto") })
	esperarA(t, 3*time.Second, "el aviso de enlace caído", func() bool { return tiene("se cayó el enlace") })
}

// TestAhoraGPS comprueba la cuenta del alive contra la época de GPS, que es la
// misma que usa SCTE 35 para su hora UTC.
func TestAhoraGPS(t *testing.T) {
	// Un instante conocido: 1 de enero de 2020, 00:00:00.500 UTC.
	momento := time.Date(2020, time.January, 1, 0, 0, 0, 500_000_000, time.UTC)
	c := NuevoCliente(Opciones{Reloj: func() time.Time { return momento }})
	salio := c.ahoraGPS()
	quieroSegundos := uint32(momento.Sub(EpocaGPS) / time.Second)
	if salio.Segundos != quieroSegundos {
		t.Errorf("los segundos salieron %d, quiero %d", salio.Segundos, quieroSegundos)
	}
	if salio.Microsegundos != 500_000 {
		t.Errorf("los microsegundos salieron %d, quiero 500000", salio.Microsegundos)
	}

	// Un reloj sin poner en hora (antes de 1980) no desborda: manda cero.
	viejo := NuevoCliente(Opciones{Reloj: func() time.Time { return time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC) }})
	if salio := viejo.ahoraGPS(); salio != (Tiempo{}) {
		t.Errorf("un reloj de 1970 tenía que dar cero, salió %+v", salio)
	}
}
