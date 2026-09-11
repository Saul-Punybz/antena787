# Por qué el decoding no tiene aceleración por hardware (11 sept 2026)

## 1 · ¿Hay una razón escrita?

**No existe ninguna decisión escrita que diga «el decoding se queda por
software a propósito».** Lo que sí está escrito, en varios sitios y de forma
consistente, es que la aceleración por hardware es un concepto que el
proyecto definió **solo para el encoder**, nunca para el decoder. No es que
alguien haya escrito «no aceleramos el decode» — es que el decode nunca entró
en la conversación como candidato a acelerarse.

- `internal/adr/../docs/adr/0001-own-playout-engine-in-go.md` (líneas 1-3 y la
  última línea): el ADR entero describe la decisión como mantener «one
  hardware-accelerated ffmpeg **encoder**» vivo para siempre, y cierra con
  «Hardware acceleration stops being a convenience and becomes structural:
  **software encoding** costs a full core per channel». Las dos frases que
  mencionan aceleración hablan de encoding. El decoder no aparece.
- `PRD.md:1342` (§14, tabla del stack), fila **«Aceleración por hardware»**:
  «Al arrancar, el motor **codifica** diez segundos de prueba con cada
  **encoder** candidato —QuickSync, NVENC, VAAPI, software— mide CPU y elige
  el mejor». Es la definición operativa del concepto en todo el documento, y
  es de encoders, no de decoders.
- `docs/ACEPTACION.md:748` (F2-15): «Dado el motor arrancando en una máquina
  con QuickSync, NVENC, VAAPI y software disponibles · Cuando corre la prueba
  de 10 segundos con cada **encoder** candidato · Entonces la pantalla de
  Ajustes muestra cuál fue elegido y el uso de CPU medido de cada uno.» —
  ver punto 4 más abajo, cubre solo encoding.
- El commit `985e332` (11 sept, el que crea `engine.Acelerador`) explica en
  su propio mensaje qué resuelve: «con qué se comprime el video» —
  comprimir es encoding— y por qué: F2-11 necesita relanzar el encoder
  colgado «con el mismo acelerador». No menciona el decoder en ningún punto.

## 2 · Si no la hay, decirlo claramente

**No existe ninguna decisión escrita sobre por qué el decoding se queda sin
aceleración.** No es una inferencia deducida del estilo del proyecto: es la
ausencia comprobada de la palabra en el lugar donde tendría que estar. Se
verificó con:

- `internal/engine/decoder.go` completo: cero apariciones de `hwaccel`,
  `Acelerador`, `hardware` o `GPU`. Tampoco hay un comentario que explique la
  omisión — el archivo simplemente no habla del tema.
- `git log -S hwaccel --all`, `-S GPU --all`, `-S NVENC --all`: **nunca
  existió una línea con `-hwaccel` en el historial completo del repo.** No es
  que se quitó — nunca se escribió. `-S acelerador --all` solo aparece a
  partir del 11 sept (`985e332` en adelante), y siempre en `encoder.go`,
  `estado.go`, `app.go`, nunca en `decoder.go`.
- `docs/f2/PLAN-F2.md:18`: la fila de `decoder.go` en el plan de F2 dice
  **«Sí, entero»** (ya cumple, no hay trabajo pendiente) — es decir, el plan
  de la fase donde se construyó la aceleración por hardware del encoder
  (F2-11, F2-15) da el decoder por terminado sin tocarlo. Ni una palabra de
  por qué.
- `docs/investigacion/`, `docs/auditoria/`, `CONTINUAR.md`, `CONTEXT.md`: sin
  una sola mención de aceleración de decode.

**Matiz que sí hay que decir, porque cambia la respuesta a medias:** el PRD
tiene una frase que apunta en la dirección contraria a lo implementado.
`PRD.md:1626-1628` (§18), hablando del caso CAtv (transmisor recibe MPEG-2 por
software, sin necesidad de acelerarlo): «Ahí la aceleración se gasta en
**decodificar** la biblioteca (H.264) y en las salidas de internet (H.264),
no en la del transmisor.» Esta frase, escrita en la versión v8 del PRD (commit
`43582b2`), sí contempla gastar aceleración por hardware en decodificar. Pero
esa intención de §18 **nunca bajó a una definición operativa**: ni la tabla
del stack (§14), ni el ADR 0001, ni F2-15, ni el commit del 11 sept que por
fin construyó `engine.Acelerador` la retoman. Lo que se construyó el 11 de
septiembre implementa literalmente la definición de §14 (candidatos de
*encoder*), no la frase de §18. Es una discrepancia entre una intención
redactada una sola vez en la prosa y la definición operativa que sí se llevó
al código — no una decisión de dejar el decode fuera, sino una idea que quedó
sin desarrollar y que el trabajo real no recogió.

## 3 · Requisitos de la máquina — ¿se prometió correr sin tarjeta de video?

Sí, de forma explícita y repetida, y con datos medidos, no solo prometidos:

- `PRD.md:1618-1623` (§18): «**La aceleración por hardware es estructural,
  no una conveniencia.** Con encoder persistente, recodificar 1080p cuesta
  ~15% del CPU con QuickSync en un N100 *(estimado)*, pero **un núcleo
  completo por software** *(estimado)*.» — la cifra es del **encoder**.
- `PRD.md:1631-1633`: «**Presupuesto cero** | La máquina que ya tienes... Es
  el caso de CAtv: Windows 10 existente, sin comprar nada | $0».
- `PRD.md:1638-1642`: «El despliegue de referencia corre con presupuesto
  cero... sobre un Windows 10 que ya tiene, sin comprar hardware... Si
  funciona ahí, la tesis del proyecto se sostiene sola.»
- Medición real, no estimada: `docs/f0/REPORTE-mac-m4-2026-09-08.md`, fila
  **F0-07**: «CPU media 110 % (de un núcleo), RAM media 699 MB, 689
  muestras» — **PASA**. Esa corrida (2 h, dos salidas simultáneas) es
  enteramente por software: el `Acelerador` no existía todavía (se creó el
  11 de septiembre, tres días después de este reporte), así que este número
  incluye **decode por software de cada clip + encode por software**, y aun
  así se queda en poco más de un núcleo. Es la evidencia empírica de que la
  promesa de §18 (PC modesta, sin tarjeta) se sostiene incluso sin acelerar
  nada del pipeline.
- La fila **F0-08** («Sesiones de encoder por hardware que aguanta la
  máquina») quedó sin medir esa corrida: «manual: no se midió en esta
  corrida» — **—**. No hay, en ningún reporte de F0 o de modo sombra, una
  medición de CPU específicamente con decode acelerado, porque esa
  posibilidad nunca se construyó para medirla.
- `docs/f1/SOMBRA-2026-09-09.md:84-85` (hallazgo S-4): sobre la normalización
  lenta en software en el Mac M4, dice «es un dato para F2 (aceleración por
  hardware, que el asistente ya dice que se mide «al arrancar el motor»)» —
  otra vez la aceleración referida es la del motor/encoder al arrancar, no
  la del decode del ingest.

## 4 · ¿F2-15 cubre el decoding o solo el encoding?

**Solo el encoding.** Cita completa (`docs/ACEPTACION.md:748-750`, texto
original de F2-15, mecánicamente citada):

> «**F2-15** [MANUAL] — Dado el motor arrancando en una máquina con
> QuickSync, NVENC, VAAPI y software disponibles · Cuando corre la prueba de
> 10 segundos con cada **encoder** candidato · Entonces la pantalla de
> Ajustes muestra cuál fue elegido y el uso de CPU medido de cada uno.»

Los seis valores de `Acelerador` (`internal/engine/encoder.go:100-109`), la
función `Disponible` que pregunta `ffmpeg -encoders` (línea 198), y el
watchdog de F2-11 que «relanza con el mismo acelerador» — todo es sobre qué
códec de **video usa el encoder** para comprimir la salida. El texto de
F2-15 no menciona `decoder.go`, `hwaccel` de entrada, ni decodificación en
ningún punto, ni en su redacción original ni en la nota «A medias (11 sept
2026)» que se le añadió el mismo día (`docs/ACEPTACION.md:753-769`, commit
`ace8ad9`), que tampoco toca el tema del decode — habla de que `Disponible`
pregunta el binario en vez de probar la tarjeta de verdad, un problema
distinto y solo del lado del encoder.

## 5 · Otras decisiones que dependen de esto

- **ADR 0001** (encoder persistente): depende del *encoding* acelerado para
  sostener el costo casi lineal por canal («hardware encoding measures
  around 15% of the CPU on a low-power N100»). No depende de que el decode
  esté o no acelerado; el ADR nunca lo necesitó porque el contrato
  decoder→frameserver→encoder (`PRD.md:1334`) ya asumía cuadros crudos
  (`rawvideo`/PCM) entrando al servidor de cuadros, sin importar cómo se
  generaron.
- **Normalización al ingerir** (`internal/ingest`, hallazgo S-4 del modo
  sombra): usa `libx264 -preset medium -crf 20` por software para el
  encoding de la normalización; el propio hallazgo la señala como «un dato
  para F2 (aceleración por hardware...)» — es decir, si algún día se
  extiende `Acelerador` a la normalización, sería otra vez del lado del
  encode de esa pasada, no del decode.
- **Watchdog de F2-11** (`PRD.md:445-448`, `internal/engine/encoder.go`
  watchdog del acelerador): baja el canal a software cuando el encoder por
  hardware deja de responder. Es una decisión que solo tiene sentido porque
  existe `Acelerador` en el encoder; no tiene contraparte en el decoder
  porque el decoder nunca tuvo un modo «por hardware» del que caer.
- **`docs/f2/PLAN-F2.md:18`**: como se cita arriba, el plan de F2 (la fase que
  sí construyó `Acelerador`) da `decoder.go` por completo desde antes de
  empezar — «Sí, entero» — así que ninguna tanda de F2 tenía en su alcance
  tocarlo. Esto explica el *cómo* (nunca estuvo en el plan de trabajo) pero
  no es una justificación técnica del *por qué*, y no se presenta como tal
  en ningún documento.

## Lo que sí está bien escrito (para no mentir por omisión)

El concepto de aceleración que sí existe —el del encoder— está documentado
con cuidado real: nombrado en un solo sitio (`engine.Acelerador`, commit
`985e332`) antes de que lo consuma el watchdog o la pantalla; con `auto`
resolviendo deliberadamente a software para no aterrizar a nadie en una
tarjeta sin haberla pedido; y con el defecto de VAAPI en Mac (F2-15,
`Disponible` preguntando el códec equivocado) encontrado y corregido el mismo
día en que se escribió, con la fecha y quien lo señaló (Saul) dejados en el
commit y en `ACEPTACION.md`. Es una pieza chica pero trazable de punta a
punta — el problema de esta investigación es exclusivamente que esa misma
trazabilidad nunca se extendió a preguntarse por el decoder.
