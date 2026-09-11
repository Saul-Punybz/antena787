package api

import (
	"net/http"
	"strings"
	"testing"

	"antena787/internal/app"
)

// Los ajustes del detector de silencio y negro (T3 de F2): GET los manda
// siempre —con el valor de fábrica si nadie los tocó, para que la tarjeta de
// Ajustes no pinte «—»— y PUT los valida en palabras claras.

func TestAjustesDelDetectorSalenConSuValorDeFabrica(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	w := c.do("GET", "/api/v1/ajustes", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("los ajustes dieron %d", w.Code)
	}
	var todos map[string]string
	c.json(w, &todos)
	if todos[app.KeySilencioUmbral] != "15" {
		t.Errorf("el umbral de silencio sale %q y de fábrica son 15 s", todos[app.KeySilencioUmbral])
	}
	if todos[app.KeyNegroUmbral] != "15" {
		t.Errorf("el umbral de negro sale %q y de fábrica son 15 s", todos[app.KeyNegroUmbral])
	}
	if todos[app.KeySilencioDevuelveControl] != "si" {
		t.Errorf("«devuelve el control» sale %q y de fábrica está encendido",
			todos[app.KeySilencioDevuelveControl])
	}
}

func TestAjustesDelDetectorSeGuardanYSeValidan(t *testing.T) {
	c := nuevo(t).conClave().entrar()

	w := c.do("PUT", "/api/v1/ajustes", map[string]string{
		app.KeySilencioUmbral:          "20",
		app.KeyNegroUmbral:             "8",
		app.KeySilencioDevuelveControl: "no",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("guardar los ajustes dio %d", w.Code)
	}
	var todos map[string]string
	c.json(w, &todos)
	if todos[app.KeySilencioUmbral] != "20" || todos[app.KeyNegroUmbral] != "8" {
		t.Fatalf("los umbrales quedaron en %q y %q",
			todos[app.KeySilencioUmbral], todos[app.KeyNegroUmbral])
	}
	if todos[app.KeySilencioDevuelveControl] != "no" {
		t.Fatalf("el interruptor quedó en %q", todos[app.KeySilencioDevuelveControl])
	}

	// Lo que no se puede guardar se contesta en palabras claras, diciendo el rango.
	for clave, valor := range map[string]string{
		app.KeySilencioUmbral: "1",
		app.KeyNegroUmbral:    "500",
	} {
		w := c.do("PUT", "/api/v1/ajustes", map[string]string{clave: valor})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("guardar %s=%s dio %d, se esperaba 400", clave, valor, w.Code)
		}
		var e errorBody
		c.json(w, &e)
		if !strings.Contains(e.Error, "segundos") {
			t.Errorf("el error de %s no dice el rango en segundos: %q", clave, e.Error)
		}
		if e.Field != clave {
			t.Errorf("el error no dice qué campo está mal: %q", e.Field)
		}
	}

	w = c.do("PUT", "/api/v1/ajustes", map[string]string{app.KeySilencioUmbral: "quince"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("guardar «quince» dio %d, se esperaba 400", w.Code)
	}

	w = c.do("PUT", "/api/v1/ajustes", map[string]string{app.KeySilencioDevuelveControl: "quizás"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("guardar «quizás» en el interruptor dio %d, se esperaba 400", w.Code)
	}
}
