package ingest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// Deps es todo lo que el ingest necesita de fuera. No hay nada global: dos
// canales en la misma máquina pueden tener formatos y objetivos de volumen
// distintos y no se pisan.
type Deps struct {
	FFmpeg  string // ruta al binario (engine.FFmpeg())
	FFprobe string // ruta al binario (engine.FFprobe())

	Format     engine.Format // formato de casa del canal
	TargetLUFS float64       // objetivo del perfil del país: −24 en EEUU y PR
	TruePeak   float64       // −2 dBTP en el mismo perfil

	ChannelID *int64 // a qué canal pertenece; nil = biblioteca compartida

	ThumbnailDir string // dónde van las miniaturas; vacío = ".miniaturas" junto al archivo
	ArtworkDir   string // dónde va la carátula extraída; vacío = junto al archivo

	Providers []Provider // drivers de fichas por red; vacío = nada de red

	ComputeHash bool // calcular el SHA-256 (lee el archivo entero)

	Retry RetryPolicy      // reintento de fallos de lectura pasajeros
	Now   func() time.Time // reloj, para las pruebas
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now().UTC()
	}
	return time.Now().UTC()
}

func (d Deps) retryPolicy() RetryPolicy {
	if d.Retry.Delay == 0 && d.Retry.Sleep == nil {
		return DefaultRetry()
	}
	if d.Retry.Sleep == nil {
		d.Retry.Sleep = SleepCtx
	}
	return d.Retry
}

func (d Deps) thumbDir(path string) string {
	if d.ThumbnailDir != "" {
		return d.ThumbnailDir
	}
	return filepath.Join(filepath.Dir(path), ".miniaturas")
}

// Ingest es el paso 1 del PRD de principio a fin: mide, mira el negro y el
// silencio, saca la miniatura, recoge los subtítulos que vengan al lado y
// busca la ficha. Deja el archivo listo y la normalización pendiente — esa
// la corre la Queue aparte, priorizada por hora de aire (AUDITORIA B7).
//
// Si algo va mal devuelve el asset ya marcado en cuarentena, con su
// motivo_en_cristiano puesto, y además el error: el que llama guarda el
// asset igual, porque un archivo en cuarentena también se ve en Biblioteca y
// tiene su botón de "dejarlo pasar bajo mi responsabilidad" (PRD §9 paso 1).
func Ingest(ctx context.Context, d Deps, path string) (model.MediaAsset, model.Title, []model.Episode, error) {
	now := d.now()
	retry := d.retryPolicy()

	asset := model.MediaAsset{
		ChannelID:      d.ChannelID,
		Path:           path,
		State:          model.AssetIngesting,
		NormalizeState: NormalizePending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	var title model.Title

	fail := func(err error) (model.MediaAsset, model.Title, []model.Episode, error) {
		asset.State = model.AssetQuarantine
		asset.PlainReason = Plain(err)
		asset.UpdatedAt = d.now()
		return asset, title, nil, err
	}

	// 0 · ¿el archivo está ahí y tiene algo dentro?
	var st os.FileInfo
	if err := retry.retry(ctx, func() error {
		var e error
		st, e = os.Stat(path)
		return e
	}); err != nil {
		return fail(Plainf(err, "no se puede abrir el archivo %q: puede que se haya movido o que el disco no responda", trimName(path)))
	}
	if st.IsDir() {
		return fail(Plainf(nil, "%q es una carpeta, no un archivo", trimName(path)))
	}
	if st.Size() == 0 {
		return fail(Plainf(nil, "el archivo %q está vacío (0 bytes): la copia no terminó o el origen falló", trimName(path)))
	}

	// 1 · medir con ffprobe
	var m Measure
	if err := retry.retry(ctx, func() error {
		var e error
		m, e = Probe(ctx, d.FFprobe, path)
		return e
	}); err != nil {
		return fail(err)
	}
	// Lo medido se apunta antes de decidir nada: un archivo que acabe en
	// cuarentena también se ve en Biblioteca, y se ve mejor con su duración
	// y su formato puestos. Quien lo deje pasar no tiene que remedirlo.
	asset.Codec = m.Codec
	asset.Resolution = m.Resolution
	asset.FPS = m.FPS
	asset.AudioChannels = m.AudioChannels
	asset.DurationMs = m.DurationMs
	asset.HasCaptions = m.HasCaptions
	asset.CaptionFormat = m.CaptionFormat

	if !m.HasVideo && !m.HasAudio {
		return fail(Plainf(nil, "el archivo %q no tiene ni imagen ni sonido: no hay nada que emitir", trimName(path)))
	}
	if m.DurationMs <= 0 {
		return fail(Plainf(nil, "el archivo %q dura cero: está incompleto o se cortó al copiarlo", trimName(path)))
	}
	if err := retry.retry(ctx, func() error { return CheckDecodable(ctx, d.FFmpeg, path) }); err != nil {
		return fail(err)
	}
	// Sin sonido detectable no se da por listo solo (F1-10). No es un
	// archivo roto —puede ser material mudo a propósito— así que tampoco se
	// descarta: queda en cuarentena con su motivo en cristiano y decide una
	// persona con el botón de "dejarlo pasar bajo mi responsabilidad" (PRD
	// §9 paso 1). Si lo deja pasar, la normalización le pone el silencio de
	// casa (NormalizeOptions.NoAudio, que sale de NormalizeOptionsFor) y el
	// archivo entra al formato de casa como cualquier otro.
	if !m.HasAudio {
		return fail(Plainf(nil, "%q no trae sonido: revisa el máster antes de programarlo", trimName(path)))
	}

	if d.ComputeHash {
		if h, err := HashFile(ctx, path); err == nil {
			asset.Hash = h
		}
	}

	// 2 · negro y silencio. Que el análisis falle no manda nada a
	// cuarentena: el archivo ya se sabe que abre, y sin recorte se emite
	// igual, solo con el slate delante.
	if bs, err := Analyze(ctx, d.FFmpeg, path, time.Duration(m.DurationMs)*time.Millisecond); err == nil {
		asset.HeadBlackMs = bs.HeadMs
		asset.TailBlackMs = bs.TailMs
		asset.BreakMarksMs = bs.MarksMs
	}

	// 3 · miniatura. También es de mejor esfuerzo: un archivo sin miniatura
	// se emite perfectamente, solo se ve peor en Biblioteca.
	if m.HasVideo {
		dst := filepath.Join(d.thumbDir(path), thumbName(path))
		if err := Thumbnail(ctx, d.FFmpeg, path, ThumbnailAt(m.DurationMs), dst); err == nil {
			asset.Thumbnail = dst
		}
	}

	// 4 · subtítulos que vengan al lado. Un .srt roto no tumba el ingest: se
	// avisa y el archivo entra sin él.
	if sidecar, ok := FindSidecar(path); ok {
		if err := AttachCaptions(&asset, sidecar); err != nil {
			asset.PlainReason = Plain(err)
		}
	}

	// 5 · ficha y carátula: local primero, red al final (PRD §10)
	card, _ := Metadata(ctx, path, m, MetadataDeps{
		FFmpeg:     d.FFmpeg,
		ArtworkDir: d.ArtworkDir,
		Providers:  d.Providers,
	})
	title, episodes := cardToModel(card, d.ChannelID)

	asset.State = model.AssetReady
	asset.NormalizeState = NormalizePending
	asset.UpdatedAt = d.now()
	return asset, title, episodes, nil
}

// cardToModel pasa la ficha a los tipos del dominio. El media_asset_id lo
// pone quien guarda: aquí todavía no hay ID.
func cardToModel(c Card, channelID *int64) (model.Title, []model.Episode) {
	t := model.Title{
		ChannelID:      channelID,
		Name:           c.Name,
		Kind:           c.Kind,
		Synopsis:       c.Synopsis,
		Genre:          c.Genre,
		ContentRating:  c.ContentRating,
		AudienceRating: c.AudienceRating,
		Artwork:        firstNonEmpty(c.ArtworkPath, c.ArtworkURL),
		MetadataSource: strings.Join(c.Sources, "+"),
	}
	if c.Year > 0 {
		y := c.Year
		t.Year = &y
	}
	if c.Kind != model.TitleSeries || (c.Season == 0 && c.Episode == 0) {
		return t, nil
	}
	return t, []model.Episode{{
		Season: c.Season,
		Number: c.Episode,
		Name:   firstNonEmpty(c.EpisodeName, c.Name),
	}}
}

// NormalizeOptionsFor arma las opciones de normalización de un asset ya
// ingerido: lo que se recorta, si hay que desentrelazar y qué hacer con los
// subtítulos. Es lo que le pasa la cola a Normalize.
//
// Un archivo sin sonido solo llega hasta aquí si una persona lo dejó pasar
// bajo su responsabilidad (F1-10): en ese caso se marca NoAudio y la copia de
// casa sale con el silencio sintetizado del perfil, en vez de sin pista.
func NormalizeOptionsFor(a model.MediaAsset, m Measure) NormalizeOptions {
	o := NormalizeOptions{
		SourceDurationMs: a.DurationMs,
		Deinterlace:      m.Interlaced,
		KeepCaptions:     m.HasCaptions && m.CaptionStream >= 0,
		EmbeddedCEA608:   m.CaptionFormat == "cea-608",
		NoAudio:          !m.HasAudio,
	}
	// El material marcado como negro intencional no se recorta: abre en
	// negro a propósito (PRD §9 paso 1).
	if !a.IntentionalBlack {
		o.TrimHeadMs = a.HeadBlackMs
		o.TrimTailMs = a.TailBlackMs
	}
	return o
}

// NormalizedPathFor decide dónde va la copia normalizada: misma carpeta de
// destino, mismo nombre, contenedor del perfil.
func NormalizedPathFor(dir, src, ext string) string {
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	if ext == "" {
		ext = ".mp4"
	}
	return filepath.Join(dir, base+"-casa"+ext)
}

func thumbName(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return base + ".png"
}
