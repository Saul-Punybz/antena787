package despierto

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// procesoFalso es el guardián de mentira: se acuerda de si lo mataron y de si
// alguien lo esperó (esperar es lo que evita el proceso zombi).
type procesoFalso struct {
	mu      sync.Mutex
	muerto  bool
	esperas int
}

func (p *procesoFalso) Matar() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.muerto = true
	return nil
}

func (p *procesoFalso) Esperar() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.esperas++
	return nil
}

func (p *procesoFalso) estado() (bool, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.muerto, p.esperas
}

// lanzadorFalso apunta lo que se le pidió lanzar y devuelve un procesoFalso.
// Ninguna prueba de este paquete lanza caffeinate ni systemd-inhibit de
// verdad: sostener el sistema no es cosa de una prueba.
type lanzadorFalso struct {
	programa string
	args     []string
	veces    int
	proc     *procesoFalso
	err      error
}

func (l *lanzadorFalso) lanzar(_ context.Context, programa string, args ...string) (Proceso, error) {
	l.veces++
	l.programa, l.args = programa, args
	if l.err != nil {
		return nil, l.err
	}
	l.proc = &procesoFalso{}
	return l.proc, nil
}

// hay es un buscador que dice que sí; noHay, uno que dice que no.
func hay(string) (string, error) { return "/de/mentira", nil }
func noHay(p string) (string, error) {
	return "", errors.New("no está " + p)
}

func TestMacLanzaCaffeinateAtadoANuestroPid(t *testing.T) {
	l := &lanzadorFalso{}
	soltar, err := sostenerCaffeinate(context.Background(), l.lanzar, hay)
	if err != nil {
		t.Fatalf("no sostuvo: %v", err)
	}
	if l.programa != "caffeinate" {
		t.Fatalf("lanzó %q y esperaba caffeinate", l.programa)
	}
	junto := strings.Join(l.args, " ")
	for _, quiero := range []string{"-i", "-s", "-w", strconv.Itoa(os.Getpid())} {
		if !strings.Contains(junto, quiero) {
			t.Fatalf("a caffeinate le falta %q; le pasé: %q", quiero, junto)
		}
	}

	soltar()
	muerto, esperas := l.proc.estado()
	if !muerto {
		t.Fatal("soltar no mató a caffeinate")
	}
	if esperas != 1 {
		t.Fatalf("soltar esperó al proceso %d veces y tenía que ser 1 (o queda un zombi)", esperas)
	}

	// Soltar dos veces no puede hacer daño: el bucle de app lo llama al
	// apagar y el contexto puede haberlo llamado ya.
	soltar()
	if _, esperas := l.proc.estado(); esperas != 1 {
		t.Fatalf("soltar dos veces mató o esperó dos veces (%d)", esperas)
	}
}

func TestMacSinCaffeinateLoDiceEnCristiano(t *testing.T) {
	l := &lanzadorFalso{}
	_, err := sostenerCaffeinate(context.Background(), l.lanzar, noHay)
	if err == nil {
		t.Fatal("sin caffeinate tenía que dar error")
	}
	if l.veces != 0 {
		t.Fatal("no encontró caffeinate y aun así intentó lanzarlo")
	}
	if !strings.Contains(err.Error(), "caffeinate") || !strings.Contains(err.Error(), "suspensión") {
		t.Fatalf("el error no le dice a una persona qué hacer: %q", err)
	}
}

func TestLanzarQueFallaDevuelveErrorLegible(t *testing.T) {
	l := &lanzadorFalso{err: errors.New("no hay memoria")}
	soltar, err := sostenerCaffeinate(context.Background(), l.lanzar, hay)
	if err == nil {
		t.Fatal("el lanzador falló y no salió error")
	}
	if soltar != nil {
		t.Fatal("con error, soltar tiene que ser nil")
	}
	if !strings.Contains(err.Error(), "no hay memoria") {
		t.Fatalf("el error se comió la causa: %q", err)
	}
}

func TestLinuxUsaSystemdInhibitBloqueandoInactividadYSueño(t *testing.T) {
	l := &lanzadorFalso{}
	soltar, err := sostenerSystemdInhibit(context.Background(), l.lanzar, hay)
	if err != nil {
		t.Fatalf("no sostuvo: %v", err)
	}
	defer soltar()

	if l.programa != "systemd-inhibit" {
		t.Fatalf("lanzó %q y esperaba systemd-inhibit", l.programa)
	}
	junto := strings.Join(l.args, " ")
	for _, quiero := range []string{"--what=idle:sleep", "--who=Antena787", "--why=canal encendido", "--mode=block", "sleep infinity"} {
		if !strings.Contains(junto, quiero) {
			t.Fatalf("a systemd-inhibit le falta %q; le pasé: %q", quiero, junto)
		}
	}
}

func TestLinuxSinSystemdInhibitLoDiceEnCristiano(t *testing.T) {
	l := &lanzadorFalso{}
	_, err := sostenerSystemdInhibit(context.Background(), l.lanzar, noHay)
	if err == nil {
		t.Fatal("sin systemd-inhibit tenía que dar error")
	}
	if l.veces != 0 {
		t.Fatal("no hay systemd-inhibit y aun así intentó lanzarlo")
	}
	if !strings.Contains(err.Error(), "systemd-inhibit") || !strings.Contains(err.Error(), "a mano") {
		t.Fatalf("el error no le dice a una persona qué hacer: %q", err)
	}
}

// SostenerCon es la puerta de cada sistema: con las piezas de mentira puestas
// tiene que sostener sin lanzar nada de verdad, o decir claro que no pudo.
// Nunca entra en pánico y nunca devuelve las dos cosas a la vez.
func TestSostenerConNuncaEntraEnPanico(t *testing.T) {
	l := &lanzadorFalso{}
	soltar, err := SostenerCon(context.Background(), l.lanzar, hay)
	switch {
	case err != nil && soltar != nil:
		t.Fatal("con error, soltar tiene que ser nil")
	case err == nil && soltar == nil:
		t.Fatal("sin error, soltar no puede ser nil")
	case err == nil:
		soltar()
		soltar()
	default:
		t.Logf("este sistema no se pudo sostener y lo dijo claro: %v", err)
	}
}
