package resolver

import (
	"reflect"
	"testing"
	"time"

	"antena787/internal/model"
)

// F1-12, F1-18, B1: fecha_fin es inclusiva hasta el cierre del día de
// emisión, y el día siguiente la regla ya no pone nada. Familia Robinson
// termina el domingo 6 de septiembre en la hoja de CAtv.
func TestFechaFinInclusiva(t *testing.T) {
	f := newCAtv()
	f.serie(273, "Familia Robinson", 12, 24*time.Minute)
	f.serie(321, "Sonic The Hedgehog", 12, 24*time.Minute)
	f.filler(1, time.Minute)
	f.rule(273, 273, "_____SD", "9:00", "2026-03-21", "2026-09-06", 1)
	f.rule(321, 321, "_____SD", "9:30", "2026-07-26", "2027-03-07", 1)

	out := Resolve(f.in(f.local(t, "2026-09-06 06:00"), 48*time.Hour))
	if got := f.titleOf(f.at(t, out, "2026-09-06 09:00")); got != "Familia Robinson" {
		t.Fatalf("el domingo 6 a las 9:00 esperaba Familia Robinson y salió %q\n%s", got, f.dump(out))
	}
	if day := f.at(t, out, "2026-09-06 09:00").BroadcastDay; day != "2026-09-06" {
		t.Fatalf("el día de emisión debería ser 2026-09-06 y es %s", day)
	}

	// El domingo siguiente ya no le toca.
	f2 := newCAtv()
	f2.serie(273, "Familia Robinson", 12, 24*time.Minute)
	f2.serie(321, "Sonic The Hedgehog", 12, 24*time.Minute)
	f2.filler(1, time.Minute)
	f2.rule(273, 273, "_____SD", "9:00", "2026-03-21", "2026-09-06", 1)
	f2.rule(321, 321, "_____SD", "9:30", "2026-07-26", "2027-03-07", 1)
	out2 := Resolve(f2.in(f2.local(t, "2026-09-13 06:00"), 24*time.Hour))
	f2.none(t, out2, "2026-09-13 09:00")
	if got := f2.titleOf(f2.at(t, out2, "2026-09-13 09:30")); got != "Sonic The Hedgehog" {
		t.Fatalf("Sonic sigue vigente el 13 y salió %q", got)
	}
}

// F1-33, F1-34, B1: la madrugada se lee por día de emisión. Magic Knight
// (313) termina el día de emisión del lunes 7 y Zoids (353) entra el martes
// 8; ambos salen a las 12:00 AM del calendario siguiente.
func TestMadrugadaPorDiaDeEmision(t *testing.T) {
	f := newCAtv()
	f.serie(313, "Magic Knight Rayearth", 26, 24*time.Minute)
	f.serie(353, "Zoids", 26, 24*time.Minute)
	f.serie(304, "Mazinger Z", 26, 24*time.Minute)
	f.filler(1, time.Minute)
	f.rule(313, 313, "LMMJV__", "0:00", "2026-07-01", "2026-09-07", 1)
	f.rule(353, 353, "LMMJV__", "0:00", "2026-09-08", "2026-12-09", 1)
	f.rule(304, 304, "LMMJV__", "0:30", "2026-06-10", "2026-10-15", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 48*time.Hour))

	lunes := f.at(t, out, "2026-09-08 00:00")
	if f.titleOf(lunes) != "Magic Knight Rayearth" {
		t.Fatalf("a las 12:00 AM del martes de calendario esperaba Magic Knight y salió %q\n%s", f.titleOf(lunes), f.dump(out))
	}
	if lunes.BroadcastDay != "2026-09-07" {
		t.Fatalf("esa emisión es del día de emisión del lunes 7 y quedó en %s", lunes.BroadcastDay)
	}
	if lunes.LocalClock != "00:00" {
		t.Fatalf("la hora local debería ser 00:00 y es %q", lunes.LocalClock)
	}
	martes := f.at(t, out, "2026-09-09 00:00")
	if f.titleOf(martes) != "Zoids" {
		t.Fatalf("el martes 8 de emisión ya es Zoids y salió %q", f.titleOf(martes))
	}
	if martes.BroadcastDay != "2026-09-08" {
		t.Fatalf("día de emisión equivocado: %s", martes.BroadcastDay)
	}
}

// F1-15: un relevo no es un conflicto. 346 (Magic Knight, 3 PM) le entrega a
// 352 (Zoids, 3 PM) en la hoja de CAtv.
func TestRelevoNoEsConflicto(t *testing.T) {
	build := func(desde string, relevo bool) (*fx, Output) {
		f := newCAtv()
		f.serie(346, "Magic Knight Rayearth", 26, 24*time.Minute)
		f.serie(352, "Zoids", 26, 24*time.Minute)
		f.filler(1, time.Minute)
		f.rule(346, 346, "LMMJV__", "15:00", "2026-07-01", "2026-09-07", 1)
		f.rule(352, 352, "LMMJV__", "15:00", desde, "2026-12-09", 1)
		if relevo {
			f.tweak(352, func(r *model.ScheduleRule) { id := int64(346); r.HandsOffTo = &id })
		}
		return f, Resolve(f.in(f.local(t, "2026-09-07 06:00"), 48*time.Hour))
	}

	// Como está en la hoja: una termina, la otra empieza al día siguiente.
	f, out := build("2026-09-08", true)
	if got := f.titleOf(f.at(t, out, "2026-09-07 15:00")); got != "Magic Knight Rayearth" {
		t.Fatalf("el lunes 7 a las 3 PM esperaba Magic Knight y salió %q", got)
	}
	if got := f.titleOf(f.at(t, out, "2026-09-08 15:00")); got != "Zoids" {
		t.Fatalf("el martes 8 a las 3 PM esperaba Zoids y salió %q", got)
	}
	if warned(out, WarnBadRule, "") {
		t.Fatalf("un relevo no es un conflicto\n%s", f.dump(out))
	}

	// Si se solapan pero hay relevo cargado, tampoco es conflicto: manda quien releva.
	f, out = build("2026-09-07", true)
	if warned(out, WarnBadRule, "") {
		t.Fatalf("con releva_a cargado no debería avisar de conflicto\n%s", f.dump(out))
	}
	if got := f.titleOf(f.at(t, out, "2026-09-07 15:00")); got != "Zoids" {
		t.Fatalf("quien releva es quien sale: esperaba Zoids y salió %q", got)
	}

	// Sin relevo, dos reglas a la misma hora sí es conflicto: gana la de menor id.
	f, out = build("2026-09-07", false)
	if !warned(out, WarnBadRule, "ninguna releva a la otra") {
		t.Fatalf("faltó el aviso de solape\n%s", f.dump(out))
	}
	if got := f.titleOf(f.at(t, out, "2026-09-07 15:00")); got != "Magic Knight Rayearth" {
		t.Fatalf("gana la regla de menor id (346): esperaba Magic Knight y salió %q", got)
	}
}

// F1-38, B4: el reloj manda. Gaming Longplays pide diez episodios de 33
// minutos en las cinco horas de 6 a 11 PM; caben nueve.
func TestSobrecupoDeDiezCabenNueve(t *testing.T) {
	f := newCAtv()
	f.serie(328, "Gaming Longplays", 12, 33*time.Minute)
	f.serie(315, "You're Under Arrest", 26, 24*time.Minute)
	f.filler(1, time.Minute)
	f.rule(328, 328, "LMMJV__", "18:00", "2026-08-31", "2026-09-28", 10)
	f.rule(315, 315, "LMMJV__", "23:00", "2026-07-07", "2026-09-16", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 24*time.Hour))

	n := 0
	for _, p := range out.Items {
		if f.titleOf(p) == "Gaming Longplays" {
			n++
		}
	}
	if n != 9 {
		t.Fatalf("esperaba 9 episodios de Gaming Longplays y hay %d\n%s", n, f.dump(out))
	}
	if !warned(out, WarnOverbook, "caben 9") {
		t.Fatalf("faltó el aviso 'de 10 episodios caben 9'\n%s", f.dump(out))
	}
	// Ningún episodio cortado: el noveno termina a las 22:57 y de ahí a las
	// 23:00 va relleno.
	last := f.at(t, out, "2026-09-07 22:24")
	if !last.End().Equal(f.local(t, "2026-09-07 22:57")) {
		t.Fatalf("el noveno episodio debería terminar a las 22:57 y termina a las %s", last.End().In(f.loc).Format("15:04"))
	}
	relleno := f.at(t, out, "2026-09-07 22:57")
	if relleno.Origin != "relleno" {
		t.Fatalf("de 22:57 a 23:00 va relleno y hay %q", relleno.Origin)
	}
	if got := f.titleOf(f.at(t, out, "2026-09-07 23:00")); got != "You're Under Arrest" {
		t.Fatalf("a las 11 PM empieza la regla siguiente y salió %q", got)
	}
	// Y el contador avanza solo por lo que salió: nueve episodios.
	if got := out.EpisodeAdvance[328]; got != 328*100+9 {
		t.Fatalf("el contador debería quedar en el noveno episodio y quedó en %d", got)
	}
}

// F1-16, F1-37, B5: el segundo pase de las 11 PM emite el mismo episodio que
// la primaria de las 2 PM, y no lleva contador propio.
func TestSegundoPaseMismoEpisodio(t *testing.T) {
	f := newCAtv()
	f.serie(344, "You're Under Arrest", 26, 24*time.Minute)
	f.filler(1, time.Minute)
	f.rule(344, 344, "LMMJV__", "14:00", "2026-07-07", "2026-09-16", 1)
	f.rule(315, 344, "LMMJV__", "23:00", "2026-07-07", "2026-09-16", 1)
	f.tweak(315, func(r *model.ScheduleRule) { id := int64(344); r.RepeatsOf = &id })

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 24*time.Hour))
	tarde := f.at(t, out, "2026-09-07 14:00")
	noche := f.at(t, out, "2026-09-07 23:00")
	if tarde.EpisodeID == nil || noche.EpisodeID == nil || *tarde.EpisodeID != *noche.EpisodeID {
		t.Fatalf("las 11 PM tienen que repetir el episodio de las 2 PM\n%s", f.dump(out))
	}
	if _, ok := out.EpisodeAdvance[315]; ok {
		t.Fatalf("la repetición no lleva contador propio y devolvió avance: %v", out.EpisodeAdvance)
	}
	if out.EpisodeAdvance[344] != *tarde.EpisodeID {
		t.Fatalf("el contador compartido es el de la primaria")
	}

	// Veinte días seguidos: nunca se desincronizan.
	prev := int64(0)
	for d := 0; d < 20; d++ {
		day := f.local(t, "2026-09-07 06:00").AddDate(0, 0, d)
		in := f.in(day, 24*time.Hour)
		if prev != 0 {
			p := prev
			rules := append([]model.ScheduleRule(nil), f.rules...)
			for i := range rules {
				if rules[i].ID == 344 {
					rules[i].LastEpisodeAired = &p
				}
			}
			in.Rules = rules
		}
		o := Resolve(in)
		var a, b *int64
		for _, p := range o.Items {
			if p.RuleID == nil || p.EpisodeID == nil {
				continue
			}
			if *p.RuleID == 344 {
				a = p.EpisodeID
			}
			if *p.RuleID == 315 {
				b = p.EpisodeID
			}
		}
		if a == nil || b == nil {
			continue // fin de semana: ninguna de las dos corre
		}
		if *a != *b {
			t.Fatalf("día %d: la primaria puso %d y la repetición %d", d, *a, *b)
		}
		prev = o.EpisodeAdvance[344]
	}
}

// F1-36: si la primaria no emitió nada ese día, la repetición toma el
// siguiente episodio y avanza el contador compartido.
func TestSegundoPaseSinPrimaria(t *testing.T) {
	f := newCAtv()
	f.serie(344, "You're Under Arrest", 26, 24*time.Minute)
	f.filler(1, time.Minute)
	f.rule(344, 344, "LMMJV__", "14:00", "2026-07-07", "2026-09-04", 1) // ya vencida
	f.rule(315, 344, "LMMJV__", "23:00", "2026-07-07", "2026-09-16", 1)
	f.tweak(315, func(r *model.ScheduleRule) { id := int64(344); r.RepeatsOf = &id })

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 24*time.Hour))
	f.none(t, out, "2026-09-07 14:00")
	noche := f.at(t, out, "2026-09-07 23:00")
	if noche.EpisodeID == nil || *noche.EpisodeID != 344*100+1 {
		t.Fatalf("la repetición debería tomar el siguiente episodio de la primaria\n%s", f.dump(out))
	}
	if out.EpisodeAdvance[344] != 344*100+1 {
		t.Fatalf("el contador compartido tiene que avanzar: %v", out.EpisodeAdvance)
	}
}

// F1-39, C1: el fin de un bloque en vivo es tan duro como el de un slot de
// archivo. RadioOnce Live! de 10 a 13 en la hoja de CAtv.
func TestVivoFinDuro(t *testing.T) {
	f := newCAtv()
	f.serie(337, "Los Simuladores", 20, 24*time.Minute)
	f.filler(1, time.Minute)
	f.live(349, 1, "LMMJV__", "10:00", "2026-09-03", "2026-12-31", 3*time.Hour)
	f.rule(337, 337, "LMMJV__", "13:00", "2026-08-11", "2026-09-11", 2)

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 24*time.Hour))
	vivo := f.at(t, out, "2026-09-07 10:00")
	if vivo.Origin != "live_source" {
		t.Fatalf("el bloque de las 10 es una fuente en vivo y salió como %q", vivo.Origin)
	}
	if !vivo.End().Equal(f.local(t, "2026-09-07 13:00")) {
		t.Fatalf("el bloque en vivo termina a su hora (13:00) y termina a las %s", vivo.End().In(f.loc).Format("15:04"))
	}
	if got := f.titleOf(f.at(t, out, "2026-09-07 13:00")); got != "Los Simuladores" {
		t.Fatalf("a la 1 PM entra Los Simuladores y salió %q", got)
	}
}

// Lo que no termina antes del siguiente inicio duro no se arranca, venga de
// donde venga: aquí un programa de 24 minutos con solo 10 por delante.
func TestNoSeArrancaLoQueNoCabe(t *testing.T) {
	f := newCAtv()
	f.serie(700, "Comics 9th Art", 10, 24*time.Minute)
	f.serie(337, "Los Simuladores", 10, 24*time.Minute)
	f.filler(1, time.Minute)
	f.rule(700, 700, "LMMJV__", "12:50", "2026-09-01", "2026-09-30", 1)
	f.rule(337, 337, "LMMJV__", "13:00", "2026-08-11", "2026-09-11", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 24*time.Hour))
	p := f.at(t, out, "2026-09-07 12:50")
	if p.Origin != "relleno" {
		t.Fatalf("no cabía: esos diez minutos son relleno y salió %q (%s)", p.Origin, f.titleOf(p))
	}
	if !warned(out, WarnOverbook, "caben 0") {
		t.Fatalf("faltó decir que no cupo\n%s", f.dump(out))
	}
	if got := f.titleOf(f.at(t, out, "2026-09-07 13:00")); got != "Los Simuladores" {
		t.Fatalf("la regla siguiente arranca a su hora y salió %q", got)
	}
}

// PRD §9 paso 8 y B9: el diferido de la madrugada reprograma los archivos de
// la ventana de la mañana del mismo día de emisión.
func TestDiferido(t *testing.T) {
	f := newCAtv()
	f.serie(264, "Get Smart", 20, 24*time.Minute)
	f.serie(322, "Kojak", 20, 48*time.Minute)
	f.filler(1, time.Minute)
	f.rule(264, 264, "LMMJV__", "7:00", "2026-03-07", "2026-09-16", 1)
	f.rule(322, 322, "LMMJV__", "8:00", "2026-07-27", "2027-01-04", 1)
	f.timeShift(900, "LMMJV__", "1:00", "7:00", "12:00", "2026-09-01", "2026-12-31", 5*time.Hour)

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 24*time.Hour))

	manana := f.at(t, out, "2026-09-07 07:00")
	diferido := f.at(t, out, "2026-09-08 01:00")
	if diferido.Origin != "asset" || diferido.MediaAssetID == nil || *diferido.MediaAssetID != *manana.MediaAssetID {
		t.Fatalf("el diferido reprograma el mismo archivo de las 7:00\n%s", f.dump(out))
	}
	if diferido.BroadcastDay != "2026-09-07" {
		t.Fatalf("la 1 AM del calendario 8 es del día de emisión del 7 y quedó en %s", diferido.BroadcastDay)
	}
	segundo := f.at(t, out, "2026-09-08 01:24")
	if segundo.MediaAssetID == nil || *segundo.MediaAssetID != *f.at(t, out, "2026-09-07 08:00").MediaAssetID {
		t.Fatalf("el segundo programa diferido es el de las 8:00\n%s", f.dump(out))
	}
	// Solo se difunde el programa: el relleno de la mañana no se copia.
	n := 0
	for _, p := range out.Items {
		if p.RuleID != nil && *p.RuleID == 900 {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("el diferido debería reprogramar 2 programas y reprogramó %d\n%s", n, f.dump(out))
	}
}

// B9: una ventana sin programa no produce diferido, y esa hora cae a relleno.
func TestMadrugadaSinDiferidoVaARelleno(t *testing.T) {
	f := newCAtv()
	f.serie(304, "Mazinger Z", 20, 24*time.Minute)
	f.filler(1, time.Minute)
	f.filler(2, 5*time.Minute)
	f.rule(304, 304, "LMMJV__", "0:30", "2026-06-10", "2026-10-15", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 24*time.Hour))
	for _, p := range out.Items {
		if p.PlannedAt.Before(f.local(t, "2026-09-08 00:54")) {
			continue
		}
		if p.Origin != "relleno" && p.Origin != "cartel" {
			t.Fatalf("la madrugada sin diferido se llena con relleno y hay %q a las %s",
				p.Origin, p.PlannedAt.In(f.loc).Format("15:04"))
		}
	}
	if !warned(out, WarnGap, "no hay nada programado") {
		t.Fatalf("faltó avisar del hueco de la madrugada\n%s", f.dump(out))
	}
	covered(t, f, out, f.local(t, "2026-09-07 06:00"), f.local(t, "2026-09-08 06:00"))
}

// F1-11, regla de integridad: el bug del Hellsing. El esquema ya lo rechaza,
// pero el resolver tampoco se fía.
func TestHellsingNoSeProgramaJamas(t *testing.T) {
	f := newCAtv()
	f.serie(336, "Hellsing", 13, 24*time.Minute)
	f.filler(1, time.Minute)
	f.rule(336, 336, "L_____D", "0:00", "2026-09-15", "2026-01-01", 1)

	out := Resolve(f.in(f.local(t, "2026-09-06 06:00"), 48*time.Hour))
	for _, p := range out.Items {
		if f.titleOf(p) == "Hellsing" {
			t.Fatalf("Hellsing tiene la fecha de fin antes que la de inicio: no puede salir")
		}
	}
	if !warned(out, WarnBadRule, "antes de empezar") {
		t.Fatalf("faltó decir por qué no se programa\n%s", f.dump(out))
	}
}

// §15: la hora que no existe en el cambio de horario se corre a la siguiente
// válida. 8 de marzo de 2026 en Nueva York: no hay 2:30 AM.
func TestHorarioDeVeranoPrimavera(t *testing.T) {
	f := newFx("America/New_York")
	f.serie(500, "Trasnoche", 10, 20*time.Minute)
	f.filler(1, time.Minute)
	f.rule(500, 500, "_____S_", "2:30", "2026-01-01", "2026-12-31", 1)

	out := Resolve(f.in(f.local(t, "2026-03-07 06:00"), 48*time.Hour))
	var found *model.PlanItem
	for i, p := range out.Items {
		if f.titleOf(p) == "Trasnoche" {
			found = &out.Items[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("la regla tenía que salir corrida, no desaparecer\n%s", f.dump(out))
	}
	if got := found.PlannedAt.In(f.loc).Format("2006-01-02 15:04 -0700"); got != "2026-03-08 03:00 -0500" && got != "2026-03-08 03:00 -0400" {
		t.Fatalf("las 2:30 AM del 8 de marzo no existen: esperaba las 3:00 y salió %s", got)
	}
	if found.LocalClock != "03:00" {
		t.Fatalf("la hora local del ítem debería ser 03:00 y es %q", found.LocalClock)
	}
	if found.BroadcastDay != "2026-03-07" {
		t.Fatalf("sigue siendo el día de emisión del sábado 7 y quedó en %s", found.BroadcastDay)
	}
}

// §15: la hora que existe dos veces sale en la primera. 1 de noviembre de
// 2026 en Nueva York: la 1:30 AM pasa dos veces.
func TestHorarioDeVeranoOtono(t *testing.T) {
	f := newFx("America/New_York")
	f.serie(500, "Trasnoche", 10, 20*time.Minute)
	f.filler(1, time.Minute)
	f.rule(500, 500, "_____S_", "1:30", "2026-01-01", "2026-12-31", 1)

	out := Resolve(f.in(f.local(t, "2026-10-31 06:00"), 48*time.Hour))
	var found *model.PlanItem
	for i, p := range out.Items {
		if f.titleOf(p) == "Trasnoche" {
			found = &out.Items[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("faltó la emisión de la madrugada\n%s", f.dump(out))
	}
	if got := found.PlannedAt.UTC().Format("2006-01-02 15:04"); got != "2026-11-01 05:30" {
		t.Fatalf("la 1:30 AM del 1 de noviembre sale en la primera pasada (05:30 UTC) y salió %s UTC", got)
	}
	if found.LocalClock != "01:30" {
		t.Fatalf("la hora local debería ser 01:30 y es %q", found.LocalClock)
	}
}

// B1: una regla de las 5:59 AM pertenece al día de emisión anterior.
func TestReglaDeLasCincoCincuentaYNueve(t *testing.T) {
	f := newCAtv()
	f.serie(600, "Cierre", 10, 1*time.Minute)
	f.filler(1, time.Minute)
	f.rule(600, 600, "LMMJV__", "5:59", "2026-09-01", "2026-09-30", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 24*time.Hour))
	p := f.at(t, out, "2026-09-08 05:59")
	if p.BroadcastDay != "2026-09-07" {
		t.Fatalf("las 5:59 AM del martes de calendario son del lunes de emisión y quedaron en %s", p.BroadcastDay)
	}
}

// F1-40, B3: un programa de tres horas que arranca a las 5:00 AM pertenece
// entero al día de emisión de su inicio, aunque termine a las 8:00.
func TestProgramaQueCruzaElIniciodelDia(t *testing.T) {
	f := newCAtv()
	f.pelicula(610, "Maratón de madrugada", 3*time.Hour)
	f.filler(1, time.Minute)
	f.rule(610, 610, "LMMJV__", "5:00", "2026-09-01", "2026-09-30", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 06:00"), 48*time.Hour))
	p := f.at(t, out, "2026-09-08 05:00")
	if p.PlannedMs != (3 * time.Hour).Milliseconds() {
		t.Fatalf("el bloque dura tres horas y quedó en %d ms", p.PlannedMs)
	}
	if p.BroadcastDay != "2026-09-07" {
		t.Fatalf("pertenece al día de emisión del lunes 7 y quedó en %s", p.BroadcastDay)
	}
	if !p.End().Equal(f.local(t, "2026-09-08 08:00")) {
		t.Fatalf("termina a las 8:00 del martes y termina a las %s", p.End().In(f.loc).Format("15:04"))
	}
}

// F1-21 y F1-22, §14.1: el relleno cuadra exacto, y si no puede, excede como
// mucho 5 segundos recortando el último clip.
func TestRellenoEmpaqueta(t *testing.T) {
	lib := []model.FillerAsset{
		{ID: 1, DurationMs: (4 * time.Minute).Milliseconds()},
		{ID: 2, DurationMs: (3 * time.Minute).Milliseconds()},
		{ID: 3, DurationMs: (2 * time.Minute).Milliseconds()},
	}
	plan := packFillers((7 * time.Minute).Milliseconds(), lib)
	if len(plan.clips) != 2 || plan.trim != 0 || plan.short != 0 {
		t.Fatalf("un hueco de 7:00 se cubre con 3:00 + 4:00 exactos: %+v", plan)
	}
	if plan.clips[0].DurationMs != (3*time.Minute).Milliseconds() || plan.clips[1].DurationMs != (4*time.Minute).Milliseconds() {
		t.Fatalf("esperaba 3:00 y luego 4:00 y salió %d + %d", plan.clips[0].DurationMs, plan.clips[1].DurationMs)
	}

	lib2 := []model.FillerAsset{
		{ID: 1, DurationMs: (3 * time.Minute).Milliseconds()},
		{ID: 2, DurationMs: (2 * time.Minute).Milliseconds()},
	}
	plan2 := packFillers((5*time.Minute + 57*time.Second).Milliseconds(), lib2)
	var sum int64
	for _, c := range plan2.clips {
		sum += c.DurationMs
	}
	if sum-plan2.trim != (5*time.Minute + 57*time.Second).Milliseconds() {
		t.Fatalf("el hueco residual tiene que quedar en cero: %+v", plan2)
	}
	if plan2.trim <= 0 || plan2.trim > MaxFillerExcess.Milliseconds() {
		t.Fatalf("el recorte tiene que caber en los 5 s del §14.1 y es %d ms", plan2.trim)
	}
	if plan2.short != 0 {
		t.Fatalf("no debería quedar nada al cartel: %+v", plan2)
	}

	// Sin biblioteca de relleno, todo el hueco va al cartel.
	plan3 := packFillers((30 * time.Minute).Milliseconds(), nil)
	if plan3.short != (30 * time.Minute).Milliseconds() {
		t.Fatalf("sin relleno el hueco entero va al cartel: %+v", plan3)
	}
}

// Sin relleno cargado, el hueco lo cubre el cartel de la estación y se dice.
func TestSinRellenoSaleElCartel(t *testing.T) {
	f := newCAtv()
	f.serie(304, "Mazinger Z", 20, 24*time.Minute)
	f.rule(304, 304, "LMMJV__", "8:00", "2026-06-10", "2026-10-15", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 08:00"), 2*time.Hour))
	if countOrigin(out, "cartel") == 0 {
		t.Fatalf("sin relleno tiene que salir el cartel\n%s", f.dump(out))
	}
	if !warned(out, WarnNoFiller, "cartel de la estación") {
		t.Fatalf("faltó decir que no hay relleno\n%s", f.dump(out))
	}
	covered(t, f, out, f.local(t, "2026-09-07 08:00"), f.local(t, "2026-09-07 10:00"))
}

// F1-20, F1-41, B7: solo entra al plan material listo y normalizado.
func TestSoloMaterialListo(t *testing.T) {
	f := newCAtv()
	f.sinMaterial(620, "Película en cuarentena", time.Hour)
	f.filler(1, time.Minute)
	f.rule(620, 620, "LMMJV__", "8:00", "2026-09-01", "2026-09-30", 1)

	out := Resolve(f.in(f.local(t, "2026-09-07 08:00"), 2*time.Hour))
	for _, p := range out.Items {
		if p.Origin == "asset" {
			t.Fatalf("no había material listo y programó algo igual\n%s", f.dump(out))
		}
	}
	if !warned(out, WarnNoMaterial, "no tiene archivo listo para aire") {
		t.Fatalf("faltó el aviso de material\n%s", f.dump(out))
	}

	// Lo mismo si el archivo está listo pero la normalización no ha terminado.
	f2 := newCAtv()
	f2.pelicula(621, "Recién subida", time.Hour)
	f2.filler(1, time.Minute)
	for id, a := range f2.assets {
		if a.DurationMs == time.Hour.Milliseconds() {
			a.NormalizeState = "en_curso"
			f2.assets[id] = a
		}
	}
	f2.rule(621, 621, "LMMJV__", "8:00", "2026-09-01", "2026-09-30", 1)
	out2 := Resolve(f2.in(f2.local(t, "2026-09-07 08:00"), 2*time.Hour))
	if !warned(out2, WarnNoMaterial, "") {
		t.Fatalf("un archivo a medio normalizar no entra al plan\n%s", f2.dump(out2))
	}
}

// F1-23, F1-45, B10: avisos a 30, 14 y 7 días, y silencio si hay relevo.
func TestAvisosDeVencimiento(t *testing.T) {
	cases := []struct {
		fin    string
		notice string
	}{
		{"2026-10-07", "30"},
		{"2026-09-21", "14"},
		{"2026-09-14", "7"},
		{"2026-11-30", ""},
	}
	for _, c := range cases {
		f := newCAtv()
		f.serie(322, "Kojak", 20, 24*time.Minute)
		f.filler(1, time.Minute)
		f.rule(322, 322, "LMMJV__", "8:00", "2026-07-27", c.fin, 1)
		out := Resolve(f.in(f.local(t, "2026-09-07 08:00"), time.Hour))
		got := ""
		for _, w := range out.Warnings {
			if w.Kind == WarnExpiry {
				got = w.Notice
			}
		}
		if got != c.notice {
			t.Fatalf("con fin %s esperaba el aviso %q y salió %q", c.fin, c.notice, got)
		}
	}

	// Con relevo cargado no se avisa: hay quien la sustituya.
	f := newCAtv()
	f.serie(346, "Magic Knight Rayearth", 20, 24*time.Minute)
	f.serie(352, "Zoids", 20, 24*time.Minute)
	f.filler(1, time.Minute)
	f.rule(346, 346, "LMMJV__", "15:00", "2026-07-01", "2026-09-14", 1)
	f.rule(352, 352, "LMMJV__", "15:00", "2026-09-15", "2026-12-09", 1)
	f.tweak(352, func(r *model.ScheduleRule) { id := int64(346); r.HandsOffTo = &id })
	out := Resolve(f.in(f.local(t, "2026-09-07 08:00"), time.Hour))
	if warned(out, WarnExpiry, "") {
		t.Fatalf("con relevo cargado no se avisa del vencimiento\n%s", f.dump(out))
	}

	// Y no se repite el mismo aviso dos veces.
	f2 := newCAtv()
	f2.serie(322, "Kojak", 20, 24*time.Minute)
	f2.filler(1, time.Minute)
	f2.rule(322, 322, "LMMJV__", "8:00", "2026-07-27", "2026-09-14", 1)
	f2.tweak(322, func(r *model.ScheduleRule) { r.LastNoticeSent = "7" })
	out2 := Resolve(f2.in(f2.local(t, "2026-09-07 08:00"), time.Hour))
	if warned(out2, WarnExpiry, "") {
		t.Fatalf("ese aviso ya se había mandado\n%s", f2.dump(out2))
	}
}

// Lo que ya está cued o aired no se mueve ni se duplica; lo planned se
// ignora, porque el store lo reemplaza con lo que devolvemos.
func TestNoTocaLoQueYaEstaEnMarcha(t *testing.T) {
	f := newCAtv()
	f.serie(322, "Kojak", 20, 24*time.Minute)
	f.filler(1, time.Minute)
	f.rule(322, 322, "LMMJV__", "8:00", "2026-07-27", "2027-01-04", 1)

	in := f.in(f.local(t, "2026-09-07 08:00"), 2*time.Hour)
	in.Existing = []model.PlanItem{{
		ID: 99, ChannelID: 1, DeckID: deckProg,
		PlannedAt: f.local(t, "2026-09-07 08:00"), PlannedMs: (30 * time.Minute).Milliseconds(),
		Origin: "asset", State: model.Aired,
	}, {
		ID: 100, ChannelID: 1, DeckID: deckProg,
		PlannedAt: f.local(t, "2026-09-07 09:00"), PlannedMs: (10 * time.Minute).Milliseconds(),
		Origin: "asset", State: model.Planned,
	}}
	out := Resolve(in)
	for _, p := range out.Items {
		if p.PlannedAt.Before(f.local(t, "2026-09-07 08:30")) {
			t.Fatalf("no se puede meter nada donde ya salió algo: %s %s",
				p.PlannedAt.In(f.loc).Format("15:04"), p.Origin)
		}
	}
	// El hueco de las 9:00, que era un planned viejo, sí se vuelve a llenar.
	if len(out.Items) == 0 {
		t.Fatalf("debería haber plan nuevo después de las 8:30")
	}
}

// Determinista: la misma entrada da exactamente la misma salida, y correrlo
// dos veces no duplica nada.
func TestDeterminista(t *testing.T) {
	f := catvWeek()
	in := f.in(f.local(t, "2026-09-06 06:00"), 48*time.Hour)
	a := Resolve(in)
	b := Resolve(in)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("dos corridas con la misma entrada dieron planes distintos")
	}
	if len(a.Items) == 0 {
		t.Fatalf("la semana de CAtv no puede quedar vacía")
	}
}

// F1-24: la semana entera queda cubierta, sin un segundo de aire sin ítem.
func TestSemanaDeCAtvSinHuecos(t *testing.T) {
	f := catvWeek()
	from := f.local(t, "2026-09-06 06:00")
	out := Resolve(f.in(from, 48*time.Hour))
	covered(t, f, out, from, from.Add(48*time.Hour))

	// Y el domingo, que en la hoja está casi vacío, sale con relleno y aviso.
	if !warned(out, WarnGap, "") {
		t.Fatalf("el domingo de CAtv tiene horas vacías: hay que decirlo\n%s", f.dump(out))
	}
	if countOrigin(out, "relleno") == 0 {
		t.Fatalf("faltó el relleno")
	}
}

// catvWeek arma la parrilla de la hoja de CAtv para la semana del 6 al 12 de
// septiembre de 2026, con los ids reales de las reglas.
func catvWeek() *fx {
	f := newCAtv()
	type s struct {
		id   int64
		name string
		mins int
	}
	series := []s{
		{264, "Get Smart", 24}, {265, "I Dream of Jeannie", 24}, {316, "Tarzan", 24},
		{322, "Kojak", 24}, {350, "Comics 9th Art", 24}, {351, "Gilligan's Island", 24},
		{337, "Los Simuladores", 44}, {344, "You're Under Arrest", 24}, {345, "Samurai X", 24},
		{346, "Magic Knight Rayearth", 24}, {347, "Mazinger Z", 24}, {343, "Green Hornet", 24},
		{314, "Zorro 57", 24}, {342, "Los Lorcanitos", 24}, {328, "Gaming Longplays", 33},
		{311, "SamuraiX", 24}, {313, "Magic Knight Rayearth (madrugada)", 24},
		{304, "Mazinger Z (madrugada)", 24}, {352, "Zoids", 24}, {353, "Zoids (madrugada)", 24},
		{339, "Carmen Sandiego", 24}, {330, "Don Quijote", 24}, {331, "TinTin", 24},
		{273, "Familia Robinson", 24}, {321, "Sonic The Hedgehog", 24}, {310, "Astroboy", 24},
		{329, "Corrector Yui", 24}, {209, "Voyagesea", 24}, {303, "SaberMarionette", 24},
		{317, "BT'x", 24}, {333, "Hack Legend", 24}, {336, "Hellsing", 24},
	}
	for _, x := range series {
		f.serie(x.id, x.name, 26, time.Duration(x.mins)*time.Minute)
	}
	for i, d := range []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 5 * time.Minute} {
		f.filler(int64(i+1), d)
	}

	// Entre semana.
	f.rule(264, 264, "LMMJV__", "6:00", "2026-03-07", "2026-09-16", 1)
	f.rule(265, 265, "LMMJV__", "6:30", "2026-03-07", "2026-09-15", 1)
	f.rule(316, 316, "LMMJV__", "7:00", "2026-07-07", "2026-09-23", 2)
	f.rule(322, 322, "LMMJV__", "8:00", "2026-07-27", "2027-01-04", 2)
	f.rule(350, 350, "LMMJV__", "9:00", "2026-09-04", "2026-09-22", 1)
	f.rule(351, 351, "LMMJV__", "9:30", "2026-09-04", "2027-01-19", 1)
	f.live(349, 1, "LMMJV__", "10:00", "2026-09-03", "2026-12-31", 3*time.Hour)
	f.rule(337, 337, "LMMJV__", "13:00", "2026-08-11", "2026-09-11", 1)
	f.rule(344, 344, "LMMJV__", "14:00", "2026-07-07", "2026-09-16", 1)
	f.rule(345, 345, "LMMJV__", "14:30", "2026-06-25", "2026-11-04", 1)
	f.rule(346, 346, "LMMJV__", "15:00", "2026-07-01", "2026-09-07", 1)
	f.rule(352, 352, "LMMJV__", "15:00", "2026-09-08", "2026-12-09", 1)
	f.tweak(352, func(r *model.ScheduleRule) { id := int64(346); r.HandsOffTo = &id })
	f.rule(347, 347, "LMMJV__", "15:30", "2026-06-10", "2026-10-15", 1)
	f.rule(343, 343, "LMMJV__", "16:00", "2026-08-26", "2026-09-30", 1)
	f.rule(314, 314, "LMMJV__", "16:30", "2026-07-02", "2026-10-21", 1)
	f.rule(342, 342, "LMMJV__", "17:00", "2026-08-26", "2027-01-04", 2)
	f.rule(328, 328, "LMMJV__", "18:00", "2026-08-31", "2026-09-28", 10)
	f.rule(315, 344, "LMMJV__", "23:00", "2026-07-07", "2026-09-16", 1)
	f.tweak(315, func(r *model.ScheduleRule) { id := int64(344); r.RepeatsOf = &id })
	f.rule(311, 311, "LMMJV__", "23:30", "2026-06-25", "2026-11-04", 1)
	// Madrugada, ya en día de emisión (el importador corrió las fechas un día).
	f.rule(313, 313, "LMMJV__", "0:00", "2026-07-01", "2026-09-07", 1)
	f.rule(353, 353, "LMMJV__", "0:00", "2026-09-08", "2026-12-09", 1)
	f.rule(304, 304, "LMMJV__", "0:30", "2026-06-10", "2026-10-15", 1)

	// Fin de semana.
	f.rule(339, 339, "_____SD", "7:30", "2026-08-22", "2027-01-03", 1)
	f.rule(330, 330, "_____SD", "8:00", "2026-08-09", "2026-12-20", 1)
	f.rule(331, 331, "_____SD", "8:30", "2026-08-09", "2026-12-20", 1)
	f.rule(273, 273, "_____SD", "9:00", "2026-03-21", "2026-09-06", 1)
	f.rule(321, 321, "_____SD", "9:30", "2026-07-26", "2027-03-07", 1)
	f.rule(310, 310, "_____SD", "10:00", "2026-06-13", "2026-12-06", 1)
	f.rule(329, 329, "_____SD", "10:30", "2026-08-09", "2027-02-06", 1)
	f.rule(209, 209, "_____SD", "14:00", "2025-10-24", "2026-11-08", 2)
	f.rule(327, 328, "_____SD", "18:00", "2026-08-08", "2026-10-17", 10)
	f.rule(303, 303, "_____SD", "23:00", "2026-05-25", "2026-12-12", 1)
	f.rule(317, 317, "_____SD", "23:30", "2026-07-07", "2026-10-03", 1)
	f.rule(333, 333, "_____SD", "0:00", "2026-08-07", "2026-09-13", 1)
	// La fila del Hellsing: termina antes de empezar.
	f.rule(336, 336, "_____SD", "0:00", "2026-09-15", "2026-01-01", 1)
	return f
}
