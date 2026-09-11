package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"antena787/internal/model"
)

// Emparejar títulos (F1-64 a F1-67).
//
// El importador nunca rechaza una hoja: si un nombre no cuadra con ninguna
// ficha del catálogo, la regla entra igual y el título queda **por
// emparejar**, con los candidatos que se le parecían. Aquí está lo que pasa
// después, cuando una persona decide:
//
//   - Emparejar: es esta ficha. Las reglas pasan a ella, el título
//     provisional desaparece y el nombre de la hoja queda guardado como
//     alias, para que la próxima hoja se empareje sola (F1-66).
//   - MarcarPropio: es un título nuevo de verdad, se queda como ficha
//     propia (F1-67).
//   - QuitarProvisional: no es un programa —el nombre de una fuente en
//     vivo, por ejemplo—: se van el título y las reglas que lo usaban,
//     diciendo cuántas (F1-67).
//
// Todo pasa en una sola transacción con su anotación en la bitácora: o entra
// todo, o no entra nada.

// formatoCreado es como se guarda title_alias.creado: una fecha que se lee
// sin herramientas y ordena sola.
const formatoCreado = time.RFC3339

// ── alias ─────────────────────────────────────────────────────────────

// AliasRepo es la memoria de los emparejamientos: cómo llama la hoja a cada
// ficha del catálogo. Se busca por la clave del nombre (model.ClaveDeNombre),
// así que «Samurai X», «SAMURAIX» y «samurai-x» encuentran lo mismo.
type AliasRepo struct{ db *sql.DB }

const aliasCols = `id, channel_id, alias, clave, title_id, creado`

func scanAlias(sc interface{ Scan(...any) error }) (model.TitleAlias, error) {
	var a model.TitleAlias
	var canal sql.NullInt64
	var creado string
	if err := sc.Scan(&a.ID, &canal, &a.Alias, &a.Clave, &a.TitleID, &creado); err != nil {
		return model.TitleAlias{}, err
	}
	a.ChannelID = ptrInt64(canal)
	a.Creado, _ = time.Parse(formatoCreado, creado)
	return a, nil
}

// Insert guarda un alias. Si ya había uno con la misma clave en ese canal, lo
// reapunta a la ficha nueva en vez de crear un segundo: aprender otra vez lo
// mismo no ensucia la tabla.
func (r *AliasRepo) Insert(ctx context.Context, a *model.TitleAlias) error {
	return insertarAlias(ctx, r.db, a)
}

// insertarAlias es Insert sin transacción propia: la pone quien llama, para
// que emparejar y recordar el alias sean una sola escritura.
func insertarAlias(ctx context.Context, db ejecutor, a *model.TitleAlias) error {
	if a == nil {
		return errors.New("hace falta el alias")
	}
	if a.Clave == "" {
		a.Clave = model.ClaveDeNombre(a.Alias)
	}
	if a.Clave == "" {
		return errors.New("ese nombre no deja nada que recordar: el alias no se guardó")
	}
	if a.Creado.IsZero() {
		a.Creado = time.Now().UTC()
	}

	// La clave se compara con IS para que un alias sin canal (channel_id
	// nulo) también sea único: en SQLite dos NULL no son iguales y la
	// restricción UNIQUE sola dejaría pasar repetidos.
	var id int64
	err := db.QueryRowContext(ctx,
		`SELECT id FROM title_alias WHERE clave = ? AND channel_id IS ?`,
		a.Clave, nullInt64(a.ChannelID)).Scan(&id)
	switch {
	case err == nil:
		if _, err := db.ExecContext(ctx,
			`UPDATE title_alias SET alias = ?, title_id = ?, creado = ? WHERE id = ?`,
			a.Alias, a.TitleID, a.Creado.UTC().Format(formatoCreado), id); err != nil {
			return translate("guardar el alias", err)
		}
		a.ID = id
		return nil
	case errors.Is(err, sql.ErrNoRows):
		// sigue
	default:
		return translate("guardar el alias", err)
	}

	res, err := db.ExecContext(ctx, `
		INSERT INTO title_alias (channel_id, alias, clave, title_id, creado)
		VALUES (?, ?, ?, ?, ?)`,
		nullInt64(a.ChannelID), a.Alias, a.Clave, a.TitleID,
		a.Creado.UTC().Format(formatoCreado))
	if err != nil {
		return translate("guardar el alias", err)
	}
	nuevo, err := res.LastInsertId()
	if err != nil {
		return translate("guardar el alias", err)
	}
	a.ID = nuevo
	return nil
}

// Resolve dice a qué ficha va un nombre de la hoja, si ya se aprendió. Busca
// por la clave del nombre, no por su ortografía; el alias del canal manda
// sobre el que vale para todos.
func (r *AliasRepo) Resolve(ctx context.Context, channelID int64, nombre string) (int64, bool, error) {
	clave := model.ClaveDeNombre(nombre)
	if clave == "" {
		return 0, false, nil
	}
	var titleID int64
	err := r.db.QueryRowContext(ctx, `
		SELECT title_id FROM title_alias
		WHERE clave = ? AND (channel_id = ? OR channel_id IS NULL)
		ORDER BY (channel_id IS NULL), id LIMIT 1`, clave, channelID).Scan(&titleID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, translate("buscar el alias", err)
	}
	return titleID, true, nil
}

// ListByChannel devuelve los alias de un canal más los que valen para todos.
func (r *AliasRepo) ListByChannel(ctx context.Context, channelID int64) ([]model.TitleAlias, error) {
	return r.listar(ctx, `
		SELECT `+aliasCols+` FROM title_alias
		WHERE channel_id = ? OR channel_id IS NULL
		ORDER BY alias, id`, channelID)
}

// ListByTitle devuelve todos los nombres con que la hoja llama a una ficha.
func (r *AliasRepo) ListByTitle(ctx context.Context, titleID int64) ([]model.TitleAlias, error) {
	return r.listar(ctx,
		`SELECT `+aliasCols+` FROM title_alias WHERE title_id = ? ORDER BY alias, id`, titleID)
}

// Delete borra un alias: la próxima hoja con ese nombre volverá a preguntar.
func (r *AliasRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM title_alias WHERE id = ?`, id)
	if err != nil {
		return translate("borrar el alias", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("alias %d: %w", id, ErrNotFound)
	}
	return nil
}

func (r *AliasRepo) listar(ctx context.Context, consulta string, args ...any) ([]model.TitleAlias, error) {
	rows, err := r.db.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, translate("listar los alias", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.TitleAlias
	for rows.Next() {
		a, err := scanAlias(rows)
		if err != nil {
			return nil, translate("listar los alias", err)
		}
		out = append(out, a)
	}
	return out, translate("listar los alias", rows.Err())
}

// ── los títulos por emparejar ─────────────────────────────────────────

// ListPendientes devuelve los títulos que el importador dejó por emparejar
// en ese canal (y los que no son de ningún canal), por nombre. Es la lista
// que sale en la pantalla de Reglas y la que hace salir el aviso en Al aire
// mientras quede alguno (F1-64).
func (r *TitleRepo) ListPendientes(ctx context.Context, channelID int64) ([]model.Title, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+titleCols+` FROM title
		WHERE pendiente_emparejar = 1 AND (channel_id = ? OR channel_id IS NULL)
		ORDER BY nombre`, channelID)
	if err != nil {
		return nil, translate("listar los títulos por emparejar", err)
	}
	defer func() { _ = rows.Close() }()

	var out []model.Title
	for rows.Next() {
		t, err := scanTitle(rows)
		if err != nil {
			return nil, translate("listar los títulos por emparejar", err)
		}
		out = append(out, t)
	}
	return out, translate("listar los títulos por emparejar", rows.Err())
}

// Emparejar dice que el título provisional es en realidad la ficha destino
// (F1-66). En una sola transacción:
//
//   - las reglas que usaban el provisional pasan a la ficha (y devuelve
//     cuántas);
//   - los episodios y los alias que colgaban del provisional pasan también;
//   - el nombre del provisional queda guardado como alias de la ficha, para
//     que la próxima hoja que lo traiga se empareje sola;
//   - el título provisional se borra;
//   - queda anotado en la bitácora.
//
// Si alguno de los dos ids no existe devuelve ErrNotFound; si son el mismo,
// ErrMismoTitulo; y si la ficha destino también está por emparejar,
// ErrDestinoPendiente: primero hay que resolver esa.
func (r *TitleRepo) Emparejar(ctx context.Context, provisionalID, destinoID int64) (int, error) {
	const op = "emparejar el título"
	if provisionalID == destinoID {
		return 0, fmt.Errorf("%s: %w", op, ErrMismoTitulo)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, translate(op, err)
	}
	defer func() { _ = tx.Rollback() }()

	provisional, err := tituloEnTx(ctx, tx, provisionalID, op)
	if err != nil {
		return 0, err
	}
	destino, err := tituloEnTx(ctx, tx, destinoID, op)
	if err != nil {
		return 0, err
	}
	if destino.pendiente {
		return 0, fmt.Errorf("%s: %w", op, ErrDestinoPendiente)
	}

	// Las reglas cambian de ficha: es lo único que ve el aire.
	res, err := tx.ExecContext(ctx,
		`UPDATE schedule_rule SET title_id = ? WHERE title_id = ?`, destinoID, provisionalID)
	if err != nil {
		return 0, translate(op, err)
	}
	movidas, _ := res.RowsAffected()

	// Los episodios se van con las reglas. Si la ficha destino ya tenía ese
	// mismo episodio (misma temporada y número), el del provisional se queda
	// atrás y se va con él: no se duplica un episodio.
	if _, err := tx.ExecContext(ctx,
		`UPDATE OR IGNORE episode SET title_id = ? WHERE title_id = ?`, destinoID, provisionalID); err != nil {
		return 0, translate(op, err)
	}

	// Los alias que ya apuntaban al provisional apuntan ahora a la ficha; si
	// la ficha ya tenía uno con esa misma clave, el del provisional sobra.
	if _, err := tx.ExecContext(ctx,
		`UPDATE OR IGNORE title_alias SET title_id = ? WHERE title_id = ?`, destinoID, provisionalID); err != nil {
		return 0, translate(op, err)
	}

	// Y el nombre de la hoja queda aprendido.
	alias := &model.TitleAlias{
		ChannelID: provisional.channelID, Alias: provisional.nombre, TitleID: destinoID,
	}
	if alias.ChannelID == nil {
		alias.ChannelID = destino.channelID
	}
	if model.ClaveDeNombre(alias.Alias) != "" {
		if err := insertarAlias(ctx, tx, alias); err != nil {
			return 0, err
		}
	}

	// El provisional ya no hace falta: lo que colgaba de él está movido.
	if _, err := tx.ExecContext(ctx, `DELETE FROM title WHERE id = ?`, provisionalID); err != nil {
		return 0, translate(op, err)
	}

	if err := r.anotar(ctx, tx, destinoID, "emparejado",
		provisional.nombre,
		fmt.Sprintf("emparejado con «%s»: %s", destino.nombre, reglasEnPalabras(int(movidas)))); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, translate(op, err)
	}
	return int(movidas), nil
}

// MarcarPropio dice que el título por emparejar es un programa nuevo de
// verdad: se queda como ficha propia y deja de estar por emparejar (F1-67).
// Si ya era una ficha propia no hace nada y no se queja.
func (r *TitleRepo) MarcarPropio(ctx context.Context, id int64) error {
	const op = "marcar el título como propio"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return translate(op, err)
	}
	defer func() { _ = tx.Rollback() }()

	t, err := tituloEnTx(ctx, tx, id, op)
	if err != nil {
		return err
	}
	if !t.pendiente {
		return nil
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE title SET pendiente_emparejar = 0, candidatos = '[]' WHERE id = ?`, id); err != nil {
		return translate(op, err)
	}
	if err := r.anotar(ctx, tx, id, "pendiente_emparejar",
		"por emparejar", fmt.Sprintf("es un título propio: «%s» se queda en el catálogo", t.nombre)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return translate(op, err)
	}
	return nil
}

// QuitarProvisional se lleva el título por emparejar y las reglas que lo
// usaban, y devuelve cuántas reglas se fueron (F1-67). Es lo que se hace con
// lo que no es un programa: el nombre de una fuente en vivo, por ejemplo.
//
// Solo vale para un título por emparejar: sobre una ficha del catálogo
// devuelve ErrNoPendiente, para que nadie se lleve por delante media
// programación con un clic.
func (r *TitleRepo) QuitarProvisional(ctx context.Context, id int64) (int, error) {
	const op = "quitar el título por emparejar"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, translate(op, err)
	}
	defer func() { _ = tx.Rollback() }()

	t, err := tituloEnTx(ctx, tx, id, op)
	if err != nil {
		return 0, err
	}
	if !t.pendiente {
		return 0, fmt.Errorf("%s: %w", op, ErrNoPendiente)
	}

	// Los ítems del plan citan la regla, y la regla cita el título: hay que
	// soltar de abajo hacia arriba o la llave foránea lo impide.
	if _, err := tx.ExecContext(ctx, `
		UPDATE plan_item SET dentro_de = NULL
		WHERE dentro_de IN (SELECT id FROM plan_item
		                    WHERE schedule_rule_id IN (SELECT id FROM schedule_rule WHERE title_id = ?))`,
		id); err != nil {
		return 0, translate(op, err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM plan_item
		WHERE schedule_rule_id IN (SELECT id FROM schedule_rule WHERE title_id = ?)`, id); err != nil {
		return 0, translate(op, err)
	}
	// Un relevo o una repetición que apuntaba a una de estas reglas se queda
	// sin a quién apuntar: se le quita la referencia, no se borra la regla.
	for _, campo := range []string{"releva_a", "repite_a"} {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
			UPDATE schedule_rule SET %s = NULL
			WHERE %s IN (SELECT id FROM schedule_rule WHERE title_id = ?)`, campo, campo),
			id); err != nil {
			return 0, translate(op, err)
		}
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM schedule_rule WHERE title_id = ?`, id)
	if err != nil {
		return 0, translate(op, err)
	}
	quitadas, _ := res.RowsAffected()

	// Los episodios y los alias del título se van con él (ON DELETE CASCADE).
	if _, err := tx.ExecContext(ctx, `DELETE FROM title WHERE id = ?`, id); err != nil {
		return 0, translate(op, err)
	}

	if err := r.anotar(ctx, tx, id, "quitado", t.nombre,
		fmt.Sprintf("no es un programa: se quitó «%s» y %s", t.nombre, reglasEnPalabras(int(quitadas)))); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, translate(op, err)
	}
	return int(quitadas), nil
}

// ── ayudas ────────────────────────────────────────────────────────────

// tituloBreve es lo poco que hace falta saber de un título para emparejarlo.
type tituloBreve struct {
	nombre    string
	channelID *int64
	pendiente bool
}

func tituloEnTx(ctx context.Context, tx *sql.Tx, id int64, op string) (tituloBreve, error) {
	var t tituloBreve
	var canal sql.NullInt64
	err := tx.QueryRowContext(ctx,
		`SELECT nombre, channel_id, pendiente_emparejar FROM title WHERE id = ?`, id).
		Scan(&t.nombre, &canal, &t.pendiente)
	if errors.Is(err, sql.ErrNoRows) {
		return tituloBreve{}, fmt.Errorf("%s: título %d: %w", op, id, ErrNotFound)
	}
	if err != nil {
		return tituloBreve{}, translate(op, err)
	}
	t.channelID = ptrInt64(canal)
	return t, nil
}

// anotar deja el cambio en la bitácora encadenada, dentro de la misma
// transacción que lo hizo. Sin bitácora a mano (un TitleRepo suelto en una
// prueba) no se anota nada y el cambio sigue.
func (r *TitleRepo) anotar(ctx context.Context, tx *sql.Tx, id int64, campo, antes, despues string) error {
	if r.audit == nil {
		return nil
	}
	entidad := id
	e := model.AuditEntry{
		Entity: "title", EntityID: &entidad, Field: campo,
		Before: antes, After: despues, Origin: "humano", Kind: "cambio",
	}
	return r.audit.appendIn(ctx, tx, &e)
}

// reglasEnPalabras cuenta reglas en palabras claras, para la bitácora y para lo
// que se le enseña a una persona.
func reglasEnPalabras(n int) string {
	switch n {
	case 0:
		return "ninguna regla lo usaba"
	case 1:
		return "1 regla"
	default:
		return fmt.Sprintf("%d reglas", n)
	}
}
