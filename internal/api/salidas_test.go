package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// El CRUD de las salidas (T2 de F2): que se puedan escribir, cambiar y quitar,
// que lo que un multiplexor no aceptaría se conteste en cristiano y no se
// guarde, y que Al aire las reciba en /estado.

func TestLasSalidasSeEscribenSeCambianYSeQuitan(t *testing.T) {
	c := nuevo(t).conClave().entrar()

	// Sin ninguna: la lista viene vacía y con los drivers que se pueden
	// ofrecer, `udp-ts` entre ellos (F2-50).
	w := c.do("GET", "/api/v1/salidas", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/salidas dio %d: %s", w.Code, w.Body.String())
	}
	var lista struct {
		Salidas []map[string]any `json:"salidas"`
		Drivers []map[string]any `json:"drivers_disponibles"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &lista); err != nil {
		t.Fatalf("la lista de salidas no es JSON: %s", w.Body.String())
	}
	if len(lista.Salidas) != 0 {
		t.Fatalf("un canal nuevo no tiene salidas y vinieron %d", len(lista.Salidas))
	}
	var hayUDP bool
	for _, d := range lista.Drivers {
		if d["driver"] == "udp-ts" && d["nombre"] != "" {
			hayUDP = true
		}
	}
	if !hayUDP {
		t.Fatalf("udp-ts tiene que estar entre los drivers que se ofrecen: %v", lista.Drivers)
	}

	// Una al multiplexor.
	w = c.do("POST", "/api/v1/salidas", map[string]any{
		"nombre":           "Transmisor",
		"driver":           "udp-ts",
		"parametros":       `{"destino": "192.168.1.50:1234", "program": 7}`,
		"objetivo_volumen": -24,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("crear la salida dio %d: %s", w.Code, w.Body.String())
	}
	var creada map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &creada); err != nil {
		t.Fatalf("la salida creada no es JSON: %s", w.Body.String())
	}
	if texto, _ := creada["texto"].(string); !strings.Contains(texto, "al receptor 192.168.1.50:1234") {
		t.Fatalf("la salida no dice a dónde va: %q", texto)
	}
	id := int64(creada["id"].(float64))

	// Lo que un multiplexor descartaría no se guarda, y se contesta con el
	// campo y la frase que ve la persona.
	w = c.do("PUT", "/api/v1/salidas/"+itoa(id), map[string]any{
		"parametros": `{"destino": "192.168.1.50:1234", "pcr_ms": 80}`,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("un PCR de 80 ms se guardó con %d: %s", w.Code, w.Body.String())
	}
	var e errorBody
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if !strings.Contains(e.Error, "cada 40 ms o menos") || e.Field != "parametros" {
		t.Fatalf("el error del PCR es %+v", e)
	}

	// Cambiar a dónde va deja el estado sin probar: lo que decía antes era de
	// la dirección anterior.
	w = c.do("PUT", "/api/v1/salidas/"+itoa(id), map[string]any{
		"parametros": `{"destino": "239.1.1.1:1234", "ttl": 4}`,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("cambiar la salida dio %d: %s", w.Code, w.Body.String())
	}
	var cambiada map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &cambiada)
	if texto, _ := cambiada["texto"].(string); !strings.Contains(texto, "al grupo 239.1.1.1:1234, 4 salto(s)") {
		t.Fatalf("la salida a un grupo no lo dice: %q", texto)
	}
	if cambiada["estado_conexion"] != "sin_probar" {
		t.Fatalf("tras cambiar la dirección el estado es %v", cambiada["estado_conexion"])
	}

	// Y en /estado van las salidas, que es lo que pinta Al aire.
	w = c.do("GET", "/api/v1/estado", nil)
	salidas := campo(t, w.Body.Bytes(), "salidas")
	var enEstado []map[string]any
	if err := json.Unmarshal(salidas, &enEstado); err != nil || len(enEstado) != 1 {
		t.Fatalf("/estado no lleva la salida: %s", salidas)
	}
	exigeTodas(t, "GET /estado → salidas[0]", mustJSON(t, enEstado[0]), "Salida")

	// Quitar la última lo dice: mientras no haya otra, lo que salga se graba.
	w = c.do("DELETE", "/api/v1/salidas/"+itoa(id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("borrar la salida dio %d: %s", w.Code, w.Body.String())
	}
	var borrada map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &borrada)
	if aviso, _ := borrada["aviso"].(string); !strings.Contains(aviso, "se graba en la carpeta de datos") {
		t.Fatalf("borrar la última salida no avisó: %q", aviso)
	}
	if w := c.do("DELETE", "/api/v1/salidas/"+itoa(id), nil); w.Code != http.StatusNotFound {
		t.Fatalf("borrar dos veces dio %d", w.Code)
	}
}

// Una salida sin nombre no se guarda: el nombre es lo que se ve en Al aire.
func TestUnaSalidaSinNombreNoSeGuarda(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	w := c.do("POST", "/api/v1/salidas", map[string]any{
		"driver":     "udp-ts",
		"parametros": `{"destino": "192.168.1.50:1234"}`,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("una salida sin nombre dio %d: %s", w.Code, w.Body.String())
	}
	var e errorBody
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if e.Field != "nombre" {
		t.Fatalf("el error no señala el nombre: %+v", e)
	}
}

// mustJSON vuelve a serializar un objeto ya leído, para pasárselo a la prueba
// de contrato.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("no pude serializar: %v", err)
	}
	return raw
}
