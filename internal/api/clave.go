package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"antena787/internal/app"
	"antena787/internal/model"
	"antena787/internal/store"
)

// CookieName es la cookie de la clave de estación. HttpOnly y SameSite=Strict
// (PRD §19, auditoría F): no es un login con roles, es una sola clave que se
// pide una vez por navegador.
const CookieName = "antena_sesion"

// SessionLife es lo que dura una sesión. Larga a propósito: la clave existe
// para que el sobrino que se conecta al wifi no saque el canal del aire, no
// para estorbarle a quien opera.
const SessionLife = 365 * 24 * time.Hour

// Límite de intentos: cinco por minuto y por dirección (auditoría F).
const (
	MaxAttempts   = 5
	AttemptWindow = time.Minute
)

// sessions son las sesiones vivas en memoria. La cookie va firmada con un
// secreto guardado en settings, así que una sesión sobrevive a un reinicio
// del proceso aunque el mapa se haya vaciado.
type sessions struct {
	mu   sync.Mutex
	live map[string]string // id → autor
}

func newSessions() *sessions { return &sessions{live: map[string]string{}} }

func (s *sessions) add(id, author string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.live[id] = author
}

func (s *sessions) drop(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.live, id)
}

func (s *sessions) author(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.live[id]
	return a, ok
}

// attempts cuenta los intentos de clave por dirección.
type attempts struct {
	mu   sync.Mutex
	seen map[string][]time.Time
}

func newAttempts() *attempts { return &attempts{seen: map[string][]time.Time{}} }

// allow apunta un intento y dice si todavía cabe.
func (a *attempts) allow(ip string, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	var recent []time.Time
	for _, t := range a.seen[ip] {
		if now.Sub(t) < AttemptWindow {
			recent = append(recent, t)
		}
	}
	if len(recent) >= MaxAttempts {
		a.seen[ip] = recent
		return false
	}
	a.seen[ip] = append(recent, now)
	return true
}

func (a *attempts) clear(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.seen, ip)
}

// ── la cookie ─────────────────────────────────────────────────────────

// secret devuelve el secreto con que se firma la cookie, creándolo la
// primera vez. Vive en settings y no sale en GET /ajustes.
func (s *Server) secret(ctx context.Context) ([]byte, error) {
	raw, err := s.App.Store.Settings.Get(ctx, app.KeySessionSecret)
	if err == nil && raw != "" {
		return hex.DecodeString(raw)
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	hexed := hex.EncodeToString(buf)
	if err := s.App.Store.Settings.Set(ctx, app.KeySessionSecret, hexed); err != nil {
		return nil, err
	}
	return buf, nil
}

// sign arma el valor de la cookie: identificador, instante de emisión y la
// firma de los dos. Sin la firma no vale, y la firma sale del secreto.
func sign(secret []byte, id string, issued time.Time) string {
	payload := id + "." + time.Time(issued).UTC().Format(time.RFC3339)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

// verify comprueba la firma y la edad, y devuelve el identificador.
func verify(secret []byte, value string, now time.Time) (string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return "", false
	}
	payload := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(parts[2])) {
		return "", false
	}
	issued, err := time.Parse(time.RFC3339, parts[1])
	if err != nil || now.Sub(issued) > SessionLife {
		return "", false
	}
	return parts[0], true
}

// setCookie deja la sesión puesta en el navegador.
func (s *Server) setCookie(w http.ResponseWriter, ctx context.Context, author string) error {
	secret, err := s.secret(ctx)
	if err != nil {
		return err
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return err
	}
	id := hex.EncodeToString(buf)
	s.sessions.add(id, author)
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    sign(secret, id, s.Now()),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   s.opts.SecureCookie,
		MaxAge:   int(SessionLife / time.Second),
	})
	return nil
}

// clearCookie borra la sesión del navegador.
func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   s.opts.SecureCookie,
		MaxAge:   -1,
	})
}

// ── el guardia ────────────────────────────────────────────────────────

type ctxKey int

const authorKey ctxKey = 1

// guard exige la clave de estación salvo donde el contrato dice que no:
// /estado, /entrar, /guia.xml, y el asistente mientras no haya clave puesta.
func (s *Server) guard(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		hasPIN, err := s.App.Store.Settings.HasPIN(ctx)
		if err != nil {
			fail(w, http.StatusInternalServerError, "no se pudo leer la clave de la estación: "+err.Error(), "")
			return
		}
		// Sin clave puesta, el asistente es lo único que existe: es donde se
		// pone la clave (paso 1).
		if !hasPIN && strings.HasPrefix(r.URL.Path, "/api/v1/instalacion") {
			h(w, r.WithContext(context.WithValue(ctx, authorKey, "estación")))
			return
		}
		author, ok := s.author(r)
		if !ok {
			if !hasPIN {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error":                "todavía no hay clave de estación: termina la instalación primero",
					"necesita_instalacion": true,
				})
				return
			}
			fail(w, http.StatusUnauthorized, "hace falta la clave de la estación para entrar aquí", "clave")
			return
		}
		h(w, r.WithContext(context.WithValue(ctx, authorKey, author)))
	})
}

// author lee la sesión de la cookie. Una firma buena vale aunque el proceso
// se haya reiniciado: para eso el secreto vive en settings.
func (s *Server) author(r *http.Request) (string, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	secret, err := s.secret(r.Context())
	if err != nil {
		return "", false
	}
	id, ok := verify(secret, c.Value, s.Now())
	if !ok {
		return "", false
	}
	if a, live := s.sessions.author(id); live {
		return a, true
	}
	// Sesión firmada de antes del reinicio: se readmite con el autor que
	// diga la configuración.
	a := s.operator(r.Context())
	s.sessions.add(id, a)
	return a, true
}

// operator es quien firma la auditoría: el nombre configurado o "estación".
func (s *Server) operator(ctx context.Context) string {
	if v, err := s.App.Store.Settings.Get(ctx, app.KeyOperator); err == nil && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return "estación"
}

// autor es quien está haciendo el cambio, para el audit_log.
func autor(r *http.Request) string {
	if a, ok := r.Context().Value(authorKey).(string); ok && a != "" {
		return a
	}
	return "estación"
}

// ── entrar y salir ────────────────────────────────────────────────────

func (s *Server) entrar(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Clave string `json:"clave"`
	}
	if !decode(w, r, &body) {
		return
	}
	ctx := r.Context()
	ip := clientIP(r)
	if !s.attempts.allow(ip, s.Now()) {
		fail(w, http.StatusTooManyRequests,
			"demasiados intentos seguidos: espera un minuto y vuelve a probar", "clave")
		return
	}
	ok, err := s.App.Store.Settings.CheckPIN(ctx, body.Clave)
	if err != nil {
		fail(w, http.StatusInternalServerError, "no se pudo comprobar la clave: "+err.Error(), "")
		return
	}
	if !ok {
		hasPIN, _ := s.App.Store.Settings.HasPIN(ctx)
		if !hasPIN {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error":                "todavía no hay clave de estación: termina la instalación primero",
				"necesita_instalacion": true,
			})
			return
		}
		fail(w, http.StatusUnauthorized, "esa no es la clave de la estación", "clave")
		return
	}
	s.attempts.clear(ip)
	author := s.operator(ctx)
	if err := s.setCookie(w, ctx, author); err != nil {
		fail(w, http.StatusInternalServerError, "no se pudo abrir la sesión: "+err.Error(), "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entraste": true, "autor": author})
}

func (s *Server) salir(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		if secret, serr := s.secret(r.Context()); serr == nil {
			if id, ok := verify(secret, c.Value, s.Now()); ok {
				s.sessions.drop(id)
			}
		}
	}
	s.clearCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"saliste": true})
}

// clientIP es la dirección desde la que llega la petición, para el límite de
// intentos. No se mira ningún encabezado de proxy: esto escucha en loopback
// y en Tailscale, y un encabezado se falsifica solo.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ── auditoría ─────────────────────────────────────────────────────────

// audit deja constancia de un cambio con origen humano y el autor de la
// sesión. Si no se puede escribir, el cambio ya está hecho: no se deshace,
// pero se avisa por el canal de eventos.
func (s *Server) audit(r *http.Request, entity string, id *int64, field, before, after string) {
	e := model.AuditEntry{
		Entity:   entity,
		EntityID: id,
		Field:    field,
		Before:   before,
		After:    after,
		Author:   autor(r),
		Origin:   "humano",
		At:       s.Now(),
		Kind:     "cambio",
	}
	if err := s.App.Store.Audit.Append(r.Context(), &e); err != nil {
		s.App.Publish("auditoria", "fallo", "no se pudo anotar el cambio: "+err.Error())
	}
}
