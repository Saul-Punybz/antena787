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
// que no pasa queda en cuarentena, guardado igual, con su motivo en
// cristiano: en Biblioteca se ve, y tiene su botón de dejarlo pasar.
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
	if err := a.Store.Media.Insert(ctx, &asset); err != nil {
		a.Publish("ingest", "material", "no se pudo guardar "+filepath.Base(path)+": "+err.Error())
		return
	}
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

	for i := range episodes {
		episodes[i].TitleID = title.ID
		episodes[i].MediaAssetID = &asset.ID
		if err := a.Store.Episode.Insert(ctx, &episodes[i]); err != nil {
			return err
		}
	}
	return nil
}

// airsAt dice cuándo sale al aire ese archivo, para que la cola de
// normalización lo ordene: lo que sale antes se normaliza antes (B7).
func (a *App) airsAt(ctx context.Context, assetID int64) time.Time {
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
		Now:         a.now,
	}, nil
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
	opts := ingest.NormalizeOptionsFor(asset, m)
	if _, err := ingest.Normalize(ctx, a.FFmpeg, asset.Path, dst, FormatOf(ch.FormatProfile), lufs, peak, opts); err != nil {
		return "", err
	}
	return dst, nil
}

// persist es el puente entre la cola de normalización y el store. Cuando un
// archivo queda listo, pide un recálculo: ya puede entrar al plan.
type persist struct{ a *App }

func (p *persist) SaveAsset(ctx context.Context, asset *model.MediaAsset) error {
	if asset.ID == 0 {
		return p.a.Store.Media.Insert(ctx, asset)
	}
	return p.a.Store.Media.Update(ctx, asset)
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
