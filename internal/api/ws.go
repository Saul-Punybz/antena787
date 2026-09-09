package api

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// El WebSocket va con la biblioteca estándar: el apretón de manos son doce
// líneas y los marcos que hacen falta son tres (texto, ping/pong y cierre).
// Meter una dependencia entera —y su superficie de seguridad— para eso, en
// un programa que emite 24/7 y cuya única dependencia externa es ffmpeg, no
// sale a cuenta.
//
// Lo que empuja: `{"tipo":"estado", …}` cada segundo y `{"tipo":"evento", …}`
// en cada incidente o cambio de plan (docs/API.md).

// wsGUID es la constante del RFC 6455 con la que se firma el apretón.
const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Marcos del RFC 6455 que este servidor entiende.
const (
	opText  = 0x1
	opClose = 0x8
	opPing  = 0x9
	opPong  = 0xA
)

// StatusEvery es cada cuánto se empuja el estado.
const StatusEvery = time.Second

// wsConn es una conexión WebSocket ya negociada. Escribe un marco entero de
// una vez, con candado: dos goroutines no pueden entrelazar marcos.
type wsConn struct {
	conn net.Conn
	rw   *bufio.ReadWriter
	mu   sync.Mutex
}

// websocket sube la conexión y empuja estado y eventos hasta que el
// navegador se va.
func (s *Server) websocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrade(w, r)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error(), "")
		return
	}
	defer func() { _ = ws.conn.Close() }()

	events, unsubscribe := s.App.Subscribe()
	defer unsubscribe()

	// Un lector que solo atiende ping y cierre: sin él, el navegador que se
	// va deja la conexión colgada hasta que el sistema operativo se dé cuenta.
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			op, payload, err := ws.read()
			if err != nil {
				return
			}
			switch op {
			case opClose:
				_ = ws.write(opClose, nil)
				return
			case opPing:
				if err := ws.write(opPong, payload); err != nil {
					return
				}
			}
		}
	}()

	tick := time.NewTicker(StatusEvery)
	defer tick.Stop()

	if err := s.pushStatus(ws, r); err != nil {
		return
	}
	for {
		select {
		case <-closed:
			return
		case <-r.Context().Done():
			return
		case e, ok := <-events:
			if !ok {
				return
			}
			if err := ws.writeJSON(e); err != nil {
				return
			}
		case <-tick.C:
			if err := s.pushStatus(ws, r); err != nil {
				return
			}
		}
	}
}

// pushStatus manda el estado de ahora mismo.
func (s *Server) pushStatus(ws *wsConn, r *http.Request) error {
	ctx := r.Context()
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		return err
	}
	now := s.Now()
	msg := map[string]any{
		"tipo":        "estado",
		"ahora":       now,
		"modo":        ch.Mode,
		"dia_emision": ch.BroadcastDay(now),
		"alarmas":     s.App.Alarms(),
		"version":     s.App.Version,
		// El menú lee esto en cada empujón, no solo en el primer /estado: si
		// no fuera, la sexta entrada aparecería y desaparecería sola.
		"hay_anunciantes": s.hayAnunciantes(ctx),
	}
	items, err := s.App.Store.Plan.ListRange(ctx, s.App.ChannelID, now.Add(-6*time.Hour), now.Add(6*time.Hour))
	if err == nil {
		for i := range items {
			it := items[i]
			if !it.PlannedAt.After(now) && it.End().After(now) {
				msg["al_aire"] = it
			} else if it.PlannedAt.After(now) {
				if _, hay := msg["siguiente"]; !hay {
					msg["siguiente"] = it
				}
			}
		}
	}
	return ws.writeJSON(msg)
}

// ── el protocolo ──────────────────────────────────────────────────────

// upgrade hace el apretón de manos del RFC 6455 y se queda con la conexión.
func upgrade(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
		!strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
		return nil, errors.New("esta ruta es un WebSocket: hay que pedirla con Upgrade: websocket")
	}
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		return nil, errors.New("solo hablo la versión 13 del WebSocket")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, errors.New("falta la clave del WebSocket")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("este servidor no puede soltar la conexión para el WebSocket")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}

	sum := sha1.Sum([]byte(key + wsGUID))
	accept := base64.StdEncoding.EncodeToString(sum[:])
	respuesta := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := rw.WriteString(respuesta); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &wsConn{conn: conn, rw: rw}, nil
}

// writeJSON manda un objeto como marco de texto.
func (c *wsConn) writeJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.write(opText, data)
}

// write manda un marco entero, sin máscara (el servidor no enmascara).
func (c *wsConn) write(op byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	head := []byte{0x80 | op}
	n := len(payload)
	switch {
	case n < 126:
		head = append(head, byte(n))
	case n <= 0xFFFF:
		head = append(head, 126, 0, 0)
		binary.BigEndian.PutUint16(head[2:], uint16(n))
	default:
		head = append(head, 127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(head[2:], uint64(n))
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := c.rw.Write(head); err != nil {
		return err
	}
	if _, err := c.rw.Write(payload); err != nil {
		return err
	}
	return c.rw.Flush()
}

// maxFrame es lo más grande que se acepta de un cliente. El navegador solo
// manda ping y cierre: cualquier cosa mayor es un error o un abuso.
const maxFrame = 1 << 20

// read lee un marco del cliente. Los del cliente siempre vienen enmascarados.
func (c *wsConn) read() (byte, []byte, error) {
	var head [2]byte
	if _, err := io.ReadFull(c.rw, head[:]); err != nil {
		return 0, nil, err
	}
	op := head[0] & 0x0F
	masked := head[1]&0x80 != 0
	n := int64(head[1] & 0x7F)

	switch n {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(c.rw, ext[:]); err != nil {
			return 0, nil, err
		}
		n = int64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(c.rw, ext[:]); err != nil {
			return 0, nil, err
		}
		n = int64(binary.BigEndian.Uint64(ext[:]))
	}
	if n > maxFrame {
		return 0, nil, errors.New("el marco del WebSocket es demasiado grande")
	}

	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(c.rw, mask[:]); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(c.rw, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return op, payload, nil
}
