package app

import (
	"context"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"antena787/internal/engine"
	"antena787/internal/ingest"
	"antena787/internal/model"
)

// ── lo que la máquina averigua sola ───────────────────────────────────

func TestDiscoYRedSeDicenEnUnaFrase(t *testing.T) {
	a, _ := abrirParaElAsistente(t)

	disco := a.DiscoEnCristiano()
	if disco == "" {
		t.Fatal("el disco tiene que decir algo, aunque sea que no se pudo medir")
	}
	if disco != "no pude medir el disco" {
		for _, quiero := range []string{"libres de", "disco de datos"} {
			if !strings.Contains(disco, quiero) {
				t.Fatalf("la frase del disco no se entiende: %q", disco)
			}
		}
	}

	red := RedEnCristiano()
	if !strings.Contains(red, "conectado") && !strings.Contains(red, "sin red") {
		t.Fatalf("la frase de la red no se entiende: %q", red)
	}
}

func TestTamañoEnCristiano(t *testing.T) {
	casos := []struct {
		bytes  uint64
		quiero string
	}{
		{890_000_000_000, "890 GB"},
		{3_500_000_000_000, "3.5 TB"},
		{1_200_000_000, "1.2 GB"},
		{512, "512 bytes"},
	}
	for _, c := range casos {
		if got := tamañoEnCristiano(c.bytes); got != c.quiero {
			t.Errorf("%d bytes se dicen %q, se esperaba %q", c.bytes, got, c.quiero)
		}
	}
}

func TestLasOpcionesNoPidenElegirPorNombreTecnico(t *testing.T) {
	op := OpcionesDelAsistente()
	for _, lista := range []string{"modo", "destino", "retorno", "calidad"} {
		if len(op[lista]) == 0 {
			t.Fatalf("la lista %q llegó vacía", lista)
		}
		for _, o := range op[lista] {
			if o.Valor == "" || o.Texto == "" {
				t.Fatalf("una opción de %q viene a medias: %+v", lista, o)
			}
		}
	}
	// «Todavía no» es siempre una respuesta válida, y se enseña (PRD §13).
	if !OpcionValida(OpcionesDeDestino, "ninguna") {
		t.Fatal("el destino tiene que aceptar «todavía no»")
	}
	if !OpcionValida(OpcionesDeRetorno, "ninguno") {
		t.Fatal("el retorno de aire tiene que aceptar «todavía no»")
	}
	// La calidad tiene que ser exactamente la lista de formatos de casa, y
	// FormatOf tiene que entender cada valor.
	for _, o := range OpcionesDeCalidad {
		f := FormatOf(o.Valor)
		if f.Width == 0 || f.FPSNum == 0 {
			t.Fatalf("la calidad %q no se entiende: %+v", o.Valor, f)
		}
	}
	if len(OpcionesDeCalidad) != 9 {
		t.Fatalf("los formatos de casa del PRD §10 son nueve y hay %d", len(OpcionesDeCalidad))
	}
}

// ── la primera parrilla ───────────────────────────────────────────────

func TestProponerParrillaConDosSeriesYUnaPelicula(t *testing.T) {
	a, ctx := abrirParaElAsistente(t)

	serie(t, a, "Kojak", 2, 22*60*1000)
	serie(t, a, "Zoids", 2, 22*60*1000)
	pelicula(t, a, "La Batalla", 95*60*1000)
	// Un título sin material: se cuenta, no se programa.
	sinMaterial(t, a, "Lo que falta")

	p, err := a.ProponerParrilla(ctx)
	if err != nil {
		t.Fatalf("la propuesta falló: %v", err)
	}
	if p.YaHabiaReglas {
		t.Fatal("no había reglas: la propuesta tenía que armarlas")
	}
	if p.ReglasCreadas != 3 {
		t.Fatalf("se esperaban tres espacios propuestos y salieron %d", p.ReglasCreadas)
	}
	if p.SinMaterial != 1 {
		t.Fatalf("se esperaba un título sin material y hay %d", p.SinMaterial)
	}
	if p.Bloques == 0 {
		t.Fatal("la propuesta no dejó ni un bloque en el plan")
	}

	reglas, err := a.Store.Rule.ListAll(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer las reglas: %v", err)
	}
	if len(reglas) != 3 {
		t.Fatalf("quedaron %d reglas guardadas", len(reglas))
	}
	ch, _ := a.Store.Channel.Get(ctx, a.ChannelID)
	hoy := ch.BroadcastDay(a.Now())
	for _, r := range reglas {
		if r.Days != "LMMJVSD" {
			t.Errorf("la regla %d no va todos los días: %q", r.ID, r.Days)
		}
		if r.EpisodesPerRun != 1 {
			t.Errorf("la regla %d saca %d episodios por corrida", r.ID, r.EpisodesPerRun)
		}
		if r.From != hoy {
			t.Errorf("la regla %d empieza el %s y hoy es %s", r.ID, r.From, hoy)
		}
		if r.To != hoy.Add(DiasDePropuesta-1) {
			t.Errorf("la regla %d dura hasta %s, se esperaba %s", r.ID, r.To, hoy.Add(DiasDePropuesta-1))
		}
		if r.SlotMs%int64(BloqueDePropuesta/time.Millisecond) != 0 {
			t.Errorf("la regla %d no cuadra con la media hora: %d ms", r.ID, r.SlotMs)
		}
		if r.Kind != model.RuleNormal {
			t.Errorf("la regla %d es de un tipo que no existía antes: %q", r.ID, r.Kind)
		}
	}

	// La película cae en la franja de la noche; las series, antes.
	for _, r := range reglas {
		titulo, err := a.Store.Title.Get(ctx, *r.TitleID)
		if err != nil {
			t.Fatalf("no pude leer el título de la regla %d: %v", r.ID, err)
		}
		if titulo.Name == "La Batalla" {
			if r.At < HoraDeLaNoche {
				t.Errorf("la película va a las %s y tenía que ir de noche", r.At)
			}
			if r.SlotMs != 120*60*1000 {
				t.Errorf("la película de 95 minutos ocupa %d ms, se esperaban dos horas", r.SlotMs)
			}
			continue
		}
		if r.At < ch.BroadcastDayAt || r.At >= HoraDeLaNoche {
			t.Errorf("la serie %q va a las %s, fuera del día de emisión", titulo.Name, r.At)
		}
		if r.SlotMs != 30*60*1000 {
			t.Errorf("la serie %q de 22 minutos ocupa %d ms, se esperaba media hora", titulo.Name, r.SlotMs)
		}
	}
}

func TestProponerParrillaNoPisaLoQueYaHay(t *testing.T) {
	a, ctx := abrirParaElAsistente(t)
	titulo := pelicula(t, a, "La Batalla", 95*60*1000)

	ch, _ := a.Store.Channel.Get(ctx, a.ChannelID)
	hoy := ch.BroadcastDay(a.Now())
	mia := model.ScheduleRule{
		ChannelID: a.ChannelID, Kind: model.RuleNormal, TitleID: &titulo.ID,
		Days: "LMMJV__", At: 20 * 60, SlotMs: 2 * 60 * 60 * 1000,
		From: hoy, To: hoy.Add(10), EpisodesPerRun: 1, Active: true,
	}
	if err := a.Store.Rule.Insert(ctx, &mia); err != nil {
		t.Fatalf("no pude guardar la regla de la persona: %v", err)
	}

	p, err := a.ProponerParrilla(ctx)
	if err != nil {
		t.Fatalf("la propuesta falló: %v", err)
	}
	if !p.YaHabiaReglas {
		t.Fatal("había una regla puesta y la propuesta no lo dijo")
	}
	if p.ReglasCreadas != 0 {
		t.Fatalf("la propuesta creó %d reglas encima de las que ya había", p.ReglasCreadas)
	}
	reglas, _ := a.Store.Rule.ListAll(ctx, a.ChannelID)
	if len(reglas) != 1 || reglas[0].At != 20*60 {
		t.Fatalf("la regla de la persona no quedó como estaba: %+v", reglas)
	}
}

// ── el relleno por defecto (F2-106) ───────────────────────────────────

func TestRellenoPorDefectoEsUnCartelYNoUnasBarras(t *testing.T) {
	a, ctx := abrirParaElAsistente(t)
	if a.FFmpeg == "" || a.FFprobe == "" {
		t.Skip("hacen falta las herramientas de video para esta prueba")
	}

	ch, _ := a.Store.Channel.Get(ctx, a.ChannelID)
	ch.Name = "Caribbean Advantage TV"
	ch.CallSign = "CAtv"
	ch.LicenseCity = "Aguadilla, PR"
	// La definición más chica: la prueba tiene que ser rápida.
	ch.FormatProfile = "480i59.94"
	if err := a.Store.Channel.Update(ctx, ch); err != nil {
		t.Fatalf("no pude guardar el canal: %v", err)
	}

	contenido := t.TempDir()
	if err := a.Store.Settings.Set(ctx, KeyContentFolder, contenido); err != nil {
		t.Fatalf("no pude poner la carpeta de contenido: %v", err)
	}

	ruta, hecho, err := a.RellenoPorDefecto(ctx)
	if err != nil || hecho {
		t.Fatalf("todavía no había cartel y dice que sí (%v, %v)", hecho, err)
	}
	if quiero := filepath.Join(contenido, CarpetaDeRelleno, ArchivoDelCartel); ruta != quiero {
		t.Fatalf("el cartel iba a %q y va a %q", quiero, ruta)
	}

	if _, err := a.CrearRellenoPorDefecto(ctx); err != nil {
		t.Fatalf("no se pudo hacer el cartel: %v", err)
	}
	if _, err := os.Stat(ruta); err != nil {
		t.Fatalf("el cartel no quedó en su sitio: %v", err)
	}

	m, err := ingest.Probe(ctx, a.FFprobe, ruta)
	if err != nil {
		t.Fatalf("no pude medir el cartel: %v", err)
	}
	if !m.HasVideo || !m.HasAudio {
		t.Fatalf("el cartel tiene que llevar imagen y sonido: imagen=%v sonido=%v", m.HasVideo, m.HasAudio)
	}
	if d := m.DurationMs; d < 58_000 || d > 62_000 {
		t.Fatalf("el cartel dura %d ms y se esperaba un minuto", d)
	}

	// F2-69: nunca son barras. La comprobación barata es el brillo medio de
	// un cuadro: unas barras de color son claras, un cartel es oscuro.
	if luma := lumaMedia(t, a.FFmpeg, ruta); luma > 60 {
		t.Fatalf("el cuadro del cartel tiene un brillo medio de %.1f: eso parecen barras, y las barras nunca son respaldo del aire (F2-69)", luma)
	}

	// Y quedó en la biblioteca marcado como relleno.
	rellenos, err := a.Store.Filler.List(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer el relleno: %v", err)
	}
	if len(rellenos) != 1 {
		t.Fatalf("el cartel no entró como relleno: %+v", rellenos)
	}
	if rellenos[0].DurationMs < 58_000 {
		t.Fatalf("el relleno dice durar %d ms", rellenos[0].DurationMs)
	}

	// Y a la segunda ya está hecho.
	if _, hecho, _ := a.RellenoPorDefecto(ctx); !hecho {
		t.Fatal("después de hacerlo, el cartel tiene que constar como hecho")
	}
	if _, err := a.ReservarRellenoPorDefecto(ctx); !os.IsExist(err) {
		t.Fatalf("reservarlo dos veces tenía que decir que ya está: %v", err)
	}
}

func TestDibujarCartelPintaElIdentificativo(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "cartel.png")
	ch := model.Channel{Name: "Caribbean Advantage TV", CallSign: "CAtv", LicenseCity: "Aguadilla, PR"}
	if err := DibujarCartel(ch, engine.CAtv, dst); err != nil {
		t.Fatalf("no se pudo dibujar el cartel: %v", err)
	}
	f, err := os.Open(dst)
	if err != nil {
		t.Fatalf("no se pudo abrir el cartel: %v", err)
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("el cartel no es una imagen: %v", err)
	}
	if img.Bounds().Dx() != engine.CAtv.Width || img.Bounds().Dy() != engine.CAtv.Height {
		t.Fatalf("el cartel mide %v y el formato de casa es %dx%d", img.Bounds(), engine.CAtv.Width, engine.CAtv.Height)
	}
	oscuro, claros := brilloDe(img)
	if oscuro > 60 {
		t.Fatalf("el cartel tiene un brillo medio de %.1f: tiene que ser oscuro, no unas barras", oscuro)
	}
	if claros == 0 {
		t.Fatal("el cartel salió en negro: no se dibujó ni el nombre del canal")
	}
}

// ── andamio ───────────────────────────────────────────────────────────

func abrirParaElAsistente(t *testing.T) (*App, context.Context) {
	t.Helper()
	a, err := Open(Options{DataDir: t.TempDir(), Version: "prueba", NoMaintenance: true})
	if err != nil {
		t.Fatalf("no pude abrir la aplicación: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a, context.Background()
}

// archivoListo deja un archivo medido y normalizado, que es lo único que el
// resolver acepta meter en el plan (F1-41).
func archivoListo(t *testing.T, a *App, nombre string, durMs int64) model.MediaAsset {
	t.Helper()
	ahora := a.Now()
	asset := model.MediaAsset{
		Path:           "/medios/" + nombre + ".mkv",
		DurationMs:     durMs,
		State:          model.AssetReady,
		NormalizeState: ingest.NormalizeReady,
		NormalizedPath: "/medios/" + nombre + ".norm.mkv",
		CreatedAt:      ahora,
		UpdatedAt:      ahora,
	}
	if err := a.Store.Media.Insert(context.Background(), &asset); err != nil {
		t.Fatalf("no pude guardar el archivo %q: %v", nombre, err)
	}
	return asset
}

func serie(t *testing.T, a *App, nombre string, episodios int, durMs int64) model.Title {
	t.Helper()
	ctx := context.Background()
	titulo := model.Title{Name: nombre, Kind: model.TitleSeries}
	if err := a.Store.Title.Insert(ctx, &titulo); err != nil {
		t.Fatalf("no pude guardar la serie %q: %v", nombre, err)
	}
	for i := 1; i <= episodios; i++ {
		asset := archivoListo(t, a, nombre+"-"+strconv.Itoa(i), durMs)
		e := model.Episode{TitleID: titulo.ID, Season: 1, Number: i, Name: nombre, MediaAssetID: &asset.ID}
		if err := a.Store.Episode.Insert(ctx, &e); err != nil {
			t.Fatalf("no pude guardar el episodio %d de %q: %v", i, nombre, err)
		}
	}
	return titulo
}

func pelicula(t *testing.T, a *App, nombre string, durMs int64) model.Title {
	t.Helper()
	asset := archivoListo(t, a, nombre, durMs)
	titulo := model.Title{Name: nombre, Kind: model.TitleMovie, MediaAssetID: &asset.ID}
	if err := a.Store.Title.Insert(context.Background(), &titulo); err != nil {
		t.Fatalf("no pude guardar la película %q: %v", nombre, err)
	}
	return titulo
}

func sinMaterial(t *testing.T, a *App, nombre string) model.Title {
	t.Helper()
	titulo := model.Title{Name: nombre, Kind: model.TitleProgram}
	if err := a.Store.Title.Insert(context.Background(), &titulo); err != nil {
		t.Fatalf("no pude guardar el título %q: %v", nombre, err)
	}
	return titulo
}

// lumaMedia saca un cuadro del video y mide su brillo medio. Es la
// comprobación barata de que lo que se generó no son barras de color.
func lumaMedia(t *testing.T, ffmpeg, video string) float64 {
	t.Helper()
	cuadro := filepath.Join(t.TempDir(), "cuadro.png")
	cmd := exec.Command(ffmpeg, "-y", "-nostdin", "-hide_banner", "-loglevel", "error",
		"-ss", "5", "-i", video, "-frames:v", "1", cuadro)
	if salida, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("no pude sacar un cuadro del cartel: %v: %s", err, salida)
	}
	f, err := os.Open(cuadro)
	if err != nil {
		t.Fatalf("no pude abrir el cuadro: %v", err)
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("el cuadro no es una imagen: %v", err)
	}
	media, _ := brilloDe(img)
	return media
}

// brilloDe devuelve el brillo medio de la imagen (0..255) y cuántos pixeles
// claros tiene, que es la señal de que se dibujó algo encima del fondo.
func brilloDe(img image.Image) (media float64, claros int) {
	b := img.Bounds()
	var suma float64
	var n int
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			r, g, bl, _ := img.At(x, y).RGBA()
			luma := 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(bl>>8)
			suma += luma
			n++
			if luma > 180 {
				claros++
			}
		}
	}
	if n == 0 {
		return 0, 0
	}
	return suma / float64(n), claros
}
