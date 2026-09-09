-- Antena787 · esquema de la base (PRD §15). Versión 1.
--
-- Convenciones (§15 "Convenciones que evitan bugs enteros"):
--   · todo instante es INTEGER: milisegundos UTC desde 1970 (sufijo _ms)
--   · un día de emisión es TEXT 'YYYY-MM-DD' en la zona del canal
--   · una hora del día es INTEGER: minutos desde medianoche local (0..1439)
--   · listas y parámetros van como JSON en TEXT
--   · channel_id va en cada tabla desde F1, aunque haya un solo canal
--   · lo que el PRD pone en el esquema (fechas, solapes) vive AQUÍ, en
--     CHECK y triggers, no en código de validación
PRAGMA foreign_keys = ON;

-- ── el canal y sus salidas ────────────────────────────────────────────
CREATE TABLE channel (
  id                       INTEGER PRIMARY KEY,
  nombre                   TEXT NOT NULL,
  tipo                     TEXT NOT NULL CHECK (tipo IN ('tv','radio')),
  perfil_de_formato        TEXT NOT NULL DEFAULT '720p59.94',
  perfil_regulatorio       TEXT NOT NULL DEFAULT 'internet',
  modo                     TEXT NOT NULL DEFAULT 'sombra' CHECK (modo IN ('sombra','aire')),
  zona_horaria             TEXT NOT NULL DEFAULT 'America/Puerto_Rico',
  hora_inicio_dia_emision  INTEGER NOT NULL DEFAULT 360,   -- 6:00 AM
  carga_maxima_por_hora    INTEGER NOT NULL DEFAULT 12,    -- minutos
  identificativo           TEXT NOT NULL DEFAULT '',
  comunidad_licencia       TEXT NOT NULL DEFAULT '',
  clase_licencia           TEXT NOT NULL DEFAULT 'no_se'  -- lptv | class_a | completa | no_se | no_aplica
);

CREATE TABLE output (
  id               INTEGER PRIMARY KEY,
  channel_id       INTEGER NOT NULL REFERENCES channel(id),
  nombre           TEXT NOT NULL,
  driver           TEXT NOT NULL,                  -- udp-ts | rtp | internet | archivo | ninguna
  parametros       TEXT NOT NULL DEFAULT '{}',     -- JSON
  objetivo_volumen REAL NOT NULL DEFAULT -24,      -- LKFS/LUFS
  estado_conexion  TEXT NOT NULL DEFAULT 'sin_probar',
  reintentos       INTEGER NOT NULL DEFAULT 0,
  ultimo_error     TEXT NOT NULL DEFAULT ''
);

CREATE TABLE capture_input (
  id                INTEGER PRIMARY KEY,
  channel_id        INTEGER NOT NULL REFERENCES channel(id),
  tipo              TEXT NOT NULL CHECK (tipo IN ('receptor-tv','captura','stream','ninguno')),
  punto_de_origen   TEXT NOT NULL DEFAULT '',
  modo              TEXT NOT NULL DEFAULT 'degradado' CHECK (modo IN ('activo','degradado')),
  ultima_senal_vista_ms INTEGER
);

CREATE TABLE driver_config (
  id           INTEGER PRIMARY KEY,
  channel_id   INTEGER REFERENCES channel(id),      -- NULL = global
  tipo         TEXT NOT NULL,                        -- salida | alerta | cobro | fichas | entrada | avisos | transmisor | respaldo | retorno
  driver       TEXT NOT NULL,
  credenciales BLOB,                                 -- cifradas (DPAPI / keyring)
  parametros   TEXT NOT NULL DEFAULT '{}'
);

-- ── el contenido ──────────────────────────────────────────────────────
CREATE TABLE media_asset (
  id                    INTEGER PRIMARY KEY,
  channel_id            INTEGER REFERENCES channel(id),  -- NULL = compartido
  ruta                  TEXT NOT NULL UNIQUE,
  hash                  TEXT NOT NULL DEFAULT '',
  codec                 TEXT NOT NULL DEFAULT '',
  resolucion            TEXT NOT NULL DEFAULT '',
  fps                   TEXT NOT NULL DEFAULT '',
  canales_audio         INTEGER NOT NULL DEFAULT 0,
  duracion_medida_ms    INTEGER NOT NULL DEFAULT 0,
  lufs                  REAL,
  true_peak             REAL,
  tiene_subtitulos      INTEGER NOT NULL DEFAULT 0,
  formato_subtitulos    TEXT NOT NULL DEFAULT '',
  subtitulos_externos   TEXT,                             -- ruta .scc/.srt/.vtt
  negro_cabeza_ms       INTEGER NOT NULL DEFAULT 0,
  negro_cola_ms         INTEGER NOT NULL DEFAULT 0,
  marcas_de_corte_ms    TEXT NOT NULL DEFAULT '[]',       -- JSON [ms,...]
  cuadro_miniatura      TEXT NOT NULL DEFAULT '',         -- ruta del png
  estado                TEXT NOT NULL DEFAULT 'ingiriendo'
                        CHECK (estado IN ('ingiriendo','listo','cuarentena','fallido')),
  motivo_en_cristiano   TEXT NOT NULL DEFAULT '',
  estado_normalizacion  TEXT NOT NULL DEFAULT 'pendiente'
                        CHECK (estado_normalizacion IN ('pendiente','en_curso','listo','fallido')),
  ruta_normalizada      TEXT NOT NULL DEFAULT '',
  negro_intencional     INTEGER NOT NULL DEFAULT 0,
  sin_logo              INTEGER NOT NULL DEFAULT 0,
  dejado_pasar_por      TEXT NOT NULL DEFAULT '',         -- quien lo sacó de cuarentena bajo su nombre
  creado_ms             INTEGER NOT NULL,
  actualizado_ms        INTEGER NOT NULL
);
CREATE INDEX media_asset_estado ON media_asset(estado, estado_normalizacion);

CREATE TABLE title (
  id                       INTEGER PRIMARY KEY,
  channel_id               INTEGER REFERENCES channel(id),
  nombre                   TEXT NOT NULL,
  tipo                     TEXT NOT NULL CHECK (tipo IN ('serie','pelicula','promo','id','spot','cortinilla','programa')),
  sinopsis                 TEXT NOT NULL DEFAULT '',
  anio                     INTEGER,
  genero                   TEXT NOT NULL DEFAULT '',
  clasificacion_contenido  TEXT NOT NULL DEFAULT '',
  clasificacion_audiencia  TEXT NOT NULL DEFAULT '',
  caratula                 TEXT NOT NULL DEFAULT '',      -- ruta
  fuente_ficha             TEXT NOT NULL DEFAULT '',      -- embebida | nfo | coverart-archive | tvmaze | tmdb | manual
  media_asset_id           INTEGER REFERENCES media_asset(id)  -- si no es serie
);

CREATE TABLE episode (
  id             INTEGER PRIMARY KEY,
  title_id       INTEGER NOT NULL REFERENCES title(id) ON DELETE CASCADE,
  temporada      INTEGER NOT NULL DEFAULT 1,
  numero         INTEGER NOT NULL,
  nombre         TEXT NOT NULL DEFAULT '',
  media_asset_id INTEGER REFERENCES media_asset(id),
  UNIQUE (title_id, temporada, numero)
);

CREATE TABLE filler_asset (
  id              INTEGER PRIMARY KEY,
  media_asset_id  INTEGER NOT NULL REFERENCES media_asset(id),
  channel_id      INTEGER REFERENCES channel(id),      -- NULL = genérico
  tipo            TEXT NOT NULL DEFAULT 'promo',        -- promo | id | cortinilla | cartel
  duracion_ms     INTEGER NOT NULL
);

CREATE TABLE live_source (
  id                  INTEGER PRIMARY KEY,
  channel_id          INTEGER NOT NULL REFERENCES channel(id),
  nombre              TEXT NOT NULL,
  tipo                TEXT NOT NULL CHECK (tipo IN ('srt','rtmp','captura')),
  punto_de_escucha    TEXT NOT NULL DEFAULT '',
  solo_audio          INTEGER NOT NULL DEFAULT 0,
  duracion_prevista_ms INTEGER NOT NULL DEFAULT 0,
  filler_de_respaldo  INTEGER REFERENCES filler_asset(id),
  reloj_de_cortes     TEXT NOT NULL DEFAULT '[]',       -- JSON [minuto,...] p.ej. [0,15,30,45]
  driver_de_cue       TEXT NOT NULL DEFAULT '',
  retardo_ms          INTEGER NOT NULL DEFAULT 7000,
  gracia_s            INTEGER NOT NULL DEFAULT 30
);

-- ── la programación ───────────────────────────────────────────────────
CREATE TABLE schedule_rule (
  id                     INTEGER PRIMARY KEY,
  channel_id             INTEGER NOT NULL REFERENCES channel(id),
  tipo                   TEXT NOT NULL DEFAULT 'normal' CHECK (tipo IN ('normal','diferido','bloque_arrendado','vivo')),
  title_id               INTEGER REFERENCES title(id),
  live_source_id         INTEGER REFERENCES live_source(id),
  patron_de_dias         TEXT NOT NULL CHECK (length(patron_de_dias) = 7),  -- 'LMMJVSD', '_' = no; índice 0 = lunes
  hora                   INTEGER NOT NULL CHECK (hora BETWEEN 0 AND 1439),   -- minutos, hora local
  duracion_slot_ms       INTEGER NOT NULL DEFAULT 1800000,
  fecha_inicio           TEXT NOT NULL,                  -- día de emisión
  fecha_fin              TEXT NOT NULL,                  -- inclusivo hasta el cierre del día
  ultimo_aviso_enviado   TEXT NOT NULL DEFAULT '',       -- '' | 30 | 14 | 7
  episodios_por_corrida  INTEGER NOT NULL DEFAULT 1 CHECK (episodios_por_corrida >= 1),
  ultimo_episodio_emitido INTEGER REFERENCES episode(id),
  releva_a               INTEGER REFERENCES schedule_rule(id),
  repite_a               INTEGER REFERENCES schedule_rule(id),
  advertiser_id          INTEGER,                       -- solo bloque_arrendado
  cobro                  REAL,
  ventana_origen_inicio  INTEGER,                       -- solo diferido: minutos
  ventana_origen_fin     INTEGER,
  activa                 INTEGER NOT NULL DEFAULT 1,
  -- Regla de integridad del PRD: aquí muere el bug del Hellsing.
  CHECK (fecha_fin >= fecha_inicio)
);
CREATE INDEX schedule_rule_vigencia ON schedule_rule(channel_id, fecha_inicio, fecha_fin);

CREATE TABLE deck (
  id          INTEGER PRIMARY KEY,
  channel_id  INTEGER NOT NULL REFERENCES channel(id),
  tipo        TEXT NOT NULL CHECK (tipo IN ('manual','comercial','programa','relleno')),
  prioridad   INTEGER NOT NULL,
  UNIQUE (channel_id, tipo)
);

CREATE TABLE plan_item (
  id                  INTEGER PRIMARY KEY,
  channel_id          INTEGER NOT NULL REFERENCES channel(id),
  deck_id             INTEGER NOT NULL REFERENCES deck(id),
  schedule_rule_id    INTEGER REFERENCES schedule_rule(id),
  dia_emision         TEXT NOT NULL,                   -- el del INICIO del ítem
  instante_planeado_ms INTEGER NOT NULL,
  duracion_planeada_ms INTEGER NOT NULL CHECK (duracion_planeada_ms > 0),
  instante_real_ms    INTEGER,
  duracion_real_ms    INTEGER,
  origen              TEXT NOT NULL CHECK (origen IN ('asset','live_source','relleno','cartel')),
  media_asset_id      INTEGER REFERENCES media_asset(id),
  episode_id          INTEGER REFERENCES episode(id),
  live_source_id      INTEGER REFERENCES live_source(id),
  dentro_de           INTEGER REFERENCES plan_item(id),
  corte_id            INTEGER,
  estado              TEXT NOT NULL DEFAULT 'planned'
                      CHECK (estado IN ('planned','cued','aired','skipped','preempted','fallido','manual_hold')),
  parcial             INTEGER NOT NULL DEFAULT 0,
  cued_en_ms          INTEGER,
  error               TEXT NOT NULL DEFAULT '',
  hora_local          TEXT NOT NULL DEFAULT ''         -- 'HH:MM' para mostrar, derivada
);
CREATE INDEX plan_item_tiempo ON plan_item(channel_id, deck_id, instante_planeado_ms);
CREATE INDEX plan_item_dia ON plan_item(channel_id, dia_emision);

-- Restricción de no-solape del PRD §15: dentro de un mismo deck no pueden
-- existir dos plan_item con intervalos solapados. Índice + verificación al
-- insertar y al mover. (Los ítems "dentro_de" un vivo sí se solapan con su
-- padre: viven en otro deck, el comercial.)
CREATE TRIGGER plan_item_sin_solape_insert
BEFORE INSERT ON plan_item
BEGIN
  SELECT RAISE(ABORT, 'plan_item solapado en el mismo deck')
  WHERE EXISTS (
    SELECT 1 FROM plan_item p
    WHERE p.channel_id = NEW.channel_id AND p.deck_id = NEW.deck_id
      AND p.estado NOT IN ('skipped','fallido')
      AND p.instante_planeado_ms < NEW.instante_planeado_ms + NEW.duracion_planeada_ms
      AND NEW.instante_planeado_ms < p.instante_planeado_ms + p.duracion_planeada_ms
  );
END;
CREATE TRIGGER plan_item_sin_solape_update
BEFORE UPDATE OF instante_planeado_ms, duracion_planeada_ms, deck_id ON plan_item
BEGIN
  SELECT RAISE(ABORT, 'plan_item solapado en el mismo deck')
  WHERE EXISTS (
    SELECT 1 FROM plan_item p
    WHERE p.id <> NEW.id
      AND p.channel_id = NEW.channel_id AND p.deck_id = NEW.deck_id
      AND p.estado NOT IN ('skipped','fallido')
      AND p.instante_planeado_ms < NEW.instante_planeado_ms + NEW.duracion_planeada_ms
      AND NEW.instante_planeado_ms < p.instante_planeado_ms + p.duracion_planeada_ms
  );
END;

CREATE TABLE manual_hold (
  id          INTEGER PRIMARY KEY,
  channel_id  INTEGER NOT NULL REFERENCES channel(id),
  inicio_ms   INTEGER NOT NULL,
  fin_ms      INTEGER,
  motivo_fin  TEXT CHECK (motivo_fin IN ('soltado','fin_de_bloque','timeout','caida_del_sistema','parar_todo','quitado')),
  usuario     TEXT NOT NULL DEFAULT ''
);

CREATE TABLE air_recording (
  id              INTEGER PRIMARY KEY,
  channel_id      INTEGER NOT NULL REFERENCES channel(id),
  inicio_ms       INTEGER NOT NULL,
  fin_ms          INTEGER,
  ruta            TEXT NOT NULL,
  retencion_hasta_ms INTEGER NOT NULL
);

-- ── la publicidad (tablas desde F1 para que el esquema no cambie; se usan en F4) ──
CREATE TABLE advertiser (
  id         INTEGER PRIMARY KEY,
  nombre     TEXT NOT NULL,
  tipo       TEXT NOT NULL DEFAULT 'comercial' CHECK (tipo IN ('comercial','politico','no_lucrativo')),
  categoria  TEXT NOT NULL DEFAULT '',
  contacto   TEXT NOT NULL DEFAULT '',
  email      TEXT NOT NULL DEFAULT '',
  whatsapp   TEXT NOT NULL DEFAULT '',
  candidato  TEXT NOT NULL DEFAULT '',
  cargo      TEXT NOT NULL DEFAULT '',
  eleccion   TEXT NOT NULL DEFAULT ''
);

CREATE TABLE insertion_order (
  id               INTEGER PRIMARY KEY,
  channel_id       INTEGER NOT NULL REFERENCES channel(id),
  advertiser_id    INTEGER NOT NULL REFERENCES advertiser(id),
  media_asset_id   INTEGER REFERENCES media_asset(id),
  cantidad         INTEGER NOT NULL,
  duracion_tramo_s INTEGER NOT NULL CHECK (duracion_tramo_s IN (15,30,45,60)),
  ventana_inicio   TEXT NOT NULL,
  ventana_fin      TEXT NOT NULL,
  daypart          TEXT NOT NULL DEFAULT '',
  tope_por_hora    INTEGER NOT NULL DEFAULT 1,
  prioridad        INTEGER NOT NULL DEFAULT 5,
  estado           TEXT NOT NULL DEFAULT 'borrador' CHECK (estado IN ('borrador','pagado_sin_material','activo','terminado')),
  CHECK (ventana_fin >= ventana_inicio)
);

CREATE TABLE corte (
  id                        INTEGER PRIMARY KEY,
  channel_id                INTEGER NOT NULL REFERENCES channel(id),
  plan_item_interrumpido_id INTEGER REFERENCES plan_item(id),
  instante_ms               INTEGER NOT NULL,
  duracion_total_ms         INTEGER NOT NULL
);

CREATE TABLE break_marker (
  id          INTEGER PRIMARY KEY,
  corte_id    INTEGER NOT NULL REFERENCES corte(id),
  offset_ms   INTEGER NOT NULL,
  duracion_ms INTEGER NOT NULL,
  tipo        TEXT NOT NULL DEFAULT 'scte104'
);

CREATE TABLE spot_airing (
  id                 INTEGER PRIMARY KEY,
  insertion_order_id INTEGER NOT NULL REFERENCES insertion_order(id),
  plan_item_id       INTEGER NOT NULL REFERENCES plan_item(id),
  instante_real_ms   INTEGER,
  verificado         INTEGER NOT NULL DEFAULT 0,
  estado             TEXT NOT NULL DEFAULT 'emitido' CHECK (estado IN ('emitido','tapado','make_good')),
  es_makegood_de     INTEGER REFERENCES spot_airing(id)
);

CREATE TABLE classified (
  id                INTEGER PRIMARY KEY,
  channel_id        INTEGER NOT NULL REFERENCES channel(id),
  advertiser_id     INTEGER NOT NULL REFERENCES advertiser(id),
  texto             TEXT NOT NULL,
  ventana_inicio    TEXT NOT NULL,
  ventana_fin       TEXT NOT NULL,
  rotacion          TEXT NOT NULL DEFAULT '{}',
  estado            TEXT NOT NULL DEFAULT 'pendiente' CHECK (estado IN ('pendiente','aprobado','rechazado')),
  linea_patrocinio  TEXT NOT NULL DEFAULT ''
);

CREATE TABLE classified_airing (
  id                 INTEGER PRIMARY KEY,
  classified_id      INTEGER NOT NULL REFERENCES classified(id),
  channel_id         INTEGER NOT NULL REFERENCES channel(id),
  instante_inicio_ms INTEGER NOT NULL,
  instante_fin_ms    INTEGER
);

CREATE TABLE portal_link (
  id                     INTEGER PRIMARY KEY,
  advertiser_id          INTEGER NOT NULL REFERENCES advertiser(id),
  insertion_order_id     INTEGER REFERENCES insertion_order(id),
  classified_id          INTEGER REFERENCES classified(id),
  token                  TEXT NOT NULL UNIQUE,     -- 32 bytes, base64url
  precio                 REAL NOT NULL DEFAULT 0,
  estado                 TEXT NOT NULL DEFAULT 'nuevo',
  media_asset_recibido   INTEGER REFERENCES media_asset(id),
  vence_ms               INTEGER NOT NULL,
  revocado               INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE payment (
  id                 INTEGER PRIMARY KEY,
  portal_link_id     INTEGER NOT NULL REFERENCES portal_link(id),
  driver             TEXT NOT NULL,
  monto              REAL NOT NULL,
  estado             TEXT NOT NULL DEFAULT 'pendiente',
  recurrencia        TEXT NOT NULL DEFAULT 'unica',
  referencia_externa TEXT NOT NULL,
  clave_idempotencia TEXT NOT NULL UNIQUE
);

-- ── lo que el sistema hizo solo ───────────────────────────────────────
CREATE TABLE alert_event (
  id                 INTEGER PRIMARY KEY,
  channel_id         INTEGER NOT NULL REFERENCES channel(id),
  tipo               TEXT NOT NULL CHECK (tipo IN ('real','prueba_semanal','prueba_mensual')),
  inicio_ms          INTEGER NOT NULL,
  fin_ms             INTEGER,
  driver             TEXT NOT NULL DEFAULT '',
  confianza          REAL NOT NULL DEFAULT 1,
  alcance_geografico TEXT NOT NULL DEFAULT '',
  origen             TEXT NOT NULL DEFAULT 'automatico' CHECK (origen IN ('automatico','manual'))
  -- retención mínima 24 meses: la purga vive en código y respeta esto
);

CREATE TABLE incidente (
  id          INTEGER PRIMARY KEY,
  channel_id  INTEGER NOT NULL REFERENCES channel(id),
  tipo        TEXT NOT NULL,
  inicio_ms   INTEGER NOT NULL,
  fin_ms      INTEGER,
  detalle     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX incidente_tiempo ON incidente(channel_id, inicio_ms);

CREATE TABLE audit_log (
  id             INTEGER PRIMARY KEY,
  entidad        TEXT NOT NULL,
  entidad_id     INTEGER,
  campo          TEXT NOT NULL DEFAULT '',
  valor_anterior TEXT NOT NULL DEFAULT '',
  valor_nuevo    TEXT NOT NULL DEFAULT '',
  autor          TEXT NOT NULL DEFAULT '',
  origen         TEXT NOT NULL DEFAULT 'humano' CHECK (origen IN ('humano','mcp','sistema')),
  instante_ms    INTEGER NOT NULL,
  aplica_en      TEXT NOT NULL DEFAULT 'inmediato' CHECK (aplica_en IN ('inmediato','pendiente_de_aprobacion')),
  tipo           TEXT NOT NULL DEFAULT 'cambio',
  hash_prev      TEXT NOT NULL DEFAULT '',
  hash           TEXT NOT NULL DEFAULT ''
);

CREATE TABLE settings (
  clave  TEXT PRIMARY KEY,
  valor  TEXT NOT NULL
);

CREATE TABLE overlay (
  id            INTEGER PRIMARY KEY,
  channel_id    INTEGER NOT NULL REFERENCES channel(id),
  tipo          TEXT NOT NULL CHECK (tipo IN ('logo','clasificados')),
  parametros    TEXT NOT NULL DEFAULT '{}',
  fecha_inicio  TEXT,
  fecha_fin     TEXT
);
