package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// BackupPrefix es el nombre con que salen los respaldos de cada hora.
const BackupPrefix = "antena-"

// Umbrales de disco del PRD §19 y de la auditoría D1, en porcentaje libre.
const (
	DiskWarn  = 10.0 // suena la alarma
	DiskPurge = 5.0  // la grabación se purga sola (F2)
	DiskStop  = 2.0  // la base deja de escribir el as-run; el aire sigue
)

// backupLoop respalda la base cada hora y conserva los últimos 24. El
// respaldo va a la carpeta configurada; si no hay ninguna, a datos/respaldos.
func (a *App) backupLoop(ctx context.Context) error {
	t := time.NewTicker(BackupEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if err := a.Backup(ctx); err != nil {
				a.Publish("respaldo", "fallo", "no se pudo respaldar la base: "+err.Error())
			}
		}
	}
}

// BackupDir es a dónde van los respaldos.
func (a *App) BackupDir(ctx context.Context) string {
	if dir := a.setting(ctx, KeyBackupFolder); dir != "" {
		return dir
	}
	return filepath.Join(a.DataDir, "respaldos")
}

// Backup deja una copia de la base y borra las más viejas de la cuenta.
func (a *App) Backup(ctx context.Context) error {
	dir := a.BackupDir(ctx)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(dir, BackupPrefix+a.Now().Format("20060102-150405")+".db")
	if err := a.Store.Backup(ctx, dst); err != nil {
		return err
	}
	rotateBackups(dir, BackupsKept)
	return nil
}

// rotateBackups conserva los keep más recientes.
func rotateBackups(dir string, keep int) {
	found, err := filepath.Glob(filepath.Join(dir, BackupPrefix+"*.db"))
	if err != nil || len(found) <= keep {
		return
	}
	sort.Strings(found) // el nombre lleva la fecha: orden alfabético = orden real
	for _, old := range found[:len(found)-keep] {
		_ = os.Remove(old)
	}
}

// diskLoop vigila el espacio libre. El disco lleno nunca saca el canal del
// aire: lo que hace es avisar, y avisar a tiempo (PRD §19).
func (a *App) diskLoop(ctx context.Context) error {
	last := ""
	check := func() {
		free, err := freePercent(a.DataDir)
		if err != nil {
			return
		}
		level := ""
		switch {
		case free < DiskStop:
			level = "critico"
		case free < DiskPurge:
			level = "purga"
		case free < DiskWarn:
			level = "aviso"
		}
		if level == last {
			return
		}
		last = level
		switch level {
		case "":
			a.setAlarms("disco", nil)
		case "aviso":
			a.disk(free, "queda menos del 10 %% de disco libre (%.1f %%): conviene hacer sitio")
		case "purga":
			a.disk(free, "queda menos del 5 %% de disco libre (%.1f %%): la grabación se va a purgar sola")
		case "critico":
			a.disk(free, "queda menos del 2 %% de disco libre (%.1f %%): se deja de anotar lo emitido, pero el aire sigue")
		}
	}
	check()
	t := time.NewTicker(DiskEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			check()
		}
	}
}

func (a *App) disk(free float64, format string) {
	texto := fmt.Sprintf(format, free)
	nivel := NivelAviso
	if free < DiskStop {
		nivel = NivelProblema
	}
	a.setAlarms("disco", []Alarma{{
		Tipo:    "disco",
		Nivel:   nivel,
		Texto:   texto,
		Detalle: fmt.Sprintf("queda el %.1f %% del disco", free),
		Accion:  &AccionAlarma{Texto: "ver la biblioteca", Ruta: "/biblioteca"},
	}})
	a.Incident("disco_bajo", texto)
}

// clockLoop compara la hora de pared con el reloj monotónico. Un salto de
// más de un minuto —adelante o atrás— no es deriva: es un salto, y el
// resolver recalcula desde el instante real (PRD §14.1, auditoría D3).
func (a *App) clockLoop(ctx context.Context) error {
	base := time.Now()
	t := time.NewTicker(ClockEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			now := time.Now()
			mono := now.Sub(base)                   // monotónico: no salta
			wall := now.Round(0).Sub(base.Round(0)) // de pared: sí salta
			if d := wall - mono; d > ClockJump || d < -ClockJump {
				a.Incident("salto_de_reloj", fmt.Sprintf(
					"el reloj del sistema saltó %s (o la máquina estuvo dormida ese tiempo); nada de lo que ya salió se vuelve a emitir y el plan se recalcula desde ahora",
					humanSigned(d)))
				a.Recalc()
			}
			base = now
		}
	}
}

// humanSigned dice un salto de reloj como lo diría una persona.
func humanSigned(d time.Duration) string {
	if d < 0 {
		return "hacia atrás " + d.Abs().Round(time.Second).String()
	}
	return "hacia adelante " + d.Round(time.Second).String()
}
