package engine

import (
	"math"
	"testing"
	"time"
)

// Pruebas del detector de silencio y negro sobre la salida (T3 de F2). Corren
// sin ffmpeg y sin encoder: los cuadros se fabrican a mano y el destino es un
// recolector en memoria. Así se mide cuadro a cuadro lo que en la salida real
// tardaría veinte segundos por caso.

// cuadroDeLuma fabrica un cuadro yuv420p con toda la imagen a la misma luma y
// el croma en gris (128), que es lo que un cuadro sin color tiene.
func cuadroDeLuma(f Format, luma byte) []byte {
	fr := make([]byte, f.FrameBytes())
	n := f.Width * f.Height
	for i := 0; i < n; i++ {
		fr[i] = luma
	}
	for i := n; i < len(fr); i++ {
		fr[i] = 128
	}
	return fr
}

// audioDeNivel fabrica un bloque de PCM s16le con todas las muestras en la
// misma amplitud: el RMS del bloque es esa amplitud, así que el nivel en
// dBFS sale exacto y la prueba no depende de una forma de onda.
func audioDeNivel(f Format, muestras int, amplitud int16) []byte {
	pcm := make([]byte, muestras*f.BytesPerSample())
	for i := 0; i+1 < len(pcm); i += 2 {
		pcm[i] = byte(uint16(amplitud))
		pcm[i+1] = byte(uint16(amplitud) >> 8)
	}
	return pcm
}

// amplitudDe es la amplitud constante que da ese nivel en dBFS.
func amplitudDe(dbfs float64) int16 {
	return int16(math.Round(math.Pow(10, dbfs/20) * 32768))
}

// detectorDePrueba deja un detector con el formato chico de las pruebas del
// conformado y un recolector detrás.
func detectorDePrueba(t *testing.T) (*Detector, *recolector) {
	t.Helper()
	rec := &recolector{}
	d := NuevoDetector(formatoDePrueba, rec)
	d.Origen = time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	return d, rec
}

// emite escribe segundos de salida con esa luma y ese nivel de audio.
func emite(t *testing.T, d *Detector, segundos float64, luma byte, amplitud int16) {
	t.Helper()
	cuadros := int(math.Round(segundos * d.Formato.FPS()))
	fr := cuadroDeLuma(d.Formato, luma)
	for i := 0; i < cuadros; i++ {
		n := int(d.Formato.SamplesUpTo(d.cuadros+1) - d.Formato.SamplesUpTo(d.cuadros))
		if err := d.WriteFrame(fr); err != nil {
			t.Fatalf("el detector no pasó el cuadro: %v", err)
		}
		if err := d.WriteAudio(audioDeNivel(d.Formato, n, amplitud)); err != nil {
			t.Fatalf("el detector no pasó el audio: %v", err)
		}
	}
}

// recoge vacía el canal de estados.
func recoge(d *Detector) []Estado {
	var out []Estado
	for {
		select {
		case e := <-d.Estados():
			out = append(out, e)
		default:
			return out
		}
	}
}

// deTipo son los estados de un tipo.
func deTipo(estados []Estado, tipo string) []Estado {
	var out []Estado
	for _, e := range estados {
		if e.Tipo == tipo {
			out = append(out, e)
		}
	}
	return out
}

// El cuadro negro abre episodio y el cuadro con imagen lo cierra, con la
// duración contada en cuadros que de verdad salieron.
func TestDetectorNegroEmpiezaYTermina(t *testing.T) {
	d, _ := detectorDePrueba(t)

	emite(t, d, 2, 16, 3000) // dos segundos en negro de verdad (16 = negro digital), con sonido
	emite(t, d, 1, 90, 3000) // un segundo con imagen

	estados := recoge(d)
	if n := len(deTipo(estados, NegroEmpieza)); n != 1 {
		t.Fatalf("el negro empezó %d veces y tenía que empezar una: %v", n, estados)
	}
	fin := deTipo(estados, NegroTermina)
	if len(fin) != 1 {
		t.Fatalf("el negro terminó %d veces: %v", len(fin), estados)
	}
	if d := fin[0].Duro; d < 1900*time.Millisecond || d > 2100*time.Millisecond {
		t.Errorf("el negro duró %s y salieron dos segundos de negro", d)
	}
	// Y nada de silencio: había sonido todo el rato.
	if n := len(deTipo(estados, SilencioEmpieza)); n != 0 {
		t.Errorf("se contó silencio con el audio a −20 dBFS: %v", estados)
	}
}

// El silencio se mide igual, y no depende de que haya o no imagen: un archivo
// puede salir perfecto de imagen y mudo por un desajuste de pista.
func TestDetectorSilencioEmpiezaYTermina(t *testing.T) {
	d, _ := detectorDePrueba(t)

	emite(t, d, 3, 120, amplitudDe(-70)) // tres segundos mudos, con imagen
	emite(t, d, 1, 120, amplitudDe(-20))

	estados := recoge(d)
	if n := len(deTipo(estados, SilencioEmpieza)); n != 1 {
		t.Fatalf("el silencio empezó %d veces: %v", n, estados)
	}
	fin := deTipo(estados, SilencioTermina)
	if len(fin) != 1 {
		t.Fatalf("el silencio terminó %d veces: %v", len(fin), estados)
	}
	if d := fin[0].Duro; d < 2900*time.Millisecond || d > 3100*time.Millisecond {
		t.Errorf("el silencio duró %s y salieron tres segundos mudos", d)
	}
	if n := len(deTipo(estados, NegroEmpieza)); n != 0 {
		t.Errorf("se contó negro con la luma a 120: %v", estados)
	}
	// Y el silencio digital —el que escribe el servidor de cuadros cuando el
	// archivo se queda corto de audio— también es silencio.
	if err := d.WriteAudio(make([]byte, 1600*d.Formato.BytesPerSample())); err != nil {
		t.Fatalf("el silencio digital no pasó: %v", err)
	}
	if n := len(deTipo(recoge(d), SilencioEmpieza)); n != 1 {
		t.Error("el silencio digital (todo ceros) no se contó como silencio")
	}
}

// cuadroDestapado fabrica un cuadro negro con esa parte de las filas de
// arriba encendidas: sirve para quedarse justo entre el 98 % que abre un
// episodio de negro y el 94 % que lo cierra.
func cuadroDestapado(f Format, parte float64) []byte {
	fr := cuadroDeLuma(f, 16)
	filas := int(float64(f.Height) * parte)
	for y := 0; y < filas; y++ {
		for x := 0; x < f.Width; x++ {
			fr[y*f.Width+x] = 200
		}
	}
	return fr
}

// escribe pone el mismo cuadro tantas veces, con sonido, sin fabricar nada.
func escribe(t *testing.T, d *Detector, veces int, fr []byte) {
	t.Helper()
	for i := 0; i < veces; i++ {
		if err := d.WriteFrame(fr); err != nil {
			t.Fatalf("el detector no pasó el cuadro: %v", err)
		}
		if err := d.WriteAudio(audioDeNivel(d.Formato, 1600, 3000)); err != nil {
			t.Fatalf("el detector no pasó el audio: %v", err)
		}
	}
}

// La histéresis: una imagen que se destapa justo sobre el umbral no abre y
// cierra episodios. Con el 3 % destapado —el 97 % en negro— no se abre nada,
// pero un episodio ya abierto no se cierra: para eso hay que llegar al 6 %.
func TestDetectorHisteresisNoParpadea(t *testing.T) {
	d, _ := detectorDePrueba(t)
	casi := cuadroDestapado(d.Formato, 0.03)    // 97 % en negro
	bastante := cuadroDestapado(d.Formato, 0.1) // 90 % en negro

	// Desde imagen, el 97 % en negro no llega a ser negro.
	escribe(t, d, 30, casi)
	if estados := recoge(d); len(deTipo(estados, NegroEmpieza)) != 0 {
		t.Fatalf("el 97 %% en negro abrió episodio y hace falta el 98 %%: %v", estados)
	}
	// Con negro de verdad se abre…
	emite(t, d, 1, 16, 3000)
	if estados := recoge(d); len(deTipo(estados, NegroEmpieza)) != 1 {
		t.Fatal("el negro de verdad no abrió episodio")
	}
	// …y el 97 %% no lo cierra: la histéresis aguanta hasta el 94 %.
	escribe(t, d, 30, casi)
	if estados := recoge(d); len(deTipo(estados, NegroTermina)) != 0 {
		t.Fatalf("el 97 %% en negro cerró el episodio y la histéresis llega al 94 %%: %v", estados)
	}
	escribe(t, d, 30, bastante)
	if estados := recoge(d); len(deTipo(estados, NegroTermina)) != 1 {
		t.Fatalf("el 90 %% en negro tenía que cerrar el episodio: %v", estados)
	}

	// Y al revés con el audio: −58 dBFS está sobre −60 pero dentro de los
	// tres decibelios de histéresis.
	d2, _ := detectorDePrueba(t)
	emite(t, d2, 1, 120, amplitudDe(-70))
	emite(t, d2, 1, 120, amplitudDe(-58))
	if estados := recoge(d2); len(deTipo(estados, SilencioTermina)) != 0 {
		t.Fatalf("−58 dBFS cerró el silencio y la histéresis llega a −57: %v", estados)
	}
	emite(t, d2, 1, 120, amplitudDe(-50))
	if estados := recoge(d2); len(deTipo(estados, SilencioTermina)) != 1 {
		t.Fatal("−50 dBFS tenía que cerrar el silencio")
	}
}

// El latido cuenta cuánto lleva el episodio, en tiempo de salida: es con esto
// con lo que la vigilancia mide el umbral, no con la hora de pared.
func TestDetectorLatidoCuentaLoQueLleva(t *testing.T) {
	d, _ := detectorDePrueba(t)
	emite(t, d, 5, 16, amplitudDe(-70))

	sigue := deTipo(recoge(d), NegroSigue)
	if len(sigue) < 4 {
		t.Fatalf("cinco segundos de negro dieron %d latidos y tenían que dar cuatro o cinco", len(sigue))
	}
	ultimo := sigue[len(sigue)-1]
	if ultimo.Duro < 4*time.Second || ultimo.Duro > 5*time.Second {
		t.Errorf("el último latido dice %s y el episodio llevaba cinco segundos", ultimo.Duro)
	}
	// El instante es el del reloj propio, no time.Now.
	if !ultimo.Cuando.After(d.Origen) {
		t.Errorf("el latido dice que pasó en %s y el origen es %s", ultimo.Cuando, d.Origen)
	}
}

// El detector es transparente: al encoder le llega exactamente lo que le
// llegaba antes, byte por byte.
func TestDetectorPasaLaSalidaTalCual(t *testing.T) {
	d, rec := detectorDePrueba(t)
	emite(t, d, 1, 40, 3000)

	if len(rec.cuadros) != 30 {
		t.Fatalf("al encoder le llegaron %d cuadros de 30", len(rec.cuadros))
	}
	if luma(rec.cuadros[0], formatoDePrueba) != 40 {
		t.Errorf("el cuadro llegó cambiado: luma %.1f", luma(rec.cuadros[0], formatoDePrueba))
	}
	if pico(rec.audio) != 3000 {
		t.Errorf("el audio llegó cambiado: pico %d", pico(rec.audio))
	}
}

// El submuestreo tiene que mirar toda la imagen: una columna encendida sobre
// negro sigue siendo negro, y una imagen a rayas no es negro. Es la prueba de
// que el salto no cae siempre en la misma columna.
func TestDetectorSubmuestreoMiraTodaLaImagen(t *testing.T) {
	d, _ := detectorDePrueba(t)
	f := d.Formato

	// Negro con una sola columna blanca: negro.
	fr := cuadroDeLuma(f, 16)
	for y := 0; y < f.Height; y++ {
		fr[y*f.Width+f.Width/2] = 255
	}
	if _, parte := d.mirarLuma(fr); parte < ParteNegra {
		t.Errorf("una columna blanca sobre negro deja el %.0f %% en negro y tenía que seguir siendo negro", parte*100)
	}

	// A rayas horizontales, una de cada dos filas encendida: no es negro.
	rayas := cuadroDeLuma(f, 16)
	for y := 0; y < f.Height; y += 2 {
		for x := 0; x < f.Width; x++ {
			rayas[y*f.Width+x] = 200
		}
	}
	media, parte := d.mirarLuma(rayas)
	if parte >= ParteNegra {
		t.Errorf("una imagen a rayas deja el %.0f %% en negro: el salto está mirando una sola fila", parte*100)
	}
	if media < 100 {
		t.Errorf("la luma media de la imagen a rayas es %.1f: el salto no cruza las filas", media)
	}
}

// El negro de verdad de un video en rango limitado es luma 16, no 0: si el
// umbral fuera «bajo 16» no dispararía nunca. Esta prueba es el candado de esa
// decisión (ver el comentario de PixelNegro).
func TestDetectorElNegroDigitalEsDieciseis(t *testing.T) {
	d, _ := detectorDePrueba(t)
	media, parte := d.mirarLuma(cuadroDeLuma(d.Formato, 16))
	if media != 16 {
		t.Fatalf("la luma media del negro digital salió %.3f", media)
	}
	if parte != 1 {
		t.Fatalf("el negro digital dejó el %.0f %% de la imagen bajo el umbral", parte*100)
	}
}

// El aire nunca espera al vigilante: si nadie lee los estados, se tiran y se
// cuentan, y escribir un cuadro sigue costando lo mismo.
func TestDetectorNoBloqueaElAireSiNadieMira(t *testing.T) {
	d, _ := detectorDePrueba(t)
	// Muchos episodios cortos, sin leer nada: más de EstadosEnCola cambios.
	for i := 0; i < EstadosEnCola*2; i++ {
		emite(t, d, 0.1, 16, amplitudDe(-70))
		emite(t, d, 0.1, 120, 3000)
	}
	if d.Perdidos() == 0 {
		t.Fatal("con el canal lleno tenían que perderse estados, no bloquearse el aire")
	}
	if d.Cuadros() == 0 || d.MuestrasVistas() == 0 {
		t.Fatal("el detector dejó de contar")
	}
}
