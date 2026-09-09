package resolver

import (
	"fmt"
	"strings"
	"testing"
	"time"

	// La zona horaria va embebida: las pruebas del cambio de horario tienen
	// que correr igual en una máquina sin base de zonas (Windows).
	_ "time/tzdata"

	"antena787/internal/model"
)

// Las pruebas salen de la hoja real de CAtv (docs/catv-sheet-2026-09-04.md),
// semana del 6 al 12 de septiembre de 2026, con los ids de regla de la hoja.
// Las reglas de madrugada están escritas ya en día de emisión: el importador
// corre un día atrás las fechas y el patrón de las filas entre 12:00 y 5:59
// AM (auditoría B1), así que la fila "_MMJVS_ 12:00 AM 7/2–9/8" de la hoja
// entra aquí como "LMMJV__ 12:00 AM 7/1–9/7".

const (
	deckManual = int64(1)
	deckComm   = int64(2)
	deckProg   = int64(3)
	deckFill   = int64(4)
)

type fx struct {
	ch      model.Channel
	titles  map[int64]model.Title
	eps     map[int64][]model.Episode
	assets  map[int64]model.MediaAsset
	fillers []model.FillerAsset
	rules   []model.ScheduleRule
	loc     *time.Location
}

func newFx(tz string) *fx {
	ch := model.Channel{
		ID: 1, Name: "Caribbean Advantage TV", Kind: model.ChannelTV,
		TimeZone: tz, BroadcastDayAt: 6 * 60, MaxLoadPerHour: 12,
		CallSign: "CAtv", LicenseCity: "Aguadilla, Puerto Rico",
	}
	return &fx{
		ch:     ch,
		titles: map[int64]model.Title{},
		eps:    map[int64][]model.Episode{},
		assets: map[int64]model.MediaAsset{},
		loc:    ch.Location(),
	}
}

func newCAtv() *fx { return newFx("America/Puerto_Rico") }

func (f *fx) asset(id int64, d time.Duration) model.MediaAsset {
	return model.MediaAsset{
		ID: id, Path: fmt.Sprintf("/media/%d.mxf", id), DurationMs: d.Milliseconds(),
		State: model.AssetReady, NormalizeState: "listo",
		NormalizedPath: fmt.Sprintf("/media/casa/%d.mxf", id),
	}
}

// serie carga un título con n episodios de la misma duración, todos listos.
func (f *fx) serie(id int64, name string, n int, d time.Duration) {
	f.titles[id] = model.Title{ID: id, Name: name, Kind: model.TitleSeries, Genre: "Animación", Synopsis: "Serie de " + name}
	for k := 1; k <= n; k++ {
		epID := id*100 + int64(k)
		asID := 900000 + id*100 + int64(k)
		f.assets[asID] = f.asset(asID, d)
		a := asID
		f.eps[id] = append(f.eps[id], model.Episode{
			ID: epID, TitleID: id, Season: 1, Number: k,
			Name: fmt.Sprintf("Episodio %d", k), MediaAssetID: &a,
		})
	}
}

// pelicula carga un título de un solo archivo.
func (f *fx) pelicula(id int64, name string, d time.Duration) {
	asID := 800000 + id
	f.assets[asID] = f.asset(asID, d)
	a := asID
	f.titles[id] = model.Title{ID: id, Name: name, Kind: model.TitleMovie, MediaAssetID: &a, Genre: "Cine"}
}

// sinMaterial carga un título cuyo único archivo está en cuarentena.
func (f *fx) sinMaterial(id int64, name string, d time.Duration) {
	asID := 700000 + id
	a := f.asset(asID, d)
	a.State = model.AssetQuarantine
	a.PlainReason = "el audio viene mudo"
	f.assets[asID] = a
	ref := asID
	f.titles[id] = model.Title{ID: id, Name: name, Kind: model.TitleMovie, MediaAssetID: &ref}
}

func (f *fx) filler(id int64, d time.Duration) {
	asID := 600000 + id
	f.assets[asID] = f.asset(asID, d)
	f.fillers = append(f.fillers, model.FillerAsset{
		ID: id, MediaAssetID: asID, Kind: "promo", DurationMs: d.Milliseconds(),
	})
}

func clock(s string) model.Minutes {
	m, err := model.ParseClock(s)
	if err != nil {
		panic(err)
	}
	return m
}

// rule arma una regla normal de la hoja: id, título, patrón, hora, fechas y
// episodios por corrida. Sin duración de slot: manda hasta la regla siguiente.
func (f *fx) rule(id, title int64, pattern, at, from, to string, eps int) {
	t := title
	r := model.ScheduleRule{
		ID: id, ChannelID: f.ch.ID, Kind: model.RuleNormal, TitleID: &t,
		Days: model.DayPattern(pattern), At: clock(at),
		From: model.Day(from), To: model.Day(to),
		EpisodesPerRun: eps, Active: true,
	}
	f.rules = append(f.rules, r)
}

// tweak retoca una regla ya cargada (relevo, repetición, contador).
func (f *fx) tweak(id int64, fn func(r *model.ScheduleRule)) {
	for i := range f.rules {
		if f.rules[i].ID == id {
			fn(&f.rules[i])
			return
		}
	}
	panic(fmt.Sprintf("no existe la regla %d", id))
}

func (f *fx) live(id, source int64, pattern, at, from, to string, slot time.Duration) {
	s := source
	r := model.ScheduleRule{
		ID: id, ChannelID: f.ch.ID, Kind: model.RuleLive, LiveSourceID: &s,
		Days: model.DayPattern(pattern), At: clock(at), SlotMs: slot.Milliseconds(),
		From: model.Day(from), To: model.Day(to), EpisodesPerRun: 1, Active: true,
	}
	f.rules = append(f.rules, r)
}

func (f *fx) timeShift(id int64, pattern, at, srcFrom, srcTo, from, to string, slot time.Duration) {
	a, b := clock(srcFrom), clock(srcTo)
	r := model.ScheduleRule{
		ID: id, ChannelID: f.ch.ID, Kind: model.RuleTimeShift,
		Days: model.DayPattern(pattern), At: clock(at), SlotMs: slot.Milliseconds(),
		From: model.Day(from), To: model.Day(to), EpisodesPerRun: 1, Active: true,
		SourceWindowStart: &a, SourceWindowEnd: &b,
	}
	f.rules = append(f.rules, r)
}

func (f *fx) in(now time.Time, horizon time.Duration) Input {
	return Input{
		Channel: f.ch, Rules: f.rules, Titles: f.titles, Episodes: f.eps,
		Assets: f.assets, Fillers: f.fillers,
		Decks: map[model.DeckKind]int64{
			model.DeckManual: deckManual, model.DeckCommercial: deckComm,
			model.DeckProgram: deckProg, model.DeckFiller: deckFill,
		},
		Now: now, Horizon: horizon,
	}
}

// local lee "2026-09-06 06:00" en la zona del canal.
func (f *fx) local(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.ParseInLocation("2006-01-02 15:04", s, f.loc)
	if err != nil {
		t.Fatalf("hora de prueba mal escrita %q: %v", s, err)
	}
	return v
}

// ── ayudas de comprobación ────────────────────────────────────────────

func (f *fx) titleOf(p model.PlanItem) string {
	if p.EpisodeID != nil {
		for id, list := range f.eps {
			for _, e := range list {
				if e.ID == *p.EpisodeID {
					return f.titles[id].Name
				}
			}
		}
	}
	if p.MediaAssetID != nil {
		for _, t := range f.titles {
			if t.MediaAssetID != nil && *t.MediaAssetID == *p.MediaAssetID {
				return t.Name
			}
		}
	}
	switch p.Origin {
	case "live_source":
		return "«en vivo»"
	case "relleno":
		return "«relleno»"
	case "cartel":
		return "«cartel»"
	}
	return "«sin título»"
}

func (f *fx) at(t *testing.T, out Output, when string) model.PlanItem {
	t.Helper()
	want := f.local(t, when)
	for _, p := range out.Items {
		if p.PlannedAt.Equal(want) {
			return p
		}
	}
	t.Fatalf("no hay ningún ítem a las %s\n%s", when, f.dump(out))
	return model.PlanItem{}
}

func (f *fx) none(t *testing.T, out Output, when string) {
	t.Helper()
	want := f.local(t, when)
	for _, p := range out.Items {
		if p.PlannedAt.Equal(want) && p.Origin != "relleno" && p.Origin != "cartel" {
			t.Fatalf("no debería haber programa a las %s y hay %q", when, f.titleOf(p))
		}
	}
}

func (f *fx) dump(out Output) string {
	s := "plan:\n"
	for _, p := range out.Items {
		s += fmt.Sprintf("  %s  %-24s %6.1f min  día %s  regla %v\n",
			p.PlannedAt.In(f.loc).Format("2006-01-02 15:04"), f.titleOf(p),
			float64(p.PlannedMs)/60000, p.BroadcastDay, ruleOf(p))
	}
	s += "avisos:\n"
	for _, w := range out.Warnings {
		s += fmt.Sprintf("  [%s] %s\n", w.Kind, w.Text)
	}
	return s
}

func ruleOf(p model.PlanItem) string {
	if p.RuleID == nil {
		return "-"
	}
	return fmt.Sprint(*p.RuleID)
}

func warned(out Output, kind WarningKind, substr string) bool {
	for _, w := range out.Warnings {
		if w.Kind == kind && (substr == "" || strings.Contains(w.Text, substr)) {
			return true
		}
	}
	return false
}

func countOrigin(out Output, origin string) int {
	n := 0
	for _, p := range out.Items {
		if p.Origin == origin {
			n++
		}
	}
	return n
}

// covered comprueba que no queda un segundo de aire sin ítem en la ventana.
func covered(t *testing.T, f *fx, out Output, from, to time.Time) {
	t.Helper()
	cursor := from
	for _, p := range out.Items {
		if p.End().Before(cursor) || p.End().Equal(cursor) {
			continue
		}
		if p.PlannedAt.After(cursor) {
			t.Fatalf("queda aire sin cubrir de %s a %s\n%s",
				cursor.In(f.loc).Format("2006-01-02 15:04:05"),
				p.PlannedAt.In(f.loc).Format("2006-01-02 15:04:05"), f.dump(out))
		}
		cursor = p.End()
		if !cursor.Before(to) {
			return
		}
	}
	if cursor.Before(to) {
		t.Fatalf("queda aire sin cubrir de %s hasta el fin de la ventana",
			cursor.In(f.loc).Format("2006-01-02 15:04:05"))
	}
}
