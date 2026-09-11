package sage

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

// Las transcripciones de abajo salen del manual del Sage ENDEC Rev 1.5 (§8.1 y
// §8.4) palabra por palabra, incluidos los espacios de relleno de la señal de
// llamada. Las que no están en el manual se dicen: llevan la palabra
// «verosímil» en el comentario y son de la forma que el manual describe, no de
// una captura.
//
// sinc es el byte de sincronismo de la parte 11, 0xAB, dieciséis veces, que es
// lo que va delante de cada cabecera y de cada EOM en la salida del ENCODER
// device. En un terminal se ve como `+`, pero no es un `+`.
var sinc = strings.Repeat("\xab", 16)

// reloj fijo: 12 de abril de 2026, 03:00 UTC. El día 102 del año es el 12 de
// abril, que es justo el día de las cabeceras del manual (JJJ=102), así que las
// pruebas del año del JJJHHMM salen de una fecha real y no de una inventada.
func relojFijo() time.Time {
	return time.Date(2026, 4, 12, 3, 0, 0, 0, time.UTC)
}

// escribirDeGolpe le pasa al analizador todo el texto en una sola lectura.
func escribirDeGolpe(t *testing.T, texto string) ([]Evento, *Analizador) {
	t.Helper()
	a := NuevoAnalizador(relojFijo)
	eventos := a.Escribir([]byte(texto))
	eventos = append(eventos, a.Cerrar()...)
	return eventos, a
}

func soloTipo(eventos []Evento, tipo Tipo) []Evento {
	var salida []Evento
	for _, e := range eventos {
		if e.Tipo == tipo {
			salida = append(salida, e)
		}
	}
	return salida
}

// La prueba semanal es el latido del sistema: es lo único que llega por este
// cable cuando no pasa nada, y es lo que NO tiene que contar como interrupción
// comercial.
func TestPruebaSemanalRWTDelManual(t *testing.T) {
	// Manual §8.1, el ejemplo del DECODER device, con su texto expandido
	// partido en dos líneas como lo parte el manual.
	transcripcion := "local:ZCZC-EAS-RWT-006013+0015-1020638-SAGE    -\r\n" +
		"A Broadcast station or cable system has issued a Required Weekly Test for Contra\r\n" +
		" Costa, CA beginning at 02:38 am and ending at 02:53 am (SAGE)\r\n"

	eventos, _ := escribirDeGolpe(t, transcripcion)
	if len(eventos) != 3 {
		t.Fatalf("esperaba 3 eventos (cabecera + dos líneas de texto) y salieron %d: %+v", len(eventos), eventos)
	}

	e := eventos[0]
	if e.Tipo != TipoLocal {
		t.Fatalf("tipo %q y tenía que ser local", e.Tipo)
	}
	if !e.Interrumpe() {
		t.Fatal("un `local:` es el ENDEC tomando el aire y no lo dijo")
	}
	if e.Clase != ClasePruebaSemanal {
		t.Fatalf("una RWT salió como %q; si esto dice «real» la reposición de anuncios se inventa trabajo cada semana", e.Clase)
	}
	if !e.EsPrueba() {
		t.Fatal("una RWT no se reconoció como prueba")
	}
	if e.Crudo != "local:ZCZC-EAS-RWT-006013+0015-1020638-SAGE    -" {
		t.Fatalf("el crudo no se guardó entero: %q", e.Crudo)
	}
	if !e.Recibido.Equal(relojFijo()) {
		t.Fatalf("Recibido %v y tenía que ser la hora en que lo leímos, %v", e.Recibido, relojFijo())
	}

	c := e.Cabecera
	if c == nil {
		t.Fatal("no parseó la cabecera del ejemplo del propio manual")
	}
	if c.Origen != "EAS" || c.Evento != "RWT" {
		t.Fatalf("ORG/EEE salieron %q/%q", c.Origen, c.Evento)
	}
	if len(c.Zonas) != 1 || c.Zonas[0].Crudo != "006013" {
		t.Fatalf("zonas %v y esperaba una sola, 006013", c.Zonas)
	}
	if z := c.Zonas[0]; z.Parte != 0 || z.Estado != 6 || z.Condado != 13 {
		t.Fatalf("006013 se partió mal: parte=%d estado=%d condado=%d (esperaba 0/6/13, Contra Costa CA)", z.Parte, z.Estado, z.Condado)
	}
	if c.Duracion != 15*time.Minute {
		t.Fatalf("+0015 salió %v; TTTT es HHMM, no minutos", c.Duracion)
	}
	// 1020638 es día 102 (12 de abril) a las 06:38 **UTC**, y el texto
	// expandido de la misma línea del manual dice «beginning at 02:38 am»:
	// esas dos horas no se contradicen, la cabecera va en UTC (11.31) y el
	// texto en palabras claras va en la hora local del ENDEC. Confundirlas escribe el
	// as-run cuatro horas corrido, así que aquí se fija en UTC a propósito.
	quiero := time.Date(2026, 4, 12, 6, 38, 0, 0, time.UTC)
	if !c.Instante.Equal(quiero) {
		t.Fatalf("1020638 salió %v y esperaba %v", c.Instante, quiero)
	}
	if c.Estacion != "SAGE" {
		t.Fatalf("la señal de llamada salió %q; los espacios de relleno se recortan", c.Estacion)
	}

	// El texto en palabras claras llega detrás, como eventos propios: la cabecera no
	// se retiene esperándolo, porque la noticia es la alerta y no su prosa.
	textos := soloTipo(eventos, TipoTexto)
	if len(textos) != 2 {
		t.Fatalf("esperaba dos líneas de texto expandido y salieron %d", len(textos))
	}
	if !strings.Contains(textos[0].Texto, "Required Weekly Test") {
		t.Fatalf("el texto expandido se perdió: %q", textos[0].Texto)
	}
}

// La prueba mensual lleva dos zonas, que es donde se ve si el analizador
// entiende que el `+TTTT` va pegado a la última y no a la primera.
func TestPruebaMensualRMTConDosZonas(t *testing.T) {
	// Manual §8.4, la cabecera de dentro del bloque NEWS FEED.
	eventos, _ := escribirDeGolpe(t, "match:ZCZC-EAS-RMT-001001-001005+0015-0390352-SAGE    -\r\n")
	if len(eventos) != 1 {
		t.Fatalf("esperaba un evento y salieron %d", len(eventos))
	}
	e := eventos[0]
	if e.Tipo != TipoCoincide {
		t.Fatalf("tipo %q y tenía que ser match", e.Tipo)
	}
	// Y aquí está la diferencia que importa: `match:` NO es una interrupción.
	if e.Interrumpe() {
		t.Fatal("un `match:` es «la oí y me interesa», todavía no la manda: contarlo como interrupción escribe en el as-run algo que no pasó")
	}
	if e.Clase != ClasePruebaMensual {
		t.Fatalf("una RMT salió como %q", e.Clase)
	}
	c := e.Cabecera
	if c == nil {
		t.Fatal("no parseó la cabecera")
	}
	if len(c.Zonas) != 2 || c.Zonas[0].Crudo != "001001" || c.Zonas[1].Crudo != "001005" {
		t.Fatalf("zonas %v y esperaba 001001 y 001005 (Autauga y Barbour, AL)", c.Zonas)
	}
	if c.Duracion != 15*time.Minute {
		t.Fatalf("+0015 salió %v", c.Duracion)
	}
	if quiero := time.Date(2026, 2, 8, 3, 52, 0, 0, time.UTC); !c.Instante.Equal(quiero) {
		t.Fatalf("0390352 salió %v y esperaba %v (día 39 del año)", c.Instante, quiero)
	}
}

// Una alerta real de verdad: la que sí interrumpe y sí cuenta. Verosímil, con
// la forma del manual; la zona es Puerto Rico (estado 72, San Juan 127).
func TestAlertaRealTOR(t *testing.T) {
	eventos, _ := escribirDeGolpe(t, "local:ZCZC-WXR-TOR-072127+0030-2531830-KWNS/NWS-\r\n")
	if len(eventos) != 1 {
		t.Fatalf("esperaba un evento y salieron %d", len(eventos))
	}
	e := eventos[0]
	if e.Clase != ClaseReal {
		t.Fatalf("una TOR salió como %q y tiene que ser real", e.Clase)
	}
	if e.EsPrueba() {
		t.Fatal("una TOR no es una prueba")
	}
	if !e.Interrumpe() {
		t.Fatal("un `local:` de TOR es una interrupción de verdad")
	}
	c := e.Cabecera
	if c == nil {
		t.Fatal("no parseó la cabecera")
	}
	if c.Origen != "WXR" {
		t.Fatalf("ORG salió %q", c.Origen)
	}
	if c.Duracion != 30*time.Minute {
		t.Fatalf("+0030 salió %v", c.Duracion)
	}
	if z := c.Zonas[0]; z.Estado != 72 || z.Condado != 127 {
		t.Fatalf("072127 se partió mal: estado=%d condado=%d", z.Estado, z.Condado)
	}
	if c.Estacion != "KWNS/NWS" {
		t.Fatalf("la señal de llamada salió %q; lleva barra y son ocho caracteres", c.Estacion)
	}
}

// `nomatch:` y `dup:` se registran y no pasa nada más: ninguno toca el aire.
func TestNoCoincideYDuplicadaNoInterrumpen(t *testing.T) {
	eventos, _ := escribirDeGolpe(t,
		"nomatch:ZCZC-WXR-SVA-002010+0015-0390352-SAGE    -\r\n"+
			"dup:ZCZC-EAS-RWT-006013+0015-1020638-SAGE    -\r\n")
	cabeceras := 0
	for _, e := range eventos {
		if e.Tipo == TipoNoCoincide || e.Tipo == TipoDuplicada {
			cabeceras++
			if e.Interrumpe() {
				t.Fatalf("%q no puede contar como interrupción", e.Tipo)
			}
			if e.Cabecera == nil {
				t.Fatalf("%q perdió la cabecera", e.Tipo)
			}
		}
	}
	if cabeceras != 2 {
		t.Fatalf("esperaba un nomatch y un dup y salieron %d", cabeceras)
	}
}

// Un puerto serial no entrega líneas, entrega bytes: la cabecera del manual
// partida byte a byte tiene que dar el mismo evento que de golpe. Esta es la
// prueba que de verdad separa un parser de un `strings.Split`.
func TestCabeceraPartidaEnDosLecturas(t *testing.T) {
	completa := "local:ZCZC-EAS-RWT-006013+0015-1020638-SAGE    -\r\n"
	for corte := 1; corte < len(completa); corte++ {
		a := NuevoAnalizador(relojFijo)
		eventos := append(a.Escribir([]byte(completa[:corte])), a.Escribir([]byte(completa[corte:]))...)
		eventos = append(eventos, a.Cerrar()...)
		cabeceras := soloTipo(eventos, TipoLocal)
		if len(cabeceras) != 1 {
			t.Fatalf("partiendo en %d salieron %d cabeceras y tenía que ser 1: %+v", corte, len(cabeceras), eventos)
		}
		if c := cabeceras[0].Cabecera; c == nil || c.Evento != "RWT" || len(c.Zonas) != 1 {
			t.Fatalf("partiendo en %d la cabecera salió mal: %+v", corte, c)
		}
	}
}

// Y byte a byte, que es el caso de un cable a 1200 baudios con un lector
// impaciente.
func TestCabeceraByteAByte(t *testing.T) {
	completa := "local:ZCZC-EAS-RWT-006013+0015-1020638-SAGE    -\r\n"
	a := NuevoAnalizador(relojFijo)
	var eventos []Evento
	for i := 0; i < len(completa); i++ {
		eventos = append(eventos, a.Escribir([]byte{completa[i]})...)
	}
	if len(soloTipo(eventos, TipoLocal)) != 1 {
		t.Fatalf("byte a byte salieron %d cabeceras: %+v", len(soloTipo(eventos, TipoLocal)), eventos)
	}
}

// La salida cruda del ENCODER device: tres cabeceras con su sincronismo y una
// línea con los tres `NNNN`. Las tres repeticiones son UN mensaje y los tres
// EOM son UN fin: contarlos como seis eventos sería inventarse cinco alertas.
func TestSalidaCrudaDelEncoderYElEOM(t *testing.T) {
	cabecera := sinc + "ZCZC-EAS-RWT-006013+0015-1020624-SAGE    -\r\n"
	transcripcion := cabecera + cabecera + cabecera +
		sinc + "NNNN" + sinc + "NNNN" + sinc + "NNNN" + "\r\n"

	eventos, a := escribirDeGolpe(t, transcripcion)
	if n := len(soloTipo(eventos, TipoCabecera)); n != 3 {
		t.Fatalf("esperaba tres cabeceras crudas (las tres repeticiones de la parte 11) y salieron %d: %+v", n, eventos)
	}
	fines := soloTipo(eventos, TipoFin)
	if len(fines) != 1 {
		t.Fatalf("esperaba UN fin de mensaje de la línea con los tres NNNN y salieron %d", len(fines))
	}
	// El sincronismo no es texto: no se queda en el crudo, que va a la base de
	// datos y a la bitácora.
	for _, e := range eventos {
		if strings.ContainsRune(e.Crudo, 0xAB) {
			t.Fatalf("el byte de sincronismo 0xAB se quedó en el crudo: %q", e.Crudo)
		}
	}
	if a.Ruido().Bytes == 0 {
		t.Fatal("se quitaron bytes de sincronismo y no se contaron: sin esa cuenta no se puede diagnosticar un puerto mal configurado")
	}
	if c := soloTipo(eventos, TipoCabecera)[0].Cabecera; c == nil || c.Evento != "RWT" {
		t.Fatalf("la cabecera cruda no se parseó: %+v", c)
	}
}

// Un capturador que pasó por un editor deja los 0xAB convertidos en `+`. Se
// tratan igual, y el `+` suelto del `+TTTT` no se toca.
func TestSincronismoPintadoComoMasNoSeComeElMasDelTTTT(t *testing.T) {
	eventos, _ := escribirDeGolpe(t, "++++++++++++++++ZCZC-EAS-RWT-006013+0015-1020624-SAGE    -\r\n")
	if len(eventos) != 1 || eventos[0].Cabecera == nil {
		t.Fatalf("no entendió la cabecera con el sincronismo pintado como `+`: %+v", eventos)
	}
	if d := eventos[0].Cabecera.Duracion; d != 15*time.Minute {
		t.Fatalf("se comió el `+` del +TTTT: duración %v", d)
	}
}

// El bloque NEWS FEED (§8.4) pone el texto DELANTE de la cabecera, al revés que
// el DECODER device. Se le saca la cabecera y el texto no se le atribuye a la
// alerta anterior.
func TestBloqueDeNoticias(t *testing.T) {
	transcripcion := "<ENDECSTART>\r\n" +
		"Local Alert sent at 02/07/97 22:52:55\r\n" +
		"The National Weather Service has issued a Severe Thunderstorm\r\n" +
		"Watch for Aleutian Islands, AK beginning at 10:52 pm and ending\r\n" +
		"at 11:07 pm (SAGE)\r\n" +
		"ZCZC-WXR-SVA-002010+0015-0390352-SAGE -\r\n" +
		"<ENDECEND>\r\n"

	eventos, _ := escribirDeGolpe(t, transcripcion)
	cabeceras := soloTipo(eventos, TipoCabecera)
	if len(cabeceras) != 1 {
		t.Fatalf("esperaba una cabecera dentro del bloque y salieron %d: %+v", len(cabeceras), eventos)
	}
	if c := cabeceras[0].Cabecera; c == nil || c.Evento != "SVA" || c.Origen != "WXR" {
		t.Fatalf("la cabecera del bloque salió mal: %+v", c)
	}
	for _, e := range soloTipo(eventos, TipoTexto) {
		if e.Cabecera != nil {
			t.Fatal("el texto de un bloque de noticias va delante de su cabecera: no se le puede colgar una")
		}
	}
	if n := len(soloTipo(eventos, TipoTexto)); n != 4 {
		t.Fatalf("esperaba las cuatro líneas de texto del bloque y salieron %d", n)
	}
}

// Basura entre líneas: un equipo que arranca, un device type cambiado a mano,
// ruido en el cable. Nada de eso puede tapar la alerta que va en medio.
func TestBasuraEntreLineas(t *testing.T) {
	transcripcion := "\x00\x01\x02\x03basura de arranque\x04\r\n" +
		"\r\n\r\n" +
		"   \r\n" +
		"\x02" + "3A Broadcast station or cable system has issued a Required Weekly Test\x03\r\n" +
		"local:ZCZC-EAS-RWT-006013+0015-1020638-SAGE    -\r\n" +
		"\xff\xfe\xfd\r\n" +
		"local:ZCZC-WXR-TOR-072127+0030-2531830-KWNS/NWS-\r\n"

	eventos, a := escribirDeGolpe(t, transcripcion)
	locales := soloTipo(eventos, TipoLocal)
	if len(locales) != 2 {
		t.Fatalf("la basura se comió una alerta: salieron %d de 2 (%+v)", len(locales), eventos)
	}
	for _, e := range locales {
		if e.Cabecera == nil {
			t.Fatalf("una alerta rodeada de basura perdió su cabecera: %q", e.Crudo)
		}
	}
	if a.Ruido().Lineas == 0 {
		t.Fatal("se tiraron líneas y no se contaron")
	}
}

// Una cabecera malformada no se pierde: el Evento sale con Crudo y sin
// Cabecera, porque perder la noticia de que el ENDEC habló es peor que perder
// los campos. Y Clase queda vacía, no «real».
func TestCabeceraMalformadaNoPierdeElEvento(t *testing.T) {
	casos := []string{
		"local:ZCZC-EAS",
		"local:ZCZC-EAS-RWT-006013-1020638-SAGE-",  // sin el +TTTT
		"local:ZCZC-EA-RWT-006013+0015-1020638-S-", // ORG de dos letras
		"local:ZCZC-EAS-RWT-60013+0015-1020638-SAGE-",
		"local:ZCZC-EAS-RWT-006013+15-1020638-SAGE-",
		"local:ZCZC-EAS-RWT-006013+0015-999999-SAGE-",
		"local:ZCZC-EAS-RWT-006013+0015-0000638-SAGE-",
		"local:ZCZC-EAS-RWT-006013+0015-1020638-",
		"local:",
	}
	for _, caso := range casos {
		eventos, _ := escribirDeGolpe(t, caso+"\r\n")
		if len(eventos) != 1 {
			t.Fatalf("%q dio %d eventos y tenía que dar 1: %+v", caso, len(eventos), eventos)
		}
		e := eventos[0]
		if e.Tipo != TipoLocal {
			t.Fatalf("%q perdió el tipo: %q", caso, e.Tipo)
		}
		if e.Cabecera != nil {
			t.Fatalf("%q parseó una cabecera que no es válida: %+v", caso, e.Cabecera)
		}
		if e.Clase != "" {
			t.Fatalf("%q rellenó la clase con %q sin poder saberlo", caso, e.Clase)
		}
		if !strings.Contains(e.Crudo, "local:") {
			t.Fatalf("%q perdió el crudo: %q", caso, e.Crudo)
		}
	}
}

// Un día 001 llegando un 31 de diciembre es del año que viene, y un día 365
// llegando un 1 de enero es del año pasado. El protocolo no manda año y esta es
// la única regla que no se rompe en Nochevieja.
func TestElAnioDelDiaJulianoSeElijePorCercania(t *testing.T) {
	casos := []struct {
		nombre string
		ahora  time.Time
		jjj    string
		quiero time.Time
	}{
		{"nochevieja recibiendo el día 1", time.Date(2026, 12, 31, 23, 55, 0, 0, time.UTC), "0010010", time.Date(2027, 1, 1, 0, 10, 0, 0, time.UTC)},
		{"año nuevo recibiendo el día 365", time.Date(2027, 1, 1, 0, 5, 0, 0, time.UTC), "3652350", time.Date(2026, 12, 31, 23, 50, 0, 0, time.UTC)},
		{"un martes cualquiera", time.Date(2026, 4, 12, 3, 0, 0, 0, time.UTC), "1020638", time.Date(2026, 4, 12, 6, 38, 0, 0, time.UTC)},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			cab, err := ParsearCabecera("ZCZC-EAS-RWT-006013+0015-"+c.jjj+"-SAGE    -", c.ahora)
			if err != nil {
				t.Fatalf("no parseó: %v", err)
			}
			if !cab.Instante.Equal(c.quiero) {
				t.Fatalf("salió %v y esperaba %v", cab.Instante, c.quiero)
			}
		})
	}
}

// TTTT es HHMM. Una alerta de seis horas es +0600, no 600 minutos.
func TestDuracionEsHHMM(t *testing.T) {
	casos := map[string]time.Duration{
		"0015": 15 * time.Minute,
		"0030": 30 * time.Minute,
		"0100": time.Hour,
		"0600": 6 * time.Hour,
		"2359": 23*time.Hour + 59*time.Minute,
	}
	for tttt, quiero := range casos {
		cab, err := ParsearCabecera("ZCZC-EAS-RWT-006013+"+tttt+"-1020638-SAGE    -", relojFijo())
		if err != nil {
			t.Fatalf("+%s no parseó: %v", tttt, err)
		}
		if cab.Duracion != quiero {
			t.Fatalf("+%s salió %v y esperaba %v", tttt, cab.Duracion, quiero)
		}
	}
}

// Treinta y una zonas es el máximo de 11.31; treinta y dos es una cabecera
// corrupta y se dice, no se acepta a medias.
func TestTopeDeTreintaYUnaZonas(t *testing.T) {
	construir := func(n int) string {
		zonas := make([]string, n)
		for i := range zonas {
			zonas[i] = "072127"
		}
		return "ZCZC-EAS-RWT-" + strings.Join(zonas[:n-1], "-") + "-072127+0015-1020638-SAGE    -"
	}
	if _, err := ParsearCabecera(construir(31), relojFijo()); err != nil {
		t.Fatalf("31 zonas es válido y dio error: %v", err)
	}
	if _, err := ParsearCabecera(construir(32), relojFijo()); err == nil {
		t.Fatal("32 zonas tenía que dar error")
	}
}

// Una línea que crece sin fin no es una línea: es un puerto a los baudios
// equivocados. Se tira, se cuenta, y la memoria no crece.
func TestLineaSinFinSeTiraYSeCuenta(t *testing.T) {
	a := NuevoAnalizador(relojFijo)
	for i := 0; i < 40; i++ {
		if eventos := a.Escribir([]byte(strings.Repeat("x", 1024))); len(eventos) != 0 {
			t.Fatalf("una línea sin fin produjo %d eventos", len(eventos))
		}
	}
	if a.Ruido().Truncadas == 0 {
		t.Fatal("no contó ninguna línea truncada")
	}
	if len(a.resto) > lineaMaxima {
		t.Fatalf("el resto creció hasta %d bytes: el tope no se está aplicando", len(a.resto))
	}
	// Y después de tirar la basura, la siguiente alerta buena tiene que salir.
	eventos := a.Escribir([]byte("\r\nlocal:ZCZC-EAS-RWT-006013+0015-1020638-SAGE    -\r\n"))
	if len(soloTipo(eventos, TipoLocal)) != 1 {
		t.Fatalf("después del ruido no volvió a entender una alerta: %+v", eventos)
	}
}

// El texto en español del ENDEC no viene en UTF-8: viene en Latin-1, que es lo
// que mandaba un equipo de 1997. No se puede perder ni romper la cadena.
func TestTextoEnEspanolEnLatin1(t *testing.T) {
	// «Añasco» en Latin-1: la ñ es 0xF1.
	eventos, _ := escribirDeGolpe(t,
		"local:ZCZC-EAS-RWT-072011+0015-1020638-SAGE    -\r\n"+
			"Prueba semanal requerida para A\xf1asco, PR\r\n")
	textos := soloTipo(eventos, TipoTexto)
	if len(textos) != 1 {
		t.Fatalf("esperaba una línea de texto y salieron %d", len(textos))
	}
	if !strings.Contains(textos[0].Texto, "Añasco") {
		t.Fatalf("el texto en español se rompió: %q", textos[0].Texto)
	}
}

// El contrato que de verdad importa: esto corre al lado del aire y no entra en
// pánico con nada. Se le da la transcripción mutilada de todas las maneras, más
// bytes al azar con una semilla fija.
func TestNuncaEntraEnPanico(t *testing.T) {
	base := "local:ZCZC-EAS-RWT-006013+0015-1020638-SAGE    -\r\n" +
		"A Broadcast station or cable system has issued a Required Weekly Test\r\n" +
		sinc + "ZCZC-EAS-RMT-001001-001005+0015-0390352-SAGE    -\r\n" +
		sinc + "NNNN" + sinc + "NNNN\r\n<ENDECSTART>\r\nalgo\r\n<ENDECEND>\r\n"

	// Todos los prefijos, que es cada estado intermedio posible del búfer.
	for i := 0; i <= len(base); i++ {
		a := NuevoAnalizador(relojFijo)
		a.Escribir([]byte(base[:i]))
		a.Cerrar()
	}
	// Todos los sufijos, que es arrancar a leer un cable a media cabecera.
	for i := 0; i <= len(base); i++ {
		a := NuevoAnalizador(relojFijo)
		a.Escribir([]byte(base[i:]))
		a.Cerrar()
	}
	// Y basura pura, con semilla fija para que la prueba sea repetible.
	r := rand.New(rand.NewSource(787))
	for ronda := 0; ronda < 200; ronda++ {
		a := NuevoAnalizador(relojFijo)
		for trozo := 0; trozo < 5; trozo++ {
			b := make([]byte, r.Intn(300))
			for i := range b {
				b[i] = byte(r.Intn(256))
			}
			a.Escribir(b)
		}
		a.Cerrar()
	}
	// Y una cabecera sin nada detrás, que es lo que deja un cable cortado a
	// media línea: Cerrar la tiene que entregar igual.
	a := NuevoAnalizador(relojFijo)
	a.Escribir([]byte("local:ZCZC-EAS-RWT-006013+0015-1020638-SAGE    -"))
	if eventos := a.Cerrar(); len(eventos) != 1 || eventos[0].Cabecera == nil {
		t.Fatalf("Cerrar perdió la última línea sin salto: %+v", eventos)
	}
}

// El analizador no puede etiquetar como «texto de la alerta» un chorro infinito
// de líneas de prosa detrás de una cabecera vieja.
func TestElTextoDeUnaCabeceraNoEsInfinito(t *testing.T) {
	a := NuevoAnalizador(relojFijo)
	a.Escribir([]byte("local:ZCZC-EAS-RWT-006013+0015-1020638-SAGE    -\r\n"))
	textos := 0
	for i := 0; i < lineasDeTextoMaximas*3; i++ {
		for _, e := range a.Escribir([]byte("una linea mas de lo que sea\r\n")) {
			if e.Tipo == TipoTexto {
				textos++
			}
		}
	}
	if textos > lineasDeTextoMaximas {
		t.Fatalf("etiquetó %d líneas como el texto de la alerta y el tope es %d", textos, lineasDeTextoMaximas)
	}
	if a.Ruido().Lineas == 0 {
		t.Fatal("lo que pasó del tope tenía que contarse como ruido")
	}
}

// Una zona con condado 000 es el estado completo, y eso se enseña distinto.
func TestZonaDeEstadoCompleto(t *testing.T) {
	cab, err := ParsearCabecera("ZCZC-EAS-RMT-072000+0015-1020638-SAGE    -", relojFijo())
	if err != nil {
		t.Fatalf("no parseó: %v", err)
	}
	if !cab.Zonas[0].EsElEstadoCompleto() {
		t.Fatal("072000 es Puerto Rico entero y no lo dijo")
	}
	if cab.Zonas[0].String() != "072000" {
		t.Fatalf("String() de la zona salió %q", cab.Zonas[0].String())
	}
}
