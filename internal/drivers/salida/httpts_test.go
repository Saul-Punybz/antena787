package salida

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"antena787/internal/engine"
	"antena787/internal/model"
	"antena787/internal/ts"
)

// Pruebas de la salida servida por HTTP (F2-115): que dos programas puedan
// tirar de la misma señal a la vez sin estorbarse, que uno que no lee no
// frene a los demás, y que sin nadie conectado la señal se siga produciendo
// —si Go dejara de leer, se cuelga el encoder entero y el canal se queda sin
// aire—.
//
// El que hace de ffmpeg aquí es tsDeMentira: entrega por la misma conexión
// TCP local que el encoder de verdad usa, y con un transport stream que
// internal/ts sabe leer. Lo que se prueba es el reparto, que es lo único que
// este driver hace.

// F2-115, entero: dos clientes a la vez reciben el mismo transport stream
// completo, sin que uno afecte al otro; y sin ningún cliente conectado la
// señal se sigue produciendo, sin error ni caída.
func TestF2_115DosClientesALaVezRecibenLoMismoYSinNadieLaSenalSigue(t *testing.T) {
	puerto := puertoLibre(t)
	d := driverHTTPTS(t, map[string]any{
		"puerto": puerto, "ruta": "/stream.ts", "codec": "mpeg2",
		"bitrate_video_kbs": 2500, "bitrate_mux_kbs": 3200,
	})

	out, err := d.Abrir(engine.CAtv)
	if err != nil {
		t.Fatalf("no se pudo abrir la salida por HTTP: %v", err)
	}
	if out.TCP == "" {
		t.Fatal("la salida no le dijo al encoder por dónde entregar la señal")
	}
	if out.Kind != "mpeg2-ts" {
		t.Fatalf("la salida sale en %q y se pidió MPEG-2", out.Kind)
	}

	// El que hace de ffmpeg: conecta a la escucha que Abrir levantó y empieza
	// a entregar transport stream.
	falso := arrancaTSDeMentira(t, out.TCP)
	defer falso.parar()

	// ── Sin ningún cliente conectado, la señal se sigue produciendo ──
	// Si Go dejara de leer, esta escritura se bloquearía y en el encoder de
	// verdad eso cuelga a ffmpeg, con todas las demás salidas del canal
	// dentro.
	esperaHasta(t, 10*time.Second, func() bool { return falso.escritos() > 1<<20 },
		"sin nadie conectado la señal dejó de producirse: el que entrega se quedó bloqueado")
	if err := falso.error(); err != nil {
		t.Fatalf("sin nadie conectado la entrega falló: %v", err)
	}

	// ── Y ahora dos clientes a la vez ──
	url := fmt.Sprintf("http://127.0.0.1:%d/stream.ts", puerto)
	// Un múltiplo de 188: se cuentan paquetes enteros, no bytes sueltos.
	const bastante = 1600 * ts.PacketSize
	var uno, dos []byte
	var errUno, errDos error
	var listos sync.WaitGroup
	listos.Add(2)
	go func() { defer listos.Done(); uno, errUno = tiraDeLaSenal(url, bastante) }()
	go func() { defer listos.Done(); dos, errDos = tiraDeLaSenal(url, bastante) }()
	listos.Wait()
	if errUno != nil {
		t.Fatalf("el primer cliente no pudo tirar de la señal: %v", errUno)
	}
	if errDos != nil {
		t.Fatalf("el segundo cliente no pudo tirar de la señal: %v", errDos)
	}

	// Los dos reciben un transport stream completo: empieza en un paquete
	// entero, y trae la tabla de programas y la del programa por delante,
	// que es lo que hace falta para entender lo que viene detrás.
	for nombre, recibido := range map[string][]byte{"el primero": uno, "el segundo": dos} {
		if len(recibido)%ts.PacketSize != 0 || recibido[0] != 0x47 {
			t.Fatalf("%s no recibió paquetes enteros: %d bytes y empieza por %#x",
				nombre, len(recibido), recibido[0])
		}
		if pid := uint16(recibido[1]&0x1f)<<8 | uint16(recibido[2]); pid != 0 {
			t.Errorf("%s empezó a recibir en el PID %d y tenía que empezar en la tabla de programas", nombre, pid)
		}
		rep, err := ts.Analyze(bytes.NewReader(recibido))
		if err != nil {
			t.Fatalf("lo que recibió %s no se puede leer como transport stream: %v", nombre, err)
		}
		if rep.Programa != programaDePrueba || rep.TSID != tsidDePrueba || rep.PMTPid != pidPMTDePrueba {
			t.Errorf("%s recibió programa %d, tsid %d y tabla en %d; se emitieron %d, %d y %d",
				nombre, rep.Programa, rep.TSID, rep.PMTPid, programaDePrueba, tsidDePrueba, pidPMTDePrueba)
		}
		if rep.VideoPid() != pidVideoDePrueba || rep.PCRPid != pidVideoDePrueba {
			t.Errorf("%s recibió el video en el PID %d y el reloj en el %d; los dos iban en el %d",
				nombre, rep.VideoPid(), rep.PCRPid, pidVideoDePrueba)
		}
		if rep.CCErrors != 0 {
			t.Errorf("%s recibió %d saltos de continuidad: le faltan trozos", nombre, rep.CCErrors)
		}
	}

	// Y es la misma señal, al mismo tiempo: cada paquete de video lleva su
	// número, y los dos vieron el mismo tramo sin agujeros.
	numsUno := numerosDeVideo(t, uno)
	numsDos := numerosDeVideo(t, dos)
	comun := loQueVieronLosDos(numsUno, numsDos)
	if comun < 500 {
		t.Fatalf("los dos clientes solo coinciden en %d paquetes: no estaban recibiendo lo mismo a la vez (uno del %d al %d, otro del %d al %d)",
			comun, numsUno[0], numsUno[len(numsUno)-1], numsDos[0], numsDos[len(numsDos)-1])
	}

	// Cuando el encoder deja de entregar, la dirección deja de servir señal y
	// lo dice, en vez de quedarse colgada.
	falso.parar()
	esperaHasta(t, 5*time.Second, func() bool {
		r, err := http.Get(url)
		if err != nil {
			return false
		}
		_ = r.Body.Close()
		return r.StatusCode == http.StatusNotFound
	}, "con el encoder parado la dirección siguió diciendo que había señal")
}

// Un cliente que no lee se queda sin señal él solo: ni frena al que sí lee ni
// puede parar la lectura de lo que entrega el encoder.
func TestUnClienteQueNoLeeNoFrenaALosDemas(t *testing.T) {
	puerto := puertoLibre(t)
	d := driverHTTPTS(t, map[string]any{"puerto": puerto, "ruta": "/stream.ts"})
	out, err := d.Abrir(engine.CAtv)
	if err != nil {
		t.Fatalf("no se pudo abrir la salida por HTTP: %v", err)
	}
	falso := arrancaTSDeMentira(t, out.TCP)
	defer falso.parar()

	url := fmt.Sprintf("http://127.0.0.1:%d/stream.ts", puerto)
	// El que se conecta y no lee nunca.
	dormido, err := http.Get(url)
	if err != nil {
		t.Fatalf("el cliente dormido no se pudo conectar: %v", err)
	}
	defer func() { _ = dormido.Body.Close() }()

	// Y el que sí lee: tiene que recibir muchas veces lo que le cabe a un
	// cliente en espera, o sea que al dormido hace rato que se le cortó y a
	// este no le pasó nada.
	const quiere = 2 << 20
	recibido, err := tiraDeLaSenal(url, quiere)
	if err != nil {
		t.Fatalf("el cliente que sí lee se quedó sin señal por culpa del dormido: %v", err)
	}
	if len(recibido) < quiere {
		t.Fatalf("el cliente que sí lee recibió %d bytes de los %d que pidió", len(recibido), quiere)
	}
	if err := falso.error(); err != nil {
		t.Fatalf("la entrega del encoder falló con un cliente dormido conectado: %v", err)
	}
}

// Lo que no se puede servir se dice en palabras claras, no con un código.
func TestLaSalidaPorHTTPExplicaLoQueNoPuedeServir(t *testing.T) {
	casos := []struct {
		params map[string]any
		dice   string
	}{
		{map[string]any{"puerto": 70000}, "va del 1 al 65535"},
		{map[string]any{"puerto": 8080, "codec": "vp9"}, "MPEG-2 (mpeg2)"},
		{map[string]any{"puerto": 8080, "ruta": "/la señal.ts"}, "un espacio o un signo"},
		{map[string]any{"puerto": 8080, "bitrate_video_kbs": 3000, "bitrate_mux_kbs": 3100}, "deja al menos"},
		{map[string]any{"puerto": 8080, "codec": "h264", "audio": "ac3"}, "AAC (aac)"},
		{map[string]any{"puerto": 8080, "audio": "opus"}, "MPEG capa II (mp2)"},
	}
	for _, c := range casos {
		crudos, err := json.Marshal(c.params)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Para(model.Output{Name: "copia", Driver: DriverHTTPTS, Params: string(crudos)}, nil)
		if err == nil {
			t.Errorf("con %v la salida se abrió y no tenía que abrirse", c.params)
			continue
		}
		if !strings.Contains(err.Error(), c.dice) {
			t.Errorf("con %v el error es %q y tenía que decir %q", c.params, err.Error(), c.dice)
		}
	}

	// Sin nada escrito sale la de fábrica, y dice la dirección que hay que
	// escribir en el otro programa.
	d, err := Para(model.Output{Name: "copia", Driver: DriverHTTPTS}, nil)
	if err != nil {
		t.Fatalf("la salida de fábrica no se pudo leer: %v", err)
	}
	texto := d.Descripcion()
	for _, trozo := range []string{"8080", RutaHTTPTSPorDefecto, "MPEG-2"} {
		if !strings.Contains(texto, trozo) {
			t.Errorf("la salida se describe como %q y le falta %q", texto, trozo)
		}
	}
	// Y sin jerga: nadie tiene que saber qué es «mp2» (PRD §4.3).
	if strings.Contains(texto, "mp2") {
		t.Errorf("la descripción enseña la clave del sonido en vez del nombre: %q", texto)
	}

	// Está entre las que se le pueden ofrecer a una persona (F2-115).
	var hay bool
	for _, f := range Disponibles() {
		if f.Driver == DriverHTTPTS && f.Nombre != "" && f.Explicacion != "" {
			hay = true
		}
	}
	if !hay {
		t.Error("http-ts no está entre las salidas que se pueden ofrecer")
	}
}

// La misma salida en H.264, que es la que un navegador puede pintar: ningún
// navegador sabe decodificar MPEG-2 (F2-117).
func TestLaCopiaParaElNavegadorSaleEnH264(t *testing.T) {
	d := driverHTTPTS(t, map[string]any{"puerto": puertoLibre(t), "codec": "h264"})
	out, err := d.Abrir(engine.CAtv)
	if err != nil {
		t.Fatalf("no se pudo abrir la copia para el navegador: %v", err)
	}
	if out.Kind != "h264-ts" {
		t.Fatalf("la copia para el navegador sale en %q y tenía que salir en h264-ts", out.Kind)
	}
	if out.TCP == "" {
		t.Fatal("la copia para el navegador no le dijo al encoder por dónde entregar la señal")
	}
	// Y el encoder la escribe ahí, no a un archivo.
	destinos := strings.Join(out.Destinos(), " ")
	if destinos != "tcp://"+out.TCP {
		t.Fatalf("el encoder escribe a %q y tenía que escribir a la escucha del reparto", destinos)
	}
	if !strings.Contains(d.Descripcion(), "navegador") {
		t.Errorf("la copia para el navegador no dice para qué es: %q", d.Descripcion())
	}
}

// ── lo que hace falta para probar ─────────────────────────────────────

// Lo que emite el transport stream de mentira. Son los mismos números que la
// prueba de punta a punta de la salida al multiplexor, para que se lean igual.
const (
	tsidDePrueba     = 99
	programaDePrueba = 7
	pidPMTDePrueba   = 480
	pidVideoDePrueba = 512
	// paquetesPorTabla es cada cuántos paquetes de video se repiten las dos
	// tablas. Con 20, un cliente que llega a mitad espera muy poco.
	paquetesPorTabla = 20
)

func driverHTTPTS(t *testing.T, params map[string]any) Driver {
	t.Helper()
	crudos, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Para(model.Output{Name: "copia por HTTP", Driver: DriverHTTPTS, Params: string(crudos)}, nil)
	if err != nil {
		t.Fatalf("no se pudo leer la salida por HTTP: %v", err)
	}
	return d
}

// puertoLibre pide uno al sistema y lo suelta: es lo más cerca que se puede
// estar de un puerto libre sin quedárselo.
func puertoLibre(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("no hay puertos libres: %v", err)
	}
	puerto := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return puerto
}

// tiraDeLaSenal es lo que hace un VLC remoto: pide la dirección y lee.
func tiraDeLaSenal(url string, cuanto int) ([]byte, error) {
	// Con plazo: una prueba que se queda esperando señal para siempre no
	// dice qué se rompió.
	cliente := &http.Client{Timeout: 60 * time.Second}
	r, err := cliente.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Body.Close() }()
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("la dirección contestó %d", r.StatusCode)
	}
	if tipo := r.Header.Get("Content-Type"); tipo != "video/mp2t" {
		return nil, fmt.Errorf("la señal viene declarada como %q", tipo)
	}
	buf := make([]byte, cuanto)
	n, err := io.ReadFull(r.Body, buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// numerosDeVideo saca el número que lleva dentro cada paquete de video, para
// poder comprobar que dos clientes vieron lo mismo y sin agujeros.
func numerosDeVideo(t *testing.T, recibido []byte) []uint32 {
	t.Helper()
	var nums []uint32
	for i := 0; i+ts.PacketSize <= len(recibido); i += ts.PacketSize {
		p := recibido[i:]
		if pid := uint16(p[1]&0x1f)<<8 | uint16(p[2]); pid != pidVideoDePrueba {
			continue
		}
		n := binary.BigEndian.Uint32(p[4:8])
		if len(nums) > 0 && n != nums[len(nums)-1]+1 {
			t.Fatalf("al cliente le falta un trozo: después del paquete %d le llegó el %d", nums[len(nums)-1], n)
		}
		nums = append(nums, n)
	}
	if len(nums) == 0 {
		t.Fatal("el cliente no recibió ni un paquete de video")
	}
	return nums
}

// loQueVieronLosDos cuenta los paquetes que están en las dos listas. Las dos
// son tramos seguidos, así que basta con cruzar los extremos.
func loQueVieronLosDos(a, b []uint32) int {
	desde, hasta := a[0], a[len(a)-1]
	if b[0] > desde {
		desde = b[0]
	}
	if b[len(b)-1] < hasta {
		hasta = b[len(b)-1]
	}
	if hasta < desde {
		return 0
	}
	return int(hasta-desde) + 1
}

func esperaHasta(t *testing.T, plazo time.Duration, cumple func() bool, queja string) {
	t.Helper()
	limite := time.Now().Add(plazo)
	for time.Now().Before(limite) {
		if cumple() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(queja)
}

// ── el ffmpeg de mentira ──────────────────────────────────────────────

// tsDeMentira entrega un transport stream legible por la misma conexión por
// la que el encoder de verdad lo entrega.
type tsDeMentira struct {
	mu     sync.Mutex
	bytes  int
	err    error
	parado bool
	fin    chan struct{}
	conn   net.Conn
}

func arrancaTSDeMentira(t *testing.T, direccion string) *tsDeMentira {
	t.Helper()
	conn, err := net.Dial("tcp", direccion)
	if err != nil {
		t.Fatalf("no se pudo entregar la señal en %s: %v", direccion, err)
	}
	f := &tsDeMentira{fin: make(chan struct{}), conn: conn}
	go f.emitir()
	return f
}

// emitir escribe paquetes sin parar: las dos tablas cada pocos paquetes de
// video, y cada paquete de video con su número dentro.
func (f *tsDeMentira) emitir() {
	defer func() { _ = f.conn.Close() }()
	var ccVideo, ccPAT, ccPMT int
	var numero uint32
	// Una tanda de siete paquetes cada milisegundo es la tasa a la que sale
	// un canal de verdad, más o menos: ni tan lento que la prueba tarde ni
	// tan rápido que el reparto sea lo único que se mida.
	const porTanda = 14
	for {
		select {
		case <-f.fin:
			return
		default:
		}
		var tanda []byte
		for i := 0; i < porTanda; i++ {
			if numero%paquetesPorTabla == 0 {
				tanda = append(tanda, paqueteDeTabla(0, ccPAT, seccionPAT())...)
				tanda = append(tanda, paqueteDeTabla(pidPMTDePrueba, ccPMT, seccionPMT())...)
				ccPAT, ccPMT = (ccPAT+1)&0x0f, (ccPMT+1)&0x0f
			}
			tanda = append(tanda, paqueteDeVideo(ccVideo, numero)...)
			ccVideo = (ccVideo + 1) & 0x0f
			numero++
		}
		n, err := f.conn.Write(tanda)
		f.mu.Lock()
		f.bytes += n
		if err != nil && f.err == nil && !f.parado {
			f.err = err
		}
		parar := err != nil
		f.mu.Unlock()
		if parar {
			return
		}
		time.Sleep(time.Millisecond)
	}
}

func (f *tsDeMentira) escritos() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bytes
}

func (f *tsDeMentira) error() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

// parar deja de entregar. Se puede llamar dos veces.
func (f *tsDeMentira) parar() {
	f.mu.Lock()
	yaEstaba := f.parado
	f.parado = true
	f.mu.Unlock()
	if !yaEstaba {
		close(f.fin)
	}
}

// paqueteDeVideo es un paquete de 188 bytes con el número dentro, para saber
// cuál es cuál.
func paqueteDeVideo(cc int, numero uint32) []byte {
	carga := make([]byte, 4)
	binary.BigEndian.PutUint32(carga, numero)
	return paqueteTS(pidVideoDePrueba, cc, false, carga)
}

// paqueteDeTabla lleva una sección PSI: el byte del puntero y detrás la
// sección.
func paqueteDeTabla(pid uint16, cc int, seccion []byte) []byte {
	return paqueteTS(pid, cc, true, append([]byte{0x00}, seccion...))
}

// paqueteTS arma un paquete de transport stream con solo carga útil.
func paqueteTS(pid uint16, cc int, inicioDeTabla bool, carga []byte) []byte {
	p := make([]byte, ts.PacketSize)
	p[0] = 0x47
	p[1] = byte(pid >> 8 & 0x1f)
	if inicioDeTabla {
		p[1] |= 0x40
	}
	p[2] = byte(pid)
	p[3] = 0x10 | byte(cc&0x0f) // solo carga útil, sin campo de adaptación
	n := copy(p[4:], carga)
	for i := 4 + n; i < len(p); i++ {
		p[i] = 0xff // el relleno de una sección PSI
	}
	return p
}

// seccionPAT es la tabla de programas: un solo programa, como emite un
// playout (ISO/IEC 13818-1). Los cuatro bytes del final son la firma, que
// internal/ts no comprueba.
func seccionPAT() []byte {
	s := make([]byte, 16)
	s[0] = 0x00
	s[1], s[2] = 0xb0, byte(len(s)-3)
	s[3], s[4] = byte(tsidDePrueba>>8), byte(tsidDePrueba)
	s[5], s[6], s[7] = 0xc1, 0x00, 0x00
	s[8], s[9] = byte(programaDePrueba>>8), byte(programaDePrueba)
	s[10], s[11] = 0xe0|byte(pidPMTDePrueba>>8), byte(pidPMTDePrueba&0xff)
	return s
}

// seccionPMT es la tabla del programa: dónde va el reloj y dónde el video.
func seccionPMT() []byte {
	s := make([]byte, 21)
	s[0] = 0x02
	s[1], s[2] = 0xb0, byte(len(s)-3)
	s[3], s[4] = byte(programaDePrueba>>8), byte(programaDePrueba)
	s[5], s[6], s[7] = 0xc1, 0x00, 0x00
	s[8], s[9] = 0xe0|byte(pidVideoDePrueba>>8), byte(pidVideoDePrueba&0xff) // el PCR va con el video
	s[10], s[11] = 0xf0, 0x00
	s[12] = ts.TipoMPEG2Video
	s[13], s[14] = 0xe0|byte(pidVideoDePrueba>>8), byte(pidVideoDePrueba&0xff)
	s[15], s[16] = 0xf0, 0x00
	return s
}
