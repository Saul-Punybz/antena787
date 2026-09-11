package same

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGrabacionReal corre contra grabaciones de verdad, si hay alguna.
//
// El resto de las pruebas usa audio fabricado, que prueba el protocolo pero no
// las mañas de un ENDEC real ni de la cadena de audio de una estación. Esta
// prueba existe para que el día que haya una grabación —la prueba semanal de
// CAtv grabada del retorno, por ejemplo— entre al repositorio y se verifique
// sola, sin escribir código nuevo: se deja el `.s16` en testdata/ con un `.txt`
// al lado que diga qué cabecera debe salir. Los detalles y la procedencia están
// en testdata/LEEME.md.
//
// Hoy no hay ninguna, así que se salta.
func TestGrabacionReal(t *testing.T) {
	archivos, err := filepath.Glob(filepath.Join("testdata", "*.s16"))
	if err != nil {
		t.Fatal(err)
	}
	if len(archivos) == 0 {
		t.Skip("no hay grabaciones reales en testdata/ (ver testdata/LEEME.md)")
	}
	for _, a := range archivos {
		t.Run(filepath.Base(a), func(t *testing.T) {
			pcm, err := os.ReadFile(a)
			if err != nil {
				t.Fatal(err)
			}
			esperadas := esperadasDe(t, strings.TrimSuffix(a, ".s16")+".txt")

			d, err := Nuevo(Opciones{Reloj: relojFijo, Buffer: 256, DetectarAtencion: true})
			if err != nil {
				t.Fatal(err)
			}
			var evs []Evento
			listo := make(chan struct{})
			go func() {
				for ev := range d.Eventos() {
					evs = append(evs, ev)
				}
				close(listo)
			}()
			for i := 0; i < len(pcm); i += 4096 {
				j := i + 4096
				if j > len(pcm) {
					j = len(pcm)
				}
				if _, err := d.Escribir(pcm[i:j]); err != nil {
					t.Fatal(err)
				}
			}
			if err := d.Cerrar(); err != nil {
				t.Fatal(err)
			}
			<-listo

			cs := cabecerasDe(evs)
			for _, c := range cs {
				t.Logf("cabecera %q (confianza %.2f, %d repeticiones) en %v",
					c.Crudo, c.Confianza, c.Repeticiones, c.Desplazamiento)
			}
			for _, a := range atencionesDe(evs) {
				t.Logf("señal de atención de %v en %v", a.Duracion, a.Desplazamiento)
			}
			for _, f := range finesDe(evs) {
				t.Logf("fin de mensaje (%d repeticiones) en %v", f.Repeticiones, f.Desplazamiento)
			}
			if len(esperadas) == 0 {
				if len(cs) == 0 {
					t.Error("no se leyó ninguna cabecera")
				}
				return
			}
			for _, quiero := range esperadas {
				hallada := false
				for _, c := range cs {
					if c.Crudo == quiero {
						hallada = true
						break
					}
				}
				if !hallada {
					t.Errorf("no se leyó la cabecera esperada %q", quiero)
				}
			}
		})
	}
}

// esperadasDe lee el archivo de al lado con las cabeceras que deben salir. Si no
// existe, la prueba se conforma con que se lea alguna.
func esperadasDe(t *testing.T, ruta string) []string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimRight(l, "\r")
		if l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}
