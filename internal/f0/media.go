// Package f0 es el experimento del §22.1: fabrica los archivos de prueba,
// corre el motor durante horas y mide lo que el PRD dice que hay que medir.
package f0

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ClipSpec describe un archivo de prueba. Cada uno lleva un marcador en la
// esquina superior izquierda —cinco bloques grises: el id del clip y el
// número de cuadro en nibbles— y un pitido de 20 ms en su primera muestra.
// Con eso el analizador cuenta cuadros exactos y mide el desfase A/V sin
// oído, aunque el video se escale y se recomprima.
type ClipSpec struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	File     string  `json:"file"`
	W        int     `json:"w"`
	H        int     `json:"h"`
	FPS      string  `json:"fps"`
	Seconds  float64 `json:"seconds"`
	VFR      bool    `json:"vfr"`
	Corrupt  bool    `json:"corrupt"`
	ToneHz   int     `json:"tone_hz"`
	Note     string  `json:"note"`
	vcodec   []string
	acodec   []string
	channels string
	audioCut float64 // segundos que el audio termina antes que el video
	inter    bool
}

// Specs son los nueve del PRD (el 6 son dos: mono y 5.1) más el relleno.
var Specs = []ClipSpec{
	{ID: 0, Name: "relleno", W: 1280, H: 720, FPS: "60000/1001", Seconds: 10, ToneHz: 220, Note: "cartel de relleno", vcodec: []string{"-c:v", "libx264", "-preset", "fast", "-crf", "18"}, acodec: []string{"-c:a", "aac", "-b:a", "128k"}, channels: "stereo"},
	{ID: 1, Name: "01-h264-1080p2997-aac", W: 1920, H: 1080, FPS: "30000/1001", Seconds: 40, ToneHz: 330, Note: "el caso normal", vcodec: []string{"-c:v", "libx264", "-preset", "fast", "-crf", "18"}, acodec: []string{"-c:a", "aac", "-b:a", "160k"}, channels: "stereo"},
	{ID: 2, Name: "02-h264-720p5994", W: 1280, H: 720, FPS: "60000/1001", Seconds: 35, ToneHz: 392, Note: "cambio de resolución y de cuadros", vcodec: []string{"-c:v", "libx264", "-preset", "fast", "-crf", "18"}, acodec: []string{"-c:a", "aac", "-b:a", "160k"}, channels: "stereo"},
	{ID: 3, Name: "03-mpeg2-480i2997-ac3", W: 640, H: 480, FPS: "30000/1001", Seconds: 30, ToneHz: 440, Note: "material de archivo viejo, entrelazado 4:3", vcodec: []string{"-c:v", "mpeg2video", "-q:v", "3", "-flags", "+ilme+ildct", "-alternate_scan", "1"}, acodec: []string{"-c:a", "ac3", "-b:a", "192k"}, channels: "stereo", inter: true},
	{ID: 4, Name: "04-hevc-1080p25", W: 1920, H: 1080, FPS: "25", Seconds: 30, ToneHz: 494, Note: "cuadros PAL en un canal NTSC", vcodec: []string{"-c:v", "libx265", "-preset", "fast", "-crf", "22", "-tag:v", "hvc1"}, acodec: []string{"-c:a", "aac", "-b:a", "160k"}, channels: "stereo"},
	{ID: 5, Name: "05-h264-vfr", W: 1280, H: 720, FPS: "30000/1001", Seconds: 30, VFR: true, ToneHz: 523, Note: "cuadros variables, como una grabación de OBS", vcodec: []string{"-c:v", "libx264", "-preset", "fast", "-crf", "18"}, acodec: []string{"-c:a", "aac", "-b:a", "160k"}, channels: "stereo"},
	{ID: 6, Name: "06a-mono", W: 1280, H: 720, FPS: "30000/1001", Seconds: 20, ToneHz: 587, Note: "audio mono", vcodec: []string{"-c:v", "libx264", "-preset", "fast", "-crf", "18"}, acodec: []string{"-c:a", "aac", "-b:a", "96k"}, channels: "mono"},
	{ID: 7, Name: "06b-surround51", W: 1280, H: 720, FPS: "30000/1001", Seconds: 20, ToneHz: 659, Note: "audio 5.1", vcodec: []string{"-c:v", "libx264", "-preset", "fast", "-crf", "18"}, acodec: []string{"-c:a", "ac3", "-b:a", "384k"}, channels: "5.1"},
	{ID: 8, Name: "07-cea608", W: 1920, H: 1080, FPS: "30000/1001", Seconds: 25, ToneHz: 698, Note: "subtítulos CEA-608: hace falta un archivo real; ver README de f0", vcodec: []string{"-c:v", "libx264", "-preset", "fast", "-crf", "18"}, acodec: []string{"-c:a", "aac", "-b:a", "160k"}, channels: "stereo"},
	{ID: 9, Name: "08-audio-corto-200ms", W: 1280, H: 720, FPS: "30000/1001", Seconds: 20, ToneHz: 784, Note: "audio 200 ms más corto que el video", vcodec: []string{"-c:v", "libx264", "-preset", "fast", "-crf", "18"}, acodec: []string{"-c:a", "aac", "-b:a", "160k"}, channels: "stereo", audioCut: 0.2},
	{ID: 10, Name: "09-corrupto-final", W: 1280, H: 720, FPS: "30000/1001", Seconds: 25, Corrupt: true, ToneHz: 880, Note: "corrupto en el último segundo", vcodec: []string{"-c:v", "libx264", "-preset", "fast", "-crf", "18"}, acodec: []string{"-c:a", "aac", "-b:a", "160k"}, channels: "stereo"},
}

// MarkerBlocks es cuántos bloques lleva el marcador: dos de calibración
// (nibble 0 y nibble 15), el id, y 4 nibbles del número de cuadro. Los de
// calibración hacen que la lectura aguante cualquier conversión de rango
// —limitado a completo, gamma— que el pipeline meta por el camino.
const MarkerBlocks = 7

// MarkerLevel es el gris de un nibble: 24 + n·14 cabe en rango limitado y
// aguanta ±6 de error de compresión.
func MarkerLevel(n int) int { return 24 + n*14 }

// MarkerNibble deshace MarkerLevel usando los dos bloques de calibración.
func MarkerNibble(lum, cal0, cal15 float64) int {
	if cal15-cal0 < 30 {
		return -1
	}
	n := int((lum-cal0)/(cal15-cal0)*15 + 0.5)
	if n < 0 {
		return 0
	}
	if n > 15 {
		return 15
	}
	return n
}

// BlockPx es el lado de cada bloque en un video de ancho w.
func BlockPx(w int) int { return w / 40 }

// markerFilter fabrica la tira del marcador con geq y la superpone.
func markerFilter(id, w int) string {
	b := BlockPx(w)
	lum := fmt.Sprintf("if(lt(X,%d),%d,if(lt(X,%d),%d,if(lt(X,%d),%d,if(lt(X,%d),%s,if(lt(X,%d),%s,if(lt(X,%d),%s,%s))))))",
		b, MarkerLevel(0),
		2*b, MarkerLevel(15),
		3*b, MarkerLevel(id),
		4*b, "24+14*mod(floor(N/4096),16)",
		5*b, "24+14*mod(floor(N/256),16)",
		6*b, "24+14*mod(floor(N/16),16)",
		"24+14*mod(N,16)")
	return fmt.Sprintf("nullsrc=s=%dx%d:r=%s,format=yuv420p,geq=lum='%s':cb=128:cr=128[mark]", MarkerBlocks*b, b, "%s", lum)
}

// audioExpr es tono fijo por clip a −20 dBFS, más un pitido de 20 ms a
// 2 kHz al arrancar. El pitido es la referencia de tiempo del audio.
func audioExpr(tone int) string {
	return fmt.Sprintf("0.1*sin(2*PI*%d*t)+if(lt(t,0.02),0.5*sin(2*PI*2000*t),0)", tone)
}

// Make fabrica todos los archivos en dir y escribe clips.json.
func Make(ffmpeg, dir string, short bool, log func(string)) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var out []ClipSpec
	for _, s := range Specs {
		if short {
			s.Seconds = math.Max(4, math.Round(s.Seconds/5))
		}
		s.File = filepath.Join(dir, s.Name+ext(s))
		if err := makeOne(ffmpeg, s); err != nil {
			return fmt.Errorf("%s: %w", s.Name, err)
		}
		if s.Corrupt {
			if err := corruptTail(s.File); err != nil {
				return err
			}
		}
		log(fmt.Sprintf("  %-28s %dx%d %s %.0fs — %s", s.Name, s.W, s.H, s.FPS, s.Seconds, s.Note))
		out = append(out, s)
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return os.WriteFile(filepath.Join(dir, "clips.json"), b, 0o644)
}

func ext(s ClipSpec) string {
	if strings.Contains(strings.Join(s.vcodec, " "), "mpeg2video") {
		return ".mpg"
	}
	return ".mp4"
}

func makeOne(ffmpeg string, s ClipSpec) error {
	fps := s.FPS
	if s.inter {
		fps = "60000/1001" // dos campos por cuadro: se genera al doble y se entrelaza
	}
	mark := fmt.Sprintf(markerFilter(s.ID, s.W), fps)
	base := fmt.Sprintf("testsrc2=s=%dx%d:r=%s,format=yuv420p[base]", s.W, s.H, fps)
	vchain := "[base][mark]overlay=0:0"
	if s.VFR {
		// Deja pasar cuadros a intervalos irregulares y guarda VFR de verdad.
		vchain += ",select='not(mod(n,3))+not(mod(n,7))'"
	}
	if s.inter {
		vchain += ",tinterlace=interleave_top,setfield=tff"
	}
	var aexpr string
	switch s.channels {
	case "mono":
		aexpr = audioExpr(s.ToneHz)
	case "5.1":
		e := audioExpr(s.ToneHz)
		aexpr = strings.Join([]string{e, e, "0.05*sin(2*PI*" + fmt.Sprint(s.ToneHz) + "*t)", "0.02*sin(2*PI*60*t)", "0.03*sin(2*PI*" + fmt.Sprint(s.ToneHz*2) + "*t)", "0.03*sin(2*PI*" + fmt.Sprint(s.ToneHz*2) + "*t)"}, "|")
	default:
		e := audioExpr(s.ToneHz)
		aexpr = e + "|" + e
	}
	adur := s.Seconds - s.audioCut
	achain := fmt.Sprintf("aevalsrc='%s':s=48000:c=%s,atrim=0:%.3f[aud]", aexpr, s.channels, adur)
	fc := strings.Join([]string{mark, base, vchain + "[vid]", achain}, ";")

	args := []string{"-y", "-hide_banner", "-loglevel", "error", "-filter_complex", fc,
		"-map", "[vid]", "-map", "[aud]", "-t", fmt.Sprintf("%.3f", s.Seconds)}
	args = append(args, s.vcodec...)
	if s.VFR {
		args = append(args, "-fps_mode", "vfr")
	}
	args = append(args, s.acodec...)
	if strings.HasSuffix(s.File, ".mp4") {
		args = append(args, "-movflags", "+faststart")
	}
	args = append(args, s.File)
	cmd := exec.Command(ffmpeg, args...)
	outb, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v\n%s", err, outb)
	}
	return nil
}

// corruptTail pisa el último 3 % del archivo con basura. Con moov al frente
// (faststart) el archivo abre bien y se rompe al final, que es la idea.
func corruptTail(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	st, _ := f.Stat()
	n := st.Size() * 3 / 100
	junk := make([]byte, n)
	rand.New(rand.NewSource(787)).Read(junk)
	_, err = f.WriteAt(junk, st.Size()-n)
	return err
}

// Load lee clips.json.
func Load(dir string) ([]ClipSpec, error) {
	b, err := os.ReadFile(filepath.Join(dir, "clips.json"))
	if err != nil {
		return nil, err
	}
	var out []ClipSpec
	return out, json.Unmarshal(b, &out)
}
