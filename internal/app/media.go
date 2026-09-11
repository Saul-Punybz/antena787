package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
