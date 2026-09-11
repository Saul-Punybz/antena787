package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"antena787/internal/app"
	"antena787/internal/model"
)

// PasoFinal es el último paso del asistente (PRD §13: nueve pasos, seis
// preguntas — los pasos 3, 8 y 9 no preguntan nada).
const PasoFinal = 9

// instalacionGet dice por dónde va el asistente y qué encontró la máquina
// sola: ffmpeg, el disco, la carpeta de datos.
func (s *Server) instalacionGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hasPIN, err := s.App.Store.Settings.HasPIN(ctx)
	if err != nil {
		failStore(w, err, "leer la clave de la estación")
		return
	}
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return
	}
	paso := 1
	if raw := s.setting(r, app.KeyInstallStep); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 1 {
			paso = n
		}
	}
	fillers, _ := s.App.Store.Filler.List(ctx, s.App.ChannelID)

	detectado := map[string]any{
		"ffmpeg":            s.App.FFmpeg,
		"ffprobe":           s.App.FFprobe,
		"carpeta_datos":     s.App.DataDir,
		"carpeta_contenido": s.setting(r, app.KeyContentFolder),
		"carpeta_respaldo":  s.App.BackupDir(ctx),
		"relleno":           len(fillers),
	}
	if s.App.FFmpegErr != nil {
		detectado["problema"] = s.App.FFmpegErr.Error()
	}
	// La aceleración por hardware se mide de verdad en F2 (PRD §14.1):
	// listar -hwaccels no basta porque los drivers mienten. Aquí se dice.
	detectado["aceleracion"] = "se mide al arrancar el motor (F2); todavía no hay motor"
	// El disco y la red se miden de verdad y se cuentan en una frase, no en
	// un número que haya que interpretar (docs/API.md).
	detectado["disco"] = s.App.DiscoEnCristiano()
	detectado["red"] = app.RedEnCristiano()

	writeJSON(w, http.StatusOK, map[string]any{
		"paso":                 paso,
		"pasos":                PasoFinal,
		"completa":             s.setting(r, app.KeyInstallDone) == "si",
		"necesita_instalacion": !hasPIN,
		"canal":                ch,
		"detectado":            detectado,
		"opciones":             app.OpcionesDelAsistente(),
		"respuestas":           s.respuestasDelAsistente(r, ch, hasPIN),
		"tiempos":              s.tiemposDelAsistente(r),
	})
}

// respuestasDelAsistente devuelve lo contestado hasta ahora, paso por paso,
// para que la pantalla pueda volver atrás y enseñarlo. La clave de estación
// no sale nunca de aquí: no está, ni cifrada ni en claro.
func (s *Server) respuestasDelAsistente(r *http.Request, ch model.Channel, hayClave bool) map[string]any {
	out := map[string]any{}
	// Un paso se enseña cuando quedó apuntado que se contestó, o cuando lo
	// que contestó está guardado. Lo segundo es por las instalaciones que
	// vienen de antes de que se apuntaran los instantes.
	guarda := func(n int, contestado bool, valores map[string]any) {
		if !contestado && s.setting(r, app.KeyInstallStepAt(n)) == "" {
			return
		}
		out[strconv.Itoa(n)] = valores
	}

	// El paso 1 se da por contestado cuando hay clave de estación: es lo que
	// ese paso pone, y el nombre del canal viene de fábrica.
	guarda(1, hayClave, map[string]any{
		"nombre":             ch.Name,
		"identificativo":     ch.CallSign,
		"comunidad_licencia": ch.LicenseCity,
		"nombre_operador":    s.setting(r, app.KeyOperator),
	})

	modo := s.setting(r, app.KeyPlannedMode)
	guarda(2, modo != "", map[string]any{"modo": modo})

	destino := s.setting(r, app.KeyOutputTarget)
	retorno := s.setting(r, app.KeyAirReturn)
	nota := s.setting(r, app.KeyOutputNote)
	guarda(4, destino != "" || retorno != "" || nota != "", map[string]any{
		"destino":         destino,
		"retorno_de_aire": retorno,
		"nota":            nota,
	})

	barras := s.setting(r, app.KeyBarsSeen)
	guarda(5, barras != "", map[string]any{"ve_barras": barras == "si"})

	pais, calidad := s.setting(r, app.KeyCountry), s.setting(r, app.KeyQuality)
	guarda(6, pais != "" || calidad != "", map[string]any{"pais": pais, "calidad": calidad})

	carpeta := s.setting(r, app.KeyContentFolder)
	guarda(7, carpeta != "", map[string]any{"carpeta": carpeta})

	propuesta := s.setting(r, app.KeyProposal)
	guarda(8, propuesta != "", map[string]any{"propuesta": propuesta})

	return out
}

// tiemposDelAsistente dice cuándo se contestó cada paso. Es la única forma
// de saber en qué paso se abandona una instalación sin poner telemetría en
// la máquina de nadie (F2-108, PRD §23).
func (s *Server) tiemposDelAsistente(r *http.Request) map[string]string {
	out := map[string]string{}
	for n := 1; n <= PasoFinal; n++ {
		if v := s.setting(r, app.KeyInstallStepAt(n)); v != "" {
			out[strconv.Itoa(n)] = v
		}
	}
	return out
}

// pasoBody son todas las respuestas posibles del asistente. Cada paso usa
// las suyas; las demás llegan vacías y no se tocan.
type pasoBody struct {
	// Paso 1
	Nombre         string `json:"nombre"`
	Identificativo string `json:"identificativo"`
	Comunidad      string `json:"comunidad_licencia"`
	Clave          string `json:"clave"`
	Operador       string `json:"nombre_operador"`
	// Paso 2
	Modo string `json:"modo"`
	// Paso 4
	Destino string `json:"destino"`
	Retorno string `json:"retorno_de_aire"`
	Nota    string `json:"nota"`
	// Paso 5
	VeBarras *bool `json:"ve_barras"`
	// Paso 6
	Pais    string `json:"pais"`
	Calidad string `json:"calidad"`
	// Paso 7
	Carpeta string `json:"carpeta"`
	// Paso 8
	Propuesta string `json:"propuesta"`
}

// instalacionPaso guarda la respuesta de un paso y adelanta el asistente.
func (s *Server) instalacionPaso(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 1 || n > PasoFinal {
		failf(w, http.StatusBadRequest, "n", "el asistente tiene %d pasos, del 1 al %d", PasoFinal, PasoFinal)
		return
	}
	var body pasoBody
	_ = decodeOptional(r, &body)

	out := map[string]any{"paso": n}

	switch n {
	case 1:
		if !s.paso1(w, r, body, out) {
			return
		}
	case 2:
		modo := strings.ToLower(strings.TrimSpace(body.Modo))
		switch modo {
		case "internet", "transmisor", "no_se", "no sé", "":
		default:
			fail(w, http.StatusBadRequest, "las respuestas son: internet, transmisor o no sé", "modo")
			return
		}
		s.set(r, app.KeyPlannedMode, modo)
		// Contestar el asistente no enciende nada: el canal sigue como está, y
		// para salir de sombra hay una puerta con sus comprobaciones (F2-118).
		out["modo_del_canal"] = s.modoDelCanal(r)
		out["aviso"] = "esto queda apuntado; el canal sigue en modo sombra hasta que lo saques desde Al aire, con el botón «Salir al aire»"

	case 3:
		// La revisión automática no pregunta nada: contesta lo que encontró.
		out["ffmpeg"] = s.App.FFmpeg
		out["ffprobe"] = s.App.FFprobe
		if s.App.FFmpegErr != nil {
			out["problema"] = s.App.FFmpegErr.Error()
		}

	case 4:
		if !s.paso4(w, r, body, out) {
			return
		}

	case 5:
		// El motor de barras es F2; aquí solo se apunta la respuesta, sin
		// fingir que la prueba se hizo.
		visto := body.VeBarras != nil && *body.VeBarras
		s.set(r, app.KeyBarsSeen, boolText(visto))
		out["aviso"] = "la prueba de barras necesita el motor de emisión, que llega en F2. Tu respuesta queda apuntada."

	case 6:
		if !s.paso6(w, r, body, out) {
			return
		}

	case 7:
		if !s.paso7(w, r, body, out) {
			return
		}

	case 8:
		if !s.paso8(w, r, body, out) {
			return
		}

	case PasoFinal:
		s.set(r, app.KeyInstallDone, "si")
		out["completa"] = true
		out["modo_del_canal"] = s.modoDelCanal(r)
		out["aviso"] = "listo. El canal queda en modo sombra: resuelve el plan y publica la guía, pero todavía no emite. Cuando quieras emitir de verdad, en Al aire está el botón «Salir al aire»: te dice qué falta antes de encender nada."
	}

	siguiente := n + 1
	if n >= PasoFinal {
		siguiente = PasoFinal
	}
	s.set(r, app.KeyInstallStep, strconv.Itoa(siguiente))
	// Cuándo se contestó este paso. Sirve para acompañar una instalación por
	// teléfono y para saber dónde se abandona (F2-108).
	s.set(r, app.KeyInstallStepAt(n), s.App.Now().Format(time.RFC3339))
	out["siguiente"] = siguiente
	writeJSON(w, http.StatusOK, out)
}

// paso1: cómo se llama el canal, quién entra aquí, y la clave de estación.
func (s *Server) paso1(w http.ResponseWriter, r *http.Request, body pasoBody, out map[string]any) bool {
	ctx := r.Context()
	if strings.TrimSpace(body.Nombre) == "" {
		fail(w, http.StatusBadRequest, "el canal tiene que llamarse de alguna forma", "nombre")
		return false
	}
	clave := strings.TrimSpace(body.Clave)
	yaHabia, err := s.App.Store.Settings.HasPIN(ctx)
	if err != nil {
		fail(w, http.StatusInternalServerError, "no se pudo leer la clave de la estación: "+err.Error(), "")
		return false
	}
	// Al volver al paso 1 con la clave ya puesta, dejarla en blanco es
	// «conservar la que hay»; no se obliga a escribirla otra vez.
	conservarClave := clave == "" && yaHabia
	if !conservarClave && (len(clave) < 4 || len(clave) > 6 || !soloDigitos(clave)) {
		fail(w, http.StatusBadRequest, "la clave de la estación son de cuatro a seis dígitos", "clave")
		return false
	}

	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return false
	}
	old := ch
	ch.Name = strings.TrimSpace(body.Nombre)
	if v := strings.TrimSpace(body.Identificativo); v != "" {
		ch.CallSign = v
	}
	if v := strings.TrimSpace(body.Comunidad); v != "" {
		ch.LicenseCity = v
	}
	if err := s.App.Store.Channel.Update(ctx, ch); err != nil {
		failStore(w, err, "guardar el canal")
		return false
	}
	if !conservarClave {
		if err := s.App.Store.Settings.SetPIN(ctx, clave); err != nil {
			failf(w, http.StatusBadRequest, "clave", "%s", err)
			return false
		}
	}
	if v := strings.TrimSpace(body.Operador); v != "" {
		s.set(r, app.KeyOperator, v)
	}
	s.auditDiff(r, "channel", &ch.ID, old, ch)
	if !conservarClave {
		s.audit(r, "settings", nil, "clave_estacion", "", "puesta")
	}

	// La clave acaba de nacer: se deja la sesión abierta para que el
	// asistente pueda seguir sin volver a pedirla.
	if err := s.setCookie(w, ctx, s.operator(ctx)); err != nil {
		failf(w, http.StatusInternalServerError, "", "no se pudo abrir la sesión: %s", err)
		return false
	}
	out["canal"] = ch
	out["cartel"] = ch.CallSign + " · " + ch.LicenseCity
	return true
}

// paso4: a dónde va la señal y si se puede ver de vuelta.
//
// Nunca se pide elegir por nombre técnico (PRD §4, principio 1; F2-105): lo
// que llega es uno de los valores del catálogo que el propio asistente
// entregó en GET /instalacion, y «todavía no» es uno de ellos.
func (s *Server) paso4(w http.ResponseWriter, r *http.Request, body pasoBody, out map[string]any) bool {
	destino := strings.TrimSpace(body.Destino)
	if destino != "" && !app.OpcionValida(app.OpcionesDeDestino, destino) {
		failf(w, http.StatusBadRequest, "destino",
			"esa no es una de las respuestas: %s", app.TextosDeOpciones(app.OpcionesDeDestino))
		return false
	}
	retorno := strings.TrimSpace(body.Retorno)
	if retorno != "" && !app.OpcionValida(app.OpcionesDeRetorno, retorno) {
		failf(w, http.StatusBadRequest, "retorno_de_aire",
			"esa no es una de las respuestas: %s", app.TextosDeOpciones(app.OpcionesDeRetorno))
		return false
	}

	s.set(r, app.KeyOutputTarget, strings.ToLower(destino))
	s.set(r, app.KeyAirReturn, strings.ToLower(retorno))
	s.set(r, app.KeyOutputNote, strings.TrimSpace(body.Nota))

	// Sin retorno de aire se puede seguir. Se dice, no se esconde (PRD §13).
	if retorno == "" || strings.EqualFold(retorno, "ninguno") {
		out["aviso"] = "sin retorno de aire no podemos comparar lo que sale con lo que emites: se puede seguir, y se dice."
	}
	return true
}

// paso8: la primera parrilla. Con «automatica» el asistente propone una con
// lo que haya en la biblioteca; con «ninguna» solo arma el plan con lo que ya
// exista, que es lo que hacía antes de que la propuesta existiera.
func (s *Server) paso8(w http.ResponseWriter, r *http.Request, body pasoBody, out map[string]any) bool {
	ctx := r.Context()
	quiere := strings.ToLower(strings.TrimSpace(body.Propuesta))
	switch quiere {
	case "":
		quiere = "ninguna"
	case "automatica", "ninguna":
	default:
		fail(w, http.StatusBadRequest,
			"las respuestas son: que te la proponga yo, o dejarla para después", "propuesta")
		return false
	}
	s.set(r, app.KeyProposal, quiere)

	if quiere == "ninguna" {
		res, err := s.App.Resolve(ctx)
		if err != nil {
			failStore(w, err, "armar la primera parrilla")
			return false
		}
		out["reglas_creadas"] = 0
		out["bloques"] = len(res.Items)
		out["avisos"] = res.Warnings
		out["titulos_sin_material"] = 0
		out["aviso"] = "queda para después: en Reglas puedes armar la parrilla cuando quieras, o pegar tu hoja de programación."
		return true
	}

	p, err := s.App.ProponerParrilla(ctx)
	if err != nil {
		failStore(w, err, "armar la primera parrilla")
		return false
	}
	out["reglas_creadas"] = p.ReglasCreadas
	out["bloques"] = p.Bloques
	out["avisos"] = p.Avisos
	out["titulos_sin_material"] = p.SinMaterial
	switch {
	case p.YaHabiaReglas:
		out["aviso"] = "ya tenías la parrilla puesta, así que no toqué nada: armé el plan con tus reglas."
	case p.ReglasCreadas == 0:
		out["aviso"] = "todavía no hay material listo con qué armarla: deja tus archivos en la carpeta de contenido, o pega tu hoja de programación en Reglas."
	default:
		out["aviso"] = "esto es una propuesta: en Reglas puedes mover cada programa de hora, quitarlo, o pegar tu hoja y quedarte con la tuya."
	}
	return true
}

// rellenoPorDefecto genera el cartel de la estación con una cama musical
// cuando la biblioteca de relleno está vacía (PRD §13, F2-106): un hueco sin
// relleno sale al cartel, y eso no se descubre a las tres de la mañana.
//
// Nunca son barras y tono: las barras son de la prueba del paso 5 y no del
// respaldo del aire (F2-69).
func (s *Server) rellenoPorDefecto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if s.App.FFmpeg == "" {
		motivo := "no encuentro las herramientas de video en esta máquina"
		if s.App.FFmpegErr != nil {
			motivo = s.App.FFmpegErr.Error()
		}
		fail(w, http.StatusBadRequest, "no puedo preparar el cartel: "+motivo, "")
		return
	}

	ruta, err := s.App.ReservarRellenoPorDefecto(ctx)
	switch {
	case errors.Is(err, app.ErrSinCarpetaDeContenido):
		fail(w, http.StatusBadRequest, "primero dime en qué carpeta está tu contenido y ahí mismo dejo el cartel", "carpeta")
		return
	case errors.Is(err, os.ErrExist):
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":   "el cartel de la estación ya está hecho: lo tienes en Biblioteca, dentro del relleno",
			"archivo": ruta,
		})
		return
	case err != nil:
		failStore(w, err, "preparar el cartel de la estación")
		return
	}
	s.audit(r, "settings", nil, app.KeyDefaultFiller, "", ruta)

	// El trabajo de verdad tarda: se hace aparte y se avisa por el bus
	// cuando termina. La petición no se queda esperando a ffmpeg.
	go func() {
		_, _ = s.App.CrearRellenoPorDefecto(s.App.Context())
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"archivo": ruta,
		"aviso":   app.AvisoDelRelleno,
	})
}

// paso6: país y calidad. El país enciende el perfil regulatorio; la calidad
// fija el formato de casa.
func (s *Server) paso6(w http.ResponseWriter, r *http.Request, body pasoBody, out map[string]any) bool {
	ctx := r.Context()
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return false
	}
	calidad := strings.TrimSpace(body.Calidad)
	if calidad != "" && !app.OpcionValida(app.OpcionesDeCalidad, calidad) {
		failf(w, http.StatusBadRequest, "calidad",
			"esa calidad no está en la lista; las que hay son: %s", app.TextosDeOpciones(app.OpcionesDeCalidad))
		return false
	}
	old := ch
	if v := strings.TrimSpace(body.Pais); v != "" {
		s.set(r, app.KeyCountry, v)
		ch.RegProfile = perfilDePais(v)
	}
	if calidad != "" {
		s.set(r, app.KeyQuality, calidad)
		ch.FormatProfile = calidad
	}
	if err := s.App.Store.Channel.Update(ctx, ch); err != nil {
		failStore(w, err, "guardar el canal")
		return false
	}
	s.auditDiff(r, "channel", &ch.ID, old, ch)
	f := app.FormatOf(ch.FormatProfile)
	out["canal"] = ch
	out["formato"] = f.SizeString() + " a " + f.FPSString()
	return true
}

// perfilDePais traduce el país al perfil regulatorio. Lo que no se reconoce
// va al perfil abierto, que no exige nada: el software nunca regaña.
func perfilDePais(pais string) string {
	switch strings.ToLower(strings.TrimSpace(pais)) {
	case "pr", "puerto rico", "us", "usa", "eeuu", "estados unidos":
		return "us-fcc"
	case "":
		return ""
	default:
		return "abierto"
	}
}

// paso7: la carpeta de contenido. Se crea, se empieza a vigilar, y si la
// biblioteca de relleno está vacía se avisa (auditoría B12): un hueco sin
// relleno sale al cartel, y eso no se descubre a las 3 de la mañana.
func (s *Server) paso7(w http.ResponseWriter, r *http.Request, body pasoBody, out map[string]any) bool {
	ctx := r.Context()
	dir := strings.TrimSpace(body.Carpeta)
	if dir == "" {
		fail(w, http.StatusBadRequest, "dime en qué carpeta está tu contenido", "carpeta")
		return false
	}
	if !filepath.IsAbs(dir) {
		abs, err := filepath.Abs(dir)
		if err == nil {
			dir = abs
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		failf(w, http.StatusBadRequest, "carpeta", "no se puede usar la carpeta %q: %s", dir, err)
		return false
	}
	before := s.setting(r, app.KeyContentFolder)
	if err := s.App.Store.Settings.Set(ctx, app.KeyContentFolder, dir); err != nil {
		failStore(w, err, "guardar la carpeta de contenido")
		return false
	}
	s.audit(r, "settings", nil, app.KeyContentFolder, before, dir)
	out["carpeta"] = dir
	out["aviso_vigilancia"] = "ya la estoy mirando: lo que dejes ahí entra solo en cuanto termine de copiarse"

	fillers, _ := s.App.Store.Filler.List(ctx, s.App.ChannelID)
	if len(fillers) == 0 {
		out["aviso_relleno"] = "la biblioteca de relleno está vacía: el primer hueco que aparezca sale al cartel de la estación. Añade una cortinilla o una cama musical antes de salir al aire."
	}
	return true
}

func (s *Server) set(r *http.Request, key, value string) {
	before, _ := s.App.Store.Settings.Get(r.Context(), key)
	if err := s.App.Store.Settings.Set(r.Context(), key, value); err != nil {
		return
	}
	if before != value {
		s.audit(r, "settings", nil, key, before, value)
	}
}

func soloDigitos(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

func boolText(b bool) string {
	if b {
		return "si"
	}
	return "no"
}

// modoDelCanal es el modo que tiene el canal ahora mismo. El asistente lo
// dice, no lo decide: contestar un paso nunca enciende ni apaga el aire
// (F2-118).
func (s *Server) modoDelCanal(r *http.Request) string {
	ch, err := s.App.Store.Channel.Get(r.Context(), s.App.ChannelID)
	if err != nil {
		return app.ModoSombra
	}
	return ch.Mode
}
