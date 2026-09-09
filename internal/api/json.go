package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"antena787/internal/model"
	"antena787/internal/store"
)

// maxBody es lo más grande que se acepta en un cuerpo JSON. La hoja de CAtv
// pegada entera son unos 40 KB; ocho megas es holgura de sobra y a la vez un
// tope para que nadie llene la memoria con un POST.
const maxBody = 8 << 20

// errorBody es el error tal como lo lee una persona (docs/API.md).
type errorBody struct {
	Error string `json:"error"`
	Field string `json:"campo,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// fail contesta con un error en cristiano. field es opcional: el campo del
// formulario que la persona tiene que arreglar.
func fail(w http.ResponseWriter, code int, msg, field string) {
	writeJSON(w, code, errorBody{Error: msg, Field: field})
}

// failf es fail con formato.
func failf(w http.ResponseWriter, code int, field, format string, args ...any) {
	fail(w, code, fmt.Sprintf(format, args...), field)
}

// decode lee el cuerpo JSON. El error que devuelve ya está en cristiano.
func decode(w http.ResponseWriter, r *http.Request, into any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	if err := dec.Decode(into); err != nil {
		fail(w, http.StatusBadRequest, "no entendí lo que mandaste: "+plainJSON(err), "")
		return false
	}
	return true
}

// jsonUnmarshal es json.Unmarshal, para que el resto del paquete no tenga
// que importar encoding/json solo para esto.
func jsonUnmarshal(data []byte, into any) error { return json.Unmarshal(data, into) }

// jsonMarshal es json.Marshal, por la misma razón.
func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

// plainJSON traduce el error del decodificador a algo que se pueda leer.
func plainJSON(err error) string {
	var ut *json.UnmarshalTypeError
	if errors.As(err, &ut) {
		return fmt.Sprintf("el campo %q espera %s y llegó %s", ut.Field, ut.Type, ut.Value)
	}
	if errors.Is(err, io.EOF) {
		return "el cuerpo venía vacío"
	}
	return err.Error()
}

// pathID lee el {id} de la ruta.
func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := r.PathValue(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		failf(w, http.StatusBadRequest, name, "%q no es un número de %s", raw, name)
		return 0, false
	}
	return id, true
}

// failStore traduce un error del store al idioma de la gente y elige el
// código: lo que la persona puede arreglar es 400, lo que no, 500.
func failStore(w http.ResponseWriter, err error, what string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		failf(w, http.StatusNotFound, "", "no encontré %s", what)
	case errors.Is(err, store.ErrDates):
		fail(w, http.StatusBadRequest, store.ErrDates.Error(), "fecha_fin")
	case errors.Is(err, store.ErrOverlap):
		fail(w, http.StatusBadRequest, store.ErrOverlap.Error(), "hora")
	default:
		failf(w, http.StatusInternalServerError, "", "no se pudo %s: %s", what, err)
	}
}

// queryDay lee un día de emisión de la consulta. Vacío = el de hoy.
func queryDay(r *http.Request, key string, def model.Day) (model.Day, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return def, nil
	}
	return model.ParseDay(raw)
}

// queryTime lee un instante RFC 3339 de la consulta.
func queryTime(r *http.Request, key string, def time.Time) (time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return def, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if d, err := model.ParseDay(raw); err == nil {
		return d.Time(time.UTC), nil
	}
	return def, fmt.Errorf("%q no es una fecha: se espera AAAA-MM-DD o una hora completa", raw)
}
