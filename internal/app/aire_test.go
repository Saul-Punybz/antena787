package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"antena787/internal/model"
)

// Pruebas de la puerta del aire (F2-118): qué se comprueba antes de dejar
// encender, qué se deja encender con avisos, y que apagar apague de verdad.

// comprobacion busca una comprobación por su clave.
func comprobacion(t *testing.T, comps []Comprobacion, clave string) Comprobacion {
	t.Helper()
	for _, c := range comps {
		if c.Clave == clave {
			return c
		}
	}
	t.Fatalf("no hay comprobación %q; están %v", clave, clavesDe(comps))
	return Comprobacion{}
}

func clavesDe(comps []Comprobacion) []string {
	out := make([]string, 0, len(comps))
	for _, c := range comps {
		out = append(out, c.Clave)
	}
	return out
}

// conSalidaAArchivo deja una salida configurada que abre de verdad.
func conSalidaAArchivo(t *testing.T, a *App, ruta string) model.Output {
	t.Helper()
	o := model.Output{
		ChannelID: a.ChannelID, Name: "Grabación", Driver: "archivo",
		Params: fmt.Sprintf(`{"ruta": %q}`, ruta),
	}
	if err := a.Store.Output.Upsert(context.Background(), &o); err != nil {
		t.Fatalf("no pude configurar la salida: %v", err)
	}
	return o
}

// Sin salida configurada no se enciende: nadie sabría a dónde va la señal, y
// el canal no puede encenderse «a ver qué pasa».
func TestNoSeEnciendeSinSalida(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	conCartel(t, a)

	comps, err := a.AlAire(ctx, "Rolando")
	var no *NoSaleAlAire
	if err == nil {
		t.Fatal("el canal salió al aire sin tener a dónde mandar la señal")
	}
	if !asNoSale(err, &no) {
		t.Fatalf("el error no dice qué falta: %v", err)
	}
	if c := comprobacion(t, comps, "salidas"); c.Resultado != CompFalta {
		t.Fatalf("la comprobación de las salidas salió %q: %s", c.Resultado, c.Texto)
	} else if c.Arreglo == "" {
		t.Fatal("la comprobación que falta no dice qué hacer")
	}
	if ch, _ := a.Store.Channel.Get(ctx, a.ChannelID); ch.Mode != ModoSombra {
		t.Fatalf("el canal quedó en %q y tenía que quedarse en sombra", ch.Mode)
	}
}

// Sin el programa que produce la señal no hay nada que mandar: tampoco se
// enciende, y se dice con la misma frase con la que se dice en Ajustes.
func TestNoSeEnciendeSinFFmpeg(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	conCartel(t, a)
	conSalidaAArchivo(t, a, filepath.Join(t.TempDir(), "aire.ts"))

	// Como si no estuviera instalado.
	a.FFmpeg, a.FFprobe, a.FFmpegErr = "", "", errNoHayFFmpeg{}

	comps, err := a.AlAire(ctx, "Rolando")
	if err == nil {
		t.Fatal("el canal salió al aire sin el programa que produce la señal")
	}
	c := comprobacion(t, comps, "ffmpeg")
	if c.Resultado != CompFalta {
		t.Fatalf("la comprobación de ffmpeg salió %q: %s", c.Resultado, c.Texto)
	}
	if !strings.Contains(c.Texto, "ffmpeg") {
		t.Fatalf("no dice qué falta: %q", c.Texto)
	}
	if ch, _ := a.Store.Channel.Get(ctx, a.ChannelID); ch.Mode != ModoSombra {
		t.Fatalf("el canal quedó en %q", ch.Mode)
	}
}

type errNoHayFFmpeg struct{}

func (errNoHayFFmpeg) Error() string {
	return "no encuentro ffmpeg: va junto al ejecutable o en el PATH"
}

// Con salida y con ffmpeg, pero sin nada en la parrilla y sin relleno ni
// cartel: tampoco se enciende, porque saldría negro. Esa es la línea entera
// de la promesa.
func TestNoSeEnciendeSinConQueLlenarElAire(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	conSalidaAArchivo(t, a, filepath.Join(t.TempDir(), "aire.ts"))
	a.FFmpeg, a.FFprobe, a.FFmpegErr = "/bin/ffmpeg", "/bin/ffprobe", nil

	comps, err := a.AlAire(ctx, "Rolando")
	if err == nil {
		t.Fatal("el canal salió al aire sin nada que poner")
	}
	c := comprobacion(t, comps, "cobertura")
	if c.Resultado != CompFalta {
		t.Fatalf("la comprobación de la cobertura salió %q: %s", c.Resultado, c.Texto)
	}
	if !strings.Contains(c.Texto, "negro") {
		t.Fatalf("no dice lo que pasaría: %q", c.Texto)
	}

	// Con un cartel a mano, la misma parrilla vacía ya solo es un aviso: se
	// puede encender y se dice qué va a salir.
	conCartel(t, a)
	comps = a.ComprobacionesParaElAire(ctx)
	if c := comprobacion(t, comps, "cobertura"); c.Resultado != CompBien {
		t.Fatalf("con cartel la cobertura salió %q: %s", c.Resultado, c.Texto)
	}
	if c := comprobacion(t, comps, "plan"); c.Resultado != CompAviso {
		t.Fatalf("la parrilla vacía tenía que ser un aviso y salió %q: %s", c.Resultado, c.Texto)
	}
	if PrimeraQueFalta(comps) != nil {
		t.Fatalf("no tendría que faltar nada: %v", comps)
	}
}

// El aviso del retorno de aire no impide encender, y dice qué no se va a
// poder comprobar: el software no regaña, pero tampoco miente.
func TestSinRetornoDeAireSeEnciendeYSeDice(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	conCartel(t, a)
	conSalidaAArchivo(t, a, filepath.Join(t.TempDir(), "aire.ts"))
	a.FFmpeg, a.FFprobe, a.FFmpegErr = "/bin/ffmpeg", "/bin/ffprobe", nil

	c := comprobacion(t, a.ComprobacionesParaElAire(ctx), "retorno")
	if c.Resultado != CompAviso {
		t.Fatalf("sin retorno de aire la comprobación salió %q: %s", c.Resultado, c.Texto)
	}
	if !strings.Contains(c.Texto, "no voy a poder comprobar") {
		t.Fatalf("no dice qué no se va a poder comprobar: %q", c.Texto)
	}
}

// Con la parrilla llena, la comprobación del plan dice con qué empieza.
func TestConLaParrillaLlenaElPlanSaleBien(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	conCartel(t, a)
	conSalidaAArchivo(t, a, filepath.Join(t.TempDir(), "aire.ts"))
	conBloque(t, a, "Get Smart", a.Now().Add(-time.Minute), time.Hour)

	c := comprobacion(t, a.ComprobacionesParaElAire(ctx), "plan")
	if c.Resultado != CompBien {
		t.Fatalf("con la parrilla llena el plan salió %q: %s", c.Resultado, c.Texto)
	}
}

// La prueba de verdad: con todo listo el canal sale al aire, el motor arranca
// y el transporte crece; al volver a sombra el motor se para —el archivo deja
// de crecer, que es decir que no quedó ningún ffmpeg suelto— y los dos
// incidentes quedan escritos con su frase.
func TestSaleAlAireYVuelveASombraConElMotorDeVerdad(t *testing.T) {
	ffmpeg, _ := herramientas(t)
	a := abre(t)
	ctx := context.Background()

	// Un formato chico: la prueba tiene que caber en segundos.
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer el canal: %v", err)
	}
	ch.FormatProfile = "480i59.94"
	if err := a.Store.Channel.Update(ctx, ch); err != nil {
		t.Fatalf("no pude guardar el canal: %v", err)
	}

	dir := t.TempDir()
	clip := clipConSonido(t, ffmpeg, filepath.Join(dir, "programa.mkv"), 3)
	if err := a.Store.Settings.Set(ctx, KeyDefaultFiller, clip); err != nil {
		t.Fatalf("no pude apuntar el cartel: %v", err)
	}
	asset := model.MediaAsset{
		Path: clip, NormalizedPath: clip, DurationMs: 3000,
		State: model.AssetReady, NormalizeState: model.NormalizeReady,
		CreatedAt: a.Now(), UpdatedAt: a.Now(),
	}
	if err := a.Store.Media.Insert(ctx, &asset); err != nil {
		t.Fatalf("no pude guardar el archivo: %v", err)
	}
	conBloqueDe(t, a, asset, model.DeckProgram, a.Now(), 2*time.Second, nil)

	salida := filepath.Join(dir, "salida.ts")
	conSalidaAArchivo(t, a, salida)

	var vistos eventos
	vistos.mirando(a, t)
	vivo, cancel := context.WithCancel(ctx)
	defer cancel()
	a.Start(vivo)

	// En sombra no sale nada.
	esperar(t, 5*time.Second, func() bool { return vistos.hay("motor", "sombra", "modo sombra") },
		"el motor no dijo que el canal estaba en sombra")

	// Y ahora la puerta.
	comps, err := a.AlAire(ctx, "Rolando")
	if err != nil {
		t.Fatalf("con todo listo no se pudo salir al aire: %v (%v)", err, comps)
	}
	if de, _ := a.Store.Channel.Get(ctx, a.ChannelID); de.Mode != ModoAire {
		t.Fatalf("el canal quedó en %q", de.Mode)
	}
	esperar(t, 30*time.Second, func() bool {
		st, err := os.Stat(salida)
		return err == nil && st.Size() > 100_000
	}, "la señal no llegó a la salida: el motor no arrancó al salir de sombra")

	// Volver a sombra para el motor. El archivo deja de crecer, que es la
	// forma de ver desde aquí que no quedó ningún proceso suelto.
	if err := a.ASombra(ctx, "Rolando"); err != nil {
		t.Fatalf("no se pudo volver a sombra: %v", err)
	}
	esperar(t, 20*time.Second, func() bool { return vistos.hay("motor", "apagado", "deja de salir") },
		"el motor no dijo que se paraba al volver a sombra")
	esperar(t, 30*time.Second, func() bool { return dejoDeCrecer(salida) },
		"la señal siguió saliendo después de volver a sombra")
	if de, _ := a.Store.Channel.Get(ctx, a.ChannelID); de.Mode != ModoSombra {
		t.Fatalf("el canal quedó en %q después de apagar", de.Mode)
	}

	// Los dos incidentes, con quién lo hizo y con su frase en cristiano.
	incs, err := a.Store.Incident.List(ctx, a.ChannelID, a.Now().Add(-time.Hour), a.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("no pude leer los incidentes: %v", err)
	}
	for _, tipo := range []string{model.IncAlAire.String(), model.IncASombra.String()} {
		var encontrado *model.Incident
		for i := range incs {
			if incs[i].Kind == tipo {
				encontrado = &incs[i]
			}
		}
		if encontrado == nil {
			t.Fatalf("no quedó el incidente %q en la bitácora", tipo)
		}
		if !strings.Contains(encontrado.Detail, "Rolando") {
			t.Fatalf("el incidente %q no dice quién lo hizo: %q", tipo, encontrado.Detail)
		}
		if texto := TextoDeIncidente(tipo); texto == tipo || texto == "" {
			t.Fatalf("el incidente %q no tiene frase en cristiano", tipo)
		}
	}

	// Apagar dos veces seguidas no es un cambio: se dice que ya estaba así, y
	// no queda un segundo incidente contando algo que no pasó.
	if err := a.ASombra(ctx, "Rolando"); !errors.Is(err, ErrYaEstaba) {
		t.Fatalf("volver a sombra estando ya en sombra devolvió %v", err)
	}
	cancel()
	if err := a.Close(); err != nil {
		t.Fatalf("el apagado devolvió error: %v", err)
	}
}

// dejoDeCrecer mira dos veces el tamaño del archivo, separadas por un segundo
// y medio: si no cambió, nadie está escribiendo.
func dejoDeCrecer(ruta string) bool {
	uno, err := os.Stat(ruta)
	if err != nil {
		return false
	}
	time.Sleep(1500 * time.Millisecond)
	dos, err := os.Stat(ruta)
	return err == nil && uno.Size() == dos.Size()
}

// asNoSale es errors.As sin importar el paquete en cada prueba.
func asNoSale(err error, into **NoSaleAlAire) bool {
	no, ok := err.(*NoSaleAlAire)
	if ok {
		*into = no
	}
	return ok
}
