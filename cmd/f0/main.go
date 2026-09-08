// f0 es el experimento del §22.1 del PRD: ¿el servidor de cuadros en Go
// entre decodificadores por clip y un encoder persistente aguanta horas?
//
//	f0 media                     fabrica los archivos de prueba en f0/media
//	f0 run [-hours 8] [-udp ...] corre el motor y escribe f0/out
//	f0 analyze                   mide el resultado y escribe f0/out/REPORTE.md
//	f0 all                       las tres seguidas
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"antena787/internal/engine"
	"antena787/internal/f0"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	ffmpeg, err := engine.FFmpeg()
	die(err)
	ffprobe, err := engine.FFprobe()
	die(err)

	root := "f0"
	media := filepath.Join(root, "media")
	out := filepath.Join(root, "out")

	switch os.Args[1] {
	case "media":
		fmt.Println("Fabricando archivos de prueba…")
		die(f0.Make(ffmpeg, media, func(s string) { fmt.Println(s) }))
	case "run":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		hours := fs.Float64("hours", 8, "duración de la corrida en horas")
		udp := fs.String("udp", "", "destino UDP del TS MPEG-2, p. ej. udp://192.168.1.50:1234 (vacío = solo archivo)")
		one := fs.Bool("one", false, "una sola salida (para medir CPU con una y con dos)")
		fs.Parse(os.Args[2:])
		die(run(ffmpeg, ffprobe, media, out, time.Duration(*hours*float64(time.Hour)), *udp, *one))
	case "analyze":
		die(f0.Analyze(ffmpeg, ffprobe, media, out, func(s string) { fmt.Println(s) }))
	case "all":
		fs := flag.NewFlagSet("all", flag.ExitOnError)
		hours := fs.Float64("hours", 8, "")
		fs.Parse(os.Args[2:])
		die(f0.Make(ffmpeg, media, func(s string) { fmt.Println(s) }))
		die(run(ffmpeg, ffprobe, media, out, time.Duration(*hours*float64(time.Hour)), "", false))
		die(f0.Analyze(ffmpeg, ffprobe, media, out, func(s string) { fmt.Println(s) }))
	default:
		usage()
	}
}

func run(ffmpeg, ffprobe, media, out string, dur time.Duration, udp string, one bool) error {
	specs, err := f0.Load(media)
	if err != nil {
		return fmt.Errorf("primero: f0 media (%w)", err)
	}
	os.MkdirAll(out, 0o755)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fm := engine.CAtv
	outs := []engine.Output{{Name: "catv", Kind: "mpeg2-ts", File: filepath.Join(out, "catv.ts"), UDP: udp, VideoKbs: 8000, MuxKbs: 10000}}
	if !one {
		outs = append(outs, engine.Output{Name: "web", Kind: "h264-ts", File: filepath.Join(out, "web.ts"), VideoKbs: 3000, GainDB: -8})
	}
	enc, err := engine.StartEncoder(ctx, ffmpeg, fm, outs)
	if err != nil {
		return err
	}
	evf, err := os.Create(filepath.Join(out, "events.jsonl"))
	if err != nil {
		return err
	}
	defer evf.Close()

	var playlist []engine.Clip
	var filler engine.Clip
	for _, s := range specs {
		c := engine.Clip{Path: s.File, Name: s.Name}
		if s.ID == 0 {
			filler = c
		} else {
			playlist = append(playlist, c)
		}
	}
	srv := &engine.Server{Format: fm, Enc: enc, Playlist: playlist, Filler: filler, Events: evf, Ffmpeg: ffmpeg, Ffprobe: ffprobe}

	stats := f0.NewStats(filepath.Join(out, "stats.csv"), enc.PID())
	go stats.Run(ctx, 10*time.Second)

	fmt.Printf("Corriendo %s con %d salida(s) → %s\n", dur, len(outs), out)
	t0 := time.Now()
	runErr := srv.Run(ctx, dur)
	finErr := enc.Finish()
	stats.Close()
	fmt.Printf("Terminó en %s\n", time.Since(t0).Round(time.Second))
	if runErr != nil {
		if msg := strings.TrimSpace(enc.Stderr.String()); msg != "" {
			return fmt.Errorf("%w\nencoder dijo:\n%s", runErr, msg)
		}
		return runErr
	}
	return finErr
}

func usage() {
	fmt.Fprintln(os.Stderr, "uso: f0 media | run [-hours N] [-udp udp://host:puerto] [-one] | analyze | all [-hours N]")
	os.Exit(2)
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
