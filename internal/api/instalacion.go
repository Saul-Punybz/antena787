package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"antena787/internal/app"
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

	writeJSON(w, http.StatusOK, map[string]any{
		"paso":                 paso,
		"pasos":                PasoFinal,
		"completa":             s.setting(r, app.KeyInstallDone) == "si",
		"necesita_instalacion": !hasPIN,
		"canal":                ch,
		"detectado":            detectado,
	})
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
	// Paso 5
	VeBarras *bool `json:"ve_barras"`
	// Paso 6
	Pais    string `json:"pais"`
	Calidad string `json:"calidad"`
	// Paso 7
	Carpeta string `json:"carpeta"`
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

	ctx := r.Context()
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
		// En F1 no hay motor: el canal se queda en sombra y el estado lo dice.
		out["modo_del_canal"] = "sombra"
		out["aviso"] = "esto queda apuntado; el canal sigue en modo sombra hasta que exista el motor de emisión"

	case 3:
		// La revisión automática no pregunta nada: contesta lo que encontró.
		out["ffmpeg"] = s.App.FFmpeg
		out["ffprobe"] = s.App.FFprobe
		if s.App.FFmpegErr != nil {
			out["problema"] = s.App.FFmpegErr.Error()
		}

	case 4:
		s.set(r, app.KeyOutputTarget, body.Destino)
		s.set(r, app.KeyAirReturn, body.Retorno)
		if strings.TrimSpace(body.Retorno) == "" {
			out["aviso"] = "sin retorno de aire no podemos comparar lo que sale con lo que emites: se puede seguir, y se dice."
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
		res, err := s.App.Resolve(ctx)
		if err != nil {
			failStore(w, err, "armar la primera parrilla")
			return
		}
		out["bloques"] = len(res.Items)
		out["avisos"] = res.Warnings

	case PasoFinal:
		s.set(r, app.KeyInstallDone, "si")
		out["completa"] = true
		out["modo_del_canal"] = "sombra"
		out["aviso"] = "listo. El canal queda en modo sombra: resuelve el plan y publica la guía, pero todavía no emite — el motor es la fase siguiente."
	}

	siguiente := n + 1
	if n >= PasoFinal {
		siguiente = PasoFinal
	}
	s.set(r, app.KeyInstallStep, strconv.Itoa(siguiente))
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
	if len(clave) < 4 || len(clave) > 6 || !soloDigitos(clave) {
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
	if err := s.App.Store.Settings.SetPIN(ctx, clave); err != nil {
		failf(w, http.StatusBadRequest, "clave", "%s", err)
		return false
	}
	if v := strings.TrimSpace(body.Operador); v != "" {
		s.set(r, app.KeyOperator, v)
	}
	s.auditDiff(r, "channel", &ch.ID, old, ch)
	s.audit(r, "settings", nil, "clave_estacion", "", "puesta")

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

// paso6: país y calidad. El país enciende el perfil regulatorio; la calidad
// fija el formato de casa.
func (s *Server) paso6(w http.ResponseWriter, r *http.Request, body pasoBody, out map[string]any) bool {
	ctx := r.Context()
	ch, err := s.App.Store.Channel.Get(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "el canal")
		return false
	}
	old := ch
	if v := strings.TrimSpace(body.Pais); v != "" {
		s.set(r, app.KeyCountry, v)
		ch.RegProfile = perfilDePais(v)
	}
	if v := strings.TrimSpace(body.Calidad); v != "" {
		s.set(r, app.KeyQuality, v)
		ch.FormatProfile = v
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
