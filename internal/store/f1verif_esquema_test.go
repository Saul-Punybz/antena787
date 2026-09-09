package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"antena787/internal/model"
)

// Estas pruebas verifican los criterios de esquema de docs/ACEPTACION.md
// (F1-11, F1-12, F1-44) **a nivel de base de datos**: el SQL va crudo, sin
// pasar por los repositorios de Go, para que lo que rechace la escritura sea
// un CHECK o un trigger del esquema y no una validación de la aplicación.

// reglaCruda mete una schedule_rule con SQL crudo y devuelve el error tal cual.
func reglaCruda(ctx context.Context, s *Store, from, to string) error {
	_, err := s.DB().ExecContext(ctx, `
		INSERT INTO schedule_rule (channel_id, tipo, patron_de_dias, hora,
			duracion_slot_ms, fecha_inicio, fecha_fin, episodios_por_corrida, activa)
		VALUES (?, 'normal', 'LMMJV__', 1200, 1800000, ?, ?, 1, 1)`,
		DefaultChannelID, from, to)
	return err
}

// F1-11 — Dado un intento de guardar una schedule_rule con fecha_fin anterior
// a fecha_inicio · Cuando se ejecuta el guardado · Entonces la base de datos
// rechaza la escritura por una restricción del esquema.
func TestF1Verif11FechasAlRevesLasRechazaElEsquema(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	// El caso Hellsing del §3, tal como está en la hoja de CAtv.
	err := reglaCruda(ctx, s, "2026-09-15", "2026-01-01")
	if err == nil {
		t.Fatal("el esquema aceptó una regla que termina antes de empezar")
	}
	if !strings.Contains(err.Error(), "CHECK constraint failed") {
		t.Fatalf("la rechazó algo que no es un CHECK del esquema: %v", err)
	}

	// La misma regla con las fechas en orden entra sin protestar.
	if err := reglaCruda(ctx, s, "2026-01-01", "2026-09-15"); err != nil {
		t.Fatalf("la regla buena no entró: %v", err)
	}
	// Fecha de fin igual a la de inicio: un solo día, válido.
	if err := reglaCruda(ctx, s, "2026-09-15", "2026-09-15"); err != nil {
		t.Fatalf("una regla de un solo día tiene que entrar: %v", err)
	}

	// Y el UPDATE tampoco se escapa.
	_, err = s.DB().ExecContext(ctx,
		`UPDATE schedule_rule SET fecha_fin = '2020-01-01' WHERE id = 1`)
	if err == nil {
		t.Fatal("el esquema dejó voltear las fechas con un UPDATE")
	}
	if !strings.Contains(err.Error(), "CHECK constraint failed") {
		t.Fatalf("el UPDATE lo rechazó otra cosa: %v", err)
	}
}

// F1-12 — Dado una regla con fecha_inicio = 9 de agosto y fecha_fin = 20 de
// diciembre · Cuando el sistema intenta crear un plan_item para esa regla con
// fecha 21 de diciembre · Entonces la operación falla: ningún plan_item puede
// existir fuera del rango de fechas de su regla.
func TestF1Verif12PlanItemFueraDelRangoDeSuRegla(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	if err := reglaCruda(ctx, s, "2026-08-09", "2026-12-20"); err != nil {
		t.Fatalf("la regla no entró: %v", err)
	}
	var reglaID int64
	if err := s.DB().QueryRowContext(ctx, `SELECT id FROM schedule_rule`).Scan(&reglaID); err != nil {
		t.Fatal(err)
	}
	programa, err := s.Deck.ByKind(ctx, DefaultChannelID, model.DeckProgram)
	if err != nil {
		t.Fatal(err)
	}

	inserta := func(dia string, cuando time.Time) error {
		_, err := s.DB().ExecContext(ctx, `
			INSERT INTO plan_item (channel_id, deck_id, schedule_rule_id, dia_emision,
				instante_planeado_ms, duracion_planeada_ms, origen, estado, hora_local)
			VALUES (?, ?, ?, ?, ?, ?, 'asset', 'planned', '20:00')`,
			DefaultChannelID, programa.ID, reglaID, dia, model.Ms(cuando), int64(1800000))
		return err
	}

	// Dentro del rango: tiene que entrar.
	if err := inserta("2026-12-20", time.Date(2026, 12, 21, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("un plan_item dentro del rango de su regla tiene que entrar: %v", err)
	}

	// Fuera del rango: el esquema tiene que rechazarlo.
	if err := inserta("2026-12-21", time.Date(2026, 12, 22, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("el esquema aceptó un plan_item del 21 de diciembre para una regla que termina el 20: " +
			"no hay CHECK ni trigger que ate dia_emision al rango de su schedule_rule")
	}

	// Y por el otro lado, antes de fecha_inicio.
	if err := inserta("2026-08-08", time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("el esquema aceptó un plan_item anterior a la fecha_inicio de su regla")
	}
}

// F1-44 — Dado dos plan_item del mismo deck (y la misma salida) con
// intervalos que se solapan · Cuando se intenta insertar el segundo ·
// Entonces la base de datos lo rechaza, no una validación de la aplicación.
func TestF1Verif44NoSolapeEnElEsquema(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	programa, err := s.Deck.ByKind(ctx, DefaultChannelID, model.DeckProgram)
	if err != nil {
		t.Fatal(err)
	}
	comercial, err := s.Deck.ByKind(ctx, DefaultChannelID, model.DeckCommercial)
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 9, 7, 20, 0, 0, 0, time.UTC)
	crudo := func(deck int64, inicio time.Time, dur time.Duration, estado string) error {
		_, err := s.DB().ExecContext(ctx, `
			INSERT INTO plan_item (channel_id, deck_id, dia_emision,
				instante_planeado_ms, duracion_planeada_ms, origen, estado, hora_local)
			VALUES (?, ?, '2026-09-07', ?, ?, 'asset', ?, '20:00')`,
			DefaultChannelID, deck, model.Ms(inicio), dur.Milliseconds(), estado)
		return err
	}

	if err := crudo(programa.ID, base, 30*time.Minute, "planned"); err != nil {
		t.Fatalf("el primer ítem no entró: %v", err)
	}
	// Se pisa por dentro.
	if err := crudo(programa.ID, base.Add(10*time.Minute), 30*time.Minute, "planned"); err == nil {
		t.Fatal("el esquema aceptó dos plan_item solapados en el mismo deck")
	} else if !strings.Contains(err.Error(), "plan_item solapado") {
		t.Fatalf("lo rechazó otra cosa distinta al trigger: %v", err)
	}
	// Lo envuelve entero.
	if err := crudo(programa.ID, base.Add(-10*time.Minute), time.Hour, "planned"); err == nil {
		t.Fatal("el esquema aceptó un ítem que envuelve a otro del mismo deck")
	}
	// Se pisa por un solo milisegundo.
	if err := crudo(programa.ID, base.Add(30*time.Minute-time.Millisecond), time.Minute, "planned"); err == nil {
		t.Fatal("el esquema aceptó un solape de un milisegundo")
	}
	// Pegado al final, sin pisar: pasa.
	if err := crudo(programa.ID, base.Add(30*time.Minute), 30*time.Minute, "planned"); err != nil {
		t.Fatalf("dos ítems seguidos sin solape tienen que entrar: %v", err)
	}
	// Otro deck a la misma hora: pasa (el spot dentro de un vivo).
	if err := crudo(comercial.ID, base.Add(10*time.Minute), 2*time.Minute, "planned"); err != nil {
		t.Fatalf("el ítem del deck comercial tiene que entrar: %v", err)
	}

	// Y mover un ítem encima de otro con UPDATE también se rechaza.
	var id int64
	if err := s.DB().QueryRowContext(ctx,
		`SELECT id FROM plan_item WHERE deck_id = ? ORDER BY instante_planeado_ms DESC LIMIT 1`,
		programa.ID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx,
		`UPDATE plan_item SET instante_planeado_ms = ? WHERE id = ?`,
		model.Ms(base.Add(5*time.Minute)), id); err == nil {
		t.Fatal("el esquema dejó mover un ítem encima de otro del mismo deck")
	}
}

// F1-44 (borde) — un ítem descartado (skipped/fallido) no bloquea el hueco,
// pero revivirlo con un UPDATE de estado tiene que seguir sin poder solapar.
func TestF1Verif44RevivirUnDescartadoNoDebeSolapar(t *testing.T) {
	ctx := context.Background()
	s, _ := abrir(t)

	programa, err := s.Deck.ByKind(ctx, DefaultChannelID, model.DeckProgram)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 7, 20, 0, 0, 0, time.UTC)
	crudo := func(inicio time.Time, dur time.Duration, estado string) (int64, error) {
		res, err := s.DB().ExecContext(ctx, `
			INSERT INTO plan_item (channel_id, deck_id, dia_emision,
				instante_planeado_ms, duracion_planeada_ms, origen, estado, hora_local)
			VALUES (?, ?, '2026-09-07', ?, ?, 'asset', ?, '20:00')`,
			DefaultChannelID, programa.ID, model.Ms(inicio), dur.Milliseconds(), estado)
		if err != nil {
			return 0, err
		}
		return res.LastInsertId()
	}

	descartado, err := crudo(base, 30*time.Minute, "skipped")
	if err != nil {
		t.Fatalf("un ítem descartado tiene que poder entrar: %v", err)
	}
	if _, err := crudo(base.Add(10*time.Minute), 30*time.Minute, "planned"); err != nil {
		t.Fatalf("lo descartado no reserva el aire: %v", err)
	}

	// Devolverlo a 'planned' lo pone encima del otro: la base debería negarse.
	if _, err := s.DB().ExecContext(ctx,
		`UPDATE plan_item SET estado = 'planned' WHERE id = ?`, descartado); err == nil {
		t.Fatal("el esquema dejó revivir un ítem descartado encima de otro: " +
			"el trigger de UPDATE no vigila la columna estado")
	}
}
