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

// La frase de `guardian_caido` también existe, y es la que se deja en la
// bitácora (TextoGuardianCaido).
func TestFraseDeGuardianCaido(t *testing.T) {
	texto := TextoDeIncidente("guardian_caido")
	if texto == "" || strings.Contains(texto, "_") {
		t.Fatalf("`guardian_caido` no tiene frase en cristiano: %q", texto)
	}
	if texto != TextoGuardianCaido {
		t.Fatalf("la frase del catálogo (%q) no es TextoGuardianCaido (%q)", texto, TextoGuardianCaido)
	}
}

// F2-112 · Si el guardián se cae por su cuenta —lo matan a mano, o el
// sistema— mientras el canal sigue encendido, el bucle se entera por el
// canal `caido`, la alarma vuelve a salir mientras no se repone, y en cuanto
// se repone queda el incidente `guardian_caido` una sola vez por caída (no
// una por cada reintento, y no capado a una sola vez como `maquina_despierta`).
func TestDespiertoSeReponeSoloSiElGuardianSeCae(t *testing.T) {
	original := sostenerPorDefecto
	t.Cleanup(func() { sostenerPorDefecto = original })

	var llamada int64
	var soltadas int64
	caida1 := make(chan error, 1)
	caida2 := make(chan error, 1)

	sostenerPorDefecto = func(context.Context) (func(), <-chan error, error) {
		switch atomic.AddInt64(&llamada, 1) {
		case 1:
			// El primer sostener, al arrancar: sale bien.
			return func() { atomic.AddInt64(&soltadas, 1) }, caida1, nil
		case 2:
			// El primer reintento tras la caída: falla, para que la alarma
			// tenga tiempo de enseñarse.
			return nil, nil, errors.New("el guardián no contesta")
		case 3:
			// El segundo reintento: se repone.
			return func() { atomic.AddInt64(&soltadas, 1) }, caida2, nil
		default:
			// Cualquier caída después de esta se repone a la primera.
			return func() { atomic.AddInt64(&soltadas, 1) }, make(chan error), nil
		}
	}

	a := abre(t, func(o *Options) {
		o.NoMaintenance = false // el bucle de no-dormir va con el mantenimiento
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.Start(ctx)

	// Arranca sostenido, sin alarma, con el incidente de siempre.
	esperar(t, 3*time.Second, func() bool { return cuentaIncidentes(t, a, "maquina_despierta") == 1 },
		"no arrancó sostenido")
	if al := alarmaDespierto(a); al != nil {
		t.Fatalf("arrancó sostenido y aun así hay alarma: %q", al.Texto)
	}

	// El guardián se cae por su cuenta.
	caida1 <- errors.New("lo mataron a mano")

	// Mientras no se repone, la alarma está puesta.
	esperar(t, 3*time.Second, func() bool { return alarmaDespierto(a) != nil },
		"el guardián se cayó y no salió la alarma")

	// Se repone solo, y la alarma se va.
	esperar(t, 3*time.Second, func() bool { return alarmaDespierto(a) == nil },
		"el guardián se cayó y el bucle no se repuso solo")

	// El incidente queda una sola vez por esta caída, aunque hiciera falta
	// un reintento fallido antes de reponerse.
	esperar(t, 3*time.Second, func() bool { return cuentaIncidentes(t, a, "guardian_caido") == 1 },
		"no quedó el incidente `guardian_caido` (o quedó más de uno) tras la primera caída")

	// Una segunda caída deja su propio incidente: no se queda en uno para
	// siempre, como maquina_despierta.
	caida2 <- errors.New("se cayó otra vez")
	esperar(t, 3*time.Second, func() bool { return cuentaIncidentes(t, a, "guardian_caido") == 2 },
		"la segunda caída no dejó su propio incidente `guardian_caido`")

	// maquina_despierta sigue en uno: eso no cambia con las caídas.
	if n := cuentaIncidentes(t, a, "maquina_despierta"); n != 1 {
		t.Fatalf("las caídas del guardián dejaron %d incidentes `maquina_despierta` y tenían que ser 1", n)
	}

	cancel()
	if err := a.Close(); err != nil {
		t.Fatalf("el apagado devolvió error: %v", err)
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
