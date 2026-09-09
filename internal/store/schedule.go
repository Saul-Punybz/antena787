package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"antena787/internal/model"
)

// ScheduleRuleRepo son las reglas: la intención de una persona, no lo que
// saldrá al aire. Quien resuelve eso es el resolver.
type ScheduleRuleRepo struct{ db *sql.DB }

const ruleCols = `id, channel_id, tipo, title_id, live_source_id, patron_de_dias, hora,
	duracion_slot_ms, fecha_inicio, fecha_fin, ultimo_aviso_enviado,
	episodios_por_corrida, ultimo_episodio_emitido, releva_a, repite_a,
	advertiser_id, cobro, ventana_origen_inicio, ventana_origen_fin, activa`

func scanRule(sc interface{ Scan(...any) error }) (model.ScheduleRule, error) {
	var r model.ScheduleRule
	var (
		titulo, vivo, ultimoEp, relevaA, repiteA, anunciante sql.NullInt64
		ventanaIni, ventanaFin                               sql.NullInt64
		cobro                                                sql.NullFloat64
		hora                                                 int64
	)
	err := sc.Scan(&r.ID, &r.ChannelID, &r.Kind, &titulo, &vivo, &r.Days, &hora,
		&r.SlotMs, &r.From, &r.To, &r.LastNoticeSent, &r.EpisodesPerRun,
		&ultimoEp, &relevaA, &repiteA, &anunciante, &cobro,
		&ventanaIni, &ventanaFin, &r.Active)
	if err != nil {
		return model.ScheduleRule{}, err
	}
	r.At = model.Minutes(hora)
	r.TitleID = ptrInt64(titulo)
	r.LiveSourceID = ptrInt64(vivo)
	r.LastEpisodeAired = ptrInt64(ultimoEp)
	r.HandsOffTo = ptrInt64(relevaA)
	r.RepeatsOf = ptrInt64(repiteA)
	r.AdvertiserID = ptrInt64(anunciante)
	r.Fee = ptrFloat64(cobro)
	r.SourceWindowStart = ptrMinutes(ventanaIni)
	r.SourceWindowEnd = ptrMinutes(ventanaFin)
	return r, nil
}

// rulesErr distingue el CHECK de fechas del esquema del resto de errores.
// La regla de integridad vive en el esquema (PRD §15); aquí solo se dice en
// cristiano.
func rulesErr(op string, err error, r *model.ScheduleRule) error {
	if err != nil && r != nil && r.To < r.From &&
		strings.Contains(err.Error(), "CHECK constraint failed") {
		return fmt.Errorf("%s: %w", op, ErrDates)
	}
	return translate(op, err)
}

// Insert crea una regla y le pone el id. Si las fechas están al revés
// devuelve ErrDates: lo rechaza el esquema, no un if.
func (r *ScheduleRuleRepo) Insert(ctx context.Context, rule *model.ScheduleRule) error {
	if rule == nil {
		return errors.New("hace falta la regla")
	}
	if rule.Kind == "" {
		rule.Kind = model.RuleNormal
	}
	if rule.EpisodesPerRun == 0 {
		rule.EpisodesPerRun = 1
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO schedule_rule (channel_id, tipo, title_id, live_source_id,
			patron_de_dias, hora, duracion_slot_ms, fecha_inicio, fecha_fin,
			ultimo_aviso_enviado, episodios_por_corrida, ultimo_episodio_emitido,
			releva_a, repite_a, advertiser_id, cobro, ventana_origen_inicio,
			ventana_origen_fin, activa)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ChannelID, string(rule.Kind), nullInt64(rule.TitleID),
		nullInt64(rule.LiveSourceID), string(rule.Days), int64(rule.At), rule.SlotMs,
		string(rule.From), string(rule.To), rule.LastNoticeSent, rule.EpisodesPerRun,
		nullInt64(rule.LastEpisodeAired), nullInt64(rule.HandsOffTo),
		nullInt64(rule.RepeatsOf), nullInt64(rule.AdvertiserID), nullFloat64(rule.Fee),
		nullMinutes(rule.SourceWindowStart), nullMinutes(rule.SourceWindowEnd), rule.Active)
	if err != nil {
		return rulesErr("guardar la regla", err, rule)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return translate("guardar la regla", err)
	}
	rule.ID = id
	return nil
}

// Update guarda los cambios de una regla.
func (r *ScheduleRuleRepo) Update(ctx context.Context, rule *model.ScheduleRule) error {
	if rule == nil {
		return errors.New("hace falta la regla")
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE schedule_rule SET channel_id = ?, tipo = ?, title_id = ?, live_source_id = ?,
			patron_de_dias = ?, hora = ?, duracion_slot_ms = ?, fecha_inicio = ?,
			fecha_fin = ?, ultimo_aviso_enviado = ?, episodios_por_corrida = ?,
			ultimo_episodio_emitido = ?, releva_a = ?, repite_a = ?, advertiser_id = ?,
			cobro = ?, ventana_origen_inicio = ?, ventana_origen_fin = ?, activa = ?
		WHERE id = ?`,
		rule.ChannelID, string(rule.Kind), nullInt64(rule.TitleID),
		nullInt64(rule.LiveSourceID), string(rule.Days), int64(rule.At), rule.SlotMs,
		string(rule.From), string(rule.To), rule.LastNoticeSent, rule.EpisodesPerRun,
		nullInt64(rule.LastEpisodeAired), nullInt64(rule.HandsOffTo),
		nullInt64(rule.RepeatsOf), nullInt64(rule.AdvertiserID), nullFloat64(rule.Fee),
		nullMinutes(rule.SourceWindowStart), nullMinutes(rule.SourceWindowEnd),
		rule.Active, rule.ID)
	if err != nil {
		return rulesErr("guardar la regla", err, rule)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("regla %d: %w", rule.ID, ErrNotFound)
	}
	return nil
}

// Delete borra una regla. Los plan_item que la citan quedan con la
// referencia; no se borra nada del as-run.
func (r *ScheduleRuleRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM schedule_rule WHERE id = ?`, id)
	if err != nil {
		return translate("borrar la regla", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("regla %d: %w", id, ErrNotFound)
	}
	return nil
}

// Get busca una regla por id.
func (r *ScheduleRuleRepo) Get(ctx context.Context, id int64) (model.ScheduleRule, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+ruleCols+` FROM schedule_rule WHERE id = ?`, id)
	rule, err := scanRule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ScheduleRule{}, fmt.Errorf("regla %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return model.ScheduleRule{}, translate("leer la regla", err)
	}
	return rule, nil
}

// ListAll devuelve todas las reglas de un canal, activas o no.
func (r *ScheduleRuleRepo) ListAll(ctx context.Context, channelID int64) ([]model.ScheduleRule, error) {
	return r.list(ctx,
		`SELECT `+ruleCols+` FROM schedule_rule WHERE channel_id = ? ORDER BY hora, id`, channelID)
}

// ListActive devuelve las reglas que cubren un día de emisión: activas, con
// el día dentro de su vigencia y con ese día en su patrón semanal.
func (r *ScheduleRuleRepo) ListActive(ctx context.Context, channelID int64, day model.Day) ([]model.ScheduleRule, error) {
	reglas, err := r.list(ctx, `
		SELECT `+ruleCols+` FROM schedule_rule
		WHERE channel_id = ? AND activa = 1 AND fecha_inicio <= ? AND fecha_fin >= ?
		ORDER BY hora, id`, channelID, string(day), string(day))
	if err != nil {
		return nil, err
	}
	// El patrón semanal se filtra en Go: es el mismo Covers que usa el
	// resolver, así que no hay dos verdades.
	out := reglas[:0]
	for _, regla := range reglas {
		if regla.Covers(day) {
			out = append(out, regla)
		}
	}
	return out, nil
}

// SetLastEpisode apunta el último episodio emitido de una regla: el contador
// de la serie. Un solo contador por franja (PRD §15).
func (r *ScheduleRuleRepo) SetLastEpisode(ctx context.Context, ruleID, episodeID int64) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE schedule_rule SET ultimo_episodio_emitido = ? WHERE id = ?`, episodeID, ruleID)
	if err != nil {
		return translate("apuntar el último episodio", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("regla %d: %w", ruleID, ErrNotFound)
	}
	return nil
}

// SetLastNotice deja escrito el último aviso de vencimiento que se mandó
// ('30', '14', '7' o vacío), para no repetirlo.
func (r *ScheduleRuleRepo) SetLastNotice(ctx context.Context, ruleID int64, notice string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE schedule_rule SET ultimo_aviso_enviado = ? WHERE id = ?`, notice, ruleID)
	if err != nil {
		return translate("apuntar el aviso enviado", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("regla %d: %w", ruleID, ErrNotFound)
	}
	return nil
}

func (r *ScheduleRuleRepo) list(ctx context.Context, consulta string, args ...any) ([]model.ScheduleRule, error) {
	rows, err := r.db.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, translate("listar las reglas", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.ScheduleRule
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, translate("listar las reglas", err)
		}
		out = append(out, rule)
	}
	return out, translate("listar las reglas", rows.Err())
}
