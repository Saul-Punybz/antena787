# CONTINUAR — dónde quedamos y qué sigue

_Última sesión: 11 de septiembre de 2026 (la tanda del rescate: cinco ramas de agentes, cuatro fusionadas). Siguiente: la cadena `sout` de Rolando, y el watchdog de 3 s (F2-11)._

## Lo último · 11 de septiembre de 2026 · la tanda del rescate

La sesión empezó con tres worktrees que tenían horas de trabajo **sin un solo
commit** en sus ramas. Se rescataron los tres antes de construir nada nuevo, y
cada agente commiteó y pusheó su propia rama antes de reportar. La fusión la
hizo el orquestador, que se reservó `web/src/lib/api.ts`, `tipos.ts` y el
router para que dos worktrees no inventaran el mismo nombre por separado.

**Lo que entró en `main`:**

1. **La puerta del aire** (`agente/salir-de-sombra`). Murió el
   `nuevo.Mode = "sombra"` de `internal/api/estado.go`, la línea de F1 que
   mantenía apagado todo lo que se construyó para encenderlo. `PUT /canal` ya
   **no toca el modo nunca** —un cliente rancio reencendería el transmisor con
   un modo viejo—; la transición vive en `POST /canal/al-aire` y `/a-sombra`
   (`internal/api/aire.go`, `internal/app/aire.go`), con seis comprobaciones
   previas (ffmpeg · salidas · que las salidas abran · plan de 30 min ·
   cobertura · retorno), 409 con la lista entera cuando falta algo, y
   confirmación escrita. Volver a sombra cancela el ctx del encoder, así que
   apagar apaga de verdad, y una parada a propósito ya no cuenta como caída.
   El botón vive en `web/src/componentes/PuertaDelAire.tsx`. Criterio F2-118
   nuevo. **Dos tipos de incidente nuevos** heredados del diff: `al_aire` y
   `a_sombra` — Saul decide si se quedan.
2. **La pantalla de salidas** (`agente/pantalla-salidas`).
   `web/src/pantallas/Salidas.tsx`, 678 líneas contra la API que T2 dejó
   completa y sin cliente. Con esto el canal **se puede apuntar a la TP1000**,
   que era la otra mitad del P0: un interruptor para salir de sombra sin sitio
   donde escribir una IP no sirve de nada. Sin valores precargados a propósito,
   porque la cadena `sout` de Rolando sigue sin respuesta.
3. **Los dos bugs de contrato** (`agente/contrato-arreglos`). El importador de
   la hoja de CAtv mandaba `regla_que_vence`/`regla_que_releva` y la pantalla
   leía `regla`/`releva_a`: cada clic en «confirmar relevos» contestaba 400.
   Alineado, con la prueba del viaje entero hoja→propuesta→confirmación. Y
   **la prueba de contrato ahora compara tipos, no presencia de clave**, que
   era el agujero por el que ese bug vivió dos días. Ajustes deja de leer
   **27 claves** que ningún `.go` manda; las tarjetas de salud y respaldo
   explican por qué no hay dato en vez de fingirlo. La sesión heredada había
   intentado lo contrario —inventar las claves y construir el respaldo para
   alimentarlas—; se revirtió.
4. **`internal/ts` de 0% a 100% de cobertura** (`agente/pruebas-que-faltan`).
   Sostiene F0-04 y F2-114, y no tenía una sola prueba. Se auditó mutando
   `ts.go` en tres rutas para confirmar que las aserciones revientan de verdad.
   Y se arregló `TestAbrirCerrarDetieneLaReconexion` en `hdhomerun`, que
   **nunca llegaba a su aserción**.
5. **El Resumen de `docs/ACEPTACION.md`** decía «F2: 110 · total 184» con la
   lista trayendo F2-01 a F2-118. Contado sobre el archivo: **F0 9 · F1 80 ·
   F2 118 · total 207**, de ellos 176 `[AUTO]`, 30 `[MANUAL]` y **uno `[DOC]`**
   (F1-74), una tercera etiqueta que el Resumen nunca mencionó.

**Lo que la fusión destapó, y es la lección de la tanda:** la pantalla nueva
puso `main` en rojo en dos criterios de F1 que ninguna prueba de la rama
detectaba — **F1-57** (el menú tiene que traer cinco entradas, seis con el
primer anunciante) y **F1-56** (la lista cerrada de jerga prohibida del
principio 3 del PRD: `driver`, `códec`, `GOP`, `LKFS`, `transport stream`).
Decisión del orquestador: **«Salidas» sale del menú principal** —configurar a
dónde va la señal es instalación, no operación diaria— y se entra desde
Ajustes, conservando la ruta. La jerga se arregla renombrando identificadores
en la capa de interfaz (`driver` → `tipo`), dejando `driver: tipo` y
`salida.driver` en la frontera de la API, y reescribiendo los tres textos que de
verdad la enseñaban —incluido uno que decía «el driver no se le enseña a nadie»
usando la palabra que prometía no usar—. Fusionado desde `agente/jerga-y-menu`.
**Verificado en navegador** (Playwright, modo demo): menú de cinco entradas,
entrada desde Ajustes, y crear / cambiar / borrar una salida, que era justo lo
que nunca se había probado.

**Lo que se corrigió del plan:** el **monitor de salida (F2-117) no cabe en una
tanda de pantalla.** Pide servir la salida real por `http-ts`/HLS con menos de
3 s de retraso, y `http-ts` **no existe**: está marcado como T7 en
`internal/drivers/salida/salida.go:9`. Necesita el driver antes que la vista.
El estimado de «1 tanda» de `docs/auditoria/07-desde-cero.md` §3 estaba mal.

**Lo que sigue, en orden:**

1. **Mandar los bloques 1 y 2 de `docs/PARA-ROLANDO-WHATSAPP.md`.** Es gratis,
   es una acción humana, y es el punto de más apalancamiento del proyecto: sin
   la cadena `sout` los PIDs de T2 y los campos de la pantalla nueva siguen
   siendo valores de ejemplo.
2. **Watchdog de 3 s sobre el encoder y cascada a relleno** (F2-11). Es el
   código que sostiene el aire cuando Rolando lo rompe.
3. **Tomar y soltar el control a mano sin quedarse pegado** (T5).
4. **Grabar lo que salió** (`air_recording` existe y está vacía).
5. **Corregir `docs/PARA-ROLANDO.md`**, que hoy vende la pantalla «Anuncios» y
   los 518,400 segundos vendibles, justo lo que se pospuso el 11 de
   septiembre. Una expectativa mal puesta cuesta más que una función que falta.
6. El monitor (F2-117) **después** del driver `http-ts` de T7.

**Cómo se corrió esta tanda** quedó escrito como skill reutilizable:
`~/Desktop/PunyOS/06_Claude/skills/obra/` (`/obra arranque` · `/obra mitad` ·
`/obra cierre`) — ocho invariantes, precios por modelo, y las seis pasadas de
verificación de cierre con sus comandos.

**Nota sobre el presupuesto, para la próxima sesión de Claude Code.** Esta tanda
se repartió con un techo de **$30 / 2 horas**, pero Saul está en la **membresía
de Claude de $100 al mes**: ahí el uso va contra los límites del plan y **no se
descuentan credits**. Los dólares de esta obra son **equivalencias en precio de
lista de la API** —una vara para decidir cuántos agentes y de qué tamaño caben
en una hora—, no un cargo. La vara sirve porque las dos formas de pagar se
cobran sobre lo mismo, tokens, así que bajar de modelo en lo no esencial, dar
rutas en vez de mandar a descubrir, no hacer fan-out de solo lectura y poner
techo a la salida reducen las dos a la vez. El gasto real solo se ve en la
cuenta (uso del Claude Console si es cuenta de API con credits, vista de uso del
plan si es la suscripción); no lo inventes, y no le digas «gastamos $X de
credits» estando en la membresía. Detalle en el skill,
`references/presupuesto.md` § «Léelo primero».

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
   **T3 hecha (10 sept, integrada):** detector de silencio y negro sobre la
   salida real (`internal/engine/detector.go`, `internal/app/vigilancia.go`),
   enganchado en `correrMotor`; umbral de fábrica 15 s (F2-53); «avisa y
   devuelve el control» cableado con gancho para T5 (`app.ControlDelAire`).
   **Decisión pendiente de Saul:** el negro digital en rango limitado es
   luma 16 exactos, así que «luma < 16» (PRD §14.1, F2-52) nunca se cumple;
   el detector mide como el ingest (≥ 98 % de la imagen bajo 25.5) y F2-52
   lo explica; si se prefiere otro número es una constante.
   **Integrado el 11 sept (todo en `main`, CI por confirmar):** **T2**
   (decks y prioridad, salida `udp-ts` al multiplexor con PIDs/PCR/tsid/
   multicast, varias salidas a la vez, `internal/drivers/salida`; prueba de
   punta a punta que lee el TS por UDP y mide tasa 0.00 % de desvío, PCR
   30.5 ms, CC 0; F0 corta 0 FALLA), y cuatro bibliotecas de protocolo:
   `internal/drivers/alerta/sage` (protocolo del ENDEC por serial y TCP,
   contra el manual Rev 1.5), `internal/drivers/alerta/same` (decodificador
   SAME, 0.5 ms por segundo de audio, cero asignaciones, lee hasta −4 dB de
   SNR), `internal/drivers/captura/hdhomerun` (retorno de aire por HTTP),
   `internal/drivers/senal/scte104` (cortes hacia el inyector: 92.7 % de
   cobertura, escrito contra las dos únicas implementaciones libres que
   existen, porque SCTE publica el estándar solo tras registro) y
   `internal/resolver/pmcp.go` + `/guia.pmcp` (guía para el generador PSIP).
   Todo compila sin CGo en Windows, Linux y macOS.
   **Dos cosas que decidir, chicas:** (a) el decodificador SAME marca
   `Confianza` 1/3 cuando solo una de las tres repeticiones se leyó limpia
   —a −6 dB de SNR una de cada diez sale con un carácter mal—; hay que
   decidir si el as-run exige 2/3 para escribir la línea sin asterisco;
   (b) la cabecera SAME va en UTC y el texto del ENDEC en hora local, ya
   resuelto en el driver, pero conviene que T8 no lo olvide; (c) en SCTE-104,
   `inject_section` es el único mensaje cuyo orden de campos no se pudo
   cotejar con nadie —si un inyector lo rechaza con `result 115`, ahí hay que
   mirar primero— y el puerto 5167 sale de un datasheet de fabricante, no de
   IANA: confirmarlo con el ingeniero de CAtv.
   **Ramas de agentes en marcha al guardar (10 sept):** `agente/t2-decks-udpts`
   (T2), y cinco bibliotecas de protocolo aisladas que no tocan `app` ni
   los criterios: `agente/sage-endec` (`internal/drivers/alerta/sage`,
   protocolo serial y TCP del ENDEC), `agente/same`
   (`internal/drivers/alerta/same`, decodificador SAME solo lectura),
   `agente/scte104` (`internal/drivers/senal/scte104`), `agente/pmcp`
   (`internal/resolver/pmcp.go`, `/guia.pmcp`), `agente/hdhomerun`
   (`internal/drivers/captura/hdhomerun`). Worktrees en
   `../antena787-wt/`; `git worktree list` dice cuáles siguen. Integración:
   rebase sobre `main`, `--ff-only`, suite completa, push; una rama sin
   commit se relanza con el mismo encargo (resumido aquí).
   **T2** va bajo tres decisiones por defecto que Saul puede cambiar: (1) F2-91 a F2-102 (F2.5) no bloquean el cierre de F2; (3) el
   TP1000 recibe unicast IP:puerto por defecto y multicast+TTL es opción del
   mismo driver; (10) varias salidas simultáneas desde T2, solo `udp-ts` y
   `archivo` (internet y `http-ts` en T7). PIDs/programa/bitrate son valores
   de ejemplo configurables hasta tener la cadena `sout` de Rolando.
8. Después de F2 (PRD §22.3), donde además se cierran F1-04
   (CEA-608 con detección propia, ffprobe ≥ 9 ya no emite `closed_captions`),
   el fundido de 1 s del clip recortado (`fundido_salida_ms` ya viene en el
   plan) y los ítems `dentro_de` de un vivo.

## Lo que Saul preguntó el 11 sept y no tiene dónde vivir

Repaso hecho contra el código, no contra el PRD. Tres grupos:

**Existe, pero a medias.**
- Nombre del canal, identificativo, comunidad de licencia y zona horaria se
  ponen en el paso 1 del asistente y se ven en Ajustes → Canal, **en solo
  lectura**: no se pueden corregir después sin rehacer la instalación. Deben
  ser editables.
- El **modo demo** existe (`VITE_DEMO=1`, o cae solo si no hay servidor) pero
  no hay forma de entrar a propósito desde la interfaz. Falta un «ver un
  ejemplo» visible, que es lo que se enseña a un cliente.
- La puerta por donde entra el video de un anunciante **sí existe en el
  servidor** (`portal/entrada` → `acceptFromPortal`: valida sin red y con
  tope de tiempo, y solo si pasa lo mueve a la carpeta de contenido, de donde
  entra a la biblioteca como cualquier otra cosa). Lo que no existe es el
  portal que suba ahí.

**Diseñado, con fase asignada.**
- **Anuncios y pagos**: el menú ya tiene su entrada (aparece con el primer
  anunciante) y hoy lleva a una pantalla «por hacer». Es F4: portal del
  anunciante con enlace propio y fecha, pago o cobro en mano, subida del
  video, aprobación antes del primer aire, y reporte de ingresos.
- **Logo al aire**: la tabla `overlay` (tipo `logo`) está en el esquema desde
  F1; ponerlo en la señal es F2 (T7).

**No existe en ninguna parte. Son huecos reales.**
1. **Dónde escribir IPs, puertos y direcciones. El más urgente.** El paso 4
   del asistente pregunta el *tipo* de destino en lenguaje llano («al equipo
   que junta los canales, por el cable de red») pero **no pide la dirección**.
   T2 ya construyó toda la API (`/api/v1/salidas`, con IP, puerto, multicast,
   TTL, PIDs, programa, tsid, PCR, bitrate y códec de audio), pero su
   pantalla está asignada a **T9, al final del plan**. Hay que adelantarla:
   sin ella no se conecta nada ni se puede probar en casa de Rolando.
2. **El nombre del puerto serial** del ENDEC: tampoco hay dónde. Y como no
   enumeramos puertos (la parte de `go.bug.st/serial` que lo hace usa CGo),
   se escribe a mano: `COM3`, `/dev/ttyUSB0`. La pantalla tiene que decirlo
   con un ejemplo por sistema.
3. **Dueño o licenciatario del canal**: no hay campo. En una estación con
   licencia es un dato que se pide en todos los formularios.
4. **Términos de servicio y política de privacidad**: no existen. El repo
   tiene LICENSE, CONTRIBUTING y código de conducta, que son para quien
   contribuye al software, no para el operador ni para el anunciante. Hacen
   falta sobre todo en el portal del anunciante, que es la única pantalla
   pública y donde alguien paga.
5. **Ayuda dentro del producto**: no hay pantalla de «cómo se usa esto», solo
   textos sueltos en Ajustes.
6. **Logo de la estación en la interfaz** (la marca del canal en pantalla, no
   al aire): no existe.

## Cosas pequeñas pendientes

- ~~Hueco de producto que vio Saul (11 sept): a la Parrilla se le puede mover
  contenido, pero no añadir.~~ **Hecho (11 sept):** criterios **F1-78** [AUTO],
  **F1-79** y **F1-80** [MANUAL]. Decisión de Saul: la parrilla **no** acepta
  poner contenido directo —sigue siendo consecuencia de las reglas— y **no
  existe `POST /plan`**; lo que gana son atajos y ver el inventario sin cambiar
  de pantalla. Lo construido, todo en la interfaz:
  1. **La franja vacía es un `button`** con `aria-label` y foco visible; al
     tocarla se abre el editor de regla con el **día en el patrón** y la
     **hora redondeada a la media hora** de donde se tocó, la fecha de inicio
     de ese día y la duración más grande que quepa. La fecha de fin se deja en
     blanco: de ella salen los avisos de vencimiento.
  2. **«Escoger yo»** ya tiene acción: el primer vacío de una hora o más del
     fin de semana. Y el botón «Regenerar la guía» de Parrilla · Guía, que
     también estaba muerto, ahora recalcula y vuelve a leer la guía. La prueba
     `TestF1Verif78NingunBotonMuertoEnLaParrilla` lo vigila.
  3. **Biblioteca al lado** (`componentes/BibliotecaAlLado.tsx`): columna
     plegable, plegado recordado en `localStorage`, buscador, y **primero lo
     que no está programado** con su conteo. La tarjeta de un título ya
     programado lleva «ver su regla» → `/reglas?titulo=…`.
  4. **Vista por Día** (`pantallas/ParrillaDia.tsx`, `/parrilla/dia`): el día
     de emisión hora por hora, de qué regla sale cada bloque, y los vacíos con
     el mismo atajo. Las cuatro vistas son ahora una fila de pestañas donde la
     abierta se ve (`.pestanas`). **Semana sigue siendo la vista por defecto**
     de `/parrilla`: es la que enseña el aire vacío comparado entre días, que
     es de donde salió el problema.
  5. El `EditorDeRegla` salió de `Reglas.tsx` a `componentes/EditorDeRegla.tsx`
     y acepta valores iniciales; Reglas lo usa igual y además entiende
     `/reglas?titulo=…`.
  Lo que **no** se hizo, a propósito: arrastrar de Biblioteca a la Parrilla
  (haría falta `POST /plan`) y la salida «llenar con relleno/diferido» desde el
  hueco (ya existe el botón «Llenar el fin de semana»).

- **Segundo hueco de la misma familia (Saul, 11 sept): no hay dónde dar de
  alta una fuente en vivo.** La regla de tipo `vivo` existe (Reglas tiene el
  interruptor «Es una fuente en vivo» y `live_source_id`), la tabla
  `live_source` existe con todo lo suyo (punto de escucha SRT/RTMP, duración
  prevista, relleno de respaldo, reloj de cortes, retardo de 7 s), pero **no
  hay pantalla ni ruta de API** para crearla: hoy solo nace desde el
  importador de hojas (`internal/api/importar.go`). O sea que no hay dónde
  escribir la dirección SRT de un noticiero. Falta `GET/POST/PUT/DELETE
  /api/v1/vivos` y su pantalla (o una sección en Reglas, que es donde se
  usa). El motor que de verdad las toma es T4 de F2, pero el alta se puede
  construir antes y conviene, porque sin ella T4 no se puede ni probar a
  mano.

- **Catálogo de equipos y bibliotecas** (`docs/drivers/CATALOGO.md`, 10 sept):
  lo que T8 necesita antes de escribir drivers. Decisiones que deja: relés
  por serial (no HID), HDHomeRun como retorno de aire, PMCP para PSIP (F5),
  SCTE-104 y SAME se escriben a mano, Sage 3644 syslog/SNMP por confirmar.
  Las preguntas para el ingeniero de CAtv están juntas ahí.

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

## La prioridad, fijada por Saul el 11 de septiembre de 2026

**Esto es para Rolando**, que corre un canal chico de televisión. Es el
cliente, y es una persona. Lo que hace falta: *«lo más cercano a funcional y
completo para que él lo pueda utilizar y romper»*.

**Tiene prioridad, en este orden:** que el sistema funcione · que **conecte** ·
que **envíe, reciba y transmita** · que **se vean los videos** · que **se
comunique con los equipos** · que **lo transmitido quede salvado** (grabación)
· y el streaming, que se vea.

**No tiene prioridad:** el dashboard del cliente, pagar los anuncios y
subirlos. El portal del anunciante y los pagos (F4) quedan **pospuestos por
decisión**, no olvidados: no van en el camino crítico.

Dos consecuencias:

1. **«Que lo pueda romper» sube la resistencia al mínimo.** La cascada, el
   watchdog, nunca negro ni silencio, nada manual que se quede trabado: deja
   de ser el final del plan y pasa a ser parte de lo que hay que entregar.
   Un sistema que solo aguanta cuando todo va bien no se le da a alguien
   para que lo rompa.
2. **El eje es el camino de la señal, de punta a punta:** entra (archivo o
   vivo) → se ve en pantalla → sale al multiplexor y a internet → se graba →
   se comprueba en el retorno que salió. Cada tanda dice qué parte del camino
   completa. La pantalla de conexiones va primera: sin ella no hay «conecte».

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
