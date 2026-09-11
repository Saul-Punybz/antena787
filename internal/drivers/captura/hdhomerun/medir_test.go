package hdhomerun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMedirVePIDsProgramaVideoAudioYPCR(t *testing.T) {
	cuerpo := construirTSSintetico(200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(cuerpo)
	}))
	defer srv.Close()

	r := NuevoRetorno(&Cliente{})
	m, err := r.Medir(context.Background(), srv.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("Medir: %v", err)
	}

	if m.Packets == 0 {
		t.Fatal("no se contó ni un paquete")
	}
	if len(m.PIDs) < 3 { // PAT + PMT + video + audio + nulos, al menos 3 distintos
		t.Fatalf("se esperaban varios PIDs, llegaron %d: %v", len(m.PIDs), m.PIDs)
	}
	if _, ok := m.PIDs[pidVideoSintetico]; !ok {
		t.Fatal("no se vio el PID de video")
	}
	if _, ok := m.PIDs[pidAudioSintetico]; !ok {
		t.Fatal("no se vio el PID de audio")
	}
	if m.NullPackets == 0 {
		t.Fatal("no se contaron paquetes nulos, y el TS sintético trae varios")
	}
	if m.CCErrors != 0 {
		t.Fatalf("no debería haber errores de continuidad en el TS sintético, hubo %d", m.CCErrors)
	}
	if m.PCRCount == 0 {
		t.Fatal("no se vio ningún PCR")
	}
	if m.NumeroPrograma != programaSintetico {
		t.Fatalf("número de programa mal leído: %d", m.NumeroPrograma)
	}
	if !m.HayVideo {
		t.Fatal("HayVideo debería ser true: el PMT declara stream_type 0x02")
	}
	if !m.HayAudio {
		t.Fatal("HayAudio debería ser true: el PMT declara stream_type 0x81")
	}
}

func TestMedirErrorSiNoLlegaNiUnPaquete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{0x01, 0x02, 0x03}) // menos de 188 bytes
	}))
	defer srv.Close()

	r := NuevoRetorno(&Cliente{})
	if _, err := r.Medir(context.Background(), srv.URL, time.Second); err == nil {
		t.Fatal("se esperaba error con menos de un paquete de TS")
	}
}

func TestMedirLeeSeñalDeStatusJSONConHostAPI(t *testing.T) {
	cuerpo := construirTSSintetico(20)
	tsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(cuerpo)
	}))
	defer tsServer.Close()

	fuerza, ruido, simbolo := 91, 88, 100
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status.json" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode([]estadoTunerJSON{{
			Lock:                  "qam256",
			SignalStrengthPercent: &fuerza,
			SignalQualityPercent:  &ruido,
			SymbolQualityPercent:  &simbolo,
		}})
	}))
	defer apiServer.Close()

	r := NuevoRetorno(&Cliente{})
	r.HostAPI = apiServer.URL
	m, err := r.Medir(context.Background(), tsServer.URL, time.Second)
	if err != nil {
		t.Fatalf("Medir: %v", err)
	}
	if m.Señal == nil {
		t.Fatal("se esperaba Señal con HostAPI apuntando a un status.json válido")
	}
	if m.Señal.Bloqueo != "qam256" || m.Señal.FuerzaPct != 91 || m.Señal.RuidoPct != 88 || m.Señal.SimboloPct != 100 {
		t.Fatalf("señal mal leída: %+v", m.Señal)
	}
	if r.Estado().Señal == nil {
		t.Fatal("Estado().Señal debería quedar rellena después de Medir")
	}
}

func TestMedirSinHostAPINiStatusJSONNoFalla(t *testing.T) {
	// Sin HostAPI, y contra un servidor que no tiene status.json (como la
	// mayoría de los HDHomeRun hoy: el endpoint no está documentado por
	// escrito), Medir no debe fallar — Señal simplemente queda nil.
	cuerpo := construirTSSintetico(5)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status.json" {
			http.NotFound(w, r)
			return
		}
		w.Write(cuerpo)
	}))
	defer srv.Close()

	r := NuevoRetorno(&Cliente{})
	m, err := r.Medir(context.Background(), srv.URL, time.Second)
	if err != nil {
		t.Fatalf("Medir no debería fallar por no tener señal: %v", err)
	}
	if m.Señal != nil {
		t.Fatalf("se esperaba Señal nil, llegó %+v", m.Señal)
	}
}
