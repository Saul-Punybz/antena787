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
// en UTC, la duración exacta, la categoría Infantil de F1-76 y la
// clasificación de contenido como Rating de la RRT.
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
	id := ChannelID(f.ch)
	if !strings.Contains(s, `<Channel sourceId="`+id+`">`) && !strings.Contains(s, `<Channel sourceId="`+id+`"`) {
		t.Fatalf("el canal tiene que llevar el identificador propio (%s):\n%s", id, s[:upTo(len(s), 800)])
	}
	if !strings.Contains(s, "<ShortName>CAtv</ShortName>") {
		t.Fatalf("falta el identificativo del canal (ShortName)")
	}
	if n := strings.Count(s, "<PsipEvent"); n != 3 {
		t.Fatalf("esperaba 3 eventos (2 episodios + 1 película, ni cartel ni hueco); salieron %d:\n%s", n, s)
	}
	if strings.Contains(s, "«cartel»") || strings.Contains(s, `channelNumber="`+id+`" startTime="2026-09-07T11:00:00Z"`) {
		t.Fatalf("el cartel no se anuncia en la guía:\n%s", s)
	}
	if !strings.Contains(s, `startTime="2026-09-07T10:00:00Z"`) {
		t.Fatalf("la hora del primer episodio tiene que ir en UTC:\n%s", s[:upTo(len(s), 800)])
	}
	if !strings.Contains(s, `duration="PT0H30M0S"`) {
		t.Fatalf("la duración del episodio tiene que ser exacta (30 min):\n%s", s)
	}
	if !strings.Contains(s, `startTime="2026-09-08T23:00:00Z"`) || !strings.Contains(s, `duration="PT1H30M0S"`) {
		t.Fatalf("la película del segundo día tiene que salir con su hora y duración:\n%s", s)
	}
	if !strings.Contains(s, "<Name lang=\"es\">Kojak</Name>") {
		t.Fatalf("falta el título de Kojak")
	}
	if !strings.Contains(s, "<Genre lang=\"es\">Infantil</Genre>") || !strings.Contains(s, "<Genre lang=\"en\">Children</Genre>") {
		t.Fatalf("Kojak es infantil_core: tiene que salir la categoría Infantil/Children (F1-76):\n%s", s)
	}
	if !strings.Contains(s, `<Rating dimension="Rating" value="TV-14"></Rating>`) {
		t.Fatalf("la película tiene clasificación de contenido: tiene que salir como Rating de la RRT:\n%s", s)
	}
	if !strings.Contains(s, "<Name lang=\"es\">Una pel") {
		t.Fatalf("falta el título de la película")
	}

	if problems := ValidatePMCP(data); len(problems) != 0 {
		t.Fatalf("un PMCP bien armado tiene que pasar el validador: %v", problems)
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
	if !strings.Contains(string(data), "<Name lang=\"es\">En vivo</Name>") {
		t.Fatalf("un vivo sin título tiene que anunciarse como «En vivo»:\n%s", data)
	}
}

// TestValidadorDePMCP cubre lo que ValidatePMCP tiene que cazar: no ser XML,
// no traer el namespace, y un evento al que le falta algo esencial.
func TestValidadorDePMCP(t *testing.T) {
	cases := []struct {
		name string
		xml  string
		want string
	}{
		{
			name: "no es XML",
			xml:  `<PmcpMessage`,
			want: "no es XML válido",
		},
		{
			name: "sin namespace",
			xml:  `<PmcpMessage id="1" origin="x" dateTime="2026-09-07T10:00:00Z" type="information"></PmcpMessage>`,
			want: "namespace",
		},
		{
			name: "evento sin hora",
			xml: `<PmcpMessage xmlns="` + PMCPNamespace + `" id="1" origin="x" dateTime="2026-09-07T10:00:00Z" type="information">` +
				`<Channel sourceId="catv"></Channel>` +
				`<PsipEvent channelNumber="catv" duration="PT0H30M0S"><EventId>e1</EventId><ShowData><Name>Kojak</Name></ShowData></PsipEvent>` +
				`</PmcpMessage>`,
			want: "hora de inicio",
		},
		{
			name: "evento sin titulo",
			xml: `<PmcpMessage xmlns="` + PMCPNamespace + `" id="1" origin="x" dateTime="2026-09-07T10:00:00Z" type="information">` +
				`<Channel sourceId="catv"></Channel>` +
				`<PsipEvent channelNumber="catv" startTime="2026-09-07T10:00:00Z" duration="PT0H30M0S"><EventId>e1</EventId><ShowData></ShowData></PsipEvent>` +
				`</PmcpMessage>`,
			want: "no tiene título",
		},
	}
	for _, c := range cases {
		problems := ValidatePMCP([]byte(c.xml))
		found := false
		for _, p := range problems {
			if strings.Contains(p, c.want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: esperaba un problema que dijera %q y salió %v", c.name, c.want, problems)
		}
	}
}

// TestIsoDuration comprueba el formato de duración contra casos redondos.
func TestIsoDuration(t *testing.T) {
	cases := []struct {
		ms   int64
		want string
	}{
		{0, "PT0H0M0S"},
		{30 * 60 * 1000, "PT0H30M0S"},
		{90 * 60 * 1000, "PT1H30M0S"},
		{25 * 3600 * 1000, "PT25H0M0S"},
	}
	for _, c := range cases {
		if got := isoDuration(c.ms); got != c.want {
			t.Fatalf("isoDuration(%d) = %q, esperaba %q", c.ms, got, c.want)
		}
	}
}
