package model

import "testing"

// La clave es lo que hace que un alias aprendido una vez sirva escriba quien
// escriba la hoja (F1-66): las tres maneras de escribir «Samurai X» tienen
// que dar lo mismo.
func TestClaveDeNombreJuntaLasTresManerasDeEscribirlo(t *testing.T) {
	for _, grupo := range [][]string{
		{"Samurai X", "SAMURAIX", "samurai-x", " samurai  x ", "Samurai_X"},
		{"Rurouni Kenshin", "rurouni kenshin", "Rurouni  Kenshin"},
		{"Zorro (1957)", "zorro 1957", "ZORRO-1957"},
		{"El Chavo", "chavo", "El  Chavo"},
		{"The Green Hornet", "Green Hornet", "green-hornet"},
		{"Gilligan's Island", "Gilligans Island", "gilligan’s island"},
		{"Mañana", "manana", "MAÑANA"},
	} {
		primera := ClaveDeNombre(grupo[0])
		if primera == "" {
			t.Fatalf("«%s» no dejó clave", grupo[0])
		}
		for _, otro := range grupo[1:] {
			if c := ClaveDeNombre(otro); c != primera {
				t.Errorf("«%s» dio %q y «%s» dio %q: tenían que ser la misma clave",
					grupo[0], primera, otro, c)
			}
		}
	}
}

func TestClaveDeNombreDistingueLoQueEsDistinto(t *testing.T) {
	for _, par := range [][2]string{
		{"Saber Marionette J", "Saber Marionette R"},
		{"Zorro (1957)", "Zorro (1990)"},
		{"Samurai X", "Rurouni Kenshin"},
	} {
		if a, b := ClaveDeNombre(par[0]), ClaveDeNombre(par[1]); a == b {
			t.Errorf("«%s» y «%s» dieron la misma clave %q", par[0], par[1], a)
		}
	}
}

func TestClaveDeNombreCasosDeBorde(t *testing.T) {
	casos := map[string]string{
		"":                     "",
		"   ":                  "",
		"!!!":                  "",
		"La":                   "la", // artículo solo: no se queda sin nada
		"Comics 9th Art":       "comics9thart",
		`Hack\_Legend`:         "hacklegend", // escapes de markdown deshechos
		"You're Under Arrest!": "youreunderarrest",
	}
	for entra, espera := range casos {
		if got := ClaveDeNombre(entra); got != espera {
			t.Errorf("ClaveDeNombre(%q) = %q, se esperaba %q", entra, got, espera)
		}
	}
}
