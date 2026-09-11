// aire.go es la puerta por la que un canal sale de sombra y vuelve a sombra.
//
// Encender un transmisor no es guardar un formulario. Hasta hoy el modo del
// canal era un campo más de `PUT /canal` —y estaba clavado en «sombra» a la
// fuerza, porque en F1 no había motor—; desde el motor de F2 el modo se
// cambia por su propia puerta, a propósito, con las comprobaciones delante y
// dejando constancia de quién lo hizo (F2-118).
//
// Las comprobaciones no son un examen: son las cosas sin las que la señal
// saldría negra. Cada una se dice en cristiano, y la que no se cumple dice
// además qué hacer. Lo que no se puede comprobar se avisa y no se impide:
// el software nunca regaña y nunca miente.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"antena787/internal/drivers/salida"
	"antena787/internal/engine"
	"antena787/internal/model"
)

// Los tres resultados de una comprobación. `falta` es la única que no deja
// encender.
const (
	CompBien  = "bien"
	CompAviso = "aviso"
	CompFalta = "falta"
)

// VentanaDeArranque es cuánto aire hacia delante se mira antes de encender.
// Media hora es el bloque más corto con el que trabaja la parrilla: si hay
// con qué llenarla, hay tiempo de sobra para arreglar lo que venga después
// sin que nadie vea negro.
const VentanaDeArranque = 30 * time.Minute

// Comprobacion es una de las cosas que se miran antes de dejar salir al aire,
// contada como se le cuenta a una persona. `Clave` es lo único interno: existe
// para que una prueba pueda señalar una comprobación sin depender de su frase.
// `Comprobacion` en web/src/lib/tipos.ts.
type Comprobacion struct {
	Clave     string `json:"clave"`
	Nombre    string `json:"nombre"`
	Resultado string `json:"resultado"`
	Texto     string `json:"texto"`
	Arreglo   string `json:"arreglo,omitempty"`
	Ruta      string `json:"ruta,omitempty"`
}

// NoSaleAlAire es lo que devuelve AlAire cuando falta algo. El texto es el de
// la comprobación que falta, ya escrito para una persona.
type NoSaleAlAire struct{ Comprobacion Comprobacion }

func (e *NoSaleAlAire) Error() string { return e.Comprobacion.Texto }

// ErrYaEstaba dice que el canal ya estaba en el modo que se pedía. No es un
// fallo de nadie: es que no hay nada que hacer.
var ErrYaEstaba = errors.New("el canal ya estaba así")

// ComprobacionesParaElAire es la lista entera, en el orden en que se lee: sin
// qué producir la señal no hay nada; sin a dónde mandarla, nadie la recibe;
// sin con qué llenar la próxima media hora, sale negro. El retorno de aire va
// al final porque no impide encender: solo dice qué no se va a poder
// comprobar.
func (a *App) ComprobacionesParaElAire(ctx context.Context) []Comprobacion {
	out := []Comprobacion{a.compFFmpeg()}

	filas, err := a.Store.Output.List(ctx, a.ChannelID)
	switch {
	case err != nil:
		out = append(out, Comprobacion{
			Clave: "salidas", Nombre: "Hay a dónde mandar la señal", Resultado: CompFalta,
			Texto:   "no pude leer a dónde manda su señal este canal: " + err.Error(),
			Arreglo: "Vuelve a intentarlo. Si sigue igual, reinicia Antena787.",
		})
	case len(filas) == 0:
		out = append(out, Comprobacion{
			Clave: "salidas", Nombre: "Hay a dónde mandar la señal", Resultado: CompFalta,
			Texto:   "todavía nadie ha dicho a dónde va la señal de este canal",
			Arreglo: "Escribe la dirección y el puerto del equipo que la recibe. Si lo que quieres es probar sin molestar a nadie, vale una grabación a un archivo.",
			Ruta:    "/ajustes",
		})
	default:
		out = append(out, Comprobacion{
			Clave: "salidas", Nombre: "Hay a dónde mandar la señal", Resultado: CompBien,
			Texto: fmt.Sprintf("%s configurada(s)", cuentas(len(filas), "salida", "salidas")),
		}, a.compSalidasAbren(ctx, filas))
	}

	sinCubrir, primero := a.aireDeLaProximaMediaHora(ctx)
	cobertura, conQue := a.conQueCubrir(ctx)
	out = append(out, compPlan(sinCubrir, primero), compCobertura(sinCubrir, cobertura, conQue))
	out = append(out, a.compRetornoDeAire(ctx))
	return out
}

// compFFmpeg: sin el programa que produce la señal no hay canal. Es lo
// primero porque es lo único que no se puede sustituir por nada.
func (a *App) compFFmpeg() Comprobacion {
	c := Comprobacion{Clave: "ffmpeg", Nombre: "Está el programa que produce la señal"}
	if a.FFmpeg != "" && a.FFprobe != "" {
		c.Resultado, c.Texto = CompBien, "ffmpeg y ffprobe están donde tienen que estar"
		return c
	}
	c.Resultado = CompFalta
	c.Texto = "no está el programa que produce la señal"
	if a.FFmpegErr != nil {
		c.Texto = a.FFmpegErr.Error()
	}
	c.Arreglo = "Pon ffmpeg y ffprobe junto al ejecutable de Antena787, o instálalos en la máquina, y vuelve a probar."
	c.Ruta = "/ajustes"
	return c
}

// compSalidasAbren prueba cada salida configurada como la abriría el motor:
// si los números que hay guardados no sirven, se ve aquí y no cuando la
// señal tenía que estar saliendo. Que una no abra no calla al canal —sale por
// las demás, F2-49—, así que es aviso; que no abra ninguna sí impide
// encender.
func (a *App) compSalidasAbren(ctx context.Context, filas []model.Output) Comprobacion {
	c := Comprobacion{Clave: "salidas_abren", Nombre: "Las salidas abren"}
	formato := a.formatoDelCanal(ctx)
	var abren, rotas []string
	for _, fila := range filas {
		nombre := strings.TrimSpace(fila.Name)
		if nombre == "" {
			nombre = "sin nombre"
		}
		d, err := salida.Para(fila, nil)
		if err == nil {
			_, err = d.Abrir(formato)
		}
		if err != nil {
			rotas = append(rotas, fmt.Sprintf("«%s» %s", nombre, err.Error()))
			continue
		}
		abren = append(abren, fmt.Sprintf("«%s» %s", nombre, d.Descripcion()))
	}
	switch {
	case len(abren) == 0:
		c.Resultado = CompFalta
		c.Texto = "ninguna salida de este canal se puede abrir: " + strings.Join(rotas, "; ")
		c.Arreglo = "Arregla lo que dice ahí —casi siempre es la dirección o el puerto— y vuelve a probar."
		c.Ruta = "/ajustes"
	case len(rotas) > 0:
		c.Resultado = CompAviso
		c.Texto = fmt.Sprintf("%s: %s. Sale por %s: %s",
			cuentas(len(rotas), "no abre", "no abren"), strings.Join(rotas, "; "),
			cuentas(len(abren), "la otra", "las otras"), strings.Join(abren, " · y "))
		c.Arreglo = "Se puede encender igual: la señal sale por las que abren. Arregla o quita la que no abre cuando puedas."
		c.Ruta = "/ajustes"
	default:
		c.Resultado = CompBien
		c.Texto = "la señal va a salir " + strings.Join(abren, " · y ")
	}
	return c
}

// compPlan cuenta si hay parrilla para la próxima media hora. No impide
// encender nunca: un hueco lo cubre el relleno, y de eso responde la
// comprobación siguiente.
func compPlan(sinCubrir time.Duration, primero string) Comprobacion {
	c := Comprobacion{Clave: "plan", Nombre: "Hay programación para la próxima media hora"}
	if sinCubrir <= 0 {
		c.Resultado = CompBien
		c.Texto = "la próxima media hora está llena"
		if primero != "" {
			c.Texto += ": empieza con «" + primero + "»"
		}
		return c
	}
	c.Resultado = CompAviso
	c.Texto = fmt.Sprintf("en la próxima media hora hay %s sin nada en la parrilla",
		enMinutos(sinCubrir))
	c.Arreglo = "Se puede encender igual: el hueco lo cubre el relleno. Si prefieres poner algo, la parrilla es el sitio."
	c.Ruta = "/parrilla"
	return c
}

// compCobertura es la que de verdad guarda la promesa de que nunca sale
// negro: si la parrilla tiene huecos y no hay ni relleno ni cartel de la
// estación con qué taparlos, no se enciende.
func compCobertura(sinCubrir time.Duration, hay bool, conQue string) Comprobacion {
	c := Comprobacion{Clave: "cobertura", Nombre: "Hay con qué cubrir un hueco"}
	switch {
	case hay:
		c.Resultado, c.Texto = CompBien, "si algo falta, sale "+conQue
	case sinCubrir > 0:
		c.Resultado = CompFalta
		c.Texto = fmt.Sprintf("en la próxima media hora hay %s sin nada en la parrilla, y no hay relleno ni cartel de la estación con qué cubrirlo: saldría negro",
			enMinutos(sinCubrir))
		c.Arreglo = "Haz el cartel de la estación —es un clic y lleva tu identificativo— o pon algo de relleno en la biblioteca. También vale llenar esa media hora en la parrilla."
		c.Ruta = "/biblioteca"
	default:
		c.Resultado = CompAviso
		c.Texto = "no hay relleno ni cartel de la estación guardado. La parrilla cubre la próxima media hora, así que se puede encender; el día que algo falle, el cartel se dibuja en el momento, y eso no lo puedo comprobar desde aquí"
		c.Arreglo = "Haz el cartel de la estación cuando puedas: es el último escalón y conviene tenerlo hecho antes de necesitarlo."
		c.Ruta = "/biblioteca"
	}
	return c
}

// compRetornoDeAire no impide nada: dice qué no se va a poder comprobar. Sin
// retorno, lo que sabemos es lo que mandamos, no lo que salió (ADR 0009).
func (a *App) compRetornoDeAire(ctx context.Context) Comprobacion {
	c := Comprobacion{Clave: "retorno", Nombre: "Se va a poder comprobar lo que sale"}
	retorno := a.setting(ctx, KeyAirReturn)
	if retorno == "" || retorno == "ninguno" {
		c.Resultado = CompAviso
		c.Texto = "no hay retorno de aire conectado: se puede salir al aire, pero no voy a poder comprobar que lo que sale es lo que mandé"
		c.Arreglo = "Cuando tengas un receptor o un cable que traiga la señal de vuelta, dilo en Ajustes."
		c.Ruta = "/ajustes"
		return c
	}
	c.Resultado = CompBien
	c.Texto = "hay retorno de aire apuntado: " + textoDelRetorno(retorno)
	return c
}

// textoDelRetorno traduce la respuesta del paso 4 del asistente a la frase
// que la persona leyó cuando la contestó.
func textoDelRetorno(valor string) string {
	for _, o := range OpcionesDeRetorno {
		if o.Valor == valor {
			return o.Texto
		}
	}
	return valor
}

// aireDeLaProximaMediaHora dice cuánto de la próxima media hora está sin
// cubrir por el plan, y con qué empieza. Se cuenta sobre la unión de los
// bloques: dos bloques pegados no dejan hueco, y uno que empezó antes cuenta
// desde ahora.
func (a *App) aireDeLaProximaMediaHora(ctx context.Context) (sinCubrir time.Duration, primero string) {
	desde := a.Now()
	hasta := desde.Add(VentanaDeArranque)
	items, err := a.Store.Plan.ListRange(ctx, a.ChannelID, desde, hasta)
	if err != nil {
		// No poder leer el plan no es lo mismo que no tener plan: se dice
		// entero sin cubrir, que es lo prudente.
		return VentanaDeArranque, ""
	}
	// Los bloques llegan ordenados por instante: basta con llevar hasta dónde
	// está cubierto.
	cubiertoHasta := desde
	for _, it := range items {
		if it.State != model.Planned && it.State != model.Cued {
			continue
		}
		ini, fin := it.PlannedAt, it.End()
		if ini.After(cubiertoHasta) {
			sinCubrir += ini.Sub(cubiertoHasta)
			cubiertoHasta = ini
		}
		if fin.After(cubiertoHasta) {
			cubiertoHasta = fin
		}
		if primero == "" {
			primero = a.nombreDelBloque(ctx, it)
		}
	}
	if cubiertoHasta.Before(hasta) {
		sinCubrir += hasta.Sub(cubiertoHasta)
	}
	return sinCubrir.Round(time.Second), primero
}

// nombreDelBloque es cómo se llama lo que pone un bloque del plan, para
// decírselo a una persona.
func (a *App) nombreDelBloque(ctx context.Context, it model.PlanItem) string {
	switch it.Origin {
	case model.OriginSlate:
		return "el cartel de la estación"
	case model.OriginLiveSource:
		return "una fuente en vivo"
	}
	if it.MediaAssetID == nil {
		return ""
	}
	asset, err := a.Store.Media.Get(ctx, *it.MediaAssetID)
	if err != nil {
		return ""
	}
	return a.TituloDelArchivo(ctx, asset)
}

// conQueCubrir dice si hay algo con qué tapar un hueco ahora mismo —relleno
// de la biblioteca, o un cartel de la estación ya hecho— y cómo se llama.
// No cuenta el cartel que se dibujaría en el momento: eso es una promesa, no
// un archivo que se pueda mirar.
func (a *App) conQueCubrir(ctx context.Context) (bool, string) {
	if fillers, err := a.Store.Filler.List(ctx, a.ChannelID); err == nil {
		for _, fa := range fillers {
			asset, err := a.Store.Media.Get(ctx, fa.MediaAssetID)
			if err != nil {
				continue
			}
			if !hayArchivo(asset.AirablePath()) {
				continue
			}
			return true, "el relleno «" + a.TituloDelArchivo(ctx, asset) + "»"
		}
	}
	if hayArchivo(a.setting(ctx, KeyDefaultFiller)) {
		return true, "el cartel de la estación"
	}
	if hayArchivo(filepath.Join(a.DataDir, "motor", ArchivoDelCartel)) {
		return true, "el cartel de la estación"
	}
	return false, ""
}

// hayArchivo dice si esa ruta es un archivo que pesa algo.
func hayArchivo(ruta string) bool {
	if strings.TrimSpace(ruta) == "" {
		return false
	}
	st, err := os.Stat(ruta)
	return err == nil && st.Size() > 0
}

// formatoDelCanal es el formato de casa con el que abriría el motor.
func (a *App) formatoDelCanal(ctx context.Context) engine.Format {
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return FormatOf("")
	}
	return FormatOf(ch.FormatProfile)
}

// ── encender y apagar ─────────────────────────────────────────────────

// PrimeraQueFalta es la comprobación que impide encender, si hay alguna.
func PrimeraQueFalta(comps []Comprobacion) *Comprobacion {
	for i := range comps {
		if comps[i].Resultado == CompFalta {
			return &comps[i]
		}
	}
	return nil
}

// AlAire saca el canal de sombra. Comprueba primero: si falta algo devuelve
// las comprobaciones y un *NoSaleAlAire con la frase de lo que falta, y no
// toca el canal. Si todo está, pone el canal al aire, avisa al motor para que
// arranque en el acto y deja el incidente.
func (a *App) AlAire(ctx context.Context, quien string) ([]Comprobacion, error) {
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return nil, err
	}
	comps := a.ComprobacionesParaElAire(ctx)
	if ch.Mode == ModoAire {
		return comps, ErrYaEstaba
	}
	if falta := PrimeraQueFalta(comps); falta != nil {
		return comps, &NoSaleAlAire{Comprobacion: *falta}
	}
	ch.Mode = ModoAire
	if err := a.Store.Channel.Update(ctx, ch); err != nil {
		return comps, err
	}
	a.avisarDelModo()
	a.Incident(model.IncAlAire.String(), fmt.Sprintf(
		"%s sacó el canal de sombra: la señal empieza a salir hacia el equipo configurado. %s",
		deQuien(quien), loQueSeAviso(comps)))
	return comps, nil
}

// ASombra devuelve el canal a sombra: la señal deja de salir. No hay nada que
// comprobar —apagar siempre se puede— pero queda dicho igual, porque es lo
// que explica un canal que de pronto no está.
func (a *App) ASombra(ctx context.Context, quien string) error {
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return err
	}
	if ch.Mode != ModoAire {
		return ErrYaEstaba
	}
	ch.Mode = ModoSombra
	if err := a.Store.Channel.Update(ctx, ch); err != nil {
		return err
	}
	a.avisarDelModo()
	a.Incident(model.IncASombra.String(), fmt.Sprintf(
		"%s devolvió el canal a modo sombra: la señal dejó de salir hacia el equipo configurado. El plan y la guía se siguen armando.",
		deQuien(quien)))
	return nil
}

// avisarDelModo despierta al motor para que no espere el próximo repaso:
// encender y apagar un transmisor no puede tardar diez segundos en pasar.
// Si nadie está escuchando, el repaso lo ve igual.
func (a *App) avisarDelModo() {
	select {
	case a.modoCambio <- struct{}{}:
	default:
	}
}

// deQuien es cómo se nombra en la bitácora a quien lo pidió.
func deQuien(quien string) string {
	if q := strings.TrimSpace(quien); q != "" {
		return q
	}
	return "alguien en la estación"
}

// loQueSeAviso resume los avisos que se aceptaron al encender: lo que no se
// va a poder comprobar queda escrito donde se va a leer después.
func loQueSeAviso(comps []Comprobacion) string {
	var avisos []string
	for _, c := range comps {
		if c.Resultado == CompAviso {
			avisos = append(avisos, c.Texto)
		}
	}
	if len(avisos) == 0 {
		return "Las comprobaciones salieron todas bien."
	}
	return "Se encendió con estos avisos: " + strings.Join(avisos, "; ") + "."
}

// cuentas escribe «1 salida» o «3 salidas» sin que nadie tenga que leer
// «1 salida(s)».
func cuentas(n int, uno, varios string) string {
	if n == 1 {
		return "1 " + uno
	}
	return fmt.Sprintf("%d %s", n, varios)
}

// enMinutos dice una duración en minutos redondos, que es como se habla de
// aire vacío.
func enMinutos(d time.Duration) string {
	min := int(d.Round(time.Minute) / time.Minute)
	if min <= 0 {
		return "menos de un minuto"
	}
	return cuentas(min, "minuto", "minutos")
}
