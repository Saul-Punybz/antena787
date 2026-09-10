// Package despierto impide que la máquina se duerma mientras el canal esté
// encendido. Un servidor de playout que se echa una siesta saca el canal del
// aire sin que nadie lo toque: el 9 de septiembre de 2026, en modo sombra, la
// Mac durmió 19 minutos en dos ratos y lo único que quedó fueron cuatro
// incidentes `salto_de_reloj` (`docs/f1/SOMBRA-2026-09-09.md`, S-8). De ahí
// sale el criterio F2-112.
//
// La forma de pedirlo es distinta en cada sistema, así que aquí hay una sola
// puerta —Sostener— y tres maneras de cruzarla:
//
//   - macOS: se lanza `caffeinate -i -s -w <pid propio>` como subproceso.
//     Está en todos los macOS desde hace más de una década, así que no hay
//     nada que instalar. Lo suyo sería IOPMAssertionCreateWithName, la
//     llamada de verdad del sistema, pero eso pide CGo y este binario se
//     compila sin CGo para los tres sistemas: no se cambia esa regla por una
//     aserción. El `-w` hace que caffeinate se muera con nosotros aunque a
//     este proceso lo maten a lo bruto.
//   - Windows: SetThreadExecutionState por kernel32.dll, sin CGo, en un hilo
//     propio (la aserción es por hilo, y si el hilo muere se va con él).
//   - Linux: `systemd-inhibit --what=idle:sleep`, que es lo que usan los
//     reproductores de vídeo del escritorio. Sin systemd no hay a quién
//     pedírselo y se dice claro.
//
// Nada de aquí entra en pánico ni tumba nada: si no se puede sostener, se
// devuelve un error en cristiano y quien llame decide (en internal/app, una
// alarma de nivel aviso y un reintento cada cinco minutos).
package despierto

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

// Proceso es lo poco que hace falta de un subproceso: matarlo y esperar a que
// termine. Existe para que las pruebas no lancen `caffeinate` de verdad.
type Proceso interface {
	Matar() error
	Esperar() error
}

// Lanzador arranca un programa y devuelve el proceso vivo.
type Lanzador func(ctx context.Context, programa string, args ...string) (Proceso, error)

// Buscador dice dónde está un programa, o por qué no está.
type Buscador func(programa string) (string, error)

// Sostener pide al sistema que no se duerma por inactividad y devuelve la
// función que lo suelta. Llamar a soltar dos veces no hace daño; soltar
// también ocurre solo si se cancela el contexto.
//
// Si no se pudo, soltar es nil y el error se puede enseñar tal cual a una
// persona.
func Sostener(ctx context.Context) (soltar func(), err error) {
	return SostenerCon(ctx, nil, nil)
}

// LanzadorDelSistema es el que lanza de verdad. Un lanzador nil vale por este.
func LanzadorDelSistema(ctx context.Context, programa string, args ...string) (Proceso, error) {
	cmd := exec.CommandContext(ctx, programa, args...)
	// El subproceso no habla con nadie: ni le entra ni le sale nada.
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &procesoDelSistema{cmd: cmd}, nil
}

type procesoDelSistema struct{ cmd *exec.Cmd }

func (p *procesoDelSistema) Matar() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

func (p *procesoDelSistema) Esperar() error { return p.cmd.Wait() }

// buscadorDelSistema es exec.LookPath. Un buscador nil vale por este.
func buscadorDelSistema(programa string) (string, error) { return exec.LookPath(programa) }

// sostenerConProceso lanza un guardián y devuelve la función que lo mata. El
// guardián es un programa que, mientras viva, tiene al sistema despierto.
func sostenerConProceso(ctx context.Context, lanzar Lanzador, programa string, args ...string) (func(), error) {
	if lanzar == nil {
		lanzar = LanzadorDelSistema
	}
	p, err := lanzar(ctx, programa, args...)
	if err != nil {
		return nil, fmt.Errorf("no pude lanzar %s para impedir que la máquina se duerma: %w", programa, err)
	}
	var una sync.Once
	return func() {
		una.Do(func() {
			_ = p.Matar()
			// Esperarlo es lo que evita dejar un proceso zombi detrás.
			_ = p.Esperar()
		})
	}, nil
}

// sostenerCaffeinate es la manera de macOS: caffeinate atado a nuestro pid.
// Vive en el archivo común, y no en el de darwin, para que las pruebas la
// puedan recorrer en cualquier sistema con un lanzador de mentira.
func sostenerCaffeinate(ctx context.Context, lanzar Lanzador, buscar Buscador) (func(), error) {
	if buscar == nil {
		buscar = buscadorDelSistema
	}
	if _, err := buscar("caffeinate"); err != nil {
		return nil, fmt.Errorf("no encuentro caffeinate en esta Mac (%v): apaga la suspensión por inactividad en Ajustes del Sistema → Batería", err)
	}
	// -i: no dormirse por inactividad. -s: no dormirse el sistema mientras
	// haya corriente. -w: morirse cuando se muera este proceso.
	return sostenerConProceso(ctx, lanzar, "caffeinate", "-i", "-s", "-w", strconv.Itoa(os.Getpid()))
}

// sostenerSystemdInhibit es la manera de Linux. `sleep infinity` es el
// programa que systemd-inhibit tiene que vigilar: mientras corra, la
// prohibición está puesta.
func sostenerSystemdInhibit(ctx context.Context, lanzar Lanzador, buscar Buscador) (func(), error) {
	if buscar == nil {
		buscar = buscadorDelSistema
	}
	if _, err := buscar("systemd-inhibit"); err != nil {
		return nil, fmt.Errorf("esta máquina no tiene systemd-inhibit: apaga la suspensión a mano (%v)", err)
	}
	return sostenerConProceso(ctx, lanzar, "systemd-inhibit",
		"--what=idle:sleep",
		"--who=Antena787",
		"--why=canal encendido",
		"--mode=block",
		"sleep", "infinity")
}
