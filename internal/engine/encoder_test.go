package engine

import (
	"strings"
	"testing"
)

// Los argumentos del encoder, exactos. Esta prueba es fea a propósito: lo que
// un multiplexor acepta o rechaza es una lista de banderas, y si alguien la
// cambia sin querer, el TP1000 lo nota antes que nosotros (PRD §10, F2-46,
// F2-114).

// F2-46 y F2-114 — la salida al multiplexor sale con muxrate, PCR, PAT, PID,
// programa, tsid, tamaño de paquete y TTL, todos donde tienen que ir.
func TestLosArgumentosDelMultiplexorSonExactos(t *testing.T) {
	e := &Encoder{Format: CAtv, Outputs: []Output{{
		Name: "transmisor", Kind: "mpeg2-ts", UDP: "udp://239.10.10.10:5000",
		VideoKbs: 8000, MuxKbs: 10000,
		PIDVideo: 512, PIDAudio: 513, PIDPMT: 480, Programa: 7, TSID: 99,
		PCRms: 30, Audio: "mp2", TTL: 4, PktSize: 1316,
	}}}
	quiere := strings.Join([]string{
		"-map", "0:v", "-map", "1:a",
		"-filter:a:0", "volume=0.00dB",
		"-c:v", "mpeg2video", "-pix_fmt", "yuv420p", "-g", "30", "-bf", "2",
		"-b:v", "8000k", "-minrate", "8000k", "-maxrate", "8000k", "-bufsize", "4000k",
		"-c:a:0", "mp2", "-b:a:0", "192k",
		"-max_muxing_queue_size", "1024",
		"-streamid", "0:512", "-streamid", "1:513",
		"-f", "mpegts",
		"-muxrate", "10000000", "-pcr_period", "30", "-pat_period", "0.1",
		"-mpegts_pmt_start_pid", "480", "-mpegts_start_pid", "512",
		"-mpegts_service_id", "7", "-mpegts_transport_stream_id", "99",
		"udp://239.10.10.10:5000?pkt_size=1316&ttl=4",
	}, " ")
	if got := strings.Join(e.outputArgs(), " "); got != quiere {
		t.Fatalf("los argumentos son\n  %s\ny tenían que ser\n  %s", got, quiere)
	}
}

// La misma señal a la red y al disco sale del mismo encoder, con las mismas
// opciones de mux en las dos ramas: es el `duplicate` de VLC
// (docs/VLC-PARIDAD.md) sin un segundo ffmpeg.
func TestLaMismaSenalALaRedYAlDiscoVaEnUnSoloEncoder(t *testing.T) {
	e := &Encoder{Format: CAtv, Outputs: []Output{{
		Name: "transmisor", Kind: "mpeg2-ts", UDP: "udp://10.0.0.9:1234", File: "/tmp/aire.ts",
		VideoKbs: 8000, MuxKbs: 10000, Audio: "ac3", PIDVideo: 256, PIDAudio: 257,
		PIDPMT: 4096, Programa: 1, TSID: 1, PCRms: 20, PktSize: 1316,
	}}}
	args := e.outputArgs()
	if args[len(args)-2] != "tee" {
		t.Fatalf("con red y disco a la vez tiene que salir por tee, y salió %v", args)
	}
	spec := args[len(args)-1]
	quiere := "[f=mpegts:muxrate=10000000:pcr_period=20:pat_period=0.1:" +
		"mpegts_pmt_start_pid=4096:mpegts_start_pid=256:mpegts_service_id=1:" +
		"mpegts_transport_stream_id=1:onfail=ignore]udp://10.0.0.9:1234?pkt_size=1316|" +
		"[f=mpegts:muxrate=10000000:pcr_period=20:pat_period=0.1:" +
		"mpegts_pmt_start_pid=4096:mpegts_start_pid=256:mpegts_service_id=1:" +
		"mpegts_transport_stream_id=1]/tmp/aire.ts"
	if spec != quiere {
		t.Fatalf("la rama del tee es\n  %s\ny tenía que ser\n  %s", spec, quiere)
	}
}

// Sin nada escrito, la salida es la que midió la F0: dos pistas de audio —mp2
// y ac3, para decidir cuál se queda—, PCR cada 20 ms y 1316 bytes por
// datagrama. Los PID los pone ffmpeg, que es lo que pasaba antes de T2.
func TestSinNadaEscritoSaleLoQueMidioLaF0(t *testing.T) {
	e := &Encoder{Format: CAtv, Outputs: []Output{{
		Name: "catv", Kind: "mpeg2-ts", UDP: "udp://127.0.0.1:1234", VideoKbs: 8000, MuxKbs: 10000,
	}}}
	got := strings.Join(e.outputArgs(), " ")
	for _, trozo := range []string{
		"-c:a:0 mp2 -b:a:0 192k -c:a:1 ac3 -b:a:1 192k",
		"-pcr_period 20",
		"udp://127.0.0.1:1234?pkt_size=1316",
	} {
		if !strings.Contains(got, trozo) {
			t.Fatalf("falta %q en\n  %s", trozo, got)
		}
	}
	for _, trozo := range []string{"-streamid", "mpegts_service_id", "&ttl="} {
		if strings.Contains(got, trozo) {
			t.Fatalf("sin nada escrito no tenía que aparecer %q en\n  %s", trozo, got)
		}
	}
}

// Un PCR que un multiplexor descartaría no puede colarse por aquí: el driver
// lo valida antes (F2-46), y el encoder nunca pasa de PCRmsMaximo por su
// cuenta.
func TestElPCRDeFabricaCabeEnLoQueElPRDPide(t *testing.T) {
	if PCRmsPorDefecto > PCRmsMaximo {
		t.Fatalf("el PCR de fábrica son %d ms y el PRD pide %d como máximo", PCRmsPorDefecto, PCRmsMaximo)
	}
}
