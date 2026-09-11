package same

import (
	"math"
	"math/rand"
)

// ¡AVISO! Este modulador existe SOLO en este archivo de pruebas, y es a
// propósito.
//
// Para probar un decodificador de SAME hace falta señal de SAME, y la única
// manera honesta de tenerla sin depender de una grabación es fabricarla. Pero
// **Antena787 no emite alertas de emergencia ni sintetiza los tonos de
// atención**: eso es trabajo del ENDEC certificado que está aguas abajo, y el
// software no lo reemplaza ni le hace la competencia
// (docs/adr/0010-eas-integrate-the-endec-never-replace-it.md: «No EAS encoder,
// no SAME generator, and no tone synthesis ever ship in Antena787»).
//
// Por eso el modulador vive en un archivo `_test.go`: el compilador de Go no lo
// mete en el binario. **El binario de Antena787 no puede emitir SAME ni por
// accidente, ni por una bandera escondida, ni porque alguien llame a la función
// equivocada: la función no está ahí.** Si algún día alguien mueve esto a un
// archivo sin `_test`, está rompiendo el ADR 0010, no un detalle de estilo.

// generador fabrica el audio de una cabecera SAME tal como lo describe
// 47 CFR 11.31: FSK a 520.8333 baudios, marca 2083.3 Hz, espacio 1562.5 Hz,
// ASCII de siete bits con un octavo bit nulo, bit menos significativo primero,
// preámbulo de dieciséis bytes 0xAB.
type generador struct {
	tasa     int
	amplitud float64
	fase     float64
	// pendiente es el resto fraccionario de muestras por bit: a 48 kHz un bit
	// son 92.16 muestras, así que hay que arrastrar el 0.16 para que el
	// baudio salga exacto y no se acumule un error de tiempo.
	pendiente float64
	faseBaja  float64
	faseAlta  float64
	salida    []float64
}

func nuevoGenerador(tasa int, amplitud float64) *generador {
	return &generador{tasa: tasa, amplitud: amplitud, salida: make([]float64, 0, tasa*20)}
}

// bit escribe un bit: un tono de marca o de espacio que dura 1/520.8333 s. La
// fase sigue de largo entre bits, como en un modulador de fase continua.
func (g *generador) bit(b int) {
	f := FrecEspacio
	if b == 1 {
		f = FrecMarca
	}
	g.pendiente += float64(g.tasa) / Baudios
	n := int(g.pendiente)
	g.pendiente -= float64(n)
	inc := 2 * math.Pi * f / float64(g.tasa)
	for i := 0; i < n; i++ {
		g.salida = append(g.salida, g.amplitud*math.Sin(g.fase))
		g.fase += inc
	}
}

// octeto manda los ocho bits de un byte con el menos significativo primero.
// Para un carácter ASCII el bit de arriba es cero, que es el «octavo bit nulo»
// de la norma; para el 0xAB del preámbulo vale uno, porque ese byte no es un
// carácter.
func (g *generador) octeto(c byte) {
	for k := 0; k < 8; k++ {
		g.bit(int(c>>uint(k)) & 1)
	}
}

// rafaga es un preámbulo de dieciséis 0xAB seguido del texto.
func (g *generador) rafaga(texto string) {
	for i := 0; i < BytesPreambulo; i++ {
		g.octeto(BytePreambulo)
	}
	for i := 0; i < len(texto); i++ {
		g.octeto(texto[i])
	}
}

func (g *generador) silencio(seg float64) {
	n := int(float64(g.tasa) * seg)
	for i := 0; i < n; i++ {
		g.salida = append(g.salida, 0)
	}
}

// atencion son los dos tonos a la vez, 853 y 960 Hz, cada uno con la mitad de
// la amplitud para que juntos no recorten.
func (g *generador) atencion(seg float64) {
	n := int(float64(g.tasa) * seg)
	ib := 2 * math.Pi * FrecAtencionBaja / float64(g.tasa)
	ia := 2 * math.Pi * FrecAtencionAlta / float64(g.tasa)
	for i := 0; i < n; i++ {
		v := g.amplitud/2*math.Sin(g.faseBaja) + g.amplitud/2*math.Sin(g.faseAlta)
		g.salida = append(g.salida, v)
		g.faseBaja += ib
		g.faseAlta += ia
	}
}

// tresVeces manda un bloque las tres veces que pide la norma, con un segundo de
// silencio entre una y otra.
func (g *generador) tresVeces(texto string) {
	for i := 0; i < 3; i++ {
		g.rafaga(texto)
		g.silencio(1)
	}
}

// mensajeCompleto es una alerta como sale de un ENDEC: tres cabeceras, el
// pitido de atención, el audio del mensaje (aquí silencio: el decodificador no
// lo mira) y tres fines de mensaje.
func (g *generador) mensajeCompleto(cab string, segAtencion, segMensaje float64) {
	g.silencio(0.2)
	g.tresVeces(cab)
	if segAtencion > 0 {
		g.atencion(segAtencion)
		g.silencio(0.3)
	}
	g.silencio(segMensaje)
	g.tresVeces("NNNN")
}

// aPCM pasa las muestras a PCM s16le, que es lo que come el decodificador.
func aPCM(m []float64) []byte {
	b := make([]byte, len(m)*2)
	for i, v := range m {
		if v > 0.999 {
			v = 0.999
		} else if v < -0.999 {
			v = -0.999
		}
		s := int16(v * 32767)
		b[i*2] = byte(uint16(s))
		b[i*2+1] = byte(uint16(s) >> 8)
	}
	return b
}

// conRuido le suma ruido blanco gaussiano a todo el audio, incluidos los
// silencios, con la relación señal/ruido pedida en dB. La señal se mide solo
// donde hay señal: el RMS de las ráfagas, no el del archivo entero, que estaría
// aguado por los silencios y haría parecer la prueba más dura de lo que es.
func conRuido(m []float64, snrDB float64, semilla int64) []float64 {
	var suma float64
	var n int
	for _, v := range m {
		if v != 0 {
			suma += v * v
			n++
		}
	}
	if n == 0 {
		return m
	}
	rms := math.Sqrt(suma / float64(n))
	sigma := rms * math.Pow(10, -snrDB/20)
	r := rand.New(rand.NewSource(semilla))
	out := make([]float64, len(m))
	for i, v := range m {
		out[i] = v + sigma*r.NormFloat64()
	}
	return out
}

// musica fabrica algo que se parece a programación: tres notas con vibrato, un
// golpe de percusión cada medio segundo y ruido de fondo. No tiene nada del
// protocolo, y el decodificador no debe oír una sola alerta aquí.
func musica(tasa int, seg float64, semilla int64) []float64 {
	n := int(float64(tasa) * seg)
	out := make([]float64, n)
	r := rand.New(rand.NewSource(semilla))
	notas := []float64{220, 277.18, 329.63, 440, 1500, 2000}
	for i := 0; i < n; i++ {
		t := float64(i) / float64(tasa)
		var v float64
		for k, f := range notas {
			v += 0.12 * math.Sin(2*math.Pi*f*t+0.4*math.Sin(2*math.Pi*(3+float64(k))*t))
		}
		if math.Mod(t, 0.5) < 0.02 {
			v += 0.3 * r.NormFloat64()
		}
		v += 0.02 * r.NormFloat64()
		out[i] = v * 0.5
	}
	return out
}
