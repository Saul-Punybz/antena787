// asistente.go es lo que el asistente de instalación necesita de la
// aplicación y no cabe en el paquete de HTTP: las listas de respuestas
// posibles de cada pregunta, lo que la máquina averigua sola (disco y red) y
// la primera parrilla que se propone en el paso 8.
//
// Regla de este archivo, que viene del principio 1 del PRD (§4): **nunca se
// pide elegir por nombre técnico**. Las opciones tienen un `valor` para la
// máquina y un `texto` escrito para una persona, y «todavía no» siempre está
// en la lista como respuesta válida.
package app

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"antena787/internal/model"
	"antena787/internal/resolver"
)

// KeyInstallStepAt es la clave donde queda apuntado cuándo se contestó cada
// paso del asistente. Con eso, una instalación acompañada por teléfono se
// puede reconstruir: en qué paso se quedó y cuánto tardó (F2-108).
func KeyInstallStepAt(n int) string { return fmt.Sprintf("instalacion.paso_%d_en", n) }

// ── las respuestas posibles de cada pregunta ──────────────────────────

// OpcionDelAsistente es una respuesta posible: lo que guarda la máquina, lo
// que lee la persona, y una frase de ayuda que explica cuándo es esa.
type OpcionDelAsistente struct {
	Valor string `json:"valor"`
	Texto string `json:"texto"`
	Ayuda string `json:"ayuda,omitempty"`
}

// OpcionesDeModo son las tres del paso 2 (PRD §13).
var OpcionesDeModo = []OpcionDelAsistente{
	{Valor: "internet", Texto: "Internet", Ayuda: "el canal se va a ver en una página, en una aplicación o en una caja conectada al televisor."},
	{Valor: "transmisor", Texto: "Transmisor", Ayuda: "el canal sale por antena o por el cable de la comunidad."},
	{Valor: "no_se", Texto: "Todavía no sé", Ayuda: "es una respuesta válida: se decide después y no bloquea nada."},
}

// OpcionesDeDestino son las categorías de salida del PRD §10, dichas como
// las diría una persona. Nunca se enseña el nombre interno de una salida.
var OpcionesDeDestino = []OpcionDelAsistente{
	{
		Valor: "red",
		Texto: "Al equipo que junta los canales, por el cable de red",
		Ayuda: "es lo más común en una estación chica: la señal sale de esta máquina por la red y entra al equipo que la mezcla con los demás canales antes del transmisor.",
	},
	{
		Valor: "internet",
		Texto: "A internet",
		Ayuda: "para que el canal se vea en una página, en una aplicación o en una caja conectada al televisor.",
	},
	{
		Valor: "route-dash",
		Texto: "A un transmisor de la televisión más nueva",
		Ayuda: "la generación reciente de la televisión por antena. Queda apuntada; todavía no está construida y el asistente no te va a decir que sí funciona cuando no.",
	},
	{
		Valor: "archivo",
		Texto: "A un archivo en el disco",
		Ayuda: "se graba lo que saldría al aire y no se manda a ningún sitio. Sirve para probar sin molestar a nadie.",
	},
	{
		Valor: "ninguna",
		Texto: "Todavía no lo sé",
		Ayuda: "es una respuesta válida: se sigue la instalación igual y se decide cuando tengas el equipo delante.",
	},
}

// OpcionesDeRetorno son las del retorno de aire (PRD §10, `capture_input`).
// «Todavía no» es respuesta válida y se enseña, no se esconde (PRD §13).
var OpcionesDeRetorno = []OpcionDelAsistente{
	{
		Valor: "receptor-tv",
		Texto: "Un sintonizador de televisión dentro de esta misma máquina",
		Ayuda: "una tarjeta puesta en la computadora que capta lo que sale por el aire, como cualquier televisor.",
	},
	{
		Valor: "captura",
		Texto: "Un cable de video que trae la señal de vuelta",
		Ayuda: "el cable que sale del equipo transmisor —o de un receptor que está al lado— y entra a esta máquina.",
	},
	{
		Valor: "stream",
		Texto: "Una dirección de internet donde se ve el canal",
		Ayuda: "la página o la dirección del transmisor, o la de la red, por la que se puede mirar lo que está saliendo.",
	},
	{
		Valor: "ninguno",
		Texto: "Todavía no",
		Ayuda: "es una respuesta válida. Sin retorno no se puede comparar lo que sale con lo que emites: se puede seguir, y se dice.",
	},
}

// OpcionesDeCalidad es la lista de formatos de casa del PRD §10. El valor es
// lo que entiende FormatOf; el texto es lo que lee una persona.
var OpcionesDeCalidad = []OpcionDelAsistente{
	{Valor: "480i59.94", Texto: "480i a 59.94 (la definición de siempre en EE. UU.)"},
	{Valor: "576i50", Texto: "576i a 50 (la definición de siempre en Europa y buena parte de Sudamérica)"},
	{Valor: "720p50", Texto: "720p a 50 (alta definición donde la corriente va a 50)"},
	{Valor: "720p59.94", Texto: "720p a 59.94 (lo normal en EE. UU.)"},
	{Valor: "1080i50", Texto: "1080i a 50 (alta definición entrelazada donde la corriente va a 50)"},
	{Valor: "1080i59.94", Texto: "1080i a 59.94 (alta definición entrelazada, la que piden muchos equipos)"},
	{Valor: "1080p25", Texto: "1080p a 25"},
	{Valor: "1080p29.97", Texto: "1080p a 29.97"},
	{Valor: "1080p59.94", Texto: "1080p a 59.94 (la que más pide de la máquina)"},
}

// OpcionesDelAsistente es el catálogo entero, tal como lo pinta la pantalla.
func OpcionesDelAsistente() map[string][]OpcionDelAsistente {
	return map[string][]OpcionDelAsistente{
		"modo":    copiaOpciones(OpcionesDeModo),
		"destino": copiaOpciones(OpcionesDeDestino),
		"retorno": copiaOpciones(OpcionesDeRetorno),
		"calidad": copiaOpciones(OpcionesDeCalidad),
	}
}

func copiaOpciones(in []OpcionDelAsistente) []OpcionDelAsistente {
	return append([]OpcionDelAsistente(nil), in...)
}

// OpcionValida dice si el valor está en la lista. La comparación no mira
// mayúsculas ni espacios de sobra: lo que llega de un formulario viene como
// viene.
func OpcionValida(lista []OpcionDelAsistente, valor string) bool {
	v := strings.ToLower(strings.TrimSpace(valor))
	for _, o := range lista {
		if strings.ToLower(o.Valor) == v {
			return true
		}
	}
	return false
}

// TextosDeOpciones enumera las opciones de una lista para poder decirle a
// una persona qué se aceptaba, en vez de un "valor inválido".
func TextosDeOpciones(lista []OpcionDelAsistente) string {
	textos := make([]string, 0, len(lista))
	for _, o := range lista {
		textos = append(textos, o.Texto)
	}
	switch len(textos) {
	case 0:
		return ""
	case 1:
		return textos[0]
	}
	return strings.Join(textos[:len(textos)-1], ", ") + " y " + textos[len(textos)-1]
}

// ── lo que la máquina averigua sola ───────────────────────────────────

// DiscoEnCristiano dice cuánto disco queda en una frase que una persona
// entiende. Si no se puede medir, lo dice: no se inventa un número.
func (a *App) DiscoEnCristiano() string {
	libre, total, err := espacioLibre(a.DataDir)
	if err != nil || total == 0 {
		return "no pude medir el disco"
	}
	return fmt.Sprintf("%s libres de %s en el disco de datos", tamañoEnCristiano(libre), tamañoEnCristiano(total))
}

// tamañoEnCristiano dice unos bytes como los diría alguien: "890 GB",
// "3.5 TB". A partir de cien no hacen falta decimales.
func tamañoEnCristiano(b uint64) string {
	const k = 1000.0
	unidades := []struct {
		corte  float64
		nombre string
	}{
		{k * k * k * k, "TB"},
		{k * k * k, "GB"},
		{k * k, "MB"},
		{k, "kB"},
	}
	v := float64(b)
	for _, u := range unidades {
		if v >= u.corte {
			n := v / u.corte
			if n >= 100 {
				return fmt.Sprintf("%.0f %s", n, u.nombre)
			}
			return fmt.Sprintf("%.1f %s", n, u.nombre)
		}
	}
	return fmt.Sprintf("%d bytes", b)
}

// RedEnCristiano dice si esta máquina está conectada a una red y por dónde.
// No tener red no es un fallo: la instalación entera funciona sin ella.
func RedEnCristiano() string {
	const sinRed = "sin red: se puede seguir, la guía y el aire no la necesitan"
	tarjetas, err := net.Interfaces()
	if err != nil {
		return sinRed
	}
	for _, t := range tarjetas {
		if t.Flags&net.FlagUp == 0 || t.Flags&net.FlagLoopback != 0 {
			continue
		}
		direcciones, err := t.Addrs()
		if err != nil {
			continue
		}
		for _, d := range direcciones {
			var ip net.IP
			switch v := d.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			v4 := ip.To4()
			if v4 == nil || v4.IsLoopback() || v4.IsLinkLocalUnicast() || v4.IsUnspecified() {
				continue
			}
			return fmt.Sprintf("conectado (%s, %s)", t.Name, v4.String())
		}
	}
	return sinRed
}

// ── la primera parrilla (paso 8) ──────────────────────────────────────

const (
	// BloqueDePropuesta es la retícula con la que se acomoda la propuesta:
	// media hora, que es la que usa todo el mundo en televisión.
	BloqueDePropuesta = 30 * time.Minute
	// HoraDeLaNoche es dónde empieza la franja en la que se ponen las
	// películas y lo que va una sola vez.
	HoraDeLaNoche model.Minutes = 19 * 60
	// DiasDePropuesta es cuánto vale la propuesta antes de que haya que
	// mirarla otra vez.
	DiasDePropuesta = 30
)

// Propuesta es el resultado del paso 8: qué se creó, qué salió del plan y
// qué se quedó fuera por no tener material.
type Propuesta struct {
	ReglasCreadas int
	Bloques       int
	Avisos        []resolver.Warning
	SinMaterial   int
	YaHabiaReglas bool
}

// candidato es un título de la biblioteca que se puede programar: tiene
// material listo y se sabe cuánto dura.
type candidato struct {
	titulo model.Title
	durMs  int64
	serie  bool
}

// ProponerParrilla arma la primera parrilla con lo que hay en la biblioteca,
// y después resuelve el plan.
//
// Es una **propuesta**, no una decisión: si el canal ya tiene reglas puestas
// —porque alguien pegó su hoja o las creó a mano— no se toca nada y se
// devuelve lo que ya había. Y lo que se crea se mueve o se borra en Reglas
// como cualquier otra regla.
//
// Lo que no cubran las reglas lo cubre el relleno: eso es trabajo del
// resolver, no de aquí.
func (a *App) ProponerParrilla(ctx context.Context) (Propuesta, error) {
	var p Propuesta

	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return p, err
	}
	reglas, err := a.Store.Rule.ListAll(ctx, a.ChannelID)
	if err != nil {
		return p, err
	}
	if len(reglas) > 0 {
		// Ya hay parrilla: el asistente no la pisa.
		p.YaHabiaReglas = true
		return a.cerrarPropuesta(ctx, p)
	}

	series, sueltos, sinMaterial, err := a.candidatosParaLaPropuesta(ctx)
	if err != nil {
		return p, err
	}
	p.SinMaterial = sinMaterial

	hoy := ch.BroadcastDay(a.Now())
	nuevas := acomodarPropuesta(ch, series, sueltos, hoy)
	for i := range nuevas {
		if err := a.Store.Rule.Insert(ctx, &nuevas[i]); err != nil {
			return p, err
		}
		p.ReglasCreadas++
	}
	if p.ReglasCreadas > 0 {
		a.Incident("propuesta_del_asistente", fmt.Sprintf(
			"el asistente propuso %s con lo que había en la biblioteca, desde el %s y por %d días; se cambian, se mueven o se quitan en Reglas",
			plural(p.ReglasCreadas, "un espacio", "%d espacios"), hoy, DiasDePropuesta))
	}
	if quedaron := len(series) + len(sueltos) - p.ReglasCreadas; quedaron > 0 {
		p.Avisos = append(p.Avisos, resolver.Warning{
			Kind: "propuesta",
			Text: fmt.Sprintf("%s no cupieron en el primer día y quedaron sin espacio: están en Biblioteca y los puedes colocar en Reglas cuando quieras",
				plural(quedaron, "un título", "%d títulos")),
		})
	}
	return a.cerrarPropuesta(ctx, p)
}

// cerrarPropuesta corre el resolver y devuelve lo que salió.
func (a *App) cerrarPropuesta(ctx context.Context, p Propuesta) (Propuesta, error) {
	out, err := a.Resolve(ctx)
	if err != nil {
		return p, err
	}
	p.Bloques = len(out.Items)
	p.Avisos = append(p.Avisos, out.Warnings...)
	return p, nil
}

func plural(n int, uno, varios string) string {
	if n == 1 {
		return uno
	}
	return fmt.Sprintf(varios, n)
}

// candidatosParaLaPropuesta separa la biblioteca en lo que va todos los días
// (las series) y lo que va una vez (películas y programas sueltos), y cuenta
// los títulos que no tienen material listo.
//
// El relleno, las cortinillas, los identificativos y los anuncios no se
// programan: no son programas.
func (a *App) candidatosParaLaPropuesta(ctx context.Context) (series, sueltos []candidato, sinMaterial int, err error) {
	titulos, err := a.Store.Title.List(ctx)
	if err != nil {
		return nil, nil, 0, err
	}
	listos, err := a.Store.Media.ListReady(ctx)
	if err != nil {
		return nil, nil, 0, err
	}
	duracion := map[int64]int64{}
	for _, m := range listos {
		duracion[m.ID] = m.DurationMs
	}

	for _, t := range titulos {
		if !esProgramable(t.Kind) {
			continue
		}
		c := candidato{titulo: t, serie: t.Kind == model.TitleSeries}

		episodios, err := a.Store.Episode.ListByTitle(ctx, t.ID)
		if err != nil {
			return nil, nil, 0, err
		}
		conMaterial := 0
		for _, e := range episodios {
			if e.MediaAssetID == nil {
				continue
			}
			d, hay := duracion[*e.MediaAssetID]
			if !hay {
				continue
			}
			conMaterial++
			if d > c.durMs {
				c.durMs = d
			}
		}
		if conMaterial > 1 {
			c.serie = true
		}
		if conMaterial == 0 && t.MediaAssetID != nil {
			if d, hay := duracion[*t.MediaAssetID]; hay {
				conMaterial, c.durMs = 1, d
			}
		}
		if conMaterial == 0 {
			sinMaterial++
			continue
		}
		if c.serie {
			series = append(series, c)
			continue
		}
		sueltos = append(sueltos, c)
	}

	porNombre := func(l []candidato) {
		sort.SliceStable(l, func(i, j int) bool { return l[i].titulo.Name < l[j].titulo.Name })
	}
	porNombre(series)
	porNombre(sueltos)
	return series, sueltos, sinMaterial, nil
}

// esProgramable deja fuera lo que no es un programa: las cortinillas, los
// identificativos, las promos y los anuncios se ponen solos entre programas.
func esProgramable(k model.TitleKind) bool {
	switch k {
	case model.TitleSeries, model.TitleMovie, model.TitleProgram:
		return true
	default:
		return false
	}
}

// acomodarPropuesta reparte los títulos por el día de emisión: las series
// desde que empieza el día, una detrás de otra, y las películas y lo suelto
// en la franja de la noche. Cada espacio dura lo que dura el material,
// redondeado a la media hora de arriba.
func acomodarPropuesta(ch model.Channel, series, sueltos []candidato, hoy model.Day) []model.ScheduleRule {
	inicio := int(ch.BroadcastDayAt)
	fin := inicio + 1440
	noche := int(HoraDeLaNoche)
	for noche < inicio {
		noche += 1440
	}
	if noche > fin {
		noche = fin
	}
	hasta := hoy.Add(DiasDePropuesta - 1)

	var out []model.ScheduleRule
	// Todo se propone todos los días de la semana: la alternativa —dejar
	// seis noches de siete vacías— es peor, y mover un espacio a un solo día
	// es un clic en Reglas.
	pon := func(c candidato, minuto int, largoMs int64) model.ScheduleRule {
		id := c.titulo.ID
		return model.ScheduleRule{
			ChannelID:      ch.ID,
			Kind:           model.RuleNormal,
			TitleID:        &id,
			Days:           model.DayPattern("LMMJVSD"),
			At:             model.Minutes(minuto % 1440),
			SlotMs:         largoMs,
			From:           hoy,
			To:             hasta,
			EpisodesPerRun: 1,
			Active:         true,
		}
	}

	// 1 · las series, desde que empieza el día de emisión hasta la noche.
	pos := inicio
	pendientes := make([]candidato, 0, len(series))
	for _, c := range series {
		largo := bloqueRedondeado(c.durMs)
		minutos := int(largo / int64(time.Minute/time.Millisecond))
		if pos+minutos > noche {
			pendientes = append(pendientes, c)
			continue
		}
		out = append(out, pon(c, pos, largo))
		pos += minutos
	}

	// 2 · la noche: las películas y lo que va una sola vez.
	pos = noche
	for _, c := range sueltos {
		largo := bloqueRedondeado(c.durMs)
		minutos := int(largo / int64(time.Minute/time.Millisecond))
		if pos+minutos > fin {
			break
		}
		out = append(out, pon(c, pos, largo))
		pos += minutos
	}

	// 3 · lo que no cupo antes de la noche va detrás de ella, si queda sitio.
	for _, c := range pendientes {
		largo := bloqueRedondeado(c.durMs)
		minutos := int(largo / int64(time.Minute/time.Millisecond))
		if pos+minutos > fin {
			break
		}
		out = append(out, pon(c, pos, largo))
		pos += minutos
	}
	return out
}

// bloqueRedondeado sube la duración medida a la media hora siguiente, que es
// la retícula con la que se lee una parrilla. Un programa de 22 minutos ocupa
// media hora; el resto lo cubre el relleno.
func bloqueRedondeado(durMs int64) int64 {
	bloque := int64(BloqueDePropuesta / time.Millisecond)
	if durMs <= 0 {
		return bloque
	}
	return (durMs + bloque - 1) / bloque * bloque
}
