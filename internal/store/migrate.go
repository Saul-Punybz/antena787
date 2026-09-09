package store

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"time"
)

// schemaSQL es la versión 1 del esquema, embebida en el binario: no hay
// archivos sueltos que se puedan perder en una instalación (PRD §14.1).
//
//go:embed schema.sql
var schemaSQL string

// Migration es un escalón del esquema. La versión 1 es schema.sql; de la 2
// en adelante se añaden aquí, en orden, y nunca se editan una vez publicadas.
type Migration struct {
	Version int
	SQL     string
}

// migrations son los escalones posteriores a la versión 1. Se aplican en
// orden, tanto a una base vieja como a una recién creada con schema.sql: por
// eso una base nueva y una migrada terminan idénticas.
var migrations = []Migration{
	{Version: 2, SQL: migracion2},
}

// migracion2 cierra tres huecos de integridad del plan (F1-12, F1-22, F1-26
// y F1-44):
//
//   - ningún plan_item puede quedar fuera de la vigencia de su regla;
//   - devolver un ítem descartado a un estado vivo vuelve a comprobar el
//     no-solape (antes el trigger de UPDATE no miraba la columna estado);
//   - el plan guarda el fundido de salida del clip de relleno recortado y si
//     el ítem lo fijó una persona a mano.
const migracion2 = `
-- El fundido de salida (ms) del clip: 0 es "sale entero". Lo llena el
-- resolver en el último relleno de un hueco cuando hay que recortarlo.
ALTER TABLE plan_item ADD COLUMN fundido_salida_ms INTEGER NOT NULL DEFAULT 0;

-- Fijado = lo puso una persona a mano: el resolver no lo mueve ni lo borra.
ALTER TABLE plan_item ADD COLUMN fijado INTEGER NOT NULL DEFAULT 0;

-- Un plan_item no puede existir fuera del rango de fechas de su regla: el
-- día de emisión tiene que caer dentro de [fecha_inicio, fecha_fin].
CREATE TRIGGER plan_item_dentro_de_su_regla_insert
BEFORE INSERT ON plan_item
BEGIN
  SELECT RAISE(ABORT, 'plan_item fuera de la vigencia de su regla')
  WHERE NEW.schedule_rule_id IS NOT NULL AND EXISTS (
    SELECT 1 FROM schedule_rule s
    WHERE s.id = NEW.schedule_rule_id
      AND (NEW.dia_emision < s.fecha_inicio OR NEW.dia_emision > s.fecha_fin)
  );
END;
CREATE TRIGGER plan_item_dentro_de_su_regla_update
BEFORE UPDATE OF schedule_rule_id, dia_emision ON plan_item
BEGIN
  SELECT RAISE(ABORT, 'plan_item fuera de la vigencia de su regla')
  WHERE NEW.schedule_rule_id IS NOT NULL AND EXISTS (
    SELECT 1 FROM schedule_rule s
    WHERE s.id = NEW.schedule_rule_id
      AND (NEW.dia_emision < s.fecha_inicio OR NEW.dia_emision > s.fecha_fin)
  );
END;

-- El no-solape también vigila el estado: revivir un ítem descartado encima
-- de otro es un solape como cualquier otro.
DROP TRIGGER plan_item_sin_solape_update;
CREATE TRIGGER plan_item_sin_solape_update
BEFORE UPDATE OF instante_planeado_ms, duracion_planeada_ms, deck_id, estado ON plan_item
BEGIN
  SELECT RAISE(ABORT, 'plan_item solapado en el mismo deck')
  WHERE NEW.estado NOT IN ('skipped','fallido') AND EXISTS (
    SELECT 1 FROM plan_item p
    WHERE p.id <> NEW.id
      AND p.channel_id = NEW.channel_id AND p.deck_id = NEW.deck_id
      AND p.estado NOT IN ('skipped','fallido')
      AND p.instante_planeado_ms < NEW.instante_planeado_ms + NEW.duracion_planeada_ms
      AND NEW.instante_planeado_ms < p.instante_planeado_ms + p.duracion_planeada_ms
  );
END;
`

// SchemaVersion es la versión a la que lleva este binario.
func SchemaVersion() int {
	v := 1
	for _, m := range migrations {
		if m.Version > v {
			v = m.Version
		}
	}
	return v
}

// migrate lleva la base a la versión de este binario. Antes de cada escalón
// —nunca antes de crear la base vacía, que no tiene nada que perder— deja un
// respaldo con la versión y la fecha en el nombre (PRD §14.1).
func (s *Store) migrate(ctx context.Context) error {
	v, err := s.Version(ctx)
	if err != nil {
		return err
	}

	nueva := v == 0
	if nueva {
		if err := s.applyStep(ctx, 1, schemaSQL); err != nil {
			return fmt.Errorf("no se pudo crear el esquema: %w", err)
		}
		v = 1
	}

	for _, m := range migrations {
		if m.Version <= v {
			continue
		}
		// Una base recién creada pasa por los mismos escalones —así una base
		// nueva y una migrada quedan idénticas— pero no se respalda: no tiene
		// nada que perder.
		if !nueva {
			if _, err := s.backupBeforeMigration(ctx, v); err != nil {
				return fmt.Errorf("no se migró: falló el respaldo previo: %w", err)
			}
		}
		if err := s.applyStep(ctx, m.Version, m.SQL); err != nil {
			return fmt.Errorf("no se pudo aplicar la migración %d: %w", m.Version, err)
		}
		v = m.Version
	}
	return nil
}

// applyStep corre un escalón entero en una transacción y sube user_version.
// O entra todo, o no entra nada.
func (s *Store) applyStep(ctx context.Context, version int, script string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, script); err != nil {
		return err
	}
	// PRAGMA no admite parámetros; version es un entero nuestro, no entra
	// nada de fuera en esta cadena.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		return err
	}
	return tx.Commit()
}

// backupBeforeMigration copia la base antes de tocarla y devuelve la ruta.
// Primero intenta VACUUM INTO (limpio, con la base abierta); si no se puede,
// hace checkpoint del WAL y copia el archivo.
func (s *Store) backupBeforeMigration(ctx context.Context, fromVersion int) (string, error) {
	dst := backupName(s.path, fromVersion, time.Now())
	if err := s.Backup(ctx, dst); err == nil {
		return dst, nil
	}
	// Plan B: el archivo tal cual, con el WAL ya volcado.
	if _, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return "", err
	}
	if err := copyFile(s.path, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// backupName arma <ruta>.respaldo-<version>-<fecha>.db y, si ese nombre ya
// está tomado, le añade un número: nunca se pisa un respaldo existente.
func backupName(path string, version int, now time.Time) string {
	base := fmt.Sprintf("%s.respaldo-%d-%s", path, version, now.Format("2006-01-02"))
	name := base + ".db"
	for i := 2; i < 1000; i++ {
		if _, err := os.Stat(name); os.IsNotExist(err) {
			return name
		}
		name = fmt.Sprintf("%s-%d.db", base, i)
	}
	return name
}
