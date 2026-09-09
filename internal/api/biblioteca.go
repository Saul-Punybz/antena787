package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"antena787/internal/app"
	"antena787/internal/ingest"
	"antena787/internal/model"
	"antena787/internal/store"
)

// SubidaMaxBytes es lo más grande que se acepta por POST /material/subir:
// cuatro gigas, que es una película larga en un máster decente. El portal
// del anunciante tiene su propio límite, mucho más pequeño (ingest.UploadMaxBytes).
const SubidaMaxBytes int64 = 4 << 30

// Los tres estados del material tal como los lee una persona (docs/API.md).
const (
	EstadoListo      = "listo"
	EstadoNoListo    = "aún no listo para aire"
	EstadoCuarentena = "cuarentena"
)

// audioOut son los campos de sonido que Biblioteca pinta de cada archivo
// (web/src/lib/tipos.ts, `AudioDelMaterial`): cuál es el archivo, qué pistas
// trae, cuál sale al aire y de dónde salieron el sonido y los subtítulos que
// vinieron al lado (F1-58 a F1-62). Todos son opcionales: un título sin
// material no manda ninguno y la pantalla pinta igual.
type audioOut struct {
	MaterialID        *int64             `json:"material_id,omitempty"`
	PistasAudio       []model.PistaAudio `json:"pistas_audio,omitempty"`
	PistaAudioAire    *int               `json:"pista_audio_aire,omitempty"`
	AudioSidecar      string             `json:"audio_sidecar,omitempty"`
	SubtitulosSidecar string             `json:"subtitulos_sidecar,omitempty"`
}

// audioDe lee el sonido de un archivo. Sin archivo —o si ya no está— no
// manda nada.
func (s *Server) audioDe(ctx context.Context, assetID *int64) audioOut {
	if assetID == nil {
		return audioOut{}
	}
	a, err := s.App.Store.Media.Get(ctx, *assetID)
	if err != nil {
		return audioOut{}
	}
	id, pista := a.ID, a.PistaAudioAire
	return audioOut{
		MaterialID:        &id,
		PistasAudio:       a.PistasAudio,
		PistaAudioAire:    &pista,
		AudioSidecar:      a.AudioSidecar,
		SubtitulosSidecar: a.SubtitulosSidecar,
	}
}

// motivoCodigo traduce el motivo en cristiano de un archivo parado a su
// código. La base guarda el texto y no el código, así que se reconoce por el
// texto: hoy el único que hace falta es el del material sin sonido, que no
// tiene botón de dejarlo pasar (F1-59).
func motivoCodigo(motivo string) string {
	if ingest.TextoSinAudio(motivo) {
		return ingest.MotivoSinAudio
	}
	return ""
}

// titleOut es un título con lo que hace falta para pintarlo en Biblioteca:
// cuántos episodios tiene, en qué estado está su material, cuánto dura, y si
// hay alguna regla que lo esté programando (web/src/lib/tipos.ts,
// `TituloDeBiblioteca`).
type titleOut struct {
	model.Title
	Episodes     int    `json:"episodios"`
	Estado       string `json:"estado_material"`
	DurMs        int64  `json:"duracion_ms"`
	EnLaParrilla bool   `json:"en_la_parrilla"`
	// Hora es la hora de pared a la que sale, "HH:MM", y ReglaHasta el día
	// en que se acaba la regla que lo pone. Nulos si no está en la parrilla.
	Hora       *string    `json:"hora"`
	ReglaHasta *model.Day `json:"regla_hasta"`

	// Los dos nombres de antes, que docs/API.md prometía y alguna
	// herramienta de fuera puede estar leyendo. Dicen lo mismo que
	// estado_material y duracion_ms, en texto.
	Material string `json:"material"`
	Duration string `json:"duracion,omitempty"`

	// El sonido del archivo del título, si tiene uno propio.
	audioOut
}

// episodeOut es un episodio con el estado de su material, que es lo que la
// ficha del título pinta al lado de cada uno.
type episodeOut struct {
	model.Episode
	DurMs  int64  `json:"duracion_ms"`
	Estado string `json:"estado_material"`

	// El sonido del archivo de ese episodio.
	audioOut
}

func (s *Server) bibliotecaList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	titles, err := s.App.Store.Title.List(ctx)
	if err != nil {
		failStore(w, err, "leer la biblioteca")
		return
	}
	reglas := s.reglasPorTitulo(ctx)
	out := make([]titleOut, 0, len(titles))
	for _, t := range titles {
		out = append(out, s.titleOut(ctx, t, reglas))
	}
	writeJSON(w, http.StatusOK, out)
}

// reglasPorTitulo devuelve, por título, la regla activa que lo programa más
// temprano. Es lo que llena `en_la_parrilla`, `hora` y `regla_hasta`.
func (s *Server) reglasPorTitulo(ctx context.Context) map[int64]model.ScheduleRule {
	out := map[int64]model.ScheduleRule{}
	rules, err := s.App.Store.Rule.ListAll(ctx, s.App.ChannelID)
	if err != nil {
		return out
	}
	for _, r := range rules {
		if !r.Active || r.TitleID == nil {
			continue
		}
		anterior, hay := out[*r.TitleID]
		if !hay || r.At < anterior.At {
			out[*r.TitleID] = r
		}
	}
	return out
}

// estadoDelMaterial resume en una palabra en qué estado está el material de
// un conjunto de archivos, y cuánto suman.
func (s *Server) estadoDelMaterial(ctx context.Context, assets []int64) (estado string, ms int64) {
	estado = EstadoNoListo
	cuarentena := false
	listo := false
	for _, id := range assets {
		a, err := s.App.Store.Media.Get(ctx, id)
		if err != nil {
			continue
		}
		ms += a.DurationMs
		if a.State == model.AssetQuarantine {
			cuarentena = true
		}
		if a.Ready() {
			listo = true
		}
	}
	switch {
	case listo:
		estado = EstadoListo
	case cuarentena:
		estado = EstadoCuarentena
	}
	return estado, ms
}

func (s *Server) titleOut(ctx context.Context, t model.Title, reglas map[int64]model.ScheduleRule) titleOut {
	eps, _ := s.App.Store.Episode.ListByTitle(ctx, t.ID)
	out := titleOut{Title: t, Episodes: len(eps), audioOut: s.audioDe(ctx, t.MediaAssetID)}

	assets := []int64{}
	if t.MediaAssetID != nil {
		assets = append(assets, *t.MediaAssetID)
	}
	for _, e := range eps {
		if e.MediaAssetID != nil {
			assets = append(assets, *e.MediaAssetID)
		}
	}
	estado, ms := s.estadoDelMaterial(ctx, assets)
	out.Estado, out.Material = estado, estado
	out.DurMs = ms
	if ms > 0 {
		out.Duration = humanMs(ms)
	}
	if rule, hay := reglas[t.ID]; hay {
		hora := rule.At.String()
		hasta := rule.To
		out.EnLaParrilla = true
		out.Hora = &hora
		out.ReglaHasta = &hasta
	}
	return out
}

func (s *Server) bibliotecaGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	t, err := s.App.Store.Title.Get(ctx, id)
	if err != nil {
		failStore(w, err, "ese título")
		return
	}
	eps, _ := s.App.Store.Episode.ListByTitle(ctx, id)
	lista := make([]episodeOut, 0, len(eps))
	for _, e := range eps {
		assets := []int64{}
		if e.MediaAssetID != nil {
			assets = append(assets, *e.MediaAssetID)
		}
		estado, ms := s.estadoDelMaterial(ctx, assets)
		lista = append(lista, episodeOut{
			Episode: e, DurMs: ms, Estado: estado,
			audioOut: s.audioDe(ctx, e.MediaAssetID),
		})
	}
	// La ficha es el título entero, con la lista de episodios dentro
	// (web/src/lib/tipos.ts, `FichaDeTitulo`).
	ficha := map[string]any{"lista_de_episodios": lista}
	raw, err := jsonMarshal(s.titleOut(ctx, t, s.reglasPorTitulo(ctx)))
	if err != nil {
		failStore(w, err, "leer ese título")
		return
	}
	campos := map[string]any{}
	if err := jsonUnmarshal(raw, &campos); err != nil {
		failStore(w, err, "leer ese título")
		return
	}
	for k, v := range campos {
		ficha[k] = v
	}
	writeJSON(w, http.StatusOK, ficha)
}

func (s *Server) bibliotecaPut(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	old, err := s.App.Store.Title.Get(ctx, id)
	if err != nil {
		failStore(w, err, "ese título")
		return
	}
	nuevo := old
	if !decode(w, r, &nuevo) {
		return
	}
	nuevo.ID = old.ID
	if strings.TrimSpace(nuevo.Name) == "" {
		fail(w, http.StatusBadRequest, "el título tiene que llamarse de alguna forma", "nombre")
		return
	}
	if err := s.App.Store.Title.Update(ctx, &nuevo); err != nil {
		failStore(w, err, "guardar el título")
		return
	}
	s.auditDiff(r, "title", &nuevo.ID, old, nuevo)
	writeJSON(w, http.StatusOK, s.titleOut(ctx, nuevo, s.reglasPorTitulo(ctx)))
}

// ── el material ───────────────────────────────────────────────────────

func (s *Server) materialList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	estado := strings.TrimSpace(r.URL.Query().Get("estado"))
	var (
		list []model.MediaAsset
		err  error
	)
	if estado != "" {
		list, err = s.App.Store.Media.List(ctx, model.AssetState(estado))
	} else {
		list, err = s.allMedia(ctx)
	}
	if err != nil {
		failStore(w, err, "leer el material")
		return
	}
	if list == nil {
		list = []model.MediaAsset{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) allMedia(ctx context.Context) ([]model.MediaAsset, error) {
	var out []model.MediaAsset
	for _, st := range []model.AssetState{model.AssetIngesting, model.AssetReady, model.AssetQuarantine, model.AssetFailed} {
		part, err := s.App.Store.Media.List(ctx, st)
		if err != nil {
			return nil, err
		}
		out = append(out, part...)
	}
	return out, nil
}

func (s *Server) materialGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	a, err := s.App.Store.Media.Get(r.Context(), id)
	if err != nil {
		failStore(w, err, "ese archivo")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// materialPut es donde una persona confirma lo que la máquina solo pudo
// sospechar: que ese negro es a propósito, que ese spot va sin logo, dónde
// están los subtítulos, en qué milisegundos van los cortes y cuál de las
// pistas de sonido es la que sale al aire (F1-61).
func (s *Server) materialPut(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	old, err := s.App.Store.Media.Get(ctx, id)
	if err != nil {
		failStore(w, err, "ese archivo")
		return
	}
	var body struct {
		IntentionalBlack *bool    `json:"negro_intencional"`
		NoLogo           *bool    `json:"sin_logo"`
		ExternalCaptions *string  `json:"subtitulos_externos"`
		BreakMarks       *[]int64 `json:"marcas_de_corte_ms"`
		PistaAudioAire   *int     `json:"pista_audio_aire"`
	}
	if !decode(w, r, &body) {
		return
	}

	// La pista de sonido se cambia aparte: el store la valida, deja el
	// archivo pendiente de normalizar y lo anota en la bitácora, todo en la
	// misma escritura (F1-61).
	if body.PistaAudioAire != nil {
		err := s.App.Store.Media.SetPistaAudioAire(ctx, id, *body.PistaAudioAire)
		switch {
		case errors.Is(err, store.ErrPistaInexistente):
			fail(w, http.StatusBadRequest,
				store.ErrPistaInexistente.Error()+": escoge una de las que trae", "pista_audio_aire")
			return
		case errors.Is(err, store.ErrNotFound):
			failStore(w, err, "ese archivo")
			return
		case err != nil:
			failStore(w, err, "cambiar la pista de sonido")
			return
		}
		// El cambio ya quedó en la bitácora: lo anota el store dentro de la
		// misma escritura que lo hizo.
		s.App.RequeueNormalize(ctx, id)
		// La fila cambió por debajo: lo que se guarda ahora sale de ella.
		if refrescado, err := s.App.Store.Media.Get(ctx, id); err == nil {
			old = refrescado
		}
	}

	nuevo := old
	if body.IntentionalBlack != nil {
		nuevo.IntentionalBlack = *body.IntentionalBlack
	}
	if body.NoLogo != nil {
		nuevo.NoLogo = *body.NoLogo
	}
	if body.ExternalCaptions != nil {
		v := *body.ExternalCaptions
		nuevo.ExternalCaptions = &v
	}
	if body.BreakMarks != nil {
		nuevo.BreakMarksMs = *body.BreakMarks
	}
	nuevo.UpdatedAt = s.Now()
	if err := s.App.Store.Media.Update(ctx, &nuevo); err != nil {
		failStore(w, err, "guardar el archivo")
		return
	}
	s.auditDiff(r, "media_asset", &nuevo.ID, old, nuevo)
	writeJSON(w, http.StatusOK, s.materialOut(ctx, nuevo))
}

// materialOut es el archivo tal como lo contesta PUT /material/{id}: todo lo
// medido, como siempre, más los dos campos con los que Biblioteca lo pinta
// (web/src/lib/tipos.ts, `MaterialDeAudio`): cuál es el archivo y en qué
// estado quedó. Después de cambiarle la pista de sonido, «aún no listo para
// aire» hasta que la copia de casa se rehaga (F1-61).
func (s *Server) materialOut(ctx context.Context, a model.MediaAsset) any {
	raw, err := jsonMarshal(a)
	if err != nil {
		return a
	}
	campos := map[string]any{}
	if err := jsonUnmarshal(raw, &campos); err != nil {
		return a
	}
	campos["material_id"] = a.ID
	estado, _ := s.estadoDelMaterial(ctx, []int64{a.ID})
	campos["estado_material"] = estado
	return campos
}

// ── la cuarentena ─────────────────────────────────────────────────────

// enCuarentenaOut es un archivo parado tal como lo pinta la pantalla de
// cuarentena (web/src/lib/tipos.ts, `EnCuarentena`): el archivo entero más el
// código del motivo, que es lo que decide si se le puede pintar el botón de
// dejarlo pasar.
type enCuarentenaOut struct {
	model.MediaAsset
	// Titulo es cómo se llama el archivo para una persona: el título o el
	// episodio que lo usa; si nadie lo fichó, el nombre del archivo.
	Titulo       string `json:"titulo"`
	MotivoCodigo string `json:"motivo_codigo"`
}

func (s *Server) cuarentenaList(w http.ResponseWriter, r *http.Request) {
	list, err := s.App.Store.Media.List(r.Context(), model.AssetQuarantine)
	if err != nil {
		failStore(w, err, "leer la cuarentena")
		return
	}
	out := make([]enCuarentenaOut, 0, len(list))
	for _, a := range list {
		out = append(out, enCuarentenaOut{
			MediaAsset:   a,
			Titulo:       s.App.TituloDelArchivo(r.Context(), a),
			MotivoCodigo: motivoCodigo(a.PlainReason),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// dejarPasar es el botón "dejarlo pasar bajo mi responsabilidad" (auditoría
// E9). Deja el archivo listo, apunta quién lo autorizó y lo escribe en la
// bitácora: el sistema no discute, pero tampoco olvida.
func (s *Server) dejarPasar(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	var body struct {
		Quien string `json:"quien"`
	}
	_ = decodeOptional(r, &body)

	old, err := s.App.Store.Media.Get(ctx, id)
	if err != nil {
		failStore(w, err, "ese archivo")
		return
	}
	// Todo lo que sale al aire lleva sonido: un archivo mudo no se deja
	// pasar, se arregla poniéndole el audio al lado (F1-59).
	if motivoCodigo(old.PlainReason) == ingest.MotivoSinAudio {
		fail(w, http.StatusConflict,
			"Este archivo no trae sonido y todo lo que sale al aire lleva audio: pon a su lado un archivo de audio con el mismo nombre y se procesa solo.",
			"")
		return
	}

	quien := strings.TrimSpace(body.Quien)
	if quien == "" {
		quien = autor(r)
	}

	nuevo := old
	nuevo.State = model.AssetReady
	nuevo.LetThroughBy = quien
	// Un archivo que se deja pasar tiene que poder salir al aire: si la
	// normalización falló, se acepta el original tal cual.
	if nuevo.NormalizeState == ingest.NormalizeFailed || nuevo.NormalizeState == "" {
		nuevo.NormalizeState = ingest.NormalizeReady
	}
	nuevo.UpdatedAt = s.Now()
	if err := s.App.Store.Media.Update(ctx, &nuevo); err != nil {
		failStore(w, err, "dejar pasar el archivo")
		return
	}
	s.audit(r, "media_asset", &nuevo.ID, "dejado_pasar_por", old.PlainReason, quien)
	s.App.RefreshCuarentena(ctx)
	s.App.Recalc()
	writeJSON(w, http.StatusOK, nuevo)
}

// decodeOptional lee un cuerpo JSON que puede no venir.
func decodeOptional(r *http.Request, into any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil || len(strings.TrimSpace(string(body))) == 0 {
		return nil //nolint:nilerr // un cuerpo vacío es una respuesta válida
	}
	return jsonUnmarshal(body, into)
}

// ── el relleno ────────────────────────────────────────────────────────

// rellenoList enseña la biblioteca de relleno. Vacía es un aviso, no un
// error: significa que el primer hueco sale en negro (auditoría B12).
func (s *Server) rellenoList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	list, err := s.App.Store.Filler.List(ctx, s.App.ChannelID)
	if err != nil {
		failStore(w, err, "leer el relleno")
		return
	}
	if list == nil {
		list = []model.FillerAsset{}
	}
	out := map[string]any{"items": list}
	if len(list) == 0 {
		out["aviso"] = "la biblioteca de relleno está vacía: el primer hueco que aparezca sale al cartel de la estación. Añade al menos una cortinilla o una cama musical."
	}
	writeJSON(w, http.StatusOK, out)
}

// ── subir material ────────────────────────────────────────────────────

// materialSubir recibe un archivo por multipart y lo deja en la carpeta
// vigilada. No lo ingiere aquí: lo hace el vigilante de siempre, cuando el
// archivo termine de copiarse. Mientras se escribe lleva sufijo .part, que
// es de los que el vigilante deja en paz.
func (s *Server) materialSubir(w http.ResponseWriter, r *http.Request) {
	dir := s.setting(r, app.KeyContentFolder)
	if dir == "" {
		fail(w, http.StatusBadRequest,
			"todavía no hay carpeta de contenido: dila en el paso 7 del asistente o en Ajustes", "carpeta_contenido")
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		failf(w, http.StatusInternalServerError, "", "no se pudo usar la carpeta %q: %s", dir, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, SubidaMaxBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		fail(w, http.StatusBadRequest, "esto no llegó como un archivo subido: usa un formulario multipart", "archivo")
		return
	}

	guardados := []string{}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			failf(w, http.StatusBadRequest, "archivo", "no se pudo leer lo que subiste: %s", plainUpload(err))
			return
		}
		name := filepath.Base(part.FileName())
		if name == "" || name == "." || name == string(filepath.Separator) {
			continue
		}
		if !ingest.IsMedia(name) {
			failf(w, http.StatusBadRequest, "archivo",
				"%q no parece un video ni un audio de los que sabemos leer", name)
			return
		}
		dst := filepath.Join(dir, name)
		tmp := dst + ".part"
		f, err := os.Create(tmp)
		if err != nil {
			failf(w, http.StatusInternalServerError, "", "no se pudo crear %q: %s", tmp, err)
			return
		}
		n, err := io.Copy(f, part)
		cerr := f.Close()
		if err != nil || cerr != nil {
			_ = os.Remove(tmp)
			failf(w, http.StatusBadRequest, "archivo",
				"la subida de %q se cortó a los %s: vuelve a intentarlo", name, humanBytes(n))
			return
		}
		if err := os.Rename(tmp, dst); err != nil {
			_ = os.Remove(tmp)
			failf(w, http.StatusInternalServerError, "", "no se pudo dejar %q en su sitio: %s", name, err)
			return
		}
		guardados = append(guardados, name)
		s.audit(r, "media_asset", nil, "subido", "", name)
	}
	if len(guardados) == 0 {
		fail(w, http.StatusBadRequest, "no venía ningún archivo", "archivo")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"guardados": guardados,
		"carpeta":   dir,
		"aviso":     "el archivo entra solo en cuanto termine de copiarse; míralo en Biblioteca",
	})
}

// plainUpload traduce el error del límite de tamaño.
func plainUpload(err error) string {
	if strings.Contains(err.Error(), "http: request body too large") {
		return fmt.Sprintf("el archivo pasa de %s, que es el máximo por subida", humanBytes(SubidaMaxBytes))
	}
	return err.Error()
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/float64(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}
