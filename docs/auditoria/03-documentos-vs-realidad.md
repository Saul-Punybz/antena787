# Auditoría: lo que dicen los documentos contra lo que hace el código — 11 sept 2026

Alcance: los siete documentos que el enunciado pide, contra el código real
tras la integración de hoy (T1-T3 de F2 más los paquetes `scte104`, `sage`,
`same`, `hdhomerun`, `pmcp`). Solo lectura: no se tocó ni una línea de código
ni de documento. Verificado con `grep -n`, lectura directa de archivo y
`CGO_ENABLED=0 go build ./...` para `darwin/arm64`, `linux/amd64` y
`windows/amd64` (los tres compilan limpio, sin CGo).

## 1 · `docs/ACEPTACION.md` — criterios "Construido en Tx" contra el código

Los 24 criterios con cita de archivo:función en F2 (líneas 581-1170,
motor/detector/salidas) se verificaron uno por uno con `grep -n "func"` sobre
el archivo citado. Las 13 citas más concretas (`frameserver.go:emitir`,
`decoder.go:videoFilter`/`audioFilter`, `motor.go:queToca`/`tomaElAire`/
`posicionDe`/`correrMotor`, `frameserver.go:AirTime`, `salidas.go:
App.abrirSalidas`, `detector.go:mirarAudio`/`mirarCuadro`, `vigilancia.go:
abrirEpisodio`/`negroPermitido`, `salida.go:Ganancia`/`Disponibles`) existen
tal cual, con la firma que el criterio implica. **Veredicto: al día.**

| documento:línea | lo que dice | código:línea | veredicto | evidencia |
|---|---|---|---|---|
| ACEPTACION.md:1010-1038 (F2-52) | luma bajo 16, con nota "Corrección medida del umbral": el detector mide 98 % de la imagen bajo 25.5 | detector.go:50-76 (`PixelNegro = 0.10*255`, `ParteNegra = 0.98`) | al día | El propio criterio ya documenta la corrección; ver §2 para el PRD, que no la tiene. |
| ACEPTACION.md:945 (F2-46) | "contrato `Driver{Abrir, Vigilar}`" | salida.go:54-66 — la interfaz real tiene **tres** métodos: `Abrir`, `Vigilar`, `Descripcion` | **contradicho** | `Descripcion() string` existe en el código y se usa (pantalla/bitácora), pero no aparece en la cita del criterio. |

**Lo construido hoy sin criterio que lo reclame.** `grep -ni "scte\|sage\|hdhomerun\|pmcp\|SAME\b\|endec"` sobre `ACEPTACION.md` solo encuentra la sección "Retorno de aire" (F2-81 a F2-84, sin `Construido en`, es decir siguen sin cerrar) y una mención de "ruta SCTE alterna" como fuera de alcance. **Ninguno** de los cinco paquetes que entraron hoy tiene un criterio propio:

| Paquete construido hoy | Líneas aprox. | Criterio que lo reclama en ACEPTACION.md |
|---|---|---|
| `internal/drivers/senal/scte104` (protocolo.go, mensajes.go, cliente.go) | ~600 | ninguno — el PRD §9 paso 7 y el ADR 0004 sí lo citan, ACEPTACION.md no |
| `internal/drivers/alerta/sage` | ~300 | ninguno |
| `internal/drivers/alerta/same` (decodificador SAME) | ~400 | ninguno |
| `internal/drivers/captura/hdhomerun` | ~250 | ninguno (F2-81/82/84 existen pero sin "Construido en", así que tampoco lo reclaman aún) |
| `internal/resolver/pmcp.go` + `/guia.pmcp` | ~150 | ninguno |

Esto es coherente con `docs/f2/PLAN-F2.md` (T8, aún no "HECHA") y con
`docs/auditoria/01-codigo-muerto.md`, que ya señaló que estos cinco paquetes
son piezas aisladas a propósito, cableadas por T8. **No es una contradicción
del código: es que el propio ACEPTACION.md aún no llegó a nombrarlas.**

**Los conteos del Resumen (ACEPTACION.md:1404-1407), recontados a mano:**

| Fase | Dice el Resumen | Cuenta real (`grep -c "^\- \*\*Fx-.*\*\* \[TAG\]"`) | Diferencia |
|---|---|---|---|
| F0 | 9 (F0-01 a F0-09) | 9 | igual |
| F1 | **76** (F1-01 a F1-76) | **77** (F1-01 a F1-77; F1-74 existe, línea 528, tag `[DOC]` — un tag que el Resumen no contempla) | **+1** |
| F2 | **110** (F2-01 a F2-110) | **117** (F2-01 a F2-117) | **+7** |
| Total | **184** | **203** (9+77+117) | **+19** |
| AUTO / MANUAL | 158 / 26 (suma 184) | 174 / 28 (más 1 `[DOC]`) = 203 | +16 / +2 |

El propio Resumen ya se contradice consigo mismo: la misma línea 1404 dice
"76 criterios (F1-01 a F1-76)" y en la misma frase enumera "F1-74 a F1-77" —
que llega hasta el 77 que la cabecera niega. Y 9+76+110 tampoco da 184 (da
195): el error no es solo de conteo contra el código, es de aritmética
interna. **Veredicto: desfasado, y con una contradicción interna propia.**

## 2 · `PRD.md`

| PRD.md:línea | lo que dice | código:línea | veredicto | evidencia |
|---|---|---|---|---|
| 422, 454 | "luma bajo 16 durante más de 15 segundos" (detector de negro sobre la salida, §9 paso 4/6) | `internal/engine/detector.go:50-76` — mide **98 % de la imagen por debajo de 0.10×255=25.5**; `LumaNegra=16` solo se enseña en la alarma, no decide nada | **contradicho** | Un clip `color=c=black` mide luma media 16.000 clavados, así que "bajo 16" nunca se cumple (nota del propio detector, líneas 53-57, y ya corregida en ACEPTACION.md:1030-1038). El PRD no se corrigió. |
| 1345 | fila del ingest: "luma media bajo 16" | `internal/ingest/blacksilence.go` usa `blackdetect=pic_th=0.98:pix_th=0.10` (citado en detector.go:44-46), no una media simple | desfasado | Mismo error de fondo que arriba, en el otro lado del pipeline. |
| §10 (827-937) | catálogo completo de familias de drivers (salida, entrada vivo, cortes, alertas, retorno de aire, transmisor, respaldo, avisos, cobro) | `internal/drivers/` tiene hoy: `salida/` (udp-ts, archivo), `senal/scte104/`, `alerta/sage`, `alerta/same`, `captura/hdhomerun` | al día como diseño | El PRD no dice "construido"; es catálogo de v1 completa. Coincide con lo que existe (subconjunto) y con lo que falta (`internet`, `http-ts`, `gpi-*`, `dasdec`, transmisor, respaldo, cobro). No hay afirmación falsa, solo trabajo pendiente ya reconocido en PLAN-F2 T4/T7/T8. |
| §9 paso 7 (631-644) | "SCTE-104 al encoder, y el encoder genera el SCTE-35" (ADR 0004) | `internal/drivers/senal/scte104/protocolo.go:1-6` cita literalmente "ADR 0004, PRD §9 paso 7" | al día | El comentario del código referencia el PRD por número de paso. |
| §9 paso 6 (593-597) | escalera del regreso manual: "ámbar a 60 segundos", "rojo" en "los últimos 10 segundos" | Ningún `ambar`/`60`/`10` de escalera en `internal/app/vigilancia.go` ni en `web/src/pantallas/AlAire.tsx` | sin construir (no contradicho) | F2-33 (ACEPTACION.md:828) es `[MANUAL]` y no tiene "Construido en Tx": es diseño acordado, todavía sin UI. No es una afirmación falsa, es trabajo de T5/T9 no hecho aún. |
| §12 (1056-1120) | perfil `us-fcc`, ajuste de subtítulos de tres estados | `internal/app/cumplimiento.go:44-50`, `internal/api/instalacion.go:478`, probado en `internal/api/cumplimiento_test.go` y `api_test.go:817` | al día | El código exige literalmente `RegProfile == "us-fcc"` donde el PRD lo pide. |

## 3 · ADR contra el código de hoy

| ADR | decisión | código | veredicto |
|---|---|---|---|
| 0002 (sin CGo) | cross-compila los tres SO sin CGo | `CGO_ENABLED=0 GOOS={darwin,linux,windows} go build ./...` — **los tres, limpios, sin error**; `grep -rn "CGO"` solo aparece en un comentario que lo confirma (`transporte.go:22`) | al día |
| 0004 (SCTE-104 al encoder) | Antena787 nunca escribe SCTE-35, se lo pide al encoder | `scte104/protocolo.go` construye `splice_request_data` (MopCorte) y `mensajes.go`; ninguna función escribe secciones SCTE-35 al TS | al día |
| 0009 (verdad = señal transmitida) | grabación y `signal-compare` leen el retorno de aire, nunca la salida propia | `hdhomerun.go:1-9` se declara explícitamente "driver de retorno de aire (`capture_input` tipo stream, ADR 0009)"; detector.go (negro/silencio) sigue leyendo la salida propia, como el ADR permite (son preguntas distintas) | al día |
| 0010 (integrar el ENDEC, nunca reemplazarlo) | Antena787 nunca genera SAME ni tonos; solo decodifica lo que el ENDEC ya dijo | `sage.go:1-25` y `same.go:1-14` lo declaran igual, palabra por palabra ("nunca genera una cabecera SAME", "solo decodifica"); el modulador de pruebas vive fuera del binario (`same_test.go`, no `same.go`) | al día |
| 0001, 0003, 0005-0008 | motor propio con encoder persistente, un solo binario externo (ffmpeg), AGPL+DCO, soporte no funciones, IA solo por MCP, nunca interrumpir el aire | nada de lo integrado hoy (SCTE-104, EAS, HDHomeRun, PMCP, decks) los toca; ningún modal nuevo, ninguna dependencia C nueva, ningún cobro por función | al día, sin cambios que los afecten |

## 4 · Drivers: `README.md`, `CATALOGO.md`, `catalogo/*.md`

| documento:línea | lo que dice | código | veredicto |
|---|---|---|---|
| README.md:25 | `captura/hdhomerun` fechado "(10 sept)" | commit `368d53b`, 2026-09-11 06:48 | **desfasado** (fecha equivocada por un día; menor) |
| CATALOGO.md:39-42 | "Antena787 **necesita** un exportador PMCP además de `/guia.xml` (F5...)" — lo trata como trabajo futuro | `internal/resolver/pmcp.go` existe, probado, y servido en `GET /guia.pmcp` (commit `be54746`, el mismo 11 sept, después de que se escribiera CATALOGO.md el 10) | **desfasado** — construido el mismo día en que el catálogo lo daba por pendiente |
| README.md:24-33 | "solo una familia [`salida`] tiene interfaz común"; `alerta/sage` y `captura/hdhomerun` son "paquetes de un equipo concreto... sin nada que los generalice todavía" | Confirmado: no hay `type Driver` en `internal/drivers/alerta/` ni `internal/drivers/captura/` (solo el subpaquete `hdhomerun/`); coincide con `docs/auditoria/01-codigo-muerto.md` | al día |
| CATALOGO.md:53-56 (item 6) | corrige a README.md: "syslog y SNMP no aparecen en ninguna fuente pública... `README.md` los daba por hechos y ya no" | `README.md:143-145` **ya** dice "syslog y SNMP no confirmados" | al día — la corrección que CATALOGO.md anuncia ya está aplicada en README.md; no es la contradicción que el propio texto insinúa (puede ser que README.md se corrigiera después de escribir esa frase en CATALOGO.md) |

## 5 · `docs/VLC-PARIDAD.md` contra `udp-ts` real (T2)

Filas de la tabla §1 verificadas contra `internal/drivers/salida/udpts.go`
(284 líneas, leído completo):

| VLC-PARIDAD.md:línea | fila | fase que dice | estado real tras T2 | veredicto |
|---|---|---|---|---|
| 16 | UDP unicast (`udp{dst=ip:puerto}`, mux `ts`) | "**F2, lo primero**" | `udpts.go:86-103` (`nuevoUDPTS`) construido y probado de punta a punta; **ACEPTACION.md F2-46 ya lo marca "Construido en T2"** | **desfasado** — sigue como si fuera lo primero por hacer, ya está hecho |
| 17 | UDP multicast con `ttl=` | **F2-114** (pendiente en la tabla) | `udpts.go:esGrupo` reconoce el grupo por la IP, pone TTL=1 por defecto; **ACEPTACION.md:952-966 ya lo marca "Construido en T2"** con tres pruebas nombradas | **desfasado** — la tabla debería decir "Hecho (T2)", no dejar la fase como si aún faltara |
| 19 | PIDs, programa, `tsid`, `pcr=` | "F2 (ya en el diseño)" | `udpts.go:23-40` tiene `PIDVideo/PIDAudio/PIDPMT/Programa/TSID/PCRms` con validación completa (`valida()`, líneas 178-217); ACEPTACION.md F2-46 ya lo da por construido con PCR medido a 30.5 ms máx. Ojo: la misma fila mezcla eso con `dts-delay`, `shaping`, `use-key-frames` de VLC, que no tienen ningún equivalente en el código (`grep` vacío en encoder.go/udpts.go) | **desfasado a medias** — lo de PID/tsid/PCR ya no es "diseño", es código probado; lo de `dts-delay`/`shaping`/`use-key-frames` sigue correctamente sin construir, pero la fila no distingue las dos cosas |
| 21 | `duplicate{dst=…,dst=…}` (varias salidas a la vez) | "F2" | Varias salidas simultáneas con volumen y reconexión propios ya construidas; **ACEPTACION.md F2-46/F2-47/F2-49 marcadas "Construido en T2"**, con prueba `TestF2_47y49CadaSalidaConSuVolumenYUnaRotaNoCallaALasDemas` | **desfasado** — mismo patrón: la fase quedó sin actualizar a "hecho" |
| 23 | HTTP TS (`http-ts`) | F2-115 | `internal/drivers/salida/` solo tiene `udpts.go` y `archivo.go`: no existe `http-ts` | al día (correctamente marcado pendiente) |
| 41 | Entrada `url` (tirar de una fuente) | F2-116 | `model.LiveSource.Kind` solo admite `srt`/`rtmp`/`captura` (confirmado también por CONTINUAR.md:339, "no hay dónde dar de alta una fuente en vivo") | al día |
| 22, 58 | Ventana de monitor local | F2-117 | no existe pantalla de monitor en `web/src/pantallas/` | al día |

## 6 · `docs/f2/PLAN-F2.md`

**Tandas:** T1 (9 sept, motor) y T3 (10 sept, detector) coinciden con la
fecha real de sus commits (`9c1839b` 09-09 23:50; `4d7ca8f`/`a710871` 09-10
23:04-23:06). **T2** ("Decks, prioridad y salida `udp-ts` — HECHA (10 sept
2026)", línea 89) tiene su commit real (`10a63c9`, decks+prioridad+udp-ts)
fechado **2026-09-11 06:42** — un día después de lo que dice el encabezado
(menor, probablemente trabajo de madrugada sin cambiar la fecha del título).

**Contrato `salida.Driver` (§3, líneas 379-386): contradicho por el propio
documento y por ACEPTACION.md.** El código real
(`internal/drivers/salida/salida.go:54-66`) tiene **tres** métodos —
`Abrir`, `Vigilar`, `Descripcion`. La tabla del §1 del mismo PLAN-F2.md
(línea 27) ya lo dice bien: `Driver{Abrir, Vigilar, Descripcion}`. Pero el
bloque de código Go del §3, veinte líneas más abajo en el mismo archivo,
solo declara `Abrir` y `Vigilar` — y `docs/ACEPTACION.md:945` repite el
contrato incompleto. Tres lugares, dos versiones del mismo contrato, dentro
del mismo repo.

**T8 (§2, líneas 278-296): los nombres de archivo planeados no coinciden con
los paquetes que se construyeron hoy.** T8 promete `internal/drivers/captura/
receptor-tv.go` y `tarjeta.go`, `internal/drivers/alerta/signal-compare.go`,
`endec-serial.go` y `endec-rele.go`, más los tipos `model.CaptureInput` y
`model.AlertEvent` — cierra F2-81 a F2-85 y F2-111. Lo que entró hoy es
`internal/drivers/captura/hdhomerun/` (paquete propio, no archivos sueltos en
la raíz de `captura/`), `internal/drivers/alerta/sage/` y `internal/drivers/
alerta/same/` (paquetes propios, no `signal-compare.go`/`endec-*.go`), y
`internal/model/model.go` **no tiene** `CaptureInput` ni `AlertEvent` (`grep`
sin resultados). T8 formalmente sigue sin "HECHA", así que no hay criterio
roto — pero quien abra T8 va a encontrar trabajo ya hecho bajo una forma
distinta a la planeada, y tendrá que decidir si lo adopta o lo reescribe.

**`engine.ClipSource` (§3, líneas 359-368): coincide en la firma, difiere en
la semántica del caso límite.** El contrato documentado dice que `until` es
"el próximo corte conocido... o 'no sé, pregunta en 1s' si no hay nada
resuelto" cuando `Next` no tiene certeza. El código real
(`internal/engine/frameserver.go:25-35`) dice: "...o el cero de `time.Time`
si el clip sale entero, dure lo que diga el archivo" — una convención
distinta para el mismo caso (cero de `time.Time`, no "pregunta en 1s").
Verificado también contra la implementación real, `internal/engine/lista.go:29`
(`func (l *Lista) Next(time.Time) (Clip, time.Time, error)`), que coincide
en tipos con ambas versiones del comentario — el desacuerdo es solo de
semántica documentada, no de firma.

**`internal/model/incidentes.go` contra el catálogo ilustrado en §3:** el
código real tiene al menos 20 constantes `Inc*` (dos bloques, líneas 15-57
vistas) contra las 9 que el snippet de PLAN-F2.md muestra. No es
contradicción: el propio §3 dice "no son el diseño final, son lo mínimo".

**`package vivo` y `package captura` con `Driver{Comparar}` (§3, líneas
403-410): todavía no existen como el documento los describe.** No hay
`internal/drivers/vivo/` (T4 no está "HECHA"). `internal/drivers/captura/`
existe pero **solo** como el subpaquete `hdhomerun/` — sin el `type Driver`
con `Comparar` que T8 promete escribir en la raíz del paquete. Coincide con
que T8 tampoco está "HECHA": es plan pendiente, no contradicción, pero vale
la nota porque el trabajo de hoy (HDHomeRun, sage, same) ya construyó buena
parte de lo que T8 iba a construir, bajo una forma distinta (subpaquetes en
vez de la interfaz común planeada) — ver también `docs/auditoria/01-codigo-muerto.md`.

## 7 · `CONTINUAR.md` y `docs/API.md`

`docs/API.md:87-88` documenta `GET /guia.xml` y `GET /guia.pmcp`; ambos
existen en `internal/api/server.go:114-119` con las mismas rutas exactas
(`/guia.xml`, `/api/v1/guia.xml`, `/guia.pmcp`, `/api/v1/guia.pmcp`). **Al
día.**

`CONTINUAR.md:339` ("no hay dónde dar de alta una fuente en vivo: falta
`GET/POST/PUT/DELETE /api/v1/vivos`") se confirmó cierto: no existe esa ruta
en `server.go`. **Al día**, y sigue sin resolverse tras los commits
posteriores de la noche (SCTE-104, EAS, HDHomeRun no la tocan).

**Contradicción interna en CONTINUAR.md.** Las líneas 214 y 231-232 (entrada
de hoy, "T2 y cuatro bibliotecas de protocolo integradas") ya dan por hecho
`internal/resolver/pmcp.go` + `/guia.pmcp`. Pero la línea 346 (entrada
anterior del mismo día, sobre `docs/drivers/CATALOGO.md`) sigue resumiendo
el catálogo diciendo "PMCP para PSIP (**F5**)" — la misma bitácora se
contradice a sí misma sobre si PMCP ya está construido, porque la segunda
entrada nunca actualizó el resumen de la primera.

## 8 · Documentos que se contradicen entre sí

| Asunto | Documento A | Documento B | Naturaleza del choque |
|---|---|---|---|
| Umbral de negro del detector de salida | PRD.md:422,454 ("luma bajo 16") | ACEPTACION.md:1010-1038 (F2-52, "98 % bajo 25.5", con nota explícita de por qué "bajo 16" nunca se cumple) | ACEPTACION.md ya corrigió lo que PRD.md no |
| Contrato `salida.Driver` | PLAN-F2.md:27 (3 métodos) | PLAN-F2.md:379-386 y ACEPTACION.md:945 (2 métodos) | el propio repo tiene dos versiones vigentes del mismo contrato |
| Estado de `http-ts`/multicast/PIDs de `udp-ts` | VLC-PARIDAD.md:16,17,19,21 (fases "F2, lo primero"/"F2-114"/"F2, ya en el diseño"/"F2") | ACEPTACION.md:945-966 (F2-46, F2-47, F2-49, F2-114, todas "Construido en T2") | VLC-PARIDAD.md no se actualizó cuando T2 cerró |
| PMCP: ¿pendiente o construido? | CATALOGO.md:39-42 y CONTINUAR.md:346 ("F5", futuro) | CONTINUAR.md:214,231-232, API.md:87-88, código real (hecho hoy) | dos documentos (y dos entradas del mismo documento) en desacuerdo sobre el mismo día |
| Fecha de `captura/hdhomerun` | README.md:25 ("10 sept") | `git log` del archivo (11 sept, 06:48) | error de un día, sin impacto de fondo |
| Nombres de archivo de "retorno de aire/ENDEC" | PLAN-F2.md:290-293 (T8: `receptor-tv.go`, `tarjeta.go`, `signal-compare.go`, `endec-serial.go`, `endec-rele.go`, tipos `CaptureInput`/`AlertEvent`) | código de hoy: `internal/drivers/captura/hdhomerun/`, `internal/drivers/alerta/sage/`, `internal/drivers/alerta/same/`, sin `CaptureInput` ni `AlertEvent` en `model.go` | mismo objetivo (ADR 0009/0010), construido en paralelo bajo nombres y forma distintos a los planeados en T8 |

## Qué hay que corregir, priorizado

**El documento está mal (no requiere tocar código):**

1. **ACEPTACION.md:1404-1407 — recontar el Resumen.** F1 es 77, no 76; F2 es
   117, no 110; Total es 203, no 184; AUTO es 174 y hay un tag `[DOC]` (1)
   que el Resumen no menciona. Es aritmética, no criterio.
2. **PLAN-F2.md:379-386 y ACEPTACION.md:945 — añadir `Descripcion() string`**
   al contrato `salida.Driver` citado, para que coincida con `salida.go:54-66`
   y con la propia tabla de PLAN-F2.md:27.
3. **VLC-PARIDAD.md:16,17,19,21 — marcar "Hecho (T2)"** las filas de UDP
   unicast, multicast/TTL, PIDs/programa/tsid/PCR y `duplicate` (varias
   salidas), con la referencia a F2-46/F2-47/F2-49/F2-114; en la fila 19,
   separar lo hecho (PID/tsid/PCR) de lo que sigue sin construirse
   (`dts-delay`, `shaping`, `use-key-frames`).
4. **PRD.md:422,454,1345 — cambiar "luma bajo 16" por el umbral real** (98 %
   de la imagen bajo 25.5, con `LumaNegra=16` como el número que se muestra,
   no el que decide), copiando la nota que ACEPTACION.md:1030-1038 ya tiene.
5. **CATALOGO.md:41 y CONTINUAR.md:346 — quitar "PMCP (F5)"** de la lista de
   pendientes: ya está construido y servido en `/guia.pmcp` desde hoy.
6. **README.md:25 — corregir la fecha de `captura/hdhomerun`** de "10 sept"
   a "11 sept" (commit `368d53b`).
7. **PLAN-F2.md:89 — revisar la fecha de cierre de T2** ("10 sept") contra
   el commit real (`10a63c9`, 11 sept 06:42), o aclarar que es trabajo de
   madrugada del mismo tramo.

**Falta escribir el criterio (el código va adelante del documento):**

8. **ACEPTACION.md — abrir sección para SCTE-104, sage-endec, SAME y PMCP.**
   Los cinco paquetes de hoy no tienen un solo criterio "Construido en Tx"
   que los reclame, a pesar de que PRD §9 paso 7 y los ADR 0004/0010 ya los
   describen como parte del diseño. No es urgente para F2 (T8 aún no cierra),
   pero sin criterio propio, la próxima verificación de fase no tiene qué
   marcar como hecho.

**Nada que corregir, solo confirmado por esta auditoría:**

9. ADR 0002 (sin CGo), 0004 (SCTE-104 al encoder), 0009 (retorno de aire) y
   0010 (integrar el ENDEC) se respetan al pie de la letra en el código de
   hoy — incluidos los comentarios de paquete que citan el número de ADR.
10. `docs/API.md`, `docs/drivers/README.md` y `CONTINUAR.md:339` están al
    día en lo que afirman sobre rutas y huecos existentes; no se tocan.
