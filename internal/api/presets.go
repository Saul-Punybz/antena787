// presets.go son las rutas de los presets de preparación: cómo quiere el dueño
// del canal que suene y se vea su material (esquema v10).
//
// La palabra «preset» se usa tal cual a propósito: es la que dice la gente que
// trabaja en una estación. La explicación va debajo, en español.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"antena787/internal/model"
	"antena787/internal/store"
)

// presetBody es lo que se manda al crear o cambiar un preset. `ajustes` viaja
// como objeto, no como cadena: quien escribe la pantalla no tiene por qué
// serializar nada a mano.
type presetBody struct {
	Nombre  string                `json:"nombre"`
	Ajustes model.AjustesDePreset `json:"ajustes"`
}

// presetEnPantalla es un preset tal como sale: con sus ajustes ya como objeto
// y con dónde se está usando, que es lo que hay que enseñar antes de dejar
// borrarlo.
type presetEnPantalla struct {
	ID       int64                 `json:"id"`
	Nombre   string                `json:"nombre"`
	Ajustes  model.AjustesDePreset `json:"ajustes"`
	Creado   string                `json:"creado"`
	EnCanal  bool                  `json:"en_canal"`
	Titulos  int                   `json:"titulos"`
	Archivos int                   `json:"archivos"`
}

func (s *Server) presetEnPantallaDe(r *http.Request, p model.Preset) presetEnPantalla {
	var a model.AjustesDePreset
	_ = json.Unmarshal([]byte(p.Settings), &a)
	canal, titulos, archivos, _ := s.App.Store.Preset.EnUso(r.Context(), p.ID)
	return presetEnPantalla{
		ID: p.ID, Nombre: p.Name, Ajustes: a,
		Creado:  p.Created.Format("2006-01-02T15:04:05Z07:00"),
		EnCanal: canal > 0, Titulos: titulos, Archivos: archivos,
	}
}

func (s *Server) presetsList(w http.ResponseWriter, r *http.Request) {
	ps, err := s.App.Store.Preset.List(r.Context(), s.App.ChannelID)
	if err != nil {
		failStore(w, err, "los presets")
		return
	}
	out := make([]presetEnPantalla, 0, len(ps))
	for _, p := range ps {
		out = append(out, s.presetEnPantallaDe(r, p))
	}
	// El volumen vigente va con la lista: es el número que gobierna todo y la
	// pantalla tiene que poder enseñarlo sin pedirlo aparte.
	lkfs, pico, porque := s.App.VolumenDelPerfil(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"presets":        out,
		"volumen_lkfs":   lkfs,
		"pico_db":        pico,
		"volumen_porque": porque,
	})
}

func (s *Server) presetsPost(w http.ResponseWriter, r *http.Request) {
	var b presetBody
	if !decode(w, r, &b) {
		return
	}
	if strings.TrimSpace(b.Nombre) == "" {
		fail(w, http.StatusBadRequest, "ponle un nombre al preset: es como lo vas a reconocer después", "nombre")
		return
	}
	if aviso, malo := revisarAjustes(b.Ajustes); malo != "" {
		fail(w, http.StatusBadRequest, malo, "ajustes")
		return
	} else if aviso != "" {
		w.Header().Set("X-Aviso", aviso)
	}
	datos, _ := json.Marshal(b.Ajustes)
	canal := s.App.ChannelID
	p := model.Preset{ChannelID: &canal, Name: strings.TrimSpace(b.Nombre), Settings: string(datos)}
	if err := s.App.Store.Preset.Insert(r.Context(), &p); err != nil {
		failStore(w, err, "guardar el preset")
		return
	}
	s.audit(r, "preset", &p.ID, "nombre", "", p.Name)
	writeJSON(w, http.StatusOK, s.presetEnPantallaDe(r, p))
}

func (s *Server) presetsPut(w http.ResponseWriter, r *http.Request) {
	id, ok := idDeRuta(w, r)
	if !ok {
		return
	}
	viejo, err := s.App.Store.Preset.Get(r.Context(), id)
	if err != nil {
		failStore(w, err, "el preset")
		return
	}
	var b presetBody
	if !decode(w, r, &b) {
		return
	}
	if strings.TrimSpace(b.Nombre) == "" {
		b.Nombre = viejo.Name
	}
	if _, malo := revisarAjustes(b.Ajustes); malo != "" {
		fail(w, http.StatusBadRequest, malo, "ajustes")
		return
	}
	datos, _ := json.Marshal(b.Ajustes)
	nuevo := model.Preset{ID: id, Name: strings.TrimSpace(b.Nombre), Settings: string(datos), Created: viejo.Created}
	if err := s.App.Store.Preset.Update(r.Context(), nuevo); err != nil {
		failStore(w, err, "guardar el preset")
		return
	}
	s.auditDiff(r, "preset", &id, viejo, nuevo)
	writeJSON(w, http.StatusOK, s.presetEnPantallaDe(r, nuevo))
}

func (s *Server) presetsDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := idDeRuta(w, r)
	if !ok {
		return
	}
	canal, titulos, archivos, _ := s.App.Store.Preset.EnUso(r.Context(), id)
	if err := s.App.Store.Preset.Delete(r.Context(), id); err != nil {
		failStore(w, err, "borrar el preset")
		return
	}
	s.audit(r, "preset", &id, "borrado", "", "sí")
	// Lo que lo usaba no se queda roto: vuelve a heredar del nivel de arriba,
	// que es como estaba antes de que este preset existiera.
	aviso := ""
	if n := titulos + archivos + boolAInt(canal > 0); n > 0 {
		aviso = "lo que lo usaba vuelve a prepararse como el resto del canal"
	}
	writeJSON(w, http.StatusOK, map[string]any{"borrado": id, "aviso": aviso})
}

func boolAInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// revisarAjustes comprueba lo que se puede comprobar sin la máquina delante.
// Devuelve un aviso —algo que se deja hacer pero conviene saber— y un motivo
// de rechazo, que sí lo impide.
func revisarAjustes(a model.AjustesDePreset) (aviso, malo string) {
	if a.Calidad != "" && a.Calidad != "alta" && a.Calidad != "normal" && a.Calidad != "baja" {
		return "", "la calidad se escribe alta, normal o baja"
	}
	if a.GOPSegundos != nil && (*a.GOPSegundos < 1 || *a.GOPSegundos > 10) {
		return "", "los segundos entre cuadros clave van de 1 a 10; uno es lo normal"
	}
	if a.VolumenRelativoDB != nil && (*a.VolumenRelativoDB < -20 || *a.VolumenRelativoDB > 20) {
		return "", "la corrección de volumen va de −20 a +20 dB; más que eso no arregla un archivo, lo rompe"
	}
	if a.RecorteCabezaMs != nil && *a.RecorteCabezaMs < 0 {
		return "", "el recorte de cabeza no puede ser negativo"
	}
	if a.RecorteColaMs != nil && *a.RecorteColaMs < 0 {
		return "", "el recorte de cola no puede ser negativo"
	}
	return "", ""
}

// idDeRuta saca el {id} de la ruta y contesta el error si no sirve.
func idDeRuta(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest, "no reconozco ese identificador", "id")
		return 0, false
	}
	return id, true
}

// volverAPreparar manda un archivo otra vez a la cola de preparación. Hace
// falta porque cambiar un preset no rehace solo lo que ya estaba convertido:
// lo hecho se queda como estaba hasta que alguien lo pida.
//
// No convierte nada aquí mismo: lo encola. La cola ordena por hora de aire y
// se aparta cuando el aire sufre, así que pedir esto nunca puede costarle la
// señal a nadie (ADR 0008).
func (s *Server) volverAPreparar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDeRuta(w, r)
	if !ok {
		return
	}
	texto, err := s.App.VolverAPreparar(r.Context(), id)
	if err != nil {
		failStore(w, err, "volver a preparar el archivo")
		return
	}
	s.audit(r, "media_asset", &id, "volver_a_preparar", "", "sí")
	writeJSON(w, http.StatusOK, map[string]any{"encolado": id, "texto": texto})
}

// presetAplicable comprueba que el preset que se le quiere poner a algo
// exista y sea de este canal. Sin esto, la clave foránea del esquema lo
// pararía igual, pero contestando un error de base de datos: quien lo lea no
// sabría si se le cayó el canal o si escribió mal un número.
//
// Vale para los tres niveles —canal, título y archivo— porque el error es el
// mismo en los tres. `nil` significa «que herede», que siempre se permite.
func (s *Server) presetAplicable(ctx context.Context, id *int64) (string, bool) {
	if id == nil {
		return "", true
	}
	p, err := s.App.Store.Preset.Get(ctx, *id)
	if errors.Is(err, store.ErrNotFound) {
		return "ese preset ya no existe: recarga la pantalla y escoge otro", false
	}
	if err != nil {
		return "no se pudo comprobar el preset", false
	}
	if p.ChannelID != nil && *p.ChannelID != s.App.ChannelID {
		return "ese preset es de otro canal", false
	}
	return "", true
}

// sueltoDe devuelve una copia del puntero, no el mismo puntero.
//
// Hace falta antes de cada `decode` sobre una fila leída de la base, y el
// motivo es una trampa de encoding/json que no se ve leyendo el código: al
// decodificar en un campo `*int64` que **ya apunta a algo**, no crea un
// puntero nuevo — escribe dentro del que hay. Como la fila nueva se hace
// copiando la vieja (`nuevo := old`), las dos comparten el puntero, y
// entonces pasan dos cosas a la vez: la fila vieja cambia sola, y comparar
// `nuevo.X != old.X` para saber si algo cambió sale siempre que no.
//
// Lo cazó el binario de verdad el 12 de septiembre de 2026: aplicarle al
// canal un preset que no existe contestaba «FOREIGN KEY constraint failed»
// en vez de la frase escrita para eso, porque la comprobación colgaba de esa
// comparación y nunca entraba.
func sueltoDe(p *int64) *int64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
