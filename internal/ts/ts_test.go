package ts

import (
	"bytes"
	"errors"
	"fmt"

	"math"
	"math/rand"
	"strings"
	"testing"
)

// internal/ts es el medidor: es quien dice si un transport stream serviría
// para un multiplexor. De él dependen F0-01 a F0-04 (marcas monotónicas, sin
// cuadros perdidos, continuidad) y F2-114 (el TS llega completo al grupo
// multicast). Hasta hoy no tenía ni un archivo de prueba: el medidor no
// estaba medido. Lo que sigue lo mide con streams hechos a mano, donde el
// defecto se pone a propósito y se sabe cuántos hay.

// analizar corre Analyze sobre unos bytes y se asegura de que nunca entre en
// pánico: un TS es datos que entran por la red, y un medidor que revienta con
// basura tira el aire abajo. Devuelve el informe y el error tal cual.
func analizar(t *testing.T, datos []byte) (Report, error) {
	t.Helper()
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("Analyze entró en pánico con %d bytes: %v", len(datos), p)
		}
	}()
	return Analyze(bytes.NewReader(datos))
}

// ── el TS limpio, medido entero ───────────────────────────────────────

// Un stream sin un solo defecto tiene que salir con todo en su sitio: la
// cuenta de paquetes exacta, cero errores de continuidad, y las tablas
// leídas del propio stream (número de programa, tsid, PID). Si esta prueba
// falla, no hay ninguna otra que valga.
func TestUnTSLimpioSeMideEntero(t *testing.T) {
	const porPID = 200
	datos := streamLimpio(porPID)
	r, err := analizar(t, datos)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	esperados := int64(len(datos) / PacketSize)
	if r.Packets != esperados {
		t.Errorf("paquetes: %d, se esperaban %d", r.Packets, esperados)
	}
	if r.CCErrors != 0 {
		t.Errorf("errores de continuidad: %d, el stream está limpio", r.CCErrors)
	}
	// Un nulo cada veinte vueltas del bucle: 10 en 200.
	if quiere := int64(porPID / 20); r.NullPackets != quiere {
		t.Errorf("paquetes nulos: %d, se esperaban %d", r.NullPackets, quiere)
	}
	// PAT, PMT, video, audio y nulos: cinco PID distintos y ni uno más.
	if len(r.PIDs) != 5 {
		t.Errorf("PIDs distintos: %d (%v), se esperaban 5", len(r.PIDs), r.PIDs)
	}
	for pid, quiere := range map[uint16]int64{
		pidPAT:   1,
		pidPMT:   1,
		pidVideo: porPID,
		pidAudio: porPID,
		pidNul:   porPID / 20,
	} {
		if r.PIDs[pid] != quiere {
			t.Errorf("PID %d: %d paquetes, se esperaban %d", pid, r.PIDs[pid], quiere)
		}
	}
	if quiere := int64(porPID / 10); r.PCRCount != quiere {
		t.Errorf("PCR contados: %d, se esperaban %d", r.PCRCount, quiere)
	}

	// Las tablas: es lo primero que lee un multiplexor y lo que la persona
	// escribió una vez en la pantalla de salidas.
	if r.TSID != tsid {
		t.Errorf("tsid: %d, se esperaba %d", r.TSID, tsid)
	}
	if r.Programa != programa {
		t.Errorf("número de programa: %d, se esperaba %d", r.Programa, programa)
	}
	if r.PMTPid != pidPMT {
		t.Errorf("PID de la PMT: %d, se esperaba %d", r.PMTPid, pidPMT)
	}
	if r.PCRPid != pidVideo {
		t.Errorf("PID del PCR: %d, se esperaba %d", r.PCRPid, pidVideo)
	}
	if r.PATCount != 1 || r.PMTCount != 1 {
		t.Errorf("tablas vistas: PAT %d, PMT %d; se esperaba una de cada", r.PATCount, r.PMTCount)
	}
	if len(r.Elementales) != 2 {
		t.Errorf("flujos elementales: %v, se esperaban dos", r.Elementales)
	}
	if r.Elementales[pidVideo] != TipoMPEG2Video {
		t.Errorf("tipo del video: 0x%02x, se esperaba 0x%02x", r.Elementales[pidVideo], TipoMPEG2Video)
	}
	if r.Elementales[pidAudio] != TipoAC3 {
		t.Errorf("tipo del audio: 0x%02x, se esperaba 0x%02x", r.Elementales[pidAudio], TipoAC3)
	}
	if r.VideoPid() != pidVideo {
		t.Errorf("VideoPid: %d, se esperaba %d", r.VideoPid(), pidVideo)
	}
	if r.AudioPid() != pidAudio {
		t.Errorf("AudioPid: %d, se esperaba %d", r.AudioPid(), pidAudio)
	}

	// El PCR sube 0,1 s cada diez paquetes de video: 20 PCR son 1,9 s entre
	// el primero y el último.
	if quiere := 1.9; math.Abs(r.DurationSec-quiere) > 0.01 {
		t.Errorf("duración: %.3f s, se esperaban %.3f", r.DurationSec, quiere)
	}
	if quiere := 100.0; math.Abs(r.PCRMeanGapMs-quiere) > 0.5 || math.Abs(r.PCRMaxGapMs-quiere) > 0.5 {
		t.Errorf("brechas de PCR: media %.1f ms, máxima %.1f ms; se esperaban %.1f de las dos",
			r.PCRMeanGapMs, r.PCRMaxGapMs, quiere)
	}
	if r.TSNonMonotone != 0 || r.TSGapsOver1s != 0 {
		t.Errorf("marcas de tiempo: %d hacia atrás y %d saltos, el stream limpio no lleva PES",
			r.TSNonMonotone, r.TSGapsOver1s)
	}

	// String y Tablas son lo que se pega en un reporte de F0 y lo que sale en
	// la pantalla de conexiones: si dejan de nombrar el programa, alguien lee
	// un informe que no dice nada.
	linea := r.String()
	for _, trozo := range []string{"paquetes 412", "errores CC 0", fmt.Sprintf("programa %d", programa)} {
		if !strings.Contains(linea, trozo) {
			t.Errorf("String() no dice %q:\n%s", trozo, linea)
		}
	}
}

// ── continuidad ──────────────────────────────────────────────────────

// El contador de continuidad es lo que delata un paquete perdido en el
// camino, y es la mitad de F0-03: un paquete que no llegó es un cuadro que
// no se ve. Se cuenta un error por cada salto, ni uno más.
func TestLaContinuidadRotaSeCuentaUnaVezPorSalto(t *testing.T) {
	casos := []struct {
		nombre string
		saltos int
	}{
		{"un paquete perdido", 1},
		{"tres paquetes perdidos en sitios distintos", 3},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			c := nuevoConstructor()
			c.seccion(pidPAT, armarPAT(tsid, programaPAT{Numero: programa, PMTPid: pidPMT}))
			c.seccion(pidPMT, armarPMT(programa, pidVideo, flujo{Tipo: TipoH264, PID: pidVideo}))
			for i := 0; i < 100; i++ {
				// Los saltos se reparten cada 25 paquetes: nunca dos seguidos,
				// para que no se puedan confundir en una sola cuenta.
				if caso.saltos > i/25 && i%25 == 12 {
					c.saltarCC(pidVideo)
				}
				c.carga(pidVideo, 0xaa)
			}
			r, err := analizar(t, c.bytes())
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if r.CCErrors != int64(caso.saltos) {
				t.Fatalf("errores de continuidad: %d, se metieron %d saltos", r.CCErrors, caso.saltos)
			}
		})
	}
}

// La vuelta del contador (15 → 0) es correcta y no es un error. Un medidor
// que la contara daría un error de continuidad cada dieciséis paquetes, o sea
// siempre, y el informe no serviría para nada.
func TestLaVueltaDelContadorNoEsUnError(t *testing.T) {
	c := nuevoConstructor()
	for i := 0; i < 64; i++ { // cuatro vueltas completas
		c.carga(pidVideo, 0xaa)
	}
	r, err := analizar(t, c.bytes())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.CCErrors != 0 {
		t.Fatalf("errores de continuidad: %d; dar la vuelta en 15 → 0 es lo normal", r.CCErrors)
	}
}

// Un paquete sin carga (solo campo de adaptación) no avanza el contador por
// norma. Si el medidor lo contara vería un salto donde el stream está bien,
// y la salida a un multiplexor —que mete estos paquetes para rellenar— saldría
// marcada como mala.
func TestLosPaquetesSinCargaNoRompenLaContinuidad(t *testing.T) {
	c := nuevoConstructor()
	for i := 0; i < 40; i++ {
		c.carga(pidVideo, 0xaa)
		if i%5 == 0 {
			c.soloAdaptacion(pidVideo)
		}
	}
	r, err := analizar(t, c.bytes())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.CCErrors != 0 {
		t.Fatalf("errores de continuidad: %d; los paquetes sin carga no llevan contador", r.CCErrors)
	}
}

// ── PCR ──────────────────────────────────────────────────────────────

// Un TS sin PCR no se puede sincronizar: el receptor no sabe a qué ritmo
// sacarlo. El medidor tiene que decirlo con un cero, no adivinar una tasa.
func TestSinPCRNoHayTasaNiDuracion(t *testing.T) {
	c := nuevoConstructor()
	c.seccion(pidPAT, armarPAT(tsid, programaPAT{Numero: programa, PMTPid: pidPMT}))
	c.seccion(pidPMT, armarPMT(programa, pidVideo, flujo{Tipo: TipoMPEG2Video, PID: pidVideo}))
	for i := 0; i < 100; i++ {
		c.carga(pidVideo, 0xaa)
	}
	r, err := analizar(t, c.bytes())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.PCRCount != 0 {
		t.Errorf("PCR contados: %d, el stream no lleva ninguno", r.PCRCount)
	}
	if r.PCRMaxGapMs != 0 || r.PCRMeanGapMs != 0 {
		t.Errorf("brechas de PCR sin PCR: máxima %.1f, media %.1f", r.PCRMaxGapMs, r.PCRMeanGapMs)
	}
	if r.BitrateMean != 0 || r.BitrateMin != 0 || r.BitrateMax != 0 {
		t.Errorf("tasa sin PCR: media %.0f, min %.0f, max %.0f; debía quedar en cero",
			r.BitrateMean, r.BitrateMin, r.BitrateMax)
	}
	if r.DurationSec != 0 {
		t.Errorf("duración sin PCR: %.3f s; no hay reloj del que sacarla", r.DurationSec)
	}
	// Pero las tablas sí se leen: un stream sin PCR todavía dice qué programa
	// lleva, y eso es lo que la pantalla de conexiones necesita enseñar.
	if r.Programa != programa || r.PMTPid != pidPMT {
		t.Errorf("las tablas se leen igual sin PCR: programa %d, PMT en %d", r.Programa, r.PMTPid)
	}
}

// Un salto del reloj de programa es lo que el PRD llama «brecha»: el receptor
// se queda sin referencia y el multiplexor lo rechaza. Tiene que salir en la
// brecha máxima, y la media no puede esconderlo.
func TestElSaltoDePCRSaleEnLaBrechaMaxima(t *testing.T) {
	c := nuevoConstructor()
	c.seccion(pidPAT, armarPAT(tsid, programaPAT{Numero: programa, PMTPid: pidPMT}))
	c.seccion(pidPMT, armarPMT(programa, pidVideo, flujo{Tipo: TipoMPEG2Video, PID: pidVideo}))
	// Cuatro PCR cada 40 ms y luego un hueco de dos segundos.
	for _, seg := range []float64{0, 0.04, 0.08, 0.12, 2.12} {
		c.conPCR(pidVideo, seg, 0xaa)
		c.carga(pidVideo, 0xaa)
	}
	r, err := analizar(t, c.bytes())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.PCRCount != 5 {
		t.Fatalf("PCR contados: %d, se esperaban 5", r.PCRCount)
	}
	if quiere := 2000.0; math.Abs(r.PCRMaxGapMs-quiere) > 1 {
		t.Errorf("brecha máxima: %.1f ms, se esperaban %.0f", r.PCRMaxGapMs, quiere)
	}
	// Tres brechas de 40 ms y una de 2000: media 530.
	if quiere := 530.0; math.Abs(r.PCRMeanGapMs-quiere) > 1 {
		t.Errorf("brecha media: %.1f ms, se esperaban %.0f", r.PCRMeanGapMs, quiere)
	}
	if quiere := 2.12; math.Abs(r.DurationSec-quiere) > 0.01 {
		t.Errorf("duración: %.3f s, se esperaban %.2f", r.DurationSec, quiere)
	}
}

// Un PCR que va hacia atrás (el reloj da la vuelta, o alguien empalmó dos
// streams) no puede convertirse en una brecha negativa que baje la media y
// haga pasar por bueno un stream roto.
func TestUnPCRHaciaAtrasNoSeCuentaComoBrecha(t *testing.T) {
	c := nuevoConstructor()
	for _, seg := range []float64{10.0, 10.04, 0.5, 0.54} { // el tercero retrocede
		c.conPCR(pidVideo, seg, 0xaa)
	}
	r, err := analizar(t, c.bytes())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.PCRMaxGapMs < 0 || r.PCRMeanGapMs < 0 {
		t.Fatalf("brechas negativas: máxima %.1f, media %.1f", r.PCRMaxGapMs, r.PCRMeanGapMs)
	}
	// Solo las dos brechas hacia adelante de 40 ms cuentan.
	if quiere := 40.0; math.Abs(r.PCRMaxGapMs-quiere) > 1 || math.Abs(r.PCRMeanGapMs-quiere) > 1 {
		t.Errorf("brechas: máxima %.1f ms, media %.1f ms; se esperaban %.0f de las dos",
			r.PCRMaxGapMs, r.PCRMeanGapMs, quiere)
	}
	// El último PCR es menor que el primero, así que no hay duración que dar:
	// más vale cero que un número negativo en un informe.
	if r.DurationSec != 0 {
		t.Errorf("duración con el reloj hacia atrás: %.3f s; se esperaba cero", r.DurationSec)
	}
}

// ── tasa ─────────────────────────────────────────────────────────────

// streamConTasa arma un TS con un PCR al arrancar y, después, un tramo por
// cada segundo pedido, con tantos paquetes como diga la lista y un PCR al
// cerrarlo. Así la tasa que mide Analyze es exactamente
// paquetes × 188 × 8 bits por segundo, sin nada que estimar.
func streamConTasa(paquetesPorSegundo ...int) []byte {
	c := nuevoConstructor()
	c.seccion(pidPAT, armarPAT(tsid, programaPAT{Numero: programa, PMTPid: pidPMT}))
	c.seccion(pidPMT, armarPMT(programa, pidVideo, flujo{Tipo: TipoMPEG2Video, PID: pidVideo}))
	c.conPCR(pidVideo, 0, 0xaa) // abre la primera ventana
	for i, n := range paquetesPorSegundo {
		for k := 0; k < n-1; k++ {
			c.carga(pidVideo, 0xaa)
		}
		c.conPCR(pidVideo, float64(i+1), 0xaa) // cierra el segundo
	}
	return c.bytes()
}

// bitsPorSegundo es la tasa exacta de n paquetes de TS en un segundo.
func bitsPorSegundo(n int) float64 { return float64(n) * PacketSize * 8 }

// Una salida a un multiplexor va a tasa constante: el multiplexor reserva su
// hueco y lo que se pase se cae. El medidor tiene que ver la diferencia entre
// constante y variable, porque es la señal de que la salida no cabe.
func TestLaTasaConstanteSaleConDesvioCero(t *testing.T) {
	const porSegundo = 500
	r, err := analizar(t, streamConTasa(porSegundo, porSegundo, porSegundo))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	quiere := bitsPorSegundo(porSegundo)
	for nombre, medido := range map[string]float64{"media": r.BitrateMean, "mínima": r.BitrateMin, "máxima": r.BitrateMax} {
		if math.Abs(medido-quiere) > 1 {
			t.Errorf("tasa %s: %.0f bits/s, se esperaban %.0f", nombre, medido, quiere)
		}
	}
	if r.BitrateDevPct > 0.001 {
		t.Errorf("desvío: %.4f%%; una tasa constante no desvía", r.BitrateDevPct)
	}
}

func TestLaTasaVariableSaleConSuDesvio(t *testing.T) {
	// El segundo tramo lleva la mitad de paquetes: es lo que pasa cuando el
	// encoder no llena su hueco y el multiplexor se queda esperando.
	r, err := analizar(t, streamConTasa(1000, 500))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if quiere := bitsPorSegundo(1000); math.Abs(r.BitrateMax-quiere) > 1 {
		t.Errorf("tasa máxima: %.0f, se esperaban %.0f", r.BitrateMax, quiere)
	}
	if quiere := bitsPorSegundo(500); math.Abs(r.BitrateMin-quiere) > 1 {
		t.Errorf("tasa mínima: %.0f, se esperaban %.0f", r.BitrateMin, quiere)
	}
	if quiere := bitsPorSegundo(750); math.Abs(r.BitrateMean-quiere) > 1 {
		t.Errorf("tasa media: %.0f, se esperaban %.0f", r.BitrateMean, quiere)
	}
	// 250 de 750 es un tercio arriba y un tercio abajo.
	if quiere := 100.0 / 3; math.Abs(r.BitrateDevPct-quiere) > 0.1 {
		t.Errorf("desvío: %.2f%%, se esperaba %.2f%%", r.BitrateDevPct, quiere)
	}
}

// ── PIDs, programa y tablas ──────────────────────────────────────────

// La entrada de programa 0 de la PAT es la tabla de red, no un programa. Un
// medidor que la tomara por el programa del canal diría «programa 0» en la
// pantalla de conexiones y nadie entendería por qué.
func TestLaEntradaDeRedDeLaPATNoEsUnPrograma(t *testing.T) {
	c := nuevoConstructor()
	c.seccion(pidPAT, armarPAT(0x1234,
		programaPAT{Numero: 0, PMTPid: 0x0010},  // la NIT, a saltar
		programaPAT{Numero: 55, PMTPid: 0x0abc}, // el programa de verdad
	))
	c.seccion(0x0abc, armarPMT(55, 0x0201, flujo{Tipo: TipoH264, PID: 0x0201}))
	c.carga(0x0201, 0xaa)
	r, err := analizar(t, c.bytes())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.Programa != 55 {
		t.Errorf("número de programa: %d, se esperaba 55 (la entrada 0 es la NIT)", r.Programa)
	}
	if r.PMTPid != 0x0abc {
		t.Errorf("PID de la PMT: 0x%04x, se esperaba 0x0abc", r.PMTPid)
	}
	if r.TSID != 0x1234 {
		t.Errorf("tsid: 0x%04x, se esperaba 0x1234", r.TSID)
	}
	if r.Elementales[0x0201] != TipoH264 {
		t.Errorf("la PMT anunciada por la PAT no se leyó: %v", r.Elementales)
	}
}

// La PMT no se puede leer antes que la PAT: sin PAT nadie sabe en qué PID
// viaja. Queda dicho porque no es un defecto, es el orden de las cosas, y una
// medición de un trozo de stream que empiece a mitad puede no traer tablas.
func TestSinPATNoSeLeeLaPMT(t *testing.T) {
	c := nuevoConstructor()
	c.seccion(pidPMT, armarPMT(programa, pidVideo, flujo{Tipo: TipoH264, PID: pidVideo}))
	c.carga(pidVideo, 0xaa)
	r, err := analizar(t, c.bytes())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.PMTCount != 0 || len(r.Elementales) != 0 {
		t.Errorf("se leyó una PMT que la PAT no anunció: %d tablas, %v", r.PMTCount, r.Elementales)
	}
	if r.VideoPid() != 0 || r.AudioPid() != 0 {
		t.Errorf("sin PMT no hay PID de video ni de audio: %d / %d", r.VideoPid(), r.AudioPid())
	}
}

// Cada cuánto se repite la PAT es lo que decide cuánto tarda un receptor en
// enganchar el canal. El medidor la cuenta en tiempo de PCR, no de pared.
func TestLaBrechaDeLaPATSeMideEnTiempoDePCR(t *testing.T) {
	c := nuevoConstructor()
	pat := armarPAT(tsid, programaPAT{Numero: programa, PMTPid: pidPMT})
	c.conPCR(pidVideo, 0, 0xaa)
	c.seccion(pidPAT, pat)
	c.conPCR(pidVideo, 0.1, 0xaa)
	c.seccion(pidPAT, pat) // 100 ms después de la anterior
	c.conPCR(pidVideo, 0.9, 0xaa)
	c.seccion(pidPAT, pat) // 800 ms después: la peor
	r, err := analizar(t, c.bytes())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.PATCount != 3 {
		t.Fatalf("PAT contadas: %d, se esperaban 3", r.PATCount)
	}
	if quiere := 800.0; math.Abs(r.PATMaxGapMs-quiere) > 1 {
		t.Errorf("brecha máxima de la PAT: %.1f ms, se esperaban %.0f", r.PATMaxGapMs, quiere)
	}
}

// VideoPid y AudioPid son lo que la pantalla de conexiones enseña y lo que
// F2-46/F2-114 comprueban contra lo que la persona escribió. Con dos flujos
// del mismo tipo manda el PID más bajo, que es el que el receptor toma.
func TestVideoPidYAudioPidEligenElPIDMasBajo(t *testing.T) {
	casos := []struct {
		nombre       string
		flujos       []flujo
		video, audio uint16
	}{
		{"MPEG-2 y capa II", []flujo{{TipoMPEG2Video, 0x0100}, {TipoMPEGAudio, 0x0101}}, 0x0100, 0x0101},
		{"H.264 y AAC", []flujo{{TipoH264, 0x0200}, {TipoAAC, 0x0201}}, 0x0200, 0x0201},
		{"dos audios: manda el PID más bajo", []flujo{{TipoH264, 0x0100}, {TipoAC3, 0x0300}, {TipoAAC, 0x0101}}, 0x0100, 0x0101},
		{"dos videos: manda el PID más bajo", []flujo{{TipoH264, 0x0300}, {TipoMPEG2Video, 0x0100}, {TipoAC3, 0x0400}}, 0x0100, 0x0400},
		{"solo video, el canal se quedó mudo", []flujo{{TipoMPEG2Video, 0x0100}}, 0x0100, 0},
		{"solo audio, una emisora de radio", []flujo{{TipoMPEG2Audio, 0x0100}}, 0, 0x0100},
		{"un tipo que no conocemos", []flujo{{0x06, 0x0100}}, 0, 0},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			c := nuevoConstructor()
			c.seccion(pidPAT, armarPAT(tsid, programaPAT{Numero: programa, PMTPid: pidPMT}))
			c.seccion(pidPMT, armarPMT(programa, caso.flujos[0].PID, caso.flujos...))
			r, err := analizar(t, c.bytes())
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if r.VideoPid() != caso.video {
				t.Errorf("VideoPid: 0x%04x, se esperaba 0x%04x", r.VideoPid(), caso.video)
			}
			if r.AudioPid() != caso.audio {
				t.Errorf("AudioPid: 0x%04x, se esperaba 0x%04x", r.AudioPid(), caso.audio)
			}
			if len(r.Elementales) != len(caso.flujos) {
				t.Errorf("flujos leídos: %v, se esperaban %d", r.Elementales, len(caso.flujos))
			}
		})
	}
}

// ── paquetes nulos ───────────────────────────────────────────────────

// El relleno de un TS a tasa constante son paquetes nulos, y son la mayoría
// cuando la imagen es quieta. No llevan contador, no llevan tablas, y solo se
// cuentan: si el medidor intentara leerlos como carga, el informe diría que
// hay errores de continuidad en un stream perfecto.
func TestLosPaquetesNulosSoloSeCuentan(t *testing.T) {
	c := nuevoConstructor()
	c.seccion(pidPAT, armarPAT(tsid, programaPAT{Numero: programa, PMTPid: pidPMT}))
	for i := 0; i < 300; i++ {
		c.nulo()
	}
	c.carga(pidVideo, 0xaa)
	r, err := analizar(t, c.bytes())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.NullPackets != 300 {
		t.Errorf("paquetes nulos: %d, se escribieron 300", r.NullPackets)
	}
	if r.CCErrors != 0 {
		t.Errorf("errores de continuidad: %d; los nulos no llevan contador", r.CCErrors)
	}
	if quiere := int64(302); r.Packets != quiere {
		t.Errorf("paquetes: %d, se esperaban %d", r.Packets, quiere)
	}
	// El porcentaje de relleno es lo que se lee en el informe para saber si
	// la tasa sobra: 300 de 302 es el 99,3%.
	if linea := r.String(); !strings.Contains(linea, "nulos 99.3%") {
		t.Errorf("String() no dice el porcentaje de nulos:\n%s", linea)
	}
}

// ── marcas de tiempo (F0-04) ─────────────────────────────────────────

// F0-04 pide que las marcas de tiempo de la salida sean monotónicas y sin
// saltos. Este es el único sitio del repo que lo sabe medir, así que tiene
// que contar bien las dos cosas: las que van hacia atrás y los huecos de más
// de un segundo.
func TestLasMarcasDeTiempoQueSeSalenDeOrdenSeCuentan(t *testing.T) {
	const seg = int64(90000) // 90 kHz: un segundo

	casos := []struct {
		nombre             string
		marcas             []int64
		haciaAtras, saltos int64
	}{
		{"todo en orden, cada 40 ms", []int64{0, 3600, 7200, 10800}, 0, 0},
		{"una marca retrocede", []int64{0, 3600, 1800, 5400}, 1, 0},
		{"dos marcas retroceden", []int64{0, 3600, 1800, 7200, 3600}, 2, 0},
		{"un hueco de dos segundos", []int64{0, 3600, 3600 + 2*seg}, 0, 1},
		// Justo un segundo no es un salto: el umbral es «más de un segundo».
		{"justo un segundo no es salto", []int64{0, seg}, 0, 0},
		{"un poco más de un segundo sí", []int64{0, seg + 1}, 0, 1},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			c := nuevoConstructor()
			for _, m := range caso.marcas {
				c.pes(pidVideo, m, m)
			}
			r, err := analizar(t, c.bytes())
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if r.TSNonMonotone != caso.haciaAtras {
				t.Errorf("marcas hacia atrás: %d, se esperaban %d", r.TSNonMonotone, caso.haciaAtras)
			}
			if r.TSGapsOver1s != caso.saltos {
				t.Errorf("saltos de más de 1 s: %d, se esperaban %d", r.TSGapsOver1s, caso.saltos)
			}
		})
	}
}

// Las marcas se llevan por PID: el audio y el video van cada uno a su ritmo y
// el audio adelantado del vecino no puede parecer un retroceso.
func TestLasMarcasSeLlevanPorPIDPorSeparado(t *testing.T) {
	c := nuevoConstructor()
	// Video en 0, 3600, 7200; audio en 100000, 103600, 107200. Intercalados,
	// el audio siempre por delante: si se mezclaran, cada cambio de PID
	// parecería un retroceso.
	for i := int64(0); i < 3; i++ {
		c.pes(pidVideo, i*3600, i*3600)
		c.pes(pidAudio, 100000+i*3600, -1) // el audio solo lleva PTS
	}
	r, err := analizar(t, c.bytes())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.TSNonMonotone != 0 || r.TSGapsOver1s != 0 {
		t.Fatalf("marcas: %d hacia atrás y %d saltos; cada PID va en orden por su cuenta",
			r.TSNonMonotone, r.TSGapsOver1s)
	}
}

// ── basura: nunca entrar en pánico ───────────────────────────────────

// Lo que entra por aquí llega de la red o del disco de alguien: un HDHomeRun
// que contestó una página de error, un archivo a medio copiar, un puerto
// equivocado. El medidor puede decir que no sirve, pero nunca puede reventar:
// si revienta, se lleva el aire con él.
func TestBasuraQueNoEsUnTSNoEntraEnPanico(t *testing.T) {
	limpio := streamLimpio(20)

	// Un stream con 0x47 cada 188 bytes y basura determinista en medio: es lo
	// que más ramas raras toca (campos de adaptación imposibles, secciones que
	// mienten su largo, cabeceras PES a medias) sin perder la sincronía.
	sincronizadoPeroBasura := func() []byte {
		aleatorio := rand.New(rand.NewSource(1787))
		b := make([]byte, 300*PacketSize)
		aleatorio.Read(b)
		for i := 0; i < len(b); i += PacketSize {
			b[i] = 0x47
		}
		return b
	}()

	// Una PAT que dice ocupar más de lo que cabe en el paquete: la sección se
	// salta y el programa se queda sin leer, pero nada revienta.
	seccionQueMiente := func() []byte {
		c := nuevoConstructor()
		pat := armarPAT(tsid, programaPAT{Numero: programa, PMTPid: pidPMT})
		pat[1] = 0xb3 // section_length con los bits altos: pasa de 900 bytes
		c.seccion(pidPAT, pat)
		c.carga(pidVideo, 0xaa)
		return c.bytes()
	}()

	// Un campo de adaptación que se pasa del paquete entero.
	adaptacionImposible := func() []byte {
		c := nuevoConstructor()
		p := cabecera(pidVideo, true, 3, 0)
		p[4] = 0xff // 255 bytes de adaptación en un paquete de 188
		p[5] = 0x10 // y encima dice llevar PCR
		c.crudo(p)
		return c.bytes()
	}()

	casos := []struct {
		nombre      string
		datos       []byte
		quiereError bool
		quierePkts  int64
	}{
		{"nada", nil, false, 0},
		{"ceros", bytes.Repeat([]byte{0x00}, 3*PacketSize), true, 1},
		{"unos", bytes.Repeat([]byte{0xff}, 3*PacketSize), true, 1},
		{"una página de error HTML, más corta que un paquete", []byte("<html><body>404 no encontrado</body></html>"), false, 0},
		{"187 bytes: un paquete al que le falta uno", bytes.Repeat([]byte{0x47}, PacketSize-1), false, 0},
		{"un TS bueno con 100 bytes de cola", append(append([]byte{}, limpio...), bytes.Repeat([]byte{0x47}, 100)...), false, int64(len(limpio) / PacketSize)},
		{"un TS bueno que pierde la sincronía a mitad", append(append([]byte{}, limpio...), bytes.Repeat([]byte{0x13}, PacketSize)...), true, int64(len(limpio)/PacketSize) + 1},
		{"basura sincronizada cada 188", sincronizadoPeroBasura, false, 300},
		{"una sección PSI que miente su largo", seccionQueMiente, false, 2},
		{"un campo de adaptación imposible", adaptacionImposible, false, 1},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			r, err := analizar(t, caso.datos)
			if caso.quiereError && err == nil {
				t.Errorf("se esperaba error y no hubo; informe: %s", r)
			}
			if !caso.quiereError && err != nil {
				t.Errorf("no se esperaba error: %v", err)
			}
			if r.Packets != caso.quierePkts {
				t.Errorf("paquetes contados: %d, se esperaban %d", r.Packets, caso.quierePkts)
			}
			// Aunque no haya error, el informe tiene que poder imprimirse: es
			// lo que se pega en la bitácora cuando una medición sale mal.
			_ = r.String()
		})
	}
}

// Un TS que no arranca con el byte de sincronía se rechaza en el primer
// paquete y lo dice con el número de paquete, que es lo que hace falta para
// entender un volcado.
func TestElByteDeSincroniaSeExigeYSeDiceDonde(t *testing.T) {
	datos := streamLimpio(20)
	datos[5*PacketSize] = 0x00 // el sexto paquete pierde la sincronía
	r, err := analizar(t, datos)
	if err == nil {
		t.Fatal("se esperaba error por el paquete sin byte de sincronía")
	}
	if !strings.Contains(err.Error(), "sincronía") {
		t.Errorf("el error no habla de sincronía: %v", err)
	}
	if r.Packets != 6 {
		t.Errorf("paquetes contados antes de fallar: %d, se esperaban 6", r.Packets)
	}
	// Lo medido hasta el fallo se devuelve igual: sirve para saber hasta dónde
	// llegó bien el stream.
	if r.Programa != programa {
		t.Errorf("el informe parcial perdió el programa: %d", r.Programa)
	}
}

// lectorQueFalla devuelve unos bytes buenos y después un error de E/S: es el
// socket que se cae a mitad de una medición, no un TS mal formado.
type lectorQueFalla struct {
	datos []byte
	pos   int
	err   error
}

func (l *lectorQueFalla) Read(p []byte) (int, error) {
	if l.pos >= len(l.datos) {
		return 0, l.err
	}
	n := copy(p, l.datos[l.pos:])
	l.pos += n
	return n, nil
}

// El error de la lectura se devuelve tal cual, sin disfrazarlo de TS
// inválido: quien mide necesita distinguir «el cable se cayó» de «esto no es
// un transport stream».
func TestElErrorDeLecturaSubeTalCual(t *testing.T) {
	miErr := errors.New("el cable se cayó")
	l := &lectorQueFalla{datos: streamLimpio(5), err: miErr}
	r, err := Analyze(l)
	if !errors.Is(err, miErr) {
		t.Fatalf("se esperaba el error de lectura, llegó: %v", err)
	}
	if r.Packets == 0 {
		t.Error("lo medido antes del error se perdió")
	}
}

// Un EOF a mitad de paquete es un archivo truncado, no un error: se para y se
// devuelve lo medido. Es el caso de una grabación que se cortó.
func TestUnPaqueteAMediasTerminaSinError(t *testing.T) {
	datos := append(streamLimpio(5), 0x47, 0x00, 0x00)
	r, err := analizar(t, datos)
	if err != nil {
		t.Fatalf("un paquete truncado al final no es un error: %v", err)
	}
	if quiere := int64(len(datos) / PacketSize); r.Packets != quiere {
		t.Errorf("paquetes: %d, se esperaban %d (los tres bytes de cola no cuentan)", r.Packets, quiere)
	}
}

// ── secciones y cabeceras que no son lo que se espera ────────────────

// En el PID 0 y en el de la PMT puede aparecer cualquier cosa: otra tabla que
// no nos toca, una sección cortada, un puntero que apunta fuera del paquete.
// El medidor tiene que dejarlas pasar sin quedarse con datos a medias: una
// tabla mal leída pondría un número de programa inventado en la pantalla de
// conexiones, que es peor que no poner ninguno.
func TestLasSeccionesQueNoTocanSeDejanPasar(t *testing.T) {
	// otraTablaEnElPIDCero: una sección con otro table_id en el PID de la PAT.
	otraTabla := func() []byte {
		c := nuevoConstructor()
		sec := armarPAT(tsid, programaPAT{Numero: programa, PMTPid: pidPMT})
		sec[0] = 0x40 // ya no es una PAT
		c.seccion(pidPAT, sec)
		return c.bytes()
	}()

	// seccionCorta: una sección que dice medir dos bytes. Cabe en el paquete,
	// así que se lee, pero no llega ni al número de programa.
	seccionCorta := func() []byte {
		c := nuevoConstructor()
		c.seccion(pidPAT, []byte{0x00, 0xb0, 0x02, 0x00, 0x00})
		return c.bytes()
	}()

	// punteroFuera: el pointer_field manda la sección más allá del paquete.
	punteroFuera := func() []byte {
		c := nuevoConstructor()
		p := cabecera(pidPAT, true, 1, 0)
		p[4] = 200 // saltar 200 bytes dentro de un paquete de 188
		c.crudo(p)
		return c.bytes()
	}()

	// pmtSinFlujos: una PMT válida que no declara ningún flujo. Pasa cuando el
	// canal se acaba de dar de alta y todavía no hay nada dentro.
	pmtSinFlujos := func() []byte {
		c := nuevoConstructor()
		c.seccion(pidPAT, armarPAT(tsid, programaPAT{Numero: programa, PMTPid: pidPMT}))
		c.seccion(pidPMT, armarPMT(programa, pidVideo))
		return c.bytes()
	}()

	// otraTablaEnElPIDDeLaPMT: la PAT anuncia el PID, pero ahí viaja otra cosa.
	otraTablaEnLaPMT := func() []byte {
		c := nuevoConstructor()
		c.seccion(pidPAT, armarPAT(tsid, programaPAT{Numero: programa, PMTPid: pidPMT}))
		sec := armarPMT(programa, pidVideo, flujo{Tipo: TipoH264, PID: pidVideo})
		sec[0] = 0x42 // una SDT donde debía ir la PMT
		c.seccion(pidPMT, sec)
		return c.bytes()
	}()

	// patSinProgramas: solo la entrada de la tabla de red.
	patSinProgramas := func() []byte {
		c := nuevoConstructor()
		c.seccion(pidPAT, armarPAT(tsid))
		return c.bytes()
	}()

	casos := []struct {
		nombre        string
		datos         []byte
		programa, pmt uint16
		flujos        int
	}{
		{"otro table_id en el PID 0", otraTabla, 0, 0, 0},
		{"una sección de dos bytes", seccionCorta, 0, 0, 0},
		{"el puntero apunta fuera del paquete", punteroFuera, 0, 0, 0},
		{"una PAT sin programas, solo la NIT", patSinProgramas, 0, 0, 0},
		{"una PMT sin flujos", pmtSinFlujos, programa, pidPMT, 0},
		{"otra tabla en el PID que la PAT anunció", otraTablaEnLaPMT, programa, pidPMT, 0},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			r, err := analizar(t, caso.datos)
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			if r.Programa != caso.programa {
				t.Errorf("número de programa: %d, se esperaba %d", r.Programa, caso.programa)
			}
			if r.PMTPid != caso.pmt {
				t.Errorf("PID de la PMT: %d, se esperaba %d", r.PMTPid, caso.pmt)
			}
			if len(r.Elementales) != caso.flujos {
				t.Errorf("flujos leídos: %v, se esperaban %d", r.Elementales, caso.flujos)
			}
		})
	}
}

// Un arranque de PES que cae en los últimos bytes del paquete —detrás de un
// campo de adaptación largo— no deja sitio para la marca de tiempo. El medidor
// tiene que no leer nada en vez de leer lo que haya detrás.
func TestUnPESSinSitioParaLaMarcaNoSeLee(t *testing.T) {
	c := nuevoConstructor()
	// Primero una marca buena, para que haya con qué comparar.
	c.pes(pidVideo, 90000, 90000)

	// Y ahora el paquete raro: 169 bytes de adaptación dejan la carga en el
	// byte 174, justo donde el prefijo de PES cabe y la marca ya no.
	p := cabecera(pidVideo, true, 3, c.siguienteCC(pidVideo))
	p[4] = 169
	const carga = 4 + 1 + 169
	p[carga], p[carga+1], p[carga+2] = 0x00, 0x00, 0x01
	p[carga+7] = 0xc0 // dice llevar PTS y DTS
	c.crudo(p)

	r, err := analizar(t, c.bytes())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.TSNonMonotone != 0 || r.TSGapsOver1s != 0 {
		t.Fatalf("se inventó una marca donde no cabía: %d hacia atrás, %d saltos",
			r.TSNonMonotone, r.TSGapsOver1s)
	}
}

// Una tabla que no cabe en un paquete sigue en el siguiente, y ese segundo
// paquete no lleva la marca de arranque de unidad. El medidor no puede contarlo
// como una tabla nueva: si lo hiciera, la cuenta de PAT y la brecha entre
// repeticiones —lo que decide cuánto tarda un receptor en enganchar— saldrían
// mal.
func TestLaContinuacionDeUnaTablaNoEsUnaTablaNueva(t *testing.T) {
	c := nuevoConstructor()
	c.seccion(pidPAT, armarPAT(tsid, programaPAT{Numero: programa, PMTPid: pidPMT}))
	c.carga(pidPAT, 0x00) // continuación: mismo PID, sin marca de arranque
	c.carga(pidPAT, 0x00)

	r, err := analizar(t, c.bytes())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if r.PATCount != 1 {
		t.Fatalf("PAT contadas: %d; solo una empieza de verdad", r.PATCount)
	}
	if r.Programa != programa || r.PMTPid != pidPMT {
		t.Errorf("la continuación estropeó lo leído: programa %d, PMT en %d", r.Programa, r.PMTPid)
	}
}
