package api

// F1-26 desde fuera: la parrilla mueve un bloque a mano por PUT /plan/{id},
// el bloque queda fijado, y soltarlo lo devuelve a la regla.

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"antena787/internal/model"
)

// conRegla deja un título con material y una regla diaria a la hora dada.
// Devuelve el título, para poder reconocer después sus bloques en el plan.
func conRegla(t *testing.T, c *cliente, nombre string, hora int) model.Title {
	t.Helper()
	title := c.conMaterial(nombre)
	id := title.ID
	w := c.do("POST", "/api/v1/reglas", map[string]any{
		"tipo":                  "normal",
		"title_id":              id,
		"patron_de_dias":        "LMMJVSD",
		"hora":                  hora,
		"duracion_slot_ms":      30 * 60 * 1000,
		"fecha_inicio":          string(hoy(c).Add(-1)),
		"fecha_fin":             string(hoy(c).Add(60)),
		"episodios_por_corrida": 1,
		"activa":                true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("crear la regla de %s dio %d: %s", nombre, w.Code, w.Body.String())
	}
	return title
}

// bloqueDe busca en el plan el primer bloque futuro de ese título, con
// margen de sobra para que no sea el que está saliendo ahora mismo.
func bloqueDe(t *testing.T, c *cliente, title model.Title) model.PlanItem {
	t.Helper()
	if title.MediaAssetID == nil {
		t.Fatalf("el título %q no tiene material", title.Name)
	}
	ahora := c.a.Now()
	items, err := c.a.Store.Plan.ListRange(context.Background(), c.a.ChannelID, ahora, ahora.Add(72*time.Hour))
	if err != nil {
		t.Fatalf("no pude leer el plan: %v", err)
	}
	for _, it := range items {
		if it.MediaAssetID != nil && *it.MediaAssetID == *title.MediaAssetID &&
			it.PlannedAt.After(ahora.Add(2*time.Hour)) {
			return it
		}
	}
	t.Fatalf("el plan no trae ningún bloque de %s en el futuro", title.Name)
	return model.PlanItem{}
}

// recalcular fuerza una corrida del resolver.
func recalcular(t *testing.T, c *cliente) {
	t.Helper()
	if w := c.do("POST", "/api/v1/plan/recalcular", nil); w.Code != http.StatusOK {
		t.Fatalf("recalcular dio %d: %s", w.Code, w.Body.String())
	}
}

func TestF1_26_MoverUnBloqueAManoYSoltarlo(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	kojak := conRegla(t, c, "Kojak", 14*60)
	recalcular(t, c)
	bloque := bloqueDe(t, c, kojak)

	// Se mueve cinco minutos antes, con la hora escrita como la manda la
	// interfaz: ISO 8601 con desfase.
	movido := bloque.PlannedAt.Add(-5 * time.Minute).In(time.FixedZone("AST", -4*3600))
	w := c.do("PUT", "/api/v1/plan/"+itoa(bloque.ID), map[string]any{
		"instante_planeado": movido.Format(time.RFC3339),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("mover el bloque dio %d: %s", w.Code, w.Body.String())
	}
	var out map[string]any
	c.json(w, &out)
	if out["fijado"] != true {
		t.Fatalf("el bloque movido no quedó fijado: %s", w.Body.String())
	}
	if out["id"] == nil || out["instante_planeado"] == nil || out["hora_local"] == nil {
		t.Fatalf("la respuesta no es un elemento del plan: %s", w.Body.String())
	}

	// Y sobrevive a la corrida siguiente del resolver: eso es F1-26.
	recalcular(t, c)
	despues, err := c.a.PlanItem(context.Background(), bloque.ID)
	if err != nil {
		t.Fatalf("el resolver se llevó por delante el bloque fijado: %v", err)
	}
	if !despues.PlannedAt.Equal(movido.UTC()) {
		t.Fatalf("el bloque fijado se movió a %s; se había puesto en %s", despues.PlannedAt, movido.UTC())
	}
	if !despues.Fijado {
		t.Fatal("el bloque dejó de estar fijado solo")
	}

	// Soltarlo lo devuelve a la regla.
	w = c.do("PUT", "/api/v1/plan/"+itoa(bloque.ID), map[string]any{"fijado": false})
	if w.Code != http.StatusOK {
		t.Fatalf("soltar el bloque dio %d: %s", w.Code, w.Body.String())
	}
	c.json(w, &out)
	if out["fijado"] != false {
		t.Fatalf("el bloque soltado sigue fijado: %s", w.Body.String())
	}
	recalcular(t, c)
	// Ya suelto, la hora la vuelve a mandar la regla: a las 14:00 en punto.
	ahora := c.a.Now()
	items, err := c.a.Store.Plan.ListRange(context.Background(), c.a.ChannelID, ahora, ahora.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("no pude leer el plan: %v", err)
	}
	for _, it := range items {
		if it.Fijado {
			t.Fatalf("después de soltarlo quedó un bloque fijado: %+v", it)
		}
	}
}

func TestF1_26_LosErroresDeMoverSeEntienden(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	kojak := conRegla(t, c, "Kojak", 14*60)
	tarzan := conRegla(t, c, "Tarzan", 16*60)
	recalcular(t, c)
	uno := bloqueDe(t, c, kojak)
	dos := bloqueDe(t, c, tarzan)

	// Una hora que no se entiende.
	w := c.do("PUT", "/api/v1/plan/"+itoa(uno.ID), map[string]any{"instante_planeado": "mañana a las tres"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("una hora ilegible dio %d: %s", w.Code, w.Body.String())
	}
	var e errorBody
	c.json(w, &e)
	if e.Field != "instante_planeado" || !strings.Contains(e.Error, "no entendí la hora") {
		t.Fatalf("el error de la hora ilegible no ayuda: %+v", e)
	}

	// Un bloque que no existe.
	if w := c.do("PUT", "/api/v1/plan/999999", map[string]any{"fijado": false}); w.Code != http.StatusNotFound {
		t.Fatalf("mover un bloque que no existe dio %d: %s", w.Code, w.Body.String())
	}

	// Un cuerpo que no pide nada.
	w = c.do("PUT", "/api/v1/plan/"+itoa(uno.ID), map[string]any{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("un cambio vacío dio %d: %s", w.Code, w.Body.String())
	}

	// Y encima de otro bloque: 409 con la frase clara.
	w = c.do("PUT", "/api/v1/plan/"+itoa(uno.ID), map[string]any{
		"instante_planeado": dos.PlannedAt.UTC().Format(time.RFC3339),
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("mover un bloque encima de otro dio %d: %s", w.Code, w.Body.String())
	}
	c.json(w, &e)
	if !strings.Contains(e.Error, "a esa hora ya") {
		t.Fatalf("el choque no se explica: %+v", e)
	}
	if e.Field != "instante_planeado" {
		t.Fatalf("el choque no dice qué campo arreglar: %+v", e)
	}
}

// F1-46 — Las alarmas de /estado son objetos con nivel y texto, que es lo
// que Al aire sabe pintar, y el aviso de vencimiento llega hasta ahí.
func TestF1_46_ElVencimientoLlegaAAlAire(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	title := c.conMaterial("Zoids")
	id := title.ID
	// Una regla que se acaba dentro de seis días: cae en el umbral de 7.
	w := c.do("POST", "/api/v1/reglas", map[string]any{
		"tipo":                  "normal",
		"title_id":              id,
		"patron_de_dias":        "LMMJVSD",
		"hora":                  15 * 60,
		"duracion_slot_ms":      30 * 60 * 1000,
		"fecha_inicio":          string(hoy(c)),
		"fecha_fin":             string(hoy(c).Add(6)),
		"episodios_por_corrida": 1,
		"activa":                true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("crear la regla dio %d: %s", w.Code, w.Body.String())
	}
	if w := c.do("POST", "/api/v1/plan/recalcular", nil); w.Code != http.StatusOK {
		t.Fatalf("recalcular dio %d: %s", w.Code, w.Body.String())
	}

	w = c.do("GET", "/api/v1/estado", nil)
	var estado struct {
		Alarmas []struct {
			Tipo    string `json:"tipo"`
			Nivel   string `json:"nivel"`
			Texto   string `json:"texto"`
			Detalle string `json:"detalle"`
			Accion  *struct {
				Texto string `json:"texto"`
				Ruta  string `json:"ruta"`
			} `json:"accion"`
		} `json:"alarmas"`
	}
	c.json(w, &estado)
	for _, al := range estado.Alarmas {
		if al.Tipo != "vencimiento" {
			continue
		}
		if al.Nivel != "aviso" {
			t.Fatalf("el aviso de vencimiento salió con nivel %q", al.Nivel)
		}
		if !strings.Contains(al.Texto, "Zoids") {
			t.Fatalf("el aviso no dice de qué programa habla: %q", al.Texto)
		}
		if al.Accion == nil || al.Accion.Ruta == "" {
			t.Fatalf("el aviso de vencimiento no lleva a ninguna parte: %+v", al)
		}
		return
	}
	t.Fatalf("Al aire no enseña el aviso de vencimiento: %s", w.Body.String())
}

// Los secretos de los ajustes no salen en claro, y devolverlos tapados no
// los borra.
func TestLosSecretosNoSalenEnClaro(t *testing.T) {
	c := nuevo(t).conClave().entrar()

	w := c.do("PUT", "/api/v1/ajustes", map[string]string{
		"fichas_en_linea":       "si",
		"clave_tmdb":            "una-clave-de-verdad",
		"avisos_canal":          "telegram",
		"avisos_telegram_token": "123:abc",
		"avisos_telegram_chat":  "42",
		"avisos_smtp_clave":     "otra-clave",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("guardar los ajustes dio %d: %s", w.Code, w.Body.String())
	}
	var ajustes map[string]string
	c.json(w, &ajustes)
	for _, k := range []string{"clave_tmdb", "avisos_telegram_token", "avisos_smtp_clave"} {
		if ajustes[k] != Tapado {
			t.Fatalf("el ajuste %q salió como %q y tenía que salir tapado", k, ajustes[k])
		}
	}
	if ajustes["fichas_en_linea"] != "si" || ajustes["avisos_telegram_chat"] != "42" {
		t.Fatalf("los ajustes que no son secretos tienen que ir y volver: %+v", ajustes)
	}

	// Volver a mandar lo tapado no borra el valor guardado.
	w = c.do("PUT", "/api/v1/ajustes", map[string]string{"clave_tmdb": Tapado})
	if w.Code != http.StatusOK {
		t.Fatalf("guardar los ajustes dio %d: %s", w.Code, w.Body.String())
	}
	guardado, err := c.a.Store.Settings.Get(context.Background(), "clave_tmdb")
	if err != nil || guardado != "una-clave-de-verdad" {
		t.Fatalf("devolver el valor tapado pisó la clave: %q (%v)", guardado, err)
	}

	// Y ningún secreto sale por la puerta abierta.
	w = c.do("GET", "/api/v1/estado", nil)
	if strings.Contains(w.Body.String(), "una-clave-de-verdad") ||
		strings.Contains(w.Body.String(), "123:abc") {
		t.Fatalf("/estado, que contesta sin clave, enseñó un secreto: %s", w.Body.String())
	}
}
