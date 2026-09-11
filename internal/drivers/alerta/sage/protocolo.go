// protocolo.go convierte un chorro de bytes del ENDEC en Eventos.
//
// Es todo el trabajo de verdad del paquete y no toca ni un puerto ni un socket:
// se le escriben trozos de lo que llegó y devuelve los eventos completos. Así
// las pruebas corren con transcripciones del manual y sin hardware, que es
// exactamente el caso normal de este proyecto (docs/drivers/README.md, «cómo se
// prueba sin el equipo en la mano»).
//
// Dos obsesiones aquí dentro:
//
//   - **tolerante al ruido y a las lecturas partidas.** Un puerto serial no
//     entrega líneas, entrega bytes cuando le da la gana: una cabecera puede
//     llegar en dos lecturas, o en catorce. Y el cable comparte el mundo con
//     bytes de sincronismo, basura de arranque del equipo y lo que sea que
//     mandara el device type anterior si alguien cambió el menú.
//   - **nunca entra en pánico.** Esto corre al lado del aire: una cabecera
//     malformada tiene que salir como un Evento sin cabecera parseada y con el
//     crudo intacto, no tumbar el proceso.
package sage

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Tipo es lo que la línea significa. Los seis primeros vienen del protocolo
// del ENDEC; los dos últimos no los dice el equipo, los pone este driver para
// contar el estado del cable (transporte.go).
type Tipo string

const (
	// TipoLocal — `local:`. **El ENDEC está enviando la alerta**: el aire es
	// suyo desde ahora. Es el único tipo que significa una interrupción
	// (manual §8.1, «An alert is being sent by the ENDEC»).
	TipoLocal Tipo = "local"
	// TipoCoincide — `match:`. La oyó en un monitor y coincide con un filtro
	// de entrada. Todavía no la manda: es el estado del relé PENDING.
	TipoCoincide Tipo = "match"
	// TipoNoCoincide — `nomatch:`. La oyó y no coincide con ningún filtro. Se
	// registra y no pasa nada más.
	TipoNoCoincide Tipo = "nomatch"
	// TipoDuplicada — `dup:`. Ya la había oído; no se retransmite.
	TipoDuplicada Tipo = "dup"
	// TipoCabecera — una cabecera ZCZC sin prefijo de tipo: la copia cruda del
	// ENCODER device (§8.1) o la que va dentro de un bloque NEWS FEED (§8.4).
	// No dice si la alerta se originó aquí o se retransmitió, y no se adivina.
	TipoCabecera Tipo = "cabecera"
	// TipoFin — el `NNNN` de fin de mensaje. Solo sale por el ENCODER device.
	// Por el DECODER device **no llega nunca**, y esa es la razón de que este
	// driver no pueda cerrar solo una interrupción (ver sage.go).
	TipoFin Tipo = "fin"
	// TipoTexto — el texto expandido en cristiano que el ENDEC manda detrás de
	// la cabecera. Es lo que se le puede enseñar a una persona tal cual.
	TipoTexto Tipo = "texto"

	// TipoEnlaceCaido y TipoEnlaceVuelto los pone transporte.go, no el ENDEC:
	// se cayó el puerto o el socket, y volvió. T8 los mapea a
	// model.IncEnlaceCaido; aquí no se toca el catálogo de incidentes.
	TipoEnlaceCaido  Tipo = "enlace_caido"
	TipoEnlaceVuelto Tipo = "enlace_vuelto"
)

// Clase separa las pruebas de lo real. Los valores son exactamente los tres de
// la columna `tipo` de la tabla alert_event (PRD §15), porque una prueba
// semanal o mensual **no cuenta como interrupción y no genera make-goods**: si
// aquí se dijera «real» a una RWT, la reposición de anuncios de F4 se
// inventaría trabajo cada martes.
type Clase string

const (
	ClaseReal          Clase = "real"
	ClasePruebaSemanal Clase = "prueba_semanal"
	ClasePruebaMensual Clase = "prueba_mensual"
)

// Evento es una línea del ENDEC ya entendida.
type Evento struct {
	Tipo Tipo
	// Cabecera es la cabecera SAME parseada, o nil si la línea no traía una o
	// venía malformada. Nil no es un error: el Evento sale igual con su Crudo,
	// porque perder la noticia de que el ENDEC habló es peor que perder los
	// campos.
	Cabecera *CabeceraSAME
	// Clase es real, prueba semanal o prueba mensual. Sin cabecera no se puede
	// saber y queda vacía — no se rellena con «real» por defecto, que sería
	// justo la mentira que prohíbe la regla 3 del contrato de drivers.
	Clase Clase
	// Texto es el texto expandido, solo en los eventos TipoTexto.
	Texto string
	// Crudo es la línea como llegó, sin el salto de línea y sin los bytes de
	// sincronismo. Se guarda entera a propósito: es la evidencia de la
	// bitácora de alertas (PRD §12, 24 meses) y lo único que sirve para
	// depurar un ENDEC con un firmware que nadie ha visto.
	Crudo string
	// Recibido es cuándo lo leímos nosotros. No se deduce de la cabecera: el
	// JJJHHMM de la cabecera es la hora del que **originó** la alerta, que
	// puede llevar minutos de camino entre estaciones, y el as-run necesita la
	// hora en que esta estación la vio.
	Recibido time.Time
}

// Interrumpe dice si este evento significa que el ENDEC está tomando el aire.
// Solo `local:` lo significa. Ver el punto 2 de la lista de sage.go.
func (e Evento) Interrumpe() bool { return e.Tipo == TipoLocal }

// EsPrueba dice si es una RWT o una RMT. Una prueba no interrumpe a efectos
// comerciales aunque el aire cambie de verdad.
func (e Evento) EsPrueba() bool {
	return e.Clase == ClasePruebaSemanal || e.Clase == ClasePruebaMensual
}

// CabeceraSAME es el `ZCZC-ORG-EEE-PSSCCC-PSSCCC…+TTTT-JJJHHMM-LLLLLLLL-` de
// 47 CFR 11.31, ya partido en campos.
type CabeceraSAME struct {
	// Origen es ORG: EAS (emisora o cable), CIV (autoridad civil), WXR
	// (servicio meteorológico), PEP (nivel presidencial), EAN.
	Origen string
	// Evento es EEE, el código de tres letras: RWT prueba semanal, RMT prueba
	// mensual, TOR aviso de tornado, y los demás de la tabla de 11.31.
	Evento string
	// Zonas son los PSSCCC, hasta 31 por cabecera.
	Zonas []Zona
	// Duracion es el `+TTTT`, que viene en HHMM. **Es la vigencia de la
	// alerta, no lo que va a durar la interrupción**: una TOR de media hora no
	// quiere decir media hora de ENDEC en el aire.
	Duracion time.Duration
	// Instante es el JJJHHMM en UTC: día del año y hora en que el que originó
	// la alerta la emitió. El protocolo no lleva año, así que se completa con
	// el año más cercano al reloj (ver instanteDelDiaJuliano).
	//
	// **La cabecera va en UTC y el texto expandido va en la hora local del
	// ENDEC**, y eso no es una contradicción sino dos relojes: el ejemplo del
	// propio manual §8.1 trae `1020638` (06:38 UTC) y debajo «beginning at
	// 02:38 am». Tomar la hora del texto como si fuera la de la cabecera
	// escribe el as-run corrido por el huso entero.
	Instante time.Time
	// Estacion es el LLLLLLLL, ocho caracteres rellenados con espacios en el
	// cable y aquí ya recortados. Es la señal de llamada del que originó.
	Estacion string
	// Crudo es la cadena ZCZC completa tal como venía.
	Crudo string
}

// Zona es un PSSCCC: parte del condado, estado y condado en código FIPS.
type Zona struct {
	// Crudo son los seis dígitos tal cual, que es lo que se guarda y se
	// compara; los tres campos de abajo son para enseñarlos.
	Crudo string
	// Parte es la P: 0 es el condado entero, 1 a 9 son los novenos que define
	// 11.31 (1 noroeste, 2 norte central, …).
	Parte int
	// Estado es SS y Condado es CCC, los códigos FIPS. 000 en Condado
	// significa el estado completo.
	Estado  int
	Condado int
}

// EsElEstadoCompleto dice si el condado es 000, o sea todo el estado.
func (z Zona) EsElEstadoCompleto() bool { return z.Condado == 0 }

func (z Zona) String() string { return z.Crudo }

// claseDelCodigo traduce EEE a la clase de alert_event. Solo RWT y RMT tienen
// un valor propio en el esquema; todo lo demás es «real».
//
// NPT (prueba periódica nacional) y DMO (demostración) son pruebas que no son
// ni semanal ni mensual, y el esquema del PRD no les deja hueco: caen en
// «real» a propósito, porque una NPT sí sale al aire de verdad y sí hay que
// probar que salió. Si T8 necesita distinguirlas, eso es una columna nueva y
// una decisión de producto, no una suposición de este parser.
func claseDelCodigo(eee string) Clase {
	switch strings.ToUpper(eee) {
	case "RWT":
		return ClasePruebaSemanal
	case "RMT":
		return ClasePruebaMensual
	default:
		return ClaseReal
	}
}

// ParsearCabecera lee una cadena ZCZC. La cadena puede venir con cualquier
// cosa delante: se empieza a leer en el primer «ZCZC».
//
// Es deliberadamente estricta con la forma (los campos tienen largo fijo en
// 11.31) y deliberadamente suave con el final: después del LLLLLLLL puede
// haber un guion, espacios de relleno o nada, y las tres cosas son válidas en
// el cable.
func ParsearCabecera(cadena string, ahora time.Time) (*CabeceraSAME, error) {
	i := strings.Index(cadena, "ZCZC")
	if i < 0 {
		return nil, fmt.Errorf("no hay ZCZC en %q", recortar(cadena))
	}
	crudo := strings.TrimRight(cadena[i:], " \t-")
	partes := strings.Split(cadena[i:], "-")
	// ZCZC, ORG, EEE, al menos una zona con su +TTTT, JJJHHMM, LLLLLLLL.
	if len(partes) < 6 {
		return nil, fmt.Errorf("cabecera a medias, solo %d campos en %q", len(partes), recortar(cadena[i:]))
	}

	org, eee := strings.TrimSpace(partes[1]), strings.TrimSpace(partes[2])
	if !esCodigoDeTres(org) {
		return nil, fmt.Errorf("ORG inválido %q", recortar(org))
	}
	if !esCodigoDeTres(eee) {
		return nil, fmt.Errorf("EEE inválido %q", recortar(eee))
	}

	// El `+TTTT` viene pegado a la última zona: 006013+0015. Se busca el campo
	// que lo trae, y todo lo anterior desde el índice 3 son zonas.
	corte := -1
	for j := 3; j < len(partes); j++ {
		if strings.Contains(partes[j], "+") {
			corte = j
			break
		}
	}
	if corte < 3 {
		return nil, fmt.Errorf("no encuentro el +TTTT en %q", recortar(cadena[i:]))
	}
	ultima, ttttTexto, _ := strings.Cut(partes[corte], "+")

	zonas := make([]Zona, 0, corte-2)
	for _, texto := range append(partes[3:corte:corte], ultima) {
		z, err := parsearZona(texto)
		if err != nil {
			return nil, err
		}
		zonas = append(zonas, z)
	}
	// 11.31 permite hasta 31 zonas. Más que eso es una cabecera corrupta o dos
	// pegadas, y vale más decirlo que quedarse con una lista absurda.
	if len(zonas) > 31 {
		return nil, fmt.Errorf("%d zonas en la cabecera y el máximo son 31", len(zonas))
	}

	duracion, err := parsearDuracion(ttttTexto)
	if err != nil {
		return nil, err
	}
	if corte+2 >= len(partes) {
		return nil, fmt.Errorf("falta el JJJHHMM o la señal de llamada en %q", recortar(cadena[i:]))
	}
	instante, err := instanteDelDiaJuliano(strings.TrimSpace(partes[corte+1]), ahora)
	if err != nil {
		return nil, err
	}
	estacion := strings.TrimSpace(partes[corte+2])
	if estacion == "" {
		return nil, fmt.Errorf("señal de llamada vacía en %q", recortar(cadena[i:]))
	}

	return &CabeceraSAME{
		Origen:   strings.ToUpper(org),
		Evento:   strings.ToUpper(eee),
		Zonas:    zonas,
		Duracion: duracion,
		Instante: instante,
		Estacion: estacion,
		Crudo:    crudo,
	}, nil
}

func esCodigoDeTres(s string) bool {
	if len(s) != 3 {
		return false
	}
	for i := 0; i < 3; i++ {
		if c := s[i]; c < 'A' || c > 'Z' {
			if c < 'a' || c > 'z' {
				return false
			}
		}
	}
	return true
}

func parsearZona(texto string) (Zona, error) {
	s := strings.TrimSpace(texto)
	if len(s) != 6 {
		return Zona{}, fmt.Errorf("zona %q: se esperan seis dígitos PSSCCC", recortar(s))
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return Zona{}, fmt.Errorf("zona %q: no son dígitos", recortar(s))
	}
	return Zona{
		Crudo:   s,
		Parte:   n / 100000,
		Estado:  (n / 1000) % 100,
		Condado: n % 1000,
	}, nil
}

// parsearDuracion lee el TTTT, que es HHMM y no minutos.
func parsearDuracion(texto string) (time.Duration, error) {
	s := strings.TrimSpace(texto)
	if len(s) != 4 {
		return 0, fmt.Errorf("duración %q: se esperan cuatro dígitos HHMM", recortar(s))
	}
	horas, err1 := strconv.Atoi(s[:2])
	minutos, err2 := strconv.Atoi(s[2:])
	if err1 != nil || err2 != nil || horas < 0 || minutos < 0 || minutos > 59 {
		return 0, fmt.Errorf("duración %q: no es HHMM", recortar(s))
	}
	return time.Duration(horas)*time.Hour + time.Duration(minutos)*time.Minute, nil
}

// instanteDelDiaJuliano completa el JJJHHMM con un año.
//
// El protocolo no manda año: manda día del año (001 a 366) y hora UTC. Así que
// hay que elegirlo, y la única regla que no se rompe en Nochevieja es tomar el
// año cuyo candidato quede más cerca del reloj. Un 31 de diciembre a las 23:55
// recibiendo el día 001 da el 1 de enero del año siguiente, y un 1 de enero
// recibiendo el día 365 da el 31 de diciembre del anterior.
func instanteDelDiaJuliano(texto string, ahora time.Time) (time.Time, error) {
	if len(texto) != 7 {
		return time.Time{}, fmt.Errorf("JJJHHMM %q: se esperan siete dígitos", recortar(texto))
	}
	dia, err1 := strconv.Atoi(texto[:3])
	hora, err2 := strconv.Atoi(texto[3:5])
	minuto, err3 := strconv.Atoi(texto[5:])
	if err1 != nil || err2 != nil || err3 != nil {
		return time.Time{}, fmt.Errorf("JJJHHMM %q: no son dígitos", recortar(texto))
	}
	if dia < 1 || dia > 366 || hora > 23 || minuto > 59 {
		return time.Time{}, fmt.Errorf("JJJHHMM %q: fuera de rango", recortar(texto))
	}
	if ahora.IsZero() {
		ahora = time.Now()
	}
	ahora = ahora.UTC()
	var elegido time.Time
	for _, año := range []int{ahora.Year() - 1, ahora.Year(), ahora.Year() + 1} {
		c := time.Date(año, 1, 1, hora, minuto, 0, 0, time.UTC).AddDate(0, 0, dia-1)
		if elegido.IsZero() || abs(c.Sub(ahora)) < abs(elegido.Sub(ahora)) {
			elegido = c
		}
	}
	return elegido, nil
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// recortar corta un texto para meterlo en un error sin vomitar dos mil
// caracteres de basura en la bitácora.
func recortar(s string) string {
	const tope = 80
	if len(s) <= tope {
		return s
	}
	return s[:tope] + "…"
}

// ── el analizador ─────────────────────────────────────────────────────────

const (
	// lineaMaxima es lo más largo que puede ser una línea antes de darla por
	// basura. El texto expandido del CGEN llega a 2000 caracteres con inglés y
	// español a la vez (§8.3), así que 8 KiB sobra y a la vez impide que un
	// puerto que escupe ruido sin un solo salto de línea se coma la memoria.
	lineaMaxima = 8 << 10
	// lineasDeTextoMaximas es cuántas líneas seguidas se aceptan como el texto
	// expandido de una cabecera. Detrás de una cabecera vienen dos o tres
	// líneas; cuarenta es holgura para el caso bilingüe. Pasado ese número, lo
	// que sigue llegando es ruido y se cuenta como ruido, no se etiqueta como
	// el texto de una alerta.
	lineasDeTextoMaximas = 40
)

// Ruido es lo que se tiró, para que un puerto mal configurado se pueda
// diagnosticar sin adivinar: si todo son bytes sueltos, el device type del
// ENDEC no es DECODER, o los baudios no son los que dice el manual.
type Ruido struct {
	// Lineas son las que no se pudieron entender como nada.
	Lineas int
	// Bytes son los de sincronismo y los no imprimibles que se quitaron.
	Bytes int
	// Truncadas son las veces que una línea pasó de lineaMaxima y se tiró.
	Truncadas int
}

// Analizador guarda lo que quedó a medias entre dos lecturas. No es seguro
// usarlo desde dos goroutines: el transporte lo usa desde una sola, la que lee
// el puerto.
type Analizador struct {
	// reloj es de dónde sale la hora, inyectable porque el año del JJJHHMM
	// depende de ella y una prueba no puede depender de la fecha de hoy.
	reloj func() time.Time

	resto         []byte
	lineasDeTexto int
	enNoticias    bool
	ruido         Ruido
}

// NuevoAnalizador. Un reloj nil vale por time.Now.
func NuevoAnalizador(reloj func() time.Time) *Analizador {
	if reloj == nil {
		reloj = time.Now
	}
	return &Analizador{reloj: reloj}
}

// Ruido devuelve la cuenta de lo descartado hasta ahora.
func (a *Analizador) Ruido() Ruido { return a.ruido }

// Escribir come un trozo de lo que llegó por el cable y devuelve los eventos
// que ya están completos. Nunca devuelve error: lo que no se entiende se
// cuenta en Ruido y lo que se entiende a medias sale como Evento con Cabecera
// nil. Y nunca entra en pánico, que es el contrato que de verdad importa
// cuando esto corre al lado del aire.
func (a *Analizador) Escribir(p []byte) []Evento {
	var eventos []Evento
	a.resto = append(a.resto, p...)
	for {
		// El ENDEC termina las líneas con CR LF; un capturador que las haya
		// pasado por un editor puede dejar solo una de las dos. Las dos cortan.
		i := indiceDeFinDeLinea(a.resto)
		if i < 0 {
			break
		}
		linea := a.resto[:i]
		a.resto = a.resto[i+1:]
		eventos = append(eventos, a.linea(linea)...)
	}
	// Una línea que crece sin fin no es una línea: es un puerto escupiendo
	// ruido, o los baudios equivocados. Se tira y se cuenta.
	if len(a.resto) > lineaMaxima {
		a.ruido.Truncadas++
		a.ruido.Bytes += len(a.resto)
		a.resto = a.resto[:0]
	}
	return eventos
}

// Cerrar entrega lo que quedara sin salto de línea al final del flujo. El
// último mensaje de un ENDEC al que le cortan el cable suele quedarse así.
func (a *Analizador) Cerrar() []Evento {
	if len(a.resto) == 0 {
		return nil
	}
	linea := a.resto
	a.resto = nil
	return a.linea(linea)
}

func indiceDeFinDeLinea(b []byte) int {
	for i, c := range b {
		if c == '\n' || c == '\r' {
			return i
		}
	}
	return -1
}

// linea clasifica una línea ya cortada.
func (a *Analizador) linea(cruda []byte) []Evento {
	texto, tirados := limpiar(cruda)
	a.ruido.Bytes += tirados
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return nil
	}
	ahora := a.reloj()

	// Los delimitadores del NEWS FEED (§8.4) no son un evento: son el sobre.
	// Dentro de ese sobre el texto en cristiano va **delante** de la cabecera,
	// al contrario que en el DECODER device, así que se marca el bloque para
	// no atribuirle esas líneas a la alerta anterior.
	switch {
	case strings.Contains(texto, "<ENDECSTART>"):
		a.enNoticias = true
		a.lineasDeTexto = 0
		return nil
	case strings.Contains(texto, "<ENDECEND>"):
		a.enNoticias = false
		return nil
	}

	if tipo, resto, ok := prefijoDeEstado(texto); ok {
		a.lineasDeTexto = 0
		return []Evento{a.deCabecera(tipo, resto, texto, ahora)}
	}
	if strings.Contains(texto, "ZCZC") {
		a.lineasDeTexto = 0
		return []Evento{a.deCabecera(TipoCabecera, texto, texto, ahora)}
	}
	if esFinDeMensaje(texto) {
		a.lineasDeTexto = 0
		return []Evento{{Tipo: TipoFin, Crudo: texto, Recibido: ahora}}
	}
	// Lo que sigue a una cabecera es su texto expandido, y lo que va dentro de
	// un bloque de noticias es texto también. Cualquier otra cosa es ruido:
	// etiquetar basura como «el texto de la alerta» es peor que tirarla.
	if a.enNoticias || (a.lineasDeTexto > 0 && a.lineasDeTexto <= lineasDeTextoMaximas) {
		if !a.enNoticias {
			a.lineasDeTexto++
		}
		return []Evento{{Tipo: TipoTexto, Texto: texto, Crudo: texto, Recibido: ahora}}
	}
	a.ruido.Lineas++
	return nil
}

// deCabecera arma el evento de una línea que trae (o debería traer) un ZCZC.
// Si la cabecera no se puede parsear, el evento sale igual: la noticia de que
// el ENDEC habló no se pierde por un campo raro.
func (a *Analizador) deCabecera(tipo Tipo, cadena, crudo string, ahora time.Time) Evento {
	e := Evento{Tipo: tipo, Crudo: crudo, Recibido: ahora}
	if cab, err := ParsearCabecera(cadena, ahora); err == nil {
		e.Cabecera = cab
		e.Clase = claseDelCodigo(cab.Evento)
		// Solo después de una cabecera buena tiene sentido esperar su texto.
		a.lineasDeTexto = 1
	} else {
		a.ruido.Lineas++
	}
	return e
}

// prefijoDeEstado reconoce `local:`, `match:`, `nomatch:` y `dup:` del DECODER
// device. Se mira sin distinguir mayúsculas porque el coste de equivocarse es
// perder una alerta y el de ser laxo es cero.
func prefijoDeEstado(linea string) (Tipo, string, bool) {
	i := strings.IndexByte(linea, ':')
	if i <= 0 {
		return "", "", false
	}
	switch strings.ToLower(strings.TrimSpace(linea[:i])) {
	case "local":
		return TipoLocal, linea[i+1:], true
	case "match":
		return TipoCoincide, linea[i+1:], true
	case "nomatch":
		return TipoNoCoincide, linea[i+1:], true
	case "dup":
		return TipoDuplicada, linea[i+1:], true
	}
	return "", "", false
}

// esFinDeMensaje reconoce el `NNNN`, y también la línea del ENCODER device que
// trae los tres seguidos —`NNNN NNNN NNNN` una vez quitado el sincronismo—,
// porque las tres repeticiones son **un solo** fin de mensaje y contarlas tres
// veces sería inventarse dos.
func esFinDeMensaje(linea string) bool {
	s := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '+' {
			return -1
		}
		return r
	}, linea)
	if s == "" || len(s)%4 != 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] != 'N' {
			return false
		}
	}
	return true
}

// limpiar quita de una línea lo que no es texto y devuelve cuántos bytes se
// fueron.
//
// Dos cosas se van:
//
//   - **las rachas de bytes de sincronismo.** El 0xAB de la parte 11 va
//     dieciséis veces delante de cada cabecera y de cada EOM (§8.1). No es un
//     carácter: guardarlo en la base o mandarlo por JSON rompe el UTF-8. Se
//     quitan las rachas de dos o más —nunca uno solo— porque 0xAB es también
//     la «« de Latin-1 y el ENDEC puede mandar texto en español. Lo mismo con
//     las rachas de `+`, que es como los pinta un terminal al copiarlas a
//     mano: una racha de `+` es sincronismo, y el `+` suelto del `+TTTT` se
//     queda donde está.
//   - **los bytes de control.** El STX/ETX del GENERIC CGEN (§8.3) y lo que
//     suelte un equipo al arrancar.
//
// Y si lo que queda no es UTF-8 válido, se lee como Latin-1, que es lo que
// manda un equipo de 1997 cuando le pides el texto en español.
func limpiar(cruda []byte) (string, int) {
	fuera := 0
	limpio := make([]byte, 0, len(cruda))
	for i := 0; i < len(cruda); {
		c := cruda[i]
		if c == 0xAB || c == '+' {
			j := i
			for j < len(cruda) && cruda[j] == c {
				j++
			}
			if racha := j - i; racha >= 2 {
				fuera += racha
				i = j
				continue
			}
		}
		// Se queda todo lo imprimible y el tabulador; el resto es control.
		if c == '\t' || (c >= 0x20 && c != 0x7F) {
			limpio = append(limpio, c)
		} else {
			fuera++
		}
		i++
	}
	if utf8.Valid(limpio) {
		return string(limpio), fuera
	}
	var b strings.Builder
	b.Grow(len(limpio))
	for _, c := range limpio {
		b.WriteRune(rune(c))
	}
	return b.String(), fuera
}
