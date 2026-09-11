# CONTINUAR — dónde quedamos y qué sigue

_Última sesión: 9 de septiembre de 2026 (ocho tandas; la octava con agentes en paralelo). Siguiente: T2 y T3 de F2 en paralelo._

## Dónde estamos

- **F0 cerrada.** Reporte en `docs/f0/REPORTE-mac-m4-2026-09-08.md`.
- **F1 construida y verificada.** Los 57 criterios (63 desde la segunda tanda) de `docs/ACEPTACION.md`
  recorridos uno por uno con pruebas (`f1verif_*_test.go` en cada paquete);
  informe completo en **`docs/f1/VERIFICACION-F1-2026-09-09.md`**. De 18
  defectos encontrados, 16 corregidos en la misma sesión y 2 diferidos con
  nota en `ACEPTACION.md` (F1-04 subtítulos → F2; reporte de ingresos de
  F1-55 → F4). Lo que se construyó al corregir:
  - Esquema **versión 2** (migración automática): `plan_item.fijado`,
    `fundido_salida_ms`, triggers de vigencia y de solape por `estado`.
  - **Edición a mano de la parrilla** que sobrevive al resolver:
    `PUT /api/v1/plan/{id}`, panel «¿Solo hoy, o siempre?» cableado, chincheta
    y «Soltar».
  - **Validador de la guía** corriendo antes de publicar (lo grave no sale; los
    huecos se publican y se avisan).
  - **Avisos de vencimiento** en Al aire y por **Telegram o correo** a 7 días
    (Ajustes → Avisos).
  - **Fichas en línea** (TVmaze → TMDB con clave) encendibles desde Ajustes.
  - **Envío opcional de la guía por HTTP** (`guia_destino_http`).
  - La API emite exactamente el contrato de `web/src/lib/tipos.ts` (prueba de
    contrato en `internal/api/contrato_test.go`); Biblioteca, Parrilla
    semana/mes/guía y Al aire pintan datos reales.
  - Un archivo **sin audio va a cuarentena** (se puede soltar bajo
    responsabilidad del operador); LUFS y pico verdadero se guardan.
  - La cola de normalización prioriza por hora de aire calculada desde las
    reglas.
- **Repo:** https://github.com/Saul-Punybz/antena787 (privado). `main` está verde.
- **Investigación y decisiones nuevas (9 sept):**
  `docs/investigacion/PROYECTOS-SIMILARES-2026-09-09.md` (27 proyectos
  comparables) y `docs/adr/0010-eas-integrate-the-endec-never-replace-it.md`
  (EAS: integrar el ENDEC, evidencia SAME en el retorno, CAP informativo).

## Cómo arrancar

```
cd ~/Downloads/_Projects/antena787
git pull
make antena                      # npm ci + build de la interfaz + go build
bin/antena -datos ~/antena-datos # abre http://127.0.0.1:7870
go test ./... -count=1           # todo debe estar verde
```

## Lo próximo, en orden

1. ~~Audio de todo el material~~ **Hecho (9 sept, segunda tanda):** criterios
   F1-58 a F1-63 en `ACEPTACION.md`, esquema **versión 3** (`pistas_audio`,
   `pista_audio_aire`, `pista_audio_sap` reservado para F2, `audio_sidecar`,
   `subtitulos_sidecar`). Archivos de al lado por nombre (audio `.wav/.m4a/
   .aac/.mp3/.flac`, subtítulos `.srt/.vtt` muxeados, `.scc` guardado para
   F2), reingesta sola cuando el audio llega tarde, selector de pista en
   Biblioteca (default por `idioma_audio_preferido`, Ajustes → Audio), y un
   archivo mudo **no se puede soltar**: todo lo que sale al aire lleva audio.
   ~~Pendiente menor: `GET /cuarentena` no manda `titulo`.~~ Resuelto en la
   quinta tanda (punto 4).
2. ~~Asistente de instalación~~ **Hecho (9 sept, tercera tanda; cierra el issue
   #5):** los nueve pasos con lógica real en `web/src/pantallas/Asistente.tsx`
   (riel de progreso, reanudar desde `paso`, respuestas precargadas, barras
   SMPTE dibujadas en el navegador con las dos respuestas activas y una línea
   honesta de que la prueba en la salida llega con el motor). El servidor
   detecta disco y red, entrega las listas de opciones de los pasos 2/4/6
   (nunca se pide un nombre de driver), guarda respuestas y tiempos por paso
   (para saber dónde se abandona, F2-108), valida `calidad`, y en el paso 8
   **arma una propuesta** con lo que haya en la biblioteca (`App.ProponerParrilla`:
   series a diario desde el inicio del día de emisión, películas a las 19:00,
   30 días; no toca nada si ya hay reglas). Nuevo
   `POST /instalacion/relleno-por-defecto`: cartel de la estación (dibujado en
   Go, fuentes Go embebidas) con cama de tonos suaves, 60 s, nunca barras
   (F2-69). `necesita_instalacion` se mantiene hasta el paso 9. Volver al paso 1
   con la clave en blanco conserva la que hay. Modo demo completo
   (`?instalar=1`, `&reiniciar=1` para empezar de cero). Humo de punta a punta
   con el binario real: verde.
   Pendiente menor: `schedule_rule` no tiene campo de nota, así que la
   procedencia «propuesta del asistente» queda en el incidente y en
   `instalacion.propuesta`, no en cada regla (columna `nota` + migración es el
   arreglo). La opción `custom` de formato no se ofrece en el asistente.
3. ~~Pantalla de emparejar títulos~~ **Hecho (9 sept, cuarta tanda; cierra el
   issue #13):** criterios F1-64 a F1-67. Esquema **v4** (`title.pendiente_emparejar`,
   `title.candidatos`, tabla `title_alias`), importador con alias antes del
   difuso y `Unmatched` con reglas/franjas/puntuaciones, las fuentes en vivo
   ya no crean título, `GET /titulos/sin-emparejar`, `GET /titulos/buscar`,
   `POST /titulos/{id}/emparejar` (usar / propio / quitar), alarma «N títulos
   por emparejar» con acción a Reglas, sección «Títulos por emparejar» en
   Reglas con buscador y demo completa. Probado con la hoja real de CAtv:
   3 por emparejar (Samurai X, SaberMarionette con dos candidatos, Los
   Simuladores); tras emparejar Samurai X → Rurouni Kenshin, la reimportación
   ya no pregunta («lo recordaba de otra hoja»).
   Nota: si la hoja pegada no trae su catálogo, el alias se aplica por la API
   al crear la regla (enlaza bien, pero sin el aviso de «lo recordaba»).
4. ~~Cuarentena e incidentes en pantalla~~ **Hecho (9 sept, quinta tanda;
   cierra los issues #6 y #7):** criterios F1-68 y F1-69. `GET /cuarentena`
   ya manda `titulo` con nombre de persona (título o «Serie · T1E4 Nombre»,
   o el nombre del archivo si nadie lo fichó: `TitleRepo.NombreDelArchivo`),
   y mientras quede algo parado Al aire enseña el aviso «N archivos en
   cuarentena» → Biblioteca (`App.RefreshCuarentena`: al arrancar, tras cada
   ingest y al dejar pasar). La **bitácora** vive en Al aire
   (`componentes/Bitacora.tsx`): tarjeta con lo último y panel al lado con
   7/30/90 días agrupado por día; `GET /incidentes` manda `texto`, la frase
   en cristiano de cada tipo (`app.TextoDeIncidente`, con los tipos de F2
   ya escritos), y la pantalla se refresca con cada `{"tipo":"evento"}` del
   WebSocket (`suscribirseAEventos` en `lib/api.ts`). Cambio de semántica:
   **`App.Incident` cierra el suceso en el mismo instante** (`fin` =
   `inicio`); antes quedaban abiertos y «la última semana» arrastraba todo
   lo viejo. Lo que dura (vivo ausente, apagón, F2) se inserta abierto y se
   cierra con `Store.Incident.Close`. Humo con el binario real y un video
   mudo hecho con ffmpeg: cuarentena con nombre, alarma, incidente con
   frase, 409 al intentar soltarlo. Verde.
   Pendiente menor: las filas de `incidente` anteriores a esta tanda siguen
   con `fin` nulo (en una base de pruebas, nada que migrar).
   **Sexta tanda (9 sept):** un agente con sonnet comparó nuestra cuarentena y
   bitácora con ffplayout, nebula, Rivendell, LibreTime y PlayoutAutomation
   (`docs/investigacion/CUARENTENA-Y-BITACORA-COMPARADAS-2026-09-09.md`).
   Nadie tiene cuarentena con motivo+código+quién autorizó ni bitácora en
   tabla; Saul aceptó las dos recomendaciones chicas y están hechas
   (criterios F1-70 y F1-71, esquema **v5** con `media_asset.motivo_codigo`):
   - **Desfase imagen/sonido** > 4 s (`ingest.DesfaseAVMaximo`, como
     ffplayout) → cuarentena con `duracion_av_no_coincide`; el motivo dice las
     dos duraciones. `Measure` ahora trae `VideoMs` y `AudioMs`.
   - **Normalización colgada**: plazo `max(15 min, 4 × duración)`
     (`App.normalizeDeadline`, `Options.NormalizeTimeout` para pruebas); al
     pasarse cuenta como intento fallido «se quedó colgada». Y toda
     normalización que la cola da por perdida **va a cuarentena** con
     `normalizacion_fallida` en vez de quedarse «aún no listo para aire» para
     siempre; dejarla pasar saca el original tal cual (Biblioteca lo dice).
   Del informe quedan sin hacer, por fase: as-run con datos comerciales
   (F4), exportación CSV/PDF de as-run y bitácora (F2/F4), y la advertencia de
   no añadir un QC manual sin regla ni auditoría. Dos recomendaciones del
   agente ya estaban hechas (decode real además de ffprobe; negro en cabeza y
   cola).
5. ~~Modo sombra con archivos de verdad~~ **Hecho (9 sept, séptima tanda):**
   informe **`docs/f1/SOMBRA-2026-09-09.md`**. Seis videos reales (enlaces
   duros en `~/antena-sombra/contenido`, datos en `~/antena-sombra/datos`,
   servidor en `:7871`), asistente por la API, ingest, normalización,
   propuesta, plan y guía de punta a punta. Nueve hallazgos, siete
   corregidos el mismo día: **F1-72** ficha desde el nombre del archivo
   (`internal/ingest/nombre.go`: `S04E01`, `4x01`, `T1E4`, `Cap 07`,
   película con año, cola técnica y firma de grupo fuera; etiquetas que son
   el nombre con puntos o la firma del grupo no mandan), **F1-73** el archivo
   se ve desde el primer segundo (fila en `ingiriendo`, franja «Entrando» en
   Biblioteca, estado final y sidecars en una sola escritura), sinopsis con
   basura de encoder fuera, subtítulo vacío si el episodio no tiene nombre,
   `salto_de_reloj` menciona el sueño. Pendiente: **F2-112** (la máquina no
   se duerme con el canal encendido: la Mac durmió 19 min durante la prueba).
   Tiempos: 5 h 47 min de material en 1 h 35 min de cola (≈4× tiempo real,
   libx264). **Y en el navegador** (S-10 a S-14 del informe): estado sin
   canal por WebSocket, elementos del plan sin nombre, `Regla.titulo` como
   objeto, reglas nuevas apagadas por defecto, semana que solo pintaba 48 h
   (ahora proyecta con el resolver) y «undefined s» en Ajustes; todo
   corregido con prueba. **Windows (issue #14, cerrado):** la F0 corta se colgaba en
   `Decoder.Close` → `cmd.Wait` esperando a que ffmpeg soltara sus
   tuberías; con `WaitDelay` + `Kill` explícito la F0 corta pasa entera en
   Windows en el CI (run 34394830791). Es la primera evidencia de F0 en
   Windows, la plataforma de Rolando. Los tres criterios manuales siguen por firmar con Rolando
   (F1-32 necesita su registro de una hora con VLC; F1-43 visto aquí; F1-53
   con su hoja). Para repetirlo: `bin/antena -datos ~/antena-sombra/datos
   -escucha 127.0.0.1:7871` (la base ya tiene la instalación hecha, clave 1234).
   También hoy: `docs/investigacion/COMPETENCIA-COMERCIAL-2026-09-09.md`
   (Dinesat, VirtualPowerVideo y 15 más) y
   `docs/investigacion/SUBTITULOS-Y-METADATA-2026-09-09.md` (79.1, 608/708,
   SAP, PSIP, **E/I para Class A: no existe en el PRD**, licencias de fichas)
   con criterios propuestos y preguntas para Rolando; **ADR 0010 + F2-111**
   con la regla de CAtv tras una alerta (el corte se termina entero, el
   programa absorbe el tiempo). CI: ffmpeg en el job de pruebas, bloqueo
   real de archivo en Windows (`bloqueo_windows_test.go`); issues #8 y #9
   cerrados.
6. ~~Lo que dejaron los dos informes~~ **Hecho (9 sept, octava tanda, con
   agentes en paralelo):** E/I en la ficha (esquema **v6**, `title.infantil_core`,
   interruptor en la ficha, categoría Infantil/Children en la guía, F1-76; el
   conteo de 156 h/año es F4), umbral de $3M y licencia de TMDB en
   `COMPLIANCE.md` y en Ajustes (F1-74), sidecar `.mcc` (F1-75), F2-112 (la
   máquina no se duerme: `internal/despierto`, probado en el Mac), criterios
   de paridad con VLC (F2-114 a F2-117) y **`docs/f2/PLAN-F2.md`** (diez
   tandas paralelizables, contratos Go entre tandas, puerta de cierre,
   diez preguntas para Saul). Pendiente chico: el selector de subtítulos de
   tres estados del perfil `us-fcc` no existe en la interfaz (el texto del
   $3M vive en `COMPLIANCE.md` hasta entonces). Las preguntas para el
   ingeniero de CAtv y la cadena `sout` de VLC siguen en `PARA-ROLANDO.md`.
7. **F2 · Playout**, por las tandas de `docs/f2/PLAN-F2.md`. **T1 hecha
   (9 sept, rama `agente/t1-motor` integrada):** el motor corre dentro de
   `antena` cuando `channel.modo == "aire"` (`internal/app/motor.go`,
   `engine.ClipSource`, `engine.NuevaLista` para F0, catálogo
   `internal/model/incidentes.go`, `a.guard("motor", …)`); F2-03 (sostener
   el último cuadro) se construyó de paso; en sombra nada cambia. F0 corta
   en `main` tras la fusión: 0 FALLA. Decisiones del agente en el commit
   `14cef5e` (rebasado) y en el plan. **Siguiente: T2 (decks + `udp-ts` con
   PIDs/PCR/multicast) y T3 (detector de silencio/negro) en paralelo, cada
   una en su worktree.** Antes de T2, contestar las preguntas 1, 3 y 10 del
   plan (F2.5, unicast/multicast al TP1000, varias salidas desde el
   principio) y pedir a Rolando su cadena `sout` de VLC.
   **T2 y T3 en curso (10 sept)** con dos agentes (`agente/t2-decks-udpts`,
   `agente/t3-detector`), bajo tres decisiones por defecto que Saul puede
   cambiar: (1) F2-91 a F2-102 (F2.5) no bloquean el cierre de F2; (3) el
   TP1000 recibe unicast IP:puerto por defecto y multicast+TTL es opción del
   mismo driver; (10) varias salidas simultáneas desde T2, solo `udp-ts` y
   `archivo` (internet y `http-ts` en T7). PIDs/programa/bitrate son valores
   de ejemplo configurables hasta tener la cadena `sout` de Rolando.
8. Después de F2 (PRD §22.3), donde además se cierran F1-04
   (CEA-608 con detección propia, ffprobe ≥ 9 ya no emite `closed_captions`),
   el fundido de 1 s del clip recortado (`fundido_salida_ms` ya viene en el
   plan) y los ítems `dentro_de` de un vivo.

## Cosas pequeñas pendientes

- **Decisión pendiente de Saul (10 sept):** página pública «ver en vivo» servida
  por el propio Antena787 (reproductor HLS con el nombre del canal y la guía
  al lado, para el celular del televidente, sin depender de YouTube). Sacar
  la señal a HLS/RTMP ya está en el diseño (modo `internet`, T7 de F2); lo
  que no está es el reproductor público. Salió de comparar con PlayCamTV
  (playcam.tv: biblioteca web gratuita sin app + YouTube/Facebook).
  Criterio de Saul (10 sept): PlayCamTV no es referencia para Antena787; lo
  único con valor propio ahí es el marcador sobre el video, y lo demás
  (teléfono como cámara, YouTube/Facebook, biblioteca, highlights) ya existe
  gratis o lo cubre el diseño (T7, `rtmp-listen`/`srt-listen`). No se copia
  nada de deporte.

- `MarkAired` ya escribe el contador en la regla dueña; el motor (F2) es quien
  lo llamará.
- `media_asset` no tiene columna para el número de pasadas de volumen; hoy el
  registro de las dos pasadas son los valores medidos + evento `material/volumen`.
- `horas_vacias` de Parrilla · Semana cuenta sobre el día natural (la cuadrícula
  arranca a medianoche, como asume la interfaz); si se quiere por día de
  emisión, es un cambio pequeño en `internal/api/plan.go`.
- Cuándo hacer público el repo: el asistente y el modo sombra con archivos
  reales ya funcionan de punta a punta en el Mac (9 sept); falta verlo en la
  PC de Rolando con Windows.
- La propuesta del asistente a 30 días dispara cuatro avisos de vencimiento
  en el acto (S-9 del informe sombra); si a Rolando le hace ruido, la
  propuesta puede salir sin fecha de fin.
- Los incidentes se pintan con la frase del servidor; si un servidor viejo no
  manda `texto`, la interfaz enseña el tipo legible (`textoDe` en
  `Bitacora.tsx`).

## Reglas que no cambian

- **Paridad con VLC** (Saul, 9 sept 2026): todo lo que Rolando hace con el
  stream output de VLC se tiene que poder hacer aquí; lista y huecos en
  `docs/VLC-PARIDAD.md`, firma en F2-113. Pedirle a Rolando su cadena `sout`
  exacta antes de F2.

- Commits limpios, sin atribución a herramientas. Autor: Saul A. González Alonso.
- Agentes con `opus` solo para lo esencial; `sonnet`/`haiku` para lo demás.
  Nunca el modelo de la sesión.
- El software nunca regaña; el cumplimiento se ofrece. Nunca negro, nunca
  silencio, nada manual que no vuelva solo.
- CAtv es donde se prueba, no el molde.
