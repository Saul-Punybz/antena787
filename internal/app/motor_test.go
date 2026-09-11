package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// Pruebas de la T1 de F2: la fuente que traduce el plan a clips, y el bucle
// del motor. Lo que se mide del conformado —audio corto, video corto,
// pillarbox, cambio de clip— está en internal/engine; aquí se mide lo que
// el motor entiende del plan.

// archivoDePrueba deja un archivo con algo dentro: la fuente comprueba que
// exista y que no esté en cero, no lo decodifica.
func archivoDePrueba(t *testing.T, ruta string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatalf("no pude crear la carpeta: %v", err)
	}
	if err := os.WriteFile(ruta, []byte("no es un video de verdad, pero pesa"), 0o644); err != nil {
		t.Fatalf("no pude escribir el archivo: %v", err)
	}
	return ruta
}

// conCartel apunta un cartel de la estación que ya existe, para que la
// fuente no tenga que dibujar uno con ffmpeg en cada prueba.
func conCartel(t *testing.T, a *App) string {
	t.Helper()
	ruta := archivoDePrueba(t, filepath.Join(t.TempDir(), "cartel.mkv"))
	if err := a.Store.Settings.Set(context.Background(), KeyDefaultFiller, ruta); err != nil {
		t.Fatalf("no pude apuntar el cartel: %v", err)
	}
	return ruta
}

// conBloque mete un archivo listo para aire y un bloque del plan que lo pone
// a esa hora. El bloque queda fijado para que una corrida del resolver no se
// lo lleve.
func conBloque(t *testing.T, a *App, nombre string, arranca time.Time, dura time.Duration) (model.MediaAsset, model.PlanItem) {
	t.Helper()
	ctx := context.Background()
	ruta := archivoDePrueba(t, filepath.Join(t.TempDir(), nombre+".norm.mkv"))
	asset := model.MediaAsset{
		Path:           filepath.Join(t.TempDir(), nombre+".mkv"),
		DurationMs:     dura.Milliseconds(),
		State:          model.AssetReady,
		NormalizeState: model.NormalizeReady,
		NormalizedPath: ruta,
		CreatedAt:      a.Now(),
		UpdatedAt:      a.Now(),
	}
	if err := a.Store.Media.Insert(ctx, &asset); err != nil {
		t.Fatalf("no pude guardar el archivo: %v", err)
	}
	return asset, conBloqueDe(t, a, asset, model.DeckProgram, arranca, dura, nil)
}

// conBloqueDe mete un bloque del plan que pone ese archivo en ese deck.
func conBloqueDe(t *testing.T, a *App, asset model.MediaAsset, deck model.DeckKind,
	arranca time.Time, dura time.Duration, dentroDe *int64) model.PlanItem {
	t.Helper()
	ctx := context.Background()
	// La base guarda milisegundos: se redondea aquí para que lo que devuelve
	// el ayudante sea exactamente lo que hay guardado.
	arranca = arranca.Truncate(time.Millisecond)
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer el canal: %v", err)
	}
	d, err := a.Store.Deck.ByKind(ctx, a.ChannelID, deck)
	if err != nil {
		t.Fatalf("no pude leer el deck: %v", err)
	}
	id := asset.ID
	item := model.PlanItem{
		ChannelID:    a.ChannelID,
		DeckID:       d.ID,
		BroadcastDay: ch.BroadcastDay(arranca),
		PlannedAt:    arranca,
		PlannedMs:    dura.Milliseconds(),
		Origin:       model.OriginAsset,
		MediaAssetID: &id,
		Inside:       dentroDe,
		State:        model.Planned,
		Fijado:       true,
	}
	if err := a.Store.Plan.Insert(ctx, &item); err != nil {
		t.Fatalf("no pude guardar el bloque: %v", err)
	}
	return item
}

// F2-13 — el bloque que empezó hace 7:32 entra por el minuto 7:32 del
// archivo, no desde el principio.
func TestF2_13ElBloqueEnCursoEntraPorDondeToca(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	conCartel(t, a)

	ahora := a.Now()
	asset, item := conBloque(t, a, "Get Smart", ahora.Add(-7*time.Minute-32*time.Second), 20*time.Minute)

	f := a.nuevaFuenteDelPlan(ctx, engine.CAtv)
	clip, hasta, err := f.Next(ahora)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	if clip.Path != asset.NormalizedPath {
		t.Fatalf("sale %q y tenía que salir la copia normalizada %q", clip.Path, asset.NormalizedPath)
	}
	if clip.Ref != item.ID {
		t.Fatalf("el clip no dice de qué bloque es: %d", clip.Ref)
	}
	if clip.SeekMs < 450_000 || clip.SeekMs > 454_000 {
		t.Fatalf("entra por el milisegundo %d y tocaba por el 452000", clip.SeekMs)
	}
	if !hasta.Equal(item.End()) {
		t.Fatalf("el corte es %s y el bloque acaba a las %s", hasta, item.End())
	}

	// Y el bloque queda cargado: si el servicio se reinicia, ResetCuedToPlanned
	// lo devuelve a planeado y vuelve a entrar por donde toque.
	de, err := a.PlanItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("no pude releer el bloque: %v", err)
	}
	if de.State != model.Cued {
		t.Fatalf("el bloque quedó en %q y tenía que quedar cargado", de.State)
	}
}

// F2-09 y F2-10 — sin plan sale el relleno, y sin biblioteca de relleno sale
// el cartel de la estación. Nunca nada vacío.
func TestF2_09SinPlanSaleElRellenoYAlFinalElCartel(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	cartel := conCartel(t, a)

	f := a.nuevaFuenteDelPlan(ctx, engine.CAtv)
	clip, hasta, err := f.Next(a.Now())
	if err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	if clip.Path != cartel {
		t.Fatalf("sin relleno cargado tenía que salir el cartel y sale %q", clip.Path)
	}
	if hasta.Before(a.Now()) {
		t.Fatalf("el corte del relleno ya pasó: %s", hasta)
	}

	// Con algo en la biblioteca de relleno, sale eso y no el cartel.
	asset, _ := conBloque(t, a, "cortinilla", a.Now().Add(48*time.Hour), time.Minute)
	canal := a.ChannelID
	if err := a.Store.Filler.Insert(ctx, &model.FillerAsset{
		MediaAssetID: asset.ID, ChannelID: &canal, Kind: "cortinilla", DurationMs: 60_000,
	}); err != nil {
		t.Fatalf("no pude meter el relleno: %v", err)
	}
	if got := f.Filler().Path; got != asset.NormalizedPath {
		t.Fatalf("el relleno vigente es %q y tenía que ser %q", got, asset.NormalizedPath)
	}
}

// El bloque que sale cierra su as-run: es el motor quien por fin llama a
// MarkAired, que estaba escrita desde F1 sin nadie que la usara.
func TestElBloqueQueSaleCierraSuAsRun(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	conCartel(t, a)

	arranque := a.Now().Add(-30 * time.Minute).Truncate(time.Millisecond)
	_, item := conBloque(t, a, "Kojak", arranque, 30*time.Minute)

	f := a.nuevaFuenteDelPlan(ctx, engine.CAtv)
	// Media hora a 59.94 son 107892 cuadros.
	f.ClipSalio(engine.Clip{Path: "x", Ref: item.ID}, arranque, 107_892, true)

	de, err := a.PlanItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("no pude releer el bloque: %v", err)
	}
	if de.State != model.Aired {
		t.Fatalf("el bloque quedó en %q y tenía que quedar emitido", de.State)
	}
	if de.ActualAt == nil || !de.ActualAt.Equal(arranque.UTC()) {
		t.Fatalf("el instante real es %v y tenía que ser %s", de.ActualAt, arranque)
	}
	if de.ActualMs == nil || *de.ActualMs < 1_795_000 || *de.ActualMs > 1_805_000 {
		t.Fatalf("la duración real es %v ms y tenía que ser media hora", de.ActualMs)
	}
	if de.Partial {
		t.Fatal("el bloque salió entero y quedó marcado como parcial")
	}
}

// F2-12 — un archivo que falla al aire dispara la cascada siempre, pero solo
// va a cuarentena al segundo fallo separado por más de cinco minutos.
func TestF2_12ElArchivoVaACuarentenaAlSegundoFallo(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	conCartel(t, a)

	asset, item := conBloque(t, a, "Tarzan", a.Now(), 30*time.Minute)
	f := a.nuevaFuenteDelPlan(ctx, engine.CAtv)

	// Primer fallo: el bloque se marca fallido y el archivo sigue en la
	// parrilla. Un parpadeo de red no saca un programa bueno.
	f.ClipFallo(engine.Clip{Path: asset.NormalizedPath, Ref: item.ID}, "el NAS se reinició")
	de, err := a.Store.Media.Get(ctx, asset.ID)
	if err != nil {
		t.Fatalf("no pude releer el archivo: %v", err)
	}
	if de.State != model.AssetReady {
		t.Fatalf("al primer fallo el archivo quedó en %q", de.State)
	}
	bloque, err := a.PlanItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("no pude releer el bloque: %v", err)
	}
	if bloque.State != model.Failed {
		t.Fatalf("el bloque quedó en %q y tenía que quedar fallido", bloque.State)
	}

	// Un segundo fallo dentro de los cinco minutos tampoco lo saca.
	otro := conBloqueDe(t, a, asset, model.DeckProgram, a.Now().Add(time.Hour), 30*time.Minute, nil)
	f.ClipFallo(engine.Clip{Path: asset.NormalizedPath, Ref: otro.ID}, "otra vez el NAS")
	if de, _ := a.Store.Media.Get(ctx, asset.ID); de.State != model.AssetReady {
		t.Fatalf("dos fallos seguidos sacaron el archivo de la parrilla: %q", de.State)
	}

	// Separados por más de cinco minutos, sí: para que no se programe otra
	// vez la semana que viene y falle igual.
	f.mu.Lock()
	f.fallos[asset.ID] = a.Now().Add(-6 * time.Minute)
	f.mu.Unlock()
	tercero := conBloqueDe(t, a, asset, model.DeckProgram, a.Now().Add(2*time.Hour), 30*time.Minute, nil)
	f.ClipFallo(engine.Clip{Path: asset.NormalizedPath, Ref: tercero.ID}, "el archivo está en cero")
	de, err = a.Store.Media.Get(ctx, asset.ID)
	if err != nil {
		t.Fatalf("no pude releer el archivo: %v", err)
	}
	if de.State != model.AssetQuarantine {
		t.Fatalf("el archivo quedó en %q y tenía que quedar en cuarentena", de.State)
	}
}

// F2-16 — un elemento programado dentro de otro bloque le quita el aire
// mientras dura, y al terminar el bloque de dentro el de fuera vuelve solo,
// por el instante que le toca.
func TestF2_16ElElementoDeDentroTomaElAireYElDeFueraVuelveSolo(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	conCartel(t, a)

	ahora := a.Now()
	_, bloque := conBloque(t, a, "RadioOnce", ahora, 30*time.Minute)
	id, _ := conBloque(t, a, "identificativo", ahora.Add(48*time.Hour), time.Minute)
	dentro := conBloqueDe(t, a, id, model.DeckCommercial, ahora.Add(10*time.Minute), 20*time.Second, &bloque.ID)

	f := a.nuevaFuenteDelPlan(ctx, engine.CAtv)

	// Antes de que le toque, el aire es del bloque de fuera, y el corte es
	// justo cuando entra el de dentro.
	clip, hasta, err := f.Next(ahora)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	if clip.Ref != bloque.ID {
		t.Fatalf("a las %s sale el bloque %d y tenía que salir el %d", ahora, clip.Ref, bloque.ID)
	}
	if !hasta.Equal(dentro.PlannedAt) {
		t.Fatalf("el corte es %s y tenía que ser cuando entra el de dentro (%s)", hasta, dentro.PlannedAt)
	}

	// Mientras dura, el aire es suyo.
	clip, hasta, err = f.Next(dentro.PlannedAt)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	if clip.Ref != dentro.ID {
		t.Fatalf("el elemento de dentro no tomó el aire: salió el bloque %d", clip.Ref)
	}
	if !hasta.Equal(dentro.End()) {
		t.Fatalf("el corte es %s y tenía que ser el fin del elemento (%s)", hasta, dentro.End())
	}

	// Y al terminar, el de fuera vuelve solo, por donde iba. Ojo: el bloque de
	// fuera de esta prueba es un archivo, así que **se pausa** mientras el de
	// dentro tiene el aire y reanuda donde se quedó (F2-07, construido en T2);
	// antes de T2 volvía por la hora de pared, que es lo que hace una señal en
	// vivo de verdad (F2-08, su propia prueba).
	vuelta := dentro.End()
	clip, _, err = f.Next(vuelta)
	if err != nil {
		t.Fatalf("la fuente no supo qué poner: %v", err)
	}
	if clip.Ref != bloque.ID {
		t.Fatalf("el bloque de fuera no volvió: salió el %d", clip.Ref)
	}
	if quiere := dentro.PlannedAt.Sub(bloque.PlannedAt).Milliseconds(); clip.SeekMs != quiere {
		t.Fatalf("vuelve por el milisegundo %d y tocaba por el %d, donde se quedó", clip.SeekMs, quiere)
	}
}

// En modo sombra el motor no arranca: lo dice una vez y no toca nada. F1 no
// cambia de conducta porque F2 exista (F1-31).
func TestEnSombraElMotorNoArranca(t *testing.T) {
	a := abre(t)
	var vistos eventos
	vistos.mirando(a, t)
	conCartel(t, a)
	_, item := conBloque(t, a, "Space Cobra", a.Now(), 30*time.Minute)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.Start(ctx)

	esperar(t, 5*time.Second, func() bool { return vistos.hay("motor", "sombra", "modo sombra") },
		"el motor no dijo que el canal está en sombra")

	// Un segundo de gracia: si fuera a arrancar, ya habría cargado el bloque.
	time.Sleep(time.Second)
	de, err := a.PlanItem(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("no pude releer el bloque: %v", err)
	}
	if de.State != model.Planned {
		t.Fatalf("el bloque quedó en %q: en sombra nadie lo carga ni lo emite", de.State)
	}
	if _, err := os.Stat(filepath.Join(a.DataDir, "motor")); err == nil {
		t.Fatal("el motor dejó su registro en la carpeta de datos y en sombra no corre")
	}
}

// El motor de verdad, de punta a punta: canal al aire, un bloque con un clip
// hecho con ffmpeg, y un transport stream que crece. Es la primera vez que
// F2-01 se puede medir con datos reales (la corrida de 8 horas es del soak).
func TestElMotorAlAireEscribeUnTransportStream(t *testing.T) {
	ffmpeg, _ := herramientas(t)
	a := abre(t)
	ctx := context.Background()

	// Un formato chico: la prueba tiene que caber en segundos, no en horas.
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer el canal: %v", err)
	}
	ch.Mode, ch.FormatProfile = ModoAire, "480i59.94"
	if err := a.Store.Channel.Update(ctx, ch); err != nil {
		t.Fatalf("no pude poner el canal al aire: %v", err)
	}

	dir := t.TempDir()
	clip := clipConSonido(t, ffmpeg, filepath.Join(dir, "programa.mkv"), 3)
	if err := a.Store.Settings.Set(ctx, KeyDefaultFiller, clip); err != nil {
		t.Fatalf("no pude apuntar el cartel: %v", err)
	}

	asset := model.MediaAsset{
		Path: clip, NormalizedPath: clip, DurationMs: 3000,
		State: model.AssetReady, NormalizeState: model.NormalizeReady,
		CreatedAt: a.Now(), UpdatedAt: a.Now(),
	}
	if err := a.Store.Media.Insert(ctx, &asset); err != nil {
		t.Fatalf("no pude guardar el archivo: %v", err)
	}
	item := conBloqueDe(t, a, asset, model.DeckProgram, a.Now(), 2*time.Second, nil)

	salida := filepath.Join(dir, "salida.ts")
	canal := a.ChannelID
	if err := a.Store.Output.Upsert(ctx, &model.Output{
		ChannelID: canal, Name: "archivo", Driver: "archivo",
		Params: fmt.Sprintf(`{"ruta": %q}`, salida),
	}); err != nil {
		t.Fatalf("no pude configurar la salida: %v", err)
	}

	vivo, cancel := context.WithCancel(ctx)
	a.Start(vivo)

	esperar(t, 30*time.Second, func() bool {
		st, err := os.Stat(salida)
		return err == nil && st.Size() > 100_000
	}, "el transport stream no creció: el motor no llegó a la salida")

	// El bloque de dos segundos ya salió: su as-run está escrito.
	esperar(t, 30*time.Second, func() bool {
		de, err := a.PlanItem(ctx, item.ID)
		return err == nil && de.State == model.Aired && de.ActualAt != nil
	}, "el bloque no quedó emitido con su instante real")

	cancel()
	_ = a.Close()

	// Y el motor dejó dicho lo que hizo, que es la evidencia del soak.
	registro, err := filepath.Glob(filepath.Join(a.DataDir, "motor", "eventos-*.jsonl"))
	if err != nil || len(registro) == 0 {
		t.Fatal("el motor no dejó su registro de eventos")
	}
}

// clipConSonido fabrica un clip con imagen y sonido de verdad.
func clipConSonido(t *testing.T, ffmpeg, dst string, segundos float64) string {
	t.Helper()
	dur := fmt.Sprintf("%.3f", segundos)
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-t", dur, "-i", "testsrc2=s=640x480:r=30",
		"-f", "lavfi", "-t", dur, "-i", "sine=f=440:r=48000",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-g", "30",
		"-c:a", "aac", "-ar", "48000", "-ac", "2", dst}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("no se pudo fabricar el clip: %v\n%s", err, out)
	}
	return dst
}
