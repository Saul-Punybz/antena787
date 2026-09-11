package resolver

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"antena787/internal/model"
)

// PMCP es el formato que de verdad consume un generador PSIP (Triveni
// GuideBuilder y comparables): el estándar ATSC A/76B, «Programming
// Metadata Communication Protocol». XMLTV sirve para guías web o de IPTV;
// PMCP es la guía que le habla al equipo de la estación
// (docs/drivers/catalogo/03-multiplexores-psip-cortes.md §2). Este archivo
// sigue el texto normativo del estándar —Documento A/76B, 14 de enero de
// 2008, reafirmado el 9 de febrero de 2016— y, sobre todo, el esquema XSD
// del Anexo A, que es la parte normativa que decide si un documento vale o
// no.
//
// El esquema no está en atsc.org, pero sí en el repositorio oficial de
// esquemas de ATSC —https://www.atsc-schemas.org/pmcp/2006/3.0/, con los
// ejemplos del propio estándar en .../pmcp/2006/3/XMLSamples/, comprobado el
// 11 de septiembre de 2026—. Todo lo que este archivo afirma del formato
// sale de ahí, no de memoria: el detalle de la auditoría, elemento por
// elemento, está en docs/investigacion/PMCP-A76B-AUDITORIA-2026-09-11.md.
// ValidatePMCP hace las comprobaciones estructurales que reemplazan a correr
// el XSD (no hay validador de XSD en la biblioteca estándar de Go y no se
// añaden dependencias, ADR 0002/0003).
const (
	// PMCPNamespace es el targetNamespace del esquema de PMCP 3.0
	// (pmcp30.xsd, Anexo A) y el que declaran los ejemplos del estándar.
	// Ojo: lleva "/XMLSchemas/" en medio; la ruta donde está alojado el
	// archivo (/pmcp/2006/3.0/) no es el namespace.
	PMCPNamespace = "http://www.atsc.org/XMLSchemas/pmcp/2006/3.0"

	// pmcpOrigin es el nombre del dispositivo (A/76B §5.5.1): único dentro
	// de la instalación, no hace falta que sea único en el mundo.
	pmcpOrigin = "Antena787"
	// pmcpOriginType es uno de los tipos del registro de puntos de código de
	// ATSC; Antena787 es el sistema de automatización que alimenta al
	// generador PSIP, así que es "Automation".
	pmcpOriginType = "Automation"

	// pmcpRegionEEUU es la región de clasificación de los Estados Unidos en
	// la RRT (A/65 §6.4). Puerto Rico entra en ella: la tabla normativa se
	// llama "US (50 states + possessions)" (ejemplo USRatingTable.xml del
	// estándar). Es el `region` obligatorio de ParentalRating.
	pmcpRegionEEUU = 1

	// pmcpIdioma y pmcpIdiomaIngles son códigos de tres letras de ISO 639-2,
	// que es lo único que acepta el atributo `lang` del esquema
	// (pmcptype.xsd, languageType: patrón "[a-z]{3}"). "es"/"en" —los de
	// XMLTV— no valen aquí.
	pmcpIdioma       = "spa"
	pmcpIdiomaIngles = "eng"

	// NumeroDeCanalPMCPPorDefecto es lo que se manda como número de canal
	// virtual cuando nadie dijo cuál es. El esquema obliga a poner uno en
	// cada evento (event.xsd, EventIdType/@channelNumber, use="required") y
	// solo acepta un número de una parte ("5") o de dos ("57-2"): no hay
	// forma de decir "no lo sé". model.Channel todavía no guarda el número
	// mayor-menor de ATSC de la estación, así que el valor de una sola
	// parte "1" es el marcador de un canal único, y el generador PSIP lo
	// mapea al suyo igual que hace con el TSID. **En cuanto el modelo tenga
	// el número real hay que pasarlo por PMCPCompleto**: un generador que
	// no conozca el canal "1" descarta los eventos.
	NumeroDeCanalPMCPPorDefecto = "1"
)

// numeroDeCanalPMCP es el formato que el esquema acepta para un número de
// canal virtual (pmcptype.xsd, channelNumberType): una parte —"5", "1205"— o
// dos partes separadas por guion —"57-2"—. Con punto, "57.2", no vale.
var numeroDeCanalPMCP = regexp.MustCompile(`^(?:[1-9][0-9]{0,3}|[1-9][0-9]{0,2}-[0-9]{1,3})$`)

// idiomaPMCP es el `lang` de cualquier texto: tres letras minúsculas de
// ISO 639-2 (pmcptype.xsd, languageType).
var idiomaPMCP = regexp.MustCompile(`^[a-z]{3}$`)

// duracionPMCP lee una duración de xs:duration con las partes que Antena787
// puede escribir. Años y meses no entran a propósito: no se sabe cuánto dura
// un mes, así que una guía que los use no es medible.
var duracionPMCP = regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)

// pmcpMessage es el elemento raíz, "PmcpMessage" (pmcp30.xsd): trae quién lo
// manda, cuándo, y de qué tipo es el mensaje. Antena787 solo manda mensajes
// de tipo "information" —no implementa el protocolo de conexión con acuse de
// recibo (§5.7): esto es transporte por archivo/HTTP, §4.2— así que nunca
// lleva PmcpReply. El orden de los hijos es una xsd:sequence, no una
// elección: TransportStream, Channel, Show, PsipEvent… en ese orden, que es
// el orden en que están declarados los campos aquí abajo.
type pmcpMessage struct {
	XMLName    xml.Name      `xml:"PmcpMessage"`
	Xmlns      string        `xml:"xmlns,attr"`
	ID         uint32        `xml:"id,attr"`
	Origin     string        `xml:"origin,attr"`
	OriginType string        `xml:"originType,attr"`
	DateTime   string        `xml:"dateTime,attr"`
	Type       string        `xml:"type,attr"`
	Channels   []pmcpChannel `xml:"Channel"`
	Events     []pmcpEvent   `xml:"PsipEvent"`
}

// pmcpChannel es el elemento "Channel" (channel.xsd): el canal virtual que
// declara el sistema. El nombre largo va en el hijo "Name" y el corto en el
// atributo `shortName`, de siete caracteres como máximo —es el short_name
// del VCT (A/65 §6.3)—. `sourceId` existe, pero es un xsd:unsignedShort:
// no admite un identificador de texto, así que Antena787 no lo manda.
type pmcpChannel struct {
	ChannelNumber string     `xml:"channelNumber,attr,omitempty"`
	ShortName     string     `xml:"shortName,attr,omitempty"`
	Names         []pmcpText `xml:"Name"`
}

// pmcpEvent es el elemento "PsipEvent" (event.xsd): un bloque de la parrilla
// con su duración y de qué trata. El canal y la hora prevista **no** son
// atributos suyos: viven en el hijo EventId, que es obligatorio. Su propio
// `startTime` es, por definición del esquema, la hora **real** de salida
// "cuando es distinta de la prevista", así que aquí solo se llena cuando el
// bloque ya salió al aire.
type pmcpEvent struct {
	StartTime string       `xml:"startTime,attr,omitempty"`
	Duration  string       `xml:"duration,attr"`
	EventID   pmcpEventID  `xml:"EventId"`
	Show      pmcpShowData `xml:"ShowData"`
}

// pmcpEventID identifica el evento (event.xsd, EventIdType). El canal es
// obligatorio. De las formas de nombrar el evento, Antena787 usa las dos que
// el estándar recomienda para una descarga de parrilla: el identificador
// propio del sistema que lo creó —PmcpEventId, "preferred referencing
// method"— y la hora a la que estaba previsto —InitialSchedule—. Ese es el
// mismo par que usa el ejemplo ScheduleDownload.xml del estándar.
type pmcpEventID struct {
	ChannelNumber string               `xml:"channelNumber,attr"`
	Propio        *pmcpEventIDPropio   `xml:"PmcpEventId"`
	Previsto      *pmcpInitialSchedule `xml:"InitialSchedule"`
}

// pmcpEventIDPropio es "PmcpEventId": quién creó el evento y con qué número.
// El número es un xsd:unsignedInt, así que un plan_item con un id que no
// quepa en 32 bits sin signo se manda sin él, no con un número truncado.
type pmcpEventIDPropio struct {
	Creator string `xml:"creator,attr"`
	ID      uint32 `xml:"id,attr"`
}

// pmcpInitialSchedule es "InitialSchedule": la hora a la que el bloque
// estaba previsto.
type pmcpInitialSchedule struct {
	StartTime string `xml:"startTime,attr"`
}

// pmcpShowData es el "ShowData" de un PsipEvent (essencemetadata.xsd): el
// estándar permite mandarlo dentro del evento —sin un elemento "Show"
// aparte— cuando el contenido no se comparte entre varias emisiones, que es
// siempre el caso aquí. Sus hijos también van en orden fijo: Name,
// Description, ParentalRating, Audios, Captions…
//
// **No hay elemento "Genre" en PMCP.** El género de la ficha y la marca de
// programación infantil viajan dentro de Description, que es lo que sale por
// el ETT del evento.
type pmcpShowData struct {
	Name        []pmcpText           `xml:"Name"`
	Description []pmcpText           `xml:"Description"`
	Rating      []pmcpParentalRating `xml:"ParentalRating"`
	Captions    *pmcpCaptions        `xml:"Captions"`
}

// pmcpText es un campo de los que el estándar llama "Multiple String
// Structure" (A/65 §6.10): un valor por idioma. `lang` es obligatorio.
type pmcpText struct {
	Lang  string `xml:"lang,attr"`
	Value string `xml:",chardata"`
}

// pmcpParentalRating es la clasificación de una región (regionrating.xsd):
// el Content Advisory Descriptor del EIT (A/65 §6.9.4). Lleva tantas
// dimensiones como haga falta; `region` es obligatorio.
type pmcpParentalRating struct {
	Region  int          `xml:"region,attr"`
	Ratings []pmcpRating `xml:"Rating"`
}

// pmcpRating es una dimensión de la RRT. Los nombres de dimensión y de valor
// se escriben tal cual los trae la tabla —"Entire Audience", "Children",
// "MPAA", "Fantasy Violence"—, que es lo que pide A/76B §5.9.9 al remitir a
// CEA-766: "the strings of CEA 766 shall be used verbatim".
type pmcpRating struct {
	Dimension string `xml:"dimension,attr"`
	Value     string `xml:"value,attr,omitempty"`
}

// pmcpCaptions es el Caption Service Descriptor (captions.xsd, A/65 §6.9.3).
// O trae un "Null" —que quiere decir, dicho por el estándar, que este bloque
// no lleva servicio de subtítulos— o trae los servicios que lleva. Las dos
// cosas son información útil para el televidente; callarse no lo es.
type pmcpCaptions struct {
	SinServicio *vacio           `xml:"Null"`
	C608        *vacio           `xml:"Caption608"`
	C708        []pmcpCaption708 `xml:"Caption708"`
}

// pmcpCaption708 es un servicio de subtítulos digitales: su número del 1 al
// 63 y, si se sabe, su idioma.
type pmcpCaption708 struct {
	Service int    `xml:"service,attr"`
	Lang    string `xml:"lang,attr,omitempty"`
}

// vacio es un elemento sin contenido ni atributos, de los que el esquema usa
// como bandera ("Null", "Caption608").
type vacio struct{}

// PMCP escribe la guía del canal en ATSC A/76 (PMCP) a partir del mismo plan
// y el mismo criterio que XMLTV: mismos ítems publicables (asset o vivo,
// nunca relleno ni cartel), mismo título por episodio o por archivo
// (resolveTitle). Es PMCPCompleto sin el número de canal real y sin las
// fichas de los archivos, que es todo lo que la aplicación le sabe pasar
// hoy.
func PMCP(ch model.Channel, items []model.PlanItem, titles map[int64]model.Title, episodes map[int64]model.Episode) ([]byte, error) {
	return PMCPCompleto(ch, "", items, titles, episodes, nil)
}

// PMCPCompleto escribe la guía en PMCP sabiendo dos cosas más que PMCP:
//
//   - numeroDeCanal, el número del canal virtual tal como lo conoce el
//     generador PSIP ("5" o "57-2"). Vacío usa NumeroDeCanalPMCPPorDefecto.
//   - assets, las fichas de los archivos, indexadas por su id, para poder
//     decir si cada bloque lleva subtítulos. Nil quiere decir "no se sabe", y
//     entonces el documento no afirma ni que los lleva ni que no.
//
// Los instantes van siempre en UTC —es el generador PSIP quien conoce la
// zona de emisión de su propio TSID, y el estándar mismo dice que "PMCP time
// will be ultimately referenced to UTC" (§5.10)—.
//
// Lo que cubre: el canal (su nombre y su identificativo corto), un PsipEvent
// por bloque con su hora prevista, su hora real si ya salió, su duración,
// título, sinopsis, episodio y género, la marca de programación infantil en
// las dos lenguas (F1-76), la clasificación de contenido en las dimensiones
// de la RRT de EE. UU., y el servicio de subtítulos.
//
// Lo que no cubre, porque Antena787 no lo tiene: el TSID y la red del
// multiplexor, el sourceId numérico del VCT, las pistas de audio del evento
// (elemento Audios), el elemento "Show" independiente con su propio
// contentId, y AcapDataService/ACAP (Antena787 no hace datacasting).
func PMCPCompleto(ch model.Channel, numeroDeCanal string, items []model.PlanItem, titles map[int64]model.Title, episodes map[int64]model.Episode, assets map[int64]model.MediaAsset) ([]byte, error) {
	if numeroDeCanal == "" {
		numeroDeCanal = NumeroDeCanalPMCPPorDefecto
	}
	if !numeroDeCanalPMCP.MatchString(numeroDeCanal) {
		return nil, fmt.Errorf("el número de canal %q no es de los que acepta PMCP: una parte («5») o dos separadas por guion («57-2»)", numeroDeCanal)
	}
	byAsset := titlesByAsset(titles)

	now := time.Now().UTC()
	canal := pmcpChannel{
		ChannelNumber: numeroDeCanal,
		ShortName:     recorta(ch.CallSign, 7),
	}
	if nombre := strings.TrimSpace(ch.Name); nombre != "" {
		canal.Names = []pmcpText{{Lang: pmcpIdioma, Value: nombre}}
	}
	doc := pmcpMessage{
		Xmlns: PMCPNamespace,
		// El id del mensaje es un xsd:unsignedInt: los nanosegundos del reloj
		// no caben. Los segundos sí, y siguen subiendo mensaje a mensaje.
		ID:         uint32(now.Unix()),
		Origin:     pmcpOrigin,
		OriginType: pmcpOriginType,
		DateTime:   now.Format(time.RFC3339),
		Type:       "information",
		Channels:   []pmcpChannel{canal},
	}

	ordered := append([]model.PlanItem(nil), items...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].PlannedAt.Before(ordered[j].PlannedAt) })

	for _, p := range ordered {
		if !esContenidoPublicable(p.Origin) {
			continue
		}
		if p.PlannedMs <= 0 {
			continue
		}
		title, ep, haveEp := resolveTitle(p, titles, episodes, byAsset)

		ev := pmcpEvent{
			Duration: isoDuration(p.PlannedMs),
			EventID: pmcpEventID{
				ChannelNumber: numeroDeCanal,
				Previsto:      &pmcpInitialSchedule{StartTime: p.PlannedAt.UTC().Format(time.RFC3339)},
			},
		}
		if p.ID > 0 && p.ID <= 0xFFFFFFFF {
			ev.EventID.Propio = &pmcpEventIDPropio{Creator: pmcpOrigin, ID: uint32(p.ID)}
		}
		// La hora real solo se manda cuando de verdad difiere de la prevista:
		// el esquema define startTime como "actual start time… when different
		// from the scheduled start time", y repetir la prevista ahí sería
		// afirmar que ya salió cuando todavía no ha salido.
		if p.ActualAt != nil && !p.ActualAt.Equal(p.PlannedAt) {
			ev.StartTime = p.ActualAt.UTC().Format(time.RFC3339)
		}

		switch {
		case title.Name != "":
			ev.Show.Name = []pmcpText{{Lang: pmcpIdioma, Value: title.Name}}
		case p.Origin == "live_source":
			ev.Show.Name = []pmcpText{{Lang: pmcpIdioma, Value: "En vivo"}}
		default:
			ev.Show.Name = []pmcpText{{Lang: pmcpIdioma, Value: ch.Name}}
		}

		// PMCP no tiene ni subtítulo de episodio ni género: lo que en XMLTV
		// son <sub-title> y <category> aquí cabe todo en Description, que es
		// el texto que el receptor enseña cuando el televidente pide más
		// detalle de un bloque (el ETT del evento, A/65 §6.6).
		var partes []string
		if haveEp && ep.Name != "" {
			partes = append(partes, ep.Name)
		}
		if title.Genre != "" {
			partes = append(partes, title.Genre)
		}
		if title.InfantilCore {
			partes = append(partes, "Infantil")
		}
		if title.Synopsis != "" {
			partes = append(partes, title.Synopsis)
		}
		if len(partes) > 0 {
			ev.Show.Description = append(ev.Show.Description,
				pmcpText{Lang: pmcpIdioma, Value: strings.Join(partes, " · ")})
		}
		// Mismo criterio que XMLTV (F1-76): un programa de educación o
		// información para niños se anuncia como tal en las dos lenguas, para
		// que se vea desde fuera sin que nadie lo busque.
		if title.InfantilCore {
			ev.Show.Description = append(ev.Show.Description,
				pmcpText{Lang: pmcpIdiomaIngles, Value: "Children"})
		}

		if rs := clasificacionPMCP(title.ContentRating); len(rs) > 0 {
			ev.Show.Rating = []pmcpParentalRating{{Region: pmcpRegionEEUU, Ratings: rs}}
		}
		ev.Show.Captions = subtitulosPMCP(p, title, ep, haveEp, assets)

		doc.Events = append(doc.Events, ev)
	}

	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("no se pudo escribir la guía PMCP: %w", err)
	}
	out := append([]byte(xml.Header), body...)
	out = append(out, '\n')
	return out, nil
}

// recorta deja un texto en n caracteres como mucho, contando por runa para
// no partir una letra acentuada por la mitad.
func recorta(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n])
}

// archivoDelItem dice qué archivo sale al aire en un bloque: el que el plan
// nombra, si no el del episodio, si no el del título de un solo archivo.
func archivoDelItem(p model.PlanItem, title model.Title, ep model.Episode, haveEp bool) (int64, bool) {
	switch {
	case p.MediaAssetID != nil:
		return *p.MediaAssetID, true
	case haveEp && ep.MediaAssetID != nil:
		return *ep.MediaAssetID, true
	case title.MediaAssetID != nil:
		return *title.MediaAssetID, true
	}
	return 0, false
}

// subtitulosPMCP arma el Caption Service Descriptor del bloque. Devuelve nil
// —no manda el elemento— cuando no hay ficha del archivo que lo diga: el
// documento se calla en vez de afirmar que un bloque no lleva subtítulos
// solo porque nadie miró. Un vivo tampoco lleva ficha, y por eso se calla.
func subtitulosPMCP(p model.PlanItem, title model.Title, ep model.Episode, haveEp bool, assets map[int64]model.MediaAsset) *pmcpCaptions {
	if assets == nil {
		return nil
	}
	id, ok := archivoDelItem(p, title, ep, haveEp)
	if !ok {
		return nil
	}
	a, ok := assets[id]
	if !ok {
		return nil
	}
	lleva := a.HasCaptions || a.SubtitulosSidecar != "" ||
		(a.ExternalCaptions != nil && *a.ExternalCaptions != "")
	if !lleva {
		return &pmcpCaptions{SinServicio: &vacio{}}
	}
	c := &pmcpCaptions{
		// El servicio 1 es el principal de CEA-708, el que el receptor
		// enciende por defecto; es el que usan todos los ejemplos del
		// estándar. El idioma es el mismo que el de los títulos: lo que el
		// canal emite.
		C708: []pmcpCaption708{{Service: 1, Lang: pmcpIdioma}},
	}
	// Un archivo con 608 embebido lo sigue llevando al aire tal cual
	// (ingest/normalize.go pasa los datos A/53 con -a53cc), así que se
	// declara además del 708.
	if strings.Contains(strings.ToLower(a.CaptionFormat), "608") {
		c.C608 = &vacio{}
	}
	return c
}

// dimensionesRRT son las dimensiones de la tabla de clasificación de EE. UU.
// (A/65 §6.4; ejemplo USRatingTable.xml del estándar) a las que Antena787
// sabe traducir el campo clasificacion_contenido de la ficha.
var dimensionesRRT = []struct {
	valor     string // como se escribe en la ficha
	dimension string // nombre de la dimensión en la RRT
}{
	// El orden importa: "TV-Y7" tiene que probarse antes que "TV-Y".
	{"TV-Y7", "Children"},
	{"TV-Y", "Children"},
	{"TV-MA", "Entire Audience"},
	{"TV-14", "Entire Audience"},
	{"TV-PG", "Entire Audience"},
	{"TV-G", "Entire Audience"},
}

// subdimensionesRRT son las letras que acompañan a una clasificación de TV
// Parental Guidelines; cada una es una dimensión propia de la RRT.
var subdimensionesRRT = []struct {
	letra     string
	dimension string
}{
	{"FV", "Fantasy Violence"},
	{"D", "Dialogue"},
	{"L", "Language"},
	{"S", "Sex"},
	{"V", "Violence"},
}

// clasificacionesMPAA son los valores de la dimensión "MPAA" de la misma
// tabla, para las fichas de cine que traen la clasificación de la película.
var clasificacionesMPAA = map[string]bool{
	"G": true, "PG": true, "PG-13": true, "R": true,
	"NC-17": true, "X": true, "NR": true, "N/A": true,
}

// clasificacionPMCP traduce el campo clasificacion_contenido de la ficha a
// las dimensiones de la RRT de EE. UU. «TV-14» es la dimensión "Entire
// Audience"; «TV-Y7» es "Children"; «PG-13» es "MPAA"; y las letras que a
// veces siguen —«TV-14-DLV», «TV-Y7-FV»— son cada una su propia dimensión.
// Una clasificación que no se reconoce no se manda: vale más un bloque sin
// clasificación que un descriptor que el receptor no sabe leer.
func clasificacionPMCP(valor string) []pmcpRating {
	v := strings.ToUpper(strings.TrimSpace(valor))
	if v == "" {
		return nil
	}
	v = strings.Join(strings.Fields(strings.ReplaceAll(v, "_", " ")), "-")
	for strings.Contains(v, "--") {
		v = strings.ReplaceAll(v, "--", "-")
	}
	if clasificacionesMPAA[v] {
		return []pmcpRating{{Dimension: "MPAA", Value: v}}
	}
	for _, d := range dimensionesRRT {
		if !strings.HasPrefix(v, d.valor) {
			continue
		}
		out := []pmcpRating{{Dimension: d.dimension, Value: d.valor}}
		resto := strings.TrimPrefix(strings.TrimPrefix(v, d.valor), "-")
		for _, s := range subdimensionesRRT {
			if !strings.Contains(resto, s.letra) {
				continue
			}
			resto = strings.Replace(resto, s.letra, "", 1)
			out = append(out, pmcpRating{Dimension: s.dimension, Value: s.letra})
		}
		return out
	}
	return nil
}

// isoDuration escribe milisegundos en el formato de duración de XML Schema
// (xs:duration) que pide el esquema para el atributo `duration` del evento.
// Se escribe en la forma corta —"PT30M", "PT1H30M"— que es la que usan los
// ejemplos del propio estándar. Antena787 nunca planea bloques de días
// completos, así que alcanza con horas, minutos y segundos.
//
// Un bloque que existe nunca dura cero: el EIT mide en segundos enteros, así
// que medio segundo de programa se anuncia como un segundo y no como nada.
func isoDuration(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	total := int64((time.Duration(ms) * time.Millisecond).Round(time.Second) / time.Second)
	if total == 0 {
		if ms == 0 {
			return "PT0S"
		}
		total = 1
	}
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	out := "PT"
	if h > 0 {
		out += strconv.FormatInt(h, 10) + "H"
	}
	if m > 0 {
		out += strconv.FormatInt(m, 10) + "M"
	}
	if s > 0 {
		out += strconv.FormatInt(s, 10) + "S"
	}
	return out
}

// leeDuracionPMCP lee una duración de xs:duration. Devuelve false cuando no
// se entiende o cuando no trae ninguna parte ("P" o "PT" a secas).
func leeDuracionPMCP(s string) (time.Duration, bool) {
	m := duracionPMCP.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	var d time.Duration
	unidades := []time.Duration{0, 24 * time.Hour, time.Hour, time.Minute, time.Second}
	trajo := false
	for i := 1; i <= 4; i++ {
		if m[i] == "" {
			continue
		}
		f, err := strconv.ParseFloat(m[i], 64)
		if err != nil {
			return 0, false
		}
		d += time.Duration(f * float64(unidades[i]))
		trajo = true
	}
	if !trajo {
		return 0, false
	}
	return d, true
}

// ValidatePMCP revisa la guía en PMCP y devuelve lo **grave**: lo que hace
// que un generador PSIP la rechace o la entienda mal, y por lo tanto lo que
// nunca debe salir publicado. Lista vacía quiere decir que se puede
// publicar. Es el mismo criterio que el validador del XMLTV (F1-28), con una
// diferencia deliberada: ahí los avisos viajan en la misma lista y aquí no,
// porque esta lista es la puerta de la publicación. Los avisos —los tramos
// de aire sin describir, que en PSIP son exactamente la guía en blanco que
// ve el televidente— salen por ValidatePMCPPorGravedad.
func ValidatePMCP(data []byte) []string {
	graves, _ := ValidatePMCPPorGravedad(data)
	return graves
}

// ValidatePMCPPorGravedad es ValidatePMCP con los problemas partidos en dos:
// los graves (XML roto, namespace que no es, atributos obligatorios que
// faltan o no valen contra el esquema, eventos que se pisan) hacen que la
// guía no se publique; los avisos (aire sin describir por encima de
// MaxGuideGap) se publican igual y solo se avisan, porque un canal que
// todavía no está lleno deja huecos a propósito.
//
// Estas comprobaciones son las que reemplazan a correr el XSD del Anexo A;
// cada una dice de dónde sale en docs/investigacion/PMCP-A76B-AUDITORIA-2026-09-11.md.
func ValidatePMCPPorGravedad(data []byte) (graves, avisos []string) {
	var doc pmcpMessage
	if err := xml.Unmarshal(data, &doc); err != nil {
		return []string{fmt.Sprintf("el documento PMCP no es XML válido: %v", err)}, nil
	}
	// Unmarshal ya rechaza cualquier raíz que no sea PmcpMessage —el campo
	// XMLName lo exige—, así que aquí no hace falta volver a mirarlo.
	if doc.Xmlns != PMCPNamespace {
		graves = append(graves, fmt.Sprintf("el documento no declara el namespace de PMCP (%s) sino %q", PMCPNamespace, doc.Xmlns))
	}
	if doc.ID == 0 {
		graves = append(graves, "el mensaje PMCP no trae id")
	}
	if doc.Origin == "" {
		graves = append(graves, "el mensaje PMCP no dice quién lo manda (origin)")
	}
	if doc.OriginType == "" {
		graves = append(graves, "el mensaje PMCP no dice de qué tipo es el sistema que lo manda (originType)")
	}
	switch doc.Type {
	case "", "information", "request", "reply":
	default:
		graves = append(graves, fmt.Sprintf("el tipo de mensaje %q no existe en PMCP: solo valen information, request y reply", doc.Type))
	}
	if doc.DateTime == "" {
		graves = append(graves, "el mensaje PMCP no trae dateTime")
	} else if _, err := time.Parse(time.RFC3339, doc.DateTime); err != nil {
		graves = append(graves, fmt.Sprintf("el dateTime del mensaje no se entiende (%q)", doc.DateTime))
	}

	if len(doc.Channels) == 0 {
		graves = append(graves, "el documento no declara ningún canal")
	}
	canales := map[string]bool{}
	for i, c := range doc.Channels {
		etiqueta := fmt.Sprintf("el canal %d", i+1)
		if c.ChannelNumber == "" {
			graves = append(graves, etiqueta+" no trae número de canal")
		} else if !numeroDeCanalPMCP.MatchString(c.ChannelNumber) {
			graves = append(graves, fmt.Sprintf("%s tiene un número que PMCP no acepta (%q): va de una parte («5») o de dos con guion («57-2»)", etiqueta, c.ChannelNumber))
		} else {
			canales[c.ChannelNumber] = true
			etiqueta = "el canal " + c.ChannelNumber
		}
		if len([]rune(c.ShortName)) > 7 {
			graves = append(graves, fmt.Sprintf("%s tiene un identificativo corto de más de 7 caracteres (%q)", etiqueta, c.ShortName))
		}
		if len(c.Names) == 0 || strings.TrimSpace(c.Names[0].Value) == "" {
			graves = append(graves, etiqueta+" no tiene nombre")
		}
		graves = append(graves, problemasDeIdioma(etiqueta, c.Names)...)
	}

	type tramo struct {
		desde, hasta time.Time
		titulo       string
	}
	tramos := map[string][]tramo{}
	for i, ev := range doc.Events {
		etiqueta := fmt.Sprintf("el evento %d", i+1)
		if len(ev.Show.Name) > 0 && strings.TrimSpace(ev.Show.Name[0].Value) != "" {
			etiqueta = strings.TrimSpace(ev.Show.Name[0].Value)
		} else {
			graves = append(graves, etiqueta+" no tiene título")
		}
		graves = append(graves, problemasDeIdioma(etiqueta, ev.Show.Name)...)
		graves = append(graves, problemasDeIdioma(etiqueta, ev.Show.Description)...)

		canal := ev.EventID.ChannelNumber
		switch {
		case canal == "":
			graves = append(graves, etiqueta+" no dice a qué canal pertenece")
		case !numeroDeCanalPMCP.MatchString(canal):
			graves = append(graves, fmt.Sprintf("%s dice ir en un canal con un número que PMCP no acepta (%q)", etiqueta, canal))
		case !canales[canal]:
			graves = append(graves, fmt.Sprintf("%s dice ir en el canal %s, que el documento no declara", etiqueta, canal))
		}

		if ev.EventID.Propio != nil && ev.EventID.Propio.Creator == "" {
			graves = append(graves, etiqueta+" trae un identificador propio sin decir quién lo creó")
		}

		// La hora prevista y la real son campos distintos: sin ninguna de las
		// dos el generador PSIP no sabe dónde poner el bloque.
		var desde time.Time
		tieneHora := false
		if ev.EventID.Previsto != nil {
			t, err := time.Parse(time.RFC3339, ev.EventID.Previsto.StartTime)
			if err != nil {
				graves = append(graves, fmt.Sprintf("%s tiene una hora prevista que no se entiende (%q)", etiqueta, ev.EventID.Previsto.StartTime))
			} else {
				desde, tieneHora = t, true
			}
		}
		if ev.StartTime != "" {
			t, err := time.Parse(time.RFC3339, ev.StartTime)
			if err != nil {
				graves = append(graves, fmt.Sprintf("%s tiene una hora de salida que no se entiende (%q)", etiqueta, ev.StartTime))
			} else {
				desde, tieneHora = t, true
			}
		}
		if !tieneHora {
			graves = append(graves, etiqueta+" no tiene hora de inicio")
		}

		dur, okDur := leeDuracionPMCP(ev.Duration)
		switch {
		case ev.Duration == "":
			graves = append(graves, etiqueta+" no trae duración")
		case !okDur:
			graves = append(graves, fmt.Sprintf("%s tiene una duración que no se entiende (%q): PMCP la escribe como «PT30M»", etiqueta, ev.Duration))
		case dur <= 0:
			graves = append(graves, etiqueta+" dura cero")
		}

		for _, pr := range ev.Show.Rating {
			if pr.Region < 0 || pr.Region > 255 {
				graves = append(graves, fmt.Sprintf("%s trae una región de clasificación fuera de rango (%d)", etiqueta, pr.Region))
			}
			for _, r := range pr.Ratings {
				if strings.TrimSpace(r.Dimension) == "" {
					graves = append(graves, etiqueta+" trae una clasificación sin decir de qué dimensión es")
				}
			}
		}
		if c := ev.Show.Captions; c != nil {
			if c.SinServicio != nil && (c.C608 != nil || len(c.C708) > 0) {
				graves = append(graves, etiqueta+" dice a la vez que no lleva subtítulos y que sí")
			}
			for _, s := range c.C708 {
				if s.Service < 1 || s.Service > 63 {
					graves = append(graves, fmt.Sprintf("%s trae un servicio de subtítulos fuera de rango (%d): va del 1 al 63", etiqueta, s.Service))
				}
				if s.Lang != "" && !idiomaPMCP.MatchString(s.Lang) {
					graves = append(graves, fmt.Sprintf("%s trae un servicio de subtítulos con un idioma que no es de tres letras (%q)", etiqueta, s.Lang))
				}
			}
		}

		if tieneHora && okDur && dur > 0 && canal != "" {
			tramos[canal] = append(tramos[canal], tramo{desde, desde.Add(dur), etiqueta})
		}
	}

	canalesOrdenados := make([]string, 0, len(tramos))
	for c := range tramos {
		canalesOrdenados = append(canalesOrdenados, c)
	}
	sort.Strings(canalesOrdenados)
	for _, c := range canalesOrdenados {
		lista := tramos[c]
		sort.Slice(lista, func(i, j int) bool {
			if !lista[i].desde.Equal(lista[j].desde) {
				return lista[i].desde.Before(lista[j].desde)
			}
			return lista[i].hasta.Before(lista[j].hasta)
		})
		for i := 1; i < len(lista); i++ {
			previo, actual := lista[i-1], lista[i]
			if actual.desde.Before(previo.hasta) {
				graves = append(graves, fmt.Sprintf("%s y %s se pisan a las %s",
					previo.titulo, actual.titulo, actual.desde.Format("2006-01-02 15:04")))
				continue
			}
			if hueco := actual.desde.Sub(previo.hasta); hueco > MaxGuideGap {
				avisos = append(avisos, fmt.Sprintf("la guía deja %s sin describir entre %s y %s",
					humanDuration(hueco), previo.hasta.Format("2006-01-02 15:04"), actual.desde.Format("2006-01-02 15:04")))
			}
		}
	}
	return graves, avisos
}

// problemasDeIdioma comprueba el `lang` de una lista de textos: el esquema lo
// exige en todos y solo acepta tres letras minúsculas de ISO 639-2, así que
// el "es" de XMLTV aquí es un documento inválido.
func problemasDeIdioma(etiqueta string, textos []pmcpText) []string {
	var out []string
	for _, t := range textos {
		if t.Lang == "" {
			out = append(out, etiqueta+" trae un texto sin decir en qué idioma está")
			continue
		}
		if !idiomaPMCP.MatchString(t.Lang) {
			out = append(out, fmt.Sprintf("%s trae un texto en el idioma %q, y PMCP los escribe con tres letras («spa», «eng»)", etiqueta, t.Lang))
		}
	}
	return out
}
