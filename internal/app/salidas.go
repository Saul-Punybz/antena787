// salidas.go es lo que el motor abre antes de encender el encoder: las
// salidas del canal —una al multiplexor, otra a un archivo, las que haya—,
// cada una con su driver, su propio objetivo de volumen y su propio estado de
// conexión (F2-46, F2-47, F2-49). Es la tanda T2 de docs/f2/PLAN-F2.md.
//
// El encoder sigue siendo uno solo: varias salidas son varias ramas del mismo
// ffmpeg persistente, no varios ffmpeg. Eso es lo que hace que una salida no
// pueda cortar a las otras, y lo que ya midió la F0 con dos a la vez.
package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"antena787/internal/drivers/salida"
	"antena787/internal/engine"
	"antena787/internal/model"
)

// tandaDeSalidas son las salidas de una vida del encoder: lo que ffmpeg va a
// producir y el driver que vigila cada una.
type tandaDeSalidas struct {
	outs    []engine.Output
	drivers []salida.Driver
	filas   []model.Output
	avisos  []chan error
	textos  []string
}

// abrirSalidas traduce las salidas configuradas del canal a lo que el encoder
// entiende. Una salida que no se puede abrir —una dirección mal escrita, un
// PCR imposible— **no calla el canal**: se dice en palabras claras, queda apuntada
// en su propia fila, y las demás salen igual (F2-49).
func (a *App) abrirSalidas(ctx context.Context, formato engine.Format) (*tandaDeSalidas, error) {
	filas, err := a.Store.Output.List(ctx, a.ChannelID)
	if err != nil {
		return nil, err
	}
	objetivo, _ := a.loudnessTarget(ctx)

	t := &tandaDeSalidas{}
	for _, fila := range filas {
		d, err := salida.Para(fila, a.Store.Output)
		if err == nil {
			var out engine.Output
			if out, err = d.Abrir(formato); err == nil {
				// Cada salida a su propio volumen, no a uno compartido
				// (F2-47): −24 LKFS al transmisor, otro a internet.
				out.GainDB = salida.Ganancia(objetivo, fila)
				t.anade(fila, d, out, d.Descripcion())
				continue
			}
		}
		a.salidaNoAbre(ctx, fila, err)
	}
	if len(t.outs) > 0 {
		return t, nil
	}

	// Nadie ha dicho todavía a qué dirección va la señal. En vez de no emitir
	// —y dejar el canal callado sin explicar por qué— se graba en la carpeta
	// de datos y se dice. La retención de esa grabación es T7.
	ruta := filepath.Join(a.DataDir, "aire", fmt.Sprintf("aire-%s.ts", a.Now().Format("20060102-1504")))
	fila := model.Output{ChannelID: a.ChannelID, Name: "archivo", Driver: salida.DriverArchivo,
		Params: fmt.Sprintf(`{"ruta": %q}`, ruta)}
	d, err := salida.Para(fila, nil)
	if err != nil {
		return nil, err
	}
	out, err := d.Abrir(formato)
	if err != nil {
		return nil, err
	}
	a.Publish("motor", "salida", "todavía no me has dicho a qué dirección mandar la señal: por ahora se graba en "+ruta)
	t.anade(fila, d, out, d.Descripcion())
	return t, nil
}

// salidaNoAbre deja dicho que una salida se quedó fuera, y por qué. El texto
// es el que ve la persona: la pantalla de Al aire pinta `ultimo_error` tal
// cual.
func (a *App) salidaNoAbre(ctx context.Context, fila model.Output, err error) {
	motivo := "no se pudo abrir"
	if err != nil {
		motivo = err.Error()
	}
	a.Publish("motor", "salida", fmt.Sprintf("la salida «%s» se queda fuera: %s", fila.Name, motivo))
	if fila.ID != 0 {
		_ = a.Store.Output.SetConnection(ctx, fila.ID, salida.Apagada, fila.Retries, motivo)
	}
	a.Incident(model.IncEnlaceCaido.String(),
		fmt.Sprintf("la salida «%s» no pudo abrirse y el canal sale por las demás: %s", fila.Name, motivo))
}

// anade guarda una salida abierta con su driver y su aviso.
func (t *tandaDeSalidas) anade(fila model.Output, d salida.Driver, out engine.Output, texto string) {
	t.filas = append(t.filas, fila)
	t.drivers = append(t.drivers, d)
	t.outs = append(t.outs, out)
	t.textos = append(t.textos, texto)
	// Con espacio de sobra: avisar nunca puede bloquear al motor.
	t.avisos = append(t.avisos, make(chan error, 4))
}

// vigilar arranca el Vigilar de cada driver, cada uno en su goroutine.
func (t *tandaDeSalidas) vigilar(ctx context.Context) {
	for i, d := range t.drivers {
		go d.Vigilar(ctx, t.filas[i], t.avisos[i])
	}
}

// avisar le dice a todas las salidas cómo fue: nil es «está saliendo», y un
// error es la caída, que cada driver cuenta como reintento (F2-48).
func (t *tandaDeSalidas) avisar(err error) {
	for _, c := range t.avisos {
		select {
		case c <- err:
		default:
		}
	}
}

// cerrar suelta los avisos: los vigilantes ven el canal cerrado y apuntan la
// salida como apagada.
func (t *tandaDeSalidas) cerrar() {
	for _, c := range t.avisos {
		close(c)
	}
}

// texto es a dónde está saliendo el canal, en palabras claras, para la bitácora.
func (t *tandaDeSalidas) texto() string {
	if len(t.textos) == 0 {
		return "ninguna salida"
	}
	return strings.Join(t.textos, " · y ")
}

// SalidasDelCanal son las salidas configuradas tal como las pinta Al aire:
// el estado que dejó el driver más la frase de a dónde va cada una. Es lo que
// va en `salidas` de GET /estado y del empujón del WebSocket.
func (a *App) SalidasDelCanal(ctx context.Context) []SalidaEnPantalla {
	filas, err := a.Store.Output.List(ctx, a.ChannelID)
	if err != nil {
		return nil
	}
	out := make([]SalidaEnPantalla, 0, len(filas))
	for _, fila := range filas {
		out = append(out, a.UnaSalida(ctx, fila))
	}
	return out
}

// UnaSalida es una salida con la frase de a dónde va. Cuando el driver no
// puede con lo que tiene guardado, la frase es el motivo: es lo que la
// pantalla enseña en vez de callarse.
func (a *App) UnaSalida(_ context.Context, fila model.Output) SalidaEnPantalla {
	s := SalidaEnPantalla{Output: fila}
	if s.ConnectionState == "" {
		s.ConnectionState = "sin_probar"
	}
	if d, err := salida.Para(fila, nil); err == nil {
		s.Texto = d.Descripcion()
	} else {
		s.Texto = err.Error()
	}
	return s
}

// SalidaEnPantalla es una salida con la frase de a dónde va. `Salida` en
// web/src/lib/tipos.ts.
type SalidaEnPantalla struct {
	model.Output
	Texto string `json:"texto"`
}

// DriversDeSalida son los drivers de salida que se pueden ofrecer hoy, con su
// nombre de pantalla. `udp-ts` está entre ellos desde F2 (F2-50).
func DriversDeSalida() []salida.Ficha { return salida.Disponibles() }
