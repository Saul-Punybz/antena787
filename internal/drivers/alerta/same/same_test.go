package same

import (
	"math"
	"testing"
	"time"
)

// Pruebas del decodificador SAME. El audio se fabrica aquí mismo con el
// modulador de modulador_test.go —que no existe fuera de las pruebas, ver el
// aviso de ese archivo y el ADR 0010— y se le mete al decodificador en bloques,
// como llega el audio de un retorno de aire.

// Las cabeceras de prueba. La prueba semanal de una estación de Puerto Rico:
// FIPS 72 es Puerto Rico y 000 es «todo el territorio».
const (
	cabRWT = "ZCZC-EAS-RWT-072000+0015-2531200-WPRT/TV -"
	cabTOR = "ZCZC-WXR-TOR-072021-072127-072113+0030-2531215-KSJT/NWS-"
)

// unaHora es una hora fija para que Detectado sea comparable.
var unaHora = time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

func relojFijo() time.Time { return unaHora }

// correr le mete el audio al decodificador en bloques de 4096 bytes —dos mil y
// pico de muestras, lo que trae una tubería de audio— y devuelve todo lo que
// salió por el canal.
func correr(t *testing.T, muestras []float64, o Opciones) []Evento {
	t.Helper()
	if o.Reloj == nil {
		o.Reloj = relojFijo
	}
	if o.Buffer == 0 {
		o.Buffer = 64
	}
	d, err := Nuevo(o)
	if err != nil {
		t.Fatalf("Nuevo: %v", err)
	}
	var evs []Evento
	listo := make(chan struct{})
	go func() {
		for ev := range d.Eventos() {
			evs = append(evs, ev)
		}
		close(listo)
	}()
	pcm := aPCM(muestras)
	for i := 0; i < len(pcm); i += 4096 {
		j := i + 4096
		if j > len(pcm) {
			j = len(pcm)
		}
		if _, err := d.Escribir(pcm[i:j]); err != nil {
			t.Fatalf("Escribir: %v", err)
		}
	}
	if err := d.Cerrar(); err != nil {
		t.Fatalf("Cerrar: %v", err)
	}
	<-listo
	if n := d.Descartados(); n != 0 {
		t.Fatalf("se descartaron %d eventos: nadie los estaba leyendo", n)
	}
	return evs
}

func cabecerasDe(evs []Evento) []*Cabecera {
	var out []*Cabecera
	for _, e := range evs {
		if e.Tipo == TipoCabecera {
			out = append(out, e.Cabecera)
		}
	}
	return out
}

func finesDe(evs []Evento) []*FinDeMensaje {
	var out []*FinDeMensaje
	for _, e := range evs {
		if e.Tipo == TipoFin {
			out = append(out, e.Fin)
		}
	}
	return out
}

func atencionesDe(evs []Evento) []*SenalDeAtencion {
	var out []*SenalDeAtencion
	for _, e := range evs {
		if e.Tipo == TipoAtencion {
			out = append(out, e.Atencion)
		}
	}
	return out
}

// unaCabecera exige exactamente una cabecera y la devuelve.
func unaCabecera(t *testing.T, evs []Evento) *Cabecera {
	t.Helper()
	cs := cabecerasDe(evs)
	if len(cs) != 1 {
		t.Fatalf("se esperaba 1 cabecera, salieron %d", len(cs))
	}
	return cs[0]
}

func TestRWTLimpio(t *testing.T) {
	g := nuevoGenerador(48000, 0.5)
	g.mensajeCompleto(cabRWT, 0, 0.5)
	evs := correr(t, g.salida, Opciones{})

	c := unaCabecera(t, evs)
	if c.Crudo != cabRWT {
		t.Errorf("crudo = %q, se esperaba %q", c.Crudo, cabRWT)
	}
	if c.Org != "EAS" || c.Evento != "RWT" {
		t.Errorf("org/evento = %q/%q", c.Org, c.Evento)
	}
	if len(c.Zonas) != 1 || c.Zonas[0] != "072000" {
		t.Errorf("zonas = %v", c.Zonas)
	}
	if c.Duracion != 15*time.Minute {
		t.Errorf("duración = %v", c.Duracion)
	}
	if c.Instante != "2531200" {
		t.Errorf("instante = %q", c.Instante)
	}
	if c.Llamada != "WPRT/TV" {
		t.Errorf("llamada = %q (los espacios de relleno se quitan)", c.Llamada)
	}
	if c.Repeticiones != 3 || c.Confianza != 1 {
		t.Errorf("repeticiones = %d, confianza = %v; se esperaban 3 y 1", c.Repeticiones, c.Confianza)
	}
	if !c.Detectado.Equal(unaHora) {
		t.Errorf("detectado = %v", c.Detectado)
	}
	// La primera cabecera empieza tras 0.2 s de silencio y dura unos 0.83 s.
	if c.Desplazamiento < 900*time.Millisecond || c.Desplazamiento > 1300*time.Millisecond {
		t.Errorf("desplazamiento = %v, fuera de lo razonable", c.Desplazamiento)
	}

	f := finesDe(evs)
	if len(f) != 1 {
		t.Fatalf("se esperaba 1 fin de mensaje, salieron %d", len(f))
	}
	if f[0].Crudo != "NNNN" || f[0].Repeticiones != 3 {
		t.Errorf("fin = %+v", f[0])
	}
	if f[0].Desplazamiento <= c.Desplazamiento {
		t.Errorf("el fin de mensaje no puede ir antes de la cabecera")
	}
}

func TestTORConTresZonas(t *testing.T) {
	g := nuevoGenerador(48000, 0.5)
	g.mensajeCompleto(cabTOR, 0, 0.5)
	c := unaCabecera(t, correr(t, g.salida, Opciones{}))

	if c.Org != "WXR" || c.Evento != "TOR" {
		t.Errorf("org/evento = %q/%q", c.Org, c.Evento)
	}
	esperadas := []string{"072021", "072127", "072113"}
	if len(c.Zonas) != 3 {
		t.Fatalf("zonas = %v", c.Zonas)
	}
	for i, z := range esperadas {
		if c.Zonas[i] != z {
			t.Errorf("zona %d = %q, se esperaba %q", i, c.Zonas[i], z)
		}
	}
	if c.Duracion != 30*time.Minute {
		t.Errorf("duración = %v", c.Duracion)
	}
	if c.Llamada != "KSJT/NWS" {
		t.Errorf("llamada = %q", c.Llamada)
	}
}

func TestRuidoBlanco(t *testing.T) {
	for _, snr := range []float64{20, 6} {
		g := nuevoGenerador(48000, 0.5)
		g.mensajeCompleto(cabRWT, 0, 0.5)
		evs := correr(t, conRuido(g.salida, snr, 7), Opciones{})
		c := unaCabecera(t, evs)
		if c.Crudo != cabRWT {
			t.Errorf("con ruido a %.0f dB: crudo = %q", snr, c.Crudo)
		}
		if len(finesDe(evs)) != 1 {
			t.Errorf("con ruido a %.0f dB: no salió el fin de mensaje", snr)
		}
	}
}

// TestLimiteDeRuido baja la relación señal/ruido hasta que el decodificador
// deja de leer la cabecera, y deja el número en el registro de la prueba. Es la
// medida que interesa de verdad: el filtro adaptado gana unos 16 dB —la
// ventana de un bit escucha 520 Hz de los 24 kHz que trae el audio—, así que
// tiene que aguantar bastante por debajo de cero.
func TestLimiteDeRuido(t *testing.T) {
	const intentos = 10
	limite, ultimaConAlgo := 99.0, 99.0
	for snr := 20.0; snr >= -22.0; snr -= 2 {
		bien, falsas := 0, 0
		for semilla := int64(1); semilla <= intentos; semilla++ {
			g := nuevoGenerador(48000, 0.5)
			g.mensajeCompleto(cabRWT, 0, 0.2)
			evs := correr(t, conRuido(g.salida, snr, semilla), Opciones{})
			cs := cabecerasDe(evs)
			switch {
			case len(cs) == 1 && cs[0].Crudo == cabRWT:
				bien++
			case len(cs) > 0:
				falsas++
			}
		}
		t.Logf("SNR %+5.0f dB: %2d de %d cabeceras enteras, %d equivocadas", snr, bien, intentos, falsas)
		if bien == intentos {
			limite = snr
		}
		if bien > 0 {
			ultimaConAlgo = snr
		}
	}
	t.Logf("las lee todas hasta %.0f dB; alguna todavía sale a %.0f dB", limite, ultimaConAlgo)
	if limite > 6 {
		t.Errorf("la cabecera solo se lee de %.0f dB para arriba; se esperaba aguantar al menos 6 dB", limite)
	}
}

// TestTasaCorrida es el caso de una grabación hecha a 47.9 kHz y leída como si
// fuera de 48: el bit dura un 0.2 % menos de lo que el decodificador espera, y
// a lo largo de una cabecera eso es más de medio bit. Lo salva el lazo de
// reloj, que además del desfase corrige el largo del bit.
func TestTasaCorrida(t *testing.T) {
	g := nuevoGenerador(47900, 0.5)
	g.mensajeCompleto(cabRWT, 0, 0.5)
	evs := correr(t, g.salida, Opciones{Tasa: 48000})
	c := unaCabecera(t, evs)
	if c.Crudo != cabRWT {
		t.Errorf("crudo = %q", c.Crudo)
	}
	if len(finesDe(evs)) != 1 {
		t.Error("no salió el fin de mensaje")
	}
}

// TestNivelBajo es una alerta que llega floja: -30 dBFS, que es lo que se ve
// cuando el retorno entra por una toma de línea mal ajustada.
func TestNivelBajo(t *testing.T) {
	amp := math.Pow(10, -30.0/20) // -30 dBFS
	g := nuevoGenerador(48000, amp)
	g.mensajeCompleto(cabRWT, 0, 0.5)
	c := unaCabecera(t, correr(t, g.salida, Opciones{}))
	if c.Crudo != cabRWT {
		t.Errorf("crudo = %q", c.Crudo)
	}
}

// TestSilencio: un minuto de silencio digital no es una alerta.
func TestSilencio(t *testing.T) {
	evs := correr(t, make([]float64, 48000*60), Opciones{DetectarAtencion: true})
	if len(evs) != 0 {
		t.Fatalf("60 s de silencio dieron %d eventos: %+v", len(evs), evs)
	}
}

// TestMusicaSinSAME: un minuto de programación normal tampoco. Cero falsos
// positivos, que es la única cifra aceptable: una alerta inventada en el as-run
// es peor que un hueco.
func TestMusicaSinSAME(t *testing.T) {
	evs := correr(t, musica(48000, 60, 3), Opciones{DetectarAtencion: true})
	if len(evs) != 0 {
		t.Fatalf("60 s de música dieron %d eventos: %+v", len(evs), evs)
	}
}

// TestFinDeMensajeSolo: el NNNN se lee aunque no se haya oído la cabecera (un
// retorno que se enchufó a media alerta).
func TestFinDeMensajeSolo(t *testing.T) {
	g := nuevoGenerador(48000, 0.5)
	g.silencio(0.2)
	g.tresVeces("NNNN")
	evs := correr(t, g.salida, Opciones{})
	if len(cabecerasDe(evs)) != 0 {
		t.Errorf("salió una cabecera de la nada")
	}
	f := finesDe(evs)
	if len(f) != 1 || f[0].Repeticiones != 3 {
		t.Fatalf("fines = %+v", f)
	}
}

// TestUnaSolaRepeticionLimpia: con una sola copia limpia alcanza, y la
// confianza lo dice (1 de 3).
func TestUnaSolaRepeticionLimpia(t *testing.T) {
	g := nuevoGenerador(48000, 0.5)
	g.silencio(0.2)
	g.rafaga(cabRWT)
	g.silencio(1)
	c := unaCabecera(t, correr(t, g.salida, Opciones{}))
	if c.Crudo != cabRWT {
		t.Errorf("crudo = %q", c.Crudo)
	}
	if c.Repeticiones != 1 {
		t.Errorf("repeticiones = %d", c.Repeticiones)
	}
	if math.Abs(c.Confianza-1.0/3) > 1e-9 {
		t.Errorf("confianza = %v, se esperaba 1/3", c.Confianza)
	}
}

// TestVotoPorMayoria: una de las tres copias llegó rota. Gana la mayoría y la
// confianza baja a 2 de 3.
func TestVotoPorMayoria(t *testing.T) {
	rota := []byte(cabRWT)
	rota[15] = 'X' // un dígito de la zona
	g := nuevoGenerador(48000, 0.5)
	g.silencio(0.2)
	g.rafaga(cabRWT)
	g.silencio(1)
	g.rafaga(string(rota))
	g.silencio(1)
	g.rafaga(cabRWT)
	g.silencio(1)

	c := unaCabecera(t, correr(t, g.salida, Opciones{}))
	if c.Crudo != cabRWT {
		t.Errorf("crudo = %q, se esperaba la mayoría %q", c.Crudo, cabRWT)
	}
	if c.Repeticiones != 3 {
		t.Errorf("repeticiones = %d", c.Repeticiones)
	}
	if math.Abs(c.Confianza-2.0/3) > 1e-9 {
		t.Errorf("confianza = %v, se esperaba 2/3", c.Confianza)
	}
}

// TestVotoPorCaracter: las tres copias llegaron rotas, cada una en un sitio
// distinto. Ninguna se puede leer sola; votando carácter por carácter sale la
// buena. Esto es para lo que el protocolo manda la cabecera tres veces.
func TestVotoPorCaracter(t *testing.T) {
	romper := func(pos int, c byte) string {
		b := []byte(cabRWT)
		b[pos] = c
		return string(b)
	}
	g := nuevoGenerador(48000, 0.5)
	g.silencio(0.2)
	for _, s := range []string{romper(6, 'Q'), romper(15, 'X'), romper(30, 'Y')} {
		g.rafaga(s)
		g.silencio(1)
	}
	c := unaCabecera(t, correr(t, g.salida, Opciones{}))
	if c.Crudo != cabRWT {
		t.Fatalf("crudo = %q, se esperaba %q reconstruida", c.Crudo, cabRWT)
	}
	if c.Org != "EAS" || c.Evento != "RWT" || len(c.Zonas) != 1 {
		t.Errorf("cabecera reconstruida mal: %+v", c)
	}
	if math.Abs(c.Confianza-1.0/3) > 1e-9 {
		t.Errorf("confianza = %v: una cabecera reconstruida vale 1/3", c.Confianza)
	}
}

// TestSenalDeAtencion: el pitido de 853+960 Hz sale como evento aparte, con su
// duración. Antena787 no lo genera nunca; solo lo oye (ADR 0010).
func TestSenalDeAtencion(t *testing.T) {
	g := nuevoGenerador(48000, 0.5)
	g.mensajeCompleto(cabRWT, 2, 0.5)
	evs := correr(t, g.salida, Opciones{DetectarAtencion: true})
	_ = unaCabecera(t, evs)
	as := atencionesDe(evs)
	if len(as) != 1 {
		t.Fatalf("se esperaba 1 señal de atención, salieron %d", len(as))
	}
	if d := as[0].Duracion; d < 1800*time.Millisecond || d > 2200*time.Millisecond {
		t.Errorf("duración = %v, se esperaban 2 s", d)
	}
}

// TestAtencionApagada: si no se pide, no se mira. Es lo de fábrica, porque son
// tres correladores más por muestra.
func TestAtencionApagada(t *testing.T) {
	g := nuevoGenerador(48000, 0.5)
	g.mensajeCompleto(cabRWT, 2, 0.5)
	evs := correr(t, g.salida, Opciones{})
	if len(atencionesDe(evs)) != 0 {
		t.Error("salió una señal de atención con el detector apagado")
	}
}

// TestBloquesImpares: el PCM puede llegar partido a la mitad de una muestra. Se
// mete byte por byte para forzar el caso peor.
func TestBloquesImpares(t *testing.T) {
	g := nuevoGenerador(48000, 0.5)
	g.mensajeCompleto(cabRWT, 0, 0.2)
	pcm := aPCM(g.salida)

	d, err := Nuevo(Opciones{Reloj: relojFijo, Buffer: 64})
	if err != nil {
		t.Fatal(err)
	}
	var evs []Evento
	listo := make(chan struct{})
	go func() {
		for ev := range d.Eventos() {
			evs = append(evs, ev)
		}
		close(listo)
	}()
	// Bloques de largo impar y cambiante: 1, 3, 7, 1, 3, 7…
	largos := []int{1, 3, 7}
	i, k := 0, 0
	for i < len(pcm) {
		n := largos[k%len(largos)]
		k++
		if i+n > len(pcm) {
			n = len(pcm) - i
		}
		if _, err := d.Escribir(pcm[i : i+n]); err != nil {
			t.Fatal(err)
		}
		i += n
	}
	if err := d.Cerrar(); err != nil {
		t.Fatal(err)
	}
	<-listo

	cs := cabecerasDe(evs)
	if len(cs) != 1 || cs[0].Crudo != cabRWT {
		t.Fatalf("cabeceras = %+v", cs)
	}
}

// TestDosAlertasSeguidas: dos alertas en el mismo audio son dos cabeceras y dos
// fines, no una mezcla de las dos.
func TestDosAlertasSeguidas(t *testing.T) {
	g := nuevoGenerador(48000, 0.5)
	g.mensajeCompleto(cabRWT, 0, 0.3)
	g.silencio(2)
	g.mensajeCompleto(cabTOR, 0, 0.3)
	evs := correr(t, g.salida, Opciones{})
	cs := cabecerasDe(evs)
	if len(cs) != 2 {
		t.Fatalf("se esperaban 2 cabeceras, salieron %d", len(cs))
	}
	if cs[0].Evento != "RWT" || cs[1].Evento != "TOR" {
		t.Errorf("eventos = %q, %q", cs[0].Evento, cs[1].Evento)
	}
	if n := len(finesDe(evs)); n != 2 {
		t.Errorf("fines = %d", n)
	}
}

func TestTasaMuyBaja(t *testing.T) {
	if _, err := Nuevo(Opciones{Tasa: 4000}); err != ErrTasa {
		t.Errorf("err = %v, se esperaba ErrTasa", err)
	}
}

func TestEscribirDespuesDeCerrar(t *testing.T) {
	d, err := Nuevo(Opciones{})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range d.Eventos() {
		}
	}()
	if err := d.Cerrar(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Escribir([]byte{0, 0}); err != ErrCerrado {
		t.Errorf("err = %v, se esperaba ErrCerrado", err)
	}
	if err := d.Cerrar(); err != nil {
		t.Errorf("Cerrar dos veces: %v", err)
	}
}

// TestTasaDe22k: el retorno de aire de una radio puede llegar a 22050. El
// protocolo no cambia; lo único que cambia es cuántas muestras tiene un bit.
func TestTasaDe22k(t *testing.T) {
	g := nuevoGenerador(22050, 0.5)
	g.mensajeCompleto(cabRWT, 0, 0.3)
	c := unaCabecera(t, correr(t, g.salida, Opciones{Tasa: 22050}))
	if c.Crudo != cabRWT {
		t.Errorf("crudo = %q", c.Crudo)
	}
}

// TestAnalizarCabecera prueba el analizador por su cuenta, sin audio: es donde
// se decide qué entra al as-run y qué no.
func TestAnalizarCabecera(t *testing.T) {
	casos := []struct {
		nombre string
		s      string
		ok     bool
	}{
		{"prueba semanal", cabRWT, true},
		{"tres zonas", cabTOR, true},
		{"seis horas", "ZCZC-PEP-EAN-000000+0600-2531200-WPRT/TV -", true},
		{"sin ZCZC", "XCZC-EAS-RWT-072000+0015-2531200-WPRT/TV -", false},
		{"org con dígito", "ZCZC-EA5-RWT-072000+0015-2531200-WPRT/TV -", false},
		{"zona con letra", "ZCZC-EAS-RWT-07200X+0015-2531200-WPRT/TV -", false},
		{"zona de cinco", "ZCZC-EAS-RWT-07200+0015-2531200-WPRT/TV -", false},
		{"sin el más", "ZCZC-EAS-RWT-072000-0015-2531200-WPRT/TV -", false},
		{"minutos imposibles", "ZCZC-EAS-RWT-072000+0075-2531200-WPRT/TV -", false},
		{"día cero", "ZCZC-EAS-RWT-072000+0015-0001200-WPRT/TV -", false},
		{"hora 25", "ZCZC-EAS-RWT-072000+0015-2532500-WPRT/TV -", false},
		{"llamada de siete", "ZCZC-EAS-RWT-072000+0015-2531200-WPRT/TV-", false},
		{"sin guion final", "ZCZC-EAS-RWT-072000+0015-2531200-WPRT/TV ", false},
		{"cola de más", "ZCZC-EAS-RWT-072000+0015-2531200-WPRT/TV -X", false},
		{"vacía", "", false},
	}
	for _, c := range casos {
		if _, ok := analizarCabecera([]byte(c.s)); ok != c.ok {
			t.Errorf("%s: ok = %v, se esperaba %v", c.nombre, ok, c.ok)
		}
	}

	// Un evento de duración larga: seis horas.
	v, ok := analizarCabecera([]byte("ZCZC-PEP-EAN-000000+0600-2531200-WPRT/TV -"))
	if !ok {
		t.Fatal("no analizó la alerta nacional")
	}
	if v.Duracion != 6*time.Hour {
		t.Errorf("duración = %v", v.Duracion)
	}
	if v.Org != "PEP" || v.Evento != "EAN" {
		t.Errorf("org/evento = %q/%q", v.Org, v.Evento)
	}
}

// TestTreintaYUnaZonas: el tope de la norma.
func TestTreintaYUnaZonas(t *testing.T) {
	s := "ZCZC-WXR-TOR"
	for i := 0; i < 31; i++ {
		s += "-0720" + string(rune('0'+byte(i/10))) + string(rune('0'+byte(i%10)))
	}
	s += "+0030-2531215-KSJT/NWS-"
	c, ok := analizarCabecera([]byte(s))
	if !ok {
		t.Fatalf("no analizó 31 zonas: %q", s)
	}
	if len(c.Zonas) != 31 {
		t.Errorf("zonas = %d", len(c.Zonas))
	}

	// Treinta y dos ya no.
	s32 := "ZCZC-WXR-TOR"
	for i := 0; i < 32; i++ {
		s32 += "-0720" + string(rune('0'+byte(i/10))) + string(rune('0'+byte(i%10)))
	}
	s32 += "+0030-2531215-KSJT/NWS-"
	if _, ok := analizarCabecera([]byte(s32)); ok {
		t.Error("32 zonas pasaron y no deberían")
	}
}
