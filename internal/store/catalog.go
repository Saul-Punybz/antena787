package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"antena787/internal/model"
)

// ── títulos ───────────────────────────────────────────────────────────

// TitleRepo son las obras: series, películas, promos, ids, spots.
type TitleRepo struct{ db *sql.DB }

const titleCols = `id, channel_id, nombre, tipo, sinopsis, anio, genero,
	clasificacion_contenido, clasificacion_audiencia, caratula, fuente_ficha,
	media_asset_id`

func scanTitle(sc interface{ Scan(...any) error }) (model.Title, error) {
	var t model.Title
	var canal, asset sql.NullInt64
	var anio sql.NullInt64
	err := sc.Scan(&t.ID, &canal, &t.Name, &t.Kind, &t.Synopsis, &anio, &t.Genre,
		&t.ContentRating, &t.AudienceRating, &t.Artwork, &t.MetadataSource, &asset)
	if err != nil {
		return model.Title{}, err
	}
	t.ChannelID = ptrInt64(canal)
	t.Year = ptrInt(anio)
	t.MediaAssetID = ptrInt64(asset)
	return t, nil
}

// Insert crea un título y le pone el id.
func (r *TitleRepo) Insert(ctx context.Context, t *model.Title) error {
	if t == nil {
		return errors.New("hace falta el título")
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO title (channel_id, nombre, tipo, sinopsis, anio, genero,
			clasificacion_contenido, clasificacion_audiencia, caratula,
			fuente_ficha, media_asset_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt64(t.ChannelID), t.Name, string(t.Kind), t.Synopsis, nullInt(t.Year),
		t.Genre, t.ContentRating, t.AudienceRating, t.Artwork, t.MetadataSource,
		nullInt64(t.MediaAssetID))
	if err != nil {
		return translate("guardar el título", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return translate("guardar el título", err)
	}
	t.ID = id
	return nil
}

// Update guarda los cambios de un título.
func (r *TitleRepo) Update(ctx context.Context, t *model.Title) error {
	if t == nil {
		return errors.New("hace falta el título")
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE title SET channel_id = ?, nombre = ?, tipo = ?, sinopsis = ?, anio = ?,
			genero = ?, clasificacion_contenido = ?, clasificacion_audiencia = ?,
			caratula = ?, fuente_ficha = ?, media_asset_id = ?
		WHERE id = ?`,
		nullInt64(t.ChannelID), t.Name, string(t.Kind), t.Synopsis, nullInt(t.Year),
		t.Genre, t.ContentRating, t.AudienceRating, t.Artwork, t.MetadataSource,
		nullInt64(t.MediaAssetID), t.ID)
	if err != nil {
		return translate("guardar el título", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("título %d: %w", t.ID, ErrNotFound)
	}
	return nil
}

// Get busca un título por id.
func (r *TitleRepo) Get(ctx context.Context, id int64) (model.Title, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+titleCols+` FROM title WHERE id = ?`, id)
	t, err := scanTitle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Title{}, fmt.Errorf("título %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return model.Title{}, translate("leer el título", err)
	}
	return t, nil
}

// List devuelve todos los títulos, por nombre.
func (r *TitleRepo) List(ctx context.Context) ([]model.Title, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+titleCols+` FROM title ORDER BY nombre`)
	if err != nil {
		return nil, translate("listar los títulos", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.Title
	for rows.Next() {
		t, err := scanTitle(rows)
		if err != nil {
			return nil, translate("listar los títulos", err)
		}
		out = append(out, t)
	}
	return out, translate("listar los títulos", rows.Err())
}

// FindByName busca un título por su nombre exacto, sin distinguir mayúsculas.
// Es lo que pregunta el importador antes de crear uno nuevo. Si no hay
// ninguno devuelve ErrNotFound; si hay varios, el de menor id.
func (r *TitleRepo) FindByName(ctx context.Context, name string) (model.Title, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+titleCols+` FROM title WHERE nombre = ? COLLATE NOCASE ORDER BY id LIMIT 1`, name)
	t, err := scanTitle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Title{}, fmt.Errorf("título %q: %w", name, ErrNotFound)
	}
	if err != nil {
		return model.Title{}, translate("buscar el título", err)
	}
	return t, nil
}

// ── episodios ─────────────────────────────────────────────────────────

// EpisodeRepo son los episodios de una serie y, sobre todo, el contador que
// dice cuál toca la próxima vez.
type EpisodeRepo struct{ db *sql.DB }

const episodeCols = `id, title_id, temporada, numero, nombre, media_asset_id`

func scanEpisode(sc interface{ Scan(...any) error }) (model.Episode, error) {
	var e model.Episode
	var asset sql.NullInt64
	err := sc.Scan(&e.ID, &e.TitleID, &e.Season, &e.Number, &e.Name, &asset)
	if err != nil {
		return model.Episode{}, err
	}
	e.MediaAssetID = ptrInt64(asset)
	return e, nil
}

// Insert crea un episodio y le pone el id.
func (r *EpisodeRepo) Insert(ctx context.Context, e *model.Episode) error {
	if e == nil {
		return errors.New("hace falta el episodio")
	}
	if e.Season == 0 {
		e.Season = 1
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO episode (title_id, temporada, numero, nombre, media_asset_id)
		VALUES (?, ?, ?, ?, ?)`,
		e.TitleID, e.Season, e.Number, e.Name, nullInt64(e.MediaAssetID))
	if err != nil {
		return translate("guardar el episodio", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return translate("guardar el episodio", err)
	}
	e.ID = id
	return nil
}

// ListByTitle devuelve los episodios de una serie en orden de emisión.
func (r *EpisodeRepo) ListByTitle(ctx context.Context, titleID int64) ([]model.Episode, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+episodeCols+` FROM episode WHERE title_id = ? ORDER BY temporada, numero`, titleID)
	if err != nil {
		return nil, translate("listar los episodios", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.Episode
	for rows.Next() {
		e, err := scanEpisode(rows)
		if err != nil {
			return nil, translate("listar los episodios", err)
		}
		out = append(out, e)
	}
	return out, translate("listar los episodios", rows.Err())
}

// NextAfter devuelve el episodio que toca después del último emitido. Si no
// hay último, o si el último era el final de la serie, empieza otra vez por
// el primero: la serie da la vuelta sola, que es lo que hace una estación.
// Si la serie no tiene episodios devuelve ErrNotFound.
func (r *EpisodeRepo) NextAfter(ctx context.Context, titleID int64, lastEpisodeID *int64) (model.Episode, error) {
	primero := func() (model.Episode, error) {
		row := r.db.QueryRowContext(ctx,
			`SELECT `+episodeCols+` FROM episode WHERE title_id = ?
			 ORDER BY temporada, numero LIMIT 1`, titleID)
		e, err := scanEpisode(row)
		if errors.Is(err, sql.ErrNoRows) {
			return model.Episode{}, fmt.Errorf("la serie %d no tiene episodios: %w", titleID, ErrNotFound)
		}
		if err != nil {
			return model.Episode{}, translate("buscar el próximo episodio", err)
		}
		return e, nil
	}

	if lastEpisodeID == nil {
		return primero()
	}

	var temporada, numero int
	err := r.db.QueryRowContext(ctx,
		`SELECT temporada, numero FROM episode WHERE id = ? AND title_id = ?`,
		*lastEpisodeID, titleID).Scan(&temporada, &numero)
	if errors.Is(err, sql.ErrNoRows) {
		// El último emitido ya no pertenece a esta serie: se empieza de cero.
		return primero()
	}
	if err != nil {
		return model.Episode{}, translate("buscar el próximo episodio", err)
	}

	row := r.db.QueryRowContext(ctx,
		`SELECT `+episodeCols+` FROM episode
		 WHERE title_id = ? AND (temporada > ? OR (temporada = ? AND numero > ?))
		 ORDER BY temporada, numero LIMIT 1`,
		titleID, temporada, temporada, numero)
	e, err := scanEpisode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return primero() // se acabó la serie: vuelve a empezar
	}
	if err != nil {
		return model.Episode{}, translate("buscar el próximo episodio", err)
	}
	return e, nil
}

// ── relleno ───────────────────────────────────────────────────────────

// FillerRepo es el material de relleno: promos, ids y cortinillas con la
// duración medida, que es lo que deja cuadrar un hueco.
type FillerRepo struct{ db *sql.DB }

const fillerCols = `id, media_asset_id, channel_id, tipo, duracion_ms`

func scanFiller(sc interface{ Scan(...any) error }) (model.FillerAsset, error) {
	var f model.FillerAsset
	var canal sql.NullInt64
	err := sc.Scan(&f.ID, &f.MediaAssetID, &canal, &f.Kind, &f.DurationMs)
	if err != nil {
		return model.FillerAsset{}, err
	}
	f.ChannelID = ptrInt64(canal)
	return f, nil
}

// List devuelve el relleno de un canal más el genérico (channel_id nulo),
// del más corto al más largo, que es como lo recorre el que cuadra huecos.
func (r *FillerRepo) List(ctx context.Context, channelID int64) ([]model.FillerAsset, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+fillerCols+` FROM filler_asset
		 WHERE channel_id = ? OR channel_id IS NULL
		 ORDER BY duracion_ms, id`, channelID)
	if err != nil {
		return nil, translate("listar el relleno", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.FillerAsset
	for rows.Next() {
		f, err := scanFiller(rows)
		if err != nil {
			return nil, translate("listar el relleno", err)
		}
		out = append(out, f)
	}
	return out, translate("listar el relleno", rows.Err())
}

// Insert añade una pieza de relleno y le pone el id.
func (r *FillerRepo) Insert(ctx context.Context, f *model.FillerAsset) error {
	if f == nil {
		return errors.New("hace falta la pieza de relleno")
	}
	if f.Kind == "" {
		f.Kind = "promo"
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO filler_asset (media_asset_id, channel_id, tipo, duracion_ms)
		VALUES (?, ?, ?, ?)`, f.MediaAssetID, nullInt64(f.ChannelID), f.Kind, f.DurationMs)
	if err != nil {
		return translate("guardar el relleno", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return translate("guardar el relleno", err)
	}
	f.ID = id
	return nil
}

// ── fuentes en vivo ───────────────────────────────────────────────────

// LiveSourceRepo son las entradas en vivo: SRT, RTMP o una captura.
type LiveSourceRepo struct{ db *sql.DB }

const liveCols = `id, channel_id, nombre, tipo, punto_de_escucha, solo_audio,
	duracion_prevista_ms, filler_de_respaldo, reloj_de_cortes, driver_de_cue,
	retardo_ms, gracia_s`

func scanLive(sc interface{ Scan(...any) error }) (model.LiveSource, error) {
	var l model.LiveSource
	var respaldo sql.NullInt64
	var reloj string
	err := sc.Scan(&l.ID, &l.ChannelID, &l.Name, &l.Kind, &l.ListenPoint, &l.AudioOnly,
		&l.PlannedDurationMs, &respaldo, &reloj, &l.CueDriver, &l.DelayMs, &l.GraceSeconds)
	if err != nil {
		return model.LiveSource{}, err
	}
	l.BackupFillerID = ptrInt64(respaldo)
	l.BreakClock = parseInts(reloj)
	return l, nil
}

// List devuelve las fuentes en vivo de un canal.
func (r *LiveSourceRepo) List(ctx context.Context, channelID int64) ([]model.LiveSource, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+liveCols+` FROM live_source WHERE channel_id = ? ORDER BY id`, channelID)
	if err != nil {
		return nil, translate("listar las fuentes en vivo", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.LiveSource
	for rows.Next() {
		l, err := scanLive(rows)
		if err != nil {
			return nil, translate("listar las fuentes en vivo", err)
		}
		out = append(out, l)
	}
	return out, translate("listar las fuentes en vivo", rows.Err())
}

// Upsert inserta la fuente si no tiene id y la actualiza si lo tiene.
func (r *LiveSourceRepo) Upsert(ctx context.Context, l *model.LiveSource) error {
	if l == nil {
		return errors.New("hace falta la fuente en vivo")
	}
	if l.ID == 0 {
		res, err := r.db.ExecContext(ctx, `
			INSERT INTO live_source (channel_id, nombre, tipo, punto_de_escucha,
				solo_audio, duracion_prevista_ms, filler_de_respaldo, reloj_de_cortes,
				driver_de_cue, retardo_ms, gracia_s)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			l.ChannelID, l.Name, l.Kind, l.ListenPoint, l.AudioOnly, l.PlannedDurationMs,
			nullInt64(l.BackupFillerID), jsonInts(l.BreakClock), l.CueDriver,
			l.DelayMs, l.GraceSeconds)
		if err != nil {
			return translate("guardar la fuente en vivo", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return translate("guardar la fuente en vivo", err)
		}
		l.ID = id
		return nil
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE live_source SET channel_id = ?, nombre = ?, tipo = ?, punto_de_escucha = ?,
			solo_audio = ?, duracion_prevista_ms = ?, filler_de_respaldo = ?,
			reloj_de_cortes = ?, driver_de_cue = ?, retardo_ms = ?, gracia_s = ?
		WHERE id = ?`,
		l.ChannelID, l.Name, l.Kind, l.ListenPoint, l.AudioOnly, l.PlannedDurationMs,
		nullInt64(l.BackupFillerID), jsonInts(l.BreakClock), l.CueDriver,
		l.DelayMs, l.GraceSeconds, l.ID)
	if err != nil {
		return translate("guardar la fuente en vivo", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("fuente en vivo %d: %w", l.ID, ErrNotFound)
	}
	return nil
}
