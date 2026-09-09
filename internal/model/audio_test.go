package model

import "testing"

// F1-60 — de varias pistas rotuladas por idioma, la preferida es la primera
// que está en el idioma que pide el canal; si ninguna lo está, la primera del
// archivo.
func TestPistaPreferida(t *testing.T) {
	tres := MediaAsset{PistasAudio: []PistaAudio{
		{Indice: 0, Idioma: "en", Canales: 6, Titulo: "English"},
		{Indice: 1, Idioma: "es", Canales: 2, Titulo: "Español"},
		{Indice: 2, Idioma: "es", Canales: 2, Titulo: "Español latino"},
	}}
	etiquetas := MediaAsset{PistasAudio: []PistaAudio{
		{Indice: 0, Idioma: "POR", Canales: 2},
		{Indice: 1, Idioma: "Spa", Canales: 2},
		{Indice: 2, Idioma: " ENG ", Canales: 2},
	}}
	sinPistas := MediaAsset{}

	casos := []struct {
		nombre   string
		archivo  MediaAsset
		idioma   string
		esperado int
	}{
		{"la primera en español", tres, "es", 1},
		{"la primera en inglés", tres, "en", 0},
		{"nadie habla francés: la primera del archivo", tres, "fr", 0},
		{"sin idioma pedido: la primera del archivo", tres, "", 0},
		{"spa es el mismo español", etiquetas, "es", 1},
		{"esp también", etiquetas, "esp", 1},
		{"eng es el mismo inglés, con mayúsculas y espacios", etiquetas, "en", 2},
		{"un archivo sin pistas declaradas: la primera", sinPistas, "es", 0},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := c.archivo.PistaPreferida(c.idioma); got != c.esperado {
				t.Fatalf("PistaPreferida(%q) = %d, se esperaba %d", c.idioma, got, c.esperado)
			}
		})
	}
}

// El índice que se devuelve es el del archivo (el que usa el mux), no la
// posición en la lista: si las pistas vienen desordenadas, manda el índice.
func TestPistaPreferidaDevuelveElIndiceDelArchivo(t *testing.T) {
	a := MediaAsset{PistasAudio: []PistaAudio{
		{Indice: 3, Idioma: "es", Canales: 2},
		{Indice: 0, Idioma: "en", Canales: 2},
	}}
	if got := a.PistaPreferida("es"); got != 3 {
		t.Fatalf("PistaPreferida(\"es\") = %d, se esperaba 3", got)
	}
}
