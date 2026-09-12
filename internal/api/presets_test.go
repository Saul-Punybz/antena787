package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Los presets de preparación (esquema v10): que se escriban, se apliquen a
// los tres niveles y se quiten sin dejar nada roto detrás.
//
// La resolución de los tres niveles ya estaba probada en internal/app. Lo que
// faltaba probar es lo otro: que se pueda **llegar** a ella desde fuera.

func TestElPresetSeCreaSeAplicaAlCanalYSeQuita(t *testing.T) {
	c := nuevo(t).conClave().entrar()

	// Un canal nuevo no tiene ninguno, y aun así contesta el volumen que lo
	// gobierna: es lo que la pantalla enseña arriba del todo.
	w := c.do("GET", "/api/v1/presets", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/presets dio %d: %s", w.Code, w.Body.String())
	}
	var lista struct {
		Presets []map[string]any `json:"presets"`
		LKFS    float64          `json:"volumen_lkfs"`
		Porque  string           `json:"volumen_porque"`
	}
	c.json(w, &lista)
	if len(lista.Presets) != 0 {
		t.Fatalf("un canal nuevo no tiene presets y vinieron %d", len(lista.Presets))
	}
	if lista.LKFS == 0 || strings.TrimSpace(lista.Porque) == "" {
		t.Fatalf("el volumen del canal viene sin número o sin motivo: %+v", lista)
	}

	// Uno de verdad: material viejo que viene bajito.
	w = c.do("POST", "/api/v1/presets", map[string]any{
		"nombre": "Películas viejas",
		"ajustes": map[string]any{
			"volumen_relativo_db": 3,
			"recorte_cabeza_ms":   2000,
			"calidad":             "normal",
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("crear el preset dio %d: %s", w.Code, w.Body.String())
	}
	var creado map[string]any
	c.json(w, &creado)
	id := int64(creado["id"].(float64))
	if creado["en_canal"] != false {
		t.Fatal("un preset recién creado no se le aplica a nada todavía")
	}

	// Aplicarlo a todo el canal es el nivel más general de los tres.
	w = c.do("PUT", "/api/v1/canal", map[string]any{"preset_id": id})
	if w.Code != http.StatusOK {
		t.Fatalf("aplicar el preset al canal dio %d: %s", w.Code, w.Body.String())
	}
	w = c.do("GET", "/api/v1/presets", nil)
	c.json(w, &lista)
	if len(lista.Presets) != 1 || lista.Presets[0]["en_canal"] != true {
		t.Fatalf("el preset no dice que está puesto en el canal: %+v", lista.Presets)
	}

	// Quitarlo no deja al canal apuntando a algo que ya no existe: lo que lo
	// usaba vuelve a heredar, que es como estaba antes.
	w = c.do("DELETE", "/api/v1/presets/"+itoa(id), map[string]any{})
	if w.Code != http.StatusOK {
		t.Fatalf("borrar el preset dio %d: %s", w.Code, w.Body.String())
	}
	var borrado struct {
		Aviso string `json:"aviso"`
	}
	c.json(w, &borrado)
	if borrado.Aviso == "" {
		t.Fatal("se borró un preset que estaba en uso y no se dijo qué pasa con lo que lo usaba")
	}
	w = c.do("GET", "/api/v1/canal", nil)
	var canal map[string]any
	c.json(w, &canal)
	if canal["preset_id"] != nil {
		t.Fatalf("el canal quedó apuntando a un preset borrado: %v", canal["preset_id"])
	}
}

// TestUnPresetQueNoExisteSeDiceConPalabras es la prueba del fallo del 12 de
// septiembre de 2026: la comprobación estaba escrita y no se ejecutaba nunca.
//
// Colgaba de `nuevo.PresetID != old.PresetID`, y esa comparación **siempre**
// salía «iguales» porque al decodificar sobre un `*int64` que ya apunta a
// algo, encoding/json escribe dentro del puntero en vez de crear uno nuevo —
// y las dos filas comparten puntero. El resultado que veía una persona era
// «FOREIGN KEY constraint failed (787)».
//
// Lo cazó abrir el producto contra el binario, no la suite. Por eso está aquí.
func TestUnPresetQueNoExisteSeDiceConPalabras(t *testing.T) {
	c := nuevo(t).conClave().entrar()

	// Primero uno bueno, para que el canal quede con un puntero no nulo: ese
	// es el estado en el que el fallo aparecía.
	w := c.do("POST", "/api/v1/presets", map[string]any{
		"nombre": "El de siempre", "ajustes": map[string]any{"calidad": "normal"},
	})
	var creado map[string]any
	c.json(w, &creado)
	id := int64(creado["id"].(float64))
	if w = c.do("PUT", "/api/v1/canal", map[string]any{"preset_id": id}); w.Code != http.StatusOK {
		t.Fatalf("aplicar el preset bueno dio %d: %s", w.Code, w.Body.String())
	}

	// Y ahora uno que no existe.
	w = c.do("PUT", "/api/v1/canal", map[string]any{"preset_id": 9999})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("un preset inexistente se aceptó con %d: %s", w.Code, w.Body.String())
	}
	var e errorBody
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("el error no es JSON: %s", w.Body.String())
	}
	if e.Field != "preset_id" {
		t.Fatalf("el error no señala el campo: %+v", e)
	}
	if strings.Contains(strings.ToLower(e.Error), "constraint") ||
		strings.Contains(strings.ToLower(e.Error), "foreign key") {
		t.Fatalf("se le enseñó a una persona un error de base de datos: %q", e.Error)
	}

	// Y el canal se quedó con el que tenía: un intento fallido no le cambia
	// nada por debajo.
	w = c.do("GET", "/api/v1/canal", nil)
	var canal map[string]any
	c.json(w, &canal)
	if canal["preset_id"] == nil || int64(canal["preset_id"].(float64)) != id {
		t.Fatalf("el canal cambió de preset con una petición que falló: %v", canal["preset_id"])
	}
}

// TestElPresetDelArchivoNoSeBorraAlCambiarOtraCosa: PUT /material/{id} manda
// el formulario entero cada vez. Si «no venía preset_id» se confundiera con
// «quítalo», cambiarle la pista de sonido a un archivo le borraría el preset.
func TestElPresetDelArchivoNoSeBorraAlCambiarOtraCosa(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	titulo := c.conMaterial("Kojak")
	if titulo.MediaAssetID == nil {
		t.Fatal("el título de prueba vino sin archivo")
	}
	archivo := *titulo.MediaAssetID

	w := c.do("POST", "/api/v1/presets", map[string]any{
		"nombre": "Ya viene listo", "ajustes": map[string]any{"no_tocar": true},
	})
	var creado map[string]any
	c.json(w, &creado)
	id := int64(creado["id"].(float64))

	w = c.do("PUT", "/api/v1/material/"+itoa(archivo), map[string]any{"preset_id": id})
	if w.Code != http.StatusOK {
		t.Fatalf("ponerle el preset al archivo dio %d: %s", w.Code, w.Body.String())
	}

	// Otra cosa cualquiera, sin mandar preset_id.
	w = c.do("PUT", "/api/v1/material/"+itoa(archivo), map[string]any{"negro_intencional": true})
	if w.Code != http.StatusOK {
		t.Fatalf("cambiar otra cosa dio %d: %s", w.Code, w.Body.String())
	}
	var m map[string]any
	c.json(w, &m)
	if m["preset_id"] == nil || int64(m["preset_id"].(float64)) != id {
		t.Fatalf("cambiar otra cosa se llevó el preset del archivo: %v", m["preset_id"])
	}

	// Y mandarlo en `null` sí lo quita: son dos cosas distintas.
	w = c.do("PUT", "/api/v1/material/"+itoa(archivo), map[string]any{"preset_id": nil})
	if w.Code != http.StatusOK {
		t.Fatalf("quitar el preset dio %d: %s", w.Code, w.Body.String())
	}
	c.json(w, &m)
	if m["preset_id"] != nil {
		t.Fatalf("se pidió quitar el preset y sigue puesto: %v", m["preset_id"])
	}
}
