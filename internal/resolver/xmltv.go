package resolver

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"

	"antena787/internal/model"
)

// XMLTVTime es el formato de fecha de XMLTV: "20260906090000 -0400".
const XMLTVTime = "20060102150405 -0700"

// MaxGuideGap es el hueco a partir del cual el validador se queja de que la
// guía deja aire sin describir (PRD §9 paso 3).
const MaxGuideGap = 30 * time.Minute

type xmlTV struct {
	XMLName    xml.Name       `xml:"tv"`
	Generator  string         `xml:"generator-info-name,attr,omitempty"`
	SourceName string         `xml:"source-info-name,attr,omitempty"`
	Channels   []xmlChannel   `xml:"channel"`
	Programmes []xmlProgramme `xml:"programme"`
}

type xmlChannel struct {
	ID    string    `xml:"id,attr"`
	Names []xmlText `xml:"display-name"`
}

type xmlText struct {
	Lang  string `xml:"lang,attr,omitempty"`
	Value string `xml:",chardata"`
}

type xmlProgramme struct {
	Start    string        `xml:"start,attr"`
	Stop     string        `xml:"stop,attr"`
	Channel  string        `xml:"channel,attr"`
	Title    []xmlText     `xml:"title"`
	SubTitle []xmlText     `xml:"sub-title,omitempty"`
	Desc     []xmlText     `xml:"desc,omitempty"`
	Category []xmlText     `xml:"category,omitempty"`
	Episode  []xmlEpisode  `xml:"episode-num,omitempty"`
	Live     *xmlEmptyElem `xml:"live,omitempty"`
}

type xmlEpisode struct {
	System string `xml:"system,attr"`
	Value  string `xml:",chardata"`
}

type xmlEmptyElem struct{}

// ChannelID es el id que lleva el canal en la guía: sale del identificativo
// de la estación o, si no lo tiene, de su nombre. Nunca de una plantilla.
func ChannelID(ch model.Channel) string {
	base := slug(ch.CallSign)
	if base == "" {
		base = slug(ch.Name)
	}
	if base == "" {
		base = fmt.Sprintf("canal-%d", ch.ID)
	}
	return base + ".antena787"
}

func slug(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == 'á', r == 'à', r == 'ä', r == 'â':
			b.WriteRune('a')
			prevDash = false
		case r == 'é', r == 'è', r == 'ë', r == 'ê':
			b.WriteRune('e')
			prevDash = false
		case r == 'í', r == 'ì', r == 'ï', r == 'î':
			b.WriteRune('i')
			prevDash = false
		case r == 'ó', r == 'ò', r == 'ö', r == 'ô':
			b.WriteRune('o')
			prevDash = false
		case r == 'ú', r == 'ù', r == 'ü', r == 'û':
			b.WriteRune('u')
			prevDash = false
		case r == 'ñ':
			b.WriteRune('n')
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// XMLTV escribe la guía del canal a partir del plan. Solo publica lo que es
// contenido —archivo o vivo—: el relleno no se anuncia. Los instantes van en
// la zona horaria del canal, que es como los lee un receptor.
func XMLTV(ch model.Channel, items []model.PlanItem, titles map[int64]model.Title, episodes map[int64]model.Episode) ([]byte, error) {
	loc := ch.Location()
	id := ChannelID(ch)

	names := []xmlText{{Value: ch.Name}}
	if ch.CallSign != "" && !strings.EqualFold(ch.CallSign, ch.Name) {
		names = append(names, xmlText{Value: ch.CallSign})
	}
	doc := xmlTV{
		Generator:  "Antena787",
		SourceName: ch.Name,
		Channels:   []xmlChannel{{ID: id, Names: names}},
	}

	byAsset := map[int64]model.Title{}
	for _, t := range titles {
		if t.MediaAssetID != nil {
			byAsset[*t.MediaAssetID] = t
		}
	}

	ordered := append([]model.PlanItem(nil), items...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].PlannedAt.Before(ordered[j].PlannedAt) })

	for _, p := range ordered {
		if p.Origin != "asset" && p.Origin != "live_source" {
			continue
		}
		if p.PlannedMs <= 0 {
			continue
		}
		prog := xmlProgramme{
			Start:   p.PlannedAt.In(loc).Format(XMLTVTime),
			Stop:    p.End().In(loc).Format(XMLTVTime),
			Channel: id,
		}
		var title model.Title
		var ep model.Episode
		haveEp := false
		if p.EpisodeID != nil {
			if e, ok := episodes[*p.EpisodeID]; ok {
				ep, haveEp = e, true
				if t, ok := titles[e.TitleID]; ok {
					title = t
				}
			}
		}
		if title.Name == "" && p.MediaAssetID != nil {
			if t, ok := byAsset[*p.MediaAssetID]; ok {
				title = t
			}
		}
		switch {
		case title.Name != "":
			prog.Title = []xmlText{{Lang: "es", Value: title.Name}}
		case p.Origin == "live_source":
			prog.Title = []xmlText{{Lang: "es", Value: "En vivo"}}
		default:
			prog.Title = []xmlText{{Lang: "es", Value: ch.Name}}
		}
		if p.Origin == "live_source" {
			prog.Live = &xmlEmptyElem{}
		}
		if haveEp {
			if ep.Name != "" {
				prog.SubTitle = []xmlText{{Lang: "es", Value: ep.Name}}
			}
			prog.Episode = []xmlEpisode{
				{System: "xmltv_ns", Value: fmt.Sprintf("%d.%d.", maxInt(ep.Season-1, 0), maxInt(ep.Number-1, 0))},
				{System: "onscreen", Value: fmt.Sprintf("S%02dE%02d", ep.Season, ep.Number)},
			}
		}
		if title.Synopsis != "" {
			prog.Desc = []xmlText{{Lang: "es", Value: title.Synopsis}}
		}
		if title.Genre != "" {
			prog.Category = []xmlText{{Lang: "es", Value: title.Genre}}
		}
		doc.Programmes = append(doc.Programmes, prog)
	}

	body, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("no se pudo escribir la guía: %w", err)
	}
	var out []byte
	out = append(out, []byte(xml.Header)...)
	out = append(out, []byte("<!DOCTYPE tv SYSTEM \"xmltv.dtd\">\n")...)
	out = append(out, body...)
	out = append(out, '\n')
	return out, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ValidateXMLTV revisa la guía antes de publicarla y devuelve los problemas
// en cristiano: canal que no existe, programas solapados, huecos largos y
// fechas que no cuadran (PRD §9 paso 3, criterio F1-28). Lista vacía quiere
// decir que la guía se puede publicar.
func ValidateXMLTV(data []byte) []string {
	var doc xmlTV
	if err := xml.Unmarshal(data, &doc); err != nil {
		return []string{fmt.Sprintf("la guía no es XML válido: %v", err)}
	}
	var problems []string
	if len(doc.Channels) == 0 {
		problems = append(problems, "la guía no declara ningún canal")
	}
	known := map[string]string{}
	for _, c := range doc.Channels {
		if c.ID == "" {
			problems = append(problems, "hay un canal sin identificador")
			continue
		}
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimSpace(c.Names[0].Value)
		}
		if name == "" {
			problems = append(problems, fmt.Sprintf("el canal %s no tiene nombre", c.ID))
		}
		known[c.ID] = name
	}

	type span struct {
		from, to time.Time
		title    string
	}
	spans := map[string][]span{}
	for i, p := range doc.Programmes {
		label := fmt.Sprintf("el programa %d", i+1)
		if len(p.Title) > 0 && strings.TrimSpace(p.Title[0].Value) != "" {
			label = strings.TrimSpace(p.Title[0].Value)
		} else {
			problems = append(problems, fmt.Sprintf("%s no tiene título", label))
		}
		if _, ok := known[p.Channel]; !ok {
			problems = append(problems, fmt.Sprintf("%s dice ir en el canal %q, que la guía no declara", label, p.Channel))
			continue
		}
		from, err := time.Parse(XMLTVTime, p.Start)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s tiene una hora de inicio que no se entiende (%q)", label, p.Start))
			continue
		}
		to, err := time.Parse(XMLTVTime, p.Stop)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s tiene una hora de fin que no se entiende (%q)", label, p.Stop))
			continue
		}
		if !to.After(from) {
			problems = append(problems, fmt.Sprintf("%s termina (%s) antes o al mismo tiempo que empieza (%s)",
				label, to.Format("2006-01-02 15:04"), from.Format("2006-01-02 15:04")))
			continue
		}
		spans[p.Channel] = append(spans[p.Channel], span{from, to, label})
	}

	chans := make([]string, 0, len(spans))
	for id := range spans {
		chans = append(chans, id)
	}
	sort.Strings(chans)
	for _, id := range chans {
		list := spans[id]
		sort.Slice(list, func(i, j int) bool {
			if !list[i].from.Equal(list[j].from) {
				return list[i].from.Before(list[j].from)
			}
			return list[i].to.Before(list[j].to)
		})
		for i := 1; i < len(list); i++ {
			prev, cur := list[i-1], list[i]
			if cur.from.Before(prev.to) {
				problems = append(problems, fmt.Sprintf("%s y %s se pisan a las %s",
					prev.title, cur.title, cur.from.Format("2006-01-02 15:04")))
				continue
			}
			if gap := cur.from.Sub(prev.to); gap > MaxGuideGap {
				problems = append(problems, fmt.Sprintf("la guía deja %s sin describir entre %s y %s",
					humanDuration(gap), prev.to.Format("2006-01-02 15:04"), cur.from.Format("2006-01-02 15:04")))
			}
		}
	}
	return problems
}

// GuideAccuracy mide la exactitud de la guía contra lo que de verdad salió
// (PRD §23): la fracción de programas de la guía que aparecen en el as-run
// dentro de la tolerancia, ±30 s por defecto. 1 es la meta.
func GuideAccuracy(guide, actual []model.PlanItem, tolerance time.Duration) float64 {
	if len(guide) == 0 {
		return 1
	}
	if tolerance < 0 {
		tolerance = -tolerance
	}
	pending := map[string][]time.Time{}
	for _, a := range actual {
		at := a.PlannedAt
		if a.ActualAt != nil {
			at = *a.ActualAt
		}
		k := contentKey(a)
		pending[k] = append(pending[k], at)
	}
	for k := range pending {
		list := pending[k]
		sort.Slice(list, func(i, j int) bool { return list[i].Before(list[j]) })
		pending[k] = list
	}

	hits := 0
	for _, g := range guide {
		k := contentKey(g)
		list := pending[k]
		best := -1
		for i, at := range list {
			d := at.Sub(g.PlannedAt)
			if d < 0 {
				d = -d
			}
			if d <= tolerance {
				best = i
				break
			}
		}
		if best >= 0 {
			hits++
			pending[k] = append(list[:best], list[best+1:]...)
		}
	}
	return float64(hits) / float64(len(guide))
}

// contentKey identifica el contenido de un ítem para poder emparejar guía y
// as-run: el episodio si lo hay, si no el archivo, si no la fuente en vivo.
func contentKey(p model.PlanItem) string {
	switch {
	case p.EpisodeID != nil:
		return fmt.Sprintf("ep:%d", *p.EpisodeID)
	case p.MediaAssetID != nil:
		return fmt.Sprintf("as:%d", *p.MediaAssetID)
	case p.LiveSourceID != nil:
		return fmt.Sprintf("vivo:%d", *p.LiveSourceID)
	default:
		return "origen:" + p.Origin
	}
}
