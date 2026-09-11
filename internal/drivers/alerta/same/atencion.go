package same

import "time"

// La señal de atención es el pitido que va después de la cabecera: 853 y 960 Hz
// **al mismo tiempo**, de 8 a 25 segundos (47 CFR 11.31). No lleva datos. Se
// detecta por dos razones: para poner hora al pitido en el as-run, y porque un
// ENDEC que mandó la cabecera pero no el pitido dejó el mensaje a medias, y eso
// hay que poder decirlo.
//
// Antena787 **no sintetiza estos tonos jamás** (ADR 0010): los oye, y nada más.
const (
	FrecAtencionBaja = 853.0
	FrecAtencionAlta = 960.0
)

const (
	// ventanaAtencionMs es el largo de la ventana de medida. 50 ms son más de
	// cuarenta ciclos del tono más bajo, suficiente para separarlo de lo que
	// tenga al lado, y lo bastante corto para que el pitido se vea empezar y
	// terminar con precisión de una veintena de milisegundos.
	ventanaAtencionMs = 50
	// pasosPorVentana es cada cuánto se evalúa: cuatro veces por ventana. El
	// resto de las muestras solo alimentan las sumas corridas.
	pasosPorVentana = 4
	// parteDeCadaTono es qué parte de la potencia total tiene que llevar cada
	// uno de los dos tonos. Dos senos de la misma amplitud se llevan medio y
	// medio; pedir un cuarto cada uno deja pasar un pitido con ruido encima o
	// con los dos tonos desparejos, y descarta música y voz, donde nunca hay
	// justo dos tonos que se coman la mitad de la potencia cada uno.
	parteDeCadaTono = 0.25
	// pisoAtencion es la potencia por debajo de la cual no se mira nada: -60
	// dBFS. Debajo de eso es silencio y el cociente de potencias es puro ruido
	// de redondeo.
	pisoAtencion = 1e-6
	// toleranciaMs es cuánto se le perdona al pitido antes de darlo por
	// terminado: un hueco más corto que esto no lo corta en dos.
	toleranciaMs = 250
)

// detectorAtencion oye el par de tonos. Usa la misma maquinaria que el FSK —dos
// correladores en cuadratura con suma corrida— más una suma corrida del cuadrado
// de la muestra, que es la potencia total; el pitido se reconoce por que esos
// dos tonos solos se llevan casi toda la potencia.
type detectorAtencion struct {
	baja *correlador
	alta *correlador

	anilloE []float64
	sumaE   float64
	posE    int

	ventana    int
	cada       int
	desde      int
	minimo     uint64
	tolerancia uint64

	activo      bool
	inicio      uint64
	ultimoBueno uint64

	alTerminar func(inicio, fin uint64)
}

func nuevoDetectorAtencion(tasa int, minimo time.Duration) *detectorAtencion {
	ventana := tasa * ventanaAtencionMs / 1000
	if ventana < 8 {
		ventana = 8
	}
	cada := ventana / pasosPorVentana
	if cada < 1 {
		cada = 1
	}
	return &detectorAtencion{
		baja:       nuevoCorrelador(FrecAtencionBaja, tasa, ventana),
		alta:       nuevoCorrelador(FrecAtencionAlta, tasa, ventana),
		anilloE:    make([]float64, ventana),
		ventana:    ventana,
		cada:       cada,
		minimo:     uint64(float64(tasa) * minimo.Seconds()),
		tolerancia: uint64(tasa * toleranciaMs / 1000),
	}
}

func (a *detectorAtencion) muestra(x float64, n uint64) {
	a.baja.empujar(x)
	a.alta.empujar(x)
	p := x * x
	a.sumaE += p - a.anilloE[a.posE]
	a.anilloE[a.posE] = p
	a.posE++
	if a.posE == len(a.anilloE) {
		a.posE = 0
	}

	a.desde++
	if a.desde < a.cada {
		return
	}
	a.desde = 0
	a.evaluar(n)
}

// evaluar compara la potencia de cada tono contra la potencia total.
//
// Para un seno de amplitud A dentro de una ventana de N muestras, la suma de los
// productos vale A·N/2, así que el módulo al cuadrado por 4/N² devuelve A²: la
// potencia del tono, en las mismas unidades que la suma de los cuadrados por
// 2/N. Dividir las dos así deja la comparación libre del nivel: el pitido se
// reconoce igual a -6 que a -30 dBFS.
func (a *detectorAtencion) evaluar(n uint64) {
	nf := float64(a.ventana)
	total := a.sumaE * 2 / nf
	pb := a.baja.energia() * 4 / (nf * nf)
	pa := a.alta.energia() * 4 / (nf * nf)

	bueno := total > pisoAtencion &&
		pb > parteDeCadaTono*total &&
		pa > parteDeCadaTono*total

	if bueno {
		if !a.activo {
			a.activo = true
			// La ventana ya venía llena de pitido, así que el comienzo está una
			// ventana atrás.
			a.inicio = n - uint64(a.ventana)
		}
		a.ultimoBueno = n
		return
	}
	if a.activo && n-a.ultimoBueno > a.tolerancia {
		a.terminar(a.ultimoBueno)
	}
}

func (a *detectorAtencion) terminar(fin uint64) {
	inicio := a.inicio
	a.activo = false
	if fin > inicio && fin-inicio >= a.minimo && a.alTerminar != nil {
		a.alTerminar(inicio, fin)
	}
}

// cerrar da por terminado un pitido que todavía estaba sonando cuando se acabó
// el audio.
func (a *detectorAtencion) cerrar(n uint64) {
	if a.activo {
		a.terminar(a.ultimoBueno)
	}
	_ = n
}
