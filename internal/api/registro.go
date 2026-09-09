package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"antena787/internal/app"
	"antena787/internal/model"
)

// incidentes es la bitácora de lo que el sistema hizo solo. Sin fechas, la
// última semana.
func (s *Server) incidentes(w http.ResponseWriter, r *http.Request) {
	now := s.Now()
	from, err := queryTime(r, "desde", now.AddDate(0, 0, -7))
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error(), "desde")
		return
	}
	to, err := queryTime(r, "hasta", now.Add(time.Minute))
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error(), "hasta")
		return
	}
	list, err := s.App.Store.Incident.List(r.Context(), s.App.ChannelID, from, to)
	if err != nil {
		failStore(w, err, "leer los incidentes")
		return
	}
	out := make([]incidenteOut, 0, len(list))
	for _, inc := range list {
		out = append(out, incidenteOut{Incident: inc, Texto: app.TextoDeIncidente(inc.Kind)})
	}
	writeJSON(w, http.StatusOK, out)
}

// incidenteOut es un incidente tal como lo pinta la bitácora de Al aire
// (web/src/lib/tipos.ts, `Incidente`): la fila entera más la frase en
// cristiano de su tipo, para que la pantalla no tenga que saber qué es un
// `salto_de_reloj`.
type incidenteOut struct {
	model.Incident
	Texto string `json:"texto"`
}

// DefaultAuditLimit es cuántas entradas devuelve la auditoría si nadie dice
// otra cosa.
const DefaultAuditLimit = 500

// auditoria devuelve el audit_log, opcionalmente filtrado por entidad e id.
func (s *Server) auditoria(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := DefaultAuditLimit
	if raw := strings.TrimSpace(q.Get("limite")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			fail(w, http.StatusBadRequest, "el límite tiene que ser un número mayor que cero", "limite")
			return
		}
		limit = n
	}
	entidad := strings.TrimSpace(q.Get("entidad"))
	var id *int64
	if raw := strings.TrimSpace(q.Get("id")); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			fail(w, http.StatusBadRequest, "el id tiene que ser un número", "id")
			return
		}
		id = &n
	}

	// Se pide sin límite cuando hay filtro: filtrar después de recortar
	// devolvería menos de lo que hay.
	pedir := limit
	if entidad != "" || id != nil {
		pedir = 0
	}
	all, err := s.App.Store.Audit.List(r.Context(), pedir)
	if err != nil {
		failStore(w, err, "leer la bitácora")
		return
	}
	out := []model.AuditEntry{}
	for _, e := range all {
		if entidad != "" && e.Entity != entidad {
			continue
		}
		if id != nil && (e.EntityID == nil || *e.EntityID != *id) {
			continue
		}
		out = append(out, e)
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	writeJSON(w, http.StatusOK, out)
}

// auditoriaVerificar recorre la cadena de hash entera. Es lo que contesta
// "¿alguien tocó la bitácora?" con un sí o un no, no con un quizá.
func (s *Server) auditoriaVerificar(w http.ResponseWriter, r *http.Request) {
	rota, err := s.App.Store.Audit.Verify(r.Context())
	if err != nil {
		failStore(w, err, "verificar la bitácora")
		return
	}
	if rota == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"intacta": true,
			"texto":   "la bitácora está intacta: la cadena de hash cuadra de principio a fin",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"intacta": false,
		"texto":   "la bitácora está rota a partir de esta entrada: alguien la editó por fuera del programa",
		"entrada": rota,
	})
}
