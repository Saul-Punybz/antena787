package resolver

import (
	"strings"
	"testing"
	"time"

	"antena787/internal/model"
)

// F1-29 — La Regla A vigente hasta ayer a las 8:00 AM y la Regla B
// (releva_a = A) desde hoy en la misma franja: el XMLTV de hoy tiene que
// mostrar el título de B, no el de A.
func TestF1Verif29GuiaDelDiaMuestraElRelevo(t *testing.T) {
	f := newCAtv()
	f.serie(346, "Magic Knight Rayearth", 26, 24*time.Minute)
	f.serie(352, "Zoids", 26, 24*time.Minute)
	f.filler(1, time.Minute)
	// A vence ayer (7 de septiembre); B arranca hoy (8) y la releva.
	f.rule(346, 346, "LMMJV__", "08:00", "2026-07-01", "2026-09-07", 1)
	f.rule(352, 352, "LMMJV__", "08:00", "2026-09-08", "2026-12-09", 1)
	f.tweak(352, func(r *model.ScheduleRule) { id := int64(346); r.HandsOffTo = &id })

	hoy := f.local(t, "2026-09-08 06:00")
	out := Resolve(f.in(hoy, 12*time.Hour))

	data, err := XMLTV(f.ch, out.Items, f.titles, f.episodeIndex())
	if err != nil {
		t.Fatalf("no se pudo escribir la guía: %v", err)
	}
	s := string(data)

	bloque := programaQueEmpieza(t, s, "20260908080000 -0400")
	if !strings.Contains(bloque, "Zoids") {
		t.Fatalf("el bloque de las 8:00 del 8 de septiembre tiene que decir Zoids:\n%s", bloque)
	}
	if strings.Contains(bloque, "Magic Knight") {
		t.Fatalf("el bloque de las 8:00 sigue anunciando la regla vencida:\n%s", bloque)
	}
	if problemas := ValidateXMLTV(data); len(problemas) != 0 {
		t.Fatalf("la guía del relevo tiene que pasar el validador: %v", problemas)
	}
}

// programaQueEmpieza devuelve el <programme …> cuyo start coincide.
func programaQueEmpieza(t *testing.T, guia, start string) string {
	t.Helper()
	for _, trozo := range strings.Split(guia, "<programme") {
		if strings.Contains(trozo, `start="`+start+`"`) {
			return trozo
		}
	}
	t.Fatalf("la guía no tiene ningún programa que empiece en %s:\n%s", start, guia)
	return ""
}

// F1-30 — En modo sombra el resolver deja instante_real y duracion_real
// vacíos (no hay motor de aire), mientras instante_planeado y
// duracion_planeada sí están poblados.
func TestF1Verif30SombraNoEscribeAsRun(t *testing.T) {
	f := catvWeek()
	f.ch.Mode = "sombra"
	desde := f.local(t, "2026-09-07 06:00")
	out := Resolve(f.in(desde, 48*time.Hour))

	if len(out.Items) == 0 {
		t.Fatal("el plan salió vacío: no hay nada que comprobar")
	}
	for _, p := range out.Items {
		if p.ActualAt != nil {
			t.Fatalf("el resolver puso instante_real (%v) en %s: en sombra nadie escribe el as-run",
				*p.ActualAt, p.PlannedAt.In(f.loc).Format("2006-01-02 15:04"))
		}
		if p.ActualMs != nil {
			t.Fatalf("el resolver puso duracion_real (%v) en %s",
				*p.ActualMs, p.PlannedAt.In(f.loc).Format("2006-01-02 15:04"))
		}
		if p.CuedAt != nil {
			t.Fatalf("el resolver marcó cued_en en %s: en sombra nada se carga",
				p.PlannedAt.In(f.loc).Format("2006-01-02 15:04"))
		}
		if p.State != model.Planned {
			t.Fatalf("el ítem de las %s salió en estado %q y en sombra todo queda planned",
				p.PlannedAt.In(f.loc).Format("2006-01-02 15:04"), p.State)
		}
		if p.PlannedAt.IsZero() || p.PlannedMs <= 0 {
			t.Fatalf("el ítem de la regla %v no trae instante o duración planeada: %+v", ruleOf(p), p)
		}
	}
}

// F1-55 — Una schedule_rule con tipo = bloque_arrendado, su advertiser y su
// cobro se materializa como cualquier otra regla y queda en la parrilla.
func TestF1Verif55BloqueArrendadoEnLaParrilla(t *testing.T) {
	f := newCAtv()
	f.pelicula(700, "Iglesia Roca de Salvación", 30*time.Minute)
	f.serie(315, "You're Under Arrest", 26, 24*time.Minute)
	f.filler(1, time.Minute)
	f.rule(315, 315, "LMMJV__", "09:00", "2026-07-01", "2026-12-09", 1)
	f.rule(700, 700, "_____S_", "08:00", "2026-07-01", "2026-12-09", 1)
	anunciante := int64(1)
	cobro := 250.0
	f.tweak(700, func(r *model.ScheduleRule) {
		r.Kind = model.RuleLeased
		r.AdvertiserID = &anunciante
		r.Fee = &cobro
		r.SlotMs = (30 * time.Minute).Milliseconds()
	})

	// El sábado 12 de septiembre de 2026.
	out := Resolve(f.in(f.local(t, "2026-09-12 06:00"), 12*time.Hour))

	it := f.at(t, out, "2026-09-12 08:00")
	if got := f.titleOf(it); got != "Iglesia Roca de Salvación" {
		t.Fatalf("el bloque arrendado tiene que estar en la parrilla como cualquier otro; salió %q\n%s",
			got, f.dump(out))
	}
	if it.RuleID == nil || *it.RuleID != 700 {
		t.Fatalf("el bloque no quedó atado a su regla: %+v", it)
	}
	if warned(out, WarnBadRule, "") {
		t.Fatalf("el bloque arrendado no puede salir como regla inválida\n%s", f.dump(out))
	}

	// Y también entra en la guía, que es lo que ve el público.
	data, err := XMLTV(f.ch, out.Items, f.titles, f.episodeIndex())
	if err != nil {
		t.Fatalf("no se pudo escribir la guía: %v", err)
	}
	if !strings.Contains(string(data), "Iglesia Roca de Salvación") {
		t.Fatal("el bloque arrendado no salió en la guía")
	}
}

// F1-76 — Un título marcado como programa infantil educativo (E/I) sale en la
// guía con la categoría Infantil/Children; el que no lo está, no. Es lo que
// deja ver desde fuera la programación infantil de una estación Class A.
func TestF1Verif76LaGuiaAnunciaElProgramaInfantil(t *testing.T) {
	f := newCAtv()
	f.serie(1, "Carmen Sandiego", 26, 24*time.Minute)
	f.serie(2, "Kojak", 26, 24*time.Minute)
	infantil := f.titles[1]
	infantil.InfantilCore = true
	f.titles[1] = infantil
	f.filler(1, time.Minute)
	f.rule(1, 1, "LMMJV__", "07:30", "2026-09-01", "2026-12-31", 1)
	f.rule(2, 2, "LMMJV__", "08:00", "2026-09-01", "2026-12-31", 1)

	out := Resolve(f.in(f.local(t, "2026-09-08 06:00"), 6*time.Hour))
	data, err := XMLTV(f.ch, out.Items, f.titles, f.episodeIndex())
	if err != nil {
		t.Fatalf("no se pudo escribir la guía: %v", err)
	}
	s := string(data)

	bloque := programaQueEmpieza(t, s, "20260908073000 -0400")
	if !strings.Contains(bloque, `<category lang="es">Infantil</category>`) {
		t.Fatalf("el programa marcado infantil tiene que salir con la categoría en español:\n%s", bloque)
	}
	if !strings.Contains(bloque, `<category lang="en">Children</category>`) {
		t.Fatalf("y con la categoría en inglés, que es la que leen las guías de fuera:\n%s", bloque)
	}
	if !strings.Contains(bloque, `<category lang="es">Animación</category>`) {
		t.Fatalf("la marca no puede llevarse por delante el género de siempre:\n%s", bloque)
	}

	otro := programaQueEmpieza(t, s, "20260908080000 -0400")
	if strings.Contains(otro, "Infantil") || strings.Contains(otro, "Children") {
		t.Fatalf("un título sin marcar no puede salir como programación infantil:\n%s", otro)
	}

	if problemas := ValidateXMLTV(data); len(problemas) != 0 {
		t.Fatalf("la guía con la categoría infantil tiene que pasar el validador: %v", problemas)
	}
}
