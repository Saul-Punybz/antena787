# Cómo contribuir el perfil de cumplimiento de un país

> **Antena787 no asume Estados Unidos en ninguna parte** (PRD §4, principio
> 10; [§12](../../PRD.md#12--internacional-y-cumplimiento)).

Un **perfil de cumplimiento** son las reglas del país donde el canal transmite:
el volumen de la casa, el formato de los subtítulos, qué registros se guardan y
por cuánto tiempo. **Se escoge por país, nunca se supone**, y es lo que evita
que una universidad en España o en Colombia salga al aire con el volumen de
otro país.

Este documento es para quien quiere aportar el de su país.

---

## Lo primero: el tono. Se ofrece, no se exige

Esto manda sobre todo lo demás, y es lo que más fácil se rompe al escribir un
perfil (auditoría [sección 0](../AUDITORIA_2026-09-04.md)):

> ### **Así corren muchos canales locales en Puerto Rico y en el mundo, y no pasa nada. El software nunca regaña.**
>
> No le dice a nadie que está en falta, **no bloquea nada por cumplimiento**, y
> no da por sentado que el operador le debe algo a alguien.

Lo que hace un perfil es tener **listo lo que a veces alguien pide**: la
bitácora de alertas, el archivo de anuncios políticos, el registro del medidor
de volumen, la línea de *"anuncio pagado por"* en el crawl. Todo eso se activa
con el perfil, corre solo por debajo, y el día que llega una carta o una
llamada está ahí, exportable, **sin haber tenido que acordarse de nada.**

**Ese es el argumento: no un deber, un seguro que no cuesta trabajo.**

**Y se puede apagar entero.** Un canal por internet no enciende ninguna de estas
funciones, y **no tiene que explicar por qué.**

**Tres consecuencias prácticas** para quien escribe un perfil:

1. **Ninguna función de un perfil puede impedir que algo salga al aire.** Se
   anota, se avisa, se registra — no se bloquea.
2. **Cada función se puede apagar por separado**, y apagarla no produce una
   advertencia repetida.
3. **No se vende cumplimiento que no existe.** El as-run, por ejemplo, **no
   está en la tabla del perfil `us-fcc` a propósito**: la FCC eliminó los
   registros de programación, y la bitácora de estación (73.1820) cubre otras
   cosas. El as-run es **evidencia comercial** —facturar, reponer un spot
   tapado, probarle a un anunciante que su spot salió— y vale por eso, no por
   un reglamento.

**Y hay cosas que sencillamente no son de este software.** Las luces de la torre
no las toca Antena787, y [`COMPLIANCE.md`](../../COMPLIANCE.md) lo dice así de
claro. **Las alertas de emergencia se cumplen con hardware certificado, no con
este software.** Antena787 **ayuda a cumplir, no certifica**; no sustituye
asesoría legal ni al ingeniero de la estación.

---

## Qué define un perfil

Siete cosas. Un perfil que no las tenga todas está incompleto, y lo que falte
se dice — no se rellena con lo de otro país.

### 1 · Volumen objetivo y su norma

El número de la casa, con **la norma de donde sale**. Se aplica **solo en el
ingest**, con `loudnorm` de ffmpeg en dos pasadas —medir y luego corregir—
(§14.1), y cada salida puede tener el suyo (§7).

| | Volumen | Norma |
|---|---|---|
| Américas (ATSC) | **−24 LKFS**, TP −2 dBTP | ATSC A/85 |
| Europa, África, Asia (DVB) | **−23 LUFS** | |
| Internet | **−16 LUFS** | |

> **−24 LKFS contra −23 LUFS parece trivial y no lo es.** Es exactamente el
> tipo de dato que el perfil pone sin que nadie tenga que buscarlo.

### 2 · Formato de subtítulos

| Región | Formato |
|---|---|
| Américas (ATSC) | CEA-608/708 |
| Sudamérica y Japón (ISDB-T) | **ARIB** |
| Europa, África, Asia (DVB) | **DVB, teletexto** |

Y con esto va **una obligación técnica del pipeline, no un efecto
secundario**: como el motor decodifica y recodifica *(ADR
[0001](../adr/0001-own-playout-engine-in-go.md))*, los subtítulos embebidos
**se pierden si no se extraen y se reinsertan explícitamente en el mux de
salida** (§9 paso 1). Un perfil nuevo tiene que decir **en qué formato** hay
que reinsertarlos.

El perfil dice además si el país tiene **exenciones** y con qué criterio. En
`us-fcc` el ajuste tiene tres estados —**obligada**, **exenta** (con el motivo)
o **no sé todavía**— y **los tres funcionan igual**: los subtítulos embebidos
siempre se conservan y siempre se pueden subir. El ajuste solo cambia si el
sistema avisa. **"No sé" es el default y no bloquea nada.**

### 3 · Alertas de emergencia

Cuál es el sistema del país —**EAS** en Estados Unidos, **EWBS** en Sudamérica
y Japón (que va en la norma), sistemas nacionales en Europa— y, sobre todo:
**por dónde se entera Antena787 de que hubo una.**

**Antena787 no construye el equipo de alertas** y no lo va a construir. Lo que
el perfil aporta es qué drivers de la familia de alertas aplican en ese país, y
**si `signal-compare` sobre el retorno de aire es la única vía realista** — que
es lo normal fuera de Estados Unidos *(ADR
[0009](../adr/0009-truth-is-the-transmitted-signal.md); [§10](../../PRD.md#10--los-drivers))*.

### 4 · Registros y retención

Qué se guarda, **por cuánto tiempo**, y **qué artículo lo dice**. En `us-fcc`:
bitácora de alertas **24 meses**, archivo de anuncios políticos **dos años**.

Un perfil nuevo tiene que traer sus propios números, con su fuente. **Si no se
sabe, se dice que no se sabe** — es la misma regla de que `ninguna` nunca
miente (§10).

### 5 · Identificación de estación

Cada cuánto hay que identificar, con qué, y qué artículo lo exige. En `us-fcc`
es **cerca de cada hora** (47 CFR 73.1201).

**Y esto tiene una consecuencia de diseño que ya está construida en el PRD:**
el **cartel de respaldo** —el último escalón de la cascada, generado por el
asistente en el primer paso— lleva el identificativo y la comunidad de
licencia, así que **una caída larga sigue identificando la estación** (§9 paso
4). Si pasa una hora sin identificación, **queda anotado en incidentes —
anotado, no bloqueado.**

### 6 · Formatos de casa típicos

Cuáles de la lista del §10 usa el país, y a qué tasa de cuadros:

| | Cuadros |
|---|---|
| Américas (ATSC) e ISDB | 59.94 / 29.97 Hz |
| Europa, África, Asia (DVB) | **50 / 25 Hz** |

**Todos los formatos se traen** —`1080i59.94`, `1080p29.97`, `1080p59.94`,
`1080i50`, `1080p25`, `720p50`, `720p59.94`, `480i59.94`, `576i50`, `custom`—
y el perfil solo dice cuáles ofrecer primero. **Contenedor y códec son
propiedades del perfil, nunca constantes del motor** — es lo que permite que
ATSC 3.0 entre como perfil y no como reescritura.

*Colombia usa DVB-T2 estando en Sudamérica: por eso el perfil se elige por
país, no por continente.*

### 7 · Señalización de cortes

Qué espera la cadena del país. La ruta primaria es **SCTE-104 al encoder, que
genera el SCTE-35** *(ADR [0004](../adr/0004-scte104-to-the-encoder.md))*, y el
perfil dice si eso aplica tal cual o si el país usa otra cosa.

Y el perfil puede exigir texto en la señal: en `us-fcc`, **todo clasificado sale
con el prefijo *"Anuncio pagado por ‹nombre›"*** (73.1212), y **en ese perfil el
prefijo no se quita** — es una línea de texto y evita el único lío que sí es
fácil de evitar.

---

## Los perfiles previstos, y qué se sabe de cada uno

**Al lanzar hay dos: `us-fcc` e `internet`** (§12). Los demás son Fase 5.

### `internet` — listo en el diseño

Sin obligaciones regulatorias de radiodifusión. **Volumen −16 LUFS.** Y no es
un extra: **para la mayoría de los usuarios este es el modo principal** (§7).

Un canal en este perfil **no enciende ninguna función de cumplimiento y no
tiene que explicar por qué.**

### `us-fcc` — el más desarrollado

Lo primero que pregunta es **la clase de licencia** —potencia completa, Class A
o LPTV— y **cada una enciende solo lo que le toca**; se cambia cuando se
quiera. Todo es activable:

| Función | Qué hace |
|---|---|
| **Volumen** | −24 LKFS / TP −2 dBTP (ATSC A/85), puesto solo en el ingest |
| **Subtítulos** | CEA-608/708 conservados y reinsertados en la salida; ajuste de tres estados que no bloquea nada |
| **Identificación de estación** | Cerca de cada hora (47 CFR 73.1201). El cartel de respaldo también la lleva. Si pasa una hora sin ella, queda anotado en incidentes |
| **Bitácora de alertas** | Distingue pruebas de activaciones reales. **Se guarda 24 meses** y nada la borra antes. Las pruebas semanales se comparan contra la ventana horaria configurada; si una cae fuera, avisa |
| **Archivo de anuncios políticos** | Candidato, cargo y elección; de ahí sale un archivo exportable con la solicitud, la aceptación o el rechazo, las tarifas y los horarios (73.1942/73.1943), guardado dos años. **La tarifa mínima aparece como aviso**, no como un cálculo que el software imponga |
| **Identificación de patrocinio** | *"Anuncio pagado por ‹nombre›"* en todo clasificado (73.1212). En este perfil no se quita |
| **Registro del medidor de volumen** | Una anotación diaria automática: *"medidor activo, N archivos normalizados, 0 fallos"* (73.682(e)). Exportable |
| **Archivo público en línea** | Exportación lista para subir, **solo si la licencia lo pide** (Class A y potencia completa). **Un LPTV no lo ve** |

**Lo que viene, y por qué no cambia el diseño.** La FCC votó 3-2 en mayo de
2026 obligar a las estaciones **de potencia completa** a completar la
transición a ATSC 3.0 hacia el cuarto trimestre de 2027; el simulcast vence el
17 de julio de 2027. **LPTV, traductores y Class A están hoy exentos**, y buena
parte de los usuarios de Antena787 cae ahí. ATSC 3.0 es otra pila —HEVC, AC-4,
ROUTE/DASH sobre IP en vez de MPEG-TS— y **el diseño lo absorbe porque el
formato ya es un perfil y la salida ya es un driver.**

### `eu-ebu` — Fase 5, sin mantenedor

Lo que el PRD fija: **50 / 25 Hz**, subtítulos **DVB y teletexto**, volumen
**−23 LUFS**, sistemas nacionales de alerta. En F5 van también los subtítulos
DVB y la exportación **DVB-SI**.

**Lo demás está sin escribir** — retención de registros, identificación de
estación, y el hecho de que "Europa" no es una jurisdicción sino muchas. Es
justo lo que un mantenedor local tiene que aportar.

### `isdb-latam` — Fase 5, sin mantenedor

Lo que el PRD fija: **59.94 / 29.97 Hz**, subtítulos **ARIB**, volumen −24
LKFS, y **EWBS**, que va en la norma. En F5 van los subtítulos ARIB y la
exportación **PSIP y DVB-SI**.

**Y el mismo aviso:** "LATAM" tampoco es una jurisdicción. Cada país tiene sus
reglas y **Colombia, que está en Sudamérica, usa DVB-T2.**

---

## La regla del mantenedor local con receptor real

> **Cada perfil requiere fuente citada y un mantenedor que opere en esa
> jurisdicción, o se marca experimental** (PRD §12).

**Y esto no es burocracia.** Es la consecuencia de un hecho que el proyecto
dice de frente: **nadie del equipo central tiene un receptor DVB ni ISDB en la
mano**, ni una estación en Europa, ni en Japón, ni en Sudamérica. Es el mismo
hecho que hace de `signal-compare` la respuesta a *"cualquier marca, cualquier
país"* — **funciona con hardware que el proyecto nunca va a tener** (§10).

**Un perfil que nadie puede verificar en el aire de su país es una promesa que
no se puede sostener.** Y el costo de equivocarse no lo paga el proyecto: lo
paga una estación que creyó que estaba en regla.

**Qué se le pide a un mantenedor de perfil:**

- **Operar en esa jurisdicción**, o trabajar con quien opera.
- **Tener acceso a un receptor real** de la norma del país —o a una estación
  que lo tenga— para poder mirar la señal transmitida y no solo lo que el
  sistema cree que mandó.
- **Citar fuentes primarias**: el reglamento, la norma, el artículo. **No un
  blog, no un foro, no un resumen de otro país.** Enlace directo, y la fecha en
  que se consultó.
- **Volver cuando la regla cambie.** Un perfil no se termina: se mantiene.

**Sin mantenedor local, el perfil se puede aportar igual — y se marca
`experimental`, en el código y en la interfaz.** Se puede usar; lo que no se
puede es fingir que está verificado.

---

## El flujo: issue → borrador → PR

### 1 · Un issue con la plantilla `perfil-pais.yml`

Antes de escribir el perfil, un issue que diga:

- **Qué país**, y qué norma de transmisión usa —ATSC, ISDB-T, DVB— con la
  variante exacta si la hay.
- **Quién lo va a mantener**, y desde dónde. Si eres tú, dilo.
- **Si hay acceso a un receptor real** de esa norma. **"No" es una respuesta
  aceptable**: el perfil sale marcado `experimental`.
- **Las siete cosas de arriba**, aunque sea con huecos. Los huecos son
  información.
- **Si hay una estación esperándolo.** Un perfil con una estación detrás pesa
  más que uno hipotético.

> **La plantilla existe:**
> [`.github/ISSUE_TEMPLATE/perfil-pais.yml`](../../.github/ISSUE_TEMPLATE/perfil-pais.yml).
> Pide el país, la norma de volumen, los subtítulos, las alertas de emergencia,
> los registros y su retención, otras obligaciones, y tu relación con esa
> jurisdicción.

### 2 · Un borrador en `docs/profiles/<pais>.md`

Un archivo por país, nombrado con el código del perfil —`us-fcc.md`,
`eu-ebu.md`, `isdb-latam.md`, `mx-ift.md`— siguiendo la plantilla de abajo.

**Cada afirmación lleva su fuente primaria citada, con enlace y fecha de
consulta.** Una afirmación sin fuente no entra; se queda en la lista de "lo que
no se sabe todavía", que es una sección obligatoria del perfil.

### 3 · Un PR

- El borrador del perfil, completo o con sus huecos declarados.
- **Cómo se verificó** — con receptor real, con una estación, o sin verificar.
  **Lo último es aceptable si se dice; fingir lo primero no.**
- **`Signed-off-by`** — las contribuciones entran por DCO *(ADR
  [0005](../adr/0005-agpl-with-dco.md))*. Detalle en
  [`CONTRIBUTING.md`](../../CONTRIBUTING.md).
- **El vocabulario de [`CONTEXT.md`](../../CONTEXT.md).** Y el tono del §12: se
  ofrece, no se exige. **Un perfil que regaña se devuelve.**

---

## Plantilla de un perfil

Copia esto a `docs/profiles/<pais>.md` y llénalo. **Lo que no sepas, déjalo en
"Lo que no se sabe todavía" — no lo rellenes con lo de otro país.**

```markdown
# Perfil de cumplimiento: <País> (`<codigo-perfil>`)

**Estado:** verificado | **experimental** (sin mantenedor local o sin receptor)
**Mantenedor:** <nombre> · <dónde opera> · <cómo contactar>
**Norma de transmisión:** ATSC 1.0 | ATSC 3.0 | ISDB-T | ISDB-Tb | DVB-T | DVB-T2
**Receptor real disponible:** sí | no
**Última revisión:** <fecha>

## Qué enciende este perfil, y qué no

<Un párrafo en el tono del §12: qué queda listo por si alguien lo pide.
Nada bloquea, nada regaña, todo se puede apagar.>

## Clases de licencia

<Si el país distingue clases —potencia, cobertura, comunitaria, comercial—,
cuáles son y qué enciende cada una. Si no distingue, dilo.>

## 1 · Volumen

| Qué | Valor | Norma | Fuente |
|---|---|---|---|
| Objetivo | −XX LUFS/LKFS | <norma> | <enlace> (consultado <fecha>) |
| Pico verdadero | −X dBTP | | |

## 2 · Subtítulos

- **Formato:** <CEA-608/708 | ARIB | DVB/teletexto>
- **En qué formato se reinsertan en la salida:** <...>
- **Obligación y exenciones:** <...> · Fuente: <enlace> (<fecha>)

## 3 · Alertas de emergencia

- **Sistema del país:** <EAS | EWBS | nacional: ...>
- **Drivers que aplican:** <...>
- **¿`signal-compare` es la vía realista?** sí | no · <por qué>
- Fuente: <enlace> (<fecha>)

## 4 · Registros y retención

| Registro | Retención | Artículo | Fuente |
|---|---|---|---|
| Bitácora de alertas | <N meses> | <...> | <enlace> (<fecha>) |
| <otros> | | | |

## 5 · Identificación de estación

- **Cada cuánto:** <...> · **Con qué:** <...>
- **Artículo:** <...> · Fuente: <enlace> (<fecha>)

## 6 · Formatos de casa típicos

- **Cuadros:** <59.94/29.97 | 50/25>
- **Los que se ofrecen primero:** <...>
- **Códec y contenedor de salida:** <...>

## 7 · Señalización de cortes

- **Qué espera la cadena:** <SCTE-104 al encoder | otro>
- **Texto obligatorio en la señal:** <línea de patrocinio, si aplica>
- Fuente: <enlace> (<fecha>)

## Lo que este software NO hace por tu cumplimiento

<Lo que queda fuera y hay que resolver por otro lado: hardware certificado
de alertas, torre, licencia, y lo que sea propio del país.>

## Lo que no se sabe todavía

<Lista honesta. Cada punto: qué falta y quién lo podría contestar.>

## Fuentes

<Todas las fuentes primarias, con enlace y fecha de consulta.>
```

---

## Para seguir

| Documento | Qué trae |
|---|---|
| [`PRD.md` §12](../../PRD.md#12--internacional-y-cumplimiento) | Los perfiles, la tabla de `us-fcc`, y ATSC 3.0 |
| [`COMPLIANCE.md`](../../COMPLIANCE.md) | Qué hace y qué **no** hace por tu cumplimiento legal |
| [`docs/drivers/README.md`](../drivers/README.md) | Cómo añadir soporte para un equipo |
| [`docs/ARQUITECTURA.md`](../ARQUITECTURA.md) | Cómo encaja un perfil en el resto del sistema |
| [`CONTEXT.md`](../../CONTEXT.md) | El glosario. Empieza por *Compliance Profile* |
| [`CONTRIBUTING.md`](../../CONTRIBUTING.md) | DCO, política de IA, convenciones |
| [`docs/adr/`](../adr/) | Las decisiones grandes y su porqué |
