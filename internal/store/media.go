package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"antena787/internal/model"
)

// MediaAssetRepo es la biblioteca: un archivo por fila, con lo que el ingest
// midió y el estado en que quedó.
type MediaAssetRepo struct {
	db    *sql.DB
	audit *AuditRepo // para anotar lo que se cambia a mano; puede ser nil
}

const mediaCols = `id, channel_id, ruta, hash, codec, resolucion, fps, canales_audio,
	duracion_medida_ms, lufs, true_peak, tiene_subtitulos, formato_subtitulos,
	subtitulos_externos, negro_cabeza_ms, negro_cola_ms, marcas_de_corte_ms,
	cuadro_miniatura, estado, motivo_en_cristiano, motivo_codigo, estado_normalizacion,
	ruta_normalizada, negro_intencional, sin_logo, dejado_pasar_por,
	pistas_audio, pista_audio_aire, pista_audio_sap, audio_sidecar,
	subtitulos_sidecar, creado_ms, actualizado_ms`

func scanMedia(sc interface{ Scan(...any) error }) (model.MediaAsset, error) {
	var a model.MediaAsset
	var (
		canal      sql.NullInt64
		lufs       sql.NullFloat64
		truePeak   sql.NullFloat64
		subs       sql.NullString
		marcas     string
		pistas     string
		sap        sql.NullInt64
		creado     int64
		actualizad int64
	)
	err := sc.Scan(&a.ID, &canal, &a.Path, &a.Hash, &a.Codec, &a.Resolution, &a.FPS,
		&a.AudioChannels, &a.DurationMs, &lufs, &truePeak, &a.HasCaptions,
		&a.CaptionFormat, &subs, &a.HeadBlackMs, &a.TailBlackMs, &marcas,
		&a.Thumbnail, &a.State, &a.PlainReason, &a.MotivoCodigo, &a.NormalizeState,
		&a.NormalizedPath, &a.IntentionalBlack, &a.NoLogo, &a.LetThroughBy,
		&pistas, &a.PistaAudioAire, &sap, &a.AudioSidecar, &a.SubtitulosSidecar,
		&creado, &actualizad)
	if err != nil {
		return model.MediaAsset{}, err
	}
	a.ChannelID = ptrInt64(canal)
	a.LUFS = ptrFloat64(lufs)
	a.TruePeak = ptrFloat64(truePeak)
	a.ExternalCaptions = ptrString(subs)
	a.BreakMarksMs = parseInt64s(marcas)
	a.PistasAudio = parsePistasAudio(pistas)
	a.PistaAudioSAP = ptrInt(sap)
	a.CreatedAt = model.FromMs(creado)
	a.UpdatedAt = model.FromMs(actualizad)
	return a, nil
}

// Insert mete un archivo nuevo y deja el id en a. Si no traen fecha, creado y
// actualizado se ponen a ahora.
func (r *MediaAssetRepo) Insert(ctx context.Context, a *model.MediaAsset) error {
	if a == nil {
		return errors.New("hace falta el archivo")
	}
	ahora := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = ahora
	}
	if a.UpdatedAt.IsZero() {
		a.UpdatedAt = ahora
	}
	if a.State == "" {
		a.State = model.AssetIngesting
	}
	if a.NormalizeState == "" {
		a.NormalizeState = "pendiente"
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO media_asset (channel_id, ruta, hash, codec, resolucion, fps,
			canales_audio, duracion_medida_ms, lufs, true_peak, tiene_subtitulos,
			formato_subtitulos, subtitulos_externos, negro_cabeza_ms, negro_cola_ms,
			marcas_de_corte_ms, cuadro_miniatura, estado, motivo_en_cristiano,
			motivo_codigo, estado_normalizacion, ruta_normalizada, negro_intencional, sin_logo,
			dejado_pasar_por, pistas_audio, pista_audio_aire, pista_audio_sap,
			audio_sidecar, subtitulos_sidecar, creado_ms, actualizado_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt64(a.ChannelID), a.Path, a.Hash, a.Codec, a.Resolution, a.FPS,
		a.AudioChannels, a.DurationMs, nullFloat64(a.LUFS), nullFloat64(a.TruePeak),
		a.HasCaptions, a.CaptionFormat, nullString(a.ExternalCaptions),
		a.HeadBlackMs, a.TailBlackMs, jsonInt64s(a.BreakMarksMs), a.Thumbnail,
		string(a.State), a.PlainReason, a.MotivoCodigo, a.NormalizeState, a.NormalizedPath,
		a.IntentionalBlack, a.NoLogo, a.LetThroughBy,
		jsonPistasAudio(a.PistasAudio), a.PistaAudioAire, nullInt(a.PistaAudioSAP),
		a.AudioSidecar, a.SubtitulosSidecar,
		model.Ms(a.CreatedAt), model.Ms(a.UpdatedAt))
	if err != nil {
		return translate("guardar el archivo", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return translate("guardar el archivo", err)
	}
	a.ID = id
	return nil
}

// Update guarda los cambios de un archivo y le pone la hora de ahora.
func (r *MediaAssetRepo) Update(ctx context.Context, a *model.MediaAsset) error {
	if a == nil {
		return errors.New("hace falta el archivo")
	}
	a.UpdatedAt = time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE media_asset SET channel_id = ?, ruta = ?, hash = ?, codec = ?,
			resolucion = ?, fps = ?, canales_audio = ?, duracion_medida_ms = ?,
			lufs = ?, true_peak = ?, tiene_subtitulos = ?, formato_subtitulos = ?,
			subtitulos_externos = ?, negro_cabeza_ms = ?, negro_cola_ms = ?,
			marcas_de_corte_ms = ?, cuadro_miniatura = ?, estado = ?,
			motivo_en_cristiano = ?, motivo_codigo = ?, estado_normalizacion = ?, ruta_normalizada = ?,
			negro_intencional = ?, sin_logo = ?, dejado_pasar_por = ?,
			pistas_audio = ?, pista_audio_aire = ?, pista_audio_sap = ?,
			audio_sidecar = ?, subtitulos_sidecar = ?, actualizado_ms = ?
		WHERE id = ?`,
		nullInt64(a.ChannelID), a.Path, a.Hash, a.Codec, a.Resolution, a.FPS,
		a.AudioChannels, a.DurationMs, nullFloat64(a.LUFS), nullFloat64(a.TruePeak),
		a.HasCaptions, a.CaptionFormat, nullString(a.ExternalCaptions),
		a.HeadBlackMs, a.TailBlackMs, jsonInt64s(a.BreakMarksMs), a.Thumbnail,
		string(a.State), a.PlainReason, a.MotivoCodigo, a.NormalizeState, a.NormalizedPath,
		a.IntentionalBlack, a.NoLogo, a.LetThroughBy,
		jsonPistasAudio(a.PistasAudio), a.PistaAudioAire, nullInt(a.PistaAudioSAP),
		a.AudioSidecar, a.SubtitulosSidecar, model.Ms(a.UpdatedAt), a.ID)
	if err != nil {
		return translate("guardar el archivo", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("archivo %d: %w", a.ID, ErrNotFound)
	}
	return nil
}

// Get busca un archivo por id.
func (r *MediaAssetRepo) Get(ctx context.Context, id int64) (model.MediaAsset, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+mediaCols+` FROM media_asset WHERE id = ?`, id)
	a, err := scanMedia(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.MediaAsset{}, fmt.Errorf("archivo %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return model.MediaAsset{}, translate("leer el archivo", err)
	}
	return a, nil
}

// GetByPath busca un archivo por su ruta, que es única. Es lo que pregunta el
// ingest antes de volver a analizar algo.
func (r *MediaAssetRepo) GetByPath(ctx context.Context, path string) (model.MediaAsset, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+mediaCols+` FROM media_asset WHERE ruta = ?`, path)
	a, err := scanMedia(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.MediaAsset{}, fmt.Errorf("archivo %q: %w", path, ErrNotFound)
	}
	if err != nil {
		return model.MediaAsset{}, translate("leer el archivo", err)
	}
	return a, nil
}

// List devuelve los archivos en un estado. Con estado vacío, todos.
func (r *MediaAssetRepo) List(ctx context.Context, state model.AssetState) ([]model.MediaAsset, error) {
	consulta := `SELECT ` + mediaCols + ` FROM media_asset ORDER BY id`
	args := []any{}
	if state != "" {
		consulta = `SELECT ` + mediaCols + ` FROM media_asset WHERE estado = ? ORDER BY id`
		args = append(args, string(state))
	}
	return r.query(ctx, consulta, args...)
}

// ListReady devuelve lo que puede salir al aire: estado listo.
func (r *MediaAssetRepo) ListReady(ctx context.Context) ([]model.MediaAsset, error) {
	return r.List(ctx, model.AssetReady)
}

// SetState cambia el estado de un archivo y deja escrito el motivo en
// cristiano — el que se le enseña a la persona cuando algo quedó en
// cuarentena.
func (r *MediaAssetRepo) SetState(ctx context.Context, id int64, state model.AssetState, reason string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE media_asset SET estado = ?, motivo_en_cristiano = ?, actualizado_ms = ?
		WHERE id = ?`, string(state), reason, model.Ms(time.Now().UTC()), id)
	if err != nil {
		return translate("cambiar el estado del archivo", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("archivo %d: %w", id, ErrNotFound)
	}
	return nil
}

// SetPistaAudioAire cambia la pista de sonido que se va a oír al aire y
// devuelve el archivo a la cola de normalización, porque la copia de casa hay
// que rehacerla con la pista nueva (F1-61). Las dos cosas van en la misma
// escritura: no existe el archivo con la pista nueva y la copia vieja dada por
// buena.
//
// Si el índice no corresponde a ninguna pista del archivo devuelve
// ErrPistaInexistente y no cambia nada.
func (r *MediaAssetRepo) SetPistaAudioAire(ctx context.Context, id int64, indice int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return translate("cambiar la pista de sonido", err)
	}
	defer func() { _ = tx.Rollback() }()

	var pistas string
	var anterior int
	err = tx.QueryRowContext(ctx,
		`SELECT pistas_audio, pista_audio_aire FROM media_asset WHERE id = ?`, id).
		Scan(&pistas, &anterior)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("archivo %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return translate("cambiar la pista de sonido", err)
	}

	lista := parsePistasAudio(pistas)
	if !existePista(lista, indice) {
		return fmt.Errorf("%w: el archivo no trae ese sonido", ErrPistaInexistente)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE media_asset SET pista_audio_aire = ?, estado_normalizacion = 'pendiente',
			ruta_normalizada = '', actualizado_ms = ?
		WHERE id = ?`, indice, model.Ms(time.Now().UTC()), id); err != nil {
		return translate("cambiar la pista de sonido", err)
	}

	if err := r.anotar(ctx, tx, id, "pista_audio_aire",
		nombreDePista(lista, anterior), nombreDePista(lista, indice)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return translate("cambiar la pista de sonido", err)
	}
	return nil
}

// anotar deja el cambio en la bitácora encadenada, dentro de la misma
// transacción que lo hizo. Sin bitácora a mano (un repositorio suelto en una
// prueba) no se anota nada y el cambio sigue.
func (r *MediaAssetRepo) anotar(ctx context.Context, tx *sql.Tx, id int64, campo, antes, despues string) error {
	if r.audit == nil {
		return nil
	}
	entidad := id
	e := model.AuditEntry{
		Entity: "media_asset", EntityID: &entidad, Field: campo,
		Before: antes, After: despues, Origin: "humano", Kind: "cambio",
	}
	return r.audit.appendIn(ctx, tx, &e)
}

// existePista dice si ese índice es una de las pistas del archivo. Un archivo
// del que no se sabe qué pistas trae solo admite la primera, que es la que
// suena si nadie elige nada.
func existePista(pistas []model.PistaAudio, indice int) bool {
	if indice < 0 {
		return false
	}
	if len(pistas) == 0 {
		return indice == 0
	}
	for _, p := range pistas {
		if p.Indice == indice {
			return true
		}
	}
	return false
}

// nombreDePista escribe la pista en cristiano para la bitácora: "pista 2
// (inglés)" o, si no se sabe nada de ella, "pista 2".
func nombreDePista(pistas []model.PistaAudio, indice int) string {
	for _, p := range pistas {
		if p.Indice != indice {
			continue
		}
		rotulo := p.Titulo
		if rotulo == "" {
			rotulo = p.Idioma
		}
		if rotulo != "" {
			return fmt.Sprintf("pista %d (%s)", indice, rotulo)
		}
		break
	}
	return fmt.Sprintf("pista %d", indice)
}

// jsonPistasAudio guarda la lista de pistas. Nunca escribe "null": la columna
// es NOT NULL y el valor vacío es "[]".
func jsonPistasAudio(v []model.PistaAudio) string {
	if len(v) == 0 {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func parsePistasAudio(s string) []model.PistaAudio {
	if s == "" {
		return nil
	}
	var v []model.PistaAudio
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil
	}
	return v
}

func (r *MediaAssetRepo) query(ctx context.Context, consulta string, args ...any) ([]model.MediaAsset, error) {
	rows, err := r.db.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, translate("listar los archivos", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.MediaAsset
	for rows.Next() {
		a, err := scanMedia(rows)
		if err != nil {
			return nil, translate("listar los archivos", err)
		}
		out = append(out, a)
	}
	return out, translate("listar los archivos", rows.Err())
}
