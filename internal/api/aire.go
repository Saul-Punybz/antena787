package api

import (
	"errors"
	"net/http"
	"strings"
	"unicode"

	"antena787/internal/app"
)

// aire.go es la puerta del aire: las dos únicas rutas que cambian el modo del
// canal (F2-118). `PUT /canal` ya no lo toca —guardar el nombre de la
// estación no puede encender un transmisor de paso— y aquí, en cambio, hay
// que decirlo a propósito y escribirlo con la mano.
//
// Nada de esto es ceremonia: la confirmación escrita es lo que separa «quise
// encender el canal» de «apreté donde no era». El mismo patrón vale para
// apagar, porque apagar deja a la gente mirando negro.

// Lo que hay que escribir para confirmar. Se compara sin acentos, sin
// mayúsculas y sin espacios de sobra: se pide intención, no dictado.
const (
	ConfirmarAlAire  = "AL AIRE"
	ConfirmarASombra = "SOMBRA"
)

// cuerpoDeConfirmacion es lo que manda la pantalla: lo que la persona
// escribió.
type cuerpoDeConfirmacion struct {
	Confirmacion string `json:"confirmacion"`
}

// respuestaDelAire es lo que se contesta en las tres rutas: si se puede
// encender, las comprobaciones con su resultado, y el canal como quedó.
// `ComprobacionesDelAire` en web/src/lib/tipos.ts.
type respuestaDelAire struct {
	Puede          bool               `json:"puede"`
	Comprobaciones []app.Comprobacion `json:"comprobaciones"`
	Modo           string             `json:"modo"`
	// Aviso es la frase de lo que acaba de pasar, para que la pantalla no
	// tenga que escribirla ella.
	Aviso string `json:"aviso,omitempty"`
}

// canalComprobaciones es el ensayo: dice qué se ve sin cambiar nada. Es lo que
// la pantalla pinta en el panel antes de que nadie confirme.
func (s *Server) canalComprobaciones(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}
	comps := s.App.ComprobacionesParaElAire(ctx)
	writeJSON(w, http.StatusOK, respuestaDelAire{
		Puede:          app.PrimeraQueFalta(comps) == nil,
		Comprobaciones: comps,
		Modo:           ch.Mode,
	})
}

// canalAlAire saca el canal de sombra. Si falta algo, contesta 409 con la
// frase de lo que falta y la lista entera: la pantalla enseña las dos cosas,
// qué pasó y qué hacer.
func (s *Server) canalAlAire(w http.ResponseWriter, r *http.Request) {
	if !s.confirmado(w, r, ConfirmarAlAire,
		"para salir al aire hay que escribir «AL AIRE»: la señal va a empezar a salir hacia el equipo configurado") {
		return
	}
	ctx := r.Context()
	comps, err := s.App.AlAire(ctx, autor(r))
	switch {
	case errors.Is(err, app.ErrYaEstaba):
		fail(w, http.StatusConflict, "el canal ya está al aire", "")
		return
	case err != nil:
		var no *app.NoSaleAlAire
		if errors.As(err, &no) {
			writeJSON(w, http.StatusConflict, struct {
				Error string `json:"error"`
				Campo string `json:"campo,omitempty"`
				respuestaDelAire
			}{
				Error: "todavía no se puede salir al aire: " + no.Error(),
				Campo: no.Comprobacion.Clave,
				respuestaDelAire: respuestaDelAire{
					Puede: false, Comprobaciones: comps, Modo: app.ModoSombra,
				},
			})
			return
		}
		failStore(w, err, "poner el canal al aire")
		return
	}
	s.audit(r, "channel", &s.App.ChannelID, "modo", app.ModoSombra, app.ModoAire)
	writeJSON(w, http.StatusOK, respuestaDelAire{
		Puede: true, Comprobaciones: comps, Modo: app.ModoAire,
		Aviso: "el canal está al aire: la señal empieza a salir hacia el equipo configurado",
	})
}

// canalASombra devuelve el canal a sombra. No hay nada que comprobar —apagar
// siempre se puede— pero se pide la misma confirmación escrita: quien esté
// viendo el canal va a notarlo.
func (s *Server) canalASombra(w http.ResponseWriter, r *http.Request) {
	if !s.confirmado(w, r, ConfirmarASombra,
		"para volver a modo sombra hay que escribir «SOMBRA»: la señal va a dejar de salir") {
		return
	}
	ctx := r.Context()
	switch err := s.App.ASombra(ctx, autor(r)); {
	case errors.Is(err, app.ErrYaEstaba):
		fail(w, http.StatusConflict, "el canal ya está en modo sombra", "")
		return
	case err != nil:
		failStore(w, err, "devolver el canal a modo sombra")
		return
	}
	s.audit(r, "channel", &s.App.ChannelID, "modo", app.ModoAire, app.ModoSombra)
	comps := s.App.ComprobacionesParaElAire(ctx)
	writeJSON(w, http.StatusOK, respuestaDelAire{
		Puede:          app.PrimeraQueFalta(comps) == nil,
		Comprobaciones: comps,
		Modo:           app.ModoSombra,
		Aviso:          "el canal volvió a modo sombra: la señal dejó de salir y el plan se sigue armando",
	})
}

// confirmado lee la confirmación escrita y dice si vale. El texto del error es
// el que ve la persona, y dice qué va a pasar, no solo qué falta.
func (s *Server) confirmado(w http.ResponseWriter, r *http.Request, esperada, porQue string) bool {
	var body cuerpoDeConfirmacion
	if !decode(w, r, &body) {
		return false
	}
	if normalizarConfirmacion(body.Confirmacion) != esperada {
		fail(w, http.StatusBadRequest, porQue, "confirmacion")
		return false
	}
	return true
}

// normalizarConfirmacion deja la confirmación comparable: mayúsculas, sin
// acentos y con un solo espacio entre palabras. Escribir «al aire» cuenta;
// escribir otra cosa, no.
func normalizarConfirmacion(v string) string {
	var b strings.Builder
	espacio := false
	for _, r := range strings.TrimSpace(v) {
		if unicode.IsSpace(r) {
			espacio = true
			continue
		}
		if espacio && b.Len() > 0 {
			b.WriteRune(' ')
		}
		espacio = false
		b.WriteRune(unicode.ToUpper(sinTilde(r)))
	}
	return b.String()
}

// sinTilde quita la tilde de las vocales, que es lo único acentuado que puede
// aparecer en estas dos palabras.
func sinTilde(r rune) rune {
	switch r {
	case 'á', 'Á':
		return 'a'
	case 'é', 'É':
		return 'e'
	case 'í', 'Í':
		return 'i'
	case 'ó', 'Ó':
		return 'o'
	case 'ú', 'Ú', 'ü', 'Ü':
		return 'u'
	}
	return r
}
