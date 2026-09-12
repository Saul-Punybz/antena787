package api

import (
	"net/http"
	"strings"

	"antena787/internal/app"
	"antena787/internal/drivers/salida"
	"antena787/internal/model"
)

// salidas.go es el CRUD de «a dónde manda este canal su señal» (§10, F2-46,
// F2-50, F2-114). No tiene pantalla propia a propósito: Al aire ya pinta las
// salidas con su estado, y el asistente es quien las escribe la primera vez.
// Aquí solo está el contrato.
//
// Los parámetros de cada driver viajan como JSON tal cual se guardan
// (`parametros`), y lo que se valida es lo que el driver sabe validar: así
// nadie tiene que repetir en la API lo que el driver ya dice en palabras claras.

// salidasList devuelve las salidas del canal con la frase de a dónde va cada
// una. Es lo mismo que lleva `salidas` en GET /estado.
func (s *Server) salidasList(w http.ResponseWriter, r *http.Request) {
	out := s.App.SalidasDelCanal(r.Context())
	if out == nil {
		out = []app.SalidaEnPantalla{}
	}
	writeJSON(w, http.StatusOK, salidasBody{
		Salidas:  out,
		Drivers:  app.DriversDeSalida(),
		Tarjetas: app.TarjetasDeRed(),
	})
}

// salidasBody es la respuesta de GET /salidas: lo que hay, y lo que se puede
// elegir. Los drivers van con su nombre de pantalla porque la persona nunca
// ve la palabra driver (PRD §10, F2-50).
type salidasBody struct {
	Salidas []app.SalidaEnPantalla `json:"salidas"`
	Drivers []salida.Ficha         `json:"drivers_disponibles"` // Tarjetas son las de red de esta máquina, para que quien configure una
	// salida multicast escoja por cuál cable sale en vez de tener que saberse
	// su propia dirección IP.
	Tarjetas []app.TarjetaDeRed `json:"tarjetas"`
}

// salidasPost crea una salida. Se prueba antes de guardarla: si el driver no
// puede abrirla —una dirección a medias, un PCR que un multiplexor
// descartaría— se dice y no se guarda nada.
func (s *Server) salidasPost(w http.ResponseWriter, r *http.Request) {
	nueva := model.Output{TargetLoudness: -24}
	if !decode(w, r, &nueva) {
		return
	}
	nueva.ID = 0
	nueva.ChannelID = s.App.ChannelID
	if !s.salidaValida(w, &nueva) {
		return
	}
	if err := s.App.Store.Output.Upsert(r.Context(), &nueva); err != nil {
		failStore(w, err, "guardar la salida")
		return
	}
	s.audit(r, "output", &nueva.ID, "creada", "", nueva.Driver+" "+nueva.Params)
	writeJSON(w, http.StatusCreated, s.App.UnaSalida(r.Context(), nueva))
}

// salidasPut cambia una salida. Lo que no venga en el cuerpo se queda como
// estaba.
func (s *Server) salidasPut(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	vieja, err := s.App.Store.Output.Get(ctx, id)
	if err != nil {
		failStore(w, err, "la salida")
		return
	}
	nueva := vieja
	if !decode(w, r, &nueva) {
		return
	}
	nueva.ID, nueva.ChannelID = vieja.ID, vieja.ChannelID
	if !s.salidaValida(w, &nueva) {
		return
	}
	// Cambiar a dónde va la señal deja el estado de conexión sin probar: lo
	// que decía antes era de la dirección anterior.
	if nueva.Params != vieja.Params || nueva.Driver != vieja.Driver {
		nueva.ConnectionState, nueva.Retries, nueva.LastError = "sin_probar", 0, ""
	}
	if err := s.App.Store.Output.Upsert(ctx, &nueva); err != nil {
		failStore(w, err, "guardar la salida")
		return
	}
	s.auditDiff(r, "output", &nueva.ID, vieja, nueva)
	writeJSON(w, http.StatusOK, s.App.UnaSalida(ctx, nueva))
}

// salidasDelete quita una salida. Quitar la última deja el canal sin a dónde
// mandar la señal, y el motor lo dice solo: graba en la carpeta de datos y
// avisa. No se prohíbe —el software no regaña—, pero se contesta diciéndolo.
func (s *Server) salidasDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	vieja, err := s.App.Store.Output.Get(ctx, id)
	if err != nil {
		failStore(w, err, "la salida")
		return
	}
	if err := s.App.Store.Output.Delete(ctx, id); err != nil {
		failStore(w, err, "borrar la salida")
		return
	}
	s.audit(r, "output", &id, "borrada", vieja.Driver+" "+vieja.Params, "")

	aviso := ""
	if quedan := s.App.SalidasDelCanal(ctx); len(quedan) == 0 {
		aviso = "esa era la única salida del canal: mientras no haya otra, lo que salga se graba en la carpeta de datos"
	}
	writeJSON(w, http.StatusOK, map[string]any{"borrada": id, "aviso": aviso})
}

// salidaValida deja que el driver diga si puede con esos parámetros. El texto
// del error es el del driver, que ya está escrito para una persona.
func (s *Server) salidaValida(w http.ResponseWriter, o *model.Output) bool {
	o.Name = strings.TrimSpace(o.Name)
	o.Driver = strings.TrimSpace(o.Driver)
	if o.Name == "" {
		fail(w, http.StatusBadRequest, "ponle un nombre a la salida: es el que se ve en Al aire", "nombre")
		return false
	}
	if strings.TrimSpace(o.Params) == "" {
		o.Params = "{}"
	}
	if _, err := salida.Para(*o, nil); err != nil {
		fail(w, http.StatusBadRequest, err.Error(), "parametros")
		return false
	}
	return true
}
