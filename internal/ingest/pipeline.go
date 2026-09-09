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

	// Preferencias son el idioma en el que el canal quiere el aire y la
	// pista que haya elegido una persona. El valor cero pide español, que es
	// lo que se emite aquí (F1-60).
	Preferencias Preferencias

	// SidecarStableFor es cuánto tiene que llevar quieto un archivo de al
	// lado para darlo por copiado del todo. 0 = WatchStableFor, los mismos
	// diez segundos de la carpeta vigilada (F1-01).
	SidecarStableFor time.Duration

	// Persist es opcional. Cuando está, el ingest guarda el archivo en
	// cuanto termina de medirlo —SaveAsset le pone el ID— y anota ahí mismo
	// las pistas de sonido que trae, cuál va al aire y de dónde salieron el
	// sonido y los subtítulos de al lado (F1-58, F1-60, F1-62). Cuando no
	// está, el ingest no toca la base y quien llama guarda como siempre.
	Persist Persist

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

	// Lo que el ingest averigua del sonido. Se anota en la base tanto si el
	// archivo acaba listo como si acaba en cuarentena: en Biblioteca se ve
	// igual, y quien mire el archivo mudo tiene que poder ver que sí trae
	// tres pistas o que el audio de al lado estaba a medio copiar.
	var (
		medido       bool
		pistas       []AudioTrack
		pistaAire    int
		audioSidecar string
		subtSidecar  string
	)
	anota := func() {
		if d.Persist == nil || !medido {
			return
		}
		if err := d.Persist.SaveAsset(ctx, &asset); err != nil {
			return
		}
		// Que la base falle al anotar esto no manda el material a
		// cuarentena: el archivo está bien, lo que falta es el apunte.
		_ = d.Persist.SetAudioTracks(ctx, asset.ID, pistas, pistaAire)
		_ = d.Persist.SetSidecars(ctx, asset.ID, audioSidecar, subtSidecar)
	}

	fail := func(err error) (model.MediaAsset, model.Title, []model.Episode, error) {
		asset.State = model.AssetQuarantine
		asset.PlainReason = Plain(err)
		asset.MotivoCodigo = Motivo(err)
		asset.UpdatedAt = d.now()
		anota()
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
	medido = true
	pistas = m.AudioTracks
	pistaAire = PistaDeAire(pistas, d.Preferencias)

	if !m.HasVideo && !m.HasAudio {
		return fail(Plainf(nil, "el archivo %q no tiene ni imagen ni sonido: no hay nada que emitir", trimName(path)))
	}
	if m.DurationMs <= 0 {
		return fail(Plainf(nil, "el archivo %q dura cero: está incompleto o se cortó al copiarlo", trimName(path)))
	}
	if err := retry.retry(ctx, func() error { return CheckDecodable(ctx, d.FFmpeg, path) }); err != nil {
		return fail(err)
	}
	// Sin sonido no se sale al aire (F1-59). Pero antes de parar el archivo
	// se mira si el sonido vino al lado, en un archivo con el mismo nombre:
	// eso es lo normal en el material que llega con la imagen y el audio
	// por separado, y en ese caso se muxea y el archivo sigue su camino
	// (F1-58).
	audioMs := m.AudioMs
	if !m.HasAudio {
		hallado, hay := FindAudioSidecar(path)
		switch {
		case !hay:
			return fail(Plainc(nil, MotivoSinAudio,
				"«%s» no trae sonido: pon a su lado un archivo de audio con el mismo nombre (.wav, .m4a, .aac, .mp3 o .flac) y lo vuelvo a procesar",
				trimName(path)))
		case !StableSidecar(hallado, d.SidecarStableFor, d.now()):
			return fail(Plainc(nil, MotivoSinAudio,
				"«%s» no trae sonido y «%s», que está a su lado, todavía se está copiando: en cuanto termine lo vuelvo a procesar",
				trimName(path), trimName(hallado)))
		}
		sm, serr := Probe(ctx, d.FFprobe, hallado)
		if serr != nil || !sm.HasAudio {
			return fail(Plainc(serr, MotivoSinAudio,
				"«%s» no trae sonido y «%s», el que está a su lado, tampoco se oye: ponle uno con sonido y lo vuelvo a procesar",
				trimName(path), trimName(hallado)))
		}
		audioSidecar = hallado
		asset.AudioChannels = sm.AudioChannels
		audioMs = sm.DurationMs
	}
	// Imagen y sonido de distinta duración: el archivo llegó incompleto o se
	// cortó al copiarlo (F1-70). Se puede dejar pasar —hay material así a
	// propósito—, pero no sin que alguien lo vea.
	if desfase := DesfaseAV(m.VideoMs, audioMs); desfase > DesfaseAVMaximo {
		return fail(Plainc(nil, MotivoDesfaseAV,
			"«%s» tiene la imagen y el sonido de distinta duración (imagen %s, sonido %s): el archivo llegó incompleto o se cortó al copiarlo. Si es así a propósito, se puede dejar pasar bajo tu responsabilidad",
			trimName(path), mmss(m.VideoMs), mmss(audioMs)))
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
	if sidecar, ok := FindSubtitleSidecar(path); ok {
		if err := AttachCaptions(&asset, sidecar); err != nil {
			asset.PlainReason = Plain(err)
		} else {
			subtSidecar = sidecar
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
	anota()
	return asset, title, episodes, nil
}

// ReingestSidecar vuelve a procesar el video al que acompaña un archivo de
// al lado que acaba de aparecer. Es lo que hace falta cuando el sonido llega
// después de que el video ya quedó en cuarentena por mudo: la carpeta
// vigilada avisa del .wav con EventSidecar, y esto vuelve a correr el ingest
// del video entero, que ahora sí encuentra el sonido y sale de cuarentena
// (F1-58).
//
// El archivo de al lado nunca entra por su cuenta a la biblioteca: si no
// acompaña a ningún video, esto lo dice y no hace nada.
func ReingestSidecar(ctx context.Context, d Deps, sidecarPath string) (model.MediaAsset, model.Title, []model.Episode, error) {
	video, ok := VideoForSidecar(sidecarPath)
	if !ok {
		return model.MediaAsset{}, model.Title{}, nil,
			Plainf(nil, "«%s» no acompaña a ningún video de la carpeta", trimName(sidecarPath))
	}
	return Ingest(ctx, d, video)
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
// ingerido: lo que se recorta, si hay que desentrelazar, qué pista de sonido
// sale al aire y qué hacer con los subtítulos. Es lo que le pasa la cola a
// Normalize.
//
// prefs es opcional a propósito —quien no tenga nada que decir la omite y
// todo sigue igual— y es por donde entran el idioma preferido del canal, la
// pista que eligió una persona y las rutas de los archivos de al lado que ya
// estén guardadas en el media_asset (F1-58, F1-60, F1-62). Lo que no venga
// en prefs se busca en la carpeta del archivo.
func NormalizeOptionsFor(a model.MediaAsset, m Measure, prefs ...Preferencias) NormalizeOptions {
	var p Preferencias
	if len(prefs) > 0 {
		p = prefs[0]
	}
	o := NormalizeOptions{
		SourceDurationMs: a.DurationMs,
		Deinterlace:      m.Interlaced,
		KeepCaptions:     m.HasCaptions && m.CaptionStream >= 0,
		EmbeddedCEA608:   m.CaptionFormat == "cea-608",
		AudioTrack:       PistaDeAire(m.AudioTracks, p),
	}

	// Sonido de al lado: solo hace falta cuando el archivo no trae pista
	// propia. Si el media_asset ya sabe de dónde salió, manda eso.
	if !m.HasAudio {
		o.AudioSidecar = p.AudioSidecar
		if o.AudioSidecar == "" {
			if s, ok := FindAudioSidecar(a.Path); ok {
				o.AudioSidecar = s
			}
		}
	}

	// Subtítulos de al lado: los .srt y los .vtt se meten en la copia de
	// casa como pista de texto; los .scc se quedan guardados tal cual para
	// que F2 los reinserte como CEA-608 (F1-62).
	sub := p.SubtitulosSidecar
	if sub == "" && a.ExternalCaptions != nil {
		sub = *a.ExternalCaptions
	}
	if sub == "" {
		if s, ok := FindSubtitleSidecar(a.Path); ok {
			sub = s
		}
	}
	if SubtituloMuxeable(sub) {
		o.SubtitleSidecar = sub
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
