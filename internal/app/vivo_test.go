package app

import (
	"context"
	"testing"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// conSenal deja una señal en vivo y un bloque del plan que la usa.
func conSenal(t *testing.T, a *App, nombre, tipo, direccion string, gracia int, arranca time.Time, dura time.Duration) (model.LiveSource, model.PlanItem) {
	t.Helper()
	ctx := context.Background()
	arranca = arranca.Truncate(time.Millisecond)

	fuente := model.LiveSource{
		ChannelID: a.ChannelID, Name: nombre, Kind: tipo,
		ListenPoint: direccion, DelayMs: 7000, GraceSeconds: gracia,
	}
	if err := a.Store.Live.Upsert(ctx, &fuente); err != nil {
		t.Fatalf("no pude guardar la señal: %v", err)
	}

	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	d, err := a.Store.Deck.ByKind(ctx, a.ChannelID, model.DeckProgram)
	if err != nil {
		t.Fatal(err)
	}
	id := fuente.ID
	item := model.PlanItem{
		ChannelID: a.ChannelID, DeckID: d.ID,
		BroadcastDay: ch.BroadcastDay(arranca), PlannedAt: arranca,
		PlannedMs: dura.Milliseconds(), Origin: model.OriginLiveSource,
		LiveSourceID: &id, State: model.Planned, Fijado: true,
	}
	if err := a.Store.Plan.Insert(ctx, &item); err != nil {
		t.Fatalf("no pude guardar el bloque en vivo: %v", err)
	}
	return fuente, item
}

// F2-116 — un bloque en vivo sale como una dirección, no como un archivo, y el
// decodificador lo sabe. Antes de hoy esto devolvía un error a propósito: «las
// fuentes en vivo todavía no están construidas».
func TestUnBloqueEnVivoSaleComoDireccion(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	conCartel(t, a)

	ahora := a.Now()
	_, item := conSenal(t, a, "El HLS de la máquina", model.FuenteURL,
		"http://127.0.0.1:8080/hls/for_tv/index.m3u8", 30, ahora.Add(-time.Minute), time.Hour)

	f := a.nuevaFuenteDelPlan(ctx, engine.CAtv)
	clip, _, err := f.Next(ahora)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	if !clip.EnVivo {
		t.Fatal("el clip de una señal en vivo tiene que decir que lo es: si no, el decodificador le pregunta la duración y se cuelga")
	}
	if clip.Path != "http://127.0.0.1:8080/hls/for_tv/index.m3u8" {
		t.Fatalf("salió %q y tenía que salir la dirección de la señal", clip.Path)
	}
	if clip.SeekMs != 0 {
		t.Fatalf("se pidió entrar por el minuto %d de algo que está pasando ahora", clip.SeekMs/1000)
	}
	if clip.Ref != item.ID {
		t.Fatalf("el clip apunta al bloque %d y tenía que apuntar al %d", clip.Ref, item.ID)
	}
}

// F2-19 y F2-116 — una señal que no llega **no es un archivo roto**. Mientras
// quede gracia sale relleno y se reintenta; el bloque NO se marca como
// fallido, porque una señal casi siempre vuelve.
func TestLaSenalQueNoLlegaNoTumbaElBloque(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	cartel := conCartel(t, a)

	ahora := a.Now()
	// Una tarjeta de captura: el motor la rechaza al armar el clip, así que
	// la ausencia es determinista y no depende de la red de quien corra esto.
	_, item := conSenal(t, a, "La que no está", model.FuenteCaptura,
		"Tarjeta que no existe", 30, ahora.Add(-time.Minute), time.Hour)

	f := a.nuevaFuenteDelPlan(ctx, engine.CAtv)
	clip, hasta, err := f.Next(ahora)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	if clip.Path != cartel {
		t.Fatalf("con la señal ausente tenía que salir el relleno y salió %q", clip.Path)
	}
	// Y se vuelve a preguntar pronto, no dentro de una hora.
	if espera := hasta.Sub(ahora); espera > 5*time.Second {
		t.Fatalf("se reintenta dentro de %s: una señal que vuelve en dos segundos se habría perdido", espera)
	}

	// Lo importante: el bloque sigue vivo en la parrilla.
	vuelto, err := a.PlanItem(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if vuelto.State == model.Failed {
		t.Fatal("la señal faltó un momento y el bloque se dio por perdido: casi siempre vuelve, y entonces ya no hay bloque al que volver")
	}
}

// Pasada la gracia sí se da por perdida, con su incidente. Si no, un bloque de
// dos horas estaría reintentando dos horas sin que nadie se entere.
func TestPasadaLaGraciaLaSenalSeDaPorPerdida(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	conCartel(t, a)

	ahora := a.Now()
	fuente, item := conSenal(t, a, "La que no vuelve", model.FuenteCaptura,
		"Tarjeta que no existe", 30, ahora.Add(-time.Minute), time.Hour)
	_ = fuente

	f := a.nuevaFuenteDelPlan(ctx, engine.CAtv)

	// Primera ausencia: se aguanta.
	if _, _, err := f.Next(ahora); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.PlanItem(ctx, item.ID); v.State == model.Failed {
		t.Fatal("se dio por perdida a la primera, sin darle la gracia que la fuente pide")
	}

	// Y pasada la gracia, se suelta.
	despues := ahora.Add(31 * time.Second)
	if _, _, err := f.Next(despues); err != nil {
		t.Fatal(err)
	}
	v, err := a.PlanItem(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.State != model.Failed {
		t.Fatalf("pasada la gracia el bloque quedó en %q: nadie se enteraría de que la señal no volvió", v.State)
	}
	if n := cuentaIncidentes(t, a, model.IncVivoAusente.String()); n != 1 {
		t.Fatalf("quedaron %d incidentes `vivo_ausente` y tenía que quedar uno", n)
	}
}
