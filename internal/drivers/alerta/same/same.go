// Paquete same oye el protocolo SAME —Specific Area Message Encoding, el que
// define 47 CFR 11.31 para el sistema de alertas de emergencia de Estados
// Unidos— en el audio del **retorno de aire**.
//
// **Solo decodifica, y nunca va a hacer otra cosa.** Antena787 no emite
// alertas, no genera cabeceras SAME y no sintetiza la señal de atención: eso
// es trabajo del ENDEC certificado que está aguas abajo, y el software no lo
// reemplaza (ADR 0010,
// docs/adr/0010-eas-integrate-the-endec-never-replace-it.md). Este paquete es
// la **capa 2** de ese ADR: oír en el retorno que la alerta salió de verdad al
// aire, con hora, y dejar esa línea en el as-run. Un ENDEC que disparó sin que
// el aire cambiara es un incidente, y esta es la única manera de saberlo sin
// creerle a nuestro propio motor (ADR 0009: la verdad es la señal transmitida).
//
// Por eso aquí no hay modulador. El de las pruebas vive en modulador_test.go y
// no entra en el binario: el binario no puede emitir SAME ni por accidente.
//
// # La matemática, en palabras
//
// La cabecera SAME es audio: dos tonos que se alternan, 2083.3 Hz para el uno
// («marca») y 1562.5 Hz para el cero («espacio»), a 520.83 bits por segundo.
// Los tres números no son casualidad: la marca son exactamente cuatro ciclos
// por bit y el espacio exactamente tres, así que dentro de un bit cada tono
// entra un número entero de veces y uno no se confunde con el otro.
//
// Decodificar es entonces preguntar, para cada bit, «¿cuánto hay de 2083.3 y
// cuánto de 1562.5?» y quedarse con el que gane. Eso se hace con dos
// correladores en cuadratura (demodulación no coherente, o sea: no hace falta
// saber en qué fase venía el tono, solo cuánta energía hay a esa frecuencia):
// se multiplica el audio por un seno y un coseno de la frecuencia buscada y se
// suma el resultado sobre una ventana de un bit. El cuadrado de esas dos sumas
// es la energía del tono. Si la de marca supera a la de espacio, el bit es 1.
//
// La suma sobre la ventana se mantiene al día sumando la muestra que entra y
// restando la que sale (una sola resta y una suma por muestra, sin recorrer la
// ventana), y el seno y el coseno salen de una tabla de 4096 entradas indexada
// por un acumulador de fase entero, así que el camino caliente no llama a
// math.Cos ni asigna memoria: es para correr 24/7 en el retorno de aire.
//
// Falta saber *dónde empieza cada bit*. Para eso está el preámbulo: dieciséis
// bytes 0xAB —que en el aire es un patrón casi alternado— antes de cada
// cabecera. La diferencia entre las dos energías cruza el cero justo a mitad
// de camino entre dos bits, así que cada cruce dice cuánto se ha corrido el
// reloj, y el lazo de reloj corrige la fase (y, más despacio, el largo del
// bit, que es lo que salva un audio grabado a 47.9 kHz y leído como si fuera
// 48). Con el reloj enganchado, los bits entran en un registro de 16 y cuando
// se ven dos 0xAB seguidos ya se sabe además dónde empieza cada byte.
//
// De ahí en adelante es texto: ASCII de siete bits con un octavo bit nulo,
// enviado con el bit menos significativo primero. La cabecera
// «ZCZC-ORG-EEE-PSSCCC+TTTT-JJJHHMM-LLLLLLLL-» se manda tres veces con un
// segundo de silencio entre una y otra, y el fin de mensaje es «NNNN» tres
// veces. Las tres repeticiones son el seguro: si una llega limpia basta, y si
// difieren se vota carácter por carácter, que es lo que recupera una cabecera
// de tres copias sucias. Cuántas coincidieron sale en Confianza.
//
// # Lo que cuesta
//
// Medido con banco_test.go en un Mac mini M4 el 11 de septiembre de 2026: **0.5
// ms por segundo de audio** a 48 kHz, o 0.8 ms con el detector de la señal de
// atención encendido. Son cinco diezmilésimas de un núcleo, y **cero
// asignaciones de memoria por segundo de audio**: lo único que se asigna es la
// cabecera que se emite, una por alerta. Eso es lo que hace que se pueda dejar
// puesto en el retorno de aire y olvidarse.
package same

import (
	"errors"
	"sync/atomic"
	"time"
)

// Tipos de evento que salen por el canal.
const (
	// TipoCabecera es una cabecera SAME completa y válida: el comienzo de una
	// alerta oída en el aire.
	TipoCabecera = "cabecera"
	// TipoFin es el «NNNN» que cierra el mensaje.
	TipoFin = "fin"
	// TipoAtencion es la señal de atención (853 + 960 Hz a la vez). Es
	// opcional y no lleva datos: sirve para poner hora al pitido.
	TipoAtencion = "atencion"
)

// repeticionesEsperadas es cuántas veces manda el protocolo cada bloque: tres
// cabeceras y tres fines de mensaje (47 CFR 11.31). Es el denominador de
// Confianza.
const repeticionesEsperadas = 3

// Cabecera es una alerta oída en el retorno de aire. Los campos son los del
// protocolo; Crudo es la cadena tal como se leyó, que es la que vale como
// evidencia, y Confianza dice con cuántas de las tres repeticiones se armó.
type Cabecera struct {
	// Org es quién originó: EAS (una emisora), CIV (autoridad civil), WXR
	// (servicio meteorológico) o PEP (gobierno federal).
	Org string
	// Evento es el código de tres letras: RWT es la prueba semanal, TOR un
	// aviso de tornado, y así unos sesenta.
	Evento string
	// Zonas son los códigos PSSCCC de área (FIPS): P es la parte del condado,
	// SS el estado —72 es Puerto Rico— y CCC el condado o municipio, con 000
	// para «todo el estado». Van de una a treinta y una.
	Zonas []string
	// Duracion es cuánto vale la alerta, del campo TTTT.
	Duracion time.Duration
	// Instante es el campo JJJHHMM tal cual: día juliano del año y hora UTC en
	// que el originador soltó el mensaje. Se guarda crudo a propósito, porque
	// no trae año y convertirlo a fecha es una interpretación.
	Instante string
	// Llamada es el identificador del participante, campo LLLLLLLL, sin los
	// espacios de relleno del final.
	Llamada string
	// Crudo es la cabecera completa como se leyó del aire, desde ZCZC hasta el
	// guion final.
	Crudo string
	// Detectado es la hora de pared en que se oyó la primera de las tres
	// repeticiones. Es el dato que va al as-run.
	Detectado time.Time
	// Desplazamiento es ese mismo instante contado en audio desde la primera
	// muestra que se escribió. No depende del reloj de la máquina, así que es
	// el que sirve para ubicar la alerta dentro de una grabación.
	Desplazamiento time.Duration
	// Repeticiones es cuántas copias de la cabecera se oyeron en la ráfaga
	// (hasta tres).
	Repeticiones int
	// Confianza va de 0 a 1: cuántas de las tres repeticiones coincidieron con
	// lo que se emitió. 1 son las tres iguales; 2/3 es mayoría; 1/3 es una
	// sola copia limpia, o una cabecera reconstruida votando carácter por
	// carácter.
	Confianza float64
}

// FinDeMensaje es el «NNNN» que cierra la alerta: con él se sabe cuánto duró
// la interrupción de verdad, no cuánto decía que iba a durar.
type FinDeMensaje struct {
	// Crudo es siempre "NNNN"; se guarda para que la línea del as-run sea
	// literal.
	Crudo string
	// Detectado es la hora de pared del primer NNNN de la ráfaga.
	Detectado time.Time
	// Desplazamiento es ese instante contado en audio desde la primera muestra.
	Desplazamiento time.Duration
	// Repeticiones es cuántos NNNN se oyeron (hasta tres).
	Repeticiones int
}

// SenalDeAtencion es el pitido de dos tonos que va después de la cabecera (853
// y 960 Hz a la vez, entre 8 y 25 segundos según la norma). No dice nada de la
// alerta; sirve para poner hora al pitido y para confirmar que lo que salió al
// aire fue el mensaje completo y no solo la cabecera.
type SenalDeAtencion struct {
	Duracion       time.Duration
	Detectado      time.Time
	Desplazamiento time.Duration
}

// Evento es lo que sale por el canal. Tipo dice cuál de los tres punteros está
// lleno.
type Evento struct {
	Tipo     string
	Cabecera *Cabecera
	Fin      *FinDeMensaje
	Atencion *SenalDeAtencion
}

// Opciones configura el decodificador. Todo tiene valor de fábrica: Opciones{}
// es un decodificador a 48 kHz, sin detección de la señal de atención.
type Opciones struct {
	// Tasa es la tasa de muestreo del PCM en muestras por segundo. 48000 por
	// defecto, que es lo que sale del motor y lo que entrega el retorno de
	// aire. Tiene que ser al menos 8000: por debajo de eso los 2083.3 Hz de la
	// marca ya no caben con holgura.
	Tasa int
	// VentanaDeVoto es cuánto se espera, en tiempo de audio, a que lleguen las
	// otras dos repeticiones antes de resolver con las que haya. Cinco
	// segundos por defecto: tres cabeceras de una zona son unos 3.2 s con sus
	// silencios, y una de treinta y una zonas llega a 15 s.
	VentanaDeVoto time.Duration
	// DetectarAtencion enciende el detector de la señal de atención. Cuesta
	// tres correladores más por muestra, así que viene apagado.
	DetectarAtencion bool
	// AtencionMinima es cuánto tiene que sonar el par de tonos para contarlo.
	// Medio segundo por defecto; la norma pide de 8 a 25 s, pero un detector
	// que exige ocho segundos se pierde una señal recortada, que es
	// justamente lo que hay que poder reportar.
	AtencionMinima time.Duration
	// Reloj es de dónde sale la hora de pared. time.Now por defecto; las
	// pruebas lo cambian.
	Reloj func() time.Time
	// Buffer es la capacidad del canal de eventos. 16 por defecto.
	Buffer int
}

// ErrTasa es lo que devuelve Nuevo cuando la tasa de muestreo no alcanza.
var ErrTasa = errors.New("same: la tasa de muestreo tiene que ser de al menos 8000 muestras por segundo")

// ErrCerrado es lo que devuelve Escribir después de Cerrar.
var ErrCerrado = errors.New("same: el decodificador ya está cerrado")

// tasaMinima es la tasa por debajo de la cual no se acepta audio: con 8000
// muestras por segundo la marca de 2083.3 Hz todavía tiene casi cuatro
// muestras por ciclo y la ventana de un bit son 15 muestras.
const tasaMinima = 8000

// Decodificador lee PCM s16le mono y va soltando eventos por un canal.
//
// Lo escribe una sola goroutine (la que trae el audio) y lo lee otra (la que
// escribe el as-run). Como en el detector de negro y silencio, **el aire nunca
// espera al que mira**: si nadie está leyendo el canal, el evento se tira y se
// cuenta en Descartados. Un retorno de aire no se puede frenar porque una
// goroutine esté dormida.
type Decodificador struct {
	tasa    int
	reloj   func() time.Time
	eventos chan Evento

	demod *demodulador
	aten  *detectorAtencion

	muestras uint64
	ventana  uint64

	// Voto de las tres cabeceras.
	candidatos []candidato
	limiteCab  uint64
	detCab     time.Time
	desplCab   time.Duration

	// Voto de los tres NNNN.
	nFines    int
	limiteFin uint64
	detFin    time.Time
	desplFin  time.Duration

	// PCM s16le puede llegar partido a la mitad de una muestra.
	resto      byte
	tieneResto bool

	descartados atomic.Uint64
	cerrado     bool
}

// candidato es una cabecera oída, válida o no. Las inválidas también se
// guardan: con tres copias sucias el voto por carácter arma la buena.
type candidato struct {
	crudo  string
	valido bool
}

// Nuevo arma un decodificador.
func Nuevo(o Opciones) (*Decodificador, error) {
	if o.Tasa == 0 {
		o.Tasa = 48000
	}
	if o.Tasa < tasaMinima {
		return nil, ErrTasa
	}
	if o.VentanaDeVoto <= 0 {
		o.VentanaDeVoto = 5 * time.Second
	}
	if o.AtencionMinima <= 0 {
		o.AtencionMinima = 500 * time.Millisecond
	}
	if o.Reloj == nil {
		o.Reloj = time.Now
	}
	if o.Buffer <= 0 {
		o.Buffer = 16
	}
	d := &Decodificador{
		tasa:       o.Tasa,
		reloj:      o.Reloj,
		eventos:    make(chan Evento, o.Buffer),
		ventana:    uint64(float64(o.Tasa) * o.VentanaDeVoto.Seconds()),
		candidatos: make([]candidato, 0, repeticionesEsperadas),
	}
	d.demod = nuevoDemodulador(o.Tasa)
	d.demod.alCandidato = d.agregarCandidato
	d.demod.alFin = d.agregarFin
	if o.DetectarAtencion {
		d.aten = nuevoDetectorAtencion(o.Tasa, o.AtencionMinima)
		d.aten.alTerminar = d.emitirAtencion
	}
	return d, nil
}

// Eventos es por donde salen las cabeceras, los fines de mensaje y —si se
// pidió— las señales de atención. Se cierra en Cerrar.
func (d *Decodificador) Eventos() <-chan Evento { return d.eventos }

// Descartados es cuántos eventos se tiraron porque nadie estaba leyendo el
// canal. Si esto no es cero, el as-run tiene huecos y hay que revisar quién
// consume.
func (d *Decodificador) Descartados() uint64 { return d.descartados.Load() }

// MuestrasLeidas es cuántas muestras han entrado. Dividido por la tasa es el
// tiempo de audio visto, que es el reloj con el que se miden los plazos.
func (d *Decodificador) MuestrasLeidas() uint64 { return d.muestras }

// Escribir mete PCM s16le mono. Acepta bloques de cualquier tamaño, incluso
// impares: el byte que sobra se guarda para el próximo. No asigna memoria.
func (d *Decodificador) Escribir(p []byte) (int, error) {
	if d.cerrado {
		return 0, ErrCerrado
	}
	total := len(p)
	i := 0
	if d.tieneResto && len(p) > 0 {
		d.muestra(float64(int16(uint16(d.resto)|uint16(p[0])<<8)) / 32768)
		d.tieneResto = false
		i = 1
	}
	for ; i+1 < len(p); i += 2 {
		d.muestra(float64(int16(uint16(p[i])|uint16(p[i+1])<<8)) / 32768)
	}
	if i < len(p) {
		d.resto = p[i]
		d.tieneResto = true
	}
	d.vencimientos()
	return total, nil
}

// Write es Escribir con el nombre que pide io.Writer, para poder enchufar el
// decodificador donde ya hay una tubería de audio.
func (d *Decodificador) Write(p []byte) (int, error) { return d.Escribir(p) }

// Cerrar resuelve lo que esté a medio votar y cierra el canal. Después de esto
// Escribir devuelve ErrCerrado.
func (d *Decodificador) Cerrar() error {
	if d.cerrado {
		return nil
	}
	d.cerrado = true
	if d.aten != nil {
		d.aten.cerrar(d.muestras)
	}
	d.resolverCabeceras()
	d.resolverFines()
	close(d.eventos)
	return nil
}

// muestra es el camino caliente: una muestra, normalizada a ±1. Cada 1024
// muestras —unos 21 ms a 48 kHz— se mira si venció algún plazo de voto; así el
// plazo no depende del tamaño del bloque que trajo el audio.
func (d *Decodificador) muestra(x float64) {
	d.muestras++
	d.demod.muestra(x)
	if d.aten != nil {
		d.aten.muestra(x, d.muestras)
	}
	if d.muestras&1023 == 0 {
		d.vencimientos()
	}
}

func (d *Decodificador) vencimientos() {
	if len(d.candidatos) > 0 && d.muestras >= d.limiteCab {
		d.resolverCabeceras()
	}
	if d.nFines > 0 && d.muestras >= d.limiteFin {
		d.resolverFines()
	}
}

// desplazamiento convierte el contador de muestras en tiempo de audio.
func (d *Decodificador) desplazamiento() time.Duration {
	return time.Duration(float64(d.muestras) / float64(d.tasa) * float64(time.Second))
}

// agregarCandidato la llama el demodulador cada vez que termina de leer algo
// que parece una cabecera. No emite: junta, y quien decide es el voto.
func (d *Decodificador) agregarCandidato(crudo []byte, valido bool) {
	if len(d.candidatos) == 0 {
		d.limiteCab = d.muestras + d.ventana
		d.detCab = d.reloj()
		d.desplCab = d.desplazamiento()
	}
	d.candidatos = append(d.candidatos, candidato{crudo: string(crudo), valido: valido})
	if len(d.candidatos) >= repeticionesEsperadas {
		d.resolverCabeceras()
	}
}

// resolverCabeceras vota entre las repeticiones que llegaron y emite una sola
// cabecera. Si el voto no da nada que se pueda leer, se cae de vuelta a la
// primera repetición que sí era válida; si no hubo ninguna, no se emite nada:
// una alerta inventada en el as-run es peor que un hueco.
func (d *Decodificador) resolverCabeceras() {
	if len(d.candidatos) == 0 {
		return
	}
	n := len(d.candidatos)
	crudo, coincidencias := votar(d.candidatos)
	cab, ok := analizarCabecera([]byte(crudo))
	if !ok {
		for _, c := range d.candidatos {
			if !c.valido {
				continue
			}
			if v, ok2 := analizarCabecera([]byte(c.crudo)); ok2 {
				cab, crudo, coincidencias, ok = v, c.crudo, 1, true
				break
			}
		}
	}
	d.candidatos = d.candidatos[:0]
	if !ok {
		return
	}
	if coincidencias < 1 {
		coincidencias = 1
	}
	cab.Crudo = crudo
	cab.Repeticiones = n
	cab.Confianza = float64(coincidencias) / float64(repeticionesEsperadas)
	cab.Detectado = d.detCab
	cab.Desplazamiento = d.desplCab
	d.emitir(Evento{Tipo: TipoCabecera, Cabecera: &cab})
}

// agregarFin la llama el demodulador con cada NNNN.
func (d *Decodificador) agregarFin() {
	if d.nFines == 0 {
		d.limiteFin = d.muestras + d.ventana
		d.detFin = d.reloj()
		d.desplFin = d.desplazamiento()
	}
	d.nFines++
	if d.nFines >= repeticionesEsperadas {
		d.resolverFines()
	}
}

func (d *Decodificador) resolverFines() {
	if d.nFines == 0 {
		return
	}
	fin := &FinDeMensaje{
		Crudo:          "NNNN",
		Detectado:      d.detFin,
		Desplazamiento: d.desplFin,
		Repeticiones:   d.nFines,
	}
	d.nFines = 0
	d.emitir(Evento{Tipo: TipoFin, Fin: fin})
}

func (d *Decodificador) emitirAtencion(inicio, fin uint64) {
	seg := func(m uint64) time.Duration {
		return time.Duration(float64(m) / float64(d.tasa) * float64(time.Second))
	}
	a := &SenalDeAtencion{
		Duracion:       seg(fin - inicio),
		Detectado:      d.reloj(),
		Desplazamiento: seg(inicio),
	}
	d.emitir(Evento{Tipo: TipoAtencion, Atencion: a})
}

func (d *Decodificador) emitir(ev Evento) {
	select {
	case d.eventos <- ev:
	default:
		d.descartados.Add(1)
	}
}
