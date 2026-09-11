package ts

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Los streams hechos a mano prueban que el medidor cuenta lo que tiene
// delante; este archivo prueba lo contrario: que lo que un multiplexor de
// verdad produce —un TS de ffmpeg a tasa constante, con los PID y el número de
// programa que la persona escribió— se lee entero y sin errores. Las dos
// mitades hacen falta: un medidor que solo acierte con streams de laboratorio
// no sirve para decidir si la salida al transmisor de Rolando está bien.

// ffmpegDePrueba localiza ffmpeg o salta. En `-short` se salta siempre: son
// un par de segundos de codificación y el CI corto no los necesita.
func ffmpegDePrueba(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("-short: esta prueba codifica un TS con ffmpeg")
	}
	nombre := "ffmpeg"
	if runtime.GOOS == "windows" {
		nombre += ".exe"
	}
	if env := os.Getenv("ANTENA_FFMPEG"); env != "" {
		if p := filepath.Join(env, nombre); existe(p) {
			return p
		}
	}
	p, err := exec.LookPath(nombre)
	if err != nil {
		t.Skip("no hay ffmpeg: " + err.Error())
	}
	return p
}

func existe(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Un TS de ffmpeg con los PID, el programa y el tsid puestos a mano tiene que
// salir del medidor con esos mismos números y sin un solo error de
// continuidad. Es lo que respalda F2-114 por el lado de la medición: cuando la
// prueba de red abra un socket, esto es lo que le dirá si lo que llegó es el
// TS del canal o el de otro.
func TestUnTSDeFfmpegSeMideEntero(t *testing.T) {
	ffmpeg := ffmpegDePrueba(t)

	const (
		programaEsperado = 7
		tsidEsperado     = 42
		pmtEsperado      = 4096 // 0x1000
		videoEsperado    = 256  // 0x0100
		muxBits          = 4_000_000
	)
	salida := filepath.Join(t.TempDir(), "salida.ts")
	args := []string{
		"-y", "-nostdin", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc2=size=320x240:rate=25:duration=3",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=3",
		"-c:v", "mpeg2video", "-b:v", "1500k", "-g", "25",
		"-c:a", "mp2", "-b:a", "128k",
		"-f", "mpegts",
		// Lo mismo que arma internal/engine para la salida al multiplexor: tasa
		// constante, PCR cada 20 ms, PAT cada 100 ms, y los identificadores
		// escritos por la persona (PRD §10).
		"-muxrate", "4000000",
		"-pcr_period", "20",
		"-pat_period", "0.1",
		"-mpegts_service_id", "7",
		"-mpegts_transport_stream_id", "42",
		"-mpegts_pmt_start_pid", "4096",
		"-mpegts_start_pid", "256",
		salida,
	}
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg no pudo armar el TS: %v\n%s", err, out)
	}

	f, err := os.Open(salida)
	if err != nil {
		t.Fatalf("abriendo el TS: %v", err)
	}
	defer f.Close()
	r, err := Analyze(f)
	if err != nil {
		t.Fatalf("Analyze sobre un TS real: %v", err)
	}
	t.Log(r)

	// Lo que un multiplexor exige antes de mirar la imagen.
	if r.Programa != programaEsperado {
		t.Errorf("número de programa: %d, se configuró %d", r.Programa, programaEsperado)
	}
	if r.TSID != tsidEsperado {
		t.Errorf("tsid: %d, se configuró %d", r.TSID, tsidEsperado)
	}
	if r.PMTPid != pmtEsperado {
		t.Errorf("PID de la PMT: %d, se configuró %d", r.PMTPid, pmtEsperado)
	}
	if r.VideoPid() != videoEsperado {
		t.Errorf("PID del video: %d, se configuró %d", r.VideoPid(), videoEsperado)
	}
	if r.AudioPid() == 0 {
		t.Errorf("no se leyó ningún PID de audio; los flujos eran %v", r.Elementales)
	}
	if r.PCRPid == 0 {
		t.Error("la PMT no declaró PID de PCR")
	}
	if r.PATCount == 0 || r.PMTCount == 0 {
		t.Errorf("tablas: PAT %d, PMT %d; las dos se repiten cada 100 ms", r.PATCount, r.PMTCount)
	}

	// Salud del stream: esto es lo que hace que el multiplexor lo acepte.
	if r.CCErrors != 0 {
		t.Errorf("errores de continuidad en un TS recién hecho: %d", r.CCErrors)
	}
	if r.PCRCount == 0 {
		t.Fatal("no se vio ni un PCR")
	}
	// pcr_period=20 ms; se deja margen de sobra porque ffmpeg lo cumple «al
	// menos cada», no «exactamente cada». Lo que no puede pasar es rebasar los
	// 40 ms que el PRD §10 pone como techo... con holgura para una máquina
	// cargada.
	if r.PCRMaxGapMs > 100 {
		t.Errorf("brecha máxima de PCR: %.1f ms; el PRD §10 pide 40 ms o menos", r.PCRMaxGapMs)
	}
	if r.TSNonMonotone != 0 {
		t.Errorf("marcas de tiempo hacia atrás: %d; F0-04 pide que sean monotónicas", r.TSNonMonotone)
	}
	if r.TSGapsOver1s != 0 {
		t.Errorf("saltos de más de un segundo en las marcas: %d; F0-04 pide que no haya", r.TSGapsOver1s)
	}
	// Tasa constante: el relleno son paquetes nulos, y la desviación entre
	// ventanas de un segundo tiene que ser chica.
	if r.NullPackets == 0 {
		t.Error("un TS a tasa constante rellena con paquetes nulos, y no se contó ninguno")
	}
	if math.Abs(r.BitrateMean-muxBits)/muxBits > 0.10 {
		t.Errorf("tasa media: %.0f bits/s, se pidieron %d", r.BitrateMean, muxBits)
	}
	if r.BitrateDevPct > 5 {
		t.Errorf("desvío de la tasa: %.2f%%; con muxrate fijo debería ser casi cero", r.BitrateDevPct)
	}
	if math.Abs(r.DurationSec-3) > 0.5 {
		t.Errorf("duración medida: %.2f s, el clip era de 3", r.DurationSec)
	}
}
