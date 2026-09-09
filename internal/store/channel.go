package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"antena787/internal/model"
)

// ── el canal ──────────────────────────────────────────────────────────

// ChannelRepo lee y escribe el canal. Mientras haya uno solo, su id es
// DefaultChannelID.
type ChannelRepo struct{ db *sql.DB }

const channelCols = `id, nombre, tipo, perfil_de_formato, perfil_regulatorio, modo,
	zona_horaria, hora_inicio_dia_emision, carga_maxima_por_hora,
	identificativo, comunidad_licencia, clase_licencia`

func scanChannel(sc interface{ Scan(...any) error }) (model.Channel, error) {
	var c model.Channel
	var minutos int64
	err := sc.Scan(&c.ID, &c.Name, &c.Kind, &c.FormatProfile, &c.RegProfile, &c.Mode,
		&c.TimeZone, &minutos, &c.MaxLoadPerHour,
		&c.CallSign, &c.LicenseCity, &c.LicenseClass)
	if err != nil {
		return model.Channel{}, err
	}
	c.BroadcastDayAt = model.Minutes(minutos)
	return c, nil
}

// Get devuelve el canal por su id.
func (r *ChannelRepo) Get(ctx context.Context, id int64) (model.Channel, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+channelCols+` FROM channel WHERE id = ?`, id)
	c, err := scanChannel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Channel{}, fmt.Errorf("canal %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return model.Channel{}, translate("leer el canal", err)
	}
	return c, nil
}

// List devuelve todos los canales, en orden de id.
func (r *ChannelRepo) List(ctx context.Context) ([]model.Channel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+channelCols+` FROM channel ORDER BY id`)
	if err != nil {
		return nil, translate("listar los canales", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.Channel
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, translate("listar los canales", err)
		}
		out = append(out, c)
	}
	return out, translate("listar los canales", rows.Err())
}

// Update guarda los cambios del canal. El id tiene que existir.
func (r *ChannelRepo) Update(ctx context.Context, c model.Channel) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE channel SET nombre = ?, tipo = ?, perfil_de_formato = ?,
			perfil_regulatorio = ?, modo = ?, zona_horaria = ?,
			hora_inicio_dia_emision = ?, carga_maxima_por_hora = ?,
			identificativo = ?, comunidad_licencia = ?, clase_licencia = ?
		WHERE id = ?`,
		c.Name, string(c.Kind), c.FormatProfile, c.RegProfile, c.Mode, c.TimeZone,
		int64(c.BroadcastDayAt), c.MaxLoadPerHour,
		c.CallSign, c.LicenseCity, c.LicenseClass, c.ID)
	if err != nil {
		return translate("guardar el canal", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("canal %d: %w", c.ID, ErrNotFound)
	}
	return nil
}

// ── las salidas ───────────────────────────────────────────────────────

// OutputRepo son las salidas del canal (§10).
type OutputRepo struct{ db *sql.DB }

const outputCols = `id, channel_id, nombre, driver, parametros, objetivo_volumen,
	estado_conexion, reintentos, ultimo_error`

func scanOutput(sc interface{ Scan(...any) error }) (model.Output, error) {
	var o model.Output
	err := sc.Scan(&o.ID, &o.ChannelID, &o.Name, &o.Driver, &o.Params, &o.TargetLoudness,
		&o.ConnectionState, &o.Retries, &o.LastError)
	return o, err
}

// List devuelve las salidas de un canal.
func (r *OutputRepo) List(ctx context.Context, channelID int64) ([]model.Output, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+outputCols+` FROM output WHERE channel_id = ? ORDER BY id`, channelID)
	if err != nil {
		return nil, translate("listar las salidas", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.Output
	for rows.Next() {
		o, err := scanOutput(rows)
		if err != nil {
			return nil, translate("listar las salidas", err)
		}
		out = append(out, o)
	}
	return out, translate("listar las salidas", rows.Err())
}

// Upsert inserta la salida si no tiene id y la actualiza si lo tiene. Al
// insertar deja el id nuevo en o.
func (r *OutputRepo) Upsert(ctx context.Context, o *model.Output) error {
	if o == nil {
		return errors.New("hace falta la salida")
	}
	if o.Params == "" {
		o.Params = "{}"
	}
	if o.ID == 0 {
		res, err := r.db.ExecContext(ctx, `
			INSERT INTO output (channel_id, nombre, driver, parametros, objetivo_volumen,
				estado_conexion, reintentos, ultimo_error)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			o.ChannelID, o.Name, o.Driver, o.Params, o.TargetLoudness,
			o.ConnectionState, o.Retries, o.LastError)
		if err != nil {
			return translate("guardar la salida", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return translate("guardar la salida", err)
		}
		o.ID = id
		return nil
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE output SET channel_id = ?, nombre = ?, driver = ?, parametros = ?,
			objetivo_volumen = ?, estado_conexion = ?, reintentos = ?, ultimo_error = ?
		WHERE id = ?`,
		o.ChannelID, o.Name, o.Driver, o.Params, o.TargetLoudness,
		o.ConnectionState, o.Retries, o.LastError, o.ID)
	if err != nil {
		return translate("guardar la salida", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("salida %d: %w", o.ID, ErrNotFound)
	}
	return nil
}

// ── los decks ─────────────────────────────────────────────────────────

// DeckRepo son las cuatro capas del plan: manual, comercial, programa y
// relleno, en ese orden de prioridad.
type DeckRepo struct{ db *sql.DB }

// ByKind devuelve el deck de un canal por su tipo.
func (r *DeckRepo) ByKind(ctx context.Context, channelID int64, kind model.DeckKind) (model.Deck, error) {
	var d model.Deck
	err := r.db.QueryRowContext(ctx,
		`SELECT id, channel_id, tipo, prioridad FROM deck WHERE channel_id = ? AND tipo = ?`,
		channelID, string(kind)).Scan(&d.ID, &d.ChannelID, &d.Kind, &d.Priority)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Deck{}, fmt.Errorf("deck %q del canal %d: %w", kind, channelID, ErrNotFound)
	}
	if err != nil {
		return model.Deck{}, translate("leer el deck", err)
	}
	return d, nil
}
