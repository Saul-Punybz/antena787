// Package salida son los drivers de salida: lo que traduce «a dónde manda
// este canal su señal», escrito una vez por una persona en lenguaje llano, a
// los argumentos con los que el encoder persistente tiene que arrancar.
//
// La palabra «driver» no se le enseña a nadie (PRD §10): la pantalla pregunta
// «¿a un receptor o a un grupo?» y «¿qué espera tu multiplexor?», y aquí se
// traduce. Los dos que existen en esta tanda son `udp-ts` —el que recibe el
// multiplexor de CAtv, el primero que se construye— y `archivo`. Los de
// internet (RTMP/HLS/SRT) y `http-ts` son T7.
//
// Cada salida se guarda en la tabla `output`: el nombre que le puso la
// persona, el driver, y sus `parametros` en JSON. Nada de esto es fijo en el
// código: sin la cadena `sout` exacta de Rolando, los PID, el programa y las
// tasas son **valores de ejemplo configurables** (docs/f2/PLAN-F2.md, riesgo
// de T2).
package salida

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// Los drivers que esta tanda construye. Los nombres son los de la columna
// `output.driver` y los del PRD §10.
const (
	DriverUDPTS   = "udp-ts"
	DriverArchivo = "archivo"
	// DriverHTTPTS sirve el mismo transport stream por HTTP, para que otro
	// programa tire de él: un VLC remoto, MistServer, o el monitor de la
	// propia pantalla (F2-115, F2-117). Es el `http{mux=ts,dst=…}` de VLC.
	//
	// No lo sirve ffmpeg: su protocolo http solo acepta UN cliente a la vez y
	// el criterio pide dos. ffmpeg entrega el TS por TCP a este mismo proceso
	// —el truco que el encoder ya usa para recibir video y audio— y Go lo
	// reparte.
	DriverHTTPTS = "http-ts"
)

// Los tres estados de conexión que la interfaz pinta (web/src/lib/tipos.ts).
// `sin_probar` es el que trae el esquema antes de que el motor arranque.
const (
	Conectada    = "conectada"
	Apagada      = "apagada"
	Reintentando = "reintentando"
)

// Espera mínima y tope de la espera progresiva de F2-48: 1, 2, 4… segundos,
// sin pasar de un minuto, y sin rendirse nunca.
const (
	EsperaMin = time.Second
	EsperaMax = time.Minute
)

// Driver es una salida viva. Abrir dice con qué argumentos tiene que
// arrancar el encoder; Vigilar deja escrito en la salida cómo le va
// (docs/f2/PLAN-F2.md §3).
type Driver interface {
	// Abrir traduce los parámetros de la salida a lo que el encoder
	// entiende, en el formato de casa del canal. Un error aquí está en palabras
	// claras y es lo que se le enseña a la persona.
	Abrir(f engine.Format) (engine.Output, error)
	// Vigilar corre hasta que se cancele el contexto: cuenta reintentos y
	// guarda estado_conexion, reintentos y ultimo_error de esa salida
	// (F2-48/49). No bloquea el motor: va en su propia goroutine.
	Vigilar(ctx context.Context, salida model.Output, listo <-chan error)
	// Descripcion es a dónde va, en palabras claras, para la pantalla y la
	// bitácora: «al grupo 239.1.1.1:1234, 4 saltos».
	Descripcion() string
}

// Registro es donde el driver deja el estado de la conexión. internal/store
// lo cumple con OutputRepo; una prueba lo cumple con un mapa.
//
// SetConnection se llama **desde la goroutine del vigilante**, no desde
// quien abrió la salida: quien lo implemente tiene que aguantar que lo
// llamen mientras otro lee lo que escribió. OutputRepo lo cumple porque
// escribe en la base; un doble de prueba necesita su candado.
type Registro interface {
	SetConnection(ctx context.Context, id int64, estado string, reintentos int, ultimoError string) error
}

// Para devuelve el driver de una salida configurada. Un driver que no
// existe todavía se dice por su nombre de pantalla, no por su clave.
func Para(o model.Output, reg Registro) (Driver, error) {
	switch strings.TrimSpace(o.Driver) {
	case DriverUDPTS:
		return nuevoUDPTS(o, reg)
	case DriverArchivo:
		return nuevoArchivo(o, reg)
	case "", "ninguna":
		return nil, errors.New("esta salida no dice a dónde mandar la señal")
	default:
		return nil, fmt.Errorf("todavía no sé mandar la señal por %q: en esta versión están la salida al multiplexor y la grabación a un archivo", o.Driver)
	}
}

// Disponibles son los drivers de salida que se le pueden ofrecer hoy a una
// persona. `udp-ts` está el primero porque es el que CAtv necesita (F2-50).
func Disponibles() []Ficha {
	return []Ficha{
		{Driver: DriverUDPTS, Nombre: "Al transmisor (multiplexor)",
			Explicacion: "La señal MPEG-2 por la red, a la dirección y el puerto que espera tu multiplexor."},
		{Driver: DriverArchivo, Nombre: "A un archivo",
			Explicacion: "Guarda lo que sale, tal cual, en el disco."},
	}
}

// Ficha es cómo se le presenta un driver a una persona: nunca la clave sola.
type Ficha struct {
	Driver      string `json:"driver"`
	Nombre      string `json:"nombre"`
	Explicacion string `json:"explicacion"`
}

// EsperaDe es la espera progresiva de F2-48: 1, 2, 4… con tope de un minuto.
// El primer reintento es el 1.
func EsperaDe(reintento int) time.Duration {
	if reintento < 1 {
		return EsperaMin
	}
	d := EsperaMin
	for i := 1; i < reintento; i++ {
		if d *= 2; d >= EsperaMax {
			return EsperaMax
		}
	}
	return d
}

// Ganancia es el ajuste de volumen de una salida: la diferencia entre su
// objetivo y el volumen al que está normalizada la biblioteca. La señal
// abierta pide −24 LKFS y la web −16 LUFS, y cada salida va a su propio
// objetivo, no a uno compartido (F2-47).
func Ganancia(objetivoDeLaBiblioteca float64, o model.Output) float64 {
	if o.TargetLoudness == 0 {
		return 0 // nadie eligió: sale como está normalizada
	}
	return o.TargetLoudness - objetivoDeLaBiblioteca
}

// vigilar es el Vigilar de los dos drivers: lo que cambia entre `udp-ts` y
// `archivo` es cómo se abre, no cómo se vigila.
//
// El canal `listo` lo alimenta el motor: nil cuando el encoder arrancó y está
// produciendo, y el error cuando se cayó. La salida `udp-ts` a un multiplexor
// no tiene nada que reconectar por su cuenta —el UDP no sabe si alguien
// escucha (PRD §9 paso 4)—: lo que se reintenta es el encoder, y lo que esta
// función hace es contar esos intentos y dejarlos escritos. La reconexión de
// verdad, la de las salidas de internet, es T7 (el resto de F2-48).
func vigilar(ctx context.Context, reg Registro, salida model.Output, listo <-chan error) {
	reintentos := 0
	marcar := func(estado, ultimo string) {
		if reg == nil {
			return
		}
		// Con el contexto ya cancelado no se puede escribir: se usa uno
		// suelto y corto, que el estado final («apagada») es el que la
		// pantalla necesita ver.
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = reg.SetConnection(c, salida.ID, estado, reintentos, ultimo)
	}
	for {
		select {
		case <-ctx.Done():
			marcar(Apagada, "")
			return
		case err, abierto := <-listo:
			if !abierto {
				marcar(Apagada, "")
				return
			}
			if err == nil {
				reintentos = 0
				marcar(Conectada, "")
				continue
			}
			reintentos++
			marcar(Reintentando, err.Error())
			select {
			case <-ctx.Done():
				marcar(Apagada, err.Error())
				return
			case <-time.After(EsperaDe(reintentos)):
			}
		}
	}
}
