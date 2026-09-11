package hdhomerun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// servidorDispositivo simula un HDHomeRun de verdad: discover.json y
// lineup.json con los campos que documenta info.hdhomerun.com/info/http_api.
func servidorDispositivo(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/discover.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Dispositivo{
			DeviceID:        "10419BB4",
			FriendlyName:    "HDHomeRun Flex Duo",
			ModelNumber:     "HDFX-2US",
			FirmwareName:    "hdhomerun_atsc",
			FirmwareVersion: "20230629",
			TunerCount:      2,
		})
	})
	mux.HandleFunc("/lineup.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Canal{
			{GuideNumber: "6.1", GuideName: "CATV-HD", URL: "http://192.0.2.10:5004/auto/v6.1"},
			{GuideNumber: "6.2", GuideName: "CATV-SD", URL: "http://192.0.2.10:5004/auto/v6.2", Tags: "favorite"},
		})
	})
	return httptest.NewServer(mux)
}

func TestDescubrirEnHostLeeDiscoverJSON(t *testing.T) {
	srv := servidorDispositivo(t)
	defer srv.Close()

	c := &Cliente{}
	d, err := c.DescubrirEnHost(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("DescubrirEnHost: %v", err)
	}
	if d.DeviceID != "10419BB4" || d.TunerCount != 2 || d.FriendlyName != "HDHomeRun Flex Duo" {
		t.Fatalf("dispositivo mal leído: %+v", d)
	}
}

func TestDescubrirEnHostAceptaHostSinEsquema(t *testing.T) {
	srv := servidorDispositivo(t)
	defer srv.Close()

	c := &Cliente{}
	host := strings.TrimPrefix(srv.URL, "http://")
	d, err := c.DescubrirEnHost(context.Background(), host)
	if err != nil {
		t.Fatalf("DescubrirEnHost sin esquema: %v", err)
	}
	if d.DeviceID != "10419BB4" {
		t.Fatalf("dispositivo mal leído: %+v", d)
	}
}

func TestDescubrirEnHostErrorSiNoHayDispositivo(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	c := &Cliente{}
	if _, err := c.DescubrirEnHost(context.Background(), srv.URL); err == nil {
		t.Fatal("se esperaba error contra un servidor sin discover.json")
	}
}

func TestLineupLeeCanales(t *testing.T) {
	srv := servidorDispositivo(t)
	defer srv.Close()

	c := &Cliente{}
	canales, err := c.Lineup(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Lineup: %v", err)
	}
	if len(canales) != 2 {
		t.Fatalf("se esperaban 2 canales, llegaron %d", len(canales))
	}
	if canales[0].GuideNumber != "6.1" || canales[0].GuideName != "CATV-HD" {
		t.Fatalf("canal mal leído: %+v", canales[0])
	}
	if canales[0].URL != "http://192.0.2.10:5004/auto/v6.1" {
		t.Fatalf("URL del canal mal leída: %q", canales[0].URL)
	}
	if canales[1].Tags != "favorite" {
		t.Fatalf("Tags mal leído: %+v", canales[1])
	}
}

func TestDescubrirSinNubeNoFalla(t *testing.T) {
	// URLDescubrimientoPublico no está pensado para pruebas: en una máquina
	// sin salida a ese host (o con la nube caída), Descubrir se degrada a
	// una lista vacía, no a un error — DescubrirEnHost es quien manda.
	c := &Cliente{HTTP: &http.Client{Transport: transportQueSiempreFalla{}}}
	dispositivos, err := c.Descubrir(context.Background())
	if err != nil {
		t.Fatalf("Descubrir sin nube no debería fallar: %v", err)
	}
	if len(dispositivos) != 0 {
		t.Fatalf("se esperaba una lista vacía, llegó %+v", dispositivos)
	}
}

func TestDescubrirConfirmaContraDiscoverLocal(t *testing.T) {
	dispositivo := servidorDispositivo(t)
	defer dispositivo.Close()

	nube := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]candidatoNube{{BaseURL: dispositivo.URL}})
	}))
	defer nube.Close()

	c := &Cliente{HTTP: clienteRedirigidoA(nube.URL)}
	dispositivos, err := c.Descubrir(context.Background())
	if err != nil {
		t.Fatalf("Descubrir: %v", err)
	}
	if len(dispositivos) != 1 || dispositivos[0].DeviceID != "10419BB4" {
		t.Fatalf("dispositivos mal descubiertos: %+v", dispositivos)
	}
}

// transportQueSiempreFalla simula no tener salida a internet.
type transportQueSiempreFalla struct{}

func (transportQueSiempreFalla) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errConexionFalsa{}
}

type errConexionFalsa struct{}

func (errConexionFalsa) Error() string { return "sin red (prueba)" }

// clienteRedirigidoA manda toda petición a URLDescubrimientoPublico hacia el
// servidor de nube de la prueba, y deja pasar el resto (discover.json local)
// sin tocar.
func clienteRedirigidoA(nube string) *http.Client {
	return &http.Client{Transport: redirectorDeNube{nube: nube}}
}

type redirectorDeNube struct{ nube string }

func (r redirectorDeNube) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.String() == URLDescubrimientoPublico {
		nuevo := req.Clone(req.Context())
		u, err := req.URL.Parse(r.nube)
		if err != nil {
			return nil, err
		}
		nuevo.URL = u
		nuevo.Host = u.Host
		return http.DefaultTransport.RoundTrip(nuevo)
	}
	return http.DefaultTransport.RoundTrip(req)
}
