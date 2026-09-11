package resolver

import (
	"encoding/xml"
	"fmt"
	"sort"
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
// 2008, reafirmado el 9 de febrero de 2016— en las secciones 5.2 (namespace),
// 5.3 (convenciones de nombres), 5.4 (PmcpMessage), 5.9.3 (Channel), 5.9.5
// (PsipEvent) y 5.9.9/5.9.10 (Rating y el texto en varios idiomas).
//
// El esquema XSD normativo (Anexo A: el zip "PPMCP31.zip",
// http://www.atsc.org/XMLSchemas/pmcp/2007/3.1) ya no está alojado en
// atsc.org —comprobado el 10 de septiembre de 2026, los tres enlaces que da
// el propio documento contestan 404—, así que este exportador no valida
// contra el XSD byte a byte: sigue el texto del estándar y el ejemplo que
// trae la propia norma (§5.2.1). ValidatePMCP hace las comprobaciones
// estructurales que reemplazan a ese esquema (PRD F1-28 es la misma idea
// para XMLTV).
const (
	// PMCPNamespace es el namespace del ejemplo normativo (A/76B §5.2.1):
	// "xmlns=http://www.atsc.org/pmcp/2006/3.0". La versión de esquema que
	// declara el Anexo A (3.1) no cambia este namespace.
	PMCPNamespace = "http://www.atsc.org/pmcp/2006/3.0"

	// pmcpOrigin es el nombre del dispositivo (A/76B §5.5.1): único dentro
	// de la instalación, no hace falta que sea único en el mundo.
	pmcpOrigin = "Antena787"
	// pmcpOriginType es uno de los tipos de la Tabla 5.1 del estándar;
	// Antena787 es el sistema de automatización que alimenta al generador
	// PSIP, así que es "Automation".
	pmcpOriginType = "Automation"
)

// pmcpMessage es el elemento raíz, "PmcpMessage" (A/76B §5.4): trae quién lo
// manda, cuándo, y de qué tipo es el mensaje. Antena787 solo manda mensajes
// de tipo "information" —no implementa el protocolo de conexión con
// acuse de recibo (§5.7): esto es transporte por archivo/HTTP, §4.2— así que
// nunca lleva PmcpReply.
type pmcpMessage struct {
	XMLName    xml.Name      `xml:"PmcpMessage"`
	Xmlns      string        `xml:"xmlns,attr"`
	ID         string        `xml:"id,attr"`
	Origin     string        `xml:"origin,attr"`
	OriginType string        `xml:"originType,attr"`
	DateTime   string        `xml:"dateTime,attr"`
	Type       string        `xml:"type,attr"`
	Channels   []pmcpChannel `xml:"Channel"`
	Events     []pmcpEvent   `xml:"PsipEvent,omitempty"`
}

// pmcpChannel es el elemento "Channel" (A/76B §5.9.3): el canal virtual
// que declara el sistema. El estándar exige que traiga channelNumber o
// sourceId; Antena787 no guarda el número mayor.menor de ATSC en el canal
// (internal/model.Channel no lo tiene hoy), así que usa sourceId, que el
// propio estándar prevé justo para esto: "system implementers may choose to
// implement sourceID for channel identification... in a closed system
// environment such as an individual station" (§5.9.3). El mapeo al número
// real de PSIP queda del lado del generador, como el TSID.
type pmcpChannel struct {
	SourceID  string `xml:"sourceId,attr"`
	ShortName string `xml:"ShortName,omitempty"`
}

// pmcpEvent es el elemento "PsipEvent" (A/76B §5.9.5): un bloque de la
// parrilla con su hora, su duración y de qué trata. El estándar pide el
// canal por "channelNumber"; como Channel usa sourceId (arriba), PsipEvent
// manda el mismo identificador ahí —es el mismo canal, con el mismo
// identificador propio en las dos partes del documento—.
type pmcpEvent struct {
	ChannelNumber string       `xml:"channelNumber,attr"`
	StartTime     string       `xml:"startTime,attr"`
	Duration      string       `xml:"duration,attr"`
	EventID       string       `xml:"EventId"`
	Show          pmcpShowData `xml:"ShowData"`
}

// pmcpShowData es el "ShowData" de un PsipEvent (A/76B §5.9.5): el estándar
// permite mandarlo suelto —sin un elemento "Show" aparte— cuando el evento
// no se comparte entre varias emisiones, que es siempre el caso aquí.
type pmcpShowData struct {
	Name        []pmcpText   `xml:"Name"`
	Description []pmcpText   `xml:"Description,omitempty"`
	Genre       []pmcpText   `xml:"Genre,omitempty"`
	Rating      []pmcpRating `xml:"Rating,omitempty"`
}

// pmcpText es un campo de los que el estándar llama "Multiple String
// Structure" (§5.9.10): un valor por idioma, con "lang" como en XMLTV.
type pmcpText struct {
	Lang  string `xml:"lang,attr,omitempty"`
	Value string `xml:",chardata"`
}

// pmcpRating es un "Rating" de la RRT (A/76B §5.9.9): "the strings of CEA
// 766 shall be used verbatim when encoding the content advisory descriptor
// using the Rating element attributes, dimension and value". La dimensión
// "Rating" es la de TV Parental Guidelines (TV-Y, TV-14, TV-MA…); Antena787
// solo tiene esa —clasificacion_contenido— así que es la única que manda.
type pmcpRating struct {
	Dimension string `xml:"dimension,attr"`
	Value     string `xml:"value,attr"`
}

// PMCP escribe la guía del canal en ATSC A/76 (PMCP) a partir del mismo plan
// y el mismo criterio que XMLTV: mismos ítems publicables (asset o vivo,
// nunca relleno ni cartel), mismo título por episodio o por archivo
// (resolveTitle). Los instantes van siempre en UTC —es el generador PSIP
// quien conoce la zona de emisión de su propio TSID, y el estándar mismo
// dice que "PMCP time will be ultimately referenced to UTC" (§5.10)—.
//
// Lo que cubre: el canal (sourceId, y el identificativo si el canal lo
// tiene), un PsipEvent por bloque con su hora de inicio y duración exactas,
// título, sinopsis, género —con Infantil/Children si infantil_core, F1-76—,
// y la clasificación de contenido en un Rating de la RRT de TV Parental
// Guidelines si el título la trae.
//
// Lo que no cubre, porque Antena787 no lo tiene en esta llamada: el número
// de canal mayor.menor de ATSC y el TSID/red del multiplexor (no están en
// el modelo hoy), subtítulos y pistas de audio por evento (media_asset no
// entra en esta función, igual que en XMLTV), el elemento "Show"
// independiente con su propio contentId (se manda ShowData dentro de cada
// evento, que el estándar permite igual, §5.9.5), y AcapDataService/ACAP
// (Antena787 no hace datacasting).
func PMCP(ch model.Channel, items []model.PlanItem, titles map[int64]model.Title, episodes map[int64]model.Episode) ([]byte, error) {
	id := ChannelID(ch)
	byAsset := titlesByAsset(titles)

	now := time.Now().UTC()
	doc := pmcpMessage{
		Xmlns:      PMCPNamespace,
		ID:         fmt.Sprintf("%d", now.UnixNano()),
		Origin:     pmcpOrigin,
		OriginType: pmcpOriginType,
		DateTime:   now.Format(time.RFC3339),
		Type:       "information",
		Channels:   []pmcpChannel{{SourceID: id, ShortName: ch.CallSign}},
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
		title, _, _ := resolveTitle(p, titles, episodes, byAsset)

		ev := pmcpEvent{
			ChannelNumber: id,
			StartTime:     p.PlannedAt.UTC().Format(time.RFC3339),
			Duration:      isoDuration(p.PlannedMs),
			EventID:       fmt.Sprintf("antena787-plan-%d", p.ID),
		}
		switch {
		case title.Name != "":
			ev.Show.Name = []pmcpText{{Lang: "es", Value: title.Name}}
		case p.Origin == "live_source":
			ev.Show.Name = []pmcpText{{Lang: "es", Value: "En vivo"}}
		default:
			ev.Show.Name = []pmcpText{{Lang: "es", Value: ch.Name}}
		}
		if title.Synopsis != "" {
			ev.Show.Description = []pmcpText{{Lang: "es", Value: title.Synopsis}}
		}
		if title.Genre != "" {
			ev.Show.Genre = append(ev.Show.Genre, pmcpText{Lang: "es", Value: title.Genre})
		}
		// Mismo criterio que XMLTV (F1-76): un programa de educación o
		// información para niños se anuncia como tal en las dos lenguas.
		if title.InfantilCore {
			ev.Show.Genre = append(ev.Show.Genre,
				pmcpText{Lang: "es", Value: "Infantil"},
				pmcpText{Lang: "en", Value: "Children"})
		}
		if title.ContentRating != "" {
			ev.Show.Rating = []pmcpRating{{Dimension: "Rating", Value: title.ContentRating}}
		}
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

// isoDuration escribe milisegundos en el formato de duración de XML Schema
// (xs:duration, "PnYnMnDTnHnMnS") que pide el estándar para sus campos de
// tiempo (A/76B §5.10, que remite a xmlschema-2#datetime). Antena787 nunca
// planea bloques de días completos, así que alcanza con horas, minutos y
// segundos.
func isoDuration(ms int64) string {
	total := int64((time.Duration(ms) * time.Millisecond).Round(time.Second) / time.Second)
	if total < 0 {
		total = 0
	}
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("PT%dH%dM%dS", h, m, s)
}

// ValidatePMCP hace las comprobaciones que reemplazan al XSD normativo, que
// ya no está disponible en atsc.org (ver el comentario de arriba): que el
// documento sea XML válido, que declare el namespace del estándar, y que
// cada evento traiga lo mínimo que un generador PSIP necesita para el
// EIT/ETT (a qué canal va, cuándo empieza, cuánto dura, su identificador y
// un título). Lista vacía quiere decir que el documento está bien armado.
func ValidatePMCP(data []byte) []string {
	var doc pmcpMessage
	if err := xml.Unmarshal(data, &doc); err != nil {
		return []string{fmt.Sprintf("el documento PMCP no es XML válido: %v", err)}
	}
	var problems []string
	if doc.XMLName.Local != "PmcpMessage" {
		problems = append(problems, "el documento no tiene a PmcpMessage como raíz")
	}
	if doc.Xmlns != PMCPNamespace {
		problems = append(problems, fmt.Sprintf("el documento no declara el namespace de PMCP (%s)", PMCPNamespace))
	}
	if doc.ID == "" || doc.Origin == "" {
		problems = append(problems, "el mensaje PMCP no trae id u origin")
	}
	if doc.DateTime == "" {
		problems = append(problems, "el mensaje PMCP no trae dateTime")
	} else if _, err := time.Parse(time.RFC3339, doc.DateTime); err != nil {
		problems = append(problems, fmt.Sprintf("el dateTime del mensaje no se entiende (%q)", doc.DateTime))
	}
	if len(doc.Channels) == 0 {
		problems = append(problems, "el documento no declara ningún canal")
	}
	for _, c := range doc.Channels {
		if c.SourceID == "" {
			problems = append(problems, "hay un canal sin sourceId")
		}
	}
	for i, ev := range doc.Events {
		label := fmt.Sprintf("el evento %d", i+1)
		if ev.ChannelNumber == "" {
			problems = append(problems, label+" no dice a qué canal pertenece")
		}
		if ev.StartTime == "" {
			problems = append(problems, label+" no tiene hora de inicio")
		} else if _, err := time.Parse(time.RFC3339, ev.StartTime); err != nil {
			problems = append(problems, fmt.Sprintf("%s tiene una hora de inicio que no se entiende (%q)", label, ev.StartTime))
		}
		if ev.Duration == "" {
			problems = append(problems, label+" no trae duración")
		}
		if ev.EventID == "" {
			problems = append(problems, label+" no trae EventId")
		}
		if len(ev.Show.Name) == 0 || strings.TrimSpace(ev.Show.Name[0].Value) == "" {
			problems = append(problems, label+" no tiene título")
		}
	}
	return problems
}
