// manual.go son las rutas del control manual del aire: tomarlo, disparar,
// soltarlo y pararlo todo (PRD §9 paso 6, tanda T5).
//
// Quién lo hace no se pregunta: sale de la sesión (`autor`), que es el nombre
// que se escribió al entrar. Pedirlo otra vez en cada botón sería pedirle a
// alguien que se identifique tres veces para sacar un spot al aire.
package api

import (
	"errors"
	"net/http"
	"time"

	"antena787/internal/model"
	"antena787/internal/store"
)

// manualEnPantalla es el estado del control manual tal como lo lee la
// pantalla, con lo que hace falta para pintarlo sin calcular nada.
type manualEnPantalla struct {
	EnManual     bool       `json:"en_manual"`
	Quien        string     `json:"quien,omitempty"`
	Desde        *time.Time `json:"desde,omitempty"`
	SoltandoseEn *time.Time `json:"soltandose_en,omitempty"`
	FinDeBloque  *time.Time `json:"fin_de_bloque,omitempty"`
	// SoyYo dice si quien mira es quien tiene el control. Es la diferencia
	// entre enseñar el panel de disparo y enseñar «Rolando tiene el control
	// desde las 3:12 PM» con el botón de quitárselo (F2-77).
	SoyYo bool `json:"soy_yo"`
	// Historial son las últimas retenciones: quién tuvo el aire y cómo lo
	// soltó. Sin esto, «se fue a automático solo» no tiene explicación.
	Historial []retencionEnPantalla `json:"historial"`
}

type retencionEnPantalla struct {
	Quien  string     `json:"quien"`
	Desde  time.Time  `json:"desde"`
	Hasta  *time.Time `json:"hasta,omitempty"`
	Motivo string     `json:"motivo,omitempty"`
	// Texto es el motivo ya escrito para una persona: la pantalla no
	// interpreta claves.
	Texto string `json:"texto"`
}

// comoSeAcabo traduce el motivo del esquema a lo que lee una persona.
func comoSeAcabo(motivo string) string {
	switch motivo {
	case model.FinSoltado:
		return "lo soltó"
	case model.FinDeBloque:
		return "se acabó el bloque"
	case model.FinPorTimeout:
		return "silencio al aire: volvió solo"
	case model.FinCaidaDelSistema:
		return "se reinició el servicio"
	case model.FinPararTodo:
		return "paró todo"
	case model.FinQuitado:
		return "se lo quitaron"
	case "":
		return "lo tiene ahora mismo"
	}
	return motivo
}

func (s *Server) manualOut(r *http.Request) manualEnPantalla {
	e := s.App.ElManual()
	out := manualEnPantalla{
		EnManual:     e.EnManual,
		Quien:        e.Quien,
		Desde:        e.Desde,
		SoltandoseEn: e.SoltandoseEn,
		FinDeBloque:  e.FinDeBloque,
		SoyYo:        e.EnManual && e.Quien == autor(r),
		Historial:    []retencionEnPantalla{},
	}
	lista, err := s.App.Store.Manual.Ultimas(r.Context(), s.App.ChannelID, 8)
	if err == nil {
		for _, h := range lista {
			out.Historial = append(out.Historial, retencionEnPantalla{
				Quien: h.User, Desde: h.Start, Hasta: h.End,
				Motivo: h.EndReason, Texto: comoSeAcabo(h.EndReason),
			})
		}
	}
	return out
}

func (s *Server) manualGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.manualOut(r))
}

// manualTomar le da el aire a quien lo pide. Si ya lo tiene otra persona
// contesta 409 —no 400: no es que la petición esté mal escrita, es que el
// aire está ocupado— con quién y desde cuándo, que es lo que la pantalla
// enseña junto al botón de quitárselo (F2-77).
func (s *Server) manualTomar(w http.ResponseWriter, r *http.Request) {
	if !s.elCanalEstaAlAire(w, r) {
		return
	}
	_, err := s.App.TomarElControl(r.Context(), autor(r))
	var ocupado *store.ErrOtroTieneElControl
	if errors.As(err, &ocupado) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":  ocupado.Error(),
			"campo":  "quien",
			"quien":  ocupado.Quien,
			"desde":  ocupado.Desde,
			"manual": s.manualOut(r),
		})
		return
	}
	if err != nil {
		failStore(w, err, "tomar el control")
		return
	}
	s.audit(r, "manual_hold", nil, "control", "automático", autor(r))
	writeJSON(w, http.StatusOK, s.manualOut(r))
}

// manualQuitar se lo quita a quien lo tenga. Es otra ruta a propósito:
// quitarle el aire a una persona no puede ser lo que pasa cuando alguien
// pulsa el mismo botón dos veces.
func (s *Server) manualQuitar(w http.ResponseWriter, r *http.Request) {
	if !s.elCanalEstaAlAire(w, r) {
		return
	}
	_, aQuien, err := s.App.QuitarleElControl(r.Context(), autor(r))
	if err != nil {
		failStore(w, err, "quitar el control")
		return
	}
	s.audit(r, "manual_hold", nil, "control", aQuien, autor(r))
	writeJSON(w, http.StatusOK, s.manualOut(r))
}

// manualSoltar devuelve el aire al plan sin cortar nada por el medio: espera
// a que termine lo que suena, como máximo un minuto (F2-31). Contesta cuándo
// se entrega de verdad, porque un botón que parece no hacer nada durante
// cuarenta segundos se pulsa tres veces más.
func (s *Server) manualSoltar(w http.ResponseWriter, r *http.Request) {
	cuando, err := s.App.SoltarElControl(r.Context())
	if err != nil {
		fail(w, http.StatusConflict, err.Error(), "")
		return
	}
	s.audit(r, "manual_hold", nil, "control", autor(r), "automático")
	out := s.manualOut(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"manual": out,
		"cuando": cuando,
		"texto":  textoDeEntrega(cuando, s.Now()),
	})
}

// textoDeEntrega dice cuándo vuelve el aire al automático, en palabras.
func textoDeEntrega(cuando, ahora time.Time) string {
	falta := cuando.Sub(ahora).Round(time.Second)
	if falta <= 0 {
		return "el aire ya está en automático"
	}
	return "el aire vuelve al automático en " + falta.String() + ", cuando acabe lo que está sonando"
}

// manualParar corta en seco (F2-78). Es lo contrario de soltar, y por eso
// tiene su propia ruta y su propio botón: el caso de uso es que está saliendo
// algo que no tenía que salir.
func (s *Server) manualParar(w http.ResponseWriter, r *http.Request) {
	if err := s.App.PararTodo(r.Context(), autor(r)); err != nil {
		fail(w, http.StatusConflict, err.Error(), "")
		return
	}
	s.audit(r, "manual_hold", nil, "parar_todo", autor(r), "automático")
	writeJSON(w, http.StatusOK, s.manualOut(r))
}

// manualDisparar pone un archivo al aire ahora mismo.
func (s *Server) manualDisparar(w http.ResponseWriter, r *http.Request) {
	var body struct {
		MaterialID int64 `json:"material_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.MaterialID <= 0 {
		fail(w, http.StatusBadRequest, "dime qué archivo hay que poner al aire", "material_id")
		return
	}
	it, err := s.App.DispararAlAire(r.Context(), body.MaterialID)
	if err != nil {
		fail(w, http.StatusConflict, err.Error(), "material_id")
		return
	}
	s.audit(r, "plan_item", &it.ID, "disparo_manual", "", autor(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"bloque": it,
		"manual": s.manualOut(r),
	})
}

// elCanalEstaAlAire impide tomar el control en modo sombra: no hay aire que
// tomar, y dejar que alguien lo tome le haría creer que está emitiendo.
func (s *Server) elCanalEstaAlAire(w http.ResponseWriter, r *http.Request) bool {
	ch, err := s.App.Store.Channel.Get(r.Context(), s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return false
	}
	if ch.Mode != "aire" {
		fail(w, http.StatusConflict,
			"en modo sombra no hay aire que tomar: Antena787 todavía no está alimentando el transmisor", "")
		return false
	}
	return true
}
