// Package model son los tipos del dominio: un espejo en Go del esquema
// (internal/store/schema.sql) y del vocabulario (CONTEXT.md). Identificadores
// en inglés; las etiquetas json y db en español, como las columnas, porque
// eso es lo que ven la API, la interfaz y el MCP.
//
// Convenciones: todo instante es time.Time en UTC; un día de emisión es
// Day ('YYYY-MM-DD' en la zona del canal); una hora del día son Minutes
// desde medianoche local.
package model

import (
	"fmt"
	"strings"
	"time"
)

// Day es un día de emisión: 'YYYY-MM-DD' en la zona horaria del canal.
// Empieza a la hora que diga el canal (6:00 AM por defecto), no a medianoche.
type Day string

// ParseDay valida el formato.
func ParseDay(s string) (Day, error) {
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return "", fmt.Errorf("día inválido %q: se espera AAAA-MM-DD", s)
	}
	return Day(s), nil
}

// Time devuelve la medianoche calendario de ese día en la zona dada.
func (d Day) Time(loc *time.Location) time.Time {
	t, _ := time.ParseInLocation("2006-01-02", string(d), loc)
	return t
}

// Add suma días de calendario.
func (d Day) Add(n int) Day { return Day(d.Time(time.UTC).AddDate(0, 0, n).Format("2006-01-02")) }

// Weekday devuelve 0 = lunes … 6 = domingo (el índice del patrón).
func (d Day) Weekday() int { return (int(d.Time(time.UTC).Weekday()) + 6) % 7 }

// Minutes es una hora del día en minutos desde medianoche local (0..1439).
type Minutes int

func (m Minutes) String() string { return fmt.Sprintf("%02d:%02d", int(m)/60, int(m)%60) }

// ParseClock lee "HH:MM".
func ParseClock(s string) (Minutes, error) {
	var h, mi int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d:%d", &h, &mi); err != nil || h < 0 || h > 23 || mi < 0 || mi > 59 {
		return 0, fmt.Errorf("hora inválida %q: se espera HH:MM", s)
	}
	return Minutes(h*60 + mi), nil
}

// DayPattern es el patrón semanal de una regla: siete letras, índice 0 =
// lunes, '_' = ese día no. La forma que usa el Sheet de CAtv: "_MMJVS_".
type DayPattern string

const Letters = "LMMJVSD"

// On dice si el día (0 = lunes) está en el patrón.
func (p DayPattern) On(weekday int) bool {
	return len(p) == 7 && weekday >= 0 && weekday < 7 && p[weekday] != '_' && p[weekday] != ' '
}

// Valid exige siete posiciones con la letra que toca o '_'.
func (p DayPattern) Valid() bool {
	if len(p) != 7 {
		return false
	}
	for i := 0; i < 7; i++ {
		if p[i] != '_' && p[i] != Letters[i] {
			return false
		}
	}
	return true
}

// ── el canal ──────────────────────────────────────────────────────────

type ChannelKind string

const (
	ChannelTV    ChannelKind = "tv"
	ChannelRadio ChannelKind = "radio"
)

type Channel struct {
	ID             int64       `json:"id" db:"id"`
	Name           string      `json:"nombre" db:"nombre"`
	Kind           ChannelKind `json:"tipo" db:"tipo"`
	FormatProfile  string      `json:"perfil_de_formato" db:"perfil_de_formato"`
	RegProfile     string      `json:"perfil_regulatorio" db:"perfil_regulatorio"`
	Mode           string      `json:"modo" db:"modo"` // sombra | aire
	TimeZone       string      `json:"zona_horaria" db:"zona_horaria"`
	BroadcastDayAt Minutes     `json:"hora_inicio_dia_emision" db:"hora_inicio_dia_emision"`
	MaxLoadPerHour int         `json:"carga_maxima_por_hora" db:"carga_maxima_por_hora"`
	CallSign       string      `json:"identificativo" db:"identificativo"`
	LicenseCity    string      `json:"comunidad_licencia" db:"comunidad_licencia"`
	LicenseClass   string      `json:"clase_licencia" db:"clase_licencia"`
}

// Location resuelve la zona horaria del canal.
func (c Channel) Location() *time.Location {
	loc, err := time.LoadLocation(c.TimeZone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// BroadcastDay devuelve el día de emisión al que pertenece un instante:
// lo anterior a la hora de inicio del día cuenta para el día anterior.
func (c Channel) BroadcastDay(t time.Time) Day {
	local := t.In(c.Location())
	shifted := local.Add(-time.Duration(c.BroadcastDayAt) * time.Minute)
	return Day(shifted.Format("2006-01-02"))
}

// DayStart devuelve el instante en que empieza un día de emisión.
func (c Channel) DayStart(d Day) time.Time {
	return d.Time(c.Location()).Add(time.Duration(c.BroadcastDayAt) * time.Minute)
}

// DayEnd devuelve el instante en que termina (exclusivo): el inicio del siguiente.
func (c Channel) DayEnd(d Day) time.Time { return c.DayStart(d.Add(1)) }

type Output struct {
	ID              int64   `json:"id" db:"id"`
	ChannelID       int64   `json:"channel_id" db:"channel_id"`
	Name            string  `json:"nombre" db:"nombre"`
	Driver          string  `json:"driver" db:"driver"`
	Params          string  `json:"parametros" db:"parametros"` // JSON
	TargetLoudness  float64 `json:"objetivo_volumen" db:"objetivo_volumen"`
	ConnectionState string  `json:"estado_conexion" db:"estado_conexion"`
	Retries         int     `json:"reintentos" db:"reintentos"`
	LastError       string  `json:"ultimo_error" db:"ultimo_error"`
}

// ── el contenido ──────────────────────────────────────────────────────

type AssetState string

const (
	AssetIngesting  AssetState = "ingiriendo"
	AssetReady      AssetState = "listo"
	AssetQuarantine AssetState = "cuarentena"
	AssetFailed     AssetState = "fallido"
)

type MediaAsset struct {
	ID               int64      `json:"id" db:"id"`
	ChannelID        *int64     `json:"channel_id" db:"channel_id"`
	Path             string     `json:"ruta" db:"ruta"`
	Hash             string     `json:"hash" db:"hash"`
	Codec            string     `json:"codec" db:"codec"`
	Resolution       string     `json:"resolucion" db:"resolucion"`
	FPS              string     `json:"fps" db:"fps"`
	AudioChannels    int        `json:"canales_audio" db:"canales_audio"`
	DurationMs       int64      `json:"duracion_medida_ms" db:"duracion_medida_ms"`
	LUFS             *float64   `json:"lufs" db:"lufs"`
	TruePeak         *float64   `json:"true_peak" db:"true_peak"`
	HasCaptions      bool       `json:"tiene_subtitulos" db:"tiene_subtitulos"`
	CaptionFormat    string     `json:"formato_subtitulos" db:"formato_subtitulos"`
	ExternalCaptions *string    `json:"subtitulos_externos" db:"subtitulos_externos"`
	HeadBlackMs      int64      `json:"negro_cabeza_ms" db:"negro_cabeza_ms"`
	TailBlackMs      int64      `json:"negro_cola_ms" db:"negro_cola_ms"`
	BreakMarksMs     []int64    `json:"marcas_de_corte_ms" db:"marcas_de_corte_ms"` // JSON en la base
	Thumbnail        string     `json:"cuadro_miniatura" db:"cuadro_miniatura"`
	State            AssetState `json:"estado" db:"estado"`
	PlainReason      string     `json:"motivo_en_cristiano" db:"motivo_en_cristiano"`
	MotivoCodigo     string     `json:"motivo_codigo" db:"motivo_codigo"`               // por qué se paró, en clave: sin_audio, duracion_av_no_coincide, normalizacion_fallida; vacío en lo demás
	NormalizeState   string     `json:"estado_normalizacion" db:"estado_normalizacion"` // pendiente | en_curso | listo | fallido
	NormalizedPath   string     `json:"ruta_normalizada" db:"ruta_normalizada"`
	IntentionalBlack bool       `json:"negro_intencional" db:"negro_intencional"`
	NoLogo           bool       `json:"sin_logo" db:"sin_logo"`
	LetThroughBy     string     `json:"dejado_pasar_por" db:"dejado_pasar_por"`

	// Sonido del archivo (F1-58 a F1-63). Todo lo que sale al aire lleva
	// audio: aquí queda qué pistas trae, cuál va al aire y de dónde salió.
	PistasAudio       []PistaAudio `json:"pistas_audio" db:"pistas_audio"`             // JSON en la base
	PistaAudioAire    int          `json:"pista_audio_aire" db:"pista_audio_aire"`     // índice dentro de PistasAudio
	PistaAudioSAP     *int         `json:"pista_audio_sap" db:"pista_audio_sap"`       // segunda pista al aire; vacío hasta F2
	AudioSidecar      string       `json:"audio_sidecar" db:"audio_sidecar"`           // archivo de audio de al lado que se muxeó
	SubtitulosSidecar string       `json:"subtitulos_sidecar" db:"subtitulos_sidecar"` // archivo de subtítulos de al lado

	CreatedAt time.Time `json:"creado" db:"creado_ms"`
	UpdatedAt time.Time `json:"actualizado" db:"actualizado_ms"`
}

// PistaAudio es una de las pistas de sonido que trae el archivo, tal como la
// vio el ingest: su número dentro del archivo, el idioma que declara, cuántos
// canales lleva y el nombre con que viene rotulada.
type PistaAudio struct {
	Indice  int    `json:"indice"`
	Idioma  string `json:"idioma"`
	Canales int    `json:"canales"`
	Titulo  string `json:"titulo"`
}

// idiomasEquivalentes junta las etiquetas que en la práctica quieren decir lo
// mismo: un archivo puede venir rotulado "es", "spa" o "esp" y es el mismo
// español. La clave es la etiqueta ya en minúsculas.
var idiomasEquivalentes = map[string]string{
	"es": "es", "spa": "es", "esp": "es",
	"en": "en", "eng": "en",
}

// idiomaNormalizado devuelve la etiqueta comparable de un idioma: sin
// espacios, en minúsculas y con los equivalentes unificados.
func idiomaNormalizado(idioma string) string {
	limpio := strings.ToLower(strings.TrimSpace(idioma))
	if igual, ok := idiomasEquivalentes[limpio]; ok {
		return igual
	}
	return limpio
}

// PistaPreferida devuelve el índice de la primera pista en ese idioma. Si
// ninguna lo trae —o el archivo no declara pistas— devuelve 0: la primera del
// archivo, que es lo que se oye si no se elige nada (F1-60).
func (m MediaAsset) PistaPreferida(idioma string) int {
	buscado := idiomaNormalizado(idioma)
	if buscado == "" {
		return 0
	}
	for _, p := range m.PistasAudio {
		if idiomaNormalizado(p.Idioma) == buscado {
			return p.Indice
		}
	}
	return 0
}

// AirablePath es lo que sale al aire: la copia normalizada si existe.
func (a MediaAsset) AirablePath() string {
	if a.NormalizedPath != "" {
		return a.NormalizedPath
	}
	return a.Path
}

type TitleKind string

const (
	TitleSeries  TitleKind = "serie"
	TitleMovie   TitleKind = "pelicula"
	TitleProgram TitleKind = "programa"
	TitlePromo   TitleKind = "promo"
	TitleID      TitleKind = "id"
	TitleSpot    TitleKind = "spot"
	TitleBumper  TitleKind = "cortinilla"
)

type Title struct {
	ID             int64     `json:"id" db:"id"`
	ChannelID      *int64    `json:"channel_id" db:"channel_id"`
	Name           string    `json:"nombre" db:"nombre"`
	Kind           TitleKind `json:"tipo" db:"tipo"`
	Synopsis       string    `json:"sinopsis" db:"sinopsis"`
	Year           *int      `json:"anio" db:"anio"`
	Genre          string    `json:"genero" db:"genero"`
	ContentRating  string    `json:"clasificacion_contenido" db:"clasificacion_contenido"`
	AudienceRating string    `json:"clasificacion_audiencia" db:"clasificacion_audiencia"`
	Artwork        string    `json:"caratula" db:"caratula"`
	MetadataSource string    `json:"fuente_ficha" db:"fuente_ficha"`
	MediaAssetID   *int64    `json:"media_asset_id" db:"media_asset_id"`

	// PendienteEmparejar dice que este título lo creó el importador porque el
	// nombre de la hoja no cuadró con ninguna ficha del catálogo (F1-64): la
	// regla se importó igual, pero alguien tiene que decir con qué ficha va.
	PendienteEmparejar bool `json:"pendiente_emparejar" db:"pendiente_emparejar"`

	// Candidatos son las fichas del catálogo que se le parecían tanto que el
	// importador no se atrevió a elegir (F1-65). Vacío cuando no se parecía
	// a ninguna.
	Candidatos []int64 `json:"candidatos" db:"candidatos"`

	// InfantilCore dice que el programa es de educación o información para
	// niños —"core" en el sentido del Children's Television Act— y por eso
	// cuenta para las horas de programación infantil que una estación Class A
	// tiene que emitir (F1-76). Lo marca una persona en la ficha; el conteo de
	// las 156 horas al año y el reporte del Form 2100 Schedule H llegan con el
	// reporte de emisión (F4).
	InfantilCore bool `json:"infantil_core" db:"infantil_core"`
}

// TitleAlias es cómo llama la hoja a una ficha del catálogo: «Samurai X» es
// la ficha «Rurouni Kenshin». Se aprende cuando una persona empareja los dos
// (F1-66) y desde entonces la hoja se empareja sola.
type TitleAlias struct {
	ID        int64     `json:"id" db:"id"`
	ChannelID *int64    `json:"channel_id" db:"channel_id"`
	Alias     string    `json:"alias" db:"alias"` // como lo escribe la hoja
	Clave     string    `json:"clave" db:"clave"` // ClaveDeNombre(Alias)
	TitleID   int64     `json:"title_id" db:"title_id"`
	Creado    time.Time `json:"creado" db:"creado"`
}

type Episode struct {
	ID           int64  `json:"id" db:"id"`
	TitleID      int64  `json:"title_id" db:"title_id"`
	Season       int    `json:"temporada" db:"temporada"`
	Number       int    `json:"numero" db:"numero"`
	Name         string `json:"nombre" db:"nombre"`
	MediaAssetID *int64 `json:"media_asset_id" db:"media_asset_id"`
}

type FillerAsset struct {
	ID           int64  `json:"id" db:"id"`
	MediaAssetID int64  `json:"media_asset_id" db:"media_asset_id"`
	ChannelID    *int64 `json:"channel_id" db:"channel_id"`
	Kind         string `json:"tipo" db:"tipo"`
	DurationMs   int64  `json:"duracion_ms" db:"duracion_ms"`
}

type LiveSource struct {
	ID                int64  `json:"id" db:"id"`
	ChannelID         int64  `json:"channel_id" db:"channel_id"`
	Name              string `json:"nombre" db:"nombre"`
	Kind              string `json:"tipo" db:"tipo"` // srt | rtmp | captura
	ListenPoint       string `json:"punto_de_escucha" db:"punto_de_escucha"`
	AudioOnly         bool   `json:"solo_audio" db:"solo_audio"`
	PlannedDurationMs int64  `json:"duracion_prevista_ms" db:"duracion_prevista_ms"`
	BackupFillerID    *int64 `json:"filler_de_respaldo" db:"filler_de_respaldo"`
	BreakClock        []int  `json:"reloj_de_cortes" db:"reloj_de_cortes"` // minutos de la hora: [0,15,30,45]
	CueDriver         string `json:"driver_de_cue" db:"driver_de_cue"`
	DelayMs           int    `json:"retardo_ms" db:"retardo_ms"`
	GraceSeconds      int    `json:"gracia_s" db:"gracia_s"`
}

// ── la programación ───────────────────────────────────────────────────

type RuleKind string

const (
	RuleNormal    RuleKind = "normal"
	RuleTimeShift RuleKind = "diferido"
	RuleLeased    RuleKind = "bloque_arrendado"
	RuleLive      RuleKind = "vivo"
)

// ScheduleRule es la intención de una persona (CONTEXT: Rule). Las fechas
// son días de emisión; fecha_fin es inclusiva hasta el cierre del día.
type ScheduleRule struct {
	ID                int64      `json:"id" db:"id"`
	ChannelID         int64      `json:"channel_id" db:"channel_id"`
	Kind              RuleKind   `json:"tipo" db:"tipo"`
	TitleID           *int64     `json:"title_id" db:"title_id"`
	LiveSourceID      *int64     `json:"live_source_id" db:"live_source_id"`
	Days              DayPattern `json:"patron_de_dias" db:"patron_de_dias"`
	At                Minutes    `json:"hora" db:"hora"`
	SlotMs            int64      `json:"duracion_slot_ms" db:"duracion_slot_ms"`
	From              Day        `json:"fecha_inicio" db:"fecha_inicio"`
	To                Day        `json:"fecha_fin" db:"fecha_fin"`
	LastNoticeSent    string     `json:"ultimo_aviso_enviado" db:"ultimo_aviso_enviado"`
	EpisodesPerRun    int        `json:"episodios_por_corrida" db:"episodios_por_corrida"`
	LastEpisodeAired  *int64     `json:"ultimo_episodio_emitido" db:"ultimo_episodio_emitido"`
	HandsOffTo        *int64     `json:"releva_a" db:"releva_a"`
	RepeatsOf         *int64     `json:"repite_a" db:"repite_a"`
	AdvertiserID      *int64     `json:"advertiser_id" db:"advertiser_id"`
	Fee               *float64   `json:"cobro" db:"cobro"`
	SourceWindowStart *Minutes   `json:"ventana_origen_inicio" db:"ventana_origen_inicio"`
	SourceWindowEnd   *Minutes   `json:"ventana_origen_fin" db:"ventana_origen_fin"`
	Active            bool       `json:"activa" db:"activa"`
}

// Covers dice si la regla aplica a un día de emisión.
func (r ScheduleRule) Covers(d Day) bool {
	return r.Active && d >= r.From && d <= r.To && r.Days.On(d.Weekday())
}

// DaysLeft devuelve cuántos días de emisión quedan hasta fecha_fin (0 = hoy es el último).
func (r ScheduleRule) DaysLeft(today Day) int {
	return int(r.To.Time(time.UTC).Sub(today.Time(time.UTC)).Hours() / 24)
}

type DeckKind string

const (
	DeckManual     DeckKind = "manual"
	DeckCommercial DeckKind = "comercial"
	DeckProgram    DeckKind = "programa"
	DeckFiller     DeckKind = "relleno"
)

// DeckPriority es el orden del PRD §9 paso 4: menor número, más prioridad.
var DeckPriority = map[DeckKind]int{DeckManual: 0, DeckCommercial: 1, DeckProgram: 2, DeckFiller: 3}

type Deck struct {
	ID        int64    `json:"id" db:"id"`
	ChannelID int64    `json:"channel_id" db:"channel_id"`
	Kind      DeckKind `json:"tipo" db:"tipo"`
	Priority  int      `json:"prioridad" db:"prioridad"`
}

type PlanState string

const (
	Planned    PlanState = "planned"
	Cued       PlanState = "cued"
	Aired      PlanState = "aired"
	Skipped    PlanState = "skipped"
	Preempted  PlanState = "preempted"
	Failed     PlanState = "fallido"
	ManualHold PlanState = "manual_hold"
)

// PlanItem es una emisión resuelta a un instante exacto. Es también el
// as-run: lo planeado queda intacto y lo real se llena después.
type PlanItem struct {
	ID           int64      `json:"id" db:"id"`
	ChannelID    int64      `json:"channel_id" db:"channel_id"`
	DeckID       int64      `json:"deck_id" db:"deck_id"`
	RuleID       *int64     `json:"schedule_rule_id" db:"schedule_rule_id"`
	BroadcastDay Day        `json:"dia_emision" db:"dia_emision"`
	PlannedAt    time.Time  `json:"instante_planeado" db:"instante_planeado_ms"`
	PlannedMs    int64      `json:"duracion_planeada_ms" db:"duracion_planeada_ms"`
	ActualAt     *time.Time `json:"instante_real" db:"instante_real_ms"`
	ActualMs     *int64     `json:"duracion_real_ms" db:"duracion_real_ms"`
	Origin       string     `json:"origen" db:"origen"` // asset | live_source | relleno | cartel
	MediaAssetID *int64     `json:"media_asset_id" db:"media_asset_id"`
	EpisodeID    *int64     `json:"episode_id" db:"episode_id"`
	LiveSourceID *int64     `json:"live_source_id" db:"live_source_id"`
	Inside       *int64     `json:"dentro_de" db:"dentro_de"`
	BreakID      *int64     `json:"corte_id" db:"corte_id"`
	State        PlanState  `json:"estado" db:"estado"`
	Partial      bool       `json:"parcial" db:"parcial"`
	CuedAt       *time.Time `json:"cued_en" db:"cued_en_ms"`
	Error        string     `json:"error" db:"error"`
	LocalClock   string     `json:"hora_local" db:"hora_local"`
	// FadeOutMs es el fundido de salida, en milisegundos: 0 es "sale entero".
	// Lo pone el resolver en el último clip de relleno cuando hay que
	// recortarlo para cuadrar el hueco (PRD §14.1: fundido de 1 segundo).
	FadeOutMs int64 `json:"fundido_salida_ms" db:"fundido_salida_ms"`
	// Fijado dice que a este ítem lo puso una persona a mano: el resolver no
	// lo mueve ni lo borra en su próxima corrida, y su hora es tan dura como
	// la de un bloque en vivo.
	Fijado bool `json:"fijado" db:"fijado"`
}

// End es el fin planeado.
func (p PlanItem) End() time.Time {
	return p.PlannedAt.Add(time.Duration(p.PlannedMs) * time.Millisecond)
}

// ── lo que el sistema hizo solo ───────────────────────────────────────

type Incident struct {
	ID        int64      `json:"id" db:"id"`
	ChannelID int64      `json:"channel_id" db:"channel_id"`
	Kind      string     `json:"tipo" db:"tipo"`
	Start     time.Time  `json:"inicio" db:"inicio_ms"`
	End       *time.Time `json:"fin" db:"fin_ms"`
	Detail    string     `json:"detalle" db:"detalle"`
}

type AuditEntry struct {
	ID        int64     `json:"id" db:"id"`
	Entity    string    `json:"entidad" db:"entidad"`
	EntityID  *int64    `json:"entidad_id" db:"entidad_id"`
	Field     string    `json:"campo" db:"campo"`
	Before    string    `json:"valor_anterior" db:"valor_anterior"`
	After     string    `json:"valor_nuevo" db:"valor_nuevo"`
	Author    string    `json:"autor" db:"autor"`
	Origin    string    `json:"origen" db:"origen"` // humano | mcp | sistema
	At        time.Time `json:"instante" db:"instante_ms"`
	AppliesAt string    `json:"aplica_en" db:"aplica_en"`
	Kind      string    `json:"tipo" db:"tipo"`
	HashPrev  string    `json:"hash_prev" db:"hash_prev"`
	Hash      string    `json:"hash" db:"hash"`
}

// ── ayudas de tiempo ──────────────────────────────────────────────────

// Ms convierte un time.Time a milisegundos UTC (lo que guarda la base).
func Ms(t time.Time) int64 { return t.UnixMilli() }

// FromMs deshace Ms.
func FromMs(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

// FromMsPtr deshace Ms para columnas que pueden ser NULL.
func FromMsPtr(ms *int64) *time.Time {
	if ms == nil {
		return nil
	}
	t := FromMs(*ms)
	return &t
}

// ── vocabularios que el esquema fija en CHECK y aquí en constantes ────
// (para que tres paquetes no escriban la misma cadena de tres formas)

// Origen de un plan_item.
const (
	OriginAsset      = "asset"
	OriginLiveSource = "live_source"
	OriginFiller     = "relleno"
	OriginSlate      = "cartel"
)

// Estado de normalización de un media_asset.
const (
	NormalizePending = "pendiente"
	NormalizeRunning = "en_curso"
	NormalizeReady   = "listo"
	NormalizeFailed  = "fallido"
)

// Avisos de vencimiento de una regla (ultimo_aviso_enviado): el umbral en días.
const (
	Notice30 = "30"
	Notice14 = "14"
	Notice7  = "7"
)

// Ready dice si un asset puede salir al aire: ingerido y normalizado.
func (a MediaAsset) Ready() bool { return a.State == AssetReady && a.NormalizeState == NormalizeReady }
