package resolver

import (
	"strings"
	"testing"
	"time"

	"antena787/internal/model"
)

func (f *fx) episodeIndex() map[int64]model.Episode {
	out := map[int64]model.Episode{}
	for _, list := range f.eps {
		for _, e := range list {
			out[e.ID] = e
		}
	}
	return out
}

// F1-27, F1-47: la guía sale del plan y apunta al canal real, nunca a una
// plantilla de ejemplo.
func TestXMLTVDeLaParrillaReal(t *testing.T) {
	f := catvWeek()
	from := f.local(t, "2026-09-07 06:00")
	out := Resolve(f.in(from, 12*time.Hour))

	data, err := XMLTV(f.ch, out.Items, f.titles, f.episodeIndex())
	if err != nil {
		t.Fatalf("no se pudo escribir la guía: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "<display-name>Caribbean Advantage TV</display-name>") {
		t.Fatalf("la guía tiene que llevar el nombre del canal:\n%s", s[:upTo(len(s), 600)])
	}
	if id := ChannelID(f.ch); !strings.Contains(s, `<channel id="`+id+`">`) || !strings.Contains(s, `channel="`+id+`"`) {
		t.Fatalf("el id del canal sale del identificativo de la estación (%s)", id)
	}
	if !strings.Contains(s, `start="20260907060000 -0400"`) {
		t.Fatalf("los instantes van en formato XMLTV con la zona del canal:\n%s", s[:upTo(len(s), 1200)])
	}
	if !strings.Contains(s, "<title lang=\"es\">Get Smart</title>") {
		t.Fatalf("faltó el título de las 6:00")
	}
	if !strings.Contains(s, `<episode-num system="onscreen">S01E01</episode-num>`) {
		t.Fatalf("faltó la numeración de episodio")
	}
	if !strings.Contains(s, "<sub-title") {
		t.Fatalf("faltó el nombre del episodio")
	}
	if strings.Contains(s, "«relleno»") || strings.Contains(s, "sports1.channel") {
		t.Fatalf("la guía no anuncia el relleno ni datos de plantilla")
	}
	if problems := ValidateXMLTV(data); len(problems) != 0 {
		t.Fatalf("la guía de una jornada normal tiene que pasar el validador: %v", problems)
	}
}

// F1-28 y §9 paso 3: el validador caza lo que el tv_validate_file de
// referencia daba por bueno.
func TestValidadorDeLaGuia(t *testing.T) {
	cases := []struct {
		name string
		xml  string
		want string
	}{
		{
			name: "solape",
			xml: `<tv><channel id="catv.antena787"><display-name>CAtv</display-name></channel>` +
				`<programme start="20260907140000 -0400" stop="20260907143000 -0400" channel="catv.antena787"><title>Kojak</title></programme>` +
				`<programme start="20260907142000 -0400" stop="20260907145000 -0400" channel="catv.antena787"><title>Tarzan</title></programme></tv>`,
			want: "se pisan",
		},
		{
			name: "termina antes de empezar",
			xml: `<tv><channel id="catv.antena787"><display-name>CAtv</display-name></channel>` +
				`<programme start="20260907140000 -0400" stop="20260107140000 -0400" channel="catv.antena787"><title>Hellsing</title></programme></tv>`,
			want: "termina",
		},
		{
			name: "canal que no existe",
			xml: `<tv><channel id="catv.antena787"><display-name>CAtv</display-name></channel>` +
				`<programme start="20260907140000 -0400" stop="20260907143000 -0400" channel="sports1.channel"><title>Kojak</title></programme></tv>`,
			want: "que la guía no declara",
		},
		{
			name: "hueco largo",
			xml: `<tv><channel id="catv.antena787"><display-name>CAtv</display-name></channel>` +
				`<programme start="20260907140000 -0400" stop="20260907143000 -0400" channel="catv.antena787"><title>Kojak</title></programme>` +
				`<programme start="20260907190000 -0400" stop="20260907193000 -0400" channel="catv.antena787"><title>Zorro 57</title></programme></tv>`,
			want: "sin describir",
		},
		{
			name: "no es XML",
			xml:  `<tv><programme`,
			want: "no es XML válido",
		},
	}
	for _, c := range cases {
		problems := ValidateXMLTV([]byte(c.xml))
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

// §23: exactitud de la guía contra el as-run, con tolerancia de ±30 s.
func TestGuideAccuracy(t *testing.T) {
	base := time.Date(2026, 9, 7, 14, 0, 0, 0, time.UTC)
	ep := func(n int64) *int64 { return &n }
	guide := []model.PlanItem{
		{EpisodeID: ep(1), PlannedAt: base},
		{EpisodeID: ep(2), PlannedAt: base.Add(30 * time.Minute)},
		{EpisodeID: ep(3), PlannedAt: base.Add(time.Hour)},
	}
	shift := func(d time.Duration) *time.Time { v := base.Add(d); return &v }
	actual := []model.PlanItem{
		{EpisodeID: ep(1), PlannedAt: base, ActualAt: shift(10 * time.Second)},
		{EpisodeID: ep(2), PlannedAt: base.Add(30 * time.Minute), ActualAt: shift(30*time.Minute + 20*time.Second)},
		{EpisodeID: ep(3), PlannedAt: base.Add(time.Hour), ActualAt: shift(time.Hour + 45*time.Second)},
	}
	if got := GuideAccuracy(guide, actual, 30*time.Second); got < 0.66 || got > 0.67 {
		t.Fatalf("dos de tres dentro de ±30 s: esperaba 0.66 y salió %v", got)
	}
	if got := GuideAccuracy(guide, actual, time.Minute); got != 1 {
		t.Fatalf("con un minuto de tolerancia entran los tres: %v", got)
	}
	if got := GuideAccuracy(guide, nil, 30*time.Second); got != 0 {
		t.Fatalf("sin as-run la exactitud es cero: %v", got)
	}
	if got := GuideAccuracy(nil, actual, 30*time.Second); got != 1 {
		t.Fatalf("una guía vacía no puede fallar: %v", got)
	}
}

func upTo(a, b int) int {
	if a < b {
		return a
	}
	return b
}
