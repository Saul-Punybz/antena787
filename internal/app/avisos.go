package app

// Los avisos que salen de la máquina (F1-46) y el envío opcional de la guía
// a un destino externo (F1-49).
//
// Las dos cosas comparten la misma regla: **nada de lo que pase aquí puede
// tocar el aire**. Un aviso que no sale, o una guía que el otro lado
// rechaza, dejan una alarma de nivel «aviso» y una línea en la bitácora, y
// ahí se acaba. Nunca devuelven error al camino que arma el plan.
//
// Sin dependencias de fuera: Telegram por su API de bots con net/http y el
// correo con net/smtp, que es lo que trae Go.

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

const (
	// UmbralDeAviso es el único umbral que sale por el canal de avisos. Los
	// de 30 y 14 días se quedan en pantalla: el de 7 es el que hay que
	// atender aunque nadie esté mirando (F1-46).
	UmbralDeAviso = "7"

	// CanalNinguno, CanalTelegram y CanalCorreo son los valores del ajuste
	// avisos_canal.
	CanalNinguno  = "ninguno"
	CanalTelegram = "telegram"
	CanalCorreo   = "correo"

	// TiempoDeAviso es lo que se espera a que el otro lado conteste.
	TiempoDeAviso = 10 * time.Second
	// TiempoDeGuia es lo que se espera al mandar la guía a su destino.
	TiempoDeGuia = 10 * time.Second
)

// telegramAPI es la raíz de la API de bots de Telegram. Es una variable para
// que las pruebas puedan apuntarla a un servidor de mentira.
var telegramAPI = "https://api.telegram.org"

// avisoConfig es el canal de avisos tal como está configurado ahora mismo.
type avisoConfig struct {
	Canal string

	Token string // Telegram
	Chat  string // Telegram

	Para     string // correo
	Servidor string // "servidor:puerto"
	Usuario  string
	Clave    string
}

// leerAvisoConfig lee el canal de avisos de los ajustes.
func (a *App) leerAvisoConfig(ctx context.Context) avisoConfig {
	canal := strings.ToLower(a.setting(ctx, KeyNoticeChannel))
	if canal == "" {
		canal = CanalNinguno
	}
	return avisoConfig{
		Canal:    canal,
		Token:    a.setting(ctx, KeyTelegramToken),
		Chat:     a.setting(ctx, KeyTelegramChat),
		Para:     a.setting(ctx, KeyNoticeMailTo),
		Servidor: a.setting(ctx, KeySMTPServer),
		Usuario:  a.setting(ctx, KeySMTPUser),
		Clave:    a.setting(ctx, KeySMTPPass),
	}
}

// TextoDeAviso arma el mensaje que se manda: una línea con el asunto y
// debajo el aviso tal como se lee en pantalla. Sin formato, sin adornos:
// llega igual a un teléfono que a un correo.
func TextoDeAviso(asunto, texto string) string {
	asunto = strings.TrimSpace(asunto)
	texto = strings.TrimSpace(texto)
	switch {
	case asunto == "":
		return texto
	case texto == "":
		return asunto
	}
	return asunto + "\n\n" + texto
}

// Notificar saca un aviso por el canal configurado. No devuelve nada a
// propósito: quien la llama está armando el plan y no se puede quedar
// esperando ni fallar por esto.
func (a *App) Notificar(ctx context.Context, asunto, texto string) {
	cfg := a.leerAvisoConfig(ctx)
	if cfg.Canal == "" || cfg.Canal == CanalNinguno {
		return
	}
	var err error
	var comoSeLlama string
	switch cfg.Canal {
	case CanalTelegram:
		comoSeLlama = "Telegram"
		err = enviarPorTelegram(ctx, telegramAPI, cfg, TextoDeAviso(asunto, texto))
	case CanalCorreo:
		comoSeLlama = "correo"
		err = enviarPorCorreo(cfg, asunto, texto)
	default:
		comoSeLlama = cfg.Canal
		err = fmt.Errorf("no sé mandar avisos por %q: pon telegram, correo o ninguno", cfg.Canal)
	}
	if err != nil {
		aviso := fmt.Sprintf("no pude enviar el aviso por %s: %s", comoSeLlama, err)
		a.setAlarms("avisos", []Alarma{{
			Tipo:    "avisos",
			Nivel:   NivelAviso,
			Texto:   aviso,
			Detalle: "el aviso sigue en pantalla; revisa el canal de avisos en Ajustes",
			Accion:  &AccionAlarma{Texto: "ver los ajustes", Ruta: "/ajustes"},
		}})
		a.Publish("avisos", "envio", aviso)
		return
	}
	a.setAlarms("avisos", nil)
	a.Publish("avisos", "envio", "el aviso salió por "+comoSeLlama)
}

// ── Telegram ──────────────────────────────────────────────────────────

// enviarPorTelegram manda el texto por la API de bots. base es la raíz del
// servicio, que en producción es telegramAPI.
func enviarPorTelegram(ctx context.Context, base string, cfg avisoConfig, texto string) error {
	if cfg.Token == "" || cfg.Chat == "" {
		return errors.New("faltan la clave del bot de Telegram o el número del chat")
	}
	cuerpo, err := json.Marshal(map[string]string{"chat_id": cfg.Chat, "text": texto})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, TiempoDeAviso)
	defer cancel()

	url := strings.TrimSuffix(base, "/") + "/bot" + cfg.Token + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(cuerpo))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	respuesta, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("Telegram contestó %d: %s", res.StatusCode, unaLinea(string(respuesta)))
	}
	return nil
}

// ── correo ────────────────────────────────────────────────────────────

// CorreoCrudo arma el mensaje tal como viaja: cabeceras mínimas y el texto en
// UTF-8. El asunto va sin codificar en base64 porque los servidores que nos
// interesan aceptan 8 bits; si alguno se queja, se ve en la bitácora.
func CorreoCrudo(de, para, asunto, texto string) []byte {
	var b strings.Builder
	b.WriteString("From: " + de + "\r\n")
	b.WriteString("To: " + para + "\r\n")
	b.WriteString("Subject: " + unaLinea(asunto) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(strings.TrimSpace(texto), "\n", "\r\n"))
	b.WriteString("\r\n")
	return []byte(b.String())
}

// unaLinea deja el texto en una sola línea: una cabecera de correo no puede
// llevar saltos, y un mensaje de Telegram se lee mejor así.
func unaLinea(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// remitenteDe elige de qué dirección sale el correo: la del usuario si la
// hay, y si no una del propio programa contra el servidor configurado.
func remitenteDe(cfg avisoConfig) string {
	if strings.Contains(cfg.Usuario, "@") {
		return cfg.Usuario
	}
	host := cfg.Servidor
	if h, _, err := net.SplitHostPort(cfg.Servidor); err == nil {
		host = h
	}
	if host == "" {
		host = "localhost"
	}
	return "antena787@" + host
}

// enviarPorCorreo manda el aviso por SMTP, con cifrado en cuanto el servidor
// diga que lo tiene.
func enviarPorCorreo(cfg avisoConfig, asunto, texto string) error {
	if cfg.Servidor == "" || cfg.Para == "" {
		return errors.New("faltan el servidor de correo o a quién mandarle el aviso")
	}
	host, _, err := net.SplitHostPort(cfg.Servidor)
	if err != nil {
		return fmt.Errorf("el servidor de correo se escribe servidor:puerto, y llegó %q", cfg.Servidor)
	}
	c, err := smtp.Dial(cfg.Servidor)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if cfg.Usuario != "" {
		if err := c.Auth(smtp.PlainAuth("", cfg.Usuario, cfg.Clave, host)); err != nil {
			return err
		}
	}
	de := remitenteDe(cfg)
	if err := c.Mail(de); err != nil {
		return err
	}
	for _, para := range strings.Split(cfg.Para, ",") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if err := c.Rcpt(para); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(CorreoCrudo(de, cfg.Para, asunto, texto)); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	if err := c.Quit(); err != nil {
		var proto *textproto.Error
		if !errors.As(err, &proto) {
			return err
		}
	}
	return nil
}

// ── el envío opcional de la guía (F1-49) ──────────────────────────────

// pushGuide manda la guía al destino configurado, si lo hay, en su propia
// goroutine: la publicación local ya terminó y no espera a nadie. Que el
// destino no conteste no rompe ni la guía en disco ni /guia.xml.
func (a *App) pushGuide(ctx context.Context, data []byte) {
	destino := a.setting(ctx, KeyGuideHTTP)
	if destino == "" {
		return
	}
	copia := append([]byte(nil), data...)
	go func() {
		if err := enviarGuia(destino, copia); err != nil {
			aviso := fmt.Sprintf("no pude mandar la guía a %s: %s", destino, err)
			a.setAlarms("guia", []Alarma{{
				Tipo:    "guia",
				Nivel:   NivelAviso,
				Texto:   aviso,
				Detalle: "la guía local se publicó igual; esto es solo la copia que sale a la red",
				Accion:  &AccionAlarma{Texto: "ver los ajustes", Ruta: "/ajustes"},
			}})
			a.Publish("plan", "guia", aviso)
			return
		}
		a.Publish("plan", "guia", "la guía se mandó a "+destino)
	}()
}

// pushGuidePMCP es pushGuide para la guía en PMCP (ATSC A/76): mismo patrón,
// mismo destino opcional por su propia clave (guia_pmcp_destino_http), y que
// falle tampoco rompe nada —ni la guía PMCP en memoria ni, mucho menos, la
// guía XMLTV o el aire—.
func (a *App) pushGuidePMCP(ctx context.Context, data []byte) {
	destino := a.setting(ctx, KeyGuidePMCPHTTP)
	if destino == "" {
		return
	}
	copia := append([]byte(nil), data...)
	go func() {
		if err := enviarGuia(destino, copia); err != nil {
			aviso := fmt.Sprintf("no pude mandar la guía PMCP a %s: %s", destino, err)
			a.setAlarms("guia_pmcp", []Alarma{{
				Tipo:    "guia_pmcp",
				Nivel:   NivelAviso,
				Texto:   aviso,
				Detalle: "la guía PMCP en /guia.pmcp se publicó igual; esto es solo la copia que sale a la red",
				Accion:  &AccionAlarma{Texto: "ver los ajustes", Ruta: "/ajustes"},
			}})
			a.Publish("plan", "guia_pmcp", aviso)
			return
		}
		a.Publish("plan", "guia_pmcp", "la guía PMCP se mandó a "+destino)
	}()
}

// enviarGuia hace el POST del XML al destino, con su propio límite de tiempo.
func enviarGuia(destino string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), TiempoDeGuia)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destino, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/xml")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	respuesta, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("contestó %d: %s", res.StatusCode, unaLinea(string(respuesta)))
	}
	return nil
}
