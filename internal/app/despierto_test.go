package app

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// alarmaDespierto es la alarma de «me puedo dormir» si está puesta, o nil.
func alarmaDespierto(a *App) *Alarma {
	for _, al := range a.Alarms() {
		if al.Tipo == "maquina_puede_dormirse" {
			return &al
		}
	}
	return nil
}

// F2-112 · Si el sistema no deja impedir la suspensión, la persona se entera
// (alarma de nivel aviso, con la frase que dice qué hacer) y el canal sigue
// funcionando. Cuando el sistema deja —en el reintento—, la alarma se va y
// queda el incidente `maquina_despierta` una sola vez.
func TestDespiertoAvisaCuandoNoPudeYSeCallaCuandoSi(t *testing.T) {
	var intentos int64
	var soltadas int64
	a := abre(t, func(o *Options) {
		o.NoMaintenance = false // el bucle de no-dormir va con el mantenimiento
		o.DespiertoRetry = 20 * time.Millisecond
		o.Sostener = func(context.Context) (func(), error) {
			// El primer intento falla: es una máquina que no deja.
			if atomic.AddInt64(&intentos, 1) == 1 {
				return nil, errors.New("esta máquina no tiene systemd-inhibit: apaga la suspensión a mano")
			}
			return func() { atomic.AddInt64(&soltadas, 1) }, nil
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.Start(ctx)

	// Primero, la alarma.
	esperar(t, 3*time.Second, func() bool { return alarmaDespierto(a) != nil },
		"no pudo sostener la máquina despierta y no salió la alarma")
	al := alarmaDespierto(a)
	if al.Nivel != NivelAviso {
		t.Fatalf("la alarma es de nivel %q y tenía que ser un aviso: dormirse no saca a nadie del aire", al.Nivel)
	}
	if al.Texto != TextoDespiertoNoPude {
		t.Fatalf("la alarma dice %q y no la frase de la persona", al.Texto)
	}
	if !strings.Contains(al.Detalle, "systemd-inhibit") {
		t.Fatalf("el detalle se comió el motivo del sistema: %q", al.Detalle)
	}

	// Y en el reintento, el silencio.
	esperar(t, 3*time.Second, func() bool { return alarmaDespierto(a) == nil },
		"el reintento sostuvo la máquina y la alarma se quedó puesta")

	// El incidente queda una sola vez, aunque hubiera dos intentos.
	esperar(t, 3*time.Second, func() bool { return cuentaIncidentes(t, a, "maquina_despierta") == 1 },
		"no quedó el incidente `maquina_despierta` (o quedó más de uno)")

	// Al apagar, se suelta: la máquina vuelve a poder dormirse.
	cancel()
	if err := a.Close(); err != nil {
		t.Fatalf("el apagado devolvió error: %v", err)
	}
	if got := atomic.LoadInt64(&soltadas); got != 1 {
		t.Fatalf("al apagar se soltó %d veces y tenía que ser 1", got)
	}
}

// Si el sistema deja a la primera, no hay alarma ninguna y el incidente queda.
func TestDespiertoSostieneALaPrimeraSinAlarma(t *testing.T) {
	a := abre(t, func(o *Options) {
		o.NoMaintenance = false
		o.DespiertoRetry = 20 * time.Millisecond
		o.Sostener = func(context.Context) (func(), error) { return func() {}, nil }
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.Start(ctx)

	esperar(t, 3*time.Second, func() bool { return cuentaIncidentes(t, a, "maquina_despierta") == 1 },
		"sostuvo la máquina y no lo anotó en la bitácora")
	if al := alarmaDespierto(a); al != nil {
		t.Fatalf("sostuvo la máquina y aun así salió la alarma: %q", al.Texto)
	}
}

// La frase de la bitácora existe: un incidente sin frase se enseña en jerga.
func TestFraseDeMaquinaDespierta(t *testing.T) {
	texto := TextoDeIncidente("maquina_despierta")
	if texto == "" || strings.Contains(texto, "_") {
		t.Fatalf("`maquina_despierta` no tiene frase en cristiano: %q", texto)
	}
}

// cuentaIncidentes dice cuántos incidentes de un tipo hay en la bitácora.
func cuentaIncidentes(t *testing.T, a *App, tipo string) int {
	t.Helper()
	desde := a.Now().Add(-time.Hour)
	incs, err := a.Store.Incident.List(context.Background(), a.ChannelID, desde, a.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("no pude leer la bitácora: %v", err)
	}
	n := 0
	for _, i := range incs {
		if i.Kind == tipo {
			n++
		}
	}
	return n
}
