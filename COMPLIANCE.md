# Cumplimiento — qué hace y qué NO hace Antena787

Este archivo existe para que nadie se lleve una sorpresa. Dice, sin adornos,
qué parte del cumplimiento legal de una estación toca este software, qué
parte no toca, y qué parte no es un requisito aunque lo parezca.

Está escrito para el dueño de una estación chica. No hace falta ser
ingeniero para leerlo.

---

## 1 · La postura: se ofrece, no se exige

**Así corren muchos canales locales en Puerto Rico y en el mundo, y no pasa
nada.** Un canal comunitario que sale al aire con una hoja de cálculo, un VLC
y buena voluntad está haciendo lo que puede con lo que tiene, y eso está bien.
Antena787 no viene a decirle a nadie que lo está haciendo mal.

**El software nunca regaña.** No dice "estás en violación". No bloquea nada
por cumplimiento. No da por sentado que el operador le debe algo a alguien.
No hay una pantalla roja que aparezca a exigir papeles.

Lo que hace es otra cosa: **tener listo lo que a veces alguien pide.** La
bitácora de alertas, el archivo de anuncios políticos, el registro del medidor
de volumen, la línea de "anuncio pagado por" en el crawl — todo eso corre solo
por debajo, y el día que llega una carta o una llamada está ahí, exportable,
sin que nadie haya tenido que acordarse de nada.

**Ese es el argumento entero: no un deber, un seguro que no cuesta trabajo.**

**Lo enciende el perfil del país.** En la instalación se pregunta en qué país
está la estación, y el perfil pone los números de la casa —el objetivo de
volumen, el formato de subtítulos, qué se guarda y por cuánto tiempo— sin que
nadie tenga que buscarlos. Al lanzar hay dos perfiles: `us-fcc` e `internet`.

**Y se puede apagar.** Un canal por internet no enciende ninguna de estas
funciones, y no tiene que explicar por qué. El perfil `internet` no lleva
obligaciones de radiodifusión: solo el objetivo de volumen de la web
(−16 LUFS) y nada más.

---

## 2 · El perfil `us-fcc`, función por función

Lo primero que pregunta el perfil es la **clase de licencia** —potencia
completa, Class A o LPTV— y cada una enciende solo lo que le toca. Se cambia
cuando se quiera. Todo lo de abajo es activable; nada es obligatorio para
que el canal salga al aire.

### Identificación de estación — 47 CFR 73.1201

El sistema pone la identificación cerca de cada hora. El **cartel de
respaldo** —el que se genera en la instalación con el nombre, el identificativo
y la comunidad de licencia— también la lleva, así que una caída larga sigue
identificando la estación en vez de dejar barras mudas.

Si pasa una hora sin identificación, queda **anotado en incidentes**. Anotado,
no bloqueado: el aire nunca se detiene por esto.

### Volumen (CALM) — 47 CFR 73.682(e), ATSC A/85

El objetivo es **−24 LKFS con pico verdadero de −2 dBTP**, y se aplica **una
sola vez, en el ingest**, cuando el archivo entra a la biblioteca. No se toca
la señal en vivo camino al aire.

Cada salida puede tener su propio objetivo: la del transmisor sale a
−24 LKFS y la de internet a −16 LUFS, del mismo material, sin configurar nada
dos veces.

**Su registro diario.** El sistema escribe solo, todos los días, una anotación
del tipo *"medidor activo, N archivos normalizados, 0 fallos"*. Es exportable.
Nadie tiene que llevar ese registro a mano ni acordarse de que existe.

### Bitácora de alertas de emergencia — retención de 24 meses

Cada vez que el equipo de alertas de emergencia reemplaza la señal del canal,
queda un evento anotado con su hora de inicio y de fin.

- **Distingue pruebas de activaciones reales.** Una prueba semanal (RWT) y una
  prueba mensual (RMT) no se cuentan igual que una alerta real.
- **Se guarda 24 meses**, que es lo que se suele pedir, y nada la borra antes.
- **Las pruebas semanales se comparan contra la ventana horaria configurada.**
  Si una cae fuera de esa ventana, el sistema lo dice. Lo dice, no lo impide.
- Cada evento sabe **de dónde salió**: automático (el sistema lo detectó por
  el equipo de alertas o comparando la señal transmitida contra el plan) o
  **marcado a mano**, que es una vía perfectamente válida y visible.

> **Sin retorno de aire, la detección automática no existe, y el sistema lo
> dice.** Antena787 mira la **señal transmitida**, no su propia salida: cuando
> el equipo de alertas reemplaza la señal aguas abajo, el motor sigue
> emitiendo y nunca se entera. Si la estación no puede dar un retorno de aire
> —una tarjeta receptora, una entrada de captura, un stream de monitoreo— el
> driver corre en modo **degradado**, la interfaz lo muestra, y las
> interrupciones se marcan a mano *(ADR 0009)*. Lo que no se hace es fingir
> que se verificó desde un sitio donde verificar es imposible.

### Subtítulos

Los subtítulos CEA-608/708 que traiga un archivo **se conservan siempre** y se
reinsertan en la salida. Los que vienen aparte (`.scc`, `.srt`, `.vtt`)
**se pueden subir siempre**.

Hay estaciones exentas de rotularlos, así que hay un ajuste de **tres
estados**:

| Estado | Qué significa |
|---|---|
| **Obligada** | La estación entiende que le aplica. |
| **Exenta** | Con el motivo anotado: ingresos, canal nuevo, o programación exenta. |
| **No sé todavía** | El default. |

**Los tres funcionan igual.** Los subtítulos embebidos siempre se conservan y
siempre se pueden subir. Lo único que cambia el ajuste es **si el sistema avisa**
cuando un programa sale sin subtítulos. "No sé" no bloquea nada y nadie tiene
que resolverlo antes de salir al aire.

**El motivo de exención más común, en números.** Un canal con **ingresos
brutos anuales de menos de $3,000,000** el año anterior está exento de
gastar en subtitular su programación — **47 CFR 79.1(d)(12)**. Es una
exención **autoaplicable**: no hace falta pedirle permiso a la FCC ni
presentar nada, a diferencia de la carga económica excesiva (§79.1(f)), que sí
requiere una petición formal. Otros motivos de exención con el mismo trato —
red nueva en sus primeros 4 años (§(d)(9)), programación local sin valor de
repetición y que no sea noticias (§(d)(8)), o el horario 2 a.m.–6 a.m.
(§(d)(5))— caben igual en "Exenta", con su propio motivo anotado.

*Nota de implementación (9 sept 2026): este ajuste de tres estados está
descrito arriba y en el perfil `us-fcc` del PRD (§12), pero **todavía no
existe como un control en la interfaz** — hoy "Subtítulos" en Ajustes →
Cumplimiento solo enseña un valor fijo ("se conservan"), sin selector ni texto
de ayuda. El umbral de $3M queda documentado aquí mientras se construye esa
pantalla.*

### Archivo de anuncios políticos — 47 CFR 73.1942 / 73.1943

Aplica **solo si la clase de licencia lo pide** (Class A y potencia completa).

Cuando un anunciante se registra como **político**, se le piden candidato,
cargo y elección. De ahí sale un archivo exportable con la solicitud, la
aceptación o el rechazo, las tarifas y los horarios. **Se guarda dos años.**

**La tarifa más baja aparece como aviso, no como un cálculo que el software
imponga.** El precio lo pone la estación; el sistema solo enseña lo que ya
vendió para que la decisión se tome con el dato delante.

### Identificación de patrocinio en clasificados — 47 CFR 73.1212

Todo clasificado del crawl sale con el prefijo automático
*"Anuncio pagado por ‹nombre›"*.

**En este perfil el prefijo no se quita.** Es una línea de texto, no cuesta
nada, y evita el único lío que sí es fácil de evitar. Es la única cosa de todo
este archivo que el perfil `us-fcc` no deja apagar.

### Archivo público en línea

Una exportación lista para subir, **solo si la licencia lo pide** — Class A y
potencia completa. **Un LPTV no la ve**: no aparece en el menú, no hay un aviso
pendiente, no existe.

### Fichas de programas — la licencia es de terceros, no de la FCC

Cuando "Buscarlas en internet" está encendido (Ajustes → Fichas de programas),
Antena787 busca sinopsis y carátulas en **TVmaze** por defecto, y en **TMDB**
si se pone una clave.

Esto no es un asunto de la FCC, sino de la licencia del proveedor: **TMDB es
gratis para uso no comercial, con atribución; el uso comercial —una estación
que vende publicidad, que es el caso normal de una estación de referencia—
exige un acuerdo comercial aparte con TMDB**, negociado por separado
([themoviedb.org/api-terms-of-use](https://www.themoviedb.org/api-terms-of-use)).
**TVmaze no tiene esa restricción**: es gratis bajo CC BY-SA 4.0, con
atribución y enlace de vuelta, sin distinguir uso comercial de no comercial
([tvmaze.com/api](https://www.tvmaze.com/api)).

Antena787 es software libre; quien lo instala y vende anuncios es el
**operador**, y es el operador quien decide si necesita ese acuerdo con TMDB
antes de poner la clave. Por eso TVmaze, sin clave, sigue siendo el proveedor
de menor riesgo legal por defecto.

### PSIP (A/65) y programación infantil E/I — de la clase de licencia, no de este software

Dos obligaciones reales de Class A y potencia completa **que Antena787 no
construye hoy**, y que dependen de la misma `clase_licencia` que ya decide el
resto de esta sección:

- **PSIP completo** (ATSC A/65, con el Anexo B de canal virtual) es
  obligación de Class A y potencia completa; un LPTV solo tiene la *opción*
  de llevarlo, no la obligación. Hoy lo genera —o no— el equipo aguas abajo
  (el multiplexor o el transmisor, según el fabricante); Antena787 no genera
  tablas PSIP.
- **Programación infantil educativa/informativa (E/I)**, del Children's
  Television Act: Class A y potencia completa deben emitir 156 horas al año
  (≥26 h por trimestre) de programación "core" para menores de 16 años, y lo
  reportan en el **FCC Form 2100, Schedule H**. **No aplica a un LPTV
  simple.** No hay hoy un campo para marcar un programa "core" E/I ni un
  contador de horas.

Ninguna de las dos bloquea nada ni se vende como resuelta: quedan anotadas
aquí porque son del tipo de obligación que este archivo existe para no
esconder. Ver
[`docs/investigacion/SUBTITULOS-Y-METADATA-2026-09-09.md`](docs/investigacion/SUBTITULOS-Y-METADATA-2026-09-09.md)
para el detalle y las fuentes.

---

## 3 · Lo que NO es requisito, y este proyecto no vende como tal

### El as-run

**El as-run —el registro de lo que salió al aire y a qué hora— no es un
requisito de la FCC.** La FCC eliminó los registros de programación, y la
bitácora de estación (73.1820) cubre otras cosas.

**El as-run es evidencia comercial.** Vale para facturar, para proponer una
reposición cuando algo tapó un spot, y para probarle a un anunciante que su
anuncio salió. Vale mucho, y por eso está en el centro del producto — pero
vale por eso, no por un reglamento.

**No se vende cumplimiento que no existe.** Si alguien le dice a una estación
chica que necesita un sistema de playout "para cumplir con el as-run de la
FCC", le está vendiendo algo que no es cierto.

---

## 4 · Lo que queda fuera de este software

### El equipo de alertas de emergencia

**Antena787 no es un ENDEC y no lo reemplaza.** Las alertas de emergencia se
cumplen con **hardware certificado**, aguas abajo del playout. Ese equipo es el
que interrumpe la señal, y es el que la ley reconoce.

Lo que Antena787 hace es **integrarse**: leer de ese equipo —por relés, por
serial, por red, o comparando la señal transmitida contra el plan— para saber
*cuándo* interrumpió y dejarlo anotado. **Ni lo reemplaza, ni lo certifica, ni
lo prueba por ti.**

### Las luces de la torre — Part 17

**Antena787 no toca las luces de la torre.** No las monitorea, no las
registra, no avisa si fallan. Eso es Part 17 y es otro asunto, con su propio
equipo y su propia bitácora.

### La licencia misma

El software no tramita, no renueva, no mantiene ni interpreta la licencia de
la estación. Solo pregunta de qué clase es para saber qué encender.

---

## 5 · La clase de licencia decide qué aplica

No todo le aplica a todo el mundo, y esa es la razón por la que **el sistema
pregunta en vez de asumir**.

| | LPTV | Class A | Potencia completa |
|---|---|---|---|
| Identificación de estación | sí | sí | sí |
| Volumen y su registro diario | sí | sí | sí |
| Bitácora de alertas (24 meses) | sí | sí | sí |
| Subtítulos (ajuste de tres estados) | sí | sí | sí |
| Identificación de patrocinio | sí | sí | sí |
| Archivo de anuncios políticos | no | **sí** | **sí** |
| Archivo público en línea | no | **sí** | **sí** |

La clase se contesta una vez y **se cambia cuando se quiera**. Lo que no
aplica no aparece: un LPTV no ve el archivo público en ninguna pantalla, no
como una opción apagada, sino como algo que no existe en su instalación.

*El despliegue de referencia (CAtv) es **Class A**, así que ahí se encienden
las funciones de Class A.*

---

## 6 · Otros países

**Antena787 no asume Estados Unidos en ninguna parte.** El perfil se elige
**por país**, no por continente — Colombia usa DVB-T2 estando en Sudamérica, y
por eso el continente no sirve como criterio.

| Perfil | Estado | Qué trae |
|---|---|---|
| `us-fcc` | **al lanzar** | Todo lo de la sección 2. |
| `internet` | **al lanzar** | Sin obligaciones de radiodifusión. Volumen −16 LUFS. |
| `eu-ebu` | futuro | Volumen **−23 LUFS**, subtítulos DVB y teletexto, 50/25 Hz. |
| `isdb-latam` | futuro | Subtítulos **ARIB**, alertas **EWBS**. |

> **−24 LKFS contra −23 LUFS parece trivial y no lo es.** Una universidad en
> España o en Colombia que instale esto con el perfil equivocado sale al aire
> con el volumen de otro país. El perfil pone el número de la casa sin que
> nadie tenga que buscarlo.

**Cada perfil requiere fuente citada y un mantenedor que opere en esa
jurisdicción**, o se marca **experimental** y lo dice en la interfaz. No se
escribe el cumplimiento de un país desde afuera leyendo un PDF.

Si tu país no está y sabes cómo funciona ahí, ese es exactamente el tipo de
contribución que el proyecto necesita: **[`docs/profiles/README.md`](docs/profiles/README.md)**
explica cómo aportar el perfil de un país.

---

## 7 · Esto no es asesoría legal

**Antena787 ayuda a cumplir. No certifica.**

Este archivo, el software, y todo lo que el sistema exporta **no sustituyen
asesoría legal ni al ingeniero de la estación**. Las citas del CFR que
aparecen aquí están para que sepas qué se está guardando y por qué, no como
una interpretación de lo que a tu estación le aplica. Quien decide eso es tu
abogado, tu ingeniero, o tú.

**Y de nuevo, porque es lo más importante de este archivo: las alertas de
emergencia se cumplen con hardware certificado, no con este software.**
