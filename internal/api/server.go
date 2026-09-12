// Package api es la interfaz de Antena787 hacia fuera: el contrato de
// docs/API.md, servido con net/http de la biblioteca estándar (patrones de
// ruta de Go 1.22) y sin ninguna dependencia externa.
//
// Tres reglas mandan sobre todo lo demás:
//
//   - **Los errores están en palabras claras.** Un 4xx trae
//     `{"error": "…", "campo": "…"}` y el texto es el que ve la persona. No
//     hay códigos crípticos.
//   - **Todo lo que cambia datos queda en la auditoría**, con `origen:
//     humano` y el autor de la sesión (PRD §13, docs/API.md).
//   - **Una sola clave de estación**, no usuarios ni permisos: cookie
//     `antena_sesion`, HttpOnly y SameSite=Strict, que se pide una vez por
//     navegador (PRD §19). Sin clave puesta todavía, lo único abierto es el
//     asistente de instalación.
package api

import (
	"io/fs"
	"net/http"
	"time"

	"antena787/internal/app"
)

// Options ajusta el servidor.
type Options struct {
	// UI es el sistema de archivos de la interfaz. Nil = no se sirven
	// estáticos (solo la API), que es lo que quieren las pruebas.
	UI fs.FS
	// Now es el reloj; nil = el de la aplicación.
	Now func() time.Time
	// SecureCookie marca la cookie de sesión como Secure. En loopback y en
	// Tailscale se sirve por http, así que el default es false.
	SecureCookie bool
}

// Server es la API montada. Implementa http.Handler.
type Server struct {
	App  *app.App
	opts Options

	mux      *http.ServeMux
	sessions *sessions
	attempts *attempts
}

// New monta todas las rutas del contrato.
func New(a *app.App, opts Options) *Server {
	s := &Server{
		App:      a,
		opts:     opts,
		mux:      http.NewServeMux(),
		sessions: newSessions(),
		attempts: newAttempts(),
	}
	s.routes()
	return s
}

// Now es el reloj del servidor.
func (s *Server) Now() time.Time {
	if s.opts.Now != nil {
		return s.opts.Now().UTC()
	}
	return s.App.Now()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// routes es el contrato entero, en un sitio, para poder compararlo con
// docs/API.md de un vistazo.
func (s *Server) routes() {
	api := func(pattern string, h http.HandlerFunc) {
		s.mux.Handle(pattern, s.guard(h))
	}
	open := func(pattern string, h http.HandlerFunc) { // sin clave, a propósito
		s.mux.Handle(pattern, h)
	}

	// Estado y canal
	open("GET /api/v1/estado", s.estado)
	api("GET /api/v1/canal", s.canalGet)
	api("PUT /api/v1/canal", s.canalPut)

	// La puerta del aire (F2-118). El modo del canal no se cambia guardando
	// un formulario: se comprueba, se confirma escribiéndolo, y queda dicho
	// quién lo hizo.
	api("GET /api/v1/canal/comprobaciones", s.canalComprobaciones)
	api("POST /api/v1/canal/al-aire", s.canalAlAire)
	api("POST /api/v1/canal/a-sombra", s.canalASombra)
	api("GET /api/v1/ajustes", s.ajustesGet)
	api("PUT /api/v1/ajustes", s.ajustesPut)

	// Entrar y salir
	open("POST /api/v1/entrar", s.entrar)
	open("POST /api/v1/salir", s.salir)

	// Salidas: a dónde manda el canal su señal (§10, F2-46, F2-50)
	api("GET /api/v1/salidas", s.salidasList)
	api("POST /api/v1/salidas", s.salidasPost)
	api("PUT /api/v1/salidas/{id}", s.salidasPut)
	api("DELETE /api/v1/salidas/{id}", s.salidasDelete)

	// Los presets de preparación: cómo quiere el dueño del canal que suene y
	// se vea su material. Se aplican en tres niveles —canal, título, archivo—
	// y gana el más específico (esquema v10).
	api("GET /api/v1/presets", s.presetsList)
	api("POST /api/v1/presets", s.presetsPost)
	api("PUT /api/v1/presets/{id}", s.presetsPut)
	api("DELETE /api/v1/presets/{id}", s.presetsDelete)
	// Volver a preparar: cambiar un preset no rehace solo lo que ya estaba
	// convertido, así que hace falta poder pedirlo. Se apoya en la cola, que
	// ordena por hora de aire y se aparta cuando el aire sufre.
	api("POST /api/v1/material/{id}/volver-a-preparar", s.volverAPreparar)

	// Las señales en vivo: de dónde sale lo que no es un archivo (F2-116).
	// La tabla y el repositorio existían desde el principio y no había una
	// sola ruta que los alcanzara: se podía programar un vivo que no se podía
	// crear.
	api("GET /api/v1/fuentes", s.fuentesList)
	api("POST /api/v1/fuentes", s.fuentesPost)
	api("PUT /api/v1/fuentes/{id}", s.fuentesPut)
	api("DELETE /api/v1/fuentes/{id}", s.fuentesDelete)
	// Probar antes de guardar. De todo lo que se miró, **nadie lo hace**: lo
	// más cercano es la vista previa de MistServer, y es DESPUÉS de guardar
	// (docs/investigacion/ENTRADAS-POR-URL-COMPARADAS-2026-09-11.md). Para
	// quien es su propio departamento de IT, es la diferencia entre pegar una
	// dirección y rezar, o saberlo en tres segundos.
	api("POST /api/v1/fuentes/probar", s.fuentesProbar)

	// El control manual del aire (PRD §9 paso 6, T5). Hasta hoy el botón de
	// «Tomar el control» estaba en la pantalla y no llamaba a nada, y la
	// tabla `manual_hold` no la tocaba una línea de código.
	api("GET /api/v1/manual", s.manualGet)
	api("POST /api/v1/manual/tomar", s.manualTomar)
	// Quitárselo a otra persona es una puerta aparte a propósito: no puede
	// ser lo que pasa al pulsar el mismo botón dos veces (F2-77).
	api("POST /api/v1/manual/quitar", s.manualQuitar)
	api("POST /api/v1/manual/soltar", s.manualSoltar)
	// Parar todo corta en seco; soltar espera a que acabe lo que suena. Son
	// dos cosas distintas y por eso son dos rutas (F2-78 frente a F2-31).
	api("POST /api/v1/manual/parar", s.manualParar)
	api("POST /api/v1/manual/disparar", s.manualDisparar)

	// Reglas y plan
	api("GET /api/v1/reglas", s.reglasList)
	api("POST /api/v1/reglas", s.reglasPost)
	api("PUT /api/v1/reglas/{id}", s.reglasPut)
	api("DELETE /api/v1/reglas/{id}", s.reglasDelete)
	api("GET /api/v1/plan", s.planDia)
	api("PUT /api/v1/plan/{id}", s.planPut)
	api("GET /api/v1/plan/semana", s.planSemana)
	api("GET /api/v1/plan/mes", s.planMes)
	api("POST /api/v1/plan/recalcular", s.planRecalcular)
	api("POST /api/v1/plan/llenar-con-diferido", s.planDiferido)

	// La guía: el XMLTV se sirve sin clave, que es quien lo lee (el
	// transmisor, MistServer) no tiene navegador donde poner una.
	open("GET /guia.xml", s.guiaXML)
	open("GET /api/v1/guia.xml", s.guiaXML)
	// PMCP (ATSC A/76) es la guía que consume el generador PSIP de la
	// estación, no un navegador: sin clave, igual que guia.xml.
	open("GET /guia.pmcp", s.guiaPMCP)
	open("GET /api/v1/guia.pmcp", s.guiaPMCP)
	api("GET /api/v1/guia", s.guiaContraPlan)

	// Biblioteca
	api("GET /api/v1/biblioteca", s.bibliotecaList)
	api("GET /api/v1/biblioteca/{id}", s.bibliotecaGet)
	api("PUT /api/v1/biblioteca/{id}", s.bibliotecaPut)
	api("GET /api/v1/material", s.materialList)
	api("POST /api/v1/material/subir", s.materialSubir)
	api("GET /api/v1/material/{id}", s.materialGet)
	api("PUT /api/v1/material/{id}", s.materialPut)
	api("GET /api/v1/cuarentena", s.cuarentenaList)
	api("POST /api/v1/cuarentena/{id}/dejar-pasar", s.dejarPasar)
	api("GET /api/v1/relleno", s.rellenoList)

	// Importar
	api("POST /api/v1/importar/hoja", s.importarHoja)
	api("POST /api/v1/importar/confirmar-relevos", s.confirmarRelevos)

	// Emparejar títulos (F1-64 a F1-67)
	api("GET /api/v1/titulos/sin-emparejar", s.titulosSinEmparejar)
	api("GET /api/v1/titulos/buscar", s.titulosBuscar)
	api("POST /api/v1/titulos/{id}/emparejar", s.titulosEmparejar)

	// Lo que el sistema hizo solo
	api("GET /api/v1/incidentes", s.incidentes)
	api("GET /api/v1/auditoria", s.auditoria)
	api("GET /api/v1/auditoria/verificar", s.auditoriaVerificar)

	// El asistente: abierto mientras no haya clave puesta (guard lo sabe).
	s.mux.Handle("GET /api/v1/instalacion", s.guard(s.instalacionGet))
	s.mux.Handle("POST /api/v1/instalacion/paso/{n}", s.guard(s.instalacionPaso))
	s.mux.Handle("POST /api/v1/instalacion/relleno-por-defecto", s.guard(s.rellenoPorDefecto))

	// El estado en vivo
	s.mux.Handle("/api/v1/ws", s.guard(s.websocket))
	s.mux.Handle("/ws", s.guard(s.websocket))

	// La interfaz
	if s.opts.UI != nil {
		s.mux.Handle("/", s.static())
	} else {
		s.mux.HandleFunc("/", s.noEncontrado)
	}
}

func (s *Server) noEncontrado(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		writeJSON(w, http.StatusOK, map[string]any{
			"antena787": s.App.Version,
			"api":       "/api/v1/estado",
		})
		return
	}
	fail(w, http.StatusNotFound, "eso no está aquí", "")
}
