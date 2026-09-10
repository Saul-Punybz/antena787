package engine

import (
	"testing"
	"time"
)

// La fuente de lista es la del arnés: da vueltas a los mismos clips, cada uno
// entero, y no mira el reloj.
func TestListaDaLaVueltaYNoFijaCortes(t *testing.T) {
	uno := Clip{Path: "/medios/uno.mkv", Name: "uno"}
	dos := Clip{Path: "/medios/dos.mkv", Name: "dos"}
	relleno := Clip{Path: "/medios/relleno.mkv", Name: "relleno"}
	l := NuevaLista([]Clip{uno, dos}, relleno)

	quiere := []string{"uno", "dos", "uno", "dos", "uno"}
	ahora := time.Now()
	for i, nombre := range quiere {
		clip, hasta, err := l.Next(ahora)
		if err != nil {
			t.Fatalf("la vuelta %d falló: %v", i, err)
		}
		if clip.Name != nombre {
			t.Fatalf("la vuelta %d dio %q y tocaba %q", i, clip.Name, nombre)
		}
		if !hasta.IsZero() {
			t.Fatalf("la vuelta %d fijó un corte (%s) y la lista no fija cortes", i, hasta)
		}
	}
	if l.Filler().Name != "relleno" {
		t.Fatalf("el relleno de la lista es %q", l.Filler().Name)
	}
}

// Sin clips todo lo que sale es el relleno: la lista vacía no deja el aire
// sin nada que poner.
func TestListaVaciaSaleElRelleno(t *testing.T) {
	relleno := Clip{Path: "/medios/cartel.mkv", Name: "cartel"}
	l := NuevaLista(nil, relleno)
	clip, _, err := l.Next(time.Now())
	if err != nil {
		t.Fatalf("la lista vacía falló: %v", err)
	}
	if clip.Name != "cartel" {
		t.Fatalf("la lista vacía dio %q", clip.Name)
	}
}
