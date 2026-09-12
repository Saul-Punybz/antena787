package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"antena787/internal/engine"
	"antena787/internal/ingest"
	"antena787/internal/model"
	"antena787/internal/store"
)

// NormalizedDir es la subcarpeta de datos donde queda la copia normalizada.
const NormalizedDir = "normalizado"

// ingestLoop vigila la carpeta de contenido. La carpeta se configura en el
// paso 7 del asistente, así que al arrancar puede no haber ninguna: se
// relee cada pocos segundos y la vigilancia se levanta o se cambia sola.
func (a *App) ingestLoop(ctx context.Context) error {
	var current string
	var stopWatch context.CancelFunc
	defer func() {
		if stopWatch != nil {
			stopWatch()
		}
	}()

	t := time.NewTicker(WatchPoll)
	defer t.Stop()
	for {
		dir := a.setting(ctx, KeyContentFolder)
		if dir != current {
			if stopWatch != nil {
				stopWatch()
				stopWatch = nil
			}
			current = dir
			if dir != "" {
				wctx, cancel := context.WithCancel(ctx)
				stopWatch = cancel
				if err := a.watch(wctx, dir, false); err != nil {
					a.Publish("ingest", "carpeta", err.Error())
					stopWatch()
					stopWatch = nil
					current = ""
				}
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

// watch levanta la vigilancia de una carpeta y la deja corriendo.
func (a *App) watch(ctx context.Context, dir string, portal bool) error {
	w, err := ingest.NewWatcher(ingest.WatcherOptions{Dir: dir, Recursive: true, Now: a.now})
	if err != nil {
		return err
	}
	go func() { _ = w.Run(ctx) }()
	go func() {
		for found := range w.Events() {
			if ctx.Err() != nil {
				return
			}
			if portal {
				a.acceptFromPortal(ctx, found.Path)
				continue
			}
			// Un archivo que acompaña a un video —el sonido de uno mudo, o
			// unos subtítulos— nunca entra como material aparte: lo que se
			// vuelve a procesar es el video (F1-58, F1-62).
			if found.Kind == ingest.EventSidecar {
				a.ReingestSidecar(ctx, found.Path)
				continue
			}
			a.IngestFile(ctx, found.Path)
		}
	}()
	a.Publish("ingest", "carpeta", "vigilando "+dir)
	return nil
}

// portalLoop vigila portal/entrada, que NO hereda la exclusión del antivirus
// (auditoría A3). Lo que sube alguien de fuera se mide ahí, sin red y con
// límite de tiempo, y solo si pasa se mueve a la biblioteca.
func (a *App) portalLoop(ctx context.Context) error {
	dir := filepath.Join(a.DataDir, ingest.UploadDir)
	if err := a.watch(ctx, dir, true); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}

// acceptFromPortal valida un archivo del portal y, si pasa, lo mueve a la
// carpeta de contenido para que entre por la puerta de siempre.
func (a *App) acceptFromPortal(ctx context.Context, path string) {
	if a.FFprobe == "" {
		a.Publish("ingest", "portal", "llegó "+filepath.Base(path)+" y no hay ffprobe para revisarlo")
		return
	}
	if _, err := ingest.ValidateUpload(ctx, a.FFprobe, path, 0, 0); err != nil {
		a.Incident("subida_rechazada", fmt.Sprintf("%s: %s", filepath.Base(path), ingest.Plain(err)))
		return
	}
	dst := a.setting(ctx, KeyContentFolder)
	if dst == "" {
		a.Publish("ingest", "portal", "llegó "+filepath.Base(path)+" y todavía no hay carpeta de contenido")
		return
	}
	moved, err := ingest.AcceptUpload(path, dst)
	if err != nil {
		a.Incident("subida_rechazada", fmt.Sprintf("%s: %s", filepath.Base(path), ingest.Plain(err)))
		return
	}
	a.Publish("ingest", "portal", "entró "+filepath.Base(moved))
}

// IngestFile mide un archivo, lo guarda y encola su normalización. Un archivo
// que no pasa queda en cuarentena, guardado igual, con su motivo en palabras
// claras: en Biblioteca se ve, y tiene su botón de dejarlo pasar.
func (a *App) IngestFile(ctx context.Context, path string) {
	if _, err := a.Store.Media.GetByPath(ctx, path); err == nil {
		return // ya está: la carpeta vigilada reavisa al reiniciar
	}
	deps, err := a.ingestDeps(ctx)
	if err != nil {
		a.Publish("ingest", "material", err.Error())
		return
	}

	asset, title, episodes, ingestErr := ingest.Ingest(ctx, deps, path)
	if err := guardarFicha(ctx, a, &asset); err != nil {
		a.Publish("ingest", "material", "no se pudo guardar "+filepath.Base(path)+": "+err.Error())
		return
	}
	a.RefreshCuarentena(ctx)
	if ingestErr != nil {
		a.Incident("cuarentena", fmt.Sprintf("%s: %s", filepath.Base(path), asset.PlainReason))
		return
	}

	if err := a.saveCatalog(ctx, &asset, title, episodes); err != nil {
		a.Publish("ingest", "material", "no se pudo fichar "+filepath.Base(path)+": "+err.Error())
	}
	a.Queue.Enqueue(asset.ID, a.airsAt(ctx, asset.ID))
	a.Publish("ingest", "material", "entró "+filepath.Base(path))
}

// ReingestSidecar vuelve a procesar el video al que acompaña un archivo que
// acaba de aparecer a su lado: el sonido de un video mudo o unos subtítulos.
// Es lo que saca de cuarentena a un archivo que llegó sin sonido en cuanto
// alguien deja el audio junto a él, sin que nadie tenga que tocar nada
// (F1-58, F1-62).
//
// El archivo de al lado no se guarda como material aparte y el video no
// estrena ficha: se actualiza la que ya tenía.
func (a *App) ReingestSidecar(ctx context.Context, sidecar string) {
	if a.yaLoSabe(ctx, sidecar) {
		return
	}
	deps, err := a.ingestDeps(ctx)
	if err != nil {
		a.Publish("ingest", "material", err.Error())
		return
	}
	asset, title, episodes, ingestErr := ingest.ReingestSidecar(ctx, deps, sidecar)
	if asset.Path == "" {
		// No acompaña a ningún video de la carpeta: no hay nada que volver
		// a procesar.
		if ingestErr != nil {
			a.Publish("ingest", "material", ingest.Plain(ingestErr))
		}
		return
	}
	if err := guardarFicha(ctx, a, &asset); err != nil {
		a.Publish("ingest", "material", "no se pudo guardar "+filepath.Base(asset.Path)+": "+err.Error())
		return
	}
	a.RefreshCuarentena(ctx)
	if ingestErr != nil {
		a.Incident("cuarentena", fmt.Sprintf("%s: %s", filepath.Base(asset.Path), asset.PlainReason))
		return
	}
	if err := a.saveCatalog(ctx, &asset, title, episodes); err != nil {
		a.Publish("ingest", "material", "no se pudo fichar "+filepath.Base(asset.Path)+": "+err.Error())
	}
	a.Queue.Enqueue(asset.ID, a.airsAt(ctx, asset.ID))
	a.Publish("ingest", "material", "entró "+filepath.Base(asset.Path))
}

// guardarFicha deja el archivo en la base después del ingest. Con la
// persistencia puesta (ingestDeps la pone siempre) el ingest ya lo guardó él
// mismo y le anotó las pistas de sonido y los archivos de al lado: entonces
// no se vuelve a escribir la fila, que borraría justo esos apuntes. Solo se
// guarda aquí lo que el ingest no llegó a guardar —un archivo que ni se pudo
// medir, o una base que falló en ese momento—, y SaveAsset se encarga de que
// nunca haya dos fichas del mismo archivo.
func guardarFicha(ctx context.Context, a *App, asset *model.MediaAsset) error {
	// Con persistencia, el ingest ya guardó la fila él mismo (desde el
	// primer segundo en «ingiriendo», y al final con su estado); volver a
	// guardarla aquí pisaría lo que anotó aparte, como el sonido de al lado.
	if asset.ID != 0 {
		return nil
	}
	return (&persist{a: a}).SaveAsset(ctx, asset)
}

// yaLoSabe dice si el archivo de al lado ya está anotado en la ficha del
// video al que acompaña y el video no está parado. La carpeta vigilada
// reavisa de todo lo que hay cada vez que arranca la máquina: sin esto, cada
// arranque volvería a procesar todos los videos que tienen unos subtítulos o
// un sonido al lado, y a rehacer sus copias de casa para nada.
func (a *App) yaLoSabe(ctx context.Context, sidecar string) bool {
	video, ok := ingest.VideoForSidecar(sidecar)
	if !ok {
		return false
	}
	asset, err := a.Store.Media.GetByPath(ctx, video)
	if err != nil || asset.State == model.AssetQuarantine {
		return false
	}
	return asset.AudioSidecar == sidecar || asset.SubtitulosSidecar == sidecar
}

// RequeueNormalize devuelve un archivo a la cola de normalización. Lo llama
// la API cuando alguien cambia la pista de sonido que sale al aire: el store
// ya dejó el archivo en «pendiente» y la copia de casa hay que rehacerla con
// la pista nueva (F1-61).
func (a *App) RequeueNormalize(ctx context.Context, assetID int64) {
	a.Queue.Enqueue(assetID, a.airsAt(ctx, assetID))
	nombre := "el archivo"
	if asset, err := a.Store.Media.Get(ctx, assetID); err == nil {
		nombre = filepath.Base(asset.Path)
	}
	a.Publish("material", "pista", nombre+" vuelve a la cola con la pista de sonido que elegiste")
}

// saveCatalog deja el título y sus episodios apuntando al archivo. Un título
// que ya existe no se duplica: la ficha vieja manda y solo se le enlaza el
// material que le faltaba.
func (a *App) saveCatalog(ctx context.Context, asset *model.MediaAsset, title model.Title, episodes []model.Episode) error {
	if strings.TrimSpace(title.Name) == "" {
		return nil
	}
	existing, err := a.Store.Title.FindByName(ctx, title.Name)
	switch {
	case err == nil:
		title.ID = existing.ID
		if existing.MediaAssetID == nil && len(episodes) == 0 {
			existing.MediaAssetID = &asset.ID
			if err := a.Store.Title.Update(ctx, &existing); err != nil {
				return err
			}
		}
	case errors.Is(err, store.ErrNotFound):
		title.ChannelID = nil
		if len(episodes) == 0 {
			title.MediaAssetID = &asset.ID
		}
		if err := a.Store.Title.Insert(ctx, &title); err != nil {
			return err
		}
	default:
		return err
	}

	// Un archivo que se vuelve a procesar —porque su sonido llegó después—
	// no estrena episodios: los que ya están puestos se quedan como están.
	yaPuestos, _ := a.Store.Episode.ListByTitle(ctx, title.ID)
	for i := range episodes {
		episodes[i].TitleID = title.ID
		episodes[i].MediaAssetID = &asset.ID
		if yaEstaElEpisodio(yaPuestos, episodes[i]) {
			continue
		}
		if err := a.Store.Episode.Insert(ctx, &episodes[i]); err != nil {
			return err
		}
	}
	return nil
}

// yaEstaElEpisodio dice si ese episodio ya está fichado en el título: mismo
// número de temporada y de episodio.
func yaEstaElEpisodio(puestos []model.Episode, e model.Episode) bool {
	for _, p := range puestos {
		if p.Season == e.Season && p.Number == e.Number {
			return true
		}
	}
	return false
}

// DiasDeCola es hasta dónde se mira hacia adelante buscando cuándo sale al
// aire un archivo que espera turno en la cola de normalización.
const DiasDeCola = 30

// airsAt dice cuándo sale al aire ese archivo, para que la cola de
// normalización lo ordene: lo que sale antes se normaliza antes (B7).
//
// La hora sale de **las reglas**, no del plan. Es a propósito: el resolver
// solo mete en el plan material que ya está normalizado (F1-41), así que un
// archivo recién llegado —el único que está en la cola— nunca aparecería en
// plan_item y la cola degeneraría en orden de llegada, que es justo lo que
// F1-42 prohíbe. El plan queda de respaldo para cuando las reglas no digan
// nada (un archivo que alguien colocó a mano, por ejemplo).
func (a *App) airsAt(ctx context.Context, assetID int64) time.Time {
	if t := a.airsAtPorReglas(ctx, assetID); !t.IsZero() {
		return t
	}
	now := a.Now()
	items, err := a.Store.Plan.ListRange(ctx, a.ChannelID, now, now.Add(7*24*time.Hour))
	if err != nil {
		return time.Time{}
	}
	for _, it := range items {
		if it.MediaAssetID != nil && *it.MediaAssetID == assetID {
			return it.PlannedAt
		}
	}
	return time.Time{}
}

// airsAtPorReglas busca la primera regla activa que programe ese archivo
// —por el título que lo apunta o por uno de sus episodios— y devuelve su
// próxima salida.
func (a *App) airsAtPorReglas(ctx context.Context, assetID int64) time.Time {
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return time.Time{}
	}
	titulos := a.titlesOfAsset(ctx, assetID)
	if len(titulos) == 0 {
		return time.Time{}
	}
	rules, err := a.Store.Rule.ListAll(ctx, a.ChannelID)
	if err != nil {
		return time.Time{}
	}
	now := a.Now()
	hoy := ch.BroadcastDay(now)

	var primera time.Time
	for _, rule := range rules {
		if !rule.Active || rule.TitleID == nil || !titulos[*rule.TitleID] {
			continue
		}
		if rule.At < 0 || rule.At > 1439 || !rule.Days.Valid() {
			continue
		}
		// El día de hoy entra: puede que la regla salga esta misma tarde.
		for d := 0; d <= DiasDeCola; d++ {
			day := hoy.Add(d)
			if day > rule.To {
				break
			}
			if !rule.Covers(day) {
				continue
			}
			cuando := instanteDe(ch, day, rule.At)
			if cuando.Before(now) {
				continue
			}
			if primera.IsZero() || cuando.Before(primera) {
				primera = cuando
			}
			break
		}
	}
	return primera
}

// titlesOfAsset devuelve los títulos que apuntan a ese archivo, sea
// directamente o por medio de un episodio.
func (a *App) titlesOfAsset(ctx context.Context, assetID int64) map[int64]bool {
	out := map[int64]bool{}
	titles, err := a.Store.Title.List(ctx)
	if err != nil {
		return out
	}
	for _, t := range titles {
		if t.MediaAssetID != nil && *t.MediaAssetID == assetID {
			out[t.ID] = true
			continue
		}
		eps, err := a.Store.Episode.ListByTitle(ctx, t.ID)
		if err != nil {
			continue
		}
		for _, e := range eps {
			if e.MediaAssetID != nil && *e.MediaAssetID == assetID {
				out[t.ID] = true
				break
			}
		}
	}
	return out
}

// instanteDe traduce una hora de pared a un instante dentro de un día de
// emisión: lo anterior al inicio del día vive en el calendario siguiente,
// igual que en el resolver.
func instanteDe(ch model.Channel, day model.Day, min model.Minutes) time.Time {
	base := day.Time(ch.Location())
	if min < ch.BroadcastDayAt {
		base = base.AddDate(0, 0, 1)
	}
	return time.Date(base.Year(), base.Month(), base.Day(),
		int(min)/60, int(min)%60, 0, 0, ch.Location())
}

// ingestDeps arma lo que el ingest necesita del canal: el formato de casa y
// el objetivo de volumen del perfil.
func (a *App) ingestDeps(ctx context.Context) (ingest.Deps, error) {
	if a.FFmpeg == "" || a.FFprobe == "" {
		return ingest.Deps{}, a.FFmpegErr
	}
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return ingest.Deps{}, err
	}
	lufs, peak := a.loudnessTarget(ctx)
	return ingest.Deps{
		FFmpeg:      a.FFmpeg,
		FFprobe:     a.FFprobe,
		Format:      FormatOf(ch.FormatProfile),
		TargetLUFS:  lufs,
		TruePeak:    peak,
		ChannelID:   nil,
		ComputeHash: true,
		Providers:   a.fichasEnLinea(ctx),
		// El idioma en el que el canal quiere el aire decide qué pista de
		// sonido sale cuando el archivo trae varias (F1-60).
		Preferencias: ingest.Preferencias{IdiomaAudio: a.idiomaAudio(ctx)},
		// Con la persistencia puesta, el ingest anota él mismo las pistas
		// que trae el archivo y de dónde salieron el sonido y los subtítulos
		// de al lado (F1-58, F1-60, F1-62).
		Persist: &persist{a: a},
		Now:     a.now,
	}, nil
}

// idiomaAudio es el idioma en el que el canal quiere el aire. De fábrica,
// español: esto es Puerto Rico (F1-60).
func (a *App) idiomaAudio(ctx context.Context) string {
	if s := a.setting(ctx, KeyAudioLanguage); s != "" {
		return s
	}
	return ingest.IdiomaAudioPorDefecto
}

// fichasEnLinea enciende la búsqueda de fichas por internet si el ajuste lo
// dice. De fábrica está apagada: la instalación entera funciona sin red y sin
// que nadie tenga que sacar una clave (PRD §10). Con ella encendida se
// consulta primero lo que no pide clave —TVmaze para series, la portada de
// los discos para música— y solo después TMDB, y solo si hay clave puesta.
func (a *App) fichasEnLinea(ctx context.Context) []ingest.Provider {
	if a.setting(ctx, KeyOnlineInfo) != "si" {
		return nil
	}
	return ingest.Providers(ingest.ProviderConfig{
		TVmaze:          true,
		CoverArtArchive: true,
		TMDBAPIKey:      a.setting(ctx, KeyTMDBKey),
	})
}

// loudnessTarget: la señal abierta pide −24 LKFS y la web −16 LUFS (PRD §7).
func (a *App) loudnessTarget(ctx context.Context) (lufs, peak float64) {
	if a.setting(ctx, KeyPlannedMode) == "internet" {
		return -16, -2
	}
	return -24, -2
}

// FormatOf traduce el perfil de formato del canal ("720p59.94") al formato
// que usan el ingest y el motor. Lo que no se reconoce sale en el de casa de
// CAtv, que es el que la F0 midió.
func FormatOf(profile string) engine.Format {
	f := engine.CAtv
	p := strings.ToLower(strings.TrimSpace(profile))
	if p == "" {
		return f
	}
	switch {
	case strings.HasPrefix(p, "480"):
		f.Width, f.Height = 720, 480
	case strings.HasPrefix(p, "576"):
		f.Width, f.Height = 720, 576
	case strings.HasPrefix(p, "720"):
		f.Width, f.Height = 1280, 720
	case strings.HasPrefix(p, "1080"):
		f.Width, f.Height = 1920, 1080
	}
	switch {
	case strings.Contains(p, "59.94"), strings.Contains(p, "5994"):
		f.FPSNum, f.FPSDen = 60000, 1001
	case strings.Contains(p, "29.97"), strings.Contains(p, "2997"):
		f.FPSNum, f.FPSDen = 30000, 1001
	case strings.HasSuffix(p, "50"):
		f.FPSNum, f.FPSDen = 50, 1
	case strings.HasSuffix(p, "25"):
		f.FPSNum, f.FPSDen = 25, 1
	}
	return f
}

// normalizeLoop es el trabajador de la cola de normalización. Al arrancar
// vuelve a encolar todo lo que quedó a medias: la cola vive en memoria a
// propósito.
func (a *App) normalizeLoop(ctx context.Context) error {
	// El aire manda sobre la preparación (ADR 0008): mientras la señal vaya
	// atrasada de su propio reloj, la cola se aparta y lo dice.
	a.Queue.Permiso = a.PermisoParaPreparar
	a.Queue.Aviso = func(texto string) { a.Publish("normalizacion", "permiso", texto) }
	a.requeuePending(ctx)
	return a.Queue.Run(ctx, &persist{a: a}, a.normalizeOne)
}

// requeuePending devuelve a la cola lo que quedó pendiente o en curso de un
// arranque anterior.
func (a *App) requeuePending(ctx context.Context) {
	assets, err := a.Store.Media.List(ctx, model.AssetReady)
	if err != nil {
		return
	}
	for _, m := range assets {
		if m.NormalizeState == ingest.NormalizePending || m.NormalizeState == ingest.NormalizeRunning {
			a.Queue.Enqueue(m.ID, a.airsAt(ctx, m.ID))
		}
	}
}

// normalizeOne deja el archivo en el formato de casa. Es lo que la cola
// llama; el resultado lo anota la cola en la base por medio de persist.
func (a *App) normalizeOne(ctx context.Context, j ingest.Job) (string, error) {
	if a.FFmpeg == "" || a.FFprobe == "" {
		return "", a.FFmpegErr
	}
	asset, err := a.Store.Media.Get(ctx, j.AssetID)
	if err != nil {
		return "", err
	}
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return "", err
	}
	m, err := ingest.Probe(ctx, a.FFprobe, asset.Path)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(a.DataDir, NormalizedDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := ingest.NormalizedPathFor(dir, asset.Path, ".mkv")
	lufs, peak := a.loudnessTarget(ctx)
	opts := ingest.NormalizeOptionsFor(asset, m, a.preferenciasDe(ctx, asset))
	// Y el tope de hilos: con el canal al aire se le apartan núcleos a la
	// señal, con el canal apagado ffmpeg se queda con la máquina entera.
	opts.Threads = a.HilosParaPreparar(ctx)
	// Una preparación que no termina nunca no puede dejar el archivo en el
	// limbo de «aún no listo para aire» para siempre (F1-71): se le da un
	// plazo proporcional al archivo y, si se pasa, cuenta como un intento
	// fallido con su motivo.
	plazo := a.normalizeDeadline(asset.DurationMs)
	nctx, cancel := context.WithTimeout(ctx, plazo)
	defer cancel()
	reporte, err := ingest.Normalize(nctx, a.FFmpeg, asset.Path, dst, FormatOf(ch.FormatProfile), lufs, peak, opts)
	if err != nil {
		if errors.Is(nctx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			return "", ingest.Plainf(err, "la preparación de «%s» se quedó colgada más de %s y se canceló",
				filepath.Base(asset.Path), plazoEnCristiano(plazo))
		}
		return "", err
	}
	// F1-03 pide que quede constancia de las dos pasadas de volumen: aquí es
	// donde el reporte deja de vivir solo en memoria.
	if _, err := ingest.SaveLoudness(ctx, &persist{a: a}, asset.ID, reporte); err != nil {
		a.Publish("material", "volumen",
			"no se pudo anotar cómo quedó el sonido de "+filepath.Base(asset.Path)+": "+err.Error())
	}
	return dst, nil
}

// Plazo de la normalización: nunca menos de NormalizeMinTimeout y, para
// archivos largos, NormalizeTimeoutFactor veces lo que dura el archivo (dos
// pasadas de ffmpeg en una máquina lenta caben de sobra; una que no termina
// en eso está colgada).
const (
	NormalizeMinTimeout    = 15 * time.Minute
	NormalizeTimeoutFactor = 4
)

// normalizeDeadline es cuánto se espera por la normalización de un archivo
// que dura durationMs.
func (a *App) normalizeDeadline(durationMs int64) time.Duration {
	if a.opts.NormalizeTimeout > 0 {
		return a.opts.NormalizeTimeout
	}
	plazo := time.Duration(durationMs) * time.Millisecond * NormalizeTimeoutFactor
	if plazo < NormalizeMinTimeout {
		plazo = NormalizeMinTimeout
	}
	return plazo
}

// plazoEnCristiano escribe un plazo como lo lee una persona: «15 min», «2 h 40 min».
func plazoEnCristiano(d time.Duration) string {
	min := int(d.Round(time.Minute) / time.Minute)
	switch {
	case min < 1:
		return fmt.Sprintf("%d s", int(d.Round(time.Second)/time.Second))
	case min < 60:
		return fmt.Sprintf("%d min", min)
	case min%60 == 0:
		return fmt.Sprintf("%d h", min/60)
	}
	return fmt.Sprintf("%d h %d min", min/60, min%60)
}

// preferenciasDe arma lo que la normalización no puede adivinar de un
// archivo ya fichado: el idioma que quiere el canal, la pista que eligió una
// persona y de dónde salieron el sonido y los subtítulos de al lado (F1-58,
// F1-60 a F1-62).
//
// La pista elegida solo se manda cuando del archivo se sabe qué pistas trae:
// de un archivo del que no se apuntó ninguna, el cero guardado no es una
// elección de nadie y quien decide es el ingest.
func (a *App) preferenciasDe(ctx context.Context, asset model.MediaAsset) ingest.Preferencias {
	p := ingest.Preferencias{
		IdiomaAudio:       a.idiomaAudio(ctx),
		AudioSidecar:      asset.AudioSidecar,
		SubtitulosSidecar: asset.SubtitulosSidecar,
	}
	if len(asset.PistasAudio) > 0 {
		elegida := asset.PistaAudioAire
		p.PistaAudio = &elegida
	}
	return p
}

// persist es el puente entre la cola de normalización y el store. Cuando un
// archivo queda listo, pide un recálculo: ya puede entrar al plan.
type persist struct{ a *App }

func (p *persist) SaveAsset(ctx context.Context, asset *model.MediaAsset) error {
	if asset.ID != 0 {
		return p.a.Store.Media.Update(ctx, asset)
	}
	// El mismo archivo puede estar ya fichado: pasa cuando el sonido llega
	// después y hay que volver a procesar un video que quedó en cuarentena
	// por mudo (F1-58). Entonces se actualiza su ficha, no se crea otra, y
	// lo que puso una persona a mano se respeta.
	viejo, err := p.a.Store.Media.GetByPath(ctx, asset.Path)
	if err != nil {
		return p.a.Store.Media.Insert(ctx, asset)
	}
	asset.ID = viejo.ID
	asset.CreatedAt = viejo.CreatedAt
	asset.ChannelID = viejo.ChannelID
	asset.IntentionalBlack = viejo.IntentionalBlack
	asset.NoLogo = viejo.NoLogo
	asset.LetThroughBy = viejo.LetThroughBy
	asset.PistaAudioSAP = viejo.PistaAudioSAP
	return p.a.Store.Media.Update(ctx, asset)
}

// SetAudioTracks anota qué pistas de sonido trae el archivo y cuál de ellas
// sale al aire (F1-60). El índice es el que ve ffmpeg: la primera pista es
// la cero.
func (p *persist) SetAudioTracks(ctx context.Context, assetID int64, pistas []ingest.AudioTrack, pistaAire int) error {
	asset, err := p.a.Store.Media.Get(ctx, assetID)
	if err != nil {
		return err
	}
	lista := make([]model.PistaAudio, 0, len(pistas))
	for _, t := range pistas {
		lista = append(lista, model.PistaAudio{
			Indice:  t.Index,
			Idioma:  t.Language,
			Canales: t.Channels,
			Titulo:  t.Title,
		})
	}
	asset.PistasAudio = lista
	asset.PistaAudioAire = pistaAire
	asset.UpdatedAt = p.a.Now()
	return p.a.Store.Media.Update(ctx, &asset)
}

// SetSidecars anota de dónde salieron el sonido y los subtítulos que venían
// al lado del video (F1-58, F1-62).
func (p *persist) SetSidecars(ctx context.Context, assetID int64, audio, subtitulos string) error {
	asset, err := p.a.Store.Media.Get(ctx, assetID)
	if err != nil {
		return err
	}
	asset.AudioSidecar = audio
	asset.SubtitulosSidecar = subtitulos
	asset.UpdatedAt = p.a.Now()
	return p.a.Store.Media.Update(ctx, &asset)
}

func (p *persist) SetNormalizeState(ctx context.Context, assetID int64, state, normalizedPath, plainReason string) error {
	asset, err := p.a.Store.Media.Get(ctx, assetID)
	if err != nil {
		return err
	}
	asset.NormalizeState = state
	if normalizedPath != "" {
		asset.NormalizedPath = normalizedPath
	}
	if plainReason != "" {
		asset.PlainReason = plainReason
	}
	// Un archivo que no se pudo preparar no se queda «aún no listo para aire»
	// para siempre, invisible: va a cuarentena con su motivo, donde se ve y
	// donde alguien puede dejarlo pasar tal cual (F1-71).
	if state == ingest.NormalizeFailed {
		asset.State = model.AssetQuarantine
		asset.MotivoCodigo = ingest.MotivoNormalizacion
	}
	asset.UpdatedAt = p.a.Now()
	if err := p.a.Store.Media.Update(ctx, &asset); err != nil {
		return err
	}
	switch state {
	case ingest.NormalizeReady:
		p.a.Publish("material", "listo", filepath.Base(asset.Path)+" ya está listo para aire")
		p.a.Recalc()
	case ingest.NormalizeFailed:
		p.a.Incident("normalizacion_fallida", filepath.Base(asset.Path)+": "+plainReason)
		p.a.RefreshCuarentena(ctx)
		p.a.Recalc()
	}
	return nil
}

// SetLoudness anota cómo quedó medida la copia normalizada. Es la mitad de
// F1-03 que faltaba: sin esto el reporte de volumen se moría al terminar la
// normalización y las columnas `lufs` y `true_peak` se quedaban vacías.
//
// pasadas == 0 quiere decir que el archivo no traía sonido y no había nada
// que medir: entonces no se escribe un cero que parecería una medición.
func (p *persist) SetLoudness(ctx context.Context, assetID int64, lufs, truePeak float64, pasadas int, nota string) error {
	asset, err := p.a.Store.Media.Get(ctx, assetID)
	if err != nil {
		return err
	}
	if pasadas > 0 {
		medido, pico := lufs, truePeak
		asset.LUFS = &medido
		asset.TruePeak = &pico
	}
	asset.UpdatedAt = p.a.Now()
	if err := p.a.Store.Media.Update(ctx, &asset); err != nil {
		return err
	}
	if nota != "" {
		p.a.Incident("subtitulos", filepath.Base(asset.Path)+": "+nota)
	}
	if pasadas > 0 {
		p.a.Publish("material", "volumen", fmt.Sprintf(
			"%s quedó medido en %.1f, con el pico en %.1f, después de %d pasadas",
			filepath.Base(asset.Path), lufs, truePeak, pasadas))
	}
	return nil
}

// setting lee un ajuste sin hacer ruido: el que no está vale "".
func (a *App) setting(ctx context.Context, key string) string {
	v, err := a.Store.Settings.Get(ctx, key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(v)
}

// ── probar una señal antes de guardarla (F2-116) ──────────────────────

// FuenteAProbar es lo que se quiere abrir. La clave viene en claro porque
// esto se llama **antes** de guardar, así que todavía no hay dónde tenerla
// cifrada; no se escribe en ningún sitio y no sale de esta función.
type FuenteAProbar struct {
	Tipo      string
	Direccion string
	Usuario   string
	Clave     string
}

// ResultadoDeProbar es qué se encontró al otro lado.
type ResultadoDeProbar struct {
	Responde bool
	Texto    string
	Detalle  string
	Video    string
	Audio    string
	Duracion string
	Avisos   []string
}

// PlazoDeProbar es lo que se espera a que una señal conteste. Cinco segundos:
// una fuente que no dice nada en cinco segundos no sirve para salir al aire, y
// quien está probando no puede quedarse mirando una rueda girar.
const PlazoDeProbar = 5 * time.Second

// ProbarFuente abre la señal de verdad y cuenta qué hay. **No guarda nada.**
//
// De todos los sistemas que se miraron, ninguno prueba antes de guardar: lo
// más cercano es la vista previa de MistServer, y es después
// (docs/investigacion/ENTRADAS-POR-URL-COMPARADAS-2026-09-11.md). Para quien
// es su propio departamento de IT —que es el caso— la diferencia es entre
// pegar una dirección y rezar, o saberlo en tres segundos.
func (a *App) ProbarFuente(ctx context.Context, f FuenteAProbar) ResultadoDeProbar {
	if a.FFprobe == "" {
		return ResultadoDeProbar{Texto: "todavía no encuentro ffprobe, así que no puedo mirar la señal",
			Detalle: fmt.Sprint(a.FFmpegErr)}
	}
	if f.Tipo == model.FuenteCaptura {
		// Una tarjeta de captura no se abre con una URL: se enumera con el
		// driver del sistema, y eso es T4. Decirlo es mejor que fallar raro.
		return ResultadoDeProbar{Texto: "las tarjetas de captura todavía no se pueden probar desde aquí"}
	}

	// La dirección se revisa SIEMPRE, no solo cuando hay clave: una dirección
	// mal escrita sin credenciales se colaba y el fallo salía como un error de
	// ffprobe que no decía nada. Decirlo aquí es gratis y es exacto.
	if err := revisarDireccion(f.Direccion); err != nil {
		return ResultadoDeProbar{Texto: err.Error()}
	}
	destino := f.Direccion
	if f.Usuario != "" || f.Clave != "" {
		var err error
		if destino, err = conCredenciales(destino, f.Usuario, f.Clave); err != nil {
			return ResultadoDeProbar{Texto: "esa dirección no la entiendo como dirección de red", Detalle: err.Error()}
		}
	}

	ctx, cancel := context.WithTimeout(ctx, PlazoDeProbar)
	defer cancel()

	// El nombre se resuelve aquí, antes de llamar a ffprobe, porque ffmpeg
	// reporta un fallo de DNS como «Input/output error» y eso no le dice nada
	// a nadie. Preguntarlo antes cuesta milisegundos y convierte un mensaje
	// inútil en uno exacto.
	if host := hostDe(f.Direccion); host != "" {
		if _, err := net.DefaultResolver.LookupHost(ctx, host); err != nil {
			return ResultadoDeProbar{
				Texto:   fmt.Sprintf("esta máquina no sabe quién es %q: revisa cómo está escrito, o si hace falta el DNS de la estación", host),
				Detalle: err.Error(),
			}
		}
	}

	m, err := ingest.ProbeRemoto(ctx, a.FFprobe, destino)
	if err != nil {
		// El detalle es lo que dijo ffprobe DE VERDAD, no el mensaje de
		// ingest pensado para archivos locales: ése dice «no se puede abrir
		// el archivo» de una dirección de red, que es exactamente el tipo de
		// pista falsa que hace perder una tarde.
		crudo := motivoCrudo(err)
		return ResultadoDeProbar{
			Texto:   textoDeFalloAlProbar(ctx, crudo),
			Detalle: crudo,
		}
	}

	res := ResultadoDeProbar{Responde: true, Texto: "la señal responde"}
	if m.Width > 0 && m.Height > 0 {
		res.Video = fmt.Sprintf("%s %dx%d", m.Codec, m.Width, m.Height)
		if m.FPS != "" {
			res.Video += " a " + m.FPS
		}
	} else {
		res.Avisos = append(res.Avisos, "no trae imagen: si esto no es una señal de solo audio, algo está mal")
	}
	switch {
	case m.AudioChannels > 0:
		res.Audio = fmt.Sprintf("%d canal(es)", m.AudioChannels)
	default:
		res.Avisos = append(res.Avisos, "no trae sonido: todo lo que sale al aire tiene que llevar audio")
	}
	if m.DurationMs > 0 {
		res.Duracion = (time.Duration(m.DurationMs) * time.Millisecond).Round(time.Second).String()
	} else {
		// Una señal en vivo no tiene final, y eso está bien: es la diferencia
		// entre un archivo y un vivo.
		res.Duracion = "en vivo (sin final)"
	}
	return res
}

// textoDeFalloAlProbar traduce el fallo a algo accionable. Es la parte que
// Rolando pidió sin pedirla: él perdió una tarde con un video que marcaba
// 0:00:00 porque el problema era el nombre del archivo, no el códec, y nada
// se lo dijo.
func textoDeFalloAlProbar(ctx context.Context, crudo string) string {
	if ctx.Err() != nil {
		return fmt.Sprintf("la dirección no contestó en %s: comprueba que el otro lado esté encendido y que esta máquina llegue hasta ahí",
			PlazoDeProbar)
	}
	t := strings.ToLower(crudo)
	switch {
	case strings.Contains(t, "401") || strings.Contains(t, "unauthorized"):
		return "la dirección responde pero rechaza el usuario o la clave"
	case strings.Contains(t, "403") || strings.Contains(t, "forbidden"):
		return "la dirección responde pero no deja entrar desde aquí"
	case strings.Contains(t, "404"):
		return "esa dirección existe pero no hay nada ahí: revisa la ruta"
	case strings.Contains(t, "connection refused"):
		return "nadie está escuchando en esa dirección y ese puerto"
	case strings.Contains(t, "no route") || strings.Contains(t, "unreachable"):
		return "esta máquina no llega hasta ahí: es cosa de la red, no de la señal"
	case strings.Contains(t, "no such host") || strings.Contains(t, "name resolution"):
		return "ese nombre no existe: revisa cómo está escrito"
	case strings.Contains(t, "certificate"):
		return "el otro lado usa un certificado que esta máquina no reconoce"
	case strings.Contains(t, "invalid data") || strings.Contains(t, "invalid argument"):
		return "ahí hay algo, pero no es una señal de video que yo sepa leer"
	case strings.Contains(t, "protocol not found") || strings.Contains(t, "protocol_whitelist"):
		return "no reconozco esa forma de dirección: prueba a poner http://, https://, srt:// o rtmp:// delante"
	case strings.Contains(t, "timed out") || strings.Contains(t, "timeout"):
		return "el otro lado aceptó la conexión pero no mandó nada"
	case strings.Contains(t, "server returned 5"):
		return "el servidor del otro lado está fallando él: no es cosa tuya"
	default:
		// Sin pista, se dice que no se sabe, y el detalle queda debajo. Decir
		// una causa inventada es peor que no decir ninguna: manda a la persona
		// a buscar donde no es.
		return "no se pudo abrir la señal, y el sistema no dijo por qué"
	}
}

// motivoCrudo saca lo que de verdad dijo ffprobe, desenvolviendo el mensaje
// que ingest pone encima para los archivos locales.
func motivoCrudo(err error) string {
	if err == nil {
		return ""
	}
	// El de más adentro es el que trae la salida de ffprobe.
	for {
		dentro := errors.Unwrap(err)
		if dentro == nil {
			break
		}
		err = dentro
	}
	t := strings.TrimSpace(err.Error())
	// ffprobe escribe varias líneas y la última suele ser la que importa.
	if lineas := strings.Split(t, "\n"); len(lineas) > 1 {
		for i := len(lineas) - 1; i >= 0; i-- {
			if l := strings.TrimSpace(lineas[i]); l != "" {
				return l
			}
		}
	}
	return t
}

// revisarDireccion comprueba lo que se puede comprobar sin salir a la red, y
// lo dice con la forma correcta delante. Es lo más barato que existe: caza el
// error más común —olvidar el principio— antes de gastar cinco segundos
// esperando a una dirección que nunca iba a funcionar.
func revisarDireccion(direccion string) error {
	u, err := url.Parse(strings.TrimSpace(direccion))
	if err != nil {
		return fmt.Errorf("esa dirección no la entiendo: %s", direccion)
	}
	if !slices.Contains(esquemasDeSenal, strings.ToLower(u.Scheme)) {
		return fmt.Errorf("a la dirección le falta el principio: prueba con http://%s", strings.TrimPrefix(direccion, "//"))
	}
	if u.Host == "" && u.Scheme != "file" {
		return fmt.Errorf("a la dirección le falta la máquina: http://LAMAQUINA:PUERTO/loquesea")
	}
	return nil
}

// conCredenciales mete usuario y clave en la URL, que es como los protocolos
// de red los esperan. Se hace **solo al llamar**, nunca al guardar: en la base
// el usuario y la clave viven separados y la clave, cifrada.
func conCredenciales(direccion, usuario, clave string) (string, error) {
	u, err := url.Parse(direccion)
	if err != nil {
		return "", err
	}
	// Ojo con esto: url.Parse("localhost:8080/hls") **no falla**. Devuelve
	// Scheme "localhost", así que comprobar que el esquema no esté vacío no
	// sirve de nada. Hay que comprobar que sea uno de los que existen.
	if !slices.Contains(esquemasDeSenal, strings.ToLower(u.Scheme)) {
		return "", fmt.Errorf("le falta el principio: http://, https://, srt://, rtmp://, rtsp:// o udp://")
	}
	u.User = url.UserPassword(usuario, clave)
	return u.String(), nil
}

// esquemasDeSenal son los principios de dirección que ffmpeg sabe abrir y que
// tienen sentido como fuente de un canal.
var esquemasDeSenal = []string{"http", "https", "srt", "rtmp", "rtmps", "rtsp", "udp", "rtp", "file"}

// hostDe saca la máquina de una dirección, o cadena vacía si no la tiene o si
// ya es un número (una IP no hay que resolverla).
func hostDe(direccion string) string {
	u, err := url.Parse(strings.TrimSpace(direccion))
	if err != nil || u.Host == "" {
		return ""
	}
	h := u.Hostname()
	if h == "" || net.ParseIP(h) != nil {
		return ""
	}
	return h
}

// CredencialesDeFuente saca el usuario y la clave de una señal en vivo, si
// tiene. **Es el único sitio donde la clave sale de la base**, y sale para ir
// directa a los argumentos de ffmpeg: no se guarda en ninguna variable que
// viva más que la llamada, no se escribe en la bitácora, y no sale por la API.
//
// Si la clave no se puede descifrar —otra instalación, o alguien tocó la
// base— se devuelve vacía y la señal se intenta abrir sin ella. Fallar al
// conectar es mejor que no intentarlo: el error que salga va a decir que
// rechazaron la clave, que es una pista, y no un silencio.
func (a *App) CredencialesDeFuente(ctx context.Context, fuenteID int64) (usuario, clave string) {
	cs, err := a.Store.Conexion.List(ctx, a.ChannelID, "entrada")
	if err != nil {
		return "", ""
	}
	for _, c := range cs {
		var p struct {
			FuenteID int64  `json:"fuente_id"`
			Usuario  string `json:"usuario"`
		}
		if json.Unmarshal([]byte(c.Params), &p) != nil || p.FuenteID != fuenteID {
			continue
		}
		if !c.TieneSecreto {
			return p.Usuario, ""
		}
		completa, err := a.Store.Conexion.Get(ctx, c.ID)
		if err != nil {
			// La clave existe y no se pudo leer: se dice una vez, porque si
			// no, la señal falla al conectar y nadie sabe por qué.
			a.Publish("motor", "vivo", "hay una clave guardada para esa señal y no se pudo leer: "+err.Error())
			return p.Usuario, ""
		}
		return p.Usuario, completa.Secret
	}
	return "", ""
}
