package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"antena787/internal/model"
)

// PlanRepo es el plan y el as-run a la vez: lo planeado se queda como está y
// lo real se llena encima (PRD §15).
type PlanRepo struct{ db *sql.DB }

const planCols = `id, channel_id, deck_id, schedule_rule_id, dia_emision,
	instante_planeado_ms, duracion_planeada_ms, instante_real_ms, duracion_real_ms,
	origen, media_asset_id, episode_id, live_source_id, dentro_de, corte_id,
	estado, parcial, cued_en_ms, error, hora_local`

func scanPlanItem(sc interface{ Scan(...any) error }) (model.PlanItem, error) {
	var p model.PlanItem
	var (
		regla, asset, episodio, vivo, dentro, corte sql.NullInt64
		realMs, duracionReal, cuedMs                sql.NullInt64
		planeado                                    int64
	)
	err := sc.Scan(&p.ID, &p.ChannelID, &p.DeckID, &regla, &p.BroadcastDay,
		&planeado, &p.PlannedMs, &realMs, &duracionReal,
		&p.Origin, &asset, &episodio, &vivo, &dentro, &corte,
		&p.State, &p.Partial, &cuedMs, &p.Error, &p.LocalClock)
	if err != nil {
		return model.PlanItem{}, err
	}
	p.PlannedAt = model.FromMs(planeado)
	p.RuleID = ptrInt64(regla)
	p.ActualAt = ptrTime(realMs)
	p.ActualMs = ptrInt64(duracionReal)
	p.MediaAssetID = ptrInt64(asset)
	p.EpisodeID = ptrInt64(episodio)
	p.LiveSourceID = ptrInt64(vivo)
	p.Inside = ptrInt64(dentro)
	p.BreakID = ptrInt64(corte)
	p.CuedAt = ptrTime(cuedMs)
	return p, nil
}

const planInsert = `
	INSERT INTO plan_item (channel_id, deck_id, schedule_rule_id, dia_emision,
		instante_planeado_ms, duracion_planeada_ms, instante_real_ms, duracion_real_ms,
		origen, media_asset_id, episode_id, live_source_id, dentro_de, corte_id,
		estado, parcial, cued_en_ms, error, hora_local)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func planArgs(p *model.PlanItem) []any {
	return []any{
		p.ChannelID, p.DeckID, nullInt64(p.RuleID), string(p.BroadcastDay),
		model.Ms(p.PlannedAt), p.PlannedMs, nullMs(p.ActualAt), nullInt64(p.ActualMs),
		p.Origin, nullInt64(p.MediaAssetID), nullInt64(p.EpisodeID),
		nullInt64(p.LiveSourceID), nullInt64(p.Inside), nullInt64(p.BreakID),
		string(p.State), p.Partial, nullMs(p.CuedAt), p.Error, p.LocalClock,
	}
}

// Insert mete un ítem y le pone el id. Si pisa a otro del mismo deck devuelve
// ErrOverlap: lo rechaza el trigger del esquema, no un if.
func (r *PlanRepo) Insert(ctx context.Context, p *model.PlanItem) error {
	if p == nil {
		return errors.New("hace falta el ítem del plan")
	}
	prepararItem(p)
	res, err := r.db.ExecContext(ctx, planInsert, planArgs(p)...)
	if err != nil {
		return translate("guardar el ítem del plan", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return translate("guardar el ítem del plan", err)
	}
	p.ID = id
	return nil
}

// InsertBatch mete un día entero de una vez, en una transacción: o entra todo
// el plan o no entra nada. Es lo que usa el resolver.
func (r *PlanRepo) InsertBatch(ctx context.Context, items []*model.PlanItem) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return translate("guardar el plan", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, planInsert)
	if err != nil {
		return translate("guardar el plan", err)
	}
	defer func() { _ = stmt.Close() }()

	ids := make([]int64, len(items))
	for i, p := range items {
		if p == nil {
			return errors.New("hay un ítem vacío en el plan")
		}
		prepararItem(p)
		res, err := stmt.ExecContext(ctx, planArgs(p)...)
		if err != nil {
			return translate("guardar el plan", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return translate("guardar el plan", err)
		}
		ids[i] = id
	}
	if err := tx.Commit(); err != nil {
		return translate("guardar el plan", err)
	}
	// Los ids se reparten después del commit: si algo falló, nadie se queda
	// con un id que no existe.
	for i, p := range items {
		p.ID = ids[i]
	}
	return nil
}

// prepararItem pone los valores por defecto que la base espera.
func prepararItem(p *model.PlanItem) {
	if p.State == "" {
		p.State = model.Planned
	}
	if p.Origin == "" {
		p.Origin = "asset"
	}
}

// ListDay devuelve el plan de un día de emisión, en orden de reloj.
func (r *PlanRepo) ListDay(ctx context.Context, channelID int64, day model.Day) ([]model.PlanItem, error) {
	return r.list(ctx, `
		SELECT `+planCols+` FROM plan_item
		WHERE channel_id = ? AND dia_emision = ?
		ORDER BY instante_planeado_ms, deck_id, id`, channelID, string(day))
}

// ListRange devuelve todo lo que toca el intervalo [from, to): también lo que
// empezó antes y sigue sonando dentro.
func (r *PlanRepo) ListRange(ctx context.Context, channelID int64, from, to time.Time) ([]model.PlanItem, error) {
	return r.list(ctx, `
		SELECT `+planCols+` FROM plan_item
		WHERE channel_id = ?
		  AND instante_planeado_ms < ?
		  AND instante_planeado_ms + duracion_planeada_ms > ?
		ORDER BY instante_planeado_ms, deck_id, id`,
		channelID, model.Ms(to), model.Ms(from))
}

// DeleteFuturePlanned borra lo que todavía no ha pasado por el aire: solo
// estado planned, de from en adelante. Nunca toca cued ni aired — lo que ya
// salió o está cargado no se reescribe (ADR 0009).
func (r *PlanRepo) DeleteFuturePlanned(ctx context.Context, channelID int64, from time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM plan_item
		WHERE channel_id = ? AND estado = 'planned' AND instante_planeado_ms >= ?`,
		channelID, model.Ms(from))
	if err != nil {
		return 0, translate("borrar el plan futuro", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, translate("borrar el plan futuro", err)
	}
	return n, nil
}

// SetState mueve un ítem de estado. Al pasar a cued deja la hora de carga.
func (r *PlanRepo) SetState(ctx context.Context, id int64, state model.PlanState) error {
	var res sql.Result
	var err error
	if state == model.Cued {
		res, err = r.db.ExecContext(ctx,
			`UPDATE plan_item SET estado = ?, cued_en_ms = ? WHERE id = ?`,
			string(state), model.Ms(time.Now().UTC()), id)
	} else {
		res, err = r.db.ExecContext(ctx,
			`UPDATE plan_item SET estado = ? WHERE id = ?`, string(state), id)
	}
	if err != nil {
		return translate("cambiar el estado del ítem", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("ítem del plan %d: %w", id, ErrNotFound)
	}
	return nil
}

// MarkAired anota lo que de verdad salió: instante y duración reales. Lo
// planeado se queda como estaba, que es de donde sale la exactitud de la guía.
func (r *PlanRepo) MarkAired(ctx context.Context, id int64, actualAt time.Time, actualMs int64, partial bool) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE plan_item SET estado = 'aired', instante_real_ms = ?, duracion_real_ms = ?,
			parcial = ? WHERE id = ?`,
		model.Ms(actualAt), actualMs, partial, id)
	if err != nil {
		return translate("anotar lo que salió al aire", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("ítem del plan %d: %w", id, ErrNotFound)
	}
	return nil
}

// ResetCuedToPlanned deshace las cargas a medias al arrancar: todo cued
// vuelve a planned y el motor entra por el minuto que le toca, con seek
// dentro del archivo — nunca reinicia el bloque (PRD §14.1). Devuelve
// cuántos ítems se devolvieron.
func (r *PlanRepo) ResetCuedToPlanned(ctx context.Context, channelID int64) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE plan_item SET estado = 'planned', cued_en_ms = NULL
		WHERE channel_id = ? AND estado = 'cued'`, channelID)
	if err != nil {
		return 0, translate("devolver los ítems cargados a planeados", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, translate("devolver los ítems cargados a planeados", err)
	}
	return n, nil
}

func (r *PlanRepo) list(ctx context.Context, consulta string, args ...any) ([]model.PlanItem, error) {
	rows, err := r.db.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, translate("listar el plan", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.PlanItem
	for rows.Next() {
		p, err := scanPlanItem(rows)
		if err != nil {
			return nil, translate("listar el plan", err)
		}
		out = append(out, p)
	}
	return out, translate("listar el plan", rows.Err())
}
