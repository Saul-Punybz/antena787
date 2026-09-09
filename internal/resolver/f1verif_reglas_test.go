package resolver

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"antena787/internal/model"
)

// Verificación de los criterios F1-13, F1-14, F1-17, F1-18, F1-19, F1-22,
// F1-24, F1-25, F1-34, F1-35 y F1-39 de docs/ACEPTACION.md. Se apoyan en las
// mismas ayudas que resolver_test.go (fixtures_test.go).

// serieConTemporadas carga un título con varias temporadas de n episodios,
// todos con material listo. El id del episodio es id*1000 + temporada*100 + k.
func (f *fx) serieConTemporadas(id int64, name string, temporadas, n int, d time.Duration) {
	f.titles[id] = model.Title{ID: id, Name: name, Kind: model.TitleSeries}
	for s := 1; s <= temporadas; s++ {
		for k := 1; k <= n; k++ {
			epID := id*1000 + int64(s)*100 + int64(k)
			asID := 500000 + epID
			f.assets[asID] = f.asset(asID, d)
			a := asID
			f.eps[id] = append(f.eps[id], model.Episode{
				ID: epID, TitleID: id, Season: s, Number: k,
				Name: fmt.Sprintf("T%dE%d", s, k), MediaAssetID: &a,
			})
		}
	}
}

// episodio busca un episodio por temporada y número.
func (f *fx) episodio(t *testing.T, titleID int64, temporada, numero int) model.Episode {
	t.Helper()
	for _, e := range f.eps[titleID] {
		if e.Season == temporada && e.Number == numero {
			return e
		}
	}
	t.Fatalf("no existe el episodio T%dE%d del título %d", temporada, numero, titleID)
	return model.Episode{}
}

// F1-13 — Kojak con episodios_por_corrida = 1 y ultimo_episodio_emitido =
// temporada 2, episodio 5 · Cuando la regla vuelve a correr al día siguiente ·
// Entonces programa el episodio 6 de la temporada 2, no repite el 5.
func TestF1Verif13ElContadorAvanzaYRecuerda(t *testing.T) {
	f := newCAtv()
	f.serieConTemporadas(322, "Kojak", 3, 8, 48*time.Minute)
	f.filler(1, time.Minute)
	f.rule(322, 322, "LMMJV__", "8:00", "2026-07-27", "2027-01-04", 1)

	t2e5 := f.episodio(t, 322, 2, 5)
	t2e6 := f.episodio(t, 322, 2, 6)
	f.tweak(322, func(r *model.ScheduleRule) { id := t2e5.ID; r.LastEpisodeAired = &id })

	out := Resolve(f.in(f.local(t, "2026-09-08 06:00"), 12*time.Hour))
	p := f.at(t, out, "2026-09-08 08:00")
	if p.EpisodeID == nil {
		t.Fatalf("el bloque de las 8:00 no trae episodio\n%s", f.dump(out))
	}
	if *p.EpisodeID == t2e5.ID {
		t.Fatalf("repitió el episodio T2E5 en vez de avanzar\n%s", f.dump(out))
	}
	if *p.EpisodeID != t2e6.ID {
		t.Fatalf("esperaba T2E6 (%d) y salió %d\n%s", t2e6.ID, *p.EpisodeID, f.dump(out))
	}
	if out.EpisodeAdvance[322] != t2e6.ID {
		t.Fatalf("el contador debería quedar en T2E6 y quedó en %d", out.EpisodeAdvance[322])
	}

	// Y al día siguiente, con el contador ya en T2E6, toca T2E7.
	f.tweak(322, func(r *model.ScheduleRule) { id := t2e6.ID; r.LastEpisodeAired = &id })
	out2 := Resolve(f.in(f.local(t, "2026-09-09 06:00"), 12*time.Hour))
	p2 := f.at(t, out2, "2026-09-09 08:00")
	if p2.EpisodeID == nil || *p2.EpisodeID != f.episodio(t, 322, 2, 7).ID {
		t.Fatalf("esperaba T2E7 al día siguiente\n%s", f.dump(out2))
	}
}

// F1-14 — Gaming Longplays con episodios_por_corrida = 10 y espacio
// suficiente antes del siguiente inicio duro · Cuando corre una vez ·
// Entonces genera 10 plan_item consecutivos, uno por episodio, sin huecos.
func TestF1Verif14DiezEpisodiosCuandoCaben(t *testing.T) {
	f := newCAtv()
	// Diez de 24 min = 4 h, y de 18:00 a 23:00 hay cinco: caben los diez.
	f.serie(328, "Gaming Longplays", 12, 24*time.Minute)
	f.serie(315, "You're Under Arrest", 12, 24*time.Minute)
	f.filler(1, time.Minute)
	f.rule(328, 328, "LMMJV__", "18:00", "2026-08-31", "2026-09-28", 10)
	f.rule(315, 315, "LMMJV__", "23:00", "2026-07-07", "2026-09-16", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 24*time.Hour))

	var longplays []model.PlanItem
	for _, p := range out.Items {
		if p.RuleID != nil && *p.RuleID == 328 {
			longplays = append(longplays, p)
		}
	}
	if len(longplays) != 10 {
		t.Fatalf("esperaba 10 plan_item de Gaming Longplays y hay %d\n%s", len(longplays), f.dump(out))
	}
	// Consecutivos, sin huecos, y uno por episodio distinto.
	vistos := map[int64]bool{}
	cursor := f.local(t, "2026-09-07 18:00")
	for i, p := range longplays {
		if !p.PlannedAt.Equal(cursor) {
			t.Fatalf("el episodio %d empieza a las %s y debería empezar a las %s",
				i+1, p.PlannedAt.In(f.loc).Format("15:04"), cursor.In(f.loc).Format("15:04"))
		}
		if p.EpisodeID == nil || vistos[*p.EpisodeID] {
			t.Fatalf("el episodio %d se repite o no trae id: %+v", i+1, p.EpisodeID)
		}
		vistos[*p.EpisodeID] = true
		cursor = p.End()
	}
	if !cursor.Equal(f.local(t, "2026-09-07 22:00")) {
		t.Fatalf("los diez terminan a las 22:00 y terminan a las %s", cursor.In(f.loc).Format("15:04"))
	}
	if warned(out, WarnOverbook, "") {
		t.Fatalf("cabían los diez: no debería avisar de sobrecupo\n%s", f.dump(out))
	}
}

// F1-17 — Dado el resolver corriendo a las 10:00 AM del lunes · Entonces
// existen plan_item cubriendo como mínimo hasta las 10:00 AM del miércoles
// (48 h por adelantado), y ninguno más allá de esa ventana.
func TestF1Verif17VentanaDeCuarentaYOchoHoras(t *testing.T) {
	f := catvWeek()
	desde := f.local(t, "2026-09-07 10:00") // lunes 10:00 AM
	hasta := desde.Add(48 * time.Hour)      // miércoles 10:00 AM
	out := Resolve(f.in(desde, DefaultHorizon))

	// Cubre toda la ventana, sin un segundo de aire vacío.
	covered(t, f, out, desde, hasta)

	// Y no se sale de ella: nada arranca en el miércoles a las 10:00 o después.
	for _, p := range out.Items {
		if !p.PlannedAt.Before(hasta) {
			t.Fatalf("hay un ítem que arranca a las %s, fuera de la ventana de 48 h\n%s",
				p.PlannedAt.In(f.loc).Format("2006-01-02 15:04"), f.dump(out))
		}
		if p.PlannedAt.Before(desde) {
			t.Fatalf("hay un ítem antes del arranque del resolver: %s",
				p.PlannedAt.In(f.loc).Format("2006-01-02 15:04"))
		}
	}
}

// F1-18 — Dado una regla con fecha_fin = hoy · Cuando el resolver corre a las
// 00:05 de mañana · Entonces no genera plan_item para esa regla, y no la
// trata como error silencioso: queda simplemente fuera del plan.
func TestF1Verif18ReglaVencidaQuedaFueraSinRuido(t *testing.T) {
	f := newCAtv()
	f.serie(700, "Se acaba hoy", 12, 24*time.Minute)
	f.serie(701, "Sigue vigente", 12, 24*time.Minute)
	f.filler(1, time.Minute)
	// El lunes 7 es el último día de emisión de la 700.
	f.rule(700, 700, "LMMJV__", "20:00", "2026-08-01", "2026-09-07", 1)
	f.rule(701, 701, "LMMJV__", "21:00", "2026-08-01", "2026-12-31", 1)

	// 00:05 del martes de calendario: el bloque de las 8 PM del lunes ya pasó.
	out := Resolve(f.in(f.local(t, "2026-09-08 00:05"), 48*time.Hour))
	for _, p := range out.Items {
		if p.RuleID != nil && *p.RuleID == 700 {
			t.Fatalf("la regla vencida puso algo a las %s\n%s",
				p.PlannedAt.In(f.loc).Format("2006-01-02 15:04"), f.dump(out))
		}
	}
	// Nada de avisos de regla inválida ni de material: no es un error.
	for _, w := range out.Warnings {
		if w.RuleID != nil && *w.RuleID == 700 && (w.Kind == WarnBadRule || w.Kind == WarnNoMaterial) {
			t.Fatalf("una regla vencida no es un error: salió el aviso %q %q", w.Kind, w.Text)
		}
	}
	// Y la que sigue vigente sí pone lo suyo.
	if f.at(t, out, "2026-09-08 21:00").RuleID == nil {
		t.Fatalf("la regla vigente tenía que poner su bloque\n%s", f.dump(out))
	}
}

// F1-19 — Dos reglas distintas sin relevo en la misma franja del mismo día ·
// Cuando el resolver corre · Entonces reporta un conflicto de solape y no
// genera plan_item para ambas a la vez.
func TestF1Verif19DosReglasEnLaMismaFranjaEsConflicto(t *testing.T) {
	f := newCAtv()
	f.serie(410, "Una", 12, 24*time.Minute)
	f.serie(411, "Otra", 12, 24*time.Minute)
	f.filler(1, time.Minute)
	f.rule(410, 410, "LMMJV__", "8:00", "2026-09-01", "2026-09-30", 1)
	f.rule(411, 411, "LMMJV__", "8:00", "2026-09-01", "2026-09-30", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 12*time.Hour))
	if !warned(out, WarnBadRule, "ninguna releva a la otra") {
		t.Fatalf("faltó el conflicto de solape\n%s", f.dump(out))
	}
	n := 0
	for _, p := range out.Items {
		if p.RuleID != nil && (*p.RuleID == 410 || *p.RuleID == 411) {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("solo una de las dos reglas puede poner su bloque y hay %d\n%s", n, f.dump(out))
	}
}

// F1-21 — Dado un hueco de 7:00 min y relleno disponible de 2:00, 3:00 y
// 4:00 · Cuando el resolver corre · Entonces el plan tiene 3:00 + 4:00 min,
// en ese orden, y el hueco residual es 0.
func TestF1Verif21ElRellenoCuadraExactoEnElPlan(t *testing.T) {
	f := newCAtv()
	f.serie(415, "Programa de 23 min", 6, 23*time.Minute)
	f.serie(416, "El de las 8:30", 6, 20*time.Minute)
	f.filler(1, 2*time.Minute)
	f.filler(2, 3*time.Minute)
	f.filler(3, 4*time.Minute)
	f.rule(415, 415, "LMMJV__", "8:00", "2026-09-01", "2026-09-30", 1)
	f.rule(416, 416, "LMMJV__", "8:30", "2026-09-01", "2026-09-30", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 08:00"), 2*time.Hour))

	var relleno []model.PlanItem
	for _, p := range out.Items {
		if p.Origin == "relleno" && !p.PlannedAt.Before(f.local(t, "2026-09-07 08:23")) &&
			p.PlannedAt.Before(f.local(t, "2026-09-07 08:30")) {
			relleno = append(relleno, p)
		}
	}
	if len(relleno) != 2 {
		t.Fatalf("un hueco de 7:00 se cubre con dos clips y hay %d\n%s", len(relleno), f.dump(out))
	}
	if relleno[0].PlannedMs != (3 * time.Minute).Milliseconds() {
		t.Fatalf("primero el de 3:00 y salió uno de %d ms", relleno[0].PlannedMs)
	}
	if relleno[1].PlannedMs != (4 * time.Minute).Milliseconds() {
		t.Fatalf("después el de 4:00 y salió uno de %d ms", relleno[1].PlannedMs)
	}
	if !relleno[1].End().Equal(f.local(t, "2026-09-07 08:30")) {
		t.Fatalf("el hueco residual tiene que ser 0 y el relleno termina a las %s",
			relleno[1].End().In(f.loc).Format("15:04:05"))
	}
	if countOrigin(out, "cartel") != 0 {
		t.Fatalf("con relleno exacto no sale el cartel\n%s", f.dump(out))
	}
}

// F1-23 — Dado un plan resuelto sin vencimientos próximos · Cuando una regla
// llega a 30, luego a 14, luego a 7 días de su fecha_fin · Entonces se genera
// un aviso en cada uno de esos tres umbrales, sin duplicarse si el resolver
// corre varias veces dentro del mismo día.
func TestF1Verif23LosTresUmbralesDeVencimiento(t *testing.T) {
	fin := "2026-10-07"
	// Cada parada: el día en que corre el resolver y el umbral que toca.
	paradas := []struct {
		dia    string
		espera string
	}{
		{"2026-08-20", ""},   // 48 días: todavía no
		{"2026-09-07", "30"}, // 30 días
		{"2026-09-23", "14"}, // 14 días
		{"2026-09-30", "7"},  // 7 días
	}

	ultimo := "" // lo que quedaría guardado en schedule_rule.ultimo_aviso_enviado
	for _, p := range paradas {
		f := newCAtv()
		f.serie(322, "Kojak", 20, 24*time.Minute)
		f.filler(1, time.Minute)
		f.rule(322, 322, "LMMJV__", "8:00", "2026-07-27", fin, 1)
		guardado := ultimo
		f.tweak(322, func(r *model.ScheduleRule) { r.LastNoticeSent = guardado })

		out := Resolve(f.in(f.local(t, p.dia+" 08:00"), time.Hour))
		got := ""
		n := 0
		for _, w := range out.Warnings {
			if w.Kind == WarnExpiry {
				got = w.Notice
				n++
			}
		}
		if got != p.espera {
			t.Fatalf("el %s esperaba el aviso %q y salió %q", p.dia, p.espera, got)
		}
		if n > 1 {
			t.Fatalf("el %s salieron %d avisos de vencimiento para la misma regla", p.dia, n)
		}
		if got != "" {
			ultimo = got
			// Correrlo otra vez el mismo día, con el aviso ya guardado, no repite.
			f2 := newCAtv()
			f2.serie(322, "Kojak", 20, 24*time.Minute)
			f2.filler(1, time.Minute)
			f2.rule(322, 322, "LMMJV__", "8:00", "2026-07-27", fin, 1)
			f2.tweak(322, func(r *model.ScheduleRule) { r.LastNoticeSent = got })
			out2 := Resolve(f2.in(f2.local(t, p.dia+" 09:00"), time.Hour))
			if warned(out2, WarnExpiry, "") {
				t.Fatalf("el %s el aviso %q se repitió en la segunda corrida del día", p.dia, got)
			}
		}
	}
	if ultimo != "7" {
		t.Fatalf("los tres umbrales tenían que salir en orden y el último fue %q", ultimo)
	}
}

// F1-22 — Dado un hueco de 5:57 y relleno solo de 2:00 y 3:00 · Entonces arma
// 3:00 + 3:00 = 6:00 (exceso de 3 s, dentro del límite de 5 s del §14.1) y
// recorta el último clip del hueco. El hueco residual queda en 0.
func TestF1Verif22ExcesoDeTresSegundosSeRecortaEnElUltimoClip(t *testing.T) {
	f := newCAtv()
	// 24:03 de programa deja exactamente 5:57 antes de la regla de las 8:30.
	f.serie(420, "Programa de 24:03", 6, 24*time.Minute+3*time.Second)
	f.serie(421, "El de las 8:30", 6, 20*time.Minute)
	f.filler(1, 3*time.Minute)
	f.filler(2, 2*time.Minute)
	f.rule(420, 420, "LMMJV__", "8:00", "2026-09-01", "2026-09-30", 1)
	f.rule(421, 421, "LMMJV__", "8:30", "2026-09-01", "2026-09-30", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 08:00"), 2*time.Hour))

	var relleno []model.PlanItem
	for _, p := range out.Items {
		if p.Origin == "relleno" && p.PlannedAt.Before(f.local(t, "2026-09-07 08:30")) &&
			!p.PlannedAt.Before(f.local(t, "2026-09-07 08:00")) {
			relleno = append(relleno, p)
		}
	}
	if len(relleno) != 2 {
		t.Fatalf("el hueco de 5:57 se cubre con dos clips de relleno y hay %d\n%s", len(relleno), f.dump(out))
	}
	if relleno[0].PlannedMs != (3 * time.Minute).Milliseconds() {
		t.Fatalf("el primer clip va entero (3:00) y dura %d ms", relleno[0].PlannedMs)
	}
	if relleno[1].PlannedMs != (2*time.Minute + 57*time.Second).Milliseconds() {
		t.Fatalf("el último clip se recorta a 2:57 y dura %d ms", relleno[1].PlannedMs)
	}
	// El hueco residual queda en cero: el relleno pega justo con las 8:30.
	if !relleno[1].End().Equal(f.local(t, "2026-09-07 08:30")) {
		t.Fatalf("el relleno tiene que pegar con las 8:30 y termina a las %s",
			relleno[1].End().In(f.loc).Format("15:04:05"))
	}
	if countOrigin(out, "cartel") != 0 {
		t.Fatalf("no debería hacer falta el cartel\n%s", f.dump(out))
	}

	// Y si el exceso mínimo posible pasara de 5 s, la combinación se descarta:
	// con clips solo de 4:00, 4+4 = 8:00 se pasa 2:03 y no vale.
	plan := packFillers((5*time.Minute + 57*time.Second).Milliseconds(),
		[]model.FillerAsset{{ID: 9, DurationMs: (4 * time.Minute).Milliseconds()}})
	if plan.trim != 0 {
		t.Fatalf("un exceso de 2:03 no se puede recortar: %+v", plan)
	}
	if plan.short != (1*time.Minute + 57*time.Second).Milliseconds() {
		t.Fatalf("lo que no se cubre va al cartel (1:57) y quedó en %d ms", plan.short)
	}
}

// F1-24 — Veinte reglas cubriendo cada franja de la semana, con relleno
// suficiente · Cuando el resolver corre · Entonces produce las 336
// medias-horas de la semana (7 × 24 × 2) sin ninguna franja sin plan_item ni
// relleno asignado.
func TestF1Verif24LasTrescientasTreintaYSeisMediasHoras(t *testing.T) {
	f := catvWeek()
	desde := f.local(t, "2026-09-06 06:00") // domingo, inicio del día de emisión
	out := Resolve(f.in(desde, 7*24*time.Hour))

	// Ninguna media hora de la semana puede quedar sin nada puesto.
	for i := 0; i < 336; i++ {
		instante := desde.Add(time.Duration(i) * 30 * time.Minute)
		cubierta := false
		for _, p := range out.Items {
			if !instante.Before(p.PlannedAt) && instante.Before(p.End()) {
				cubierta = true
				break
			}
		}
		if !cubierta {
			t.Fatalf("la media hora %d (%s) quedó sin plan_item ni relleno",
				i, instante.In(f.loc).Format("2006-01-02 15:04"))
		}
	}
	// Y sin un solo segundo de aire vacío entre bloque y bloque.
	covered(t, f, out, desde, desde.Add(7*24*time.Hour))
}

// F1-25 y F1-34 — Con hora_inicio_dia_emision = 6:00 AM, un plan_item que
// sale a las 12:30 AM del martes de calendario pertenece al día de emisión
// del lunes; y una regla con patrón 'L' a las 2:00 AM se materializa el
// martes de calendario.
func TestF1Verif25y34LaMadrugadaEsDelDiaAnterior(t *testing.T) {
	f := newCAtv()
	f.serie(430, "El de las 12:30 AM", 12, 24*time.Minute)
	f.serie(431, "El de las 2:00 AM", 12, 24*time.Minute)
	f.filler(1, time.Minute)
	// Patrón solo lunes.
	f.rule(430, 430, "L______", "0:30", "2026-09-01", "2026-09-30", 1)
	f.rule(431, 431, "L______", "2:00", "2026-09-01", "2026-09-30", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 30*time.Hour))

	// F1-25: las 12:30 AM del martes de calendario son del lunes de emisión.
	p := f.at(t, out, "2026-09-08 00:30")
	if p.RuleID == nil || *p.RuleID != 430 {
		t.Fatalf("a las 12:30 AM del martes esperaba la regla 430\n%s", f.dump(out))
	}
	if p.BroadcastDay != "2026-09-07" {
		t.Fatalf("F1-25: el día de emisión tiene que ser el lunes 2026-09-07 y es %s", p.BroadcastDay)
	}
	if p.LocalClock != "00:30" {
		t.Fatalf("la hora local debería ser 00:30 y es %q", p.LocalClock)
	}

	// F1-34: el patrón 'L' a las 2:00 AM sale el martes de calendario.
	q := f.at(t, out, "2026-09-08 02:00")
	if q.RuleID == nil || *q.RuleID != 431 {
		t.Fatalf("a las 2:00 AM del martes esperaba la regla 431\n%s", f.dump(out))
	}
	if q.BroadcastDay != "2026-09-07" {
		t.Fatalf("F1-34: esas 2:00 AM son del lunes de emisión y quedaron en %s", q.BroadcastDay)
	}
	// Y no hay nada de esas reglas el miércoles de calendario (martes de emisión).
	for _, it := range out.Items {
		if it.RuleID == nil {
			continue
		}
		if (*it.RuleID == 430 || *it.RuleID == 431) && it.BroadcastDay != "2026-09-07" {
			t.Fatalf("el patrón 'L' solo cubre el lunes de emisión y salió en %s", it.BroadcastDay)
		}
	}

	// Y la regla del canal, directamente: 12:30 AM del martes → lunes.
	if got := f.ch.BroadcastDay(f.local(t, "2026-09-08 00:30")); got != "2026-09-07" {
		t.Fatalf("Channel.BroadcastDay dio %s para las 12:30 AM del martes", got)
	}
	if got := f.ch.BroadcastDay(f.local(t, "2026-09-08 06:00")); got != "2026-09-08" {
		t.Fatalf("a las 6:00 AM ya empieza el día siguiente y dio %s", got)
	}
}

// F1-35 — Dado el modelo completo de schedule_rule · Cuando se busca un campo
// de recurrencia anual o un manejo especial del 29 de febrero · Entonces no
// existe ninguno: el patrón es semanal y las fechas son absolutas.
func TestF1Verif35NoHayRecurrenciaAnualNiCasoDeBisiesto(t *testing.T) {
	prohibidas := []string{"anual", "annual", "yearly", "aniversario", "bisiesto", "leap", "02-29", "29feb", "febrero"}

	// El tipo del dominio.
	rt := reflect.TypeOf(model.ScheduleRule{})
	for i := 0; i < rt.NumField(); i++ {
		campo := rt.Field(i)
		texto := strings.ToLower(campo.Name + " " + string(campo.Tag))
		for _, mala := range prohibidas {
			if strings.Contains(texto, mala) {
				t.Fatalf("schedule_rule trae un campo de recurrencia anual: %s (%s)", campo.Name, campo.Tag)
			}
		}
	}
	// El patrón es semanal: siete posiciones, ni una más.
	if len(model.Letters) != 7 {
		t.Fatalf("el patrón semanal tiene %d letras", len(model.Letters))
	}

	// Y el esquema tampoco.
	sql, err := os.ReadFile("../store/schema.sql")
	if err != nil {
		t.Fatalf("no se pudo leer el esquema: %v", err)
	}
	tabla := string(sql)
	if i := strings.Index(tabla, "CREATE TABLE schedule_rule"); i >= 0 {
		fin := strings.Index(tabla[i:], ");")
		tabla = strings.ToLower(tabla[i : i+fin])
	}
	for _, mala := range prohibidas {
		if strings.Contains(tabla, mala) {
			t.Fatalf("el esquema de schedule_rule menciona %q", mala)
		}
	}

	// Comprobación de conducta: el 29 de febrero de un año bisiesto no es un
	// caso borde — sale como cualquier otro día del patrón.
	f := newCAtv()
	f.serie(440, "Bisiesto", 12, 24*time.Minute)
	f.filler(1, time.Minute)
	f.rule(440, 440, "LMMJVSD", "8:00", "2028-02-01", "2028-03-31", 1)
	out := Resolve(f.in(f.local(t, "2028-02-28 06:00"), 48*time.Hour))
	p := f.at(t, out, "2028-02-29 08:00")
	if p.BroadcastDay != "2028-02-29" {
		t.Fatalf("el 29 de febrero es un día de emisión normal y quedó en %s", p.BroadcastDay)
	}
}

// F1-39 — Dado un bloque en vivo de 10:00 a 13:00 y un elemento programado
// que no terminaría antes de las 13:00 · Cuando el resolver lo materializa ·
// Entonces no lo arranca: el fin del bloque en vivo es tan duro como el de un
// slot de archivo.
func TestF1Verif39ElFinDelVivoEsUnInicioDuro(t *testing.T) {
	f := newCAtv()
	f.serie(450, "Antes del vivo", 12, 45*time.Minute)
	f.serie(451, "Después del vivo", 12, 24*time.Minute)
	f.filler(1, time.Minute)
	f.filler(2, 5*time.Minute)
	f.live(449, 1, "LMMJV__", "10:00", "2026-09-01", "2026-12-31", 3*time.Hour)
	// A las 9:40 quedan 20 min antes del vivo: un programa de 45 no arranca.
	f.rule(450, 450, "LMMJV__", "9:40", "2026-09-01", "2026-09-30", 1)
	f.rule(451, 451, "LMMJV__", "13:00", "2026-09-01", "2026-09-30", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 24*time.Hour))

	// El arranque del vivo es un inicio duro: nada empieza si no termina antes.
	p := f.at(t, out, "2026-09-07 09:40")
	if p.Origin != "relleno" && p.Origin != "cartel" {
		t.Fatalf("no cabía antes del vivo: las 9:40 son relleno y salió %q (%s)\n%s",
			p.Origin, f.titleOf(p), f.dump(out))
	}
	// El bloque en vivo sale entero, de 10:00 a 13:00.
	vivo := f.at(t, out, "2026-09-07 10:00")
	if vivo.Origin != "live_source" {
		t.Fatalf("el bloque de las 10:00 tiene que ser la fuente en vivo y es %q", vivo.Origin)
	}
	if !vivo.End().Equal(f.local(t, "2026-09-07 13:00")) {
		t.Fatalf("el vivo termina a las 13:00 y termina a las %s\n%s",
			vivo.End().In(f.loc).Format("15:04"), f.dump(out))
	}
	// Nada se mete dentro del bloque en vivo ni lo desborda.
	for _, it := range out.Items {
		if it.PlannedAt.Equal(vivo.PlannedAt) {
			continue
		}
		if it.PlannedAt.Before(vivo.End()) && it.End().After(vivo.PlannedAt) {
			t.Fatalf("hay un ítem (%s, %s) metido dentro del bloque en vivo\n%s",
				it.PlannedAt.In(f.loc).Format("15:04"), it.Origin, f.dump(out))
		}
	}
}

// F1-39 (borde) — una regla de archivo dentro de la ventana del vivo no puede
// robarle el aire ni desbordar su fin duro de las 13:00.
func TestF1Verif39ReglaDentroDeLaVentanaDelVivo(t *testing.T) {
	f := newCAtv()
	f.serie(460, "El que se pasa", 12, 45*time.Minute)
	f.filler(1, time.Minute)
	f.live(459, 1, "LMMJV__", "10:00", "2026-09-01", "2026-12-31", 3*time.Hour)
	// A las 12:30, dentro del vivo: 45 min no terminan antes de las 13:00.
	f.rule(460, 460, "LMMJV__", "12:30", "2026-09-01", "2026-09-30", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 24*time.Hour))

	vivo := f.at(t, out, "2026-09-07 10:00")
	if !vivo.End().Equal(f.local(t, "2026-09-07 13:00")) {
		t.Fatalf("el bloque en vivo de 10:00 a 13:00 quedó recortado hasta las %s\n%s",
			vivo.End().In(f.loc).Format("15:04"), f.dump(out))
	}
	for _, it := range out.Items {
		if it.RuleID != nil && *it.RuleID == 460 {
			t.Fatalf("un programa de 45 min a las 12:30 no termina antes de las 13:00: no se arranca\n%s",
				f.dump(out))
		}
	}
}
