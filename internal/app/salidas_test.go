package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"antena787/internal/drivers/salida"
	"antena787/internal/engine"
	"antena787/internal/model"
	"antena787/internal/ts"
)

// Pruebas de las salidas (T2 de F2): que dos salidas reciban la misma señal a
// la vez, cada una con su volumen y su estado, y que lo que sale por la red sea
// lo que un multiplexor acepta —tasa constante, PCR a tiempo, PID y número de
// programa los que se escribieron—.

// F2-46, F2-50 y F2-114, de punta a punta y con ffmpeg de verdad: el canal al
// aire con dos salidas —una `udp-ts` a un puerto de esta misma máquina y otra a
// un archivo—, el transport stream leído del socket, y medido con el
// analizador de internal/ts.
func TestF2_46ElTSQueSalePorUDPCumpleLoQueElMultiplexorExige(t *testing.T) {
	ffmpeg, _ := herramientas(t)
	a := abre(t)
	ctx := context.Background()

	// Un formato chico y tasas chicas: la prueba tiene que caber en segundos.
	ch, err := a.Store.Channel.Get(ctx, a.ChannelID)
	if err != nil {
		t.Fatalf("no pude leer el canal: %v", err)
	}
	ch.Mode, ch.FormatProfile = ModoAire, "480i59.94"
	if err := a.Store.Channel.Update(ctx, ch); err != nil {
		t.Fatalf("no pude poner el canal al aire: %v", err)
	}

	dir := t.TempDir()
	clip := clipConSonido(t, ffmpeg, filepath.Join(dir, "relleno.mkv"), 3)
	if err := a.Store.Settings.Set(ctx, KeyDefaultFiller, clip); err != nil {
		t.Fatalf("no pude apuntar el cartel: %v", err)
	}

	// El receptor: un socket abierto en esta máquina hace de multiplexor.
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("no pude abrir el puerto de prueba: %v", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetReadBuffer(8 << 20)
	puerto := conn.LocalAddr().(*net.UDPAddr).Port

	// Lo que el multiplexor de CAtv espera encontrar, escrito una vez.
	params := map[string]any{
		"destino":           fmt.Sprintf("127.0.0.1:%d", puerto),
		"bitrate_mux_kbs":   4000,
		"bitrate_video_kbs": 2500,
		"pid_video":         512,
		"pid_audio":         513,
		"pid_pmt":           480,
		"program":           7,
		"tsid":              99,
		"pcr_ms":            30,
		"audio":             "mp2",
	}
	crudos, _ := json.Marshal(params)
	if err := a.Store.Output.Upsert(ctx, &model.Output{
		ChannelID: a.ChannelID, Name: "transmisor", Driver: salida.DriverUDPTS,
		Params: string(crudos), TargetLoudness: -24,
	}); err != nil {
		t.Fatalf("no pude configurar la salida al multiplexor: %v", err)
	}
	grabacion := filepath.Join(dir, "grabacion.ts")
	if err := a.Store.Output.Upsert(ctx, &model.Output{
		ChannelID: a.ChannelID, Name: "grabación", Driver: salida.DriverArchivo,
		Params:         fmt.Sprintf(`{"ruta": %q, "bitrate_mux_kbs": 4000, "bitrate_video_kbs": 2500}`, grabacion),
		TargetLoudness: -24,
	}); err != nil {
		t.Fatalf("no pude configurar la grabación: %v", err)
	}

	// El lector: todo lo que llegue al socket, tal cual, que es un TS.
	var mu sync.Mutex
	var recibido bytes.Buffer
	leyendo, pararLectura := context.WithCancel(ctx)
	defer pararLectura()
	go func() {
		buf := make([]byte, 65536)
		for leyendo.Err() == nil {
			_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			n, err := conn.Read(buf)
			if n > 0 {
				mu.Lock()
				recibido.Write(buf[:n])
				mu.Unlock()
			}
			if err != nil && leyendo.Err() == nil {
				continue // el plazo de lectura, nada más
			}
		}
	}()

	vivo, cancel := context.WithCancel(ctx)
	a.Start(vivo)

	// Cuatro segundos de transport stream a 4000 kb/s son dos megas: bastante
	// para medir la tasa en varias ventanas de un segundo.
	const bastante = 2 << 20
	esperar(t, 60*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return recibido.Len() >= bastante
	}, "no llegó suficiente transport stream por UDP: el canal no salió a la red")

	// Y la segunda salida recibió la misma señal, al mismo tiempo (F2-46).
	esperar(t, 30*time.Second, func() bool {
		st, err := os.Stat(grabacion)
		return err == nil && st.Size() > 100_000
	}, "la grabación no creció: la segunda salida no recibió la señal")

	cancel()
	pararLectura()
	_ = conn.Close()

	mu.Lock()
	datos := recibido.Bytes()
	mu.Unlock()

	rep, err := ts.Analyze(bytes.NewReader(datos))
	if err != nil {
		t.Fatalf("el transport stream que salió por UDP no se pudo leer: %v", err)
	}
	t.Log("lo que recibió el multiplexor: " + rep.String())

	// Tasa constante rellenada con paquetes nulos, al bitrate que se dijo
	// (PRD §10). ±1 % es lo que la F0 ya medía.
	const pedido = 4_000_000.0
	if rep.BitrateDevPct > 1 {
		t.Errorf("la tasa se movió un %.2f %% y la tasa constante admite un 1 %%", rep.BitrateDevPct)
	}
	if desvio := 100 * math.Abs(rep.BitrateMean-pedido) / pedido; desvio > 1 {
		t.Errorf("la tasa media es %.3f Mb/s y se pidieron 4.000 Mb/s (%.2f %% de diferencia)",
			rep.BitrateMean/1e6, desvio)
	}
	if rep.NullPackets == 0 {
		t.Error("no hay un solo paquete nulo: sin relleno la tasa no es constante")
	}
	// PCR a tiempo y continuidad sin saltos.
	if rep.PCRMaxGapMs > engine.PCRmsMaximo {
		t.Errorf("el PCR tardó hasta %.1f ms y el máximo son %d", rep.PCRMaxGapMs, engine.PCRmsMaximo)
	}
	if rep.CCErrors != 0 {
		t.Errorf("%d saltos de continuidad en el TS", rep.CCErrors)
	}
	// Los PID y el número de programa, los que se escribieron una vez.
	if rep.Programa != 7 || rep.TSID != 99 || rep.PMTPid != 480 {
		t.Errorf("el TS trae programa %d, tsid %d y PMT en %d; se pidieron 7, 99 y 480",
			rep.Programa, rep.TSID, rep.PMTPid)
	}
	if rep.VideoPid() != 512 || rep.AudioPid() != 513 {
		t.Errorf("el video va en el PID %d y el audio en el %d; se pidieron 512 y 513",
			rep.VideoPid(), rep.AudioPid())
	}
	if rep.PCRPid != 512 {
		t.Errorf("el PCR va en el PID %d y tenía que ir con el video, en el 512", rep.PCRPid)
	}
	if rep.PATCount == 0 || rep.PATMaxGapMs > 500 {
		t.Errorf("la PAT salió %d veces y tardó hasta %.0f ms en repetirse", rep.PATCount, rep.PATMaxGapMs)
	}

	// La grabación es el mismo transport stream, y también se lee sola.
	f, err := os.Open(grabacion)
	if err != nil {
		t.Fatalf("no pude abrir la grabación: %v", err)
	}
	defer func() { _ = f.Close() }()
	grab, err := ts.Analyze(f)
	if err != nil {
		t.Fatalf("la grabación no es un transport stream legible: %v", err)
	}
	if grab.Packets == 0 || grab.CCErrors != 0 {
		t.Errorf("la grabación trae %d paquetes y %d saltos de continuidad", grab.Packets, grab.CCErrors)
	}

	// Y las dos salidas quedaron apuntadas como conectadas, cada una con su
	// estado (F2-46: «cada una con su propio estado de conexión»).
	for _, s := range a.SalidasDelCanal(ctx) {
		if s.ConnectionState != salida.Conectada && s.ConnectionState != salida.Apagada {
			t.Errorf("la salida «%s» quedó en %q", s.Name, s.ConnectionState)
		}
		if s.Texto == "" {
			t.Errorf("la salida «%s» no dice a dónde va", s.Name)
		}
	}
}

// F2-47 y F2-49 — cada salida va a su propio objetivo de volumen, y una salida
// que no se puede abrir no calla a las demás: se dice, queda apuntada en su
// fila, y el canal sale por las que sí.
func TestF2_47y49CadaSalidaConSuVolumenYUnaRotaNoCallaALasDemas(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	var vistos eventos
	vistos.mirando(a, t)

	dir := t.TempDir()
	// La buena, al transmisor, a −24 LKFS: la biblioteca ya está ahí, así que
	// sale sin tocarle el volumen.
	if err := a.Store.Output.Upsert(ctx, &model.Output{
		ChannelID: a.ChannelID, Name: "transmisor", Driver: salida.DriverUDPTS,
		Params: `{"destino": "127.0.0.1:1234"}`, TargetLoudness: -24,
	}); err != nil {
		t.Fatalf("no pude configurar la salida: %v", err)
	}
	// La de internet, a −16 LUFS: ocho decibelios más arriba.
	if err := a.Store.Output.Upsert(ctx, &model.Output{
		ChannelID: a.ChannelID, Name: "copia para la web", Driver: salida.DriverArchivo,
		Params: fmt.Sprintf(`{"ruta": %q}`, filepath.Join(dir, "web.ts")), TargetLoudness: -16,
	}); err != nil {
		t.Fatalf("no pude configurar la salida: %v", err)
	}
	// Y una con la dirección a medias.
	rota := model.Output{
		ChannelID: a.ChannelID, Name: "el otro transmisor", Driver: salida.DriverUDPTS,
		Params: `{"destino": "192.168.1.50"}`, TargetLoudness: -24,
	}
	if err := a.Store.Output.Upsert(ctx, &rota); err != nil {
		t.Fatalf("no pude configurar la salida rota: %v", err)
	}

	tanda, err := a.abrirSalidas(ctx, engine.CAtv)
	if err != nil {
		t.Fatalf("no se pudieron abrir las salidas: %v", err)
	}
	defer tanda.cerrar()
	if len(tanda.outs) != 2 {
		t.Fatalf("se abrieron %d salidas y tenían que abrirse las dos buenas", len(tanda.outs))
	}
	for _, o := range tanda.outs {
		switch o.Name {
		case "transmisor":
			if o.GainDB != 0 {
				t.Errorf("al transmisor le tocan 0 dB y le tocaron %.1f", o.GainDB)
			}
		case "copia para la web":
			if o.GainDB != 8 {
				t.Errorf("a la copia para la web le tocan +8 dB y le tocaron %.1f", o.GainDB)
			}
		default:
			t.Errorf("salió una salida que no era: %q", o.Name)
		}
	}

	// La rota quedó dicha y apuntada, con el motivo que ve la persona.
	esperar(t, 2*time.Second, func() bool { return vistos.hay("motor", "salida", "se queda fuera") },
		"la salida que no abre no quedó dicha")
	de, err := a.Store.Output.Get(ctx, rota.ID)
	if err != nil {
		t.Fatalf("no pude releer la salida rota: %v", err)
	}
	if de.ConnectionState != salida.Apagada || de.LastError == "" {
		t.Errorf("la salida rota quedó en %q con el error %q", de.ConnectionState, de.LastError)
	}
}

// Sin ninguna salida configurada el canal no se queda callado sin decir por
// qué: graba en la carpeta de datos y lo dice.
func TestSinSalidasConfiguradasSeGrabaYSeDice(t *testing.T) {
	a := abre(t)
	ctx := context.Background()
	var vistos eventos
	vistos.mirando(a, t)

	tanda, err := a.abrirSalidas(ctx, engine.CAtv)
	if err != nil {
		t.Fatalf("no se pudieron abrir las salidas: %v", err)
	}
	defer tanda.cerrar()
	if len(tanda.outs) != 1 || tanda.outs[0].File == "" {
		t.Fatalf("sin salidas configuradas tenía que quedar una grabación y quedó %+v", tanda.outs)
	}
	esperar(t, 2*time.Second, func() bool {
		return vistos.hay("motor", "salida", "todavía no me has dicho a qué dirección")
	}, "el canal no dijo que nadie le ha dado una dirección")
}
