package store

import (
	"errors"
	"fmt"
	"strings"
)

// Los errores que una persona va a leer en pantalla. El esquema los levanta
// en inglés y en jerga de SQLite; aquí se traducen una sola vez.
var (
	// ErrNotFound es "eso no está en la base".
	ErrNotFound = errors.New("no se encontró")

	// ErrOverlap es la restricción de no-solape del esquema (PRD §15).
	ErrOverlap = errors.New("ya hay algo programado en ese momento")

	// ErrDates es el CHECK (fecha_fin >= fecha_inicio): el bug del Hellsing.
	ErrDates = errors.New("la fecha de fin no puede ser anterior a la de inicio")

	// ErrFueraDeVigencia es el trigger que ata el día de emisión de un
	// plan_item a las fechas de su regla (PRD §15).
	ErrFueraDeVigencia = errors.New("ese día queda fuera de las fechas de la regla")

	// ErrPistaInexistente es elegir para el aire una pista de sonido que el
	// archivo no trae (F1-61).
	ErrPistaInexistente = errors.New("ese archivo no tiene esa pista de sonido")

	// ErrMismoTitulo es emparejar un título consigo mismo (F1-66).
	ErrMismoTitulo = errors.New("ese título ya es ese mismo: no hay nada que emparejar")

	// ErrDestinoPendiente es emparejar contra una ficha que tampoco está
	// emparejada todavía: primero hay que resolver esa (F1-66).
	ErrDestinoPendiente = errors.New("esa ficha también está por emparejar: resuélvela primero")

	// ErrNoPendiente es querer quitar como provisional un título que no está
	// por emparejar, o sea una ficha del catálogo de verdad (F1-67).
	ErrNoPendiente = errors.New("ese título no está por emparejar")
)

// ErrCorrupt dice que la base no pasó PRAGMA integrity_check. Lleva la ruta
// del respaldo más reciente para que el arranque pueda restaurarlo, avisar y
// seguir (PRD §19: "la base se revisa al arrancar").
type ErrCorrupt struct {
	Path       string // la base que falló
	LastBackup string // el respaldo más reciente, "" si no hay ninguno
	Detail     string // lo que dijo SQLite, para el registro
}

func (e *ErrCorrupt) Error() string {
	msg := fmt.Sprintf("la base de datos %q está dañada", e.Path)
	if e.LastBackup != "" {
		msg += fmt.Sprintf("; el respaldo más reciente es %q", e.LastBackup)
	} else {
		msg += "; no hay ningún respaldo a la mano"
	}
	if e.Detail != "" {
		msg += " (" + e.Detail + ")"
	}
	return msg
}

// translate convierte el error crudo de SQLite en uno de los de arriba. Si no
// reconoce nada, devuelve el original envuelto con lo que se estaba haciendo.
func translate(op string, err error) error {
	if err == nil {
		return nil
	}
	texto := err.Error()
	switch {
	case strings.Contains(texto, "plan_item solapado"):
		return fmt.Errorf("%s: %w", op, ErrOverlap)
	case strings.Contains(texto, "fuera de la vigencia"):
		return fmt.Errorf("%s: %w", op, ErrFueraDeVigencia)
	case strings.Contains(texto, "CHECK constraint failed") &&
		(strings.Contains(texto, "fecha_fin") || strings.Contains(texto, "fecha_inicio") ||
			strings.Contains(texto, "ventana_fin") || strings.Contains(texto, "ventana_inicio")):
		return fmt.Errorf("%s: %w", op, ErrDates)
	}
	return fmt.Errorf("%s: %w", op, err)
}

// huelaARota reconoce los errores de SQLite que significan "este archivo no
// es una base sana", para tratarlos como corrupción y no como un fallo
// cualquiera de apertura.
func huelaARota(err error) bool {
	if err == nil {
		return false
	}
	texto := strings.ToLower(err.Error())
	for _, marca := range []string{"not a database", "malformed", "corrupt", "encrypted"} {
		if strings.Contains(texto, marca) {
			return true
		}
	}
	return false
}
