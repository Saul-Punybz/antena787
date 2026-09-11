package same

import "math"

// Los números del protocolo, tal como los fija 47 CFR 11.31. Se escriben como
// fracciones porque así son exactos: el baudio es 6250/12, la marca cuatro
// veces eso y el espacio tres veces eso. De ahí sale que en un bit entran
// cuatro ciclos de marca o tres de espacio, ni uno más.
const (
	// Baudios es la velocidad del FSK: 520.8333… bits por segundo. Cada bit
	// dura 1.92 ms.
	Baudios = 6250.0 / 12.0
	// FrecMarca es el tono del uno: 2083.333… Hz.
	FrecMarca = 4 * Baudios
	// FrecEspacio es el tono del cero: 1562.5 Hz.
	FrecEspacio = 3 * Baudios
	// BytePreambulo es el byte de sincronismo, 0xAB (binario 10101011), que va
	// dieciséis veces antes de cada cabecera y antes de cada NNNN. Mandado con
	// el bit menos significativo primero se oye como 1,1,0,1,0,1,0,1: casi una
	// alternancia pura, que es exactamente lo que necesita un lazo de reloj
	// para engancharse.
	BytePreambulo = 0xAB
	// BytesPreambulo es cuántas veces va ese byte.
	BytesPreambulo = 16
)

// La tabla del coseno. En vez de llamar a math.Cos por muestra —que serían
// cuatro llamadas por muestra, 200 mil por segundo de audio— se guarda un
// ciclo entero en 4096 puntos y se indexa con un acumulador de fase de 32 bits:
// sumar el incremento y quedarse con los 12 bits de arriba. Es exacto para
// siempre (la aritmética entera no acumula error como un oscilador recursivo) y
// el error por redondear la fase a 1/4096 de ciclo queda unos 70 dB abajo, mil
// veces menos que el ruido de un retorno de aire.
const (
	tamTabla     = 4096
	desplTabla   = 32 - 12
	mascaraTabla = tamTabla - 1
	// menosCuarto es restarle un cuarto de ciclo a la fase, o sea pasar de
	// coseno a seno: sen(x) = cos(x - π/2).
	menosCuarto = 3 * tamTabla / 4
	// recalcCada es cada cuántas muestras se vuelve a sumar la ventana entera
	// en vez de seguir sumando y restando. La suma corrida es exacta en
	// aritmética real pero en coma flotante acumula polvo, y esto corre meses
	// seguidos; recorrer 92 números cada 8192 muestras no se nota.
	recalcCada = 8192
)

var tablaCos [tamTabla]float64

func init() {
	for i := range tablaCos {
		tablaCos[i] = math.Cos(2 * math.Pi * float64(i) / tamTabla)
	}
}

// correlador mide cuánta energía hay a una frecuencia dentro de una ventana que
// se va corriendo muestra a muestra.
//
// La idea: multiplicar el audio por un coseno y por un seno de la frecuencia
// buscada y sumar cada producto sobre la ventana. Las dos sumas son las dos
// patas de un número complejo; su módulo al cuadrado es la energía, y —esto es
// lo que hace que sea «no coherente»— ese módulo no depende de en qué fase
// venía el tono, que es justo lo que no sabemos de un audio que llega por un
// cable.
//
// Para no recorrer la ventana en cada muestra se guarda cada producto en un
// anillo del largo de la ventana: entra el nuevo, sale el más viejo. Dos sumas
// y dos restas por muestra.
type correlador struct {
	incremento uint32
	fase       uint32
	sumaI      float64
	sumaQ      float64
	anilloI    []float64
	anilloQ    []float64
	pos        int
	desdeRecal int
}

func nuevoCorrelador(frec float64, tasa, ventana int) *correlador {
	ciclosPorMuestra := frec / float64(tasa)
	return &correlador{
		incremento: uint32(math.Round(math.Mod(ciclosPorMuestra, 1) * 4294967296)),
		anilloI:    make([]float64, ventana),
		anilloQ:    make([]float64, ventana),
	}
}

func (c *correlador) empujar(x float64) {
	i := c.fase >> desplTabla
	c.fase += c.incremento
	pi := x * tablaCos[i]
	pq := x * tablaCos[(i+menosCuarto)&mascaraTabla]
	c.sumaI += pi - c.anilloI[c.pos]
	c.sumaQ += pq - c.anilloQ[c.pos]
	c.anilloI[c.pos] = pi
	c.anilloQ[c.pos] = pq
	c.pos++
	if c.pos == len(c.anilloI) {
		c.pos = 0
	}
	c.desdeRecal++
	if c.desdeRecal >= recalcCada {
		c.desdeRecal = 0
		c.recalcular()
	}
}

func (c *correlador) recalcular() {
	var si, sq float64
	for k := range c.anilloI {
		si += c.anilloI[k]
		sq += c.anilloQ[k]
	}
	c.sumaI, c.sumaQ = si, sq
}

// energia es el módulo al cuadrado: cuánta señal hay a esa frecuencia.
func (c *correlador) energia() float64 { return c.sumaI*c.sumaI + c.sumaQ*c.sumaQ }

// Constantes del lazo de reloj de bit.
const (
	// alfaReloj es cuánto se corrige la fase con cada cruce por cero. 0.1
	// engancha en unos diez cruces —el preámbulo trae casi cien— y no se
	// vuelve loco con un cruce que puso el ruido.
	alfaReloj = 0.1
	// betaReloj es cuánto se corrige el largo del bit. Va mucho más despacio
	// que la fase porque el largo del bit no cambia: lo único que corrige es
	// que el audio venga muestreado a otra tasa (una grabación a 47.9 kHz
	// leída como 48 kHz es un 0.2 % de error, que a lo largo de una cabecera
	// de 268 bits son más de medio bit de corrimiento).
	betaReloj = 0.005
	// margenPaso es cuánto se le deja mover el largo del bit: ±3 %. Más que
	// eso no es una tasa distinta, es otra cosa.
	margenPaso = 0.03
	// coefEnergia es la memoria de la media móvil de energía, que sirve de
	// piso de ruido: con 0.001 la media vale unas mil muestras, unos 20 ms.
	coefEnergia = 0.001
)

// Estados del ensamblador de bits.
const (
	buscaSinc = iota
	enPreambulo
	enCabecera
	enFin
)

// maxBufCabecera es hasta dónde se deja crecer una cabecera antes de darla por
// perdida. La más larga que permite la norma —treinta y una zonas— mide 252
// caracteres.
const maxBufCabecera = 260

// colaCabecera es cuánto mide la cabecera después del más (+): «TTTT-JJJHHMM-
// LLLLLLLL-» son 22 caracteres, siempre. Esto es lo que permite recortar una
// cabecera **sucia** en el sitio exacto: en cuanto se ve el +, se sabe dónde
// termina sin necesidad de que lo que sigue se pueda leer. Y eso es lo que hace
// que el voto por carácter sirva de algo, porque las tres copias llegan con el
// mismo largo aunque vengan rotas.
const colaCabecera = 22

// demodulador convierte muestras en bytes: dos correladores, un lazo de reloj y
// un ensamblador. No sabe nada de alertas; cuando junta algo que parece una
// cabecera o un NNNN avisa por los dos callbacks, que se fijan una sola vez al
// construir (no hay cierres ni asignaciones en el camino caliente).
type demodulador struct {
	marca   *correlador
	espacio *correlador

	pasoNominal float64
	paso        float64
	fase        float64

	discAnt    float64
	energAnt   float64
	hayAnt     bool
	energMedia float64

	registro    uint16
	estado      int
	bitsEnByte  uint8
	byteActual  byte
	buf         []byte
	finEsperado int
	enes        int

	alCandidato func(crudo []byte, valido bool)
	alFin       func()
}

func nuevoDemodulador(tasa int) *demodulador {
	// La ventana es un bit: tantas muestras como quepan en 1.92 ms. A 48 kHz
	// son 92.16, y se redondea a 92; ese 0.17 % de desajuste no mueve la
	// aguja porque los dos tonos están a 520 Hz uno del otro.
	paso := float64(tasa) / Baudios
	ventana := int(math.Round(paso))
	if ventana < 4 {
		ventana = 4
	}
	return &demodulador{
		marca:       nuevoCorrelador(FrecMarca, tasa, ventana),
		espacio:     nuevoCorrelador(FrecEspacio, tasa, ventana),
		pasoNominal: paso,
		paso:        paso,
		buf:         make([]byte, 0, maxBufCabecera),
	}
}

// muestra es el camino caliente de verdad: unas veinte multiplicaciones por
// muestra y ninguna asignación.
func (m *demodulador) muestra(x float64) {
	m.marca.empujar(x)
	m.espacio.empujar(x)
	em := m.marca.energia()
	ee := m.espacio.energia()

	// El discriminador: positivo si gana la marca (bit 1), negativo si gana el
	// espacio (bit 0). Como las dos ventanas cubren exactamente un bit, este
	// número leído al final de un bit es la decisión de ese bit.
	disc := em - ee
	energia := em + ee
	m.energMedia += (energia - m.energMedia) * coefEnergia

	m.fase++

	// Recuperación de reloj. Cuando el bit cambia, el discriminador tarda un
	// bit entero en pasar de un extremo al otro, así que cruza el cero a mitad
	// de camino entre dos decisiones. Ese cruce es la referencia: si cae antes
	// de la mitad, la rejilla va tarde; si cae después, va temprano. El cruce
	// se ubica entre dos muestras interpolando con los dos valores, lo que da
	// resolución muy por debajo de una muestra.
	//
	// El silencio no arrastra el reloj: si no hay energía, no hay cruce que
	// valga (en silencio digital el discriminador es cero clavado).
	if m.hayAnt && (m.discAnt >= 0) != (disc >= 0) && energia+m.energAnt > m.energMedia {
		a := math.Abs(m.discAnt)
		b := math.Abs(disc)
		frac := 0.5
		if a+b > 0 {
			frac = a / (a + b)
		}
		err := m.fase - 1 + frac - m.paso/2
		if err > m.paso/2 {
			err -= m.paso
		} else if err < -m.paso/2 {
			err += m.paso
		}
		m.fase -= alfaReloj * err
		paso := m.paso + betaReloj*err
		if hi := m.pasoNominal * (1 + margenPaso); paso > hi {
			paso = hi
		} else if lo := m.pasoNominal * (1 - margenPaso); paso < lo {
			paso = lo
		}
		m.paso = paso
	}
	m.discAnt = disc
	m.energAnt = energia
	m.hayAnt = true

	if m.fase >= m.paso {
		m.fase -= m.paso
		if disc > 0 {
			m.bit(1)
		} else {
			m.bit(0)
		}
	}
}

// bit mete un bit en el ensamblador.
//
// Mientras se busca sincronía los bits entran en un registro de 16 y se compara
// con los dos primeros bytes del preámbulo: 0xAB 0xAB con el bit menos
// significativo primero es 0xABAB en el registro. Ese patrón de ocho bits no se
// parece a ninguna de sus siete rotaciones, así que verlo dice a la vez dónde
// empieza el bit y dónde empieza el byte. De ahí en adelante los bits se juntan
// de ocho en ocho.
func (m *demodulador) bit(b int) {
	if m.estado == buscaSinc {
		m.registro = m.registro>>1 | uint16(b)<<15
		if m.registro == 0xABAB {
			m.estado = enPreambulo
			m.bitsEnByte = 0
			m.byteActual = 0
		}
		return
	}
	m.byteActual |= byte(b) << m.bitsEnByte
	m.bitsEnByte++
	if m.bitsEnByte == 8 {
		c := m.byteActual
		m.byteActual = 0
		m.bitsEnByte = 0
		m.byteListo(c)
	}
}

func (m *demodulador) byteListo(c byte) {
	switch m.estado {
	case enPreambulo:
		switch {
		case c == BytePreambulo:
			// Todavía en el preámbulo: son dieciséis bytes iguales.
		case c == 'Z':
			// «ZCZC» abre la cabecera.
			m.buf = append(m.buf[:0], 'Z')
			m.estado = enCabecera
		case c == 'N':
			// «NNNN» es el fin de mensaje.
			m.enes = 1
			m.estado = enFin
		default:
			m.reiniciar()
		}
	case enCabecera:
		if c < 32 || c > 126 {
			// Basura: se entrega igual como candidato inválido, porque con tres
			// copias sucias el voto por carácter todavía puede armar la buena.
			m.abandonarCabecera()
			return
		}
		m.buf = append(m.buf, c)
		if c == '+' && m.finEsperado == 0 {
			m.finEsperado = len(m.buf) + colaCabecera
		}
		if m.finEsperado > 0 && len(m.buf) >= m.finEsperado {
			_, ok := analizarCabecera(m.buf)
			m.alCandidato(m.buf, ok)
			m.reiniciar()
			return
		}
		if len(m.buf) > maxBufCabecera {
			m.abandonarCabecera()
			return
		}
	case enFin:
		if c != 'N' {
			m.reiniciar()
			return
		}
		m.enes++
		if m.enes == 4 {
			m.alFin()
			m.reiniciar()
		}
	}
}

func (m *demodulador) abandonarCabecera() {
	if len(m.buf) >= minCabecera {
		m.alCandidato(m.buf, false)
	}
	m.reiniciar()
}

func (m *demodulador) reiniciar() {
	m.estado = buscaSinc
	m.registro = 0
	m.bitsEnByte = 0
	m.byteActual = 0
	m.buf = m.buf[:0]
	m.finEsperado = 0
	m.enes = 0
}
