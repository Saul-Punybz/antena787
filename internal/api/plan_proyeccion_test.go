package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// La tira semanal enseña lo que las reglas van a poner, no solo las 48 h que
// el plan ya tiene escritas: una semana fuera de la ventana sale con sus
// programas, marcados como proyección y sin plan_id (modo sombra, 9 sept 2026).
func TestLaSemanaProyectaLoQueLasReglasPondran(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	title := c.conMaterial("Kojak")
	w := c.do("POST", "/api/v1/reglas", map[string]any{
		"tipo":                  "normal",
		"title_id":              title.ID,
		"patron_de_dias":        "LMMJVSD",
		"hora":                  14 * 60,
		"duracion_slot_ms":      30 * 60 * 1000,
		"fecha_inicio":          string(hoy(c)),
		"fecha_fin":             string(hoy(c).Add(30)),
		"episodios_por_corrida": 1,
	})
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("crear la regla dio %d: %s", w.Code, w.Body.String())
	}
	if w := c.do("POST", "/api/v1/plan/recalcular", nil); w.Code != http.StatusOK {
		t.Fatalf("recalcular dio %d: %s", w.Code, w.Body.String())
	}

	desde := hoy(c).Add(10) // diez días después: fuera de las 48 h resueltas
	w = c.do("GET", "/api/v1/plan/semana?desde="+string(desde), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/plan/semana dio %d: %s", w.Code, w.Body.String())
	}
	var semana struct {
		Dias []struct {
			Dia     string `json:"dia"`
			Franjas []struct {
				Hora   string  `json:"hora"`
				Titulo *string `json:"titulo"`
				PlanID *int64  `json:"plan_id"`
				Estado string  `json:"estado"`
			} `json:"franjas"`
		} `json:"dias"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &semana); err != nil {
		t.Fatal(err)
	}
	if len(semana.Dias) != 7 {
		t.Fatalf("la semana trae %d días", len(semana.Dias))
	}
	vistos := 0
	for _, d := range semana.Dias {
		for _, f := range d.Franjas {
			if f.Hora != "14:00" || f.Titulo == nil {
				continue
			}
			if *f.Titulo != "Kojak" {
				t.Fatalf("%s a las 14:00 sale %q y se esperaba Kojak", d.Dia, *f.Titulo)
			}
			if f.PlanID != nil {
				t.Fatalf("%s: una proyección no tiene plan_id (%d): no se puede arrastrar lo que no existe", d.Dia, *f.PlanID)
			}
			if f.Estado != EstadoProyectado {
				t.Fatalf("%s: la franja proyectada dice estado %q", d.Dia, f.Estado)
			}
			vistos++
		}
	}
	if vistos != 7 {
		t.Fatalf("Kojak tenía que salir los 7 días a las 14:00 y salió %d", vistos)
	}
}
