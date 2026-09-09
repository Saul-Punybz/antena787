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
	{Version: 3, SQL: migracion3},
	{Version: 4, SQL: migracion4},
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

// migracion3 le da al archivo su sonido (F1-58 a F1-63): qué pistas de audio
// trae, cuál va al aire, dónde queda la segunda (SAP, reservada para F2) y de
// qué archivos de al lado salieron el audio y los subtítulos.
//
// Las columnas nuevas no van en schema.sql: ese archivo es la foto de la
// versión 1 y todas las bases —nuevas y viejas— suben por estos mismos
// escalones, que es lo que hace que terminen idénticas.
const migracion3 = `
-- Las pistas de audio que trae el archivo, tal como las vio el ingest:
-- JSON [{"indice":0,"idioma":"es","canales":2,"titulo":"Español"}, ...].
ALTER TABLE media_asset ADD COLUMN pistas_audio TEXT NOT NULL DEFAULT '[]';

-- Cuál de esas pistas se oye al aire: su índice dentro de la lista.
ALTER TABLE media_asset ADD COLUMN pista_audio_aire INTEGER NOT NULL DEFAULT 0;

-- La segunda pista al aire (español/inglés). Vacía hasta F2, pero el hueco
-- ya está hecho: el motor no tendrá que cambiar el esquema para usarla.
ALTER TABLE media_asset ADD COLUMN pista_audio_sap INTEGER;

-- El archivo de audio que estaba al lado y se metió dentro del video.
ALTER TABLE media_asset ADD COLUMN audio_sidecar TEXT NOT NULL DEFAULT '';

-- El archivo de subtítulos que estaba al lado, con el mismo nombre.
ALTER TABLE media_asset ADD COLUMN subtitulos_sidecar TEXT NOT NULL DEFAULT '';
`

// migracion4 le da al catálogo la memoria de los emparejamientos (F1-64 a
// F1-67): qué títulos quedaron por emparejar, con qué fichas se parecían, y
// cómo llama la hoja a cada ficha para que la próxima vez se empareje sola.
//
// Como las de arriba, no va en schema.sql: ese archivo es la foto de la
// versión 1 y todas las bases —nuevas y viejas— suben por estos mismos
// escalones, que es lo que hace que terminen idénticas.
const migracion4 = `
-- El importador lo creó porque el nombre de la hoja no cuadró con ninguna
-- ficha: la regla entró igual, pero falta que alguien diga con cuál va.
ALTER TABLE title ADD COLUMN pendiente_emparejar INTEGER NOT NULL DEFAULT 0;

-- Las fichas que se le parecían tanto que no se podía elegir sin adivinar:
-- JSON [12, 15]. Vacío cuando no se parecía a ninguna.
ALTER TABLE title ADD COLUMN candidatos TEXT NOT NULL DEFAULT '[]';

-- Los que no están emparejados se buscan seguido; el resto no estorba.
CREATE INDEX title_pendiente ON title(channel_id, pendiente_emparejar);

-- Cómo llama la hoja a una ficha del catálogo. El alias se guarda tal como
-- se escribió y la clave es su forma comparable (model.ClaveDeNombre), que
-- es por donde se busca: una sola por canal.
CREATE TABLE title_alias (
  id          INTEGER PRIMARY KEY,
  channel_id  INTEGER REFERENCES channel(id),   -- NULL = vale para todos
  alias       TEXT NOT NULL,                    -- como lo escribe la hoja
  clave       TEXT NOT NULL,                    -- minúsculas, sin acentos ni signos
  title_id    INTEGER NOT NULL REFERENCES title(id) ON DELETE CASCADE,
  creado      TEXT NOT NULL,
  UNIQUE (channel_id, clave)
);
CREATE INDEX title_alias_titulo ON title_alias(title_id);
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
