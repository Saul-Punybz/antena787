package resolver

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"antena787/internal/model"
)

// TestPMCPDeUnPlanDeDosDias arma un plan a mano —dos episodios de una serie,
// una película al día siguiente, un ítem de cartel y un hueco sin nada— y
// comprueba que PMCP describe justo lo que tiene que describir (F1-27/F1-47
// para XMLTV, mismo criterio aquí): nada de relleno ni de cartel, las horas
// en UTC, la duración exacta, la marca de programación infantil de F1-76 y la
// clasificación de contenido en la dimensión que le toca de la RRT.
func TestPMCPDeUnPlanDeDosDias(t *testing.T) {
	f := newCAtv()
	f.serie(100, "Kojak", 2, 30*time.Minute)
	f.pelicula(200, "Una película", 90*time.Minute)

	// Kojak es infantil_core; la película trae clasificación de contenido.
	kojak := f.titles[100]
	kojak.InfantilCore = true
	f.titles[100] = kojak
	pelicula := f.titles[200]
	pelicula.ContentRating = "TV-14"
	f.titles[200] = pelicula

	ep1, ep2 := f.eps[100][0], f.eps[100][1]
	moviePelAsset := *f.titles[200].MediaAssetID

	day1 := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC) // 6:00 AM AST
	day2 := time.Date(2026, 9, 8, 23, 0, 0, 0, time.UTC)

	items := []model.PlanItem{
		{ID: 1, Origin: "asset", EpisodeID: &ep1.ID, PlannedAt: day1, PlannedMs: 30 * 60 * 1000},
		{ID: 2, Origin: "asset", EpisodeID: &ep2.ID, PlannedAt: day1.Add(30 * time.Minute), PlannedMs: 30 * 60 * 1000},
		// Un hueco de una hora entre Kojak y el cartel: no debe salir nada ahí.
		{ID: 3, Origin: "cartel", PlannedAt: day1.Add(time.Hour), PlannedMs: 5 * 60 * 1000},
		{ID: 4, Origin: "asset", MediaAssetID: &moviePelAsset, PlannedAt: day2, PlannedMs: 90 * 60 * 1000},
	}

	data, err := PMCP(f.ch, items, f.titles, f.episodeIndex())
	if err != nil {
		t.Fatalf("no se pudo escribir el PMCP: %v", err)
	}
	s := string(data)

	if !strings.Contains(s, `xmlns="`+PMCPNamespace+`"`) {
		t.Fatalf("falta el namespace del estándar:\n%s", s[:upTo(len(s), 400)])
	}
	if strings.Contains(s, "http://www.atsc.org/pmcp/") {
		t.Fatalf("el namespace de PMCP lleva «/XMLSchemas/» en medio; el de la ruta del archivo no vale:\n%s", s[:upTo(len(s), 400)])
	}
	if !strings.Contains(s, `<Channel channelNumber="`+NumeroDeCanalPMCPPorDefecto+`" shortName="CAtv">`) {
		t.Fatalf("el canal tiene que llevar su número y su identificativo corto como atributos:\n%s", s[:upTo(len(s), 800)])
	}
	if !strings.Contains(s, `<Name lang="spa">Caribbean Advantage TV</Name>`) {
		t.Fatalf("falta el nombre largo del canal:\n%s", s[:upTo(len(s), 800)])
	}
	if n := strings.Count(s, "<PsipEvent"); n != 3 {
		t.Fatalf("esperaba 3 eventos (2 episodios + 1 película, ni cartel ni hueco); salieron %d:\n%s", n, s)
	}
	if !strings.Contains(s, `<InitialSchedule startTime="2026-09-07T10:00:00Z">`) {
		t.Fatalf("la hora prevista del primer episodio tiene que ir en UTC dentro de EventId:\n%s", s[:upTo(len(s), 1200)])
	}
	if strings.Contains(s, `<PsipEvent startTime=`) {
		t.Fatalf("ningún bloque ha salido todavía: startTime del evento es la hora real y aquí no toca:\n%s", s)
	}
	if !strings.Contains(s, `<PmcpEventId creator="Antena787" id="1">`) {
		t.Fatalf("cada evento se identifica con el id del plan que lo creó:\n%s", s[:upTo(len(s), 1200)])
	}
	if !strings.Contains(s, `duration="PT30M"`) {
		t.Fatalf("la duración del episodio tiene que ser exacta (30 min):\n%s", s)
	}
	if !strings.Contains(s, `duration="PT1H30M"`) {
		t.Fatalf("la película del segundo día tiene que salir con su duración:\n%s", s)
	}
	if !strings.Contains(s, `<InitialSchedule startTime="2026-09-08T23:00:00Z">`) {
		t.Fatalf("la película del segundo día tiene que salir con su hora:\n%s", s)
	}
	if !strings.Contains(s, `<Name lang="spa">Kojak</Name>`) {
		t.Fatalf("falta el título de Kojak")
	}
	// PMCP no tiene elemento Genre: el género, el episodio y la marca de
	// infantil viajan en Description.
	if strings.Contains(s, "<Genre") {
		t.Fatalf("PMCP no tiene elemento Genre:\n%s", s)
	}
	if !strings.Contains(s, `<Description lang="spa">Episodio 1 · Animación · Infantil · Serie de Kojak</Description>`) {
		t.Fatalf("el episodio, el género y la marca de infantil van en Description:\n%s", s)
	}
	if !strings.Contains(s, `<Description lang="eng">Children</Description>`) {
		t.Fatalf("Kojak es infantil_core: la marca tiene que salir también en inglés (F1-76):\n%s", s)
	}
	if !strings.Contains(s, `<ParentalRating region="1">`) || !strings.Contains(s, `<Rating dimension="Entire Audience" value="TV-14">`) {
		t.Fatalf("TV-14 es la dimensión «Entire Audience» de la RRT de EE. UU.:\n%s", s)
	}
	if !strings.Contains(s, `<Name lang="spa">Una pel`) {
		t.Fatalf("falta el título de la película")
	}
	// Sin fichas de archivo el documento no afirma nada sobre subtítulos.
	if strings.Contains(s, "<Captions>") {
		t.Fatalf("sin las fichas de los archivos nadie sabe si hay subtítulos, así que no se dice:\n%s", s)
	}

	if problemas := ValidatePMCP(data); len(problemas) != 0 {
		t.Fatalf("un PMCP bien armado tiene que pasar el validador: %v", problemas)
	}

	// El documento tiene que ser XML bien formado de punta a punta, no solo
	// que unos strings.Contains coincidan por casualidad.
	var doc pmcpMessage
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("el documento no es XML válido: %v", err)
	}
	if len(doc.Events) != 3 {
		t.Fatalf("al deserializar salieron %d eventos, no 3", len(doc.Events))
	}
	if doc.ID == 0 {
		t.Fatal("el mensaje tiene que traer un id, y tiene que caber en un entero de 32 bits sin signo")
	}
}

// TestPMCPSinTitulo comprueba que un bloque en vivo sin título fichado sale
// como "En vivo", y que sin ninguna referencia sale el nombre del canal: lo
// mismo que hace XMLTV, para que las dos guías no se contradigan.
func TestPMCPSinTitulo(t *testing.T) {
	f := newCAtv()
	at := time.Date(2026, 9, 7, 18, 0, 0, 0, time.UTC)
	items := []model.PlanItem{
		{ID: 9, Origin: "live_source", PlannedAt: at, PlannedMs: 3600_000},
	}
	data, err := PMCP(f.ch, items, f.titles, f.episodeIndex())
	if err != nil {
		t.Fatalf("no se pudo escribir el PMCP: %v", err)
	}
	if !strings.Contains(string(data), `<Name lang="spa">En vivo</Name>`) {
		t.Fatalf("un vivo sin título tiene que anunciarse como «En vivo»:\n%s", data)
	}
}

// TestPMCPLaHoraRealSoloCuandoYaSalio comprueba el reparto que manda el
// esquema: la hora prevista vive en InitialSchedule y el startTime del
// evento es la hora real, que solo se escribe cuando de verdad difiere.
func TestPMCPLaHoraRealSoloCuandoYaSalio(t *testing.T) {
	f := newCAtv()
	f.pelicula(300, "Tarde de cine", time.Hour)
	asset := *f.titles[300].MediaAssetID
	previsto := time.Date(2026, 9, 7, 14, 0, 0, 0, time.UTC)
	real := previsto.Add(3 * time.Minute)

	items := []model.PlanItem{
		{ID: 11, Origin: "asset", MediaAssetID: &asset, PlannedAt: previsto, PlannedMs: 3600_000, ActualAt: &real},
	}
	data, err := PMCP(f.ch, items, f.titles, f.episodeIndex())
	if err != nil {
		t.Fatalf("no se pudo escribir el PMCP: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `startTime="2026-09-07T14:03:00Z"`) {
		t.Fatalf("el bloque salió tres minutos tarde: eso es el startTime del evento:\n%s", s)
	}
	if !strings.Contains(s, `<InitialSchedule startTime="2026-09-07T14:00:00Z">`) {
		t.Fatalf("la hora prevista se sigue mandando aparte:\n%s", s)
	}
	if problemas := ValidatePMCP(data); len(problemas) != 0 {
		t.Fatalf("tenía que pasar el validador: %v", problemas)
	}
}

// TestPMCPConSubtitulosYNumeroDeCanal comprueba lo que PMCPCompleto añade:
// el número de canal real del generador PSIP y el servicio de subtítulos de
// cada bloque, que es lo que decide si el televidente puede encenderlos.
func TestPMCPConSubtitulosYNumeroDeCanal(t *testing.T) {
	f := newCAtv()
	f.pelicula(400, "Con subtítulos", time.Hour)
	f.pelicula(401, "Sin subtítulos", time.Hour)

	conSubs := *f.titles[400].MediaAssetID
	a := f.assets[conSubs]
	a.HasCaptions = true
	a.CaptionFormat = "cea-608"
	f.assets[conSubs] = a
	sinSubs := *f.titles[401].MediaAssetID

	at := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	items := []model.PlanItem{
		{ID: 21, Origin: "asset", MediaAssetID: &conSubs, PlannedAt: at, PlannedMs: 3600_000},
		{ID: 22, Origin: "asset", MediaAssetID: &sinSubs, PlannedAt: at.Add(time.Hour), PlannedMs: 3600_000},
	}
	data, err := PMCPCompleto(f.ch, "57-2", items, f.titles, f.episodeIndex(), f.assets)
	if err != nil {
		t.Fatalf("no se pudo escribir el PMCP: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `channelNumber="57-2"`) {
		t.Fatalf("el número de canal que se le pasa tiene que llegar al documento:\n%s", s)
	}
	if strings.Contains(s, `channelNumber="`+NumeroDeCanalPMCPPorDefecto+`"`) {
		t.Fatalf("con número de canal real no se usa el de por defecto:\n%s", s)
	}
	if !strings.Contains(s, `<Caption708 service="1" lang="spa">`) {
		t.Fatalf("el archivo trae subtítulos: tiene que salir el servicio 708:\n%s", s)
	}
	if !strings.Contains(s, "<Caption608>") {
		t.Fatalf("el archivo trae 608 embebido y eso sale al aire tal cual:\n%s", s)
	}
	if !strings.Contains(s, "<Null>") {
		t.Fatalf("del archivo sin subtítulos el documento sí puede decir que no los lleva:\n%s", s)
	}
	if problemas := ValidatePMCP(data); len(problemas) != 0 {
		t.Fatalf("tenía que pasar el validador: %v", problemas)
	}

	if _, err := PMCPCompleto(f.ch, "57.2", items, f.titles, f.episodeIndex(), f.assets); err == nil {
		t.Fatal("«57.2» no es un número de canal de PMCP: va con guion, no con punto")
	}
}

// TestClasificacionPMCP cubre la traducción del campo de la ficha a las
// dimensiones de la RRT de EE. UU. (ejemplo USRatingTable.xml del estándar).
func TestClasificacionPMCP(t *testing.T) {
	cases := []struct {
		ficha  string
		quiere []pmcpRating
	}{
		{"", nil},
		{"TV-G", []pmcpRating{{"Entire Audience", "TV-G"}}},
		{"TV-MA", []pmcpRating{{"Entire Audience", "TV-MA"}}},
		{"TV-Y", []pmcpRating{{"Children", "TV-Y"}}},
		{"TV-Y7", []pmcpRating{{"Children", "TV-Y7"}}},
		{"TV-Y7-FV", []pmcpRating{{"Children", "TV-Y7"}, {"Fantasy Violence", "FV"}}},
		{"TV-14-DLV", []pmcpRating{{"Entire Audience", "TV-14"}, {"Dialogue", "D"}, {"Language", "L"}, {"Violence", "V"}}},
		{"tv-pg l", []pmcpRating{{"Entire Audience", "TV-PG"}, {"Language", "L"}}},
		{"PG-13", []pmcpRating{{"MPAA", "PG-13"}}},
		{"NR", []pmcpRating{{"MPAA", "NR"}}},
		// Lo que no se reconoce no se manda: vale más sin clasificación que
		// con un descriptor que el receptor no sabe leer.
		{"apta para todos", nil},
	}
	for _, c := range cases {
		got := clasificacionPMCP(c.ficha)
		if len(got) != len(c.quiere) {
			t.Fatalf("clasificacionPMCP(%q) = %v, se esperaba %v", c.ficha, got, c.quiere)
		}
		for i := range got {
			if got[i] != c.quiere[i] {
				t.Fatalf("clasificacionPMCP(%q)[%d] = %v, se esperaba %v", c.ficha, i, got[i], c.quiere[i])
			}
		}
	}
}

// TestValidadorDePMCP cubre lo que ValidatePMCP tiene que cazar. Cada caso es
// un documento al que le falla una sola cosa, para que la prueba pueda
// fallar de verdad y no por arrastre de otro problema.
func TestValidadorDePMCP(t *testing.T) {
	// bien es un documento correcto; cada caso lo estropea en un punto.
	bien := `<PmcpMessage xmlns="` + PMCPNamespace + `" id="7" origin="Antena787" originType="Automation" dateTime="2026-09-07T09:00:00Z" type="information">` +
		`<Channel channelNumber="57-2" shortName="CAtv"><Name lang="spa">Caribbean Advantage TV</Name></Channel>` +
		`<PsipEvent duration="PT30M"><EventId channelNumber="57-2"><InitialSchedule startTime="2026-09-07T10:00:00Z"/></EventId>` +
		`<ShowData><Name lang="spa">Kojak</Name></ShowData></PsipEvent>` +
		`</PmcpMessage>`
	if problemas := ValidatePMCP([]byte(bien)); len(problemas) != 0 {
		t.Fatalf("el documento de referencia tiene que estar bien: %v", problemas)
	}

	cases := []struct {
		name string
		xml  string
		want string
	}{
		{"no es XML", `<PmcpMessage`, "no es XML válido"},
		{"la raíz no es PmcpMessage", `<Guia></Guia>`, "no es XML válido"},
		{
			"namespace de la ruta en vez del del esquema",
			strings.Replace(bien, PMCPNamespace, "http://www.atsc.org/pmcp/2006/3.0", 1),
			"namespace",
		},
		{"sin id", strings.Replace(bien, ` id="7"`, "", 1), "no trae id"},
		{"sin originType", strings.Replace(bien, ` originType="Automation"`, "", 1), "originType"},
		{"tipo de mensaje inventado", strings.Replace(bien, `type="information"`, `type="aviso"`, 1), "no existe en PMCP"},
		{"dateTime ilegible", strings.Replace(bien, `dateTime="2026-09-07T09:00:00Z"`, `dateTime="ayer"`, 1), "dateTime del mensaje"},
		{"canal sin nombre", strings.Replace(bien, `<Name lang="spa">Caribbean Advantage TV</Name>`, "", 1), "no tiene nombre"},
		{
			"identificativo corto de más de siete",
			strings.Replace(bien, `shortName="CAtv"`, `shortName="Caribbean"`, 1),
			"7 caracteres",
		},
		{
			"número de canal con punto",
			strings.ReplaceAll(bien, `channelNumber="57-2"`, `channelNumber="57.2"`),
			"número que PMCP no acepta",
		},
		{
			"el evento va a un canal que no se declara",
			strings.Replace(bien, `<EventId channelNumber="57-2">`, `<EventId channelNumber="12">`, 1),
			"que el documento no declara",
		},
		{
			"idioma de dos letras, como en XMLTV",
			strings.Replace(bien, `<Name lang="spa">Kojak</Name>`, `<Name lang="es">Kojak</Name>`, 1),
			"tres letras",
		},
		{
			"evento sin hora",
			strings.Replace(bien, `<InitialSchedule startTime="2026-09-07T10:00:00Z"/>`, "", 1),
			"hora de inicio",
		},
		{
			"hora prevista ilegible",
			strings.Replace(bien, `startTime="2026-09-07T10:00:00Z"`, `startTime="a las diez"`, 1),
			"hora prevista",
		},
		{"evento sin duración", strings.Replace(bien, ` duration="PT30M"`, "", 1), "no trae duración"},
		{
			"duración que no es xs:duration",
			strings.Replace(bien, `duration="PT30M"`, `duration="30 min"`, 1),
			"duración que no se entiende",
		},
		{"duración vacía de partes", strings.Replace(bien, `duration="PT30M"`, `duration="PT"`, 1), "duración que no se entiende"},
		{"evento sin título", strings.Replace(bien, `<Name lang="spa">Kojak</Name>`, "", 1), "no tiene título"},
		{
			"servicio de subtítulos fuera de rango",
			strings.Replace(bien, `</ShowData>`, `<Captions><Caption708 service="99" lang="spa"/></Captions></ShowData>`, 1),
			"fuera de rango",
		},
		{
			"dice a la vez que sí y que no lleva subtítulos",
			strings.Replace(bien, `</ShowData>`, `<Captions><Null/><Caption708 service="1"/></Captions></ShowData>`, 1),
			"a la vez",
		},
		{
			"dos bloques que se pisan",
			strings.Replace(bien, `</PmcpMessage>`,
				`<PsipEvent duration="PT30M"><EventId channelNumber="57-2"><InitialSchedule startTime="2026-09-07T10:15:00Z"/></EventId>`+
					`<ShowData><Name lang="spa">Otro</Name></ShowData></PsipEvent></PmcpMessage>`, 1),
			"se pisan",
		},
	}
	for _, c := range cases {
		problemas := ValidatePMCP([]byte(c.xml))
		found := false
		for _, p := range problemas {
			if strings.Contains(p, c.want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: esperaba un problema que dijera %q y salió %v", c.name, c.want, problemas)
		}
	}
}

// TestPMCPElHuecoAvisaPeroNoTumbaLaPublicacion es el mismo criterio que el
// validador del XMLTV: lo grave no sale; los huecos se publican y se avisan.
// En PSIP un hueco es literalmente la guía en blanco del televisor, así que
// se avisa; pero un canal que todavía no está lleno los deja a propósito.
func TestPMCPElHuecoAvisaPeroNoTumbaLaPublicacion(t *testing.T) {
	conHueco := `<PmcpMessage xmlns="` + PMCPNamespace + `" id="7" origin="Antena787" originType="Automation" dateTime="2026-09-07T09:00:00Z" type="information">` +
		`<Channel channelNumber="5" shortName="CAtv"><Name lang="spa">Caribbean Advantage TV</Name></Channel>` +
		`<PsipEvent duration="PT30M"><EventId channelNumber="5"><InitialSchedule startTime="2026-09-07T10:00:00Z"/></EventId>` +
		`<ShowData><Name lang="spa">Kojak</Name></ShowData></PsipEvent>` +
		`<PsipEvent duration="PT30M"><EventId channelNumber="5"><InitialSchedule startTime="2026-09-07T14:00:00Z"/></EventId>` +
		`<ShowData><Name lang="spa">Otro</Name></ShowData></PsipEvent>` +
		`</PmcpMessage>`

	graves, avisos := ValidatePMCPPorGravedad([]byte(conHueco))
	if len(graves) != 0 {
		t.Fatalf("un hueco no impide publicar: %v", graves)
	}
	if len(avisos) == 0 || !strings.Contains(avisos[0], "sin describir") {
		t.Fatalf("un hueco de tres horas y media tiene que avisarse: %v", avisos)
	}
	if problemas := ValidatePMCP([]byte(conHueco)); len(problemas) != 0 {
		t.Fatalf("ValidatePMCP es la puerta de la publicación y solo trae lo grave: %v", problemas)
	}
}

// TestIsoDuration comprueba el formato de duración contra casos redondos. Se
// escribe en la forma corta de los ejemplos del estándar, y un bloque que
// existe nunca se anuncia con duración cero.
func TestIsoDuration(t *testing.T) {
	cases := []struct {
		ms   int64
		want string
	}{
		{0, "PT0S"},
		{400, "PT1S"},
		{1000, "PT1S"},
		{30 * 60 * 1000, "PT30M"},
		{90 * 60 * 1000, "PT1H30M"},
		{25 * 3600 * 1000, "PT25H"},
		{3661 * 1000, "PT1H1M1S"},
	}
	for _, c := range cases {
		got := isoDuration(c.ms)
		if got != c.want {
			t.Fatalf("isoDuration(%d) = %q, esperaba %q", c.ms, got, c.want)
		}
		d, ok := leeDuracionPMCP(got)
		if !ok {
			t.Fatalf("isoDuration(%d) escribió %q y el propio lector no lo entiende", c.ms, got)
		}
		if c.ms > 0 && d <= 0 {
			t.Fatalf("isoDuration(%d) = %q, que se lee como cero", c.ms, got)
		}
	}
}

// TestLeeDuracionPMCP comprueba que el lector acepta las formas que escriben
// otros sistemas y rechaza lo que no es una duración medible.
func TestLeeDuracionPMCP(t *testing.T) {
	buenas := map[string]time.Duration{
		"PT30M":     30 * time.Minute,
		"PT0H30M0S": 30 * time.Minute,
		"PT1H30M":   90 * time.Minute,
		"P1DT2H":    26 * time.Hour,
		"PT0S":      0,
	}
	for s, want := range buenas {
		got, ok := leeDuracionPMCP(s)
		if !ok || got != want {
			t.Fatalf("leeDuracionPMCP(%q) = %v, %v; esperaba %v", s, got, ok, want)
		}
	}
	// Los años y los meses no entran: no se sabe cuánto dura un mes, así que
	// una guía que los use no es medible.
	for _, s := range []string{"", "P", "PT", "30M", "P1Y", "P2M", "PT30", "media hora"} {
		if _, ok := leeDuracionPMCP(s); ok {
			t.Fatalf("leeDuracionPMCP(%q) tenía que rechazarse", s)
		}
	}
}
