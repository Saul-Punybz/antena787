package app

import (
	"context"
	"testing"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// Pruebas de los decks (T2 de F2): quién tiene el aire cuando dos bloques
// quieren salir a la vez, y qué le pasa al programa cuando entra un corte.
//
// El reloj de estas pruebas está clavado a la una y cincuenta de la tarde, en
// UTC, para que «las 14:00:00» del criterio F2-06 sean de verdad las 14:00:00 y
// no «dentro de diez minutos».

// alasDosEnPunto es el instante del corte pautado de F2-06.
var alasDosEnPunto = time.Date(2026, 9, 10, 14, 0, 0, 0, time.UTC)

// conReloj abre la aplicación con el reloj clavado a un instante.
func conReloj(t *testing.T, cuando time.Time) *App {
	t.Helper()
	return abre(t, func(o *Options) { o.Now = func() time.Time { return cuando } })
}

// F2-06 — el corte pautado a las 14:00:00 entra a esa hora exacta, sin esperar
// a que el programa llegue a su propio corte natural. Y F2-07: el programa, que
// viene de un archivo, se pausa y reanuda exactamente donde iba.
func TestF2_06y07ElCorteDeLasDosEntraEnPuntoYElProgramaReanudaDondeIba(t *testing.T) {
	ahora := alasDosEnPunto.Add(-10 * time.Minute) // 13:50
	a := conReloj(t, ahora)
	ctx := context.Background()
	conCartel(t, a)

	// Un espacio de 30 minutos: 24 de programa y 6 de cortes, como el PRD.
	// Empieza a las 13:45 y el primer corte cae a las 14:00:00 en punto.
	_, programa := conBloque(t, a, "Kojak", alasDosEnPunto.Add(-15*time.Minute), 30*time.Minute)
	spot, _ := conBloque(t, a, "spot de la ferretería", ahora.Add(48*time.Hour), 2*time.Minute)
	corte := conBloqueDe(t, a, spot, model.DeckCommercial, alasDosEnPunto, 2*time.Minute, nil)

	f := a.nuevaFuenteDelPlan(ctx, engine.CAtv)

	// A las 13:50 el aire es del programa, y el corte que se sabe que viene
	// es el del deck comercial: las 14:00:00, no el fin del programa.
	clip, hasta, err := f.Next(ahora)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	if clip.Ref != programa.ID {
		t.Fatalf("a las 13:50 salió el bloque %d y tenía que salir el programa %d", clip.Ref, programa.ID)
	}
	if !hasta.Equal(alasDosEnPunto) {
		t.Fatalf("el corte es %s y tenía que ser las 14:00:00 en punto", hasta.Format("15:04:05.000"))
	}

	// A las 14:00:00 el aire lo toma el deck comercial: más prioridad.
	clip, hasta, err = f.Next(alasDosEnPunto)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	if clip.Ref != corte.ID {
		t.Fatalf("a las 14:00:00 salió el bloque %d y tenía que salir el corte %d", clip.Ref, corte.ID)
	}
	if !hasta.Equal(corte.End()) {
		t.Fatalf("el corte acaba a las %s y el fin del spot es %s", hasta, corte.End())
	}

	// Y al terminar el corte, el programa vuelve **por donde iba**: 15 minutos
	// de programa, no los 17 que dice la hora de pared (F2-07).
	vuelta := corte.End()
	clip, hasta, err = f.Next(vuelta)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	if clip.Ref != programa.ID {
		t.Fatalf("al acabar el corte salió el bloque %d y tenía que volver el programa %d", clip.Ref, programa.ID)
	}
	if quiere := int64(15 * 60 * 1000); clip.SeekMs != quiere {
		t.Fatalf("el programa vuelve por el milisegundo %d y tenía que volver por el %d, donde se quedó",
			clip.SeekMs, quiere)
	}
	if !hasta.Equal(programa.End()) {
		t.Fatalf("después del corte el programa sigue hasta %s y tenía que seguir hasta %s", hasta, programa.End())
	}

	// Un segundo corte más adelante vuelve a pausarlo, y lo que ya salió no se
	// vuelve a emitir: el avance se acumula.
	otro := conBloqueDe(t, a, spot, model.DeckCommercial, alasDosEnPunto.Add(5*time.Minute), time.Minute, nil)
	if _, _, err := f.Next(otro.PlannedAt); err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	clip, _, err = f.Next(otro.End())
	if err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	// 15 minutos antes del primer corte + los 3 que corrió entre los dos.
	if quiere := int64(18 * 60 * 1000); clip.Ref != programa.ID || clip.SeekMs != quiere {
		t.Fatalf("tras el segundo corte el programa vuelve por el ms %d (bloque %d) y tocaba por el %d",
			clip.SeekMs, clip.Ref, quiere)
	}
}

// F2-08 — cuando el corte entra sobre una fuente en vivo, la señal **no se
// pausa**: sigue corriendo por debajo, esos minutos se pierden y el bloque no
// se extiende.
//
// El vivo de verdad —abrir la señal y sacarla al aire— es T4; lo que se mide
// aquí es la regla que decide por dónde se vuelve y hasta cuándo dura el
// bloque, que es lo que el criterio pide y lo que el motor ya usa.
func TestF2_08ElVivoNoSePausaYElBloqueNoSeExtiende(t *testing.T) {
	ahora := alasDosEnPunto.Add(-10 * time.Minute)
	a := conReloj(t, ahora)
	ctx := context.Background()
	conCartel(t, a)

	vivo := bloqueEnVivo(t, a, alasDosEnPunto.Add(-15*time.Minute), 30*time.Minute)
	spot, _ := conBloque(t, a, "spot del colmado", ahora.Add(48*time.Hour), 2*time.Minute)
	corte := conBloqueDe(t, a, spot, model.DeckCommercial, alasDosEnPunto, 2*time.Minute, nil)

	f := a.nuevaFuenteDelPlan(ctx, engine.CAtv)
	items, err := a.Store.Plan.ListRange(ctx, a.ChannelID, ahora.Add(-VentanaAtras), ahora.Add(VentanaAdelante))
	if err != nil {
		t.Fatalf("no pude leer el plan: %v", err)
	}

	// A las 13:50 el aire es del vivo y el corte conocido es las 14:00:00.
	it, hay := f.queToca(items, ahora)
	if !hay || it.ID != vivo.ID {
		t.Fatalf("a las 13:50 el aire tenía que ser del vivo y es del bloque %d", it.ID)
	}
	if hasta := f.corteDe(items, it, ahora); !hasta.Equal(alasDosEnPunto) {
		t.Fatalf("el corte del vivo es %s y tenía que ser las 14:00:00", hasta.Format("15:04:05"))
	}

	// A las 14:00:00 el corte comercial tiene el aire, aunque debajo el vivo
	// siga corriendo.
	it, hay = f.queToca(items, alasDosEnPunto)
	if !hay || it.ID != corte.ID {
		t.Fatalf("a las 14:00:00 el aire tenía que ser del corte y es del bloque %d", it.ID)
	}

	// Al volver, la señal se toma en su instante actual: no hay nada que
	// reanudar, así que la posición es cero y no el punto en que se
	// interrumpió.
	vuelta := corte.End()
	it, hay = f.queToca(items, vuelta)
	if !hay || it.ID != vivo.ID {
		t.Fatalf("al acabar el corte el aire tenía que volver al vivo y volvió al bloque %d", it.ID)
	}
	if pos := f.posicionDe(it, vuelta); pos != 0 {
		t.Fatalf("al vivo se vuelve por %s y se tenía que volver a su instante actual", pos)
	}
	// Y el bloque termina a su hora: los dos minutos del corte se pierden, no
	// se le añaden al final.
	if hasta := f.corteDe(items, it, vuelta); !hasta.Equal(vivo.End()) {
		t.Fatalf("el vivo acaba a las %s y tenía que acabar a las %s, a su hora",
			hasta.Format("15:04:05"), vivo.End().Format("15:04:05"))
	}
}

// La prioridad es la del PRD §9 paso 4: manual manda sobre comercial, comercial
// sobre programa, programa sobre relleno. Y cada cambio de deck queda dicho,
// con la hora.
func TestElAireLoTieneElDeckDeMasPrioridadYElCambioQuedaDicho(t *testing.T) {
	ahora := alasDosEnPunto.Add(-10 * time.Minute)
	a := conReloj(t, ahora)
	ctx := context.Background()
	var vistos eventos
	vistos.mirando(a, t)
	conCartel(t, a)

	// Los cuatro a la vez sobre el mismo minuto, cada uno en su deck.
	base := alasDosEnPunto.Add(-15 * time.Minute)
	relleno, _ := conBloque(t, a, "cortinilla", ahora.Add(48*time.Hour), 30*time.Minute)
	programa, _ := conBloque(t, a, "Kojak", ahora.Add(72*time.Hour), 30*time.Minute)
	spot, _ := conBloque(t, a, "spot", ahora.Add(96*time.Hour), 30*time.Minute)
	mano, _ := conBloque(t, a, "lo que disparó el operador", ahora.Add(120*time.Hour), 30*time.Minute)

	conBloqueDe(t, a, relleno, model.DeckFiller, base, 30*time.Minute, nil)
	deProgram := conBloqueDe(t, a, programa, model.DeckProgram, base, 30*time.Minute, nil)
	f := a.nuevaFuenteDelPlan(ctx, engine.CAtv)

	if clip, _, _ := f.Next(ahora); clip.Ref != deProgram.ID {
		t.Fatalf("entre relleno y programa salió el bloque %d y tenía que salir el programa %d", clip.Ref, deProgram.ID)
	}
	esperar(t, 2*time.Second, func() bool {
		return vistos.hay("motor", "deck", "el aire lo toma el deck programa")
	}, "el cambio al deck programa no quedó dicho")

	deComercial := conBloqueDe(t, a, spot, model.DeckCommercial, base, 30*time.Minute, nil)
	if clip, _, _ := f.Next(ahora); clip.Ref != deComercial.ID {
		t.Fatalf("entre programa y comercial salió el bloque %d y tenía que salir el comercial %d", clip.Ref, deComercial.ID)
	}
	esperar(t, 2*time.Second, func() bool {
		return vistos.hay("motor", "deck", "el aire lo toma el deck comercial")
	}, "el cambio al deck comercial no quedó dicho")

	deManual := conBloqueDe(t, a, mano, model.DeckManual, base, 30*time.Minute, nil)
	if clip, _, _ := f.Next(ahora); clip.Ref != deManual.ID {
		t.Fatalf("entre comercial y manual salió el bloque %d y tenía que salir el manual %d", clip.Ref, deManual.ID)
	}
	esperar(t, 2*time.Second, func() bool {
		return vistos.hay("motor", "deck", "el aire lo toma el deck manual")
	}, "el cambio al deck manual no quedó dicho")

	// Y cuando el plan no tiene nada, el que tiene el aire es el relleno, y
	// también queda dicho.
	f2 := a.nuevaFuenteDelPlan(ctx, engine.CAtv)
	if _, _, err := f2.Next(ahora.Add(6 * time.Hour)); err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	esperar(t, 2*time.Second, func() bool {
		return vistos.hay("motor", "deck", "el aire lo toma el deck relleno")
	}, "el paso al relleno no quedó dicho")
}

// bloqueEnVivo mete un bloque del plan cuyo origen es una fuente en vivo. La
// fuente de verdad es T4; lo que hace falta aquí es un bloque que el motor
// trate como vivo.
func bloqueEnVivo(t *testing.T, a *App, arranca time.Time, dura time.Duration) model.PlanItem {
	t.Helper()
	ctx := context.Background()
	arranca = arranca.Truncate(time.Millisecond)
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer el canal: %v", err)
	}
	d, err := a.Store.Deck.ByKind(ctx, a.ChannelID, model.DeckProgram)
	if err != nil {
		t.Fatalf("no pude leer el deck: %v", err)
	}
	item := model.PlanItem{
		ChannelID:    a.ChannelID,
		DeckID:       d.ID,
		BroadcastDay: ch.BroadcastDay(arranca),
		PlannedAt:    arranca,
		PlannedMs:    dura.Milliseconds(),
		Origin:       model.OriginLiveSource,
		State:        model.Planned,
		Fijado:       true,
	}
	if err := a.Store.Plan.Insert(ctx, &item); err != nil {
		t.Fatalf("no pude guardar el bloque en vivo: %v", err)
	}
	return item
}
