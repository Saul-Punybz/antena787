package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"antena787/internal/model"
)

// abre deja la aplicación lista sobre una base temporal, sin las goroutines
// de mantenimiento: una prueba no quiere respaldar cada hora.
func abre(t *testing.T, opts ...func(*Options)) *App {
	t.Helper()
	o := Options{DataDir: t.TempDir(), Version: "prueba", NoMaintenance: true}
	for _, f := range opts {
		f(&o)
	}
	a, err := Open(o)
	if err != nil {
		t.Fatalf("no pude abrir la aplicación: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestArranqueYApagadoLimpio(t *testing.T) {
	a := abre(t)

	ctx, cancel := context.WithCancel(context.Background())
	a.Start(ctx)

	// El resolver corre solo al arrancar: la guía tiene que existir.
	esperar(t, 3*time.Second, func() bool {
		g, _ := a.Guide()
		return len(g) > 0
	}, "la guía no se generó al arrancar")

	cancel()
	hecho := make(chan error, 1)
	go func() { hecho <- a.Close() }()
	select {
	case err := <-hecho:
		if err != nil {
			t.Fatalf("el apagado devolvió error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("el apagado se quedó colgado: alguna goroutine no mira su contexto")
	}
}

// El pánico de una goroutine no puede tumbar el proceso (auditoría A2): deja
// un incidente `panico_<nombre>` y la goroutine vuelve sola.
func TestPanicoDejaIncidenteYRelanza(t *testing.T) {
	var vueltas int64
	a := abre(t, func(o *Options) {
		o.RelaunchDelay = 20 * time.Millisecond
		o.Tasks = []Task{{
			Name: "prueba",
			Run: func(ctx context.Context) error {
				n := atomic.AddInt64(&vueltas, 1)
				if n == 1 {
					panic("me caí a propósito")
				}
				<-ctx.Done()
				return ctx.Err()
			},
		}}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.Start(ctx)

	esperar(t, 5*time.Second, func() bool { return atomic.LoadInt64(&vueltas) >= 2 },
		"la goroutine que se cayó no volvió")

	incidentes, err := a.Store.Incident.List(context.Background(), a.ChannelID,
		time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("no pude leer los incidentes: %v", err)
	}
	var panico *model.Incident
	for i := range incidentes {
		if incidentes[i].Kind == "panico_prueba" {
			panico = &incidentes[i]
		}
	}
	if panico == nil {
		t.Fatalf("no quedó el incidente panico_prueba: %+v", incidentes)
	}
	if !strings.Contains(panico.Detail, "me caí a propósito") {
		t.Fatalf("el incidente no dice qué pasó: %q", panico.Detail)
	}
}

// Una base dañada se restaura sola del respaldo más reciente y lo cuenta
// (PRD §19, auditoría D2).
func TestBaseDanadaSeRestauraSola(t *testing.T) {
	dir := t.TempDir()

	a, err := Open(Options{DataDir: dir, Version: "prueba", NoMaintenance: true})
	if err != nil {
		t.Fatalf("no pude abrir la aplicación: %v", err)
	}
	ctx := context.Background()
	if err := a.Store.Settings.Set(ctx, KeyOperator, "Rolando"); err != nil {
		t.Fatalf("no pude guardar un ajuste: %v", err)
	}
	// El respaldo de cada hora, hecho a mano.
	if err := a.Backup(ctx); err != nil {
		t.Fatalf("no pude respaldar: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("no pude cerrar: %v", err)
	}

	// El respaldo tiene que estar donde la restauración lo busca: al lado de
	// la base, con el nombre que usa el store.
	respaldos, _ := filepath.Glob(filepath.Join(dir, "respaldos", BackupPrefix+"*.db"))
	if len(respaldos) != 1 {
		t.Fatalf("se esperaba un respaldo y hay %d", len(respaldos))
	}
	dbPath := filepath.Join(dir, DBName)
	if err := copiar(respaldos[0], dbPath+".respaldo-1-prueba.db"); err != nil {
		t.Fatalf("no pude dejar el respaldo al lado de la base: %v", err)
	}

	// Y ahora se rompe la base a propósito.
	if err := os.WriteFile(dbPath, []byte("esto no es una base de datos"), 0o644); err != nil {
		t.Fatalf("no pude romper la base: %v", err)
	}
	for _, ext := range []string{"-wal", "-shm"} {
		_ = os.Remove(dbPath + ext)
	}

	b, err := Open(Options{DataDir: dir, Version: "prueba", NoMaintenance: true})
	if err != nil {
		t.Fatalf("la base dañada tenía que restaurarse sola: %v", err)
	}
	defer func() { _ = b.Close() }()

	if !b.Restored {
		t.Fatal("se abrió sin decir que había restaurado nada")
	}
	if v, err := b.Store.Settings.Get(ctx, KeyOperator); err != nil || v != "Rolando" {
		t.Fatalf("el respaldo no traía los datos: %q (%v)", v, err)
	}
	incidentes, err := b.Store.Incident.List(ctx, b.ChannelID, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("no pude leer los incidentes: %v", err)
	}
	visto := false
	for _, i := range incidentes {
		if i.Kind == "base_restaurada" {
			visto = true
		}
	}
	if !visto {
		t.Fatalf("no quedó el incidente base_restaurada: %+v", incidentes)
	}
}

func TestRotacionDeRespaldos(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < BackupsKept+5; i++ {
		name := filepath.Join(dir, BackupPrefix+time.Date(2026, 9, 1, 0, 0, i, 0, time.UTC).Format("20060102-150405")+".db")
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rotateBackups(dir, BackupsKept)
	quedan, _ := filepath.Glob(filepath.Join(dir, BackupPrefix+"*.db"))
	if len(quedan) != BackupsKept {
		t.Fatalf("quedaron %d respaldos, se esperaban %d", len(quedan), BackupsKept)
	}
}

func TestFormatOf(t *testing.T) {
	casos := map[string]struct{ w, h, num, den int }{
		"720p59.94":  {1280, 720, 60000, 1001},
		"1080i29.97": {1920, 1080, 30000, 1001},
		"480i2997":   {720, 480, 30000, 1001},
		"1080p50":    {1920, 1080, 50, 1},
		"":           {1280, 720, 60000, 1001},
		"lo que sea": {1280, 720, 60000, 1001},
	}
	for perfil, quiero := range casos {
		f := FormatOf(perfil)
		if f.Width != quiero.w || f.Height != quiero.h || f.FPSNum != quiero.num || f.FPSDen != quiero.den {
			t.Errorf("%q dio %dx%d a %d/%d", perfil, f.Width, f.Height, f.FPSNum, f.FPSDen)
		}
	}
}

func TestUntilNextHour(t *testing.T) {
	now := time.Date(2026, 9, 8, 14, 37, 12, 0, time.UTC)
	if d := untilNextHour(now); d != 22*time.Minute+48*time.Second {
		t.Fatalf("faltan %s para la hora en punto", d)
	}
	enPunto := time.Date(2026, 9, 8, 14, 0, 0, 0, time.UTC)
	if d := untilNextHour(enPunto); d != time.Hour {
		t.Fatalf("desde la hora en punto faltan %s", d)
	}
}

// ── ayudas ────────────────────────────────────────────────────────────

func esperar(t *testing.T, limite time.Duration, cond func() bool, mensaje string) {
	t.Helper()
	hasta := time.Now().Add(limite)
	for time.Now().Before(hasta) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(mensaje)
}

func copiar(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

// Resolver dos veces seguidas no puede chocar consigo mismo. La segunda
// corrida ocurre unos milisegundos después de la primera, así que el bloque
// que la primera puso "desde ahora" queda a caballo sobre el nuevo ahora: si
// no se borra, el esquema rechaza el plan entero por solape (auditoría B6).
func TestResolverDosVecesSeguidasNoChoca(t *testing.T) {
	a := abre(t)
	ctx := context.Background()

	if _, err := a.Resolve(ctx); err != nil {
		t.Fatalf("la primera corrida falló: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, err := a.Resolve(ctx); err != nil {
		t.Fatalf("la segunda corrida falló: %v", err)
	}
	if _, err := a.Resolve(ctx); err != nil {
		t.Fatalf("la tercera corrida falló: %v", err)
	}
}
