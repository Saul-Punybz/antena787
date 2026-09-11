package salida

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
)

// ParamsArchivo es lo que se guarda de una salida a disco: nada más que
// dónde. La retención de esas grabaciones —7 y 30 días— es T7 (F2-43).
type ParamsArchivo struct {
	Ruta string `json:"ruta"`
	// Las mismas tasas que la salida al multiplexor: una grabación que se va
	// a volver a emitir (el diferido de T7) tiene que tener la misma calidad.
	BitrateMuxKbs   int    `json:"bitrate_mux_kbs"`
	BitrateVideoKbs int    `json:"bitrate_video_kbs"`
	Audio           string `json:"audio"`
	// BitrateAudioKbs es el bitrate del sonido, de 64 a 384. Vacío = 192, que
	// es el valor de fábrica y el mismo que trae MistServer. CAtv emite a 128.
	BitrateAudioKbs int `json:"bitrate_audio_kbs"`
}

// archivo es el driver de la salida a disco: el mismo transport stream, en un
// archivo. Es el que usa el modo sombra y el que graba cuando nadie ha dicho
// todavía a qué dirección va la señal.
type archivo struct {
	salida model.Output
	reg    Registro
	p      ParamsArchivo
}

func nuevoArchivo(o model.Output, reg Registro) (Driver, error) {
	p := ParamsArchivo{}
	if s := strings.TrimSpace(o.Params); s != "" && s != "{}" {
		if err := json.Unmarshal([]byte(s), &p); err != nil {
			return nil, fmt.Errorf("no entiendo lo que está guardado de esta salida: %w", err)
		}
	}
	p.Ruta = strings.TrimSpace(p.Ruta)
	if p.Ruta == "" {
		return nil, fmt.Errorf("no me has dicho en qué archivo guardar la señal")
	}
	if p.BitrateMuxKbs == 0 {
		p.BitrateMuxKbs = MuxKbsPorDefecto
	}
	if p.BitrateVideoKbs == 0 {
		p.BitrateVideoKbs = VideoKbsPorDefecto
	}
	if p.Audio == "" {
		p.Audio = AudioPorDefecto
	}
	if p.Audio != "mp2" && p.Audio != "ac3" {
		return nil, fmt.Errorf("el audio de la grabación es mp2 o ac3, y pusiste %q", p.Audio)
	}
	if p.BitrateVideoKbs+MargenDelMuxKbs > p.BitrateMuxKbs {
		return nil, fmt.Errorf("el video pide %d kb/s y la señal entera solo lleva %d: deja al menos %d kb/s de margen",
			p.BitrateVideoKbs, p.BitrateMuxKbs, MargenDelMuxKbs)
	}
	return &archivo{salida: o, reg: reg, p: p}, nil
}

// Abrir crea la carpeta si hace falta —nadie tiene que acordarse de eso— y
// devuelve la salida a disco.
func (d *archivo) Abrir(engine.Format) (engine.Output, error) {
	if dir := filepath.Dir(d.p.Ruta); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return engine.Output{}, fmt.Errorf("no pude crear la carpeta de %s: %w", d.p.Ruta, err)
		}
	}
	nombre := strings.TrimSpace(d.salida.Name)
	if nombre == "" {
		nombre = "archivo"
	}
	return engine.Output{
		Name:     nombre,
		Kind:     "mpeg2-ts",
		File:     porVida(d.p.Ruta, time.Now()),
		VideoKbs: d.p.BitrateVideoKbs,
		MuxKbs:   d.p.BitrateMuxKbs,
		Audio:    d.p.Audio,
		AudioKbs: d.p.BitrateAudioKbs,
	}, nil
}

// porVida le pone la hora al nombre del archivo, una vez por vida del
// encoder. Sin esto, cada relanzado escribía sobre la misma ruta y ffmpeg
// —que arranca sin -y, a propósito— moría con «Not overwriting», así que el
// canal se quedaba sin salida justo cuando acababa de recuperarse. Lo
// encontró el watchdog de F2-11, pero pasa igual cuando el encoder se muere
// solo.
//
// La otra salida sería -y, y es la mala: sobrescribir borraría lo que ya se
// había grabado del aire. Lo que salió al aire no se pisa (F2-43).
func porVida(ruta string, t time.Time) string {
	// La primera vida escribe donde se pidió: nada que ya funcionaba cambia
	// de sitio. Solo cuando ya hay una grabación ahí se abre una nueva al
	// lado, que es exactamente el caso que mataba al encoder relanzado.
	if _, err := os.Stat(ruta); err != nil {
		return ruta
	}
	ext := filepath.Ext(ruta)
	return strings.TrimSuffix(ruta, ext) + "-" + t.Format("2006-01-02T15-04-05") + ext
}

func (d *archivo) Vigilar(ctx context.Context, salida model.Output, listo <-chan error) {
	vigilar(ctx, d.reg, salida, listo)
}

func (d *archivo) Descripcion() string {
	return "al archivo " + d.p.Ruta
}
