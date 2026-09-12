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
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"sort"
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

// ── el monitor: ver lo que el canal está produciendo (F2-117) ─────────

// Monitor dice si hay dónde mirar la señal y de dónde tirarla.
type Monitor struct {
	// Hay es si existe una salida que un navegador pueda pintar.
	Hay bool `json:"hay"`
	// URL es de dónde tira el navegador. Relativa al propio servidor cuando
	// la salida escucha en esta máquina.
	URL string `json:"url,omitempty"`
	// Porque explica qué falta, cuando falta algo.
	Porque string `json:"porque,omitempty"`
	// SalidaID es la salida que hace de monitor, si la hay.
	SalidaID int64 `json:"salida_id,omitempty"`
}

// ElMonitor busca entre las salidas del canal una que sirva para mirar: tiene
// que ser `http-ts` —porque el navegador tira de ella por HTTP— y en H.264,
// **porque ningún navegador sabe decodificar MPEG-2**.
//
// Eso último es la razón de que el monitor sea una salida aparte y no una
// vista de la que va al transmisor: son los mismos cuadros comprimidos de otra
// forma. Es lo que en una estación se llama un *confidence monitor*, y hay que
// decir qué es y qué no: **enseña lo que el canal está produciendo, no lo que
// salió por la antena**. Lo segundo es el retorno de aire, que es otra cosa.
func (a *App) ElMonitor(ctx context.Context) Monitor {
	salidas, err := a.Store.Output.List(ctx, a.ChannelID)
	if err != nil {
		return Monitor{Porque: "no se pudieron leer las salidas del canal"}
	}
	var hayHTTP bool
	for _, s := range salidas {
		if s.Driver != salida.DriverHTTPTS {
			continue
		}
		hayHTTP = true
		var p salida.ParamsHTTPTS
		if json.Unmarshal([]byte(s.Params), &p) != nil {
			continue
		}
		if p.Codec != "h264" {
			continue
		}
		ruta := p.Ruta
		if ruta == "" {
			ruta = "/stream.ts"
		}
		return Monitor{Hay: true, SalidaID: s.ID,
			URL: fmt.Sprintf("http://%s:%d%s", "127.0.0.1", p.Puerto, ruta)}
	}
	if hayHTTP {
		return Monitor{Porque: "hay una salida para ver desde otra computadora, pero está en MPEG-2 y un navegador no sabe pintarlo: hace falta una en H.264"}
	}
	return Monitor{Porque: "todavía no hay una salida para ver la señal desde el navegador"}
}

// ── por qué cable sale la señal ───────────────────────────────────────

// TarjetaDeRed es una tarjeta de esta máquina, como se le enseña a una
// persona. Nadie tiene por qué saberse su propia dirección IP de memoria.
type TarjetaDeRed struct {
	// IP es lo que se guarda y lo que se le pasa a ffmpeg.
	IP string `json:"ip"`
	// Nombre es el del sistema: «Ethernet», «Wi-Fi», «en0».
	Nombre string `json:"nombre"`
	// Texto es la línea que se lee: «Ethernet — 192.168.1.20».
	Texto string `json:"texto"`
	// Cableada distingue un cable de un inalámbrico. Importa: una señal de
	// televisión no se manda por wifi si hay un cable al lado.
	Cableada bool `json:"cableada"`
}

// TarjetasDeRed son las tarjetas por las que se puede mandar la señal.
//
// Existe porque decirle a alguien «escribe la IP de la tarjeta» es pedirle que
// abra una consola y sepa leer `ipconfig`. En una torre con dos cables —uno al
// multiplexor y otro a la red de la estación— escoger mal no da ningún error:
// la señal se va por el cable que no es y el canal simplemente no sale.
//
// No se ofrece el bucle local ni nada que esté caído: una tarjeta apagada no
// es una opción, es una forma de perder la tarde.
func TarjetasDeRed() []TarjetaDeRed {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := []TarjetaDeRed{}
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		dirs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, d := range dirs {
			ipn, ok := d.(*net.IPNet)
			if !ok {
				continue
			}
			// Solo IPv4: el multicast de una cadena de televisión va por ahí,
			// y ofrecer direcciones IPv6 que el multiplexor no entiende sería
			// ofrecer una forma de equivocarse.
			ip := ipn.IP.To4()
			if ip == nil || ip.IsLinkLocalUnicast() {
				continue
			}
			cableada := !esInalambrica(i.Name)
			out = append(out, TarjetaDeRed{
				IP: ip.String(), Nombre: i.Name, Cableada: cableada,
				Texto: fmt.Sprintf("%s — %s%s", i.Name, ip.String(),
					map[bool]string{true: "", false: " (inalámbrica)"}[cableada]),
			})
		}
	}
	// Las de cable primero: es lo que hay que escoger en una torre.
	sort.SliceStable(out, func(a, b int) bool { return out[a].Cableada && !out[b].Cableada })
	return out
}

// esInalambrica reconoce los nombres que le pone cada sistema a una tarjeta
// inalámbrica. No es exacto y no hace falta que lo sea: solo ordena la lista y
// pone una nota, nunca impide escoger.
func esInalambrica(nombre string) bool {
	n := strings.ToLower(nombre)
	for _, p := range []string{"wi-fi", "wifi", "wlan", "wl", "airport", "en1"} {
		if strings.Contains(n, p) {
			return true
		}
	}
	return false
}
