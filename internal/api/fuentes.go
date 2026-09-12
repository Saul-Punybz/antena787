// fuentes.go son las señales en vivo del canal: de dónde sale lo que no es un
// archivo. Hasta el 11 de septiembre de 2026 la tabla y el repositorio
// existían y **no había una sola ruta que los alcanzara**: se podía programar
// un vivo que no se podía crear.
//
// La diferencia que manda en toda esta pantalla no es el protocolo, es quién
// llama a quién: hay señales que **se esperan** —se abre un puerto y alguien
// empuja— y señales que **se van a buscar**. La segunda es como se alimenta
// CAtv hoy, y es la que puede necesitar una clave.
package api

import (
	"context"
	"encoding/json"
	"net/http"

	"antena787/internal/app"
	"slices"
	"strings"

	"antena787/internal/model"
)

// fuenteBody es lo que se manda al crear o cambiar una señal.
type fuenteBody struct {
	Nombre      string `json:"nombre"`
	Tipo        string `json:"tipo"`
	Direccion   string `json:"direccion"`
	SoloAudio   bool   `json:"solo_audio"`
	RetardoMs   *int   `json:"retardo_ms"`
	GraciaS     *int   `json:"gracia_s"`
	RelojCortes []int  `json:"reloj_de_cortes"`
	// Clave es opcional y **solo se manda cuando cambia**: vacía conserva la
	// que hubiera. Nunca sale de vuelta.
	Clave string `json:"clave,omitempty"`
	// Usuario va con la clave, y sí se puede leer de vuelta.
	Usuario string `json:"usuario"`
}

// fuenteEnPantalla es una señal tal como sale. **No lleva la clave**: lo único
// que la pantalla necesita saber es si hay una guardada.
type fuenteEnPantalla struct {
	ID          int64  `json:"id"`
	Nombre      string `json:"nombre"`
	Tipo        string `json:"tipo"`
	Direccion   string `json:"direccion"`
	SeVaABuscar bool   `json:"se_va_a_buscar"`
	SoloAudio   bool   `json:"solo_audio"`
	RetardoMs   int    `json:"retardo_ms"`
	GraciaS     int    `json:"gracia_s"`
	RelojCortes []int  `json:"reloj_de_cortes"`
	Usuario     string `json:"usuario"`
	TieneClave  bool   `json:"tiene_clave"`
	Reglas      int    `json:"reglas"`
	Bloques     int    `json:"bloques"`
	// Texto es a dónde va o de dónde viene, ya escrito para una persona.
	Texto string `json:"texto"`
}

// tipoDeFuenteEnPantalla es cómo se le ofrece un tipo a una persona: nunca la
// clave sola (PRD §4.3).
type tipoDeFuenteEnPantalla struct {
	Tipo        string `json:"tipo"`
	Nombre      string `json:"nombre"`
	Explicacion string `json:"explicacion"`
	SeVaABuscar bool   `json:"se_va_a_buscar"`
	// Ejemplo es lo que se escribe en el campo de la dirección. Sin esto, la
	// persona tiene que adivinar el formato, que es donde se pierde la tarde.
	Ejemplo string `json:"ejemplo"`
}

func tiposDeFuente() []tipoDeFuenteEnPantalla {
	return []tipoDeFuenteEnPantalla{
		{Tipo: model.FuenteURL, Nombre: "Ir a buscarla a una dirección", SeVaABuscar: true,
			Explicacion: "El canal se conecta y tira de la señal. Es lo que hace falta para un stream que ya existe en otro sitio, tuyo o de un proveedor.",
			Ejemplo:     "http://localhost:8080/hls/for_tv/index.m3u8"},
		{Tipo: model.FuenteSRT, Nombre: "Esperarla por SRT",
			Explicacion: "Se abre un puerto y se espera a que alguien empuje la señal. Quien la manda tiene que saber la dirección de esta máquina.",
			Ejemplo:     "srt://0.0.0.0:9000"},
		{Tipo: model.FuenteRTMP, Nombre: "Esperarla por RTMP",
			Explicacion: "Igual que SRT, pero con el protocolo que usan la mayoría de los programas de transmisión.",
			Ejemplo:     "rtmp://0.0.0.0:1935/vivo"},
		{Tipo: model.FuenteCaptura, Nombre: "De una tarjeta de captura",
			Explicacion: "Una tarjeta metida en esta computadora, con una cámara o un mezclador conectado.",
			Ejemplo:     "Video Capture Device"},
	}
}

func (s *Server) fuentesList(w http.ResponseWriter, r *http.Request) {
	fuentes, err := s.App.Store.Live.List(r.Context(), s.App.ChannelID)
	if err != nil {
		failStore(w, err, "las señales en vivo")
		return
	}
	out := make([]fuenteEnPantalla, 0, len(fuentes))
	for _, f := range fuentes {
		out = append(out, s.fuenteEnPantallaDe(r, f))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"fuentes": out,
		"tipos":   tiposDeFuente(),
	})
}

func (s *Server) fuentesPost(w http.ResponseWriter, r *http.Request) {
	var b fuenteBody
	if !decode(w, r, &b) {
		return
	}
	f := model.LiveSource{ChannelID: s.App.ChannelID}
	if !s.aplicarFuente(w, r, &f, b) {
		return
	}
	if err := s.App.Store.Live.Upsert(r.Context(), &f); err != nil {
		failStore(w, err, "guardar la señal")
		return
	}
	if !s.guardarClaveDeFuente(w, r, &f, b) {
		return
	}
	s.audit(r, "live_source", &f.ID, "nombre", "", f.Name)
	writeJSON(w, http.StatusOK, s.fuenteEnPantallaDe(r, f))
}

func (s *Server) fuentesPut(w http.ResponseWriter, r *http.Request) {
	id, ok := idDeRuta(w, r)
	if !ok {
		return
	}
	vieja, err := s.App.Store.Live.Get(r.Context(), id)
	if err != nil {
		failStore(w, err, "la señal")
		return
	}
	var b fuenteBody
	if !decode(w, r, &b) {
		return
	}
	nueva := vieja
	if !s.aplicarFuente(w, r, &nueva, b) {
		return
	}
	if err := s.App.Store.Live.Upsert(r.Context(), &nueva); err != nil {
		failStore(w, err, "guardar la señal")
		return
	}
	if !s.guardarClaveDeFuente(w, r, &nueva, b) {
		return
	}
	s.auditDiff(r, "live_source", &id, vieja, nueva)
	writeJSON(w, http.StatusOK, s.fuenteEnPantallaDe(r, nueva))
}

func (s *Server) fuentesDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := idDeRuta(w, r)
	if !ok {
		return
	}
	if err := s.App.Store.Live.Delete(r.Context(), id); err != nil {
		failStore(w, err, "borrar la señal")
		return
	}
	s.audit(r, "live_source", &id, "borrada", "", "sí")
	writeJSON(w, http.StatusOK, map[string]any{"borrada": id})
}

// aplicarFuente valida y vuelca el cuerpo sobre la fuente. Devuelve false si
// ya contestó un error.
func (s *Server) aplicarFuente(w http.ResponseWriter, r *http.Request, f *model.LiveSource, b fuenteBody) bool {
	if n := strings.TrimSpace(b.Nombre); n != "" {
		f.Name = n
	}
	if f.Name == "" {
		fail(w, http.StatusBadRequest, "ponle un nombre a la señal: es como la vas a reconocer en la programación", "nombre")
		return false
	}
	if b.Tipo != "" {
		if !slices.Contains(model.TiposDeFuente(), b.Tipo) {
			fail(w, http.StatusBadRequest, "no reconozco esa forma de traer la señal", "tipo")
			return false
		}
		f.Kind = b.Tipo
	}
	if f.Kind == "" {
		fail(w, http.StatusBadRequest, "dime cómo llega la señal: si la esperas o si hay que ir a buscarla", "tipo")
		return false
	}
	if d := strings.TrimSpace(b.Direccion); d != "" {
		f.ListenPoint = d
	}
	if f.ListenPoint == "" {
		fail(w, http.StatusBadRequest, "falta la dirección: dónde se espera la señal, o de dónde hay que traerla", "direccion")
		return false
	}
	f.AudioOnly = b.SoloAudio
	if b.RetardoMs != nil {
		if *b.RetardoMs < 0 || *b.RetardoMs > 60000 {
			fail(w, http.StatusBadRequest, "la espera antes de dar la señal por perdida va de 0 a 60 segundos", "retardo_ms")
			return false
		}
		f.DelayMs = *b.RetardoMs
	}
	if b.GraciaS != nil {
		if *b.GraciaS < 0 || *b.GraciaS > 600 {
			fail(w, http.StatusBadRequest, "la gracia antes de soltar el aire va de 0 a 10 minutos", "gracia_s")
			return false
		}
		f.GraceSeconds = *b.GraciaS
	}
	if b.RelojCortes != nil {
		for _, m := range b.RelojCortes {
			if m < 0 || m > 59 {
				fail(w, http.StatusBadRequest, "los minutos de corte van de 0 a 59", "reloj_de_cortes")
				return false
			}
		}
		f.BreakClock = b.RelojCortes
	}
	return true
}

// paramsDeEntrada es lo NO secreto de una conexión de entrada: a qué fuente
// pertenece y con qué usuario se conecta. La clave no está aquí — vive en la
// columna cifrada de driver_config.
type paramsDeEntrada struct {
	FuenteID int64  `json:"fuente_id"`
	Usuario  string `json:"usuario"`
}

// guardarClaveDeFuente mete usuario y clave en driver_config, que es donde
// viven cifradas, y la deja apuntando a esta fuente. La clave vacía **no
// borra** la que había: la pantalla manda el formulario entero y no reenvía
// claves, que es justo como se filtran.
func (s *Server) guardarClaveDeFuente(w http.ResponseWriter, r *http.Request, f *model.LiveSource, b fuenteBody) bool {
	usuario := strings.TrimSpace(b.Usuario)
	if b.Clave == "" && usuario == "" {
		return true
	}
	ctx := r.Context()
	datos, err := json.Marshal(paramsDeEntrada{FuenteID: f.ID, Usuario: usuario})
	if err != nil {
		fail(w, http.StatusInternalServerError, "no se pudo guardar el usuario de la señal", "usuario")
		return false
	}
	canal := s.App.ChannelID
	c := model.DriverConfig{ChannelID: &canal, Kind: entradaKind, Driver: f.Kind, Params: string(datos)}
	if existente := s.conexionDeFuente(ctx, f.ID); existente != nil {
		c.ID = existente.ID
	}
	if err := s.App.Store.Conexion.Upsert(ctx, &c, b.Clave); err != nil {
		failStore(w, err, "guardar la clave de la señal")
		return false
	}
	return true
}

// entradaKind es el valor de driver_config.tipo para las conexiones de
// entrada. El esquema ya lo contemplaba desde la versión 1.
const entradaKind = "entrada"

// conexionDeFuente busca la conexión que guarda la clave de esta fuente.
func (s *Server) conexionDeFuente(ctx context.Context, id int64) *model.DriverConfig {
	cs, err := s.App.Store.Conexion.List(ctx, s.App.ChannelID, entradaKind)
	if err != nil {
		return nil
	}
	for i := range cs {
		var p paramsDeEntrada
		if json.Unmarshal([]byte(cs[i].Params), &p) == nil && p.FuenteID == id {
			return &cs[i]
		}
	}
	return nil
}

// fuenteEnPantallaDe arma lo que ve una persona: sin la clave, con si la hay,
// y con la frase de a dónde va o de dónde viene ya escrita por el servidor.
func (s *Server) fuenteEnPantallaDe(r *http.Request, f model.LiveSource) fuenteEnPantalla {
	ctx := r.Context()
	out := fuenteEnPantalla{
		ID: f.ID, Nombre: f.Name, Tipo: f.Kind, Direccion: f.ListenPoint,
		SeVaABuscar: f.SeVaABuscar(), SoloAudio: f.AudioOnly,
		RetardoMs: f.DelayMs, GraciaS: f.GraceSeconds,
		RelojCortes: f.BreakClock, Texto: textoDeFuente(f),
	}
	if out.RelojCortes == nil {
		out.RelojCortes = []int{}
	}
	if c := s.conexionDeFuente(ctx, f.ID); c != nil {
		out.TieneClave = c.TieneSecreto
		var p paramsDeEntrada
		if json.Unmarshal([]byte(c.Params), &p) == nil {
			out.Usuario = p.Usuario
		}
	}
	out.Reglas, out.Bloques, _ = s.App.Store.Live.EnUso(ctx, f.ID)
	return out
}

// textoDeFuente es a dónde va o de dónde viene, para que la pantalla no tenga
// que saber qué significa cada tipo.
func textoDeFuente(f model.LiveSource) string {
	switch f.Kind {
	case model.FuenteURL:
		return "se va a buscar a " + f.ListenPoint
	case model.FuenteCaptura:
		return "de la tarjeta «" + f.ListenPoint + "» de esta computadora"
	default:
		return "se espera en " + f.ListenPoint
	}
}

// ── probar antes de guardar ───────────────────────────────────────────

// pruebaDeFuente es lo que contesta el botón de «probar»: si responde, y qué
// viene por ahí. Todo en palabras que una persona entiende.
type pruebaDeFuente struct {
	Responde bool   `json:"responde"`
	Texto    string `json:"texto"`
	// Detalle es el motivo cuando no responde, tal como lo dijo el sistema.
	// Se enseña plegado: quien sepa leerlo lo agradece, y a quien no, no le
	// estorba.
	Detalle string `json:"detalle,omitempty"`
	// Lo que se encontró, cuando se encontró algo.
	Video    string   `json:"video,omitempty"`
	Audio    string   `json:"audio,omitempty"`
	Duracion string   `json:"duracion,omitempty"`
	Avisos   []string `json:"avisos,omitempty"`
}

// fuentesProbar abre la señal de verdad y dice qué hay. No guarda nada: se
// llama antes de guardar, y por eso acepta la clave en el cuerpo sin que
// exista todavía una fuente donde meterla.
func (s *Server) fuentesProbar(w http.ResponseWriter, r *http.Request) {
	var b fuenteBody
	if !decode(w, r, &b) {
		return
	}
	direccion := strings.TrimSpace(b.Direccion)
	if direccion == "" {
		fail(w, http.StatusBadRequest, "dime qué dirección quieres probar", "direccion")
		return
	}
	out := s.App.ProbarFuente(r.Context(), app.FuenteAProbar{
		Tipo:      b.Tipo,
		Direccion: direccion,
		Usuario:   strings.TrimSpace(b.Usuario),
		Clave:     b.Clave,
	})
	writeJSON(w, http.StatusOK, pruebaDeFuente{
		Responde: out.Responde, Texto: out.Texto, Detalle: out.Detalle,
		Video: out.Video, Audio: out.Audio, Duracion: out.Duracion, Avisos: out.Avisos,
	})
}
