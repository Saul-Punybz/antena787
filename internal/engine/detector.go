// detector.go vigila **la salida de verdad**: no el plan, no los archivos, no
// el retorno de aire. Lo que mide es exactamente lo que se le entrega al
// encoder —los cuadros yuv420p y el PCM s16le que escribe `emit`—, porque es
// ahí donde se ve lo que el plan no sabe: un archivo que pasó el ingest
// perfecto y sale mudo por un desajuste de pista, o una fuente en vivo que
// sigue "conectada" y congelada en negro (PRD §9 paso 4, §14.1).
//
// Es un Sink que se pone en medio: el servidor de cuadros le escribe a él, y
// él le escribe al encoder. Cumple el mismo contrato (engine.Sink), así que
// ni frameserver.go ni el encoder se enteran de que existe. Lo único que hace
// de más es mirar, y mirar sale barato: de cada cuadro se leen unos mil
// puntos del plano de luma con un salto fijo —sin copiar nada— y de cada
// bloque de audio se suma el cuadrado de sus muestras.
//
// No decide nada. Cuenta lo que ve por un canal de estados —negro empieza,
// negro sigue, negro termina, y lo mismo con el silencio— y quien decide es
// internal/app/vigilancia.go, que es el que sabe cuál es el umbral del canal,
// si el archivo que está saliendo abre en negro a propósito, y si el aire
// está en manual. Es la tanda T3 de docs/f2/PLAN-F2.md.
//
// **El aire nunca espera al vigilante.** Si nadie lee el canal de estados, el
// estado se tira y se cuenta cuántos se tiraron: un cambio de clip no se
// retrasa ni un cuadro por una goroutine dormida.
package engine

import (
	"math"
	"sync/atomic"
	"time"
)

// Los tipos de cambio que el detector cuenta. `sigue` es el latido de un
// episodio abierto: quien vigila mide el umbral con estos, que están contados
// en cuadros y muestras que de verdad salieron, no en hora de pared.
const (
	NegroEmpieza    = "negro_empieza"
	NegroSigue      = "negro_sigue"
	NegroTermina    = "negro_termina"
	SilencioEmpieza = "silencio_empieza"
	SilencioSigue   = "silencio_sigue"
	SilencioTermina = "silencio_termina"
)

// Umbrales de fábrica: los mismos números que el ingest le pasa a ffmpeg
// (internal/ingest/blacksilence.go, `blackdetect=pic_th=0.98:pix_th=0.10` y
// `silencedetect=n=-60dB`) y los que manda el PRD §14.1, fila «Negro y
// silencio en el ingest». Que el aire y el ingest midan con los mismos
// números es la mitad de F2-54: una sola fuente de verdad para el umbral.
const (
	// PixelNegro es la luma por debajo de la cual un punto de la imagen
	// cuenta como negro.
	//
	// El PRD dice «luma bajo 16», y 16 es exactamente el negro digital de un
	// video en rango limitado: medido en la salida real del decodificador, un
	// clip negro da 16.000 clavados, así que «bajo 16» no se cumple nunca
	// (comprobado el 10 de septiembre de 2026 con un clip de `color=c=black`
	// pasando por videoFilter). Por eso ffmpeg pide 0.10 normalizado —25.5
	// sobre 255— en su blackdetect, y por eso el ingest ya le pasa
	// `pix_th=0.10` (internal/ingest/blacksilence.go). El aire mide con el
	// mismo número que el ingest: una sola fuente de verdad (F2-54).
	PixelNegro = 0.10 * 255
	// ParteNegra es qué parte de la imagen tiene que estar por debajo de
	// PixelNegro para que el cuadro sea negro: el mismo 98 % que el
	// `pic_th=0.98` del ingest. Se mira la parte, no la media, porque la
	// media de una escena nocturna con un punto de luz puede quedar bajo el
	// umbral sin que la imagen esté en negro.
	ParteNegra = 0.98
	// ParteHisteresis es cuánto tiene que destaparse la imagen para dar un
	// episodio de negro por terminado: hace falta bajar al 94 % para cerrar
	// lo que se abrió al 98 %. Sin histéresis, un cuadro que tiembla sobre el
	// umbral —el ruido de compresión de un fundido— abre y cierra episodios
	// diez veces por segundo y la bitácora se vuelve ilegible.
	ParteHisteresis = 0.04
	// LumaNegra es la luma media que el PRD nombra, y es la que se le enseña
	// a una persona en la alarma. No es la que decide: eso son PixelNegro y
	// ParteNegra, arriba.
	LumaNegra = 16.0
	// SilencioDBFS es el nivel por debajo del cual el bloque de audio es
	// silencio. Se mide en RMS del bloque, que es lo que oye una persona: el
	// pico de una sola muestra no distingue un chasquido de sonido.
	SilencioDBFS = -60.0
	// SilencioHisteresis son los decibelios que tiene que subir el audio para
	// cerrar un episodio de silencio, por la misma razón que ParteHisteresis.
	SilencioHisteresis = 3.0
	// SuelodBFS es lo más bajo que se reporta: el silencio digital es −∞ dBFS
	// y eso no se puede escribir en un número.
	SuelodBFS = -120.0
	// LatidoDetector es cada cuánto se cuenta que un episodio sigue abierto.
	// Un segundo alcanza para un umbral que se mide en decenas de segundos y
	// deja el canal de estados en dos mensajes por segundo en el peor caso.
	LatidoDetector = time.Second
	// MuestrasDeCuadro son los puntos del plano de luma que se miran en cada
	// cuadro. Mil puntos repartidos por toda la imagen distinguen el negro de
	// una escena oscura y cuestan un microsegundo; leer los 921,600 de un
	// 720p costaría un milisegundo, que es el 6 % de un cuadro a 59.94.
	MuestrasDeCuadro = 1024
	// EstadosEnCola es el colchón del canal de estados. A dos estados por
	// segundo, sesenta y cuatro son medio minuto de retraso del vigilante
	// antes de que se pierda uno.
	EstadosEnCola = 64
)

// Estado es un cambio de lo que sale al aire, tal como lo vio el detector.
type Estado struct {
	// Tipo es uno de los seis de arriba.
	Tipo string
	// Cuando es el instante del reloj del aire en que ocurrió: contado en
	// cuadros y muestras desde que el detector se puso en medio, nunca
	// time.Now (F2-90).
	Cuando time.Time
	// Duro es cuánto lleva el episodio abierto, en tiempo de salida. Cero en
	// `empieza`; el total en `termina`.
	Duro time.Duration
	// Nivel es lo medido: la luma media del cuadro (0 a 255) en los estados
	// de negro, y el nivel en dBFS del bloque de audio en los de silencio.
	Nivel float64
	// Parte es, en los estados de negro, qué parte de la imagen estaba por
	// debajo de PixelNegro: es el número que de verdad decide. En los de
	// silencio no quiere decir nada.
	Parte float64
}

// Detector es el Sink intermedio: mide y pasa. Se arma con NuevoDetector y
// se le pasa al servidor de cuadros en lugar del encoder.
type Detector struct {
	// Formato es el formato de casa: de ahí salen la geometría del cuadro y
	// la aritmética de muestras a tiempo.
	Formato Format
	// Destino es a dónde va lo que se midió: el encoder persistente.
	Destino Sink

	// Los umbrales, con los valores de fábrica de arriba si se dejan en cero.
	PixelNegro         float64
	ParteNegra         float64
	ParteHisteresis    float64
	SilencioDBFS       float64
	SilencioHisteresis float64
	// Latido es cada cuánto se cuenta que un episodio abierto sigue abierto.
	Latido time.Duration
	// Muestras son los puntos del cuadro que se miran; cero = MuestrasDeCuadro.
	Muestras int
	// Origen es la hora de pared que le toca al primer cuadro. Cero = ahora.
	Origen time.Time

	estados  chan Estado
	perdidos atomic.Int64

	// El reloj propio: cuadros escritos y muestras de audio escritas. Son
	// dos cuentas separadas a propósito —el video y el audio no llegan
	// pareados en el mismo tamaño— y ninguna es hora de pared.
	cuadros       int64
	muestras      int64
	paso          int // salto entre puntos mirados del plano de luma
	negro         episodio
	silencio      episodio
	cuadrosLatido int64
	audioLatido   int64
}

// episodio es un tramo abierto de negro o de silencio.
type episodio struct {
	abierto bool
	desde   int64 // el cuadro (o la muestra) en que empezó
	latidos int64 // cuántos latidos se han contado ya
}

// NuevoDetector pone el detector entre el servidor de cuadros y el encoder.
// destino es el encoder (o cualquier Sink: las pruebas le ponen un recolector
// en memoria). Los umbrales quedan en los de fábrica; quien quiera otros los
// cambia en los campos antes de escribir el primer cuadro.
func NuevoDetector(f Format, destino Sink) *Detector {
	d := &Detector{Formato: f, Destino: destino}
	d.prepara()
	return d
}

// prepara pone los valores de fábrica y calcula el salto de submuestreo.
func (d *Detector) prepara() {
	if d.PixelNegro == 0 {
		d.PixelNegro = PixelNegro
	}
	if d.ParteNegra == 0 {
		d.ParteNegra = ParteNegra
	}
	if d.ParteHisteresis == 0 {
		d.ParteHisteresis = ParteHisteresis
	}
	if d.SilencioDBFS == 0 {
		d.SilencioDBFS = SilencioDBFS
	}
	if d.SilencioHisteresis == 0 {
		d.SilencioHisteresis = SilencioHisteresis
	}
	if d.Latido <= 0 {
		d.Latido = LatidoDetector
	}
	if d.Muestras <= 0 {
		d.Muestras = MuestrasDeCuadro
	}
	if d.Origen.IsZero() {
		d.Origen = time.Now().Round(0).UTC()
	}
	if d.estados == nil {
		d.estados = make(chan Estado, EstadosEnCola)
	}
	// El salto tiene que barrer todas las columnas, no unas pocas. Si
	// compartiera un divisor con el ancho, los puntos mirados caerían siempre
	// en las mismas columnas y una sola columna blanca sobre negro pesaría
	// veinte veces lo que le toca (medido: con salto 56 sobre un ancho de 320,
	// una columna pesaba el 2.5 % en vez del 0.3 %). Con el salto primo
	// respecto al ancho, cada vuelta pasa por todas las columnas una vez.
	luma := d.Formato.Width * d.Formato.Height
	d.paso = luma / d.Muestras
	if d.paso < 1 {
		d.paso = 1
	}
	for d.Formato.Width > 1 && mcd(d.paso, d.Formato.Width) != 1 {
		d.paso++
	}
	d.cuadrosLatido = int64(math.Round(float64(d.Latido) / d.Formato.FrameDurationNs()))
	if d.cuadrosLatido < 1 {
		d.cuadrosLatido = 1
	}
	d.audioLatido = int64(d.Latido.Seconds() * float64(d.Formato.SampleRate))
	if d.audioLatido < 1 {
		d.audioLatido = 1
	}
}

// mcd es el máximo común divisor, que es lo que hace falta para elegir un
// salto que barra todas las columnas del cuadro.
func mcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

// Estados es el canal por el que salen los cambios. Nunca se cierra: el
// detector no sabe cuándo se apaga el aire, y quien vigila se va cuando se
// cancela su contexto.
func (d *Detector) Estados() <-chan Estado { return d.estados }

// Perdidos son los estados que se tiraron porque nadie los leía. Si crece,
// el vigilante está bloqueado en algo —una consulta a la base, por ejemplo—:
// el aire no se paró, pero la vigilancia se quedó ciega un rato.
func (d *Detector) Perdidos() int64 { return d.perdidos.Load() }

// Cuadros y MuestrasVistas son el reloj propio del detector, para las
// pruebas y para el tablero de T9.
func (d *Detector) Cuadros() int64        { return d.cuadros }
func (d *Detector) MuestrasVistas() int64 { return d.muestras }

// WriteFrame pasa el cuadro al encoder y lo mide. El orden importa: primero
// sale, después se mira. Un cuadro que no se pudo escribir no salió al aire y
// no tiene por qué contar como negro.
func (d *Detector) WriteFrame(fr []byte) error {
	if err := d.Destino.WriteFrame(fr); err != nil {
		return err
	}
	d.mirarCuadro(fr)
	d.cuadros++
	return nil
}

// WriteAudio pasa el bloque al encoder y lo mide, con la misma regla.
func (d *Detector) WriteAudio(pcm []byte) error {
	if err := d.Destino.WriteAudio(pcm); err != nil {
		return err
	}
	d.mirarAudio(pcm)
	d.muestras += int64(d.enMuestras(len(pcm)))
	return nil
}

// ── medir ─────────────────────────────────────────────────────────────

// mirarCuadro decide si el cuadro que acaba de salir es negro y mueve el
// episodio. La histéresis va en un solo sitio: mientras el episodio está
// abierto, hace falta más luz para cerrarlo que la que hizo falta para
// abrirlo.
func (d *Detector) mirarCuadro(fr []byte) {
	luma, parte := d.mirarLuma(fr)
	tope := d.ParteNegra
	if d.negro.abierto {
		tope -= d.ParteHisteresis
	}
	d.mover(&d.negro, parte >= tope, luma, parte, d.cuadros, d.cuadrosLatido,
		NegroEmpieza, NegroSigue, NegroTermina, d.enTiempoDeCuadros)
}

// mirarAudio decide si el bloque que acaba de salir es silencio. Un bloque
// vacío no dice nada: no abre ni cierra episodio.
func (d *Detector) mirarAudio(pcm []byte) {
	n := d.enMuestras(len(pcm))
	if n == 0 {
		return
	}
	nivel := d.nivelDBFS(pcm, n)
	tope := d.SilencioDBFS
	if d.silencio.abierto {
		tope += d.SilencioHisteresis
	}
	d.mover(&d.silencio, nivel < tope, nivel, 0, d.muestras, d.audioLatido,
		SilencioEmpieza, SilencioSigue, SilencioTermina, d.enTiempoDeMuestras)
}

// mover es la máquina de estados de un episodio, común al negro y al
// silencio: el mismo mecanismo para los dos, que es lo que pide F2-54.
func (d *Detector) mover(ep *episodio, dentro bool, nivel, parte float64, ahora, latido int64,
	empieza, sigue, termina string, aTiempo func(int64) time.Duration) {
	switch {
	case dentro && !ep.abierto:
		*ep = episodio{abierto: true, desde: ahora}
		d.contar(Estado{Tipo: empieza, Cuando: d.instante(ahora, aTiempo), Nivel: nivel, Parte: parte})
	case dentro:
		llevado := ahora - ep.desde
		if n := llevado / latido; n > ep.latidos {
			ep.latidos = n
			d.contar(Estado{Tipo: sigue, Cuando: d.instante(ahora, aTiempo),
				Duro: aTiempo(llevado), Nivel: nivel, Parte: parte})
		}
	case ep.abierto:
		llevado := ahora - ep.desde
		*ep = episodio{}
		d.contar(Estado{Tipo: termina, Cuando: d.instante(ahora, aTiempo),
			Duro: aTiempo(llevado), Nivel: nivel, Parte: parte})
	}
}

// mirarLuma lee el plano de luma con salto —MuestrasDeCuadro puntos
// repartidos por toda la imagen, sin copiar nada— y devuelve dos cosas de una
// sola pasada: la luma media, que es el número que se le enseña a una
// persona, y qué parte de los puntos está por debajo de PixelNegro, que es el
// número que decide.
func (d *Detector) mirarLuma(fr []byte) (media, parte float64) {
	n := d.Formato.Width * d.Formato.Height
	if n <= 0 || len(fr) < n {
		n = len(fr)
	}
	if n <= 0 {
		return 0, 1
	}
	var suma float64
	var cuenta, negros int
	for i := 0; i < n; i += d.paso {
		v := float64(fr[i])
		suma += v
		if v < d.PixelNegro {
			negros++
		}
		cuenta++
	}
	if cuenta == 0 {
		return 0, 1
	}
	return suma / float64(cuenta), float64(negros) / float64(cuenta)
}

// nivelDBFS es el nivel RMS del bloque de audio en dBFS, sobre s16le
// intercalado. El silencio digital da SuelodBFS y no −∞.
func (d *Detector) nivelDBFS(pcm []byte, muestras int) float64 {
	canales := d.Formato.Channels
	if canales <= 0 {
		canales = 1
	}
	total := muestras * canales
	var suma float64
	for i := 0; i < total && (i+1)*2 <= len(pcm); i++ {
		v := float64(int16(uint16(pcm[i*2]) | uint16(pcm[i*2+1])<<8))
		suma += v * v
	}
	if total == 0 {
		return SuelodBFS
	}
	rms := math.Sqrt(suma/float64(total)) / 32768.0
	if rms <= 0 {
		return SuelodBFS
	}
	db := 20 * math.Log10(rms)
	if db < SuelodBFS {
		return SuelodBFS
	}
	return db
}

// enMuestras convierte bytes de PCM en muestras por canal.
func (d *Detector) enMuestras(bytes int) int {
	por := d.Formato.BytesPerSample()
	if por <= 0 {
		return 0
	}
	return bytes / por
}

// enTiempoDeCuadros y enTiempoDeMuestras traducen el reloj propio a tiempo.
func (d *Detector) enTiempoDeCuadros(n int64) time.Duration {
	return time.Duration(float64(n) * d.Formato.FrameDurationNs())
}

func (d *Detector) enTiempoDeMuestras(n int64) time.Duration {
	if d.Formato.SampleRate <= 0 {
		return 0
	}
	return time.Duration(float64(n) / float64(d.Formato.SampleRate) * 1e9)
}

// instante es la hora de pared que le toca a ese punto del reloj propio.
func (d *Detector) instante(n int64, aTiempo func(int64) time.Duration) time.Time {
	return d.Origen.Add(aTiempo(n))
}

// contar manda el estado sin esperar a nadie. Si el canal está lleno se
// tira y se cuenta: el aire no se retrasa por el vigilante.
func (d *Detector) contar(e Estado) {
	select {
	case d.estados <- e:
	default:
		d.perdidos.Add(1)
	}
}
