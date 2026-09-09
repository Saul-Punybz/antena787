package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"antena787/internal/model"
)

// ── incidentes ────────────────────────────────────────────────────────

// IncidentRepo es lo que el sistema hizo solo y hay que poder explicar
// después: una caída, un vivo que no llegó, un apagón.
type IncidentRepo struct{ db *sql.DB }

const incidentCols = `id, channel_id, tipo, inicio_ms, fin_ms, detalle`

func scanIncident(sc interface{ Scan(...any) error }) (model.Incident, error) {
	var i model.Incident
	var inicio int64
	var fin sql.NullInt64
	if err := sc.Scan(&i.ID, &i.ChannelID, &i.Kind, &inicio, &fin, &i.Detail); err != nil {
		return model.Incident{}, err
	}
	i.Start = model.FromMs(inicio)
	i.End = ptrTime(fin)
	return i, nil
}

// Insert abre un incidente y le pone el id. Si no trae hora de inicio, ahora.
func (r *IncidentRepo) Insert(ctx context.Context, i *model.Incident) error {
	if i == nil {
		return errors.New("hace falta el incidente")
	}
	if i.Start.IsZero() {
		i.Start = time.Now().UTC()
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO incidente (channel_id, tipo, inicio_ms, fin_ms, detalle)
		VALUES (?, ?, ?, ?, ?)`,
		i.ChannelID, i.Kind, model.Ms(i.Start), nullMs(i.End), i.Detail)
	if err != nil {
		return translate("guardar el incidente", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return translate("guardar el incidente", err)
	}
	i.ID = id
	return nil
}

// List devuelve los incidentes que tocan el intervalo [from, to), incluidos
// los que siguen abiertos.
func (r *IncidentRepo) List(ctx context.Context, channelID int64, from, to time.Time) ([]model.Incident, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+incidentCols+` FROM incidente
		WHERE channel_id = ? AND inicio_ms < ? AND (fin_ms IS NULL OR fin_ms >= ?)
		ORDER BY inicio_ms, id`, channelID, model.Ms(to), model.Ms(from))
	if err != nil {
		return nil, translate("listar los incidentes", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.Incident
	for rows.Next() {
		i, err := scanIncident(rows)
		if err != nil {
			return nil, translate("listar los incidentes", err)
		}
		out = append(out, i)
	}
	return out, translate("listar los incidentes", rows.Err())
}

// Close cierra un incidente abierto con su hora de fin.
func (r *IncidentRepo) Close(ctx context.Context, id int64, end time.Time) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE incidente SET fin_ms = ? WHERE id = ?`, model.Ms(end), id)
	if err != nil {
		return translate("cerrar el incidente", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("incidente %d: %w", id, ErrNotFound)
	}
	return nil
}

// ── bitácora encadenada ───────────────────────────────────────────────

// AuditRepo es el audit_log: quién cambió qué y cuándo, encadenado por hash.
// Cada entrada lleva el hash de la anterior, así que editar una fila por
// detrás rompe la cadena y Verify lo dice.
type AuditRepo struct{ db *sql.DB }

const auditCols = `id, entidad, entidad_id, campo, valor_anterior, valor_nuevo,
	autor, origen, instante_ms, aplica_en, tipo, hash_prev, hash`

func scanAudit(sc interface{ Scan(...any) error }) (model.AuditEntry, error) {
	var e model.AuditEntry
	var entidadID sql.NullInt64
	var instante int64
	err := sc.Scan(&e.ID, &e.Entity, &entidadID, &e.Field, &e.Before, &e.After,
		&e.Author, &e.Origin, &instante, &e.AppliesAt, &e.Kind, &e.HashPrev, &e.Hash)
	if err != nil {
		return model.AuditEntry{}, err
	}
	e.EntityID = ptrInt64(entidadID)
	e.At = model.FromMs(instante)
	return e, nil
}

// auditHash es el eslabón: sha256 del hash anterior más los campos de la
// entrada, en un orden fijo y con un separador que no aparece en el texto.
func auditHash(prev string, e model.AuditEntry) string {
	entidadID := ""
	if e.EntityID != nil {
		entidadID = strconv.FormatInt(*e.EntityID, 10)
	}
	campos := strings.Join([]string{
		e.Entity, entidadID, e.Field, e.Before, e.After, e.Author, e.Origin,
		strconv.FormatInt(model.Ms(e.At), 10), e.AppliesAt, e.Kind,
	}, "\x1f")
	suma := sha256.Sum256([]byte(prev + "\x1e" + campos))
	return hex.EncodeToString(suma[:])
}

// Append añade una entrada al final de la cadena. Calcula hash_prev y hash y
// los deja en e. Va en una transacción para que dos escrituras no se lleven
// el mismo eslabón.
func (r *AuditRepo) Append(ctx context.Context, e *model.AuditEntry) error {
	if e == nil {
		return errors.New("hace falta la entrada de la bitácora")
	}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	if e.Origin == "" {
		e.Origin = "humano"
	}
	if e.AppliesAt == "" {
		e.AppliesAt = "inmediato"
	}
	if e.Kind == "" {
		e.Kind = "cambio"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return translate("anotar en la bitácora", err)
	}
	defer func() { _ = tx.Rollback() }()

	var prev string
	err = tx.QueryRowContext(ctx, `SELECT hash FROM audit_log ORDER BY id DESC LIMIT 1`).Scan(&prev)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return translate("anotar en la bitácora", err)
	}
	e.HashPrev = prev
	e.Hash = auditHash(prev, *e)

	res, err := tx.ExecContext(ctx, `
		INSERT INTO audit_log (entidad, entidad_id, campo, valor_anterior, valor_nuevo,
			autor, origen, instante_ms, aplica_en, tipo, hash_prev, hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Entity, nullInt64(e.EntityID), e.Field, e.Before, e.After, e.Author,
		e.Origin, model.Ms(e.At), e.AppliesAt, e.Kind, e.HashPrev, e.Hash)
	if err != nil {
		return translate("anotar en la bitácora", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return translate("anotar en la bitácora", err)
	}
	if err := tx.Commit(); err != nil {
		return translate("anotar en la bitácora", err)
	}
	e.ID = id
	return nil
}

// List devuelve las entradas en orden, de la más vieja a la más nueva.
func (r *AuditRepo) List(ctx context.Context, limit int) ([]model.AuditEntry, error) {
	consulta := `SELECT ` + auditCols + ` FROM audit_log ORDER BY id`
	args := []any{}
	if limit > 0 {
		consulta += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := r.db.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, translate("leer la bitácora", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.AuditEntry
	for rows.Next() {
		e, err := scanAudit(rows)
		if err != nil {
			return nil, translate("leer la bitácora", err)
		}
		out = append(out, e)
	}
	return out, translate("leer la bitácora", rows.Err())
}

// Verify recorre la bitácora entera y devuelve la primera entrada rota: la
// que no encadena con la anterior o cuyo hash no cuadra con sus campos. Si
// todo está bien devuelve nil.
func (r *AuditRepo) Verify(ctx context.Context) (*model.AuditEntry, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+auditCols+` FROM audit_log ORDER BY id`)
	if err != nil {
		return nil, translate("verificar la bitácora", err)
	}
	defer func() { _ = rows.Close() }()

	prev := ""
	for rows.Next() {
		e, err := scanAudit(rows)
		if err != nil {
			return nil, translate("verificar la bitácora", err)
		}
		if e.HashPrev != prev || e.Hash != auditHash(prev, e) {
			rota := e
			return &rota, nil
		}
		prev = e.Hash
	}
	if err := rows.Err(); err != nil {
		return nil, translate("verificar la bitácora", err)
	}
	return nil, nil
}

// ── ajustes ───────────────────────────────────────────────────────────

// Claves de settings que este paquete conoce. Toda la configuración vive en
// esta tabla; no hay archivo de configuración (PRD §14.1).
const (
	// KeyPINHash y KeyPINSalt guardan la clave de estación, nunca en claro.
	KeyPINHash = "clave_estacion.hash"
	KeyPINSalt = "clave_estacion.sal"
)

// SettingsRepo es la tabla clave/valor donde vive toda la configuración.
type SettingsRepo struct{ db *sql.DB }

// esSecreta dice si una clave no puede salir en un listado ni en un respaldo
// que salga de la máquina: la clave de estación y cualquier credencial.
func esSecreta(clave string) bool {
	c := strings.ToLower(clave)
	if strings.HasPrefix(c, "clave_estacion.") {
		return true
	}
	for _, marca := range []string{"secreto", "credencial", "contrasena", "contraseña", "token", "api_key"} {
		if strings.Contains(c, marca) {
			return true
		}
	}
	return false
}

// Get lee un ajuste. Si no está, devuelve ErrNotFound.
func (r *SettingsRepo) Get(ctx context.Context, key string) (string, error) {
	var v string
	err := r.db.QueryRowContext(ctx, `SELECT valor FROM settings WHERE clave = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("ajuste %q: %w", key, ErrNotFound)
	}
	if err != nil {
		return "", translate("leer el ajuste", err)
	}
	return v, nil
}

// Set guarda un ajuste, lo hubiera o no.
func (r *SettingsRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO settings (clave, valor) VALUES (?, ?)
		ON CONFLICT (clave) DO UPDATE SET valor = excluded.valor`, key, value)
	return translate("guardar el ajuste", err)
}

// GetAll devuelve todos los ajustes menos los secretos: esto es lo que puede
// enseñarse en pantalla, mandarse por la API o salir en un reporte.
func (r *SettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT clave, valor FROM settings ORDER BY clave`)
	if err != nil {
		return nil, translate("listar los ajustes", err)
	}
	defer func() { _ = rows.Close() }()

	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, translate("listar los ajustes", err)
		}
		if esSecreta(k) {
			continue
		}
		out[k] = v
	}
	return out, translate("listar los ajustes", rows.Err())
}

// SetPIN guarda la clave de estación: sal aleatoria de 16 bytes y sha256 de
// la sal más la clave. La clave en claro no se guarda nunca (PRD §19).
func (r *SettingsRepo) SetPIN(ctx context.Context, pin string) error {
	if strings.TrimSpace(pin) == "" {
		return errors.New("la clave de estación no puede estar vacía")
	}
	sal := make([]byte, 16)
	if _, err := rand.Read(sal); err != nil {
		return fmt.Errorf("no se pudo generar la sal de la clave: %w", err)
	}
	salHex := hex.EncodeToString(sal)
	if err := r.Set(ctx, KeyPINSalt, salHex); err != nil {
		return err
	}
	return r.Set(ctx, KeyPINHash, pinHash(salHex, pin))
}

// HasPIN dice si ya hay clave de estación puesta.
func (r *SettingsRepo) HasPIN(ctx context.Context) (bool, error) {
	_, err := r.Get(ctx, KeyPINHash)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CheckPIN compara una clave con la guardada. Si no hay ninguna puesta
// devuelve false sin error: todavía no hay nada que comparar.
func (r *SettingsRepo) CheckPIN(ctx context.Context, pin string) (bool, error) {
	guardado, err := r.Get(ctx, KeyPINHash)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	sal, err := r.Get(ctx, KeyPINSalt)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	// Comparación de tiempo constante: no se filtra por dónde falla.
	igual := subtle.ConstantTimeCompare([]byte(pinHash(sal, pin)), []byte(guardado)) == 1
	return igual, nil
}

func pinHash(salHex, pin string) string {
	suma := sha256.Sum256([]byte(salHex + ":" + pin))
	return hex.EncodeToString(suma[:])
}
