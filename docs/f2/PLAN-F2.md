# Plan de F2 · Playout — tandas paralelizables

_9 de septiembre de 2026. F0 cerrada en Mac M4 (`docs/f0/REPORTE-mac-m4-2026-09-08.md`),
pendiente la corrida en la PC de CAtv. F1 construida y verificada
(`docs/f1/VERIFICACION-F1-2026-09-09.md`). Este documento reparte §22.3 del
PRD y los criterios F2-01 a F2-113 de `docs/ACEPTACION.md` en tandas que
varios agentes pueden construir a la vez sin pisarse el archivo._

**F2-112 (no dormir la máquina) ya se está construyendo aparte — no entra en
ninguna tanda de este plan.**

---

## 1 · Mapa de lo que hay

| Pieza | Qué es hoy | Se reutiliza tal cual | Se adapta | Falta |
|---|---|---|---|---|
| `internal/engine/decoder.go` | `StartDecoder`, `videoFilter` (bwdif+scale+pad+fps), `audioFilter`, `pcmReader` | **Sí, entero.** Ya cumple pillarbox (F2-04), conformado A/V (F2-02/03), pre-roll | — | — |
| `internal/engine/encoder.go` | `Encoder`, `Output{Name,Kind,File,UDP,VideoKbs,MuxKbs,GainDB}`, `outputArgs` con `mpeg2-ts`/`h264-ts`, `Done()`, `PID()` | El encoder persistente y el truco de dos TCP locales | `Output` hoy es una struct fija de F0: hace falta traducir `model.Output` (driver, `parametros` JSON) a esto, y añadir PIDs/tsid/program/multicast/TTL como parámetros, y el `Kind` `http-ts` | Watchdog de 3 s sobre `Done()` (F2-11), reconexión con backoff (F2-48/49) |
| `internal/engine/frameserver.go` | `Server.Run` recorre una `Playlist []Clip` fija en bucle; `play`/`fill`/`pump*`/`emit` ya hacen fundido, sostenido de cuadro, relleno | `pump`, `emit`, `fadeIn/fadeOut`, `samplesFor` (la aritmética de F2-02 está aquí) | `Run` asume una lista circular fija: hay que cambiarla por una fuente que se consulta en cada corte (los decks) | Selección por decks y prioridad, cortes a hora exacta (F2-06/07/08), pausar/reanudar programa (F2-07), pre-emption por alerta (F2-111) |
| `internal/engine/format.go` | `Format`, `CAtv` (720p59.94) | Sí, entero | — | Otros perfiles son F5 |
| `internal/f0/*` | Arnés de prueba: fabrica los 9+1 clips de prueba, corre `engine.Server` con ellos, analiza la salida | El analizador (`ts.Analyze`, `checkCuts`, `checkAudio`) sirve para instrumentar el soak de 30 días (T10) | — | El motor real **no vive en `f0`**: hay que moverlo a `internal/app`, con `f0` quedando solo como arnés de laboratorio |
| `internal/model/model.go` | `Deck`, `DeckKind`, `DeckPriority` (manual=0, comercial=1, programa=2, relleno=3), `PlanItem.DeckID`, `LiveSource{GraceSeconds,DelayMs,BreakClock,CueDriver}`, `Output{Driver,Params,TargetLoudness,ConnectionState,Retries,LastError}`, `PlanState.ManualHold` | Todo esto ya está pensado para F2 y no hace falta tocarlo | — | `CaptureInput`, `AlertEvent`, `AirRecording` no existen |
| `internal/store/channel.go` | `DeckRepo.ByKind`, `OutputRepo` (esqueleto) | `DeckRepo` | `OutputRepo` — completar CRUD y reconexión | Repos de `CaptureInput`, `AlertEvent`, `AirRecording`; purgas de retención (F2-43/85/91-95) |
| `internal/app/app.go` | `App.guard/Incident/Publish`, `resolverLoop/ingestLoop/normalizeLoop/backupLoop/diskLoop/clockLoop`, `MarkAired` (existe, nadie la llama) | El patrón `guard` (pánico → incidente → relanza) es la base de todo lo nuevo | `Start()` es donde cada tanda añade su `a.guard(...)` — punto de fusión, ver riesgos | `motorLoop`, `manualLoop`/estado de retención, `vivoLoop`, `watchdogLoop`, `grabacionLoop`, `diferidoLoop` |
| `internal/api/*` | HTTP+WS, `estado.go`, `ws.go` (`pushStatus`), `reglas.go`/`plan.go` | El patrón de endpoints y el WebSocket de eventos | — | Endpoints de tomar/soltar control, CRUD de fuentes en vivo y salidas, reproducción de grabación, exportación de as-run |
| `internal/drivers/` | `salida/` construido en T2: `udp-ts` (unicast, multicast y TTL, PID/programa/tsid/PCR) y `archivo`, con `Driver{Abrir, Vigilar, Descripcion}` | `salida.Driver` como patrón para los demás | — | Falta: salida `internet` y `http-ts` (T7), vivo (`srt-listen`, `rtmp-listen`, `url`, `captura`, T4), captura de retorno y ENDEC/telemetría (T8) |
| `web/src/pantallas/AlAire.tsx` | Hoy dice "Aquí se vería tu señal"; ya tiene `Bitacora.tsx` y alarmas reales de F1 | El patrón de alarmas y el WebSocket | — | Panel de disparo manual, ventana local, grabación/diferido, tablero real |

---

## 2 · Tandas

Diez tandas. El orden 1→2→3 sigue lo que §22.3 pide probar primero
(motor/conformado dentro de `antena`, decks, `udp-ts`); 4-8 siguen el resto
de §22.3; 9-10 cierran con lo que pide la tarea (tablero, telemetría/ENDEC,
soak).

### T1 · Motor y conformado dentro de `antena` — **HECHA** (9 sept 2026)

> Rama `agente/t1-motor`, un solo commit («Motor y conformado dentro de
> `antena`: el plan alimenta el servidor de cuadros»). El hash es el de esa
> rama: un commit no puede llevar su propio hash escrito dentro, así que se
> anota al fusionar. Lo construido está criterio por criterio en
> `docs/ACEPTACION.md` (F2-02, 03, 04, 05, 09, 10, 12, 13, 14, 16, 17), y
> F2-01 se midió por primera vez con la F0 corta después del cambio: 7572
> cuadros, 0 reinicios, ningún FALLA en `f0/out/REPORTE.md`.
>
> **Lo que T1 dejó puesto, y que T2 hereda:**
>
> - `engine.ClipSource` tal como decía el contrato, y `NewServer(formato,
>   enc, src, filler)` — con `enc` como la interfaz `engine.Sink` en vez de
>   `*Encoder` (que la cumple): así el conformado se prueba sin levantar
>   ffmpeg. `Server.Playlist` ya no existe; el arnés de `f0` usa
>   `engine.NuevaLista`.
> - `engine.Avisada` (opcional): la fuente que la implementa se entera de qué
>   clip salió —para `MarkAired`— y qué clip falló —para la cuarentena de
>   F2-12—. `internal/app` la implementa; el motor funciona sin ella.
> - `Clip.SeekMs` (entrar a un archivo por el medio, F2-13) y `Clip.Ref` (el
>   id del `plan_item`, que el motor no mira).
> - `until` es **corte**: al llegar, el motor pregunta otra vez. Si la fuente
>   contesta el mismo clip, no se reinicia nada: se corre el corte.
> - `engine.Output.File` puede ir vacío (salida solo por UDP). Es el único
>   cambio de T1 en `encoder.go`, que es de T2: revisarlo al fusionar.
> - El reloj del aire es `Server.AirTime` —cuadros, no `time.Now`— y la
>   deriva se corrige en `corregirDeriva`, comparando con la hora de pared
>   descontando el colchón de `Lead`.
> - Catálogo de incidentes en `internal/model/incidentes.go`. Dos nombres
>   cambiaron a lo que dicen el PRD y el contrato: `encoder_colgado` →
>   `encoder_reiniciado`, `timeout_manual` → `manual_por_timeout` (los
>   viejos siguen teniendo frase en `TextoDeIncidente`, nadie escribe con
>   ellos).

**Objetivo:** sacar el servidor de cuadros del arnés `f0` y ponerlo a correr
dentro del proceso real, alimentado por el plan de verdad, sin decks todavía
(un solo carril).
**Cierra:** F2-02, F2-03, F2-04, F2-05, F2-09, F2-10, F2-12, F2-13, F2-14,
F2-16, F2-17. (F2-01 se **mide** aquí por primera vez con datos reales, pero
solo se **cierra** en T10, con el soak de 30 días.)
**Toca:** `internal/engine/frameserver.go` (nueva interfaz `ClipSource` en
vez de `Playlist []Clip` fija — ver §3), `internal/app/motor.go` (nuevo),
`cmd/antena/main.go` (arranca el motor cuando `channel.Mode == "aire"`).
**Depende de:** nada (es la base).
**Tamaño estimado:** ~700 líneas Go.
**Riesgos:** es el archivo más leído línea por línea (F2-109); tocarlo dos
veces a la vez desde otra tanda es el error más caro. Ninguna otra tanda
debe tocar `frameserver.go` hasta que T1 esté fusionada.

### T2 · Decks, prioridad y salida `udp-ts` — **HECHA** (10 sept 2026)

> Rama `agente/t2-decks-udpts`, un solo commit. Lo construido está criterio por
> criterio en `docs/ACEPTACION.md` (F2-06, 07, 08, 46, 47, 49, 50, 114, y la
> parte de F2-48 que toca a `udp-ts`). F0 corta después del cambio: 7572
> cuadros, 0 FALLA, TS a 10.000 Mb/s ±0.00 % con PCR máx 20.3 ms.
>
> **Decisiones del agente, y lo que T4/T5/T6/T7 heredan:**
>
> - `internal/drivers/salida` con el contrato del plan más un tercer método,
>   `Descripcion() string`: la frase en cristiano de a dónde va una salida
>   («al grupo 239.1.1.1:1234, 4 salto(s) de red · MPEG-2 8000 kb/s de
>   imagen…»), que es lo que pinta Al aire y lo que contesta la API. Sin ella,
>   cada pantalla tendría que volver a interpretar los `parametros`.
>   `Vigilar` recibe un `Registro` (lo cumple `store.OutputRepo.SetConnection`)
>   en el constructor `salida.Para(model.Output, Registro)`.
> - **`engine.Output` creció** con `PIDVideo`, `PIDAudio`, `PIDPMT`,
>   `Programa`, `TSID`, `PCRms`, `Audio`, `TTL` y `PktSize`. En cero, cada uno
>   cae en lo que ya emitía la F0 (incluidas las dos pistas de audio, mp2 y
>   ac3, cuando nadie elige códec de audio), así que el arnés de `f0` no
>   cambió. Los PID se fijan con `-streamid` por flujo, no solo con
>   `mpegts_start_pid`: VLC deja escribir el del video y el del audio por
>   separado y aquí también.
> - **Los valores de ejemplo** (mux 10000 kb/s, video 8000, PMT 480, video
>   512, audio 513, programa 1, tsid 1, PCR 20 ms, audio mp2) viven en
>   constantes de `internal/drivers/salida`, nunca en el motor, y se cambian
>   por `PUT /api/v1/salidas/{id}`. Un cero en los `parametros` quiere decir
>   «esto no lo escribí».
> - **Varias salidas** son ramas del mismo encoder persistente, no varios
>   ffmpeg: `App.abrirSalidas` abre todas las que puede y **salta la que no**
>   —la deja apuntada con su motivo y avisa—, que es lo que hace F2-49 cierto
>   por construcción.
> - **Decks:** el motor lee la tabla `deck` y elige por `prioridad`.
>   `fuenteDelPlan` lleva la cuenta de por dónde va cada bloque **sobre su
>   propia línea de tiempo** (`avance`, `tomaElAire`, `posicionDe`), que es lo
>   que hace que pausar y reanudar cuadre al milisegundo; un bloque cuyo
>   origen es `live_source` no acumula: se vuelve a la señal en su instante
>   actual. Cada cambio de deck se publica una vez, con la hora.
> - **Cambio de conducta heredado de T1:** un bloque de archivo interrumpido
>   ya **no** vuelve por la hora de pared sino por donde se quedó, así que la
>   prueba de F2-16 cambió de expectativa (está dicho en el propio archivo de
>   prueba).
> - `internal/ts` aprendió a leer **PAT y PMT** (programa, tsid, PID de la
>   PMT, del PCR y de cada flujo): sin eso no se puede verificar F2-46/114 sin
>   depender de ffprobe, y el informe de la F0 lo enseña de paso.
> - **Lo que T2 no hizo:** el watchdog de 3 s (T6), `internet`/`http-ts` (T7),
>   la pantalla de salidas del asistente (T9) y sacar los cortes de
>   `media_asset.marcas_de_corte_ms` en el resolver.

**Objetivo:** que el motor elija el aire por prioridad de deck (manual >
comercial > programa > relleno) y que salga por `udp-ts` con lo que el
multiplexor de CAtv exige.
**Cierra:** F2-06, F2-07, F2-08, F2-15, F2-46, F2-47, F2-49, F2-50. (F2-48
se cierra del todo en T7, para salidas de internet; aquí solo aplica al
`udp-ts` local, que no necesita reconectar.)
**Toca:** `internal/drivers/salida/udpts.go` (nuevo), `internal/engine/encoder.go`
(adapta `Output` para aceptar PIDs/tsid/program/multicast/TTL — ver
VLC-PARIDAD.md fila "Opciones del mux TS"), `internal/app/motor.go` (decks y
prioridad, comparte archivo con T1 — coordinar tras fusionar T1),
`internal/store/channel.go` (`OutputRepo` completo).
**Depende de:** T1.
**Tamaño estimado:** ~900 líneas Go.
**Riesgos:** sin la cadena `sout` exacta de Rolando (pregunta abierta #2) se
adivinan PIDs/programa; construir con valores de ejemplo y dejarlos
configurables, no fijos.

### T3 · Detector de silencio y negro sobre la salida — **HECHA** (10 sept 2026)

> Rama `agente/t3-detector`, un solo commit. Lo construido está criterio por
> criterio en `docs/ACEPTACION.md` (F2-51, F2-52, F2-53, F2-54, F2-72, y la
> mitad de F2-30 que no necesita a T5).
>
> **Lo que T3 dejó puesto, y que T5 y T6 heredan:**
>
> - `engine.Detector`: un `engine.Sink` que se pone **entre** el servidor de
>   cuadros y el encoder y mide lo que de verdad sale. Cuenta por un canal de
>   estados (`negro_empieza` / `negro_sigue` / `negro_termina` y los tres del
>   silencio) y no decide nada. Si nadie lee el canal, el estado se tira y se
>   cuenta (`Detector.Perdidos`): **el aire no espera al vigilante**.
> - `App.Vigilar(ctx, formato, destino) engine.Sink` es el gancho, y es una
>   línea. **`motor.go` todavía no la tiene** (es de T1/T2): al fusionar hay
>   que cambiar en `correrMotor`
>   `engine.NewServer(formato, enc, …)` por
>   `engine.NewServer(formato, a.Vigilar(ctx, formato, enc), …)`.
>   La vigilancia vive lo que vive esa vida del motor; en sombra no se llama.
>   Está probado exactamente así en
>   `TestF2_51DePuntaAPuntaConUnClipNegroYMudo`.
> - `app.ControlDelAire` (`EnManual` / `VolverAlAutomatico`) y
>   `app.PonerControlDelAire`: el gancho que **T5** tiene que instalar desde
>   `manual.go` para que «avisa y devuelve el control» suelte el aire de
>   verdad. Es una variable de paquete, como `sostenerPorDefecto` de
>   `despierto.go`.
> - Tres ajustes nuevos, validados en la API y servidos siempre con su valor
>   de fábrica: `silencio_umbral_s` y `negro_umbral_s` (3 a 120 s, de fábrica
>   **15**) y `silencio_devuelve_control` (`si`/`no`, de fábrica `si`).
>   `Ajustes.tsx` los pinta; el interruptor ya no escribe `silencio_avisa`,
>   que nunca existió en el servidor.
> - **Corrección medida del umbral de negro.** El negro digital de un video en
>   rango limitado es luma **16 clavados**, así que «luma por debajo de 16» no
>   dispara nunca. El detector mide con los mismos números que el ingest ya le
>   pasa a ffmpeg: 98 % de la imagen por debajo de 25.5. Toda la explicación
>   está en F2-52 de `ACEPTACION.md` y en el comentario de `PixelNegro`.
> - Una línea en `app.go`: `"vigilancia"` al frente de `fuentesDeAlarma` (el
>   aire mudo manda sobre todo lo demás). **Ninguna** goroutine nueva en
>   `Start()`: la vigilancia no vive lo que vive la aplicación, sino lo que
>   vive el motor.

**Objetivo:** un solo mecanismo que vigile la salida real (no el plan) y
dispare alarma/incidente cuando hay silencio o negro de verdad, en
automático y en manual por igual.
**Cierra:** F2-51, F2-52, F2-53, F2-54, F2-72.
**Toca:** `internal/engine/detector.go` (nuevo), `internal/app/vigilancia.go`
(nuevo).
**Depende de:** T1 (necesita una salida real corriendo). Puede correr en
paralelo con T2 — no comparte archivos.
**Tamaño estimado:** ~400 líneas Go.
**Riesgos:** ninguno grande; es el más aislado de los diez.

### T4 · Fuentes en vivo
**Objetivo:** que un bloque en vivo (SRT/RTMP/entrada rápida) tome el aire a
su hora, con margen de gracia, reconexión y reloj de cortes.
**Cierra:** F2-18, F2-19, F2-20, F2-21, F2-22, F2-23, F2-24, F2-25, F2-26,
F2-74, F2-75, F2-76.
**Toca:** `internal/drivers/vivo/` (nuevo: `srt.go`, `rtmp.go`, `url.go`,
`captura.go`), `internal/app/vivo.go` (nuevo), `internal/model/model.go`
(añadir `LiveSourceTipoURL` a los tipos de `LiveSource.Kind` — hoy solo hay
`srt`/`rtmp`/`captura`, y VLC-PARIDAD.md dice que falta `url` para cuando
Antena787 tira de una fuente en vez de recibirla).
**Depende de:** T1, T2 (una fuente en vivo ocupa el deck programa con la
misma prioridad que un archivo).
**Tamaño estimado:** ~1,100 líneas Go.
**Riesgos:** pregunta abierta #4 (¿MistServer empuja o VLC tira?) decide si
`url` es imprescindible desde el día uno o puede esperar.

### T5 · Manual: tomar, soltar, un solo tenedor
**Objetivo:** que un operador tome el control, dispare a mano, y lo suelte
—por botón, por fin de bloque o por silencio— sin pisar a otro operador.
**Cierra:** F2-27, F2-28, F2-29, F2-30, F2-31, F2-32, F2-33, F2-34, F2-35,
F2-77, F2-78, F2-79.
**Toca:** `internal/app/manual.go` (nuevo, la máquina de estados de
`MANUAL_HOLD`), `internal/api/manual.go` (nuevo: tomar/soltar/parar todo),
`web/src/pantallas/AlAire.tsx` (panel de disparo, solo el cableado mínimo —
el tablero completo es T9).
**Depende de:** T1, T2 (el deck manual ya tiene prioridad 0 en el modelo).
**Tamaño estimado:** ~600 líneas Go + ~300 líneas TS.
**Riesgos:** F2-77 exige que nunca haya dos tenedores a la vez — la prueba
de concurrencia (dos pestañas pidiendo el control al mismo tiempo) es la
parte que más vale la pena escribir primero.

### T6 · Cascada, watchdog y resiliencia del proceso
**Objetivo:** que ningún fallo interno —encoder colgado, pánico de una
goroutine, disco lleno, base corrupta, reloj saltado, apagón— tumbe el aire
ni lo deje en negro sin decirlo.
**Cierra:** F2-11, F2-55, F2-56, F2-57, F2-58, F2-67, F2-68, F2-69, F2-70,
F2-71, F2-73, F2-86, F2-87, F2-88, F2-89, F2-90, F2-91, F2-92, F2-93, F2-94,
F2-95, F2-96, F2-97, F2-98, F2-99, F2-100, F2-101, F2-102.
**Toca:** `internal/app/watchdog.go` (nuevo), `internal/app/disco.go`
(adapta el `diskLoop` que ya existe para F1, con los umbrales 10/5/2 %),
`internal/app/db_integrity.go` (nuevo, corrupción en caliente además de al
abrir), `internal/store/migrate.go` (extiende `PRAGMA integrity_check`).
**Depende de:** T1, T2, T3 (el watchdog consume `Encoder.Done()` de T2 y la
distinción de alarmas de T3).
**Tamaño estimado:** ~1,800 líneas Go — la tanda más grande. Si un agente
solo no la termina en una sesión, se puede partir en "watchdog del encoder
y cascada" (F2-11, 67-73, 86-90) y "endurecimiento de disco/base/reloj"
(F2-55-58, 91-102) sin romper nada de este plan.
**Riesgos:** es la más grande y la que más toca `app.go` (cada `guard`
nuevo). Ver la nota de fusión en riesgos generales, abajo.

### T7 · Grabación, diferido, logo y salidas de internet
**Objetivo:** grabar la salida real sin huecos, resolver el diferido a
partir de esa grabación o de los archivos originales, poner el logo del
canal, y sacar la señal también por internet y `http-ts`.
**Cierra:** F2-36, F2-37, F2-38, F2-39, F2-40, F2-41, F2-42, F2-43, F2-44,
F2-45, F2-48 (el resto: reconexión de salidas de internet), F2-80.
**Toca:** `internal/app/grabacion.go` (nuevo), `internal/app/diferido.go`
(nuevo, sobre el `RuleTimeShift` que el modelo ya tiene), `internal/engine/overlay.go`
(nuevo: logo/marquesina — no hay número de criterio propio para el logo en
`ACEPTACION.md`; se verifica contra la fila "Marquesina y logo" de
`docs/VLC-PARIDAD.md` y contra la firma general de F2-113),
`internal/drivers/salida/internet.go` y `httpts.go` (nuevos),
`web/src/pantallas/AlAire.tsx` (ventana local, si la pregunta abierta #5 la
confirma necesaria).
**Depende de:** T1, T2.
**Tamaño estimado:** ~1,200 líneas Go + ~200 líneas TS.
**Riesgos:** el logo no tiene criterio numerado — que no se quede sin
verificar por no tener un F2-XX que lo reclame; se propone añadirlo a
`ACEPTACION.md` cuando se abra esta tanda.

### T8 · Telemetría y ENDEC como drivers de alerta
**Objetivo:** el retorno de aire real (`capture_input`), la comparación
contra el plan (`signal-compare`, modo `degradado` si no hay retorno), y el
ENDEC integrado según ADR 0010 (el corte absorbe el tiempo, nunca se
recorta el spot).
**Cierra:** F2-81, F2-82, F2-83, F2-84, F2-85, F2-111.
**Toca:** `internal/drivers/captura/` (nuevo: `receptor-tv.go`,
`tarjeta.go`), `internal/drivers/alerta/` (nuevo: `signal-compare.go`,
`endec-serial.go`, `endec-rele.go`), `internal/model/model.go` (nuevo:
`CaptureInput`, `AlertEvent`), `internal/store/` (nuevos repos con la
retención mínima de 24 meses de F2-85).
**Depende de:** T1, T2 (salida corriendo), T6 (catálogo de incidentes
compartido).
**Tamaño estimado:** ~700 líneas Go.
**Riesgos:** preguntas abiertas #7 (modelo del Sage) y las de VLC-PARIDAD
sobre `display`/HTTP — construir contra lo que responda el asistente al
escanear, no contra un modelo asumido (principio 1 del PRD).

### T9 · Tablero, Al aire real y firma de paridad con VLC
**Objetivo:** que Al aire enseñe la verdad completa —qué deck tiene el aire,
qué sale, alarmas, grabación—, que el asistente pruebe barras de verdad
(paso 5) y escanee la red antes de preguntar (paso 4), y que Rolando firme
que no le falta nada de lo que hacía con VLC.
**Cierra:** F2-59, F2-60, F2-61, F2-62, F2-63, F2-64, F2-66, F2-103,
F2-104, F2-105, F2-106, F2-107, F2-108, F2-113. (F2-65 se mide aquí y se
cierra en T10.)
**Toca:** `web/src/pantallas/AlAire.tsx`, `web/src/pantallas/Asistente.tsx`
(pasos 4 y 5 reales), `internal/api/tablero.go` (nuevo), `internal/api/instalacion.go`
(adapta: barras reales vía el motor en vez de la simulación de F1),
`internal/app/asrun.go` (nuevo: exportación del as-run).
**Depende de:** T1 a T8 — es la que junta todo en pantalla.
**Tamaño estimado:** ~500 líneas Go + ~900 líneas TS.
**Riesgos:** sin la respuesta a la pregunta abierta #2 (cadena `sout` de
Rolando) esta tanda no puede cerrar F2-113, que es la condición para apagar
VLC. Puede construirse la pantalla igual y dejar la firma pendiente.

### T10 · Prueba de resistencia de 30 días y puerta de cierre
**Objetivo:** correr el canal 30 días seguidos con instrumentación de
deriva A/V, y firmar el cierre de F2 con toda la evidencia junta.
**Cierra:** F2-01 (cierre final), F2-65, F2-66 (medición final), F2-109,
F2-110.
**Toca:** `docs/f2/SOAK-<fecha>.md` (informe, nuevo) y, si el analizador de
`internal/f0` no alcanza para 30 días de eventos, extenderlo (no es una
reescritura). Sin cambios de producto nuevos: esta tanda mide lo que las
nueve anteriores construyeron.
**Depende de:** T1 a T9, todas fusionadas y estables.
**Tamaño estimado:** ~200 líneas Go de instrumentación, más el tiempo de
correr el soak (no es trabajo de agente, es tiempo de reloj).
**Riesgos:** treinta días de calendario reales; cualquier regresión
encontrada a mitad del soak obliga a reiniciar el conteo, así que conviene
no arrancarlo hasta que T1-T9 estén, de verdad, quietas.

### Grafo de dependencias

```
T1 (motor y conformado)
 ├─→ T2 (decks + udp-ts)
 │    ├─→ T4 (fuentes en vivo)
 │    ├─→ T5 (manual)
 │    └─→ T7 (grabación, diferido, logo, internet)
 └─→ T3 (detector silencio/negro)
      └─→ T6 (cascada, watchdog, endurecimiento) ←── también depende de T2
           └─→ T8 (telemetría/ENDEC) ←── también depende de T2

T9 (tablero + firma VLC) ←── depende de T1, T2, T3, T4, T5, T6, T7, T8
T10 (soak 30 días) ←── depende de T9
```

**Paralelizable en la práctica:** después de fusionar T1, hasta cuatro
agentes pueden trabajar a la vez en T2 y T3; una vez T2 esté fusionada,
hasta cuatro más en T4, T5, T6 y T7 (T6 necesita T3 además de T2). T8 espera
a T6. T9 espera a todas. T10 espera a T9 y no se paraleliza: es una corrida.

---

## 3 · Contratos entre tandas

Firmas de Go que una tanda expone y otra consume. No son el diseño final —
son lo mínimo para que dos agentes que nunca se hablan entre sí construyan
piezas que encajen.

```go
// internal/engine — lo que T1 expone y T2/T4/T5 consumen.

// ClipSource decide qué sale después. engine no sabe qué es un plan_item,
// un deck ni una regla: solo pide "qué toca ahora" y "hasta cuándo puede
// asumirlo sin volver a preguntar". La implementa internal/app (T1/T2).
type ClipSource interface {
    // Next devuelve el clip que corresponde al instante now (reloj
    // monotónico del motor, no de pared — F2-90), y el instante en que hay
    // que volver a preguntar (el próximo corte conocido: fin de plan_item,
    // corte pautado, o "no sé, pregunta en 1s" si no hay nada resuelto).
    Next(now time.Time) (clip Clip, until time.Time, err error)
    // Filler es el relleno vigente cuando Next no tiene nada que ofrecer.
    Filler() Clip
}

// Server.Run pasa a recibir un ClipSource en vez de una Playlist fija.
// func NewServer(fmt Format, enc *Encoder, src ClipSource, filler Clip) *Server

// Salida activa (T2 la produce, T6 la vigila, T9 la enseña).
package salida // internal/drivers/salida

type Driver interface {
    // Abrir traduce model.Output (Params en JSON) a engine.Output, listo
    // para pasarle a StartEncoder.
    Abrir(fmt engine.Format) (engine.Output, error)
    // Vigilar corre hasta que ctx se cancele; cuenta reintentos y guarda
    // ConnectionState/Retries/LastError en el model.Output correspondiente
    // (F2-48/49). No bloquea el motor: corre en su propia goroutine.
    Vigilar(ctx context.Context, salida model.Output, listo <-chan error)
}

// Fuente en vivo (T4 la produce; T1/T2 la consumen igual que un Decoder).
package vivo // internal/drivers/vivo

type Driver interface {
    // Frames entrega cuadros ya en el formato de casa — mismo contrato de
    // canal que engine.Decoder.NextFrame, para que el frameserver no tenga
    // que distinguir un archivo de un vivo (F2-75: "no existe ruta cruda").
    Frames() <-chan []byte
    Samples(n int) ([]byte, int)
    // Conectada dice si hay señal ahora; el motor decide el margen de
    // gracia y el relleno (F2-19/20), el driver solo informa el hecho.
    Conectada() bool
    Cerrar()
}

// Retorno de aire (T8 lo produce; T9 lo enseña).
package captura // internal/drivers/captura

type Driver interface {
    // Comparar mide si lo transmitido de verdad corresponde a expect en el
    // instante at. degradado=true si no hay retorno de aire que consultar
    // (ADR 0009): en ese caso match no promete nada.
    Comparar(ctx context.Context, expect model.PlanItem, at time.Time) (match, degradado bool, err error)
}

// Retención manual del aire (T5 la produce; T9/API la consumen).
// internal/app/manual.go

func (a *App) TomarControl(ctx context.Context, operador string) error
func (a *App) SoltarControl(ctx context.Context, esperarClipActual bool) error
func (a *App) ParaTodo(ctx context.Context) error
func (a *App) EstadoManual() (activo bool, operador string, desde time.Time)

// Catálogo único de tipos de incidente (T1, T3, T4, T5, T6, T8 escriben
// aquí; ninguna tanda inventa un string suelto — evita que dos tandas
// llamen distinto a la misma cosa, o igual a dos cosas distintas).
// internal/model/incidentes.go

type TipoIncidente string

const (
    IncEncoderReiniciado   TipoIncidente = "encoder_reiniciado"
    IncVivoAusente         TipoIncidente = "vivo_ausente"
    IncManualPorTimeout    TipoIncidente = "manual_por_timeout"
    IncCascadaExtendida    TipoIncidente = "cascada_extendida"
    IncSolape              TipoIncidente = "solape"
    IncApagon              TipoIncidente = "apagon"
    IncSaltoDeReloj        TipoIncidente = "salto_de_reloj"
    IncEnlaceCaido         TipoIncidente = "enlace_caido"
    IncNormalizacionFallida TipoIncidente = "normalizacion_fallida"
    // … cada tanda añade la suya aquí, nunca como literal en su archivo.
)
```

**Regla de fusión para `internal/app/app.go`:** cada tanda que necesita una
goroutine nueva añade una línea `a.guard("nombre", a.xLoop)` dentro de
`Start()`. Es el único punto que todas las tandas tocan. Quien fusiona debe
revisar ese método a mano en cada integración — no es un conflicto que un
merge automático resuelva bien casi nunca.

---

## 4 · Puerta de cierre de F2

No se cierra F2 sin que todo esto esté marcado:

- [ ] **F2-109** — constancia registrada de que un humano leyó línea por
  línea el motor, el conformado y el watchdog (T1 + las partes de motor de
  T2 + T3 + T6 — unas 7,000 líneas, §22.3). Con fecha y quién.
- [ ] **Prueba de resistencia de 30 días** (T10) corrida en la máquina de
  destino, con deriva A/V acumulada medida (no estimada) y dentro de lo que
  F0 ya probó posible.
- [ ] **F2-65 y F2-66** — cero interrupciones no planificadas en los 30
  días, y el as-run contra la guía dentro de ±30 s en el 100% de los
  programas.
- [ ] **F0 corta pasando entera en las tres plataformas del CI** —
  Windows ya verde (issue #14, run 34394830791); confirmar macOS y Linux
  con el motor de F2 integrado (no solo el arnés de `f0`).
- [ ] **F2-113**, firmado por Rolando — cada opción de su cadena `sout` de
  VLC tiene equivalente en pantalla, el TP1000 recibe el TS sin cambiar
  nada de su lado, y él confirma que no le falta nada. **Sin esta firma, VLC
  no se apaga.**
- [ ] **F2-110** — al menos una persona no técnica armó una semana de
  parrilla sola, en menos de una hora, con seis preguntas o menos.
- [ ] **F2-108** — instalaciones abandonadas por debajo del 10%, medido
  sobre las instalaciones con soporte y anotado por paso.
- [ ] **Los 26 criterios [MANUAL] de F2** firmados uno por uno, no en
  bloque (F2-15, 25, 33, 45, 55, 67, 76, 77, 82, 100, 103-110, 113).
- [ ] **F2-63** revisado contra la lista real de tipos de incidente en
  producción: cada tipo se distingue de los demás, sin dos incidentes
  contando la misma cosa con nombres distintos.
- [ ] `docs/ACEPTACION.md` con cada F2-01 a F2-113 marcado con su estado
  real (hecho / diferido / no aplica), igual que se hizo con F1.
- [ ] `CONTINUAR.md` actualizado con dónde quedó cada tanda y qué sigue.

---

## 5 · Preguntas abiertas — decidir con Saul antes de cada tanda

1. **F2-91 a F2-102** (disco, base, corriente) llevan numeración F2- pero
   su encabezado en `ACEPTACION.md` dice "(F2.5, §19)". ¿Cuentan para la
   puerta de cierre de F2, o solo para la de F2.5? Decide si T6 puede
   cerrarse sin ellas.
2. La cadena `sout` exacta de VLC de Rolando (o el `.vlm`/`.xspf`, o el
   atajo de Windows) — sin ella, T2 y T9 adivinan PIDs y programa. ¿Ya se
   pidió (`docs/VLC-PARIDAD.md`, pregunta 1)?
3. ¿El TP1000 recibe unicast a una IP y puerto, o multicast, y con qué TTL?
   Decide si T2 construye multicast desde el día uno o detrás de una
   bandera que nadie prueba todavía.
4. ¿MistServer empuja a VLC, o VLC tira de MistServer? Decide si
   `live_source.tipo = url` (que hoy no existe en el modelo) es obligatorio
   para T4 desde el principio.
5. ¿Rolando usa el `display` de VLC para ver la salida en el monitor de la
   torre? Decide si la ventana local de T7 es paridad obligatoria o se
   puede posponer.
6. ¿Alguien más tira de la señal por HTTP hoy, desde otro equipo? Decide
   la urgencia de `http-ts` en T7.
7. Modelo exacto del Sage ENDEC (1822 o 3644) y camino de telemetría
   confirmado (serial, relés, red) — T8 necesita al menos un candidato para
   no construir tres drivers a ciegas.
8. ¿Quién más, además de Saul, puede firmar la lectura línea por línea de
   F2-109? Define si 7,000 líneas es una tarea de una persona o de dos.
9. ¿El soak de 30 días (T10) corre en la Mac de desarrollo, en una máquina
   equivalente a la de Rolando, o ya en la PC real de CAtv? El criterio
   original pide "la máquina de destino".
10. ¿Se construyen varias salidas simultáneas desde T2 (F2-46/47, cada una
    con su volumen) o se empieza con una sola `udp-ts` y la segunda llega en
    T7? Afecta directamente el tamaño de T2.
