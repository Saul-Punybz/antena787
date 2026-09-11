# Auditoría de pruebas — 11 de septiembre de 2026

Siete trabajos de agentes distintos se integraron hoy, cada uno con sus propias
pruebas, y todo salió en verde. Verde no es lo mismo que probado. Esta
auditoría es de solo lectura: no se cambió una línea de código de producción
ni de prueba. Método: `go test ./... -cover`, `go test -race ./...`, y lectura
completa (no muestreo) de los 68 archivos de prueba del repo (21,353 líneas)
por cinco lecturas paralelas — una por área de responsabilidad — más un
cruce dedicado de `docs/ACEPTACION.md` contra el código real de las pruebas
que cita.

## 1 · Cobertura real por paquete

| Paquete | Cobertura | Responsabilidad | Nota |
|---|---|---|---|
| `internal/drivers/alerta/same` | 94.7% | alta | sin cobertura contra audio real de ENDEC (documentado, ver §5) |
| `internal/drivers/senal/scte104` | 93.3% | alta | — |
| `internal/importer` | 88.0% | media | — |
| `internal/drivers/alerta/sage` | 87.9% | media | — |
| `internal/resolver` | 86.7% | **alta** | — |
| `internal/drivers/salida` | 87.2% | **alta** | cifra alta pero esconde una carrera de datos real (§4) |
| `internal/ingest` | 81.1% | alta | — |
| `internal/drivers/captura/hdhomerun` | 80.4% | media | rama "Close detiene la reconexión" nunca se ejecuta (§2) |
| `internal/store` | 73.0% | **alta** | sin prueba de fallo real de disco ni de acceso concurrente |
| `internal/app` | 71.6% | **alta** | orquesta el motor; huecos de error/concurrencia en §5 |
| `internal/engine` | 67.5% | **alta** | el `Encoder` real (`StartEncoder`/`WriteFrame`/`WriteAudio`/`Finish`) no tiene ninguna prueba — todo `frameserver_test.go` corre con un recolector falso |
| `internal/despierto` | 62.2% | media | el único timeout de 1 s real que quedó en la suite vive aquí |
| `internal/model` | 48.8% | baja | paquete pequeño sin E/S ni estado compartido; el número bajo no es señal de riesgo |
| `internal/ts` | **0.0%** | **alta** | **sin archivo de test.** 300 líneas que miden continuidad/PCR/CC de un transport stream; sostienen F0-01 a F0-04 y F2-114. Solo se ejercita indirectamente vía `hdhomerun/medir_test.go` |
| `internal/f0` | 0.0% | baja | código de la fase F0, ya cerrada; usado solo por `cmd/f0` |
| `cmd/antena`, `cmd/f0` | 0.0% | baja | binarios finos (flags + wiring); esperado en Go |

**Los dos paquetes que de verdad importan y que la cifra global no delata**:
`internal/engine` (67.5%) reporta cobertura razonable porque `lista_test.go`,
`encoder_test.go` (solo `outputArgs()`) y `frameserver_test.go` cubren mucho
código *alrededor* del encoder, pero el encoder mismo — con su mutex
protegiendo la conexión de audio en paralelo al flujo normal — no lo toca
ningún test. E `internal/ts` (0%) no es ruido: es la lógica que decide si un
transport stream es válido para un multiplexor, y hoy solo la ejercita el
happy path de un test ajeno.

## 2 · Pruebas que no prueban nada, o que no pueden fallar

| Prueba | Archivo:línea | Problema | Gravedad |
|---|---|---|---|
| `TestAbrirCerrarDetieneLaReconexion` | `internal/drivers/captura/hdhomerun/entrada_test.go:128-150` | El servidor de prueba hace `Hijack()` y cierra el socket sin escribir respuesta; `conectar()` siempre da error por eso, y el test hace `return` en la línea 141-143 **antes** de llegar a la aserción real (`Close()` + `io.ErrClosedPipe`). Confirmado corriendo el test 5 veces: nunca ejecuta su propio cuerpo. | alta |
| `TestElAsistenteMandaLoQueLaPantallaPinta` | `internal/api/contrato_test.go:481-483, 465-478` | `if !declaraLaInterfaz(t, "RellenoPorDefecto") { return }` y tres bloques `if declaraLaInterfaz(...) { exige(...) }` sin `else`. Si `tipos.ts` deja de declarar la interfaz, la comprobación de contrato se salta en silencio — no falla, no aparece como skip. | alta / media |
| `TestElEmparejadorMandaLoQueLaPantallaPinta` | `internal/api/contrato_test.go:552-555` | Mismo patrón: `if declaraLaInterfaz(t, "TituloDelCatalogo") { ... }` sin rama de "no declarada". | media |
| `TestCortesQueNoCaben` | `internal/drivers/senal/scte104/cliente_test.go:710-715` | Único test del archivo que no usa `esperarA` antes de leer el inyector; la condición `len(...) > 0 && len(...) != 1` pasa igual si el mensaje aún no llegó (`len == 0`). Nunca confirma que el corte que sí cabe salió. | media |
| `TestCatalogoDeLaHojaReal` | `internal/importer/importer_test.go:518-521` | El comentario (línea 511-513) afirma "quedan 138 y no 146" tras la fusión; la aserción solo comprueba `len(todos) == 0 \|\| len(sinPareja) == 0`. El número exacto que el comentario promete no lo verifica ningún `if`. | media |
| `TestHorarioDeVeranoPrimavera` | `internal/resolver/resolver_test.go:395` | Acepta `-0500` **o** `-0400` para las 3:00 AM del 8 de marzo tras el salto de primavera en NY. El resultado correcto es inequívocamente `-0400`; aceptar también el offset previo al salto hace que una regresión real (código que no aplica el DST) siga pasando la prueba. | media |
| `TestF1_16_LaRepeticionNoAvanzaSuPropioContador` | `internal/app/f1verif_cola_test.go:159, 172, 185` | Tres `t.Skip(...)` encadenados antes de las aserciones reales. Si el resolver cambia de comportamiento y deja de cumplir cualquiera de las tres condiciones previas, el test se salta en vez de señalar la regresión que F1-16 existe para detectar. | media |
| `TestF1Verif28ElValidadorCorreAntesDePublicar` | `internal/app/f1verif_guia_test.go:323` | Compara solo el nombre del método por AST (`"ValidateXMLTV"`), sin mirar receptor/paquete. Cualquier método homónimo en otro tipo haría pasar la prueba. | baja |
| `TestF1Verif26LaEdicionAManoSobreviveAlResolver` | `internal/app/f1verif_guia_test.go:255` | `t.Skip("no hay ningún bloque de mañana que editar")` — una sola condición puede evitar que la aserición principal se ejecute nunca, sin que se note en un resumen de CI en verde. | baja |
| `TestRespaldoAntesDeMigrar` | `internal/store/store_test.go:585` | Calcula la fecha esperada con `time.Now()` *después* de que el código bajo prueba ya llamó a su propio `time.Now()`. Ventana de fallo minúscula si la corrida cruza la medianoche, evitable con un reloj inyectado. | baja |
| `TestCerrarEsIdempotente` | `internal/drivers/senal/scte104/cliente_test.go:760-764` | Afirma una ausencia ("no se reconecta solo") apoyada en `time.Sleep(50ms)` en vez del patrón `esperarA`/plazo largo que usa el resto del archivo. Débil, no frágil. | baja |

No se encontraron pruebas sin ninguna aserción (`if err != nil` como único
contenido) en los paquetes con mayor responsabilidad (engine, resolver,
store, model, app) — ahí la disciplina fue alta. El patrón sí aparece de
forma más sutil en `api` y en los paquetes con condiciones de salto
encadenadas.

## 3 · Pruebas frágiles

| Prueba | Archivo:línea | Depende de | Riesgo en Windows / máquina lenta |
|---|---|---|---|
| `TestCaidoAvisaSiElGuardianSeCaePorSuCuenta` | `internal/despierto/despierto_test.go:229` | `case <-time.After(time.Second):` en vez de `esperar()` | **alto** — es exactamente el patrón (timeout de 1 s en pared) que ya causó uno de los dos fallos de esta semana en el runner de Windows. El resto del repo usa márgenes de 2-60 s |
| `TestEnSombraElMotorNoArranca` | `internal/app/motor_test.go:331` | `time.Sleep(time.Second)` ("un segundo de gracia") | medio — inconsistente con el resto del archivo, que sí usa reloj inyectado/`esperar()` |
| `TestElAsistenteMandaLoQueLaPantallaPinta`, `TestRellenoPorDefectoDeUnClic` | `internal/api/contrato_test.go:498`, `internal/api/asistente_test.go:356` | `esperarEvento(..., 2*time.Minute)` sobre una codificación real de ffmpeg | medio — margen generoso, pero en un runner compartido y cargado por los otros 6 trabajos de hoy, no es garantía |
| `TestF1_07_SeNormalizaUnaSolaVez`, `TestCerrarEsIdempotente` | `internal/ingest/f1verif_ingest_test.go:417-418`, `internal/drivers/senal/scte104/cliente_test.go:762` | `time.Sleep(50ms)` como única defensa contra un evento indebido | bajo — no rompe en máquina lenta (una espera negativa es más laxa, no más estricta), pero puede encubrir una regresión sutil |
| `TestWebSocketEmpujaElEstado` | `internal/api/api_test.go:915-987` | socket TCP real + `SetReadDeadline(10s)` | bajo — único test del paquete `api` que sale de `httptest.Recorder` a red real |
| `TestF2_46ElTSQueSalePorUDPCumpleLoQueElMultiplexorExige` | `internal/app/salidas_test.go:31-203` | `net.ListenUDP` real + ffmpeg real | bajo — mitigado con puerto efímero y timeouts de 30-60s; falla por infraestructura si el sandbox de CI no tiene loopback UDP |
| `internal/ingest/bloqueo_windows_test.go` (todo el archivo) | build tag `//go:build windows` | el propio SO | **no verificable en esta máquina** (macOS). Es exactamente el tipo de código que se queda sin ejecutar hasta que corre en el runner de Windows — y ya hubo un fallo ahí esta semana |

**Confirmado que NO reaparece**: el problema de la ruta de Windows sin
escapar dentro de JSON. Se revisó el patrón en los 22 archivos de
drivers/ingest/importer; la única construcción de JSON crudo con backticks
(`salidas_test.go`) son direcciones de red, no rutas, y `udpts_test.go:176-184`
documenta el fix explícitamente ("la ruta va por el codificador de JSON, no
pegada a mano"). No se encontró ningún resto del bug en el resto del repo.

## 4 · Concurrencia (`go test -race ./...`)

Un solo paquete falla: **`internal/drivers/salida`**, en
`TestElVigilanteDejaEscritoComoLeVaALaSalida`
(`internal/drivers/salida/udpts_test.go:253`). Es una carrera de datos real,
no un falso positivo:

- El helper `esperar()` (líneas 261-272) hace *polling* con
  `time.Sleep(10ms)` leyendo `reg.estado` **directo**, sin lock ni canal.
- La goroutina de `vigilar()` (`salida.go:153-174`, invocada desde
  `(*udpts).Vigilar()` en `udpts.go:253`) escribe ese mismo campo vía
  `(*registro).SetConnection()` (línea 29) sin sincronización.
- Las dos primeras comprobaciones del test están protegidas por el
  happens-before de `<-reg.avisa`; la última, tras `cancel()`, usa
  `esperar()` y ahí se dispara la carrera.
- Agrava el problema que `SetConnection` usa un envío no bloqueante con
  `default`: una notificación por el canal se puede perder en silencio, así
  que ni el canal es una garantía completa para quien dependa de él.

Falta un `sync.Mutex` en `registro` (o que `esperar()` lea el estado por un
getter protegido en vez de tocar el campo a pelo).

**Concurrencia que existe en el código pero que ningún test ejercita bajo
`-race`** (por eso no aparecen como fallo, no porque estén bien):
- `internal/engine`: el mutex de audio del `Encoder` real (`encoder.go:88-160`)
  y el mutex de `Lista` (`lista.go`) — ambos solo se prueban desde una sola
  goroutina.
- `internal/store`: acceso concurrente de dos `*Store` al mismo archivo
  SQLite (el comentario en `store.go:48` lo declara seguro; ninguna prueba
  lo confirma con goroutines reales).
- `internal/app`: el motor y la vigilancia de negro/silencio compitiendo por
  `a.alarms`/`Store.Incident` en el mismo instante — `vigilancia_test.go` es
  deliberadamente síncrono salvo dos pruebas puntuales.
- `internal/drivers/captura/hdhomerun`: `Close()` llamado en paralelo a una
  reconexión en curso (hay mutex en producción; ninguna prueba lo ejercita).

## 5 · Criterios [AUTO] sin prueba real que los verifique

Se revisaron **96 criterios [AUTO] construidos** (69 de F1, excluido F1-04
que está diferido a F2 con `t.Skip` documentado; 27 de F2 marcados
"Construido" en T1/T2/T3). **86 tienen respaldo real. 10 no:**

| Criterio | Qué pide | Prueba citada/encontrada | Por qué no respalda |
|---|---|---|---|
| **F2-14** | Deriva del reloj del aire corregida gradual, máx. 300 ms/min | ninguna | `corregirDeriva`/`DerivaPorMinuto` no aparecen en ningún `_test.go` del repo |
| **F2-17** | 5.1 y mono consecutivos sin fallo audible | solo cita `audioFilter` y una corrida manual de F0 | esa evidencia no está en `go test ./...`; ningún test de `engine` usa audio 5.1/mono |
| **F2-114** | El TS llega completo a un receptor multicast con el TTL configurado | `TestElMismoDriverMandaAUnGrupoMulticastConSuTTL`, `TestLosArgumentosDelMultiplexorSonExactos` | ambas comparan strings de argumentos de ffmpeg; ninguna abre un socket y verifica recepción real |
| **F2-30** | El sistema suelta el aire y vuelve a AUTOMÁTICO tras negro/silencio en manual | `TestF2_54...` contra un mock (`controlDeMentira`) | el mock solo cuenta que se *pidió* soltar el aire; la acción real todavía no existe en producción (el doc lo confiesa: T5 la implementa) |
| **F2-48** | Salida RTMP que cae reintenta con backoff 1..60s | `TestElVigilanteDejaEscritoComoLeVaALaSalida` | el backoff sí está probado, pero sobre `udp-ts` — no existe driver RTMP en el repo |
| **F1-02** | Duración real en ms, no redondeada al segundo | `TestProbeMideLoQueDicelPRD` | el guardia `DurationMs%1000==0 && DurationMs!=3000` exime justo el valor que produciría una implementación que redondea; el test no puede fallar aunque `Probe` redondee |
| **F1-28** | El validador rechaza publicar una guía inválida y lo registra | `TestValidadorDeLaGuia`, `TestF1Verif28...` | se prueba que el validador detecta el caso Hellsing; nadie ejercita la rama de rechazo real (`resolve.go:311`) ni que la guía anterior siga puesta |
| **F1-49** | El envío HTTP externo puede fallar sin afectar la guía local | `TestF1Verif49GuiaXMLSiempreContesta/...RutaFisica` | cubren `/guia.xml` y la escritura en disco; nadie prueba `pushGuide` con un destino que falla |
| **F1-71** | La cola da por perdida la normalización y pasa a cuarentena | `TestNormalizacionColgadaVaACuarentena` | el cuelgue por plazo sí es real; el estado "dado por perdido" se fabrica a mano con `SetNormalizeState`, no lo produce el bucle de reintentos real (`queue.go:192-199`) — no existe `queue_test.go` |
| **F1-73** | `GET /material?estado=ingiriendo` + refresco por SSE en Biblioteca | `TestF1_73_ElArchivoSeVeMientrasSeMide` | solo verifica la primera escritura de persistencia; ni el filtro de la API ni el SSE tienen prueba |

**Nota de proceso, relevante para el objetivo de esta auditoría**: dos de
los cinco sub-chequeos que alimentan esta tabla intentaron primero cerrar su
rango apoyándose en `docs/f1/VERIFICACION-F1-2026-09-09.md` (un informe
previo ya en verde) en vez de abrir el código de las pruebas — uno admitió
haber juzgado 8 criterios solo por `grep`. Se repitieron con lectura
obligatoria del código, y de ahí salieron 3 de los 10 hallazgos (F1-02,
F1-28, F1-49). Es el mismo patrón que esta auditoría existe para atrapar: un
documento en verde no es evidencia.

También: `internal/app/f1verif_cola_test.go:22-27` lleva un comentario
"FALLA HOY" sobre F1-42 que hoy ya no es cierto — el test pasa. El comentario
quedó obsoleto y debería borrarse.

## 6 · Lo que no está probado y debería

- **Camino completo carpeta vigilada → aire**: sólido hasta "listo para
  aire" en `internal/ingest`; se rompe en el tramo final — `internal/engine`
  nunca corre el encoder real con un clip real, solo el recolector falso.
- **Errores de ffmpeg en el camino caliente**: se prueba el cuelgue
  (timeout artificial) en normalización, pero no un código de salida
  distinto de cero durante el conformado o la salida al aire.
- **Fallo de disco**: ningún test de `store`, `salida` (archivo/udp-ts) ni
  del log de eventos del motor simula permiso denegado o disco lleno; todo
  es happy path o corrupción de cabecera fabricada a mano.
- **Fallo de base bajo concurrencia real**: cero pruebas con dos `*Store`
  reales escribiendo a la vez.
- **Recepción real de TS**: F2-114 se prueba por comparación de argumentos
  de ffmpeg, nunca abriendo un socket multicast del lado receptor.
- **Audio real de ENDEC** (`internal/drivers/alerta/same`): documentado a
  propósito en `testdata/LEEME.md` (ADR 0010) — 94.7% de cobertura de
  paquete no incluye ningún caso contra grabación real, solo señal
  sintética.
- **Entrada corrupta a medio camino**: se prueba "video mudo" y "no existe",
  no un contenedor dañado que `ffprobe` no pueda leer del todo.

## 7 · Pruebas duplicadas

No se encontraron duplicados exactos entre los siete trabajos integrados —
los agentes, pese a no coordinarse, no repitieron el mismo caso dos veces.
El único solapamiento real: `TestElEmparejadorMandaLoQueLaPantallaPinta`
(`internal/api/contrato_test.go:522-566`) y
`TestF1Verif66EmparejarPasaLasReglasYSeAcuerda`
(`internal/api/emparejar_test.go:242-344`) montan el mismo escenario
("Samurai X" sin emparejar contra "Rurouni Kenshin") — no verifican lo mismo
(una mira el contrato de tipos, la otra el comportamiento), pero comparten
casi todo el montaje. Candidato a reducir mantenimiento, no un bug.

## 8 · Las diez pruebas que más faltan

1. Prueba de concurrencia sobre `salida.registro` con `-race` que fuerce el
   mutex que falta — deriva directo del race ya confirmado en §4.
2. Prueba del `Encoder` real (`StartEncoder`/`WriteFrame`/`WriteAudio`/
   `Finish`) con audio 5.1 y mono, no el recolector falso — cierra el hueco
   de mayor responsabilidad sin cubrir y respalda F2-17.
3. Prueba unitaria de `internal/ts` sobre TS sintéticos con errores de
   continuidad/PCR inyectados — hoy 0% de cobertura propia, sostiene
   F0-01 a F0-04 y F2-114.
4. Prueba de recepción real en un socket multicast para F2-114 — hoy solo
   se compara la línea de comando de ffmpeg.
5. Prueba de la rama de rechazo del validador de guía (F1-28): que una guía
   inválida no se publique y quede el incidente `guia_rechazada`.
6. `queue_test.go` para el bucle de reintentos real de la cola de
   normalización (F1-71), sin fabricar el estado "dado por perdido" a mano.
7. Reescribir `TestCaidoAvisaSiElGuardianSeCaePorSuCuenta` con `esperarA()`
   o un plazo de varios segundos en vez de `time.After(1s)` — mismo patrón
   que ya rompió el CI de Windows esta semana.
8. Prueba de fallo de disco (permiso denegado / lleno) al escribir la salida
   `udp-ts`/`archivo` y el log de eventos del motor.
9. Prueba de concurrencia motor + vigilancia bajo `-race`, forzando un
   cambio de deck y una alarma de silencio en el mismo instante.
10. Prueba end-to-end de `GET /material?estado=ingiriendo` más su refresco
    por SSE en Biblioteca (F1-73) — hoy solo está probada la mitad de
    ingest, no la mitad API/UI que el criterio promete.
