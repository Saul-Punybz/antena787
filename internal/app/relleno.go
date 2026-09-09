// relleno.go genera el relleno por defecto que pide el PRD §13 y el
// criterio F2-106: **el cartel de la estación con una cama musical**, de un
// clic, cuando la biblioteca de relleno está vacía.
//
// Dos reglas que no se negocian aquí:
//
//   - **Nunca son barras y tono.** Las barras y el tono existen solo para la
//     prueba del paso 5 del asistente; el respaldo del aire termina siempre
//     en el cartel (F2-69, auditoría C6). Lo que se genera aquí es respaldo
//     del aire, así que es un cartel.
//   - **El texto se dibuja en Go**, no con un filtro de ffmpeg: dibujar
//     texto con ffmpeg exige que esté compilado con la biblioteca de fuentes,
//     y eso no se puede dar por hecho en la máquina de una estación.
package app

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"antena787/internal/engine"
	"antena787/internal/model"
)

const (
	// CarpetaDeRelleno es la subcarpeta de la carpeta de contenido donde vive
	// el relleno. Se crea sola.
	CarpetaDeRelleno = "relleno"
	// ArchivoDelCartel es el nombre del relleno por defecto. Se llama así
	// para que se reconozca de un vistazo en la carpeta.
	ArchivoDelCartel = "cartel-de-la-estacion.mkv"
	// DuracionDelCartel es lo que dura: un minuto cuadra cualquier hueco
	// pequeño y no cansa cuando se repite.
	DuracionDelCartel = 60 * time.Second
)

// AvisoDelRelleno es lo que se le contesta a quien pulsa el botón.
const AvisoDelRelleno = "lo estoy preparando: en un minuto aparece en Biblioteca como relleno"

// ErrSinCarpetaDeContenido dice que todavía no se sabe dónde va el material.
var ErrSinCarpetaDeContenido = errors.New("todavía no me has dicho en qué carpeta está tu contenido")

// RellenoPorDefecto dice dónde va el cartel de la estación y si ya está
// hecho —o en camino—. Sin carpeta de contenido no hay dónde ponerlo.
func (a *App) RellenoPorDefecto(ctx context.Context) (ruta string, hecho bool, err error) {
	carpeta := a.setting(ctx, KeyContentFolder)
	if carpeta == "" {
		return "", false, ErrSinCarpetaDeContenido
	}
	ruta = filepath.Join(carpeta, CarpetaDeRelleno, ArchivoDelCartel)
	if apuntado := a.setting(ctx, KeyDefaultFiller); apuntado != "" {
		return apuntado, true, nil
	}
	if _, err := os.Stat(ruta); err == nil {
		return ruta, true, nil
	}
	return ruta, false, nil
}

// ReservarRellenoPorDefecto apunta que el cartel se está haciendo, para que
// dos clics seguidos no lo hagan dos veces. Devuelve la ruta.
func (a *App) ReservarRellenoPorDefecto(ctx context.Context) (string, error) {
	ruta, hecho, err := a.RellenoPorDefecto(ctx)
	if err != nil {
		return "", err
	}
	if hecho {
		return ruta, os.ErrExist
	}
	if err := a.Store.Settings.Set(ctx, KeyDefaultFiller, ruta); err != nil {
		return "", err
	}
	return ruta, nil
}

// CrearRellenoPorDefecto dibuja el cartel de la estación, le pone la cama
// musical, lo deja en la carpeta de relleno y lo mete en la biblioteca
// marcado como relleno. Tarda: quien lo llama lo hace en su propia goroutine.
//
// Si algo falla, el apunte se borra para que se pueda volver a intentar, y
// queda un incidente con el motivo en cristiano.
func (a *App) CrearRellenoPorDefecto(ctx context.Context) (string, error) {
	ruta, _, err := a.RellenoPorDefecto(ctx)
	if err != nil {
		return "", err
	}
	ruta, err = a.hacerElCartel(ctx, ruta)
	if err != nil {
		// Se suelta el apunte: el botón vuelve a estar disponible.
		_ = a.Store.Settings.Set(ctx, KeyDefaultFiller, "")
		a.Incident("relleno_por_defecto",
			"no pude preparar el cartel de la estación: "+err.Error())
		return "", err
	}
	if err := a.Store.Settings.Set(ctx, KeyDefaultFiller, ruta); err != nil {
		return ruta, err
	}
	a.Publish("relleno", "cartel",
		"el cartel de la estación ya está listo y entró a la biblioteca como relleno")
	return ruta, nil
}

// hacerElCartel es el trabajo de verdad, separado para que el manejo del
// apunte no se mezcle con ffmpeg.
func (a *App) hacerElCartel(ctx context.Context, ruta string) (string, error) {
	if a.FFmpeg == "" {
		return "", a.FFmpegErr
	}
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		return "", err
	}
	formato := FormatOf(ch.FormatProfile)

	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return "", fmt.Errorf("no se puede usar la carpeta %q: %w", filepath.Dir(ruta), err)
	}

	cuadro := filepath.Join(a.DataDir, "cartel-de-la-estacion.png")
	if err := DibujarCartel(ch, formato, cuadro); err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(cuadro) }()

	// Se escribe a un nombre temporal y se renombra al final: la carpeta está
	// vigilada, y nadie tiene que ver medio archivo.
	tmp := ruta + ".haciendose"
	if err := correrFFmpegDelCartel(ctx, a.FFmpeg, cuadro, tmp, formato); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, ruta); err != nil {
		return "", err
	}

	// La vigilancia de la carpeta también lo va a ver; el ingest no duplica
	// nada, así que se le pide aquí para no esperar al siguiente barrido.
	a.IngestFile(ctx, ruta)
	if err := a.marcarComoRelleno(ctx, ruta); err != nil {
		return ruta, err
	}
	return ruta, nil
}

// marcarComoRelleno pone la ficha recién ingerida en la biblioteca de
// relleno. El ingest no sabe distinguir un cartel de un capítulo —no hay
// forma de decírselo por la carpeta—, así que se marca después.
func (a *App) marcarComoRelleno(ctx context.Context, ruta string) error {
	asset, err := a.Store.Media.GetByPath(ctx, ruta)
	if err != nil {
		return err
	}
	puestos, err := a.Store.Filler.List(ctx, a.ChannelID)
	if err != nil {
		return err
	}
	for _, f := range puestos {
		if f.MediaAssetID == asset.ID {
			return nil
		}
	}
	canal := a.ChannelID
	return a.Store.Filler.Insert(ctx, &model.FillerAsset{
		MediaAssetID: asset.ID,
		ChannelID:    &canal,
		Kind:         "cartel",
		DurationMs:   asset.DurationMs,
	})
}

// ── el cuadro ─────────────────────────────────────────────────────────

// Los colores del cartel. Fondo oscuro a propósito: un cartel es lo último
// de la cascada y no tiene que deslumbrar a nadie de madrugada. Y oscuro es
// justo lo contrario de las barras de color, que es lo que F2-69 exige que
// nunca aparezca aquí.
var (
	fondoDelCartel = color.RGBA{R: 0x0d, G: 0x16, B: 0x27, A: 0xff}
	textoDelCartel = color.RGBA{R: 0xf2, G: 0xf5, B: 0xf9, A: 0xff}
	pieDelCartel   = color.RGBA{R: 0x9d, G: 0xb2, B: 0xcd, A: 0xff}
	rayaDelCartel  = color.RGBA{R: 0x2b, G: 0x4a, B: 0x74, A: 0xff}
)

// DibujarCartel pinta el cartel de la estación en un PNG del tamaño del
// formato de casa: el nombre del canal grande, y debajo el identificativo y
// la comunidad de licencia, que es lo que el paso 1 del asistente generó
// (F2-67).
func DibujarCartel(ch model.Channel, f engine.Format, dst string) error {
	ancho, alto := f.Width, f.Height
	if ancho <= 0 || alto <= 0 {
		ancho, alto = engine.CAtv.Width, engine.CAtv.Height
	}
	img := image.NewRGBA(image.Rect(0, 0, ancho, alto))
	draw.Draw(img, img.Bounds(), image.NewUniform(fondoDelCartel), image.Point{}, draw.Src)

	nombre := strings.TrimSpace(ch.Name)
	if nombre == "" {
		nombre = "Tu canal"
	}
	pie := strings.TrimSpace(strings.Trim(
		strings.TrimSpace(ch.CallSign)+" · "+strings.TrimSpace(ch.LicenseCity), " ·"))

	cabe := float64(ancho) * 0.86
	caraNombre, err := caraQueQuepa(gobold.TTF, nombre, float64(alto)/6.5, cabe)
	if err != nil {
		return err
	}
	defer func() { _ = caraNombre.Close() }()

	yNombre := alto / 2
	if pie != "" {
		yNombre = alto/2 - alto/16
	}
	escribirCentrado(img, caraNombre, textoDelCartel, nombre, yNombre)

	if pie != "" {
		// Una raya fina entre el nombre y el pie: además de separar, deja
		// pixeles encendidos de sobra para que nadie confunda esto con negro.
		raya := image.Rect(ancho/2-ancho/8, yNombre+alto/16, ancho/2+ancho/8, yNombre+alto/16+maxInt(2, alto/240))
		draw.Draw(img, raya, image.NewUniform(rayaDelCartel), image.Point{}, draw.Src)

		caraPie, err := caraQueQuepa(goregular.TTF, pie, float64(alto)/20, cabe)
		if err != nil {
			return err
		}
		defer func() { _ = caraPie.Close() }()
		escribirCentrado(img, caraPie, pieDelCartel, pie, yNombre+alto/6)
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if err := png.Encode(out, img); err != nil {
		return err
	}
	return out.Close()
}

// caraQueQuepa devuelve la tipografía al tamaño más grande que quepa en el
// ancho dado, sin bajar de un mínimo legible.
func caraQueQuepa(ttf []byte, texto string, tamaño, cabe float64) (font.Face, error) {
	fuente, err := opentype.Parse(ttf)
	if err != nil {
		return nil, err
	}
	for {
		cara, err := opentype.NewFace(fuente, &opentype.FaceOptions{
			Size: tamaño, DPI: 72, Hinting: font.HintingFull,
		})
		if err != nil {
			return nil, err
		}
		ancho := font.MeasureString(cara, texto)
		if float64(ancho>>6) <= cabe || tamaño <= 12 {
			return cara, nil
		}
		_ = cara.Close()
		tamaño *= 0.9
	}
}

// escribirCentrado pone el texto centrado a lo ancho, con y como línea base.
func escribirCentrado(dst draw.Image, cara font.Face, col color.Color, texto string, y int) {
	ancho := font.MeasureString(cara, texto)
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(col),
		Face: cara,
		Dot: fixed.Point26_6{
			X: fixed.I(dst.Bounds().Dx())/2 - ancho/2,
			Y: fixed.I(y),
		},
	}
	d.DrawString(texto)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── la cama musical ───────────────────────────────────────────────────

// correrFFmpegDelCartel convierte el cuadro en un minuto de video con una
// cama musical debajo: tres tonos graves a volumen muy bajo, con una
// ondulación lenta para que no suene a pitido de prueba. Nada de tono de
// alineación: eso es de la prueba del paso 5 y no sale al aire (F2-69).
func correrFFmpegDelCartel(ctx context.Context, ffmpeg, cuadro, dst string, f engine.Format) error {
	segundos := fmt.Sprintf("%d", int(DuracionDelCartel/time.Second))
	// Un acorde menor grave: sol, si bemol, re. Suena a espera, no a alarma.
	tonos := []string{"196", "233", "294"}

	args := []string{
		"-y", "-nostdin", "-hide_banner", "-loglevel", "error",
		"-loop", "1", "-framerate", f.FPSString(), "-i", cuadro,
	}
	var mezcla strings.Builder
	for i, hz := range tonos {
		args = append(args, "-f", "lavfi", "-i",
			fmt.Sprintf("sine=frequency=%s:sample_rate=%d:duration=%s", hz, f.SampleRate, segundos))
		fmt.Fprintf(&mezcla, "[%d:a]", i+1)
	}
	fin := int(DuracionDelCartel/time.Second) - 3
	mezcla.WriteString(fmt.Sprintf(
		"amix=inputs=%d:duration=longest,tremolo=f=0.25:d=0.7,volume=0.07,"+
			"afade=t=in:st=0:d=3,afade=t=out:st=%d:d=3,"+
			"aformat=sample_fmts=fltp:channel_layouts=stereo[cama]", len(tonos), fin))

	args = append(args,
		"-filter_complex", mezcla.String(),
		"-map", "0:v", "-map", "[cama]",
		"-t", segundos,
		"-s", f.SizeString(), "-r", f.FPSString(),
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "128k", "-ar", fmt.Sprintf("%d", f.SampleRate), "-ac", "2",
		// El nombre de trabajo no lleva extensión conocida —para que la
		// carpeta vigilada no lo levante a medio hacer—, así que hay que
		// decirle a ffmpeg en qué contenedor va.
		"-f", "matroska",
		dst,
	)

	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	cmd.Stdin = nil
	salida, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("no se pudo armar el cartel: %s", ultimasLineas(string(salida), 4))
	}
	return nil
}

// ultimasLineas se queda con el final de lo que escupió ffmpeg, que es donde
// está el motivo.
func ultimasLineas(texto string, n int) string {
	lineas := strings.Split(strings.TrimSpace(texto), "\n")
	if len(lineas) > n {
		lineas = lineas[len(lineas)-n:]
	}
	return strings.TrimSpace(strings.Join(lineas, "; "))
}
