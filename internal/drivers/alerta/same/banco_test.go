package same

import "testing"

// Los bancos miden lo único que importa para dejarlo corriendo en el retorno de
// aire 24/7: **cuánto cuesta un segundo de audio**. Cada iteración procesa
// exactamente un segundo a 48 kHz, así que el ns/op se lee directo: dividido
// por mil millones es la fracción de un núcleo que se lleva el decodificador.
//
// Y ReportAllocs está puesto a propósito: el camino caliente tiene que dar cero
// asignaciones por segundo de audio. Una sola asignación por muestra serían 48
// mil por segundo, y el recolector de basura acabaría metiéndose en medio de la
// señal.

// unSegundo prepara un segundo de PCM y lo escribe en bloques, como llega.
func banco(b *testing.B, muestras []float64, o Opciones) {
	b.Helper()
	pcm := aPCM(muestras)
	o.Buffer = 1024
	if o.Reloj == nil {
		o.Reloj = relojFijo
	}
	d, err := Nuevo(o)
	if err != nil {
		b.Fatal(err)
	}
	// Alguien tiene que vaciar el canal, o los eventos se cuentan como
	// descartados y el banco mediría otra cosa.
	go func() {
		for range d.Eventos() {
		}
	}()
	b.SetBytes(int64(len(pcm)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < len(pcm); j += 4096 {
			k := j + 4096
			if k > len(pcm) {
				k = len(pcm)
			}
			if _, err := d.Escribir(pcm[j:k]); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.StopTimer()
}

// BenchmarkSegundoDeMusica es el caso normal: el retorno de aire trayendo
// programación, sin ninguna alerta. Es el 99.99 % del tiempo de vida del
// decodificador.
func BenchmarkSegundoDeMusica(b *testing.B) {
	banco(b, musica(48000, 1, 5), Opciones{})
}

// BenchmarkSegundoDeMusicaConAtencion es lo mismo con el detector del pitido
// encendido: tres correladores más por muestra.
func BenchmarkSegundoDeMusicaConAtencion(b *testing.B) {
	banco(b, musica(48000, 1, 5), Opciones{DetectarAtencion: true})
}

// BenchmarkSegundoDeSilencio es el piso: un retorno en silencio.
func BenchmarkSegundoDeSilencio(b *testing.B) {
	banco(b, make([]float64, 48000), Opciones{})
}

// BenchmarkSegundoDeCabecera es un segundo de audio que sí trae señal SAME: el
// camino con decisiones de bit y ensamblado de bytes. Aquí sí hay asignaciones,
// una por cabecera emitida, y están bien: son por alerta, no por muestra.
func BenchmarkSegundoDeCabecera(b *testing.B) {
	g := nuevoGenerador(48000, 0.5)
	g.rafaga(cabRWT)
	for len(g.salida) < 48000 {
		g.silencio(0.05)
		g.rafaga(cabRWT)
	}
	banco(b, g.salida[:48000], Opciones{})
}
