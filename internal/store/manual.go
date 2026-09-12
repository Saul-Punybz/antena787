// manual.go es la retención manual: el rato en que una persona le quita el
// aire al plan (PRD §9 paso 6, tanda T5).
//
// La tabla existía en el esquema desde el primer día y **no la tocaba ni una
// línea de código** — la auditoría del 12 de septiembre de 2026 la contó
// entre los huérfanos. Esto es lo que la alcanza.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"antena787/internal/model"
)

// ErrOtroTieneElControl dice que la retención no se pudo abrir porque ya hay
// una. Lleva dentro quién y desde cuándo, porque lo primero que pregunta la
// segunda persona es exactamente eso (F2-77).
type ErrOtroTieneElControl struct {
	Quien string
	Desde time.Time
}

func (e *ErrOtroTieneElControl) Error() string {
	quien := e.Quien
	if quien == "" {
		quien = "alguien"
	}
	return fmt.Sprintf("%s tiene el control desde las %s", quien, e.Desde.Format("3:04 PM"))
}

// ManualHoldRepo lee y escribe las retenciones manuales.
type ManualHoldRepo struct{ db *sql.DB }

const columnasDeRetencion = `id, channel_id, inicio_ms, fin_ms, motivo_fin, usuario`

func scanRetencion(sc interface{ Scan(...any) error }) (model.ManualHold, error) {
	var (
		m      model.ManualHold
		inicio int64
		fin    sql.NullInt64
		motivo sql.NullString
	)
	if err := sc.Scan(&m.ID, &m.ChannelID, &inicio, &fin, &motivo, &m.User); err != nil {
		return model.ManualHold{}, err
	}
	m.Start = time.UnixMilli(inicio).UTC()
	if fin.Valid {
		t := time.UnixMilli(fin.Int64).UTC()
		m.End = &t
	}
	m.EndReason = motivo.String
	return m, nil
}

// Abierta devuelve la retención en pie, si la hay.
func (r *ManualHoldRepo) Abierta(ctx context.Context, channelID int64) (model.ManualHold, bool, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+columnasDeRetencion+` FROM manual_hold
		 WHERE channel_id = ? AND fin_ms IS NULL ORDER BY id DESC LIMIT 1`, channelID)
	m, err := scanRetencion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ManualHold{}, false, nil
	}
	if err != nil {
		return model.ManualHold{}, false, translate("leer el control manual", err)
	}
	return m, true, nil
}

// Tomar abre una retención a nombre de alguien.
//
// La comprobación de que no hay otra y la inserción van **en la misma
// transacción**, y no por gusto: F2-77 exige que nunca haya dos tenedores a
// la vez, y dos pestañas pulsando el botón con un milisegundo de diferencia
// es justo el caso que lo rompería si se comprobara antes y se insertara
// después.
func (r *ManualHoldRepo) Tomar(ctx context.Context, channelID int64, usuario string, ahora time.Time) (model.ManualHold, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ManualHold{}, translate("tomar el control", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx,
		`SELECT `+columnasDeRetencion+` FROM manual_hold
		 WHERE channel_id = ? AND fin_ms IS NULL ORDER BY id DESC LIMIT 1`, channelID)
	otra, err := scanRetencion(row)
	switch {
	case err == nil && otra.User == usuario:
		// La misma persona pidiéndolo otra vez: ya lo tiene. Devolver el
		// error de «lo tiene otro» aquí sería decirle a alguien que el aire
		// lo tiene él mismo, que es lo que contestaba antes de que esto se
		// probara contra el binario. Un doble clic no puede leerse así.
		return otra, nil
	case err == nil:
		return model.ManualHold{}, &ErrOtroTieneElControl{Quien: otra.User, Desde: otra.Start}
	case !errors.Is(err, sql.ErrNoRows):
		return model.ManualHold{}, translate("tomar el control", err)
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO manual_hold (channel_id, inicio_ms, usuario) VALUES (?, ?, ?)`,
		channelID, ahora.UTC().UnixMilli(), usuario)
	if err != nil {
		return model.ManualHold{}, translate("tomar el control", err)
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return model.ManualHold{}, translate("tomar el control", err)
	}
	return model.ManualHold{ID: id, ChannelID: channelID, Start: ahora.UTC(), User: usuario}, nil
}

// Quitar cierra la retención que hubiera con el motivo dado y abre una nueva
// a nombre de quien se la quita, todo de una vez. Devuelve a quién se la
// quitó —vacío si no había nadie—, que es lo que hay que anotar en la
// auditoría y decirle a la persona.
func (r *ManualHoldRepo) Quitar(ctx context.Context, channelID int64, usuario string, ahora time.Time) (model.ManualHold, string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ManualHold{}, "", translate("quitar el control", err)
	}
	defer func() { _ = tx.Rollback() }()

	var aQuien string
	row := tx.QueryRowContext(ctx,
		`SELECT `+columnasDeRetencion+` FROM manual_hold
		 WHERE channel_id = ? AND fin_ms IS NULL ORDER BY id DESC LIMIT 1`, channelID)
	otra, err := scanRetencion(row)
	switch {
	case err == nil:
		aQuien = otra.User
		if _, err := tx.ExecContext(ctx,
			`UPDATE manual_hold SET fin_ms = ?, motivo_fin = ? WHERE id = ?`,
			ahora.UTC().UnixMilli(), model.FinQuitado, otra.ID); err != nil {
			return model.ManualHold{}, "", translate("quitar el control", err)
		}
	case !errors.Is(err, sql.ErrNoRows):
		return model.ManualHold{}, "", translate("quitar el control", err)
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO manual_hold (channel_id, inicio_ms, usuario) VALUES (?, ?, ?)`,
		channelID, ahora.UTC().UnixMilli(), usuario)
	if err != nil {
		return model.ManualHold{}, "", translate("quitar el control", err)
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return model.ManualHold{}, "", translate("quitar el control", err)
	}
	return model.ManualHold{ID: id, ChannelID: channelID, Start: ahora.UTC(), User: usuario}, aQuien, nil
}

// Cerrar acaba una retención con su motivo. Cerrar una ya cerrada no hace
// nada: el primer motivo es el bueno, y el segundo llegaría de una carrera
// —el timeout y el botón a la vez— donde la verdad es lo que pasó primero.
func (r *ManualHoldRepo) Cerrar(ctx context.Context, id int64, fin time.Time, motivo string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE manual_hold SET fin_ms = ?, motivo_fin = ? WHERE id = ? AND fin_ms IS NULL`,
		fin.UTC().UnixMilli(), motivo, id)
	return translate("soltar el control", err)
}

// CerrarLasQueQuedaron cierra las retenciones que se quedaron abiertas, que
// solo puede pasar de una forma: el servicio se cayó con una persona al
// mando. Se cierra con `caida_del_sistema` y no con `soltado`, porque nadie
// soltó nada — y un registro que dijera lo contrario mentiría sobre lo que
// pasó esa noche (F2-79).
func (r *ManualHoldRepo) CerrarLasQueQuedaron(ctx context.Context, channelID int64, fin time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE manual_hold SET fin_ms = ?, motivo_fin = ?
		 WHERE channel_id = ? AND fin_ms IS NULL`,
		fin.UTC().UnixMilli(), model.FinCaidaDelSistema, channelID)
	if err != nil {
		return 0, translate("cerrar el control manual que quedó abierto", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// Ultimas devuelve las retenciones más recientes, la más nueva primero. Es lo
// que enseña el historial: quién tuvo el aire, cuándo y cómo lo soltó.
func (r *ManualHoldRepo) Ultimas(ctx context.Context, channelID int64, limite int) ([]model.ManualHold, error) {
	if limite <= 0 {
		limite = 20
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+columnasDeRetencion+` FROM manual_hold
		 WHERE channel_id = ? ORDER BY id DESC LIMIT ?`, channelID, limite)
	if err != nil {
		return nil, translate("leer el historial del control manual", err)
	}
	defer func() { _ = rows.Close() }()

	out := []model.ManualHold{}
	for rows.Next() {
		m, err := scanRetencion(rows)
		if err != nil {
			return nil, translate("leer el historial del control manual", err)
		}
		out = append(out, m)
	}
	return out, translate("leer el historial del control manual", rows.Err())
}
