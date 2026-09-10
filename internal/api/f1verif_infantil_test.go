package api

import (
	"net/http"
	"testing"
)

// F1-76 (la parte de la API) — la marca de programa infantil educativo (E/I)
// viaja por la API: se pone con PUT /biblioteca/{id} y vuelve tanto en la
// lista como en la ficha del título.
func TestF1Verif76InfantilCoreViajaPorLaAPI(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	titulo := c.conMaterial("Carmen Sandiego")

	// De entrada, apagado: el cumplimiento se ofrece, no se supone.
	var ficha map[string]any
	w := c.do("GET", "/api/v1/biblioteca/"+itoa(titulo.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("la ficha dio %d: %s", w.Code, w.Body.String())
	}
	c.json(w, &ficha)
	if v, hay := ficha["infantil_core"]; !hay || v != false {
		t.Fatalf("la ficha tiene que traer infantil_core en falso; trajo %v (hay=%v)", v, hay)
	}

	// Marcarlo desde la ficha.
	w = c.do("PUT", "/api/v1/biblioteca/"+itoa(titulo.ID), map[string]any{
		"nombre":        titulo.Name,
		"infantil_core": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("marcarlo dio %d: %s", w.Code, w.Body.String())
	}
	var guardado map[string]any
	c.json(w, &guardado)
	if guardado["infantil_core"] != true {
		t.Fatalf("la respuesta del PUT no trae la marca: %v", guardado["infantil_core"])
	}

	// Y queda marcado: la ficha y la lista lo dicen las dos.
	ficha = nil
	c.json(c.do("GET", "/api/v1/biblioteca/"+itoa(titulo.ID), nil), &ficha)
	if ficha["infantil_core"] != true {
		t.Fatalf("la ficha perdió la marca: %v", ficha["infantil_core"])
	}
	var lista []map[string]any
	c.json(c.do("GET", "/api/v1/biblioteca", nil), &lista)
	if len(lista) == 0 {
		t.Fatal("la biblioteca salió vacía")
	}
	hallado := false
	for _, t2 := range lista {
		if t2["nombre"] == titulo.Name {
			hallado = true
			if t2["infantil_core"] != true {
				t.Fatalf("la lista de la biblioteca no trae la marca: %v", t2["infantil_core"])
			}
		}
	}
	if !hallado {
		t.Fatalf("el título no aparece en la biblioteca: %v", lista)
	}

	// Un PUT que no habla del asunto no apaga la marca: lo que no se manda se
	// queda como estaba.
	w = c.do("PUT", "/api/v1/biblioteca/"+itoa(titulo.ID), map[string]any{"sinopsis": "Geografía y misterio"})
	if w.Code != http.StatusOK {
		t.Fatalf("cambiar la sinopsis dio %d: %s", w.Code, w.Body.String())
	}
	guardado = nil
	c.json(w, &guardado)
	if guardado["infantil_core"] != true {
		t.Fatal("cambiar otra cosa de la ficha apagó la marca de programación infantil")
	}
}
