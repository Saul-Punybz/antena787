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

// migrations son los escalones posteriores a la versión 1. Vacía a propósito:
// el esquema nace completo y crece por aquí.
var migrations = []Migration{}

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

	if v == 0 {
		if err := s.applyStep(ctx, 1, schemaSQL); err != nil {
			return fmt.Errorf("no se pudo crear el esquema: %w", err)
		}
		v = 1
	}

	for _, m := range migrations {
		if m.Version <= v {
			continue
		}
		if _, err := s.backupBeforeMigration(ctx, v); err != nil {
			return fmt.Errorf("no se migró: falló el respaldo previo: %w", err)
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
