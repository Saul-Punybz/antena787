package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"antena787/internal/app"
	"antena787/internal/model"
)

// La puerta del aire por la API (F2-118): que guardar el canal no encienda
// nada, que encender pida las comprobaciones y la confirmación escrita, y que
// lo que pasó quede en la bitácora y en la auditoría.

// comprobacionesBody es lo que contestan las tres rutas del aire.
type comprobacionesBody struct {
	Error          string             `json:"error"`
	Campo          string             `json:"campo"`
	Puede          bool               `json:"puede"`
	Comprobaciones []app.Comprobacion `json:"comprobaciones"`
	Modo           string             `json:"modo"`
	Aviso          string             `json:"aviso"`
}

// conTodoListoParaElAire deja el canal en condiciones de encender: una salida
// que abre y un cartel de la estación con qué cubrir.
func (c *cliente) conTodoListoParaElAire() {
	c.t.Helper()
	// La comprobación de ffmpeg mira que haya ruta apuntada, no lo ejecuta:
	// nada de este camino lanza un proceso. Así que en una máquina sin ffmpeg
	// se apunta una ruta y la prueba corre igual — la ruta que sí lleva el
	// canal al aire no puede quedarse sin correr por eso. El motor de verdad
	// se prueba con ffmpeg de verdad en internal/app (F2-118). El motor no
	// está andando aquí: el arnés abre la aplicación con app.Open y nunca
	// llama a Start, así que escribir estos campos no compite con nadie.
	if c.a.FFmpeg == "" || c.a.FFprobe == "" {
		c.a.FFmpeg, c.a.FFprobe, c.a.FFmpegErr = "/bin/ffmpeg", "/bin/ffprobe", nil
	}
	ctx := context.Background()
	dir := c.t.TempDir()
	cartel := filepath.Join(dir, "cartel.mkv")
	if err := os.WriteFile(cartel, []byte("no es un video de verdad, pero pesa"), 0o644); err != nil {
		c.t.Fatalf("no pude dejar el cartel: %v", err)
	}
	if err := c.a.Store.Settings.Set(ctx, app.KeyDefaultFiller, cartel); err != nil {
		c.t.Fatalf("no pude apuntar el cartel: %v", err)
	}
	salida := model.Output{
		ChannelID: c.a.ChannelID, Name: "Grabación", Driver: "archivo",
		Params: fmt.Sprintf(`{"ruta": %q}`, filepath.Join(dir, "aire.ts")),
	}
	if err := c.a.Store.Output.Upsert(ctx, &salida); err != nil {
		c.t.Fatalf("no pude configurar la salida: %v", err)
	}
}

func (c *cliente) modoDelCanal() string {
	c.t.Helper()
	ch, err := c.a.Store.Channel.Get(context.Background(), c.a.ChannelID)
	if err != nil {
		c.t.Fatalf("no pude leer el canal: %v", err)
	}
	return ch.Mode
}

// Guardar el canal no toca el modo: ni lo fuerza a sombra ni lo pone al aire
// porque alguien mandó el campo. Encender tiene su propia puerta.
func TestGuardarElCanalNoCambiaElModo(t *testing.T) {
	c := nuevo(t).conClave().entrar()

	w := c.do("PUT", "/api/v1/canal", map[string]any{
		"nombre": "CAtv Mayagüez", "modo": "aire",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("guardar el canal dio %d: %s", w.Code, w.Body.String())
	}
	var ch map[string]any
	c.json(w, &ch)
	if ch["modo"] != "sombra" {
		t.Fatalf("guardar el canal lo puso en %v: el modo no se cambia por aquí", ch["modo"])
	}
	if ch["nombre"] != "CAtv Mayagüez" {
		t.Fatalf("el nombre no se guardó: %v", ch["nombre"])
	}
	if c.modoDelCanal() != "sombra" {
		t.Fatalf("el canal quedó en %q", c.modoDelCanal())
	}
}

// Las comprobaciones se pueden mirar sin cambiar nada, y vienen con todo lo
// que la pantalla pinta (el contrato de web/src/lib/tipos.ts).
func TestLasComprobacionesDelAireSeMiranSinEncenderNada(t *testing.T) {
	c := nuevo(t).conClave().entrar()

	w := c.do("GET", "/api/v1/canal/comprobaciones", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/canal/comprobaciones dio %d: %s", w.Code, w.Body.String())
	}
	var body comprobacionesBody
	c.json(w, &body)
	if body.Modo != "sombra" {
		t.Fatalf("el modo contestado es %q", body.Modo)
	}
	if body.Puede {
		t.Fatal("un canal recién instalado, sin salida ni relleno, no puede salir al aire")
	}
	if len(body.Comprobaciones) < 4 {
		t.Fatalf("solo se comprobaron %d cosas: %v", len(body.Comprobaciones), body.Comprobaciones)
	}

	// Cada comprobación trae lo que la interfaz declara como obligatorio.
	var crudo struct {
		Comprobaciones []map[string]any `json:"comprobaciones"`
	}
	c.json(w, &crudo)
	for _, prop := range propiedadesObligatorias(t, "Comprobacion") {
		for i, comp := range crudo.Comprobaciones {
			if _, hay := comp[prop]; !hay {
				t.Fatalf("la comprobación %d no manda %q, que la interfaz pinta: %v", i, prop, comp)
			}
		}
	}
	// Y ninguna frase es un código: se leen enteras.
	for _, comp := range body.Comprobaciones {
		if strings.TrimSpace(comp.Texto) == "" || strings.TrimSpace(comp.Nombre) == "" {
			t.Fatalf("una comprobación viene sin frase: %+v", comp)
		}
		if comp.Resultado == app.CompFalta && comp.Arreglo == "" {
			t.Fatalf("la comprobación %q falta y no dice qué hacer", comp.Clave)
		}
	}
	if c.modoDelCanal() != "sombra" {
		t.Fatal("mirar las comprobaciones cambió el modo del canal")
	}
}

// Sin la confirmación escrita no se enciende, y el error dice qué va a pasar,
// no solo que falta un campo.
func TestSalirAlAireSinConfirmarNoEnciendeNada(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	c.conTodoListoParaElAire()

	for _, escrito := range []string{"", "si", "ALAIRE?"} {
		w := c.do("POST", "/api/v1/canal/al-aire", map[string]any{"confirmacion": escrito})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("con la confirmación %q contestó %d: %s", escrito, w.Code, w.Body.String())
		}
		var e errorBody
		c.json(w, &e)
		if e.Field != "confirmacion" || !strings.Contains(e.Error, "AL AIRE") {
			t.Fatalf("el error de la confirmación es %+v", e)
		}
	}
	if c.modoDelCanal() != "sombra" {
		t.Fatal("el canal se encendió sin confirmar")
	}

	// Y escribirlo en minúsculas o con espacios de sobra sí cuenta: se pide
	// intención, no dictado.
	w := c.do("POST", "/api/v1/canal/al-aire", map[string]any{"confirmacion": "  al   aire "})
	if w.Code != http.StatusOK {
		t.Fatalf("con la confirmación escrita en minúsculas contestó %d: %s", w.Code, w.Body.String())
	}
}

// Si falta algo, el 409 trae la frase de lo que falta, el campo, y la lista
// entera para que la pantalla la pinte sin volver a preguntar.
func TestSalirAlAireSinSalidaLoDiceYNoEnciende(t *testing.T) {
	c := nuevo(t).conClave().entrar()

	w := c.do("POST", "/api/v1/canal/al-aire", map[string]any{"confirmacion": "AL AIRE"})
	if w.Code != http.StatusConflict {
		t.Fatalf("salir al aire sin salida dio %d: %s", w.Code, w.Body.String())
	}
	var body comprobacionesBody
	c.json(w, &body)
	if !strings.Contains(body.Error, "todavía no se puede salir al aire") {
		t.Fatalf("el error no está en palabras claras: %q", body.Error)
	}
	if body.Campo == "" || len(body.Comprobaciones) == 0 || body.Puede {
		t.Fatalf("el 409 no trae las comprobaciones: %+v", body)
	}
	if c.modoDelCanal() != "sombra" {
		t.Fatal("el canal se encendió aunque faltaba algo")
	}
}

// El camino entero: se enciende, queda el incidente con su frase y la
// auditoría con quién lo hizo, y se vuelve a sombra igual.
func TestElCanalSaleDeSombraYVuelve(t *testing.T) {
	c := nuevo(t).conClave().entrar()
	c.conTodoListoParaElAire()

	w := c.do("POST", "/api/v1/canal/al-aire", map[string]any{"confirmacion": "AL AIRE"})
	if w.Code != http.StatusOK {
		t.Fatalf("salir al aire dio %d: %s", w.Code, w.Body.String())
	}
	var body comprobacionesBody
	c.json(w, &body)
	if body.Modo != "aire" || !body.Puede || body.Aviso == "" {
		t.Fatalf("la respuesta de encender es %+v", body)
	}
	if c.modoDelCanal() != "aire" {
		t.Fatalf("el canal quedó en %q", c.modoDelCanal())
	}

	// Encender dos veces no es un cambio: se dice que ya está.
	w = c.do("POST", "/api/v1/canal/al-aire", map[string]any{"confirmacion": "AL AIRE"})
	if w.Code != http.StatusConflict {
		t.Fatalf("encender estando al aire dio %d: %s", w.Code, w.Body.String())
	}

	// La bitácora lo cuenta con una frase, no con un código.
	c.hayIncidente(model.IncAlAire.String())

	// Y la auditoría dice quién movió el modo.
	c.auditoriaDelModo("sombra", "aire")

	// De vuelta a sombra, con la misma seriedad.
	w = c.do("POST", "/api/v1/canal/a-sombra", map[string]any{"confirmacion": "no"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("volver a sombra sin confirmar dio %d: %s", w.Code, w.Body.String())
	}
	w = c.do("POST", "/api/v1/canal/a-sombra", map[string]any{"confirmacion": "sombra"})
	if w.Code != http.StatusOK {
		t.Fatalf("volver a sombra dio %d: %s", w.Code, w.Body.String())
	}
	if c.modoDelCanal() != "sombra" {
		t.Fatalf("el canal quedó en %q", c.modoDelCanal())
	}
	c.hayIncidente(model.IncASombra.String())
	c.auditoriaDelModo("aire", "sombra")

	// Y /estado lo dice, que es de donde la pantalla lo lee siempre.
	w = c.do("GET", "/api/v1/estado", nil)
	var est map[string]any
	c.json(w, &est)
	if est["modo"] != "sombra" {
		t.Fatalf("/estado dice %v", est["modo"])
	}
}

// hayIncidente comprueba que la bitácora tiene ese tipo, con su frase en palabras
// claras tal como la pinta Al aire.
func (c *cliente) hayIncidente(tipo string) {
	c.t.Helper()
	w := c.do("GET", "/api/v1/incidentes", nil)
	if w.Code != http.StatusOK {
		c.t.Fatalf("/incidentes dio %d: %s", w.Code, w.Body.String())
	}
	var lista []struct {
		Tipo    string `json:"tipo"`
		Texto   string `json:"texto"`
		Detalle string `json:"detalle"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &lista); err != nil {
		c.t.Fatalf("la bitácora no es JSON: %s", w.Body.String())
	}
	for _, inc := range lista {
		if inc.Tipo != tipo {
			continue
		}
		if strings.TrimSpace(inc.Texto) == "" || inc.Texto == tipo {
			c.t.Fatalf("el incidente %q no trae frase clara: %q", tipo, inc.Texto)
		}
		if !strings.Contains(inc.Detalle, "señal") {
			c.t.Fatalf("el incidente %q no dice qué pasó con la señal: %q", tipo, inc.Detalle)
		}
		return
	}
	c.t.Fatalf("no hay ningún incidente %q en la bitácora: %+v", tipo, lista)
}

// auditoriaDelModo comprueba que el cambio de modo quedó firmado.
func (c *cliente) auditoriaDelModo(antes, despues string) {
	c.t.Helper()
	w := c.do("GET", "/api/v1/auditoria?entidad=channel", nil)
	if w.Code != http.StatusOK {
		c.t.Fatalf("/auditoria dio %d: %s", w.Code, w.Body.String())
	}
	var lista []struct {
		Campo  string `json:"campo"`
		Antes  string `json:"valor_anterior"`
		Ahora  string `json:"valor_nuevo"`
		Autor  string `json:"autor"`
		Origen string `json:"origen"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &lista); err != nil {
		c.t.Fatalf("la auditoría no es JSON: %s", w.Body.String())
	}
	for _, e := range lista {
		if e.Campo == "modo" && e.Antes == antes && e.Ahora == despues {
			if strings.TrimSpace(e.Autor) == "" || e.Origen != "humano" {
				c.t.Fatalf("el cambio de modo quedó sin autor: %+v", e)
			}
			return
		}
	}
	c.t.Fatalf("el cambio de modo %s → %s no quedó en la auditoría: %+v", antes, despues, lista)
}
