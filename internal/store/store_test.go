package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"antena787/internal/model"
)

// abrir crea una base en una carpeta temporal. Sin rutas fijas: esto tiene
// que correr igual en Linux, Mac y Windows.
func abrir(t *testing.T) (*Store, string) {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "antena.db")
	s, err := Open(ruta)
	if err != nil {
		t.Fatalf("no abrió la base: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, ruta
}

func TestOpenMigraYReabre(t *testing.T) {
	ctx := context.Background()
	ruta := filepath.Join(t.TempDir(), "antena.db")

	s, err := Open(ruta)
	if err != nil {
		t.Fatalf("no abrió la base: %v", err)
	}
	v, err := s.Version(ctx)
	if err != nil {
		t.Fatalf("no leyó la versión: %v", err)
	}
	if v != SchemaVersion() {
		t.Fatalf("versión %d, se esperaba %d", v, SchemaVersion())
	}
	if err := s.Close(); err != nil {
		t.Fatalf("no cerró: %v", err)
	}

	// Reabrir no debe volver a crear nada ni fallar.
	s2, err := Open(ruta)
	if err != nil {
		t.Fatalf("no reabrió la base: %v", err)
	}
	defer func() { _ = s2.Close() }()

	if err := s2.CheckIntegrity(ctx); err != nil {
		t.Fatalf("la base no pasó la verificación: %v", err)
	}
	canales, err := s2.Channel.List(ctx)
	if err != nil {
		t.Fatalf("no listó los canales: %v", err)
	}
	if len(canales) != 1 {
		t.Fatalf("hay %d canales, se esperaba 1", len(canales))
	}
}

func TestSiembraIdempotente(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	c, err := s.Channel.Get(ctx, DefaultChannelID)
	if err != nil {
		t.Fatalf("no leyó el canal: %v", err)
	}
	if c.Name != "Mi canal" || c.Kind != model.ChannelTV {
		t.Fatalf("canal sembrado raro: %+v", c)
	}
	if c.TimeZone != "America/Puerto_Rico" || c.BroadcastDayAt != model.Minutes(360) {
		t.Fatalf("la zona o la hora de inicio no son las esperadas: %+v", c)
	}
	if c.MaxLoadPerHour != 12 {
		t.Fatalf("carga máxima %d, se esperaba 12", c.MaxLoadPerHour)
	}

	for _, k := range []model.DeckKind{model.DeckManual, model.DeckCommercial, model.DeckProgram, model.DeckFiller} {
		d, err := s.Deck.ByKind(ctx, DefaultChannelID, k)
		if err != nil {
			t.Fatalf("falta el deck %s: %v", k, err)
		}
		if d.Priority != model.DeckPriority[k] {
			t.Fatalf("deck %s con prioridad %d, se esperaba %d", k, d.Priority, model.DeckPriority[k])
		}
	}

	// Sembrar otra vez no duplica.
	if err := s.seed(ctx); err != nil {
		t.Fatalf("la segunda siembra falló: %v", err)
	}
	var canales, decks int
	if err := s.DB().QueryRowContext(ctx, `SELECT count(*) FROM channel`).Scan(&canales); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRowContext(ctx, `SELECT count(*) FROM deck`).Scan(&decks); err != nil {
		t.Fatal(err)
	}
	if canales != 1 || decks != 4 {
		t.Fatalf("después de sembrar dos veces hay %d canales y %d decks", canales, decks)
	}
}

func TestReglaConFechasCruzadasFalla(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	titulo := &model.Title{Name: "Hellsing", Kind: model.TitleSeries}
	if err := s.Title.Insert(ctx, titulo); err != nil {
		t.Fatalf("no guardó el título: %v", err)
	}

	regla := &model.ScheduleRule{
		ChannelID: DefaultChannelID,
		Kind:      model.RuleNormal,
		TitleID:   &titulo.ID,
		Days:      "LMMJV__",
		At:        model.Minutes(20 * 60),
		SlotMs:    30 * 60 * 1000,
		From:      "2026-10-01",
		To:        "2026-09-01", // al revés a propósito
		Active:    true,
	}
	err := s.Rule.Insert(ctx, regla)
	if !errors.Is(err, ErrDates) {
		t.Fatalf("se esperaba ErrDates, salió: %v", err)
	}

	// Y con las fechas en orden entra sin protestar.
	regla.To = "2026-12-31"
	if err := s.Rule.Insert(ctx, regla); err != nil {
		t.Fatalf("la regla buena no entró: %v", err)
	}
	if regla.ID == 0 {
		t.Fatal("la regla entró sin id")
	}

	// Y también se rechaza al actualizar.
	regla.To = "2020-01-01"
	if err := s.Rule.Update(ctx, regla); !errors.Is(err, ErrDates) {
		t.Fatalf("al actualizar se esperaba ErrDates, salió: %v", err)
	}
}

func TestReglasActivasDeUnDia(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	// 2026-09-07 es lunes.
	lunes := model.Day("2026-09-07")
	if lunes.Weekday() != 0 {
		t.Fatalf("el día de prueba no es lunes: %d", lunes.Weekday())
	}

	deLunes := &model.ScheduleRule{
		ChannelID: DefaultChannelID, Days: "L______", At: 600, SlotMs: 1800000,
		From: "2026-09-01", To: "2026-09-30", Active: true,
	}
	deMartes := &model.ScheduleRule{
		ChannelID: DefaultChannelID, Days: "_M_____", At: 600, SlotMs: 1800000,
		From: "2026-09-01", To: "2026-09-30", Active: true,
	}
	vencida := &model.ScheduleRule{
		ChannelID: DefaultChannelID, Days: "L______", At: 700, SlotMs: 1800000,
		From: "2026-01-01", To: "2026-02-01", Active: true,
	}
	apagada := &model.ScheduleRule{
		ChannelID: DefaultChannelID, Days: "L______", At: 800, SlotMs: 1800000,
		From: "2026-09-01", To: "2026-09-30", Active: false,
	}
	for _, r := range []*model.ScheduleRule{deLunes, deMartes, vencida, apagada} {
		if err := s.Rule.Insert(ctx, r); err != nil {
			t.Fatalf("no guardó la regla: %v", err)
		}
	}

	activas, err := s.Rule.ListActive(ctx, DefaultChannelID, lunes)
	if err != nil {
		t.Fatalf("no listó las reglas activas: %v", err)
	}
	if len(activas) != 1 || activas[0].ID != deLunes.ID {
		t.Fatalf("las reglas activas del lunes salieron mal: %+v", activas)
	}

	todas, err := s.Rule.ListAll(ctx, DefaultChannelID)
	if err != nil {
		t.Fatalf("no listó todas las reglas: %v", err)
	}
	if len(todas) != 4 {
		t.Fatalf("hay %d reglas, se esperaban 4", len(todas))
	}
}

// item arma un plan_item mínimo para las pruebas.
func item(deckID int64, inicio time.Time, dur time.Duration) *model.PlanItem {
	return &model.PlanItem{
		ChannelID:    DefaultChannelID,
		DeckID:       deckID,
		BroadcastDay: model.Day("2026-09-07"),
		PlannedAt:    inicio,
		PlannedMs:    dur.Milliseconds(),
		Origin:       "asset",
		State:        model.Planned,
	}
}

func TestPlanItemSolapadoYDecks(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	programa, err := s.Deck.ByKind(ctx, DefaultChannelID, model.DeckProgram)
	if err != nil {
		t.Fatal(err)
	}
	comercial, err := s.Deck.ByKind(ctx, DefaultChannelID, model.DeckCommercial)
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 9, 7, 20, 0, 0, 0, time.UTC)
	primero := item(programa.ID, base, 30*time.Minute)
	if err := s.Plan.Insert(ctx, primero); err != nil {
		t.Fatalf("el primer ítem no entró: %v", err)
	}

	// Mismo deck, se pisan: el trigger del esquema lo rechaza.
	encima := item(programa.ID, base.Add(10*time.Minute), 30*time.Minute)
	if err := s.Plan.Insert(ctx, encima); !errors.Is(err, ErrOverlap) {
		t.Fatalf("se esperaba ErrOverlap, salió: %v", err)
	}

	// Otro deck, a la misma hora: pasa.
	enOtroDeck := item(comercial.ID, base.Add(10*time.Minute), 2*time.Minute)
	if err := s.Plan.Insert(ctx, enOtroDeck); err != nil {
		t.Fatalf("el ítem del otro deck no entró: %v", err)
	}

	// Pegado justo al final del primero, sin pisarlo: pasa.
	seguido := item(programa.ID, base.Add(30*time.Minute), 30*time.Minute)
	if err := s.Plan.Insert(ctx, seguido); err != nil {
		t.Fatalf("el ítem seguido no entró: %v", err)
	}

	// Y el lote entero se cae si uno solo se pisa: no queda medio plan.
	lote := []*model.PlanItem{
		item(programa.ID, base.Add(2*time.Hour), 10*time.Minute),
		item(programa.ID, base.Add(2*time.Hour+5*time.Minute), 10*time.Minute),
	}
	if err := s.Plan.InsertBatch(ctx, lote); !errors.Is(err, ErrOverlap) {
		t.Fatalf("se esperaba ErrOverlap en el lote, salió: %v", err)
	}
	dia, err := s.Plan.ListDay(ctx, DefaultChannelID, model.Day("2026-09-07"))
	if err != nil {
		t.Fatal(err)
	}
	if len(dia) != 3 {
		t.Fatalf("quedaron %d ítems, se esperaban 3: el lote roto no debió entrar", len(dia))
	}
}

func TestPlanItemDentroDeUnVivo(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	programa, err := s.Deck.ByKind(ctx, DefaultChannelID, model.DeckProgram)
	if err != nil {
		t.Fatal(err)
	}
	comercial, err := s.Deck.ByKind(ctx, DefaultChannelID, model.DeckCommercial)
	if err != nil {
		t.Fatal(err)
	}

	vivo := &model.LiveSource{
		ChannelID: DefaultChannelID, Name: "Cámara del estudio", Kind: "srt",
		ListenPoint: "srt://0.0.0.0:9000", BreakClock: []int{0, 15, 30, 45},
		DelayMs: 7000, GraceSeconds: 30,
	}
	if err := s.Live.Upsert(ctx, vivo); err != nil {
		t.Fatalf("no guardó la fuente en vivo: %v", err)
	}

	base := time.Date(2026, 9, 7, 19, 0, 0, 0, time.UTC)
	padre := item(programa.ID, base, time.Hour)
	padre.Origin = "live_source"
	padre.LiveSourceID = &vivo.ID
	if err := s.Plan.Insert(ctx, padre); err != nil {
		t.Fatalf("el vivo no entró: %v", err)
	}

	// El spot va DENTRO del vivo: se solapa con él, pero vive en el deck
	// comercial, así que el esquema lo acepta.
	spot := item(comercial.ID, base.Add(15*time.Minute), 30*time.Second)
	spot.Inside = &padre.ID
	if err := s.Plan.Insert(ctx, spot); err != nil {
		t.Fatalf("el spot dentro del vivo no entró: %v", err)
	}

	leidos, err := s.Plan.ListRange(ctx, DefaultChannelID, base, base.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(leidos) != 2 {
		t.Fatalf("se leyeron %d ítems, se esperaban 2", len(leidos))
	}
	var encontrado bool
	for _, p := range leidos {
		if p.ID == spot.ID {
			if p.Inside == nil || *p.Inside != padre.ID {
				t.Fatalf("el spot perdió su dentro_de: %+v", p)
			}
			encontrado = true
		}
	}
	if !encontrado {
		t.Fatal("el spot no salió en el rango")
	}

	// Y la fuente en vivo vuelve con su reloj de cortes intacto.
	vivos, err := s.Live.List(ctx, DefaultChannelID)
	if err != nil {
		t.Fatal(err)
	}
	if len(vivos) != 1 || len(vivos[0].BreakClock) != 4 || vivos[0].BreakClock[3] != 45 {
		t.Fatalf("el reloj de cortes se perdió: %+v", vivos)
	}
}

func TestResetCuedToPlannedYAsRun(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	programa, err := s.Deck.ByKind(ctx, DefaultChannelID, model.DeckProgram)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 7, 6, 0, 0, 0, time.UTC)

	cargado := item(programa.ID, base, 10*time.Minute)
	emitido := item(programa.ID, base.Add(10*time.Minute), 10*time.Minute)
	futuro := item(programa.ID, base.Add(20*time.Minute), 10*time.Minute)
	for _, p := range []*model.PlanItem{cargado, emitido, futuro} {
		if err := s.Plan.Insert(ctx, p); err != nil {
			t.Fatalf("no entró el ítem: %v", err)
		}
	}

	if err := s.Plan.SetState(ctx, cargado.ID, model.Cued); err != nil {
		t.Fatal(err)
	}
	real := base.Add(10*time.Minute + 3*time.Second)
	if err := s.Plan.MarkAired(ctx, emitido.ID, real, (10 * time.Minute).Milliseconds(), true); err != nil {
		t.Fatal(err)
	}

	n, err := s.Plan.ResetCuedToPlanned(ctx, DefaultChannelID)
	if err != nil {
		t.Fatalf("no devolvió los cargados: %v", err)
	}
	if n != 1 {
		t.Fatalf("devolvió %d ítems, se esperaba 1", n)
	}

	dia, err := s.Plan.ListDay(ctx, DefaultChannelID, model.Day("2026-09-07"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range dia {
		switch p.ID {
		case cargado.ID:
			if p.State != model.Planned || p.CuedAt != nil {
				t.Fatalf("el cargado no volvió a planeado: %+v", p)
			}
		case emitido.ID:
			if p.State != model.Aired || p.ActualAt == nil || !p.ActualAt.Equal(real) || !p.Partial {
				t.Fatalf("el as-run no quedó bien: %+v", p)
			}
			if !p.PlannedAt.Equal(base.Add(10 * time.Minute)) {
				t.Fatalf("lo planeado se movió: %+v", p)
			}
		}
	}

	// DeleteFuturePlanned solo se lleva lo planeado de aquí en adelante.
	if err := s.Plan.SetState(ctx, cargado.ID, model.Cued); err != nil {
		t.Fatal(err)
	}
	borrados, err := s.Plan.DeleteFuturePlanned(ctx, DefaultChannelID, base)
	if err != nil {
		t.Fatal(err)
	}
	if borrados != 1 {
		t.Fatalf("borró %d ítems, se esperaba 1 (solo el futuro planeado)", borrados)
	}
	quedan, err := s.Plan.ListDay(ctx, DefaultChannelID, model.Day("2026-09-07"))
	if err != nil {
		t.Fatal(err)
	}
	if len(quedan) != 2 {
		t.Fatalf("quedaron %d ítems, se esperaban 2: cued y aired no se borran", len(quedan))
	}
}

func TestNextAfterDaLaVuelta(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	serie := &model.Title{Name: "Serie de prueba", Kind: model.TitleSeries}
	if err := s.Title.Insert(ctx, serie); err != nil {
		t.Fatal(err)
	}
	var eps []*model.Episode
	for i := 1; i <= 3; i++ {
		e := &model.Episode{TitleID: serie.ID, Season: 1, Number: i}
		if err := s.Episode.Insert(ctx, e); err != nil {
			t.Fatal(err)
		}
		eps = append(eps, e)
	}
	// Uno de la segunda temporada, para que el orden no sea el de los ids.
	segunda := &model.Episode{TitleID: serie.ID, Season: 2, Number: 1}
	if err := s.Episode.Insert(ctx, segunda); err != nil {
		t.Fatal(err)
	}

	sinContador, err := s.Episode.NextAfter(ctx, serie.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sinContador.ID != eps[0].ID {
		t.Fatalf("sin contador debía dar el primero, dio %+v", sinContador)
	}

	siguiente, err := s.Episode.NextAfter(ctx, serie.ID, &eps[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if siguiente.ID != eps[1].ID {
		t.Fatalf("después del 1x01 debía venir el 1x02, vino %+v", siguiente)
	}

	cruzaTemporada, err := s.Episode.NextAfter(ctx, serie.ID, &eps[2].ID)
	if err != nil {
		t.Fatal(err)
	}
	if cruzaTemporada.ID != segunda.ID {
		t.Fatalf("después del 1x03 debía venir el 2x01, vino %+v", cruzaTemporada)
	}

	// El final de la serie da la vuelta al principio.
	daLaVuelta, err := s.Episode.NextAfter(ctx, serie.ID, &segunda.ID)
	if err != nil {
		t.Fatal(err)
	}
	if daLaVuelta.ID != eps[0].ID {
		t.Fatalf("al terminar debía volver al primero, dio %+v", daLaVuelta)
	}

	// Una serie sin episodios no revienta: dice que no hay.
	vacia := &model.Title{Name: "Sin episodios", Kind: model.TitleSeries}
	if err := s.Title.Insert(ctx, vacia); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Episode.NextAfter(ctx, vacia.ID, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("se esperaba ErrNotFound, salió: %v", err)
	}

	// Y el contador de la regla se guarda donde toca.
	regla := &model.ScheduleRule{
		ChannelID: DefaultChannelID, TitleID: &serie.ID, Days: "LMMJV__", At: 1200,
		SlotMs: 1800000, From: "2026-09-01", To: "2026-12-31", Active: true,
	}
	if err := s.Rule.Insert(ctx, regla); err != nil {
		t.Fatal(err)
	}
	if err := s.Rule.SetLastEpisode(ctx, regla.ID, eps[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Rule.SetLastNotice(ctx, regla.ID, "30"); err != nil {
		t.Fatal(err)
	}
	leida, err := s.Rule.Get(ctx, regla.ID)
	if err != nil {
		t.Fatal(err)
	}
	if leida.LastEpisodeAired == nil || *leida.LastEpisodeAired != eps[1].ID || leida.LastNoticeSent != "30" {
		t.Fatalf("el contador o el aviso no se guardaron: %+v", leida)
	}
}

func TestAuditChainDetectaUnaFilaEditada(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	for i := 0; i < 5; i++ {
		e := &model.AuditEntry{
			Entity: "schedule_rule", Field: "hora",
			Before: "19:00", After: "20:00", Author: "rolando", Origin: "humano",
		}
		if err := s.Audit.Append(ctx, e); err != nil {
			t.Fatalf("no anotó en la bitácora: %v", err)
		}
		if e.Hash == "" {
			t.Fatal("la entrada quedó sin hash")
		}
	}

	rota, err := s.Audit.Verify(ctx)
	if err != nil {
		t.Fatalf("la verificación falló: %v", err)
	}
	if rota != nil {
		t.Fatalf("la cadena estaba rota sin que nadie la tocara: %+v", rota)
	}

	// Alguien edita una fila por detrás, con SQL, como se haría de verdad.
	if _, err := s.DB().ExecContext(ctx,
		`UPDATE audit_log SET valor_nuevo = '23:00' WHERE id = 3`); err != nil {
		t.Fatal(err)
	}

	rota, err = s.Audit.Verify(ctx)
	if err != nil {
		t.Fatalf("la verificación falló: %v", err)
	}
	if rota == nil {
		t.Fatal("la bitácora editada pasó la verificación")
	}
	if rota.ID != 3 {
		t.Fatalf("señaló la entrada %d, se esperaba la 3", rota.ID)
	}
}

func TestBackupSeAbre(t *testing.T) {
	ctx := context.Background()
	s, ruta := abrir(t)

	titulo := &model.Title{Name: "Algo que respaldar", Kind: model.TitleMovie}
	if err := s.Title.Insert(ctx, titulo); err != nil {
		t.Fatal(err)
	}

	destino := filepath.Join(filepath.Dir(ruta), "respaldo-de-prueba.db")
	if err := s.Backup(ctx, destino); err != nil {
		t.Fatalf("no respaldó: %v", err)
	}
	if _, err := os.Stat(destino); err != nil {
		t.Fatalf("el respaldo no existe: %v", err)
	}
	// No se pisa un respaldo que ya está.
	if err := s.Backup(ctx, destino); err == nil {
		t.Fatal("el segundo respaldo sobre el mismo archivo debió fallar")
	}

	copia, err := Open(destino)
	if err != nil {
		t.Fatalf("el respaldo no abre: %v", err)
	}
	defer func() { _ = copia.Close() }()

	leido, err := copia.Title.FindByName(ctx, "algo que respaldar")
	if err != nil {
		t.Fatalf("el respaldo no trae los datos: %v", err)
	}
	if leido.ID != titulo.ID {
		t.Fatalf("el título del respaldo no cuadra: %+v", leido)
	}
}

func TestRespaldoAntesDeMigrar(t *testing.T) {
	ctx := context.Background()
	s, ruta := abrir(t)

	dst, err := s.backupBeforeMigration(ctx, 1)
	if err != nil {
		t.Fatalf("no respaldó antes de migrar: %v", err)
	}
	esperado := ruta + ".respaldo-1-" + time.Now().Format("2006-01-02") + ".db"
	if dst != esperado {
		t.Fatalf("el respaldo se llama %q, se esperaba %q", dst, esperado)
	}
	if LatestBackup(ruta) != dst {
		t.Fatalf("LatestBackup dio %q, se esperaba %q", LatestBackup(ruta), dst)
	}

	// Un segundo respaldo el mismo día no pisa al primero.
	otro, err := s.backupBeforeMigration(ctx, 1)
	if err != nil {
		t.Fatalf("el segundo respaldo falló: %v", err)
	}
	if otro == dst {
		t.Fatal("el segundo respaldo pisó al primero")
	}
}

func TestBaseRotaAvisaConSuRespaldo(t *testing.T) {
	ctx := context.Background()
	s, ruta := abrir(t)
	respaldo, err := s.backupBeforeMigration(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Se destroza la cabecera del archivo, como haría un disco malo.
	f, err := os.OpenFile(ruta, os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("esto no es una base de datos, para nada"), 0); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = Open(ruta)
	if err == nil {
		t.Fatal("abrió una base rota como si nada")
	}
	var rota *ErrCorrupt
	if !errors.As(err, &rota) {
		t.Fatalf("se esperaba *ErrCorrupt, salió: %v", err)
	}
	if rota.LastBackup != respaldo {
		t.Fatalf("el error apunta a %q, se esperaba %q", rota.LastBackup, respaldo)
	}
}

func TestMediaAssetIdaYVuelta(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	ruta := filepath.Join("biblioteca", "programa.mp4")
	lufs := -23.5
	a := &model.MediaAsset{
		Path: ruta, Hash: "abc123", Codec: "h264", Resolution: "1280x720",
		FPS: "59.94", AudioChannels: 2, DurationMs: 1_800_000, LUFS: &lufs,
		BreakMarksMs: []int64{600_000, 1_200_000},
		State:        model.AssetIngesting,
	}
	if err := s.Media.Insert(ctx, a); err != nil {
		t.Fatalf("no guardó el archivo: %v", err)
	}
	if a.ID == 0 || a.CreatedAt.IsZero() {
		t.Fatalf("el archivo entró a medias: %+v", a)
	}

	leido, err := s.Media.GetByPath(ctx, ruta)
	if err != nil {
		t.Fatalf("no lo encontró por ruta: %v", err)
	}
	if len(leido.BreakMarksMs) != 2 || leido.BreakMarksMs[1] != 1_200_000 {
		t.Fatalf("las marcas de corte se perdieron: %+v", leido.BreakMarksMs)
	}
	if leido.LUFS == nil || *leido.LUFS != lufs {
		t.Fatalf("el lufs se perdió: %+v", leido.LUFS)
	}

	if err := s.Media.SetState(ctx, a.ID, model.AssetQuarantine, "el audio está mudo"); err != nil {
		t.Fatal(err)
	}
	enCuarentena, err := s.Media.List(ctx, model.AssetQuarantine)
	if err != nil {
		t.Fatal(err)
	}
	if len(enCuarentena) != 1 || enCuarentena[0].PlainReason != "el audio está mudo" {
		t.Fatalf("la cuarentena no quedó bien: %+v", enCuarentena)
	}
	listos, err := s.Media.ListReady(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listos) != 0 {
		t.Fatalf("no debía haber nada listo: %+v", listos)
	}

	if _, err := s.Media.Get(ctx, 9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("se esperaba ErrNotFound, salió: %v", err)
	}
}

func TestAjustesYClaveDeEstacion(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	if err := s.Settings.Set(ctx, "guia.publicar", "no"); err != nil {
		t.Fatal(err)
	}
	if err := s.Settings.Set(ctx, "guia.publicar", "si"); err != nil {
		t.Fatal(err)
	}
	v, err := s.Settings.Get(ctx, "guia.publicar")
	if err != nil || v != "si" {
		t.Fatalf("el ajuste no se guardó: %q, %v", v, err)
	}
	if _, err := s.Settings.Get(ctx, "no.existe"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("se esperaba ErrNotFound, salió: %v", err)
	}

	tiene, err := s.Settings.HasPIN(ctx)
	if err != nil || tiene {
		t.Fatalf("no debía haber clave todavía: %v, %v", tiene, err)
	}
	ok, err := s.Settings.CheckPIN(ctx, "1234")
	if err != nil || ok {
		t.Fatalf("sin clave puesta nada valida: %v, %v", ok, err)
	}

	if err := s.Settings.SetPIN(ctx, "clave de la estación"); err != nil {
		t.Fatal(err)
	}
	ok, err = s.Settings.CheckPIN(ctx, "clave de la estación")
	if err != nil || !ok {
		t.Fatalf("la clave buena no pasó: %v, %v", ok, err)
	}
	ok, err = s.Settings.CheckPIN(ctx, "otra cosa")
	if err != nil || ok {
		t.Fatalf("una clave mala pasó: %v, %v", ok, err)
	}
	if err := s.Settings.SetPIN(ctx, "   "); err == nil {
		t.Fatal("aceptó una clave vacía")
	}

	todos, err := s.Settings.GetAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, hay := todos[KeyPINHash]; hay {
		t.Fatal("GetAll enseñó el hash de la clave")
	}
	if _, hay := todos[KeyPINSalt]; hay {
		t.Fatal("GetAll enseñó la sal de la clave")
	}
	if todos["guia.publicar"] != "si" {
		t.Fatalf("GetAll no trae los ajustes normales: %+v", todos)
	}
}

func TestSalidasIncidentesYRelleno(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	salida := &model.Output{
		ChannelID: DefaultChannelID, Name: "Al transmisor", Driver: "udp-ts",
		TargetLoudness: -24,
	}
	if err := s.Output.Upsert(ctx, salida); err != nil {
		t.Fatal(err)
	}
	salida.ConnectionState = "conectado"
	if err := s.Output.Upsert(ctx, salida); err != nil {
		t.Fatal(err)
	}
	salidas, err := s.Output.List(ctx, DefaultChannelID)
	if err != nil {
		t.Fatal(err)
	}
	if len(salidas) != 1 || salidas[0].ConnectionState != "conectado" || salidas[0].Params != "{}" {
		t.Fatalf("la salida no quedó bien: %+v", salidas)
	}

	a := &model.MediaAsset{Path: "relleno/promo.mp4", DurationMs: 30000, State: model.AssetReady}
	if err := s.Media.Insert(ctx, a); err != nil {
		t.Fatal(err)
	}
	f := &model.FillerAsset{MediaAssetID: a.ID, DurationMs: 30000}
	if err := s.Filler.Insert(ctx, f); err != nil {
		t.Fatal(err)
	}
	rellenos, err := s.Filler.List(ctx, DefaultChannelID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rellenos) != 1 || rellenos[0].Kind != "promo" || rellenos[0].ChannelID != nil {
		t.Fatalf("el relleno genérico no salió: %+v", rellenos)
	}

	inicio := time.Date(2026, 9, 7, 3, 14, 0, 0, time.UTC)
	inc := &model.Incident{
		ChannelID: DefaultChannelID, Kind: "vivo_ausente", Start: inicio,
		Detail: "el estudio no llegó",
	}
	if err := s.Incident.Insert(ctx, inc); err != nil {
		t.Fatal(err)
	}
	abiertos, err := s.Incident.List(ctx, DefaultChannelID, inicio.Add(-time.Hour), inicio.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(abiertos) != 1 || abiertos[0].End != nil {
		t.Fatalf("el incidente abierto no salió: %+v", abiertos)
	}
	if err := s.Incident.Close(ctx, inc.ID, inicio.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	cerrados, err := s.Incident.List(ctx, DefaultChannelID, inicio.Add(-time.Hour), inicio.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(cerrados) != 1 || cerrados[0].End == nil {
		t.Fatalf("el incidente no se cerró: %+v", cerrados)
	}
}

func TestCanalSeActualiza(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	c, err := s.Channel.Get(ctx, DefaultChannelID)
	if err != nil {
		t.Fatal(err)
	}
	c.Name = "CAtv"
	c.CallSign = "WCAT-LD"
	c.Mode = "aire"
	c.BroadcastDayAt = model.Minutes(5 * 60)
	if err := s.Channel.Update(ctx, c); err != nil {
		t.Fatal(err)
	}
	leido, err := s.Channel.Get(ctx, DefaultChannelID)
	if err != nil {
		t.Fatal(err)
	}
	if leido.Name != "CAtv" || leido.CallSign != "WCAT-LD" || leido.Mode != "aire" ||
		leido.BroadcastDayAt != model.Minutes(300) {
		t.Fatalf("el canal no se guardó: %+v", leido)
	}

	fantasma := model.Channel{ID: 99, Name: "No existe", Kind: model.ChannelTV, TimeZone: "UTC"}
	if err := s.Channel.Update(ctx, fantasma); !errors.Is(err, ErrNotFound) {
		t.Fatalf("se esperaba ErrNotFound, salió: %v", err)
	}
}

// esquemaDe devuelve el esquema entero de la base —tablas, índices y
// triggers— para poder compararlo entre dos bases.
func esquemaDe(t *testing.T, s *Store) string {
	t.Helper()
	rows, err := s.DB().QueryContext(context.Background(),
		`SELECT type, name, ifnull(sql, '') FROM sqlite_master
		 WHERE name NOT LIKE 'sqlite_%' ORDER BY type, name`)
	if err != nil {
		t.Fatalf("no se pudo leer el esquema: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var todo string
	for rows.Next() {
		var tipo, nombre, sql string
		if err := rows.Scan(&tipo, &nombre, &sql); err != nil {
			t.Fatal(err)
		}
		todo += tipo + " " + nombre + "\n" + sql + "\n\n"
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return todo
}

// Una base recién creada y una base vieja migrada tienen que terminar
// idénticas: schema.sql es el punto de partida y los escalones de migrations
// se aplican a las dos.
func TestBaseNuevaYBaseMigradaQuedanIguales(t *testing.T) {
	ctx := context.Background()

	nueva, _ := abrir(t)
	if v, err := nueva.Version(ctx); err != nil || v != SchemaVersion() {
		t.Fatalf("la base nueva quedó en la versión %d (%v), se esperaba %d", v, err, SchemaVersion())
	}

	// Una base parada en cada versión publicada tiene que llegar al mismo
	// sitio: la 1 (solo schema.sql) y la 2 (schema.sql más migracion2).
	for _, caso := range []struct {
		version int
		pasos   []string
	}{
		{1, []string{schemaSQL}},
		{2, []string{schemaSQL, migracion2}},
	} {
		ruta := filepath.Join(t.TempDir(), "vieja.db")
		db, err := openDB(ruta)
		if err != nil {
			t.Fatalf("no se pudo crear la base vieja: %v", err)
		}
		for _, paso := range caso.pasos {
			if _, err := db.ExecContext(ctx, paso); err != nil {
				t.Fatalf("no se pudo armar una base de la versión %d: %v", caso.version, err)
			}
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", caso.version)); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}

		vieja, err := Open(ruta)
		if err != nil {
			t.Fatalf("la base de la versión %d no migró: %v", caso.version, err)
		}
		t.Cleanup(func() { _ = vieja.Close() })

		if v, err := vieja.Version(ctx); err != nil || v != SchemaVersion() {
			t.Fatalf("la base de la versión %d quedó en la %d (%v), se esperaba %d",
				caso.version, v, err, SchemaVersion())
		}
		if a, b := esquemaDe(t, nueva), esquemaDe(t, vieja); a != b {
			t.Fatalf("una base nueva y una migrada desde la versión %d no quedaron iguales\n--- nueva ---\n%s\n--- migrada ---\n%s",
				caso.version, a, b)
		}
		// Y la migración deja respaldo de la base que ya existía.
		if LatestBackup(ruta) == "" {
			t.Fatalf("migrar una base de la versión %d tiene que dejar un respaldo", caso.version)
		}
	}
}

// F1-22: el fundido de salida del clip recortado viaja al plan y vuelve.
func TestFundidoDeSalidaSeGuarda(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	programa, err := s.Deck.ByKind(ctx, DefaultChannelID, model.DeckProgram)
	if err != nil {
		t.Fatal(err)
	}
	inicio := time.Date(2026, 9, 7, 20, 0, 0, 0, time.UTC)
	item := model.PlanItem{
		ChannelID: DefaultChannelID, DeckID: programa.ID, BroadcastDay: "2026-09-07",
		PlannedAt: inicio, PlannedMs: 177000, Origin: "relleno",
		LocalClock: "16:00", FadeOutMs: 1000,
	}
	if err := s.Plan.Insert(ctx, &item); err != nil {
		t.Fatalf("no entró el ítem: %v", err)
	}
	leidos, err := s.Plan.ListDay(ctx, DefaultChannelID, "2026-09-07")
	if err != nil {
		t.Fatal(err)
	}
	if len(leidos) != 1 || leidos[0].FadeOutMs != 1000 || leidos[0].Fijado {
		t.Fatalf("el fundido de salida no se guardó: %+v", leidos)
	}
}

// F1-26: lo que una persona fija a mano sobrevive al resolver, queda anotado
// en la bitácora y no se puede mover encima de otro ítem.
func TestFijarYLiberarUnItemDelPlan(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	programa, err := s.Deck.ByKind(ctx, DefaultChannelID, model.DeckProgram)
	if err != nil {
		t.Fatal(err)
	}
	// El canal por defecto está en Puerto Rico y su día empieza a las 6:00 AM.
	inicio := time.Date(2026, 9, 7, 18, 0, 0, 0, time.UTC) // 14:00 local
	nuevo := func(cuando time.Time, dur time.Duration, hora string) *model.PlanItem {
		p := &model.PlanItem{
			ChannelID: DefaultChannelID, DeckID: programa.ID, BroadcastDay: "2026-09-07",
			PlannedAt: cuando, PlannedMs: dur.Milliseconds(), Origin: "asset",
			LocalClock: hora,
		}
		if err := s.Plan.Insert(ctx, p); err != nil {
			t.Fatalf("no entró el ítem de las %s: %v", hora, err)
		}
		return p
	}
	uno := nuevo(inicio, 24*time.Minute, "14:00")
	dos := nuevo(inicio.Add(30*time.Minute), 24*time.Minute, "14:30")

	// Moverlo cinco minutos más tarde y clavarlo ahí.
	cuando := model.Ms(inicio.Add(5 * time.Minute))
	if err := s.Plan.Fijar(ctx, uno.ID, cuando, nil); err != nil {
		t.Fatalf("no se pudo fijar el ítem: %v", err)
	}
	leidos, err := s.Plan.ListDay(ctx, DefaultChannelID, "2026-09-07")
	if err != nil {
		t.Fatal(err)
	}
	if len(leidos) != 2 {
		t.Fatalf("tenían que quedar dos ítems y hay %d", len(leidos))
	}
	if !leidos[0].Fijado || model.Ms(leidos[0].PlannedAt) != cuando || leidos[0].LocalClock != "14:05" {
		t.Fatalf("el ítem no quedó fijado a las 14:05: %+v", leidos[0])
	}

	// El resolver borra el futuro planeado, pero lo fijado se queda.
	n, err := s.Plan.DeleteFuturePlanned(ctx, DefaultChannelID, inicio.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("se tenía que borrar solo el ítem suelto y se borraron %d", n)
	}
	quedan, err := s.Plan.ListDay(ctx, DefaultChannelID, "2026-09-07")
	if err != nil {
		t.Fatal(err)
	}
	if len(quedan) != 1 || quedan[0].ID != uno.ID {
		t.Fatalf("lo fijado tiene que sobrevivir al resolver: %+v", quedan)
	}

	// El cambio quedó en la bitácora, y la cadena sigue sana.
	bitacora, err := s.Audit.List(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(bitacora) != 1 || bitacora[0].Entity != "plan_item" || bitacora[0].Field != "fijado" ||
		bitacora[0].EntityID == nil || *bitacora[0].EntityID != uno.ID {
		t.Fatalf("fijar un ítem tiene que anotarse en la bitácora: %+v", bitacora)
	}
	if !strings.Contains(bitacora[0].After, "fijado a las 14:05") {
		t.Fatalf("la anotación no dice cómo quedó el ítem: %q", bitacora[0].After)
	}
	if rota, err := s.Audit.Verify(ctx); err != nil || rota != nil {
		t.Fatalf("la bitácora quedó rota: %v %+v", err, rota)
	}

	// Soltarlo lo devuelve al resolver, y también se anota.
	if err := s.Plan.Liberar(ctx, uno.ID); err != nil {
		t.Fatalf("no se pudo soltar el ítem: %v", err)
	}
	sueltos, err := s.Plan.ListDay(ctx, DefaultChannelID, "2026-09-07")
	if err != nil {
		t.Fatal(err)
	}
	if len(sueltos) != 1 || sueltos[0].Fijado {
		t.Fatalf("el ítem tenía que quedar suelto: %+v", sueltos)
	}
	if bitacora, err := s.Audit.List(ctx, 0); err != nil || len(bitacora) != 2 {
		t.Fatalf("soltar un ítem también se anota: %v %+v", err, bitacora)
	}

	// Fijar un ítem encima de otro lo rechaza el esquema.
	_ = dos
	otro := nuevo(inicio.Add(2*time.Hour), 24*time.Minute, "16:00")
	if err := s.Plan.Fijar(ctx, otro.ID, model.Ms(inicio.Add(10*time.Minute)), nil); !errors.Is(err, ErrOverlap) {
		t.Fatalf("mover un ítem encima de otro tiene que dar ErrOverlap y dio: %v", err)
	}
	// Y un id que no existe se dice claro.
	if err := s.Plan.Fijar(ctx, 9999, cuando, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("se esperaba ErrNotFound y salió: %v", err)
	}
	if err := s.Plan.Liberar(ctx, 9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("se esperaba ErrNotFound y salió: %v", err)
	}
}
