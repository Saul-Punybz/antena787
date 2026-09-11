// Package hdhomerun es el driver de retorno de aire (capture_input tipo
// "stream", ADR 0009) para sintonizadores SiliconDust HDHomeRun en red:
// Flex, Flex Duo, Flex Quatro y Connect. La señal vuelve por Ethernet, no
// por un dispositivo local, así que todo esto es net/http contra URLs, sin
// CGo (ADR 0002) y sin ninguna biblioteca de terceros.
//
// Verificado contra la documentación pública de SiliconDust el 10 de
// septiembre de 2026 (docs/drivers/catalogo/04-captura-retorno-gpi.md):
//
//   - descubrimiento local:  GET http://<ip>/discover.json — verificado en
//     info.hdhomerun.com/info/http_api. Campos: FriendlyName, ModelNumber,
//     FirmwareName, FirmwareVersion, DeviceID, BaseURL, LineupURL,
//     TunerCount.
//   - lista de canales:      GET http://<ip>/lineup.json — verificado en la
//     misma página. Campos por canal: GuideNumber, GuideName, URL (y Tags,
//     opcional).
//   - TS crudo del canal:    GET a la URL que trae lineup.json para ese
//     canal, con forma http://<ip>:5004/auto/v<canal> — verificado.
//   - nivel de señal:        "HDHomeRun Development Guide" (20110518,
//     silicondust.com), sección "Checking the signal strength":
//     hdhomerun_config <id> get /tuner<n>/status devuelve
//     "ch=... lock=... ss=... snq=... seq=... bps=... pps=...". ss, snq y
//     seq son los tres números 0-100 que aquí se llaman Señal.FuerzaPct,
//     Señal.RuidoPct y Señal.SimboloPct — verificado, pero es el protocolo
//     binario de hdhomerun_config, no HTTP.
//
// Lo que NO está verificado por escrito: el descubrimiento por la nube de
// SiliconDust (URLDescubrimientoPublico) y el espejo HTTP /status.json que
// traen los modelos recientes para los mismos ss/snq/seq. Ninguno de los dos
// tiene una página de documentación pública equivalente a /info/http_api —
// info.hdhomerun.com/info/discovery_protocol y /info/http_status no existen
// todavía en el wiki de SiliconDust (comprobado el 10 sep 2026). Por eso
// ambos se leen con tolerancia: si no responden o no traen los campos
// esperados, no es un error, es simplemente que no hay dato — la vía dura,
// discover.json + lineup.json + el TS crudo medido con internal/ts, no
// depende de ninguno de los dos.
//
// El protocolo de descubrimiento UDP del puerto 65001 queda fuera por la
// misma razón: SiliconDust no publica su formato de paquete por escrito
// (catálogo). discover.json local basta y es lo que este paquete usa.
package hdhomerun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Dispositivo es lo que devuelve discover.json de un HDHomeRun.
type Dispositivo struct {
	DeviceID        string `json:"DeviceID"`
	FriendlyName    string `json:"FriendlyName"`
	ModelNumber     string `json:"ModelNumber"`
	FirmwareName    string `json:"FirmwareName"`
	FirmwareVersion string `json:"FirmwareVersion"`
	BaseURL         string `json:"BaseURL"`
	LineupURL       string `json:"LineupURL"`
	TunerCount      int    `json:"TunerCount"`
}

// Canal es una fila de lineup.json: un canal que el HDHomeRun ve ahora mismo.
type Canal struct {
	GuideNumber string `json:"GuideNumber"`
	GuideName   string `json:"GuideName"`
	URL         string `json:"URL"`
	Tags        string `json:"Tags,omitempty"`
}

// Cliente habla por HTTP con uno o varios HDHomeRun. Todo lo que toca la red
// es inyectable, para que las pruebas no salgan a la red de verdad.
type Cliente struct {
	// HTTP hace las peticiones. nil usa http.DefaultClient.
	HTTP *http.Client
	// EsperaInicial y EsperaMaxima gobiernan la espera progresiva de Abrir:
	// se dobla desde EsperaInicial hasta EsperaMaxima (PRD §10, F2-48). Cero
	// en cualquiera de los dos toma el valor de fábrica (1 s y 60 s).
	EsperaInicial time.Duration
	EsperaMaxima  time.Duration
	// Dormir espera d, o devuelve el error de ctx si se cancela antes. nil
	// usa un temporizador de verdad; las pruebas inyectan uno que no espera
	// de verdad, para no tardar minutos comprobando el tope de 60 s.
	Dormir func(ctx context.Context, d time.Duration) error
}

func (c *Cliente) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Cliente) esperaInicial() time.Duration {
	if c.EsperaInicial > 0 {
		return c.EsperaInicial
	}
	return time.Second
}

func (c *Cliente) esperaMaxima() time.Duration {
	if c.EsperaMaxima > 0 {
		return c.EsperaMaxima
	}
	return 60 * time.Second
}

func (c *Cliente) dormir(ctx context.Context, d time.Duration) error {
	if c.Dormir != nil {
		return c.Dormir(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// obtenerJSON hace un GET y decodifica la respuesta como JSON.
func (c *Cliente) obtenerJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s respondió %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// normalizarBase agrega "http://" si hace falta y quita la barra final, para
// que a las funciones de este paquete se les pueda pasar una IP sola
// ("192.168.1.10"), un host:puerto, o una BaseURL completa.
func normalizarBase(host string) string {
	host = strings.TrimSuffix(strings.TrimSpace(host), "/")
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}
	return host
}

// URLDescubrimientoPublico es el descubrimiento por la nube de SiliconDust:
// en teoría devuelve los HDHomeRun visibles desde la IP pública de quien
// pregunta, sin necesidad de estar en la misma red local. No está en la
// documentación pública (ver el aviso del paquete) — se usa a mejor
// esfuerzo, nunca como la única vía.
const URLDescubrimientoPublico = "http://ipv4-api.hdhomerun.com/discover"

// candidatoNube es lo mínimo que hace falta de la respuesta de la nube: dónde
// buscar el dispositivo en la red local. El resto de los campos que la nube
// pueda mandar no están documentados y no se leen de ahí — Descubrir siempre
// confirma contra discover.json local, la fuente verificada.
type candidatoNube struct {
	LocalIP string `json:"LocalIP"`
	BaseURL string `json:"BaseURL"`
}

// Descubrir busca HDHomeRun por la nube de SiliconDust y confirma cada uno
// contra su discover.json local. Si la nube no contesta —sin internet, o el
// servicio no está disponible— Descubrir no falla: devuelve una lista vacía,
// porque no tener nube no es un error aquí, es DescubrirEnHost quien queda a
// cargo (catálogo, 10 sep 2026: "con el discover HTTP local basta").
func (c *Cliente) Descubrir(ctx context.Context) ([]Dispositivo, error) {
	var candidatos []candidatoNube
	if err := c.obtenerJSON(ctx, URLDescubrimientoPublico, &candidatos); err != nil {
		return nil, nil
	}
	vistos := map[string]bool{}
	var dispositivos []Dispositivo
	for _, cand := range candidatos {
		host := cand.LocalIP
		if host == "" {
			host = cand.BaseURL
		}
		if host == "" || vistos[host] {
			continue
		}
		vistos[host] = true
		d, err := c.DescubrirEnHost(ctx, host)
		if err != nil {
			// Uno que no contesta no tumba el descubrimiento de los demás.
			continue
		}
		dispositivos = append(dispositivos, d)
	}
	return dispositivos, nil
}

// DescubrirEnHost consulta discover.json de un HDHomeRun ya conocido por IP.
// Es la vía documentada por escrito y la que basta cuando no hay
// descubrimiento automático: el asistente puede pedir la IP a mano.
func (c *Cliente) DescubrirEnHost(ctx context.Context, host string) (Dispositivo, error) {
	var d Dispositivo
	url := normalizarBase(host) + "/discover.json"
	if err := c.obtenerJSON(ctx, url, &d); err != nil {
		return Dispositivo{}, fmt.Errorf("discover.json de %s: %w", host, err)
	}
	return d, nil
}

// Lineup pide lineup.json: los canales que el HDHomeRun ve ahora mismo. ip
// puede ser una IP sola, un host:puerto, o una BaseURL completa.
func (c *Cliente) Lineup(ctx context.Context, ip string) ([]Canal, error) {
	var canales []Canal
	url := normalizarBase(ip) + "/lineup.json"
	if err := c.obtenerJSON(ctx, url, &canales); err != nil {
		return nil, fmt.Errorf("lineup.json de %s: %w", ip, err)
	}
	return canales, nil
}
