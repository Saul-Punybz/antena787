package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"antena787/internal/model"
)

// ── títulos ───────────────────────────────────────────────────────────

// TitleRepo son las obras: series, películas, promos, ids, spots.
type TitleRepo struct {
	db    *sql.DB
	audit *AuditRepo // para anotar lo que se empareja a mano; puede ser nil
}

const titleCols = `id, channel_id, nombre, tipo, sinopsis, anio, genero,
	clasificacion_contenido, clasificacion_audiencia, caratula, fuente_ficha,
	media_asset_id, pendiente_emparejar, candidatos, infantil_core, preset_id`

// titleColsDe es titleCols con cada columna precedida del alias de tabla,
// para las consultas que juntan title con otra tabla.
func titleColsDe(alias string) string {
	cols := strings.Split(titleCols, ",")
	for i := range cols {
		cols[i] = alias + "." + strings.TrimSpace(cols[i])
	}
	return strings.Join(cols, ", ")
}

func scanTitle(sc interface{ Scan(...any) error }) (model.Title, error) {
	var t model.Title
	var canal, asset sql.NullInt64
	var anio sql.NullInt64
	var candidatos string
	err := sc.Scan(&t.ID, &canal, &t.Name, &t.Kind, &t.Synopsis, &anio, &t.Genre,
		&t.ContentRating, &t.AudienceRating, &t.Artwork, &t.MetadataSource, &asset,
		&t.PendienteEmparejar, &candidatos, &t.InfantilCore, &t.PresetID)
	if err != nil {
		return model.Title{}, err
	}
	t.ChannelID = ptrInt64(canal)
	t.Year = ptrInt(anio)
	t.MediaAssetID = ptrInt64(asset)
	t.Candidatos = parseInt64s(candidatos)
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
			fuente_ficha, media_asset_id, pendiente_emparejar, candidatos,
			infantil_core, preset_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt64(t.ChannelID), t.Name, string(t.Kind), t.Synopsis, nullInt(t.Year),
		t.Genre, t.ContentRating, t.AudienceRating, t.Artwork, t.MetadataSource,
		nullInt64(t.MediaAssetID), t.PendienteEmparejar, jsonInt64s(t.Candidatos),
		t.InfantilCore, nullInt64(t.PresetID))
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
			caratula = ?, fuente_ficha = ?, media_asset_id = ?,
			pendiente_emparejar = ?, candidatos = ?, infantil_core = ?,
			preset_id = ?
		WHERE id = ?`,
		nullInt64(t.ChannelID), t.Name, string(t.Kind), t.Synopsis, nullInt(t.Year),
		t.Genre, t.ContentRating, t.AudienceRating, t.Artwork, t.MetadataSource,
		nullInt64(t.MediaAssetID), t.PendienteEmparejar, jsonInt64s(t.Candidatos),
		t.InfantilCore, nullInt64(t.PresetID), t.ID)
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

// ByAsset busca el título al que pertenece un archivo: el que lo tiene de
// archivo propio (película, programa) o el de la serie de uno de cuyos
// episodios sale. Es lo que Al aire necesita para poner nombre a lo que
// está saliendo sin cargar el catálogo entero.
func (r *TitleRepo) ByAsset(ctx context.Context, assetID int64) (model.Title, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+titleCols+` FROM title WHERE media_asset_id = ?
		UNION ALL
		SELECT `+titleColsDe("t")+` FROM title t JOIN episode e ON e.title_id = t.id WHERE e.media_asset_id = ?
		LIMIT 1`, assetID, assetID)
	t, err := scanTitle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Title{}, fmt.Errorf("archivo %d sin título: %w", assetID, ErrNotFound)
	}
	if err != nil {
		return model.Title{}, translate("leer el título del archivo", err)
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

// Get busca un episodio por id.
func (r *EpisodeRepo) Get(ctx context.Context, id int64) (model.Episode, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+episodeCols+` FROM episode WHERE id = ?`, id)
	e, err := scanEpisode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Episode{}, fmt.Errorf("episodio %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return model.Episode{}, translate("leer el episodio", err)
	}
	return e, nil
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

// NombreDelArchivo dice cómo se llama, para una persona, el archivo con ese
// id: el nombre del título que lo usa o, si es un episodio, «Título ·
// T1E4 Nombre del episodio». Devuelve ErrNotFound si ningún título ni
// episodio lo tiene fichado —un archivo en cuarentena que no llegó a
// catalogarse—, y quien pregunta se queda con el nombre del archivo.
func (r *TitleRepo) NombreDelArchivo(ctx context.Context, assetID int64) (string, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT t.nombre, COALESCE(e.temporada, 0), COALESCE(e.numero, 0), COALESCE(e.nombre, '')
		FROM title t
		LEFT JOIN episode e ON e.title_id = t.id AND e.media_asset_id = ?
		WHERE t.media_asset_id = ? OR e.media_asset_id = ?
		ORDER BY t.id LIMIT 1`, assetID, assetID, assetID)
	var titulo, epNombre string
	var temporada, numero int
	err := row.Scan(&titulo, &temporada, &numero, &epNombre)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("archivo %d: %w", assetID, ErrNotFound)
	}
	if err != nil {
		return "", translate("buscar el nombre del archivo", err)
	}
	if numero == 0 {
		return titulo, nil
	}
	out := fmt.Sprintf("%s · T%dE%d", titulo, temporada, numero)
	if epNombre != "" {
		out += " " + epNombre
	}
	return out, nil
}

// ── los presets de preparación (esquema v10) ──────────────────────────

// PresetRepo lee y escribe los presets: cómo quiere el dueño del canal que
// suene y se vea su material.
type PresetRepo struct{ db *sql.DB }

const presetCols = `id, channel_id, nombre, ajustes, creado`

func scanPreset(sc interface{ Scan(...any) error }) (model.Preset, error) {
	var p model.Preset
	var canal sql.NullInt64
	var creado string
	if err := sc.Scan(&p.ID, &canal, &p.Name, &p.Settings, &creado); err != nil {
		return model.Preset{}, err
	}
	p.ChannelID = ptrInt64(canal)
	p.Created, _ = time.Parse(time.RFC3339, creado)
	return p, nil
}

// List devuelve los presets del canal, por nombre.
func (r *PresetRepo) List(ctx context.Context, channelID int64) ([]model.Preset, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+presetCols+`
		FROM preset WHERE channel_id IS NULL OR channel_id = ? ORDER BY nombre`, channelID)
	if err != nil {
		return nil, translate("listar los presets", err)
	}
	defer func() { _ = rows.Close() }()
	out := []model.Preset{}
	for rows.Next() {
		p, err := scanPreset(rows)
		if err != nil {
			return nil, translate("listar los presets", err)
		}
		out = append(out, p)
	}
	return out, translate("listar los presets", rows.Err())
}

// Get busca un preset por id.
func (r *PresetRepo) Get(ctx context.Context, id int64) (model.Preset, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+presetCols+` FROM preset WHERE id = ?`, id)
	p, err := scanPreset(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Preset{}, fmt.Errorf("preset %d: %w", id, ErrNotFound)
	}
	return p, translate("leer el preset", err)
}

// Insert crea un preset y deja el id en p.
func (r *PresetRepo) Insert(ctx context.Context, p *model.Preset) error {
	if p == nil {
		return errors.New("hace falta el preset")
	}
	if p.Created.IsZero() {
		p.Created = time.Now().UTC()
	}
	if strings.TrimSpace(p.Settings) == "" {
		p.Settings = "{}"
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO preset (channel_id, nombre, ajustes, creado) VALUES (?, ?, ?, ?)`,
		nullInt64(p.ChannelID), p.Name, p.Settings, p.Created.Format(time.RFC3339))
	if err != nil {
		return translate("guardar el preset", err)
	}
	p.ID, _ = res.LastInsertId()
	return nil
}

// Update guarda los cambios de un preset.
func (r *PresetRepo) Update(ctx context.Context, p model.Preset) error {
	if strings.TrimSpace(p.Settings) == "" {
		p.Settings = "{}"
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE preset SET nombre = ?, ajustes = ? WHERE id = ?`, p.Name, p.Settings, p.ID)
	if err != nil {
		return translate("guardar el preset", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("preset %d: %w", p.ID, ErrNotFound)
	}
	return nil
}

// Delete borra un preset y suelta a quien lo estuviera usando: un archivo o
// un título que se quede sin preset **hereda del nivel de arriba**, que es lo
// que ya pasaba antes de que ese preset existiera. Borrar no puede dejar nada
// apuntando a un preset que ya no está.
func (r *PresetRepo) Delete(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return translate("borrar el preset", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, q := range []string{
		`UPDATE channel     SET preset_id = NULL WHERE preset_id = ?`,
		`UPDATE title       SET preset_id = NULL WHERE preset_id = ?`,
		`UPDATE media_asset SET preset_id = NULL WHERE preset_id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, q, id); err != nil {
			return translate("borrar el preset", err)
		}
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM preset WHERE id = ?`, id)
	if err != nil {
		return translate("borrar el preset", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("preset %d: %w", id, ErrNotFound)
	}
	return tx.Commit()
}

// EnUso dice a cuántas cosas se les está aplicando este preset. Es lo que la
// pantalla enseña antes de dejar borrarlo: «esto lo usan 12 programas».
func (r *PresetRepo) EnUso(ctx context.Context, id int64) (canal, titulos, archivos int, err error) {
	q := `SELECT
		(SELECT COUNT(*) FROM channel     WHERE preset_id = ?),
		(SELECT COUNT(*) FROM title       WHERE preset_id = ?),
		(SELECT COUNT(*) FROM media_asset WHERE preset_id = ?)`
	err = r.db.QueryRowContext(ctx, q, id, id, id).Scan(&canal, &titulos, &archivos)
	return canal, titulos, archivos, translate("contar dónde se usa el preset", err)
}

// ── la configuración de los drivers, con sus credenciales ─────────────

// DriverConfigRepo guarda cómo se conecta el canal a algo de fuera. Es la
// única parte del store que cifra: las credenciales entran y salen en claro
// por esta puerta, y en la base no hay más que bytes.
type DriverConfigRepo struct {
	db  *sql.DB
	sec *Secretos
}

const driverConfigCols = `id, channel_id, tipo, driver, parametros, credenciales`

// List devuelve las configuraciones de un tipo. **No descifra nada**: para
// listar no hace falta la clave, y no sacarla de la base es la forma más
// barata de que no se escape por un log o por una respuesta de la API.
func (r *DriverConfigRepo) List(ctx context.Context, channelID int64, tipo string) ([]model.DriverConfig, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+driverConfigCols+`
		FROM driver_config
		WHERE (channel_id IS NULL OR channel_id = ?) AND (? = '' OR tipo = ?)
		ORDER BY tipo, driver`, channelID, tipo, tipo)
	if err != nil {
		return nil, translate("listar las conexiones", err)
	}
	defer func() { _ = rows.Close() }()
	out := []model.DriverConfig{}
	for rows.Next() {
		c, _, err := scanDriverConfig(rows)
		if err != nil {
			return nil, translate("listar las conexiones", err)
		}
		out = append(out, c)
	}
	return out, translate("listar las conexiones", rows.Err())
}

// Get devuelve una configuración **con su credencial ya descifrada**. Es la
// única función que lo hace, y la llama el driver justo antes de conectar:
// cuanto menos viaje el secreto, mejor.
func (r *DriverConfigRepo) Get(ctx context.Context, id int64) (model.DriverConfig, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+driverConfigCols+` FROM driver_config WHERE id = ?`, id)
	c, cifrado, err := scanDriverConfig(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.DriverConfig{}, fmt.Errorf("conexión %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return model.DriverConfig{}, translate("leer la conexión", err)
	}
	if len(cifrado) > 0 {
		claro, err := r.sec.Descifrar(cifrado)
		if err != nil {
			// Que no se pueda leer la clave NO tumba la fila: quien llama
			// decide si puede seguir sin ella o si tiene que avisar.
			return c, fmt.Errorf("la conexión %q existe pero su clave no se pudo leer: %w", c.Driver, err)
		}
		c.Secret = claro
	}
	return c, nil
}

// Upsert guarda una configuración. `secreto` vacío **no borra** el que ya
// había: eso deja que la pantalla mande el formulario entero sin tener que
// reenviar la clave cada vez, que es cómo se filtran las claves. Para quitarla
// está BorrarSecreto.
func (r *DriverConfigRepo) Upsert(ctx context.Context, c *model.DriverConfig, secreto string) error {
	if c == nil {
		return errors.New("hace falta la conexión")
	}
	if strings.TrimSpace(c.Params) == "" {
		c.Params = "{}"
	}
	var cifrado []byte
	if secreto != "" {
		var err error
		if cifrado, err = r.sec.Cifrar(secreto); err != nil {
			return fmt.Errorf("no se pudo guardar la clave: %w", err)
		}
	}
	if c.ID == 0 {
		res, err := r.db.ExecContext(ctx, `
			INSERT INTO driver_config (channel_id, tipo, driver, parametros, credenciales)
			VALUES (?, ?, ?, ?, ?)`,
			nullInt64(c.ChannelID), c.Kind, c.Driver, c.Params, cifrado)
		if err != nil {
			return translate("guardar la conexión", err)
		}
		c.ID, _ = res.LastInsertId()
		return nil
	}
	q := `UPDATE driver_config SET channel_id = ?, tipo = ?, driver = ?, parametros = ?`
	args := []any{nullInt64(c.ChannelID), c.Kind, c.Driver, c.Params}
	if cifrado != nil {
		q += `, credenciales = ?`
		args = append(args, cifrado)
	}
	q += ` WHERE id = ?`
	args = append(args, c.ID)
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return translate("guardar la conexión", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("conexión %d: %w", c.ID, ErrNotFound)
	}
	return nil
}

// BorrarSecreto quita la clave y deja la conexión. Es lo que hace falta cuando
// un proveedor deja de pedir contraseña, y la única forma de vaciarla a
// propósito: Upsert con el secreto vacío conserva el que había.
func (r *DriverConfigRepo) BorrarSecreto(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE driver_config SET credenciales = NULL WHERE id = ?`, id)
	return translate("quitar la clave", err)
}

// Delete borra una conexión entera.
func (r *DriverConfigRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM driver_config WHERE id = ?`, id)
	if err != nil {
		return translate("borrar la conexión", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("conexión %d: %w", id, ErrNotFound)
	}
	return nil
}

func scanDriverConfig(sc interface{ Scan(...any) error }) (model.DriverConfig, []byte, error) {
	var c model.DriverConfig
	var canal sql.NullInt64
	var cred []byte
	if err := sc.Scan(&c.ID, &canal, &c.Kind, &c.Driver, &c.Params, &cred); err != nil {
		return model.DriverConfig{}, nil, err
	}
	c.ChannelID = ptrInt64(canal)
	c.TieneSecreto = len(cred) > 0
	return c, cred, nil
}
