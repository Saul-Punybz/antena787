package store

import (
	"context"
	"fmt"

	"antena787/internal/model"
)

// seed deja la base utilizable desde el primer arranque: un canal y sus
// cuatro decks. Es idempotente — si ya hay canal, no toca nada más que las
// piezas que falten.
func (s *Store) seed(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("no se pudo sembrar la base: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var canales int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM channel`).Scan(&canales); err != nil {
		return fmt.Errorf("no se pudo sembrar la base: %w", err)
	}
	if canales == 0 {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO channel (id, nombre, tipo, zona_horaria,
			                     hora_inicio_dia_emision, carga_maxima_por_hora)
			VALUES (?, ?, ?, ?, ?, ?)`,
			DefaultChannelID, "Mi canal", string(model.ChannelTV),
			"America/Puerto_Rico", int64(6*60), 12)
		if err != nil {
			return fmt.Errorf("no se pudo crear el canal por defecto: %w", err)
		}
	}

	// Los cuatro decks, con la prioridad del PRD §9 paso 4. UNIQUE(channel_id,
	// tipo) hace el resto: correrlo dos veces no duplica nada.
	rows, err := tx.QueryContext(ctx, `SELECT id FROM channel ORDER BY id`)
	if err != nil {
		return fmt.Errorf("no se pudieron leer los canales: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return fmt.Errorf("no se pudieron leer los canales: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("no se pudieron leer los canales: %w", err)
	}
	_ = rows.Close()

	for _, id := range ids {
		for _, kind := range []model.DeckKind{model.DeckManual, model.DeckCommercial, model.DeckProgram, model.DeckFiller} {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO deck (channel_id, tipo, prioridad)
				SELECT ?, ?, ?
				WHERE NOT EXISTS (SELECT 1 FROM deck WHERE channel_id = ? AND tipo = ?)`,
				id, string(kind), model.DeckPriority[kind], id, string(kind))
			if err != nil {
				return fmt.Errorf("no se pudo crear el deck %s: %w", kind, err)
			}
		}
	}

	return tx.Commit()
}
