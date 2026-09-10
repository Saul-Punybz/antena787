# Antena787

**Software libre para lanzar y operar canales de televisión**

Autor: Saul Gonzalez · Licencia AGPL-3.0 · Contribuciones por DCO
Despliegue de referencia: Caribbean Advantage TV, Puerto Rico

> **Cómo leer este documento.** Aquí está **qué** hace el sistema y **en qué
> orden se construye**. El **porqué** de cada decisión grande vive en
> `docs/adr/`, y el vocabulario exacto en `CONTEXT.md`. Cada decisión se
> escribe en un solo sitio: es lo que impide que el documento se contradiga
> consigo mismo, que fue el problema de las siete versiones anteriores.
>
> **Sobre los números.** Todo dato marcado *(medido)* sale del análisis
> completo del Google Sheet de CAtv. Todo dato marcado *(estimado)* viene de
> reportes de terceros y **no lo ha verificado este proyecto**. La Fase 0
> convierte los estimados en medidos. Y **todo número de este documento es un
> valor por defecto configurable**, no una constante del motor.
>
> **Fuentes.** Auditoría de nueve agentes del 4 sept 2026:
> `docs/AUDITORIA_2026-09-04.md`; criterios de aceptación:
> `docs/ACEPTACION.md`.

---

## 1 · Qué es

Antena787 decide qué sale al aire, lo emite, vende y prueba la publicidad
alrededor, y publica la guía electrónica.

Sirve igual a una organización sin fines de lucro con un canal por internet
que a una televisora comunitaria con transmisor licenciado.

- Un solo ejecutable. Sin Docker, sin runtime externo, sin archivos de
  configuración que editar a mano.
- Windows, Linux y ARM desde el mismo código.
- Todo se opera desde el navegador.
- **Una sola dependencia externa: ffmpeg, empaquetado adentro.**
- **Cero inteligencia artificial en el producto.**

### La promesa

> Que a quien quiere lanzar un canal **solo le falte comprar el equipo**.

### El hueco de mercado *(verificado por investigación independiente)*

El playout comercial cuesta entre decenas y cientos de miles de dólares. Y
abajo no hay nada:

- Ningún comparador de la industria lista un competidor libre en *traffic*
  de televisión. Solo propietarios.
- **ErsatzTV y Tunarr no tienen SCTE-35 ni concepto de contrato, avail o
  as-run comercial** — verificado por búsqueda de código, no leyendo
  documentación.
- **Rivendell delega el traffic a software propietario de radio.** Nunca lo
  resolvió.
- El único intento completo documentado —CasparPlay con Adempiere, en Deepto
  TV, Bangladesh— **está muerto: 16 commits, Python 2.7.**

---

## 2 · Para quién

No hay un usuario, hay un rango, y el sistema sirve a los dos extremos sin
castigar a ninguno.

| | Extremo pequeño | Extremo grande |
|---|---|---|
| Quién | ONG, universidad, comunitaria | Grupo con varias señales |
| Canales | 1 | 2 a 20 |
| Gente | **una persona** | equipo con roles |
| Conocimiento | sabe instalar software y nada más | tiene ingeniero |
| Presupuesto | cercano a cero | limitado pero real |
| Qué corre | un canal de TV, o una radio que también sale online o por TV | TV y radio, varias de cada una |
| Dónde | Puerto Rico, Estados Unidos, o cualquier país con su perfil | ídem |
| Equipo | el que ya tiene, de la marca que sea | ídem |

**Ninguna marca ni modelo se asume.** El despliegue de referencia (CAtv,
§3) es donde se prueba primero, no el molde: cada equipo que aparece ahí es
un ejemplo de una familia que tiene driver, y las marcas más usadas en
Estados Unidos —ENDEC, transmisores, encoders— vienen con el suyo (§10).

Se sirve a ambos con **revelación progresiva**: el sistema arranca en su
forma más simple y cada capacidad avanzada aparece **solo cuando alguien la
pide**. Quien tiene un canal nunca ve multi-canal, ni roles, ni publicidad
hasta que registra su primer anunciante.

---

## 3 · La evidencia: qué muestra la demostración de CAtv

CAtv opera al aire hoy con un Google Sheet hecho a mano, un humano, VLC, un
servidor de streaming, otro VLC, el equipo de alertas y el transmisor.

**El Sheet que compartió Rolando es una demostración, no la operación
completa.** Lo mandó para enseñar *cómo* corre el canal hoy, y está
incompleto a propósito: no trae los anuncios, ni las pausas, ni la
programación entera. Por eso lo que se lee ahí no son errores de nadie. Es el
retrato del trabajo que hoy se hace a mano, y que la herramienta tiene que
volver un clic: **llenar el tiempo, cambiar el tiempo, interrumpir la
programación, añadir y quitar.**

**El Sheet merece respeto: alguien resolvió su problema con lo que tenía.**
Leído tabla por tabla el 4 de septiembre de 2026, esto es lo que muestra:

| Lo que muestra *(todo medido)* | El trabajo a mano que representa |
|---|---|
| **La guía es todavía la plantilla de ejemplo** | El canal dice `Caribbean Advantage TV` pero los programas apuntan a `sports1.channel`, con fechas de mayo y contenido inventado. Mantener la guía al día es hoy un trabajo aparte; Antena787 la genera del plan. |
| **Horas sin llenar** | 103 de 343 espacios semanales. El fin de semana ~10 horas contra ~5 entre semana, y de 1:00 a 6:00 AM los siete días. **Llenar el tiempo** es lo que hoy se resuelve a mano cada semana. |
| **18 fechas de fin en 60 días** | *Familia Robinson* terminaba el 6 de septiembre; *Los Simuladores* el 11. Hoy hay que acordarse; el sistema avisa a 30, 14 y 7 días. |
| **Fechas cruzadas de *Hellsing*** | Inicio 15 sep y **fin 1 ene**, y la hoja lo marca "✓ OK". **Cambiar el tiempo** sin que nada lo revise es lo normal en una hoja; en el esquema esa fila no se puede guardar. |
| **Sin inventario publicitario cargado** | La demo no trae anuncios. Capacidad de 518,400 segundos al mes (12 min/hora × 24 h × 30 días) que hoy nadie cuenta: **añadir y quitar** anuncios es justo lo que no existe. |
| **15 horas semanales son en vivo** | `RadioOnce Live!` ocupa de 10:00 AM a 1:00 PM de lunes a viernes. **Interrumpir la programación** para dar paso al vivo, y volver, es hoy manual. |

**Dos cosas que las versiones anteriores de este documento afirmaron y son
falsas**, corregidas al leer el Sheet completo:

- **El catálogo NO está vacío.** Son **118 títulos**, con 112 sinopsis, 114
  años y 105 clasificaciones. Está casi completo.
- **La columna "Duración" no es duración.** Sus valores son 1, 2, 6 y 10: es
  la **cantidad de episodios consecutivos por corrida**. *Gaming Longplays =
  10* es por lo que ocupa de 6 a 11 PM.

Nada de esto es exclusivo de CAtv ni de su tamaño: es el trabajo diario de
cualquier canal chico.

---

## 4 · Principios de diseño

Gobiernan cualquier decisión. Si algo los viola, está mal aunque sea elegante.

1. **Detectar antes de preguntar.** El sistema escanea la máquina y la red y
   averigua lo que pueda. Solo pregunta lo que no puede saber.
2. **Probar, no asumir.** La pregunta *"¿ves las barras de color?"* vale más
   que cincuenta campos correctos.
3. **Nada de jerga.** El usuario nunca ve *driver*, *códec*, *GOP*, *LKFS* ni
   *transport stream*.
4. **Siempre hay salida.** Cada pregunta tiene "no sé" con un valor por
   defecto que funciona.
5. **Cero archivos de configuración.** Existe uno, pero lo escribe la
   aplicación.
6. **El instalador hace el trabajo sucio.** Antivirus, energía, servicio,
   rutas, permisos.
7. **Los errores se explican en cristiano.** No *"no audio stream detected"*
   sino **"Este video no tiene sonido"**, y qué hacer.
8. **La hoja de cálculo es un remedio casero, no la vara de medir.**
9. **Revelación progresiva.** Nadie paga complejidad que no usa.
10. **Nada asumido sobre el país.**
11. **Cero IA en el producto.** 100% funcional sin un solo modelo.
12. **Nada de CGo.** *(ADR 0002)*

---

## 5 · Alcance

### Qué hace
Programa, emite, rellena huecos, publica guía, recibe señal en vivo, inserta
y contabiliza publicidad, **cobra y recibe el material del anunciante**, y
prueba lo que salió al aire. **Para televisión y para radio.**

### Qué NO hace, y por qué
- **No construye el equipo de alertas de emergencia.** Es hardware
  certificado aguas abajo. Antena787 se integra con él.
- **No construye códecs.** Eso es ffmpeg. *(ADR 0003)*
- **No forkea CasparCG ni ningún motor ajeno.** *(ADR 0001)*
- **No parchea ffmpeg.** *(ADR 0004)*
- **No escribe un muxer de MPEG-TS propio.**
- **No tiene salida por tarjeta SDI ni NDI.** Esos SDK obligan a enlazar C,
  lo que viola el principio 12. Queda fuera del alcance y de las builds
  oficiales. *(ADR 0002)*
- **No se integra con Plex ni Jellyfin.** Plex es inspiración de interfaz,
  no una dependencia. *(ADR 0003)*
- **No construye publicidad direccionable.** Emite la señal estándar para que
  otros la construyan. *(ADR 0004)*
- **No genera clips ni video vertical para redes**, ni maneja redes sociales.
- **No gestiona derechos de contenido.** Sin territorios, sin vías de
  distribución, sin conteo de corridas. Solo fechas de inicio y fin por regla.
- **No lleva IA adentro.** *(ADR 0007)*

---

## 6 · Radio y televisión

**Una emisora de radio es un canal de televisión sin video.** Reglas, plan,
as-run, cortes publicitarios, fuentes en vivo, grabación de la salida,
diferido y control manual son idénticos. Cambia el formato de casa —solo
audio— y la salida.

Y hay razón de mercado: en LATAM el mismo operador chico suele tener radio y
TV. `RadioOnce Live!` es literalmente una radio simulcasteada por la antena de
CAtv.

**Un canal de radio puede salir por tres lados a la vez** — la antena de FM,
internet, y un canal de televisión— sin ser tres canales. El formato de casa
es audio; la salida de TV y la de internet reciben ese audio con el **cartel
visual** que dibuja el servidor de cuadros: carátula, título que suena, logo,
crawl. Nada se configura dos veces: las reglas, los cortes y el as-run son
uno. Y el **reloj de cortes** —a qué minutos de la hora se va a comerciales—
es propio de cada fuente en vivo de radio, y de cada emisora de radio como
canal; el sistema lo pide una vez y alinea los cortes con él.

| | Funciona hoy con lo diseñado |
|---|---|
| **Radio hablada, deportiva, de programas** | Sí. Programa por bloques, igual que TV. |
| **Radio musical** | **No completamente.** Necesita rotación musical. |

> **La rotación musical es un subsistema aparte, no un ajuste.** La radio
> musical no programa por título sino por **categorías con relojes por hora**
> y reglas de separación —que no se repita el mismo artista en tres horas, que
> no caigan dos baladas seguidas—. Va como fase propia (§22), no se asume
> gratis.

---

## 7 · Los dos modos

**Modo `internet`** — YouTube, Facebook, Twitch, servidor propio, HLS. Sin
obligaciones regulatorias de radiodifusión. Volumen −16 LUFS.
**Para la mayoría de los usuarios este es el modo principal, no un extra.**

**Modo `transmisor`** — Alimenta un transmisor licenciado. Ofrece el perfil
del país (§12) con sus bitácoras y registros, listos por si alguien los pide.

**Los dos a la vez.** Un canal puede salir por transmisor **y** por internet.
Es casi gratis porque el contenido ya está normalizado. Impone dos cosas:

1. **Volumen por salida.** Abierta pide −24 LKFS, la web −16 LUFS.
2. **Reconexión automática.** Un envío de 24/7 se cae siempre.

> **Sin control de derechos por distribución.** Antena787 no lleva registro de
> qué está licenciado para qué vía. Si un programa solo tiene derechos de
> antena y se manda a YouTube, **nada lo impide** — es responsabilidad de
> quien opera el canal.

---

## 8 · El vocabulario

El glosario completo está en **`CONTEXT.md`**. Los cuatro términos que hay
que entender antes de seguir leyendo:

- **Regla** — la intención del humano: un título, un patrón de días, una hora
  y **una fecha de inicio y una de fin**. Es lo que se edita.
- **Plan** — la emisión resuelta a instante exacto, con archivo concreto y
  duración medida. Se materializa 48 horas por adelantado.
- **As-run** — el plan después de salir al aire, con las horas reales. **No
  es otra tabla: es la misma fila en otro estado.**
- **Día de emisión** — empieza a las 6:00 AM por defecto, no a medianoche, y
  **se cambia a cualquier hora** en Ajustes. Un programa de las 12:30 AM del
  martes pertenece al lunes.

---

## 9 · Cómo funciona, paso a paso

### Paso 1 — Entra un archivo

Se arrastra a la ventana o cae en una carpeta vigilada. El sistema:

0. Espera a que el archivo **termine de copiarse** antes de tocarlo. Windows
   lo bloquea mientras se copia; sin un tiempo de espera, la carpeta vigilada
   lo lee a medias y lo manda a cuarentena por error. "Terminó de copiarse"
   quiere decir **tamaño estable durante 10 segundos y el archivo se abre sin
   bloqueo** — un default configurable, como todos los números de este
   documento.
1. Lo mide con `ffprobe`: códec, resolución, cuadros por segundo, y
   **duración real al milisegundo**.
2. Mide el volumen y lo corrige al objetivo del perfil del país.
3. Detecta subtítulos y **garantiza conservarlos** — y esto no es gratis:
   como el motor decodifica y recodifica (ADR 0001), los CEA-608/708
   embebidos en el video **se pierden si no se extraen y se reinsertan
   explícitamente en el mux de salida.** Es un paso propio del pipeline, no
   un efecto secundario. **Y se pueden subir aparte:** un archivo `.scc`
   (que ya es CEA-608) o `.srt`/`.vtt` al lado del video, o desde la ficha
   en Biblioteca, y el sistema lo convierte y lo mete en la salida como si
   viniera embebido. La conversión de `.srt` a 608 es código propio en Go;
   no hay librería que lo haga sin C.
4. Detecta negro y silencio en cabeza y cola, y recorta el slate, las barras
   y los comerciales viejos del material de archivo.
5. **Normaliza al formato de casa** — un códec, una resolución, GOP cerrado,
   un audio. Una sola vez, en segundo plano.
6. Extrae un cuadro como miniatura.
7. Busca su ficha y su carátula en ese orden: **etiquetas embebidas, `.nfo`
   de Kodi al lado, carátula dentro del contenedor**, y solo si falta algo,
   la red — **Cover Art Archive para música y TVmaze para series**, que no
   piden clave, y **TMDB** si la estación quiso configurarlo (§10).
8. **Si algo falla, va a cuarentena. Lo que falla no llega al aire, nunca.**

> **Y la cuarentena tiene una puerta.** A veces el que manda es el dueño: un
> spot que el sistema rechaza por dos decibeles, con el cliente esperando, se
> puede sacar con un botón que dice lo que dice — **"Dejarlo pasar bajo mi
> responsabilidad"** — y queda registrado con nombre y hora. El software
> protege por defecto; no manda.

> **Hay material que abre en negro a propósito.** Un clip marcado
> `negro_intencional` no dispara el detector de negro y silencio mientras
> dura. Se marca en la biblioteca, en el archivo, no en la emisión.

### Paso 2 — Se arma una regla

*"Kojak, lunes a viernes, 8:00 AM, del 9 de agosto al 20 de diciembre, un
episodio por corrida."*

Las fechas de inicio y fin **no son un sistema de gestión de derechos** —
Antena787 no lo tiene, a propósito. Son dos campos que el programador ya
mantiene hoy, y de ellos salen los avisos de vencimiento y la imposibilidad
de guardar una regla que termina antes de empezar.

Unas 35 reglas arman un canal entero. **Nadie escribe 336 celdas nunca.**

Una regla puede además: emitir **N episodios seguidos** avanzando en la serie
y recordando dónde quedó; **relevar** a otra regla en la misma franja cuando
la primera vence; o **repetir a otra regla**, que es como se arma la
retransmisión del día.

> **Las repeticiones no llevan contador propio.** Una regla con `repite_a`
> apuntando a su regla primaria emite **el mismo episodio** que la primaria
> puso ese día de emisión. Si la primaria no emitió nada ese día, la
> repetición toma el siguiente episodio de la primaria y avanza el contador
> **compartido**. Hay un solo contador por serie en su franja, así que la
> serie nunca se estanca ni se desincroniza — que es lo que pasa cuando la
> repetición de las 11 PM lleva su propia cuenta aparte de la de las 2 PM.

> **Las fechas son días de emisión, no días de calendario.** `fecha_fin` es
> inclusiva hasta el cierre del día de emisión: 5:59:59 AM del día calendario
> siguiente, si el día empieza a las 6:00 AM. El patrón de días (LMMJVSD) se
> lee igual. Un programa de las 2:00 AM del martes pertenece al lunes, y una
> regla que termina "el lunes" lo incluye.

> **No hay reglas anuales.** El patrón es semanal y las fechas son absolutas,
> así que el 29 de febrero no es un caso especial: no existe.

**Una regla puede ser un bloque arrendado.** En una emisora chica es común que
alguien compre la hora completa —un programa religioso, un espacio de salud— y
pague por ella. Ese tipo de regla lleva su anunciante y su cobro, así que el
bloque aparece igual en la parrilla y en el reporte de ingresos, en vez de
vivir en una libreta aparte.

### Paso 3 — El resolver arma el plan

Cada hora, el resolver materializa **48 horas de plan**:

- Se niega a programar una regla fuera de su rango de fechas.
- Detecta conflictos de verdad: solapes, regla vencida, archivo faltante o en
  cuarentena. **Y entiende que un relevo no es un conflicto.**
- **Rellena todo hueco** con una combinación exacta de la biblioteca de
  relleno. Es un problema de empaquetado, determinista.
- **Solo programa material listo para aire.** Un archivo que todavía se está
  normalizando no entra al plan. La cola de normalización se ordena por
  **cuándo sale al aire** —lo que sale antes se normaliza antes—, y
  Biblioteca y Anuncios muestran "aún no listo para aire" mientras tanto. Una
  compra del portal no se da por lista hasta que termina.
- **Avisa de vencimientos a 30, 14 y 7 días.** El aviso aparece en Al aire, en
  Reglas y en Parrilla · Mes, y a los 7 días sale además por el canal de
  avisos. **Se calla solo si ya hay relevo cargado**: una regla que termina y
  tiene quien la sustituya en la misma franja no es un problema, y avisar de
  ella entrena al operador a ignorar los avisos.

**El reloj manda sobre el contador de episodios.** Si una regla pide diez
episodios seguidos y el décimo no termina antes del siguiente inicio duro, el
resolver **no lo arranca**: rellena el resto del espacio y avisa "de 10
episodios caben 9". El contador avanza solo por lo que salió. **Nunca se
corta un episodio a la mitad porque no cupo.** Lo mismo vale para un bloque en
vivo: el fin del bloque es duro.

**Un programa que cruza el inicio del día de emisión pertenece al día de su
inicio.** Un bloque de tres horas que arranca a las 5:00 AM es del día
anterior completo, y así lo cuentan los reportes y el diferido.

Se puede **revisar y corregir el aire de mañana antes de que ocurra.**

**La guía se regenera con el plan.** El resolver la reescribe en cada corrida
y **también en el instante en que el plan cambia por cualquier motivo**. Se
sirve siempre en `/guia.xml`, se escribe además en una ruta configurable —el
archivo local que lee el servidor de streaming o el transmisor— y,
opcionalmente, se manda por HTTP a un destino. **La guía nunca puede ir más
de un minuto atrasada respecto al plan.**

### Paso 4 — Sale al aire

El motor lee el plan y ejecuta el reloj. *(ADR 0001)*

- **Un proceso ffmpeg que nunca muere** produce la salida continua, con
  aceleración por hardware.
- **Un decodificador por clip** le entrega video y audio crudos por tubería.
- **Conformado por clip:** rellenar con silencio si el audio queda corto,
  sostener el último cuadro si el video queda corto, ajustar la geometría.
- **Pre-roll:** el siguiente clip arranca antes de necesitarse.
**Los decks: quién tiene el aire.** Un canal no tiene una sola cola sino
cuatro, y **en cada instante el aire lo tiene el deck de mayor prioridad que
tenga algo que poner:**

```
manual      ← mientras el operador retiene el control
comercial   ← cuando toca un corte, a su hora de reloj
programa    ← la parrilla normal
relleno     ← cuando ninguno de los anteriores tiene nada
```

Viene de cómo funciona la radio de verdad, donde la lista musical y la
comercial son dos colas paralelas y la comercial se impone a su hora. La
razón de fondo es que tienen amos distintos: **la de programa va por
secuencia** —una canción termina y entra la siguiente— y **la comercial va
por reloj** —el spot vendido para las 8:00 tiene que salir entre 8:00 y
8:15—. Meterlas en una sola lista obliga a recalcular todo cada vez que algo
dura distinto de lo previsto.

**Qué le pasa al programa cuando el corte toma el aire — dos comportamientos,
según de dónde viene el programa:**

| Origen | Durante el corte | Después |
|---|---|---|
| **Archivo** | El programa **se pausa** | Reanuda donde iba. **La duración del slot ya incluye los cortes**: un espacio de 30 minutos es 24 de programa más 6 de cortes, en los puntos marcados en el archivo (`marcas_de_corte_ms`). Vienen del sindicador o se detectan por negro y silencio en medio del archivo — y son propiedad del archivo, no de la emisión, para que el diferido las respete igual. |
| **Vivo** | La señal **sigue corriendo debajo** y esos minutos **se pierden** | Vuelve a la señal en el instante actual; el bloque **no se extiende**, termina a su hora. Por eso los cortes deben caer donde la fuente también corta (§9, paso 5). |

**El deck manual no funciona por "tener algo que poner": funciona por
retención.** Cuando el operador toma el control, el deck manual **retiene el
aire** — y el operador puede estar diez minutos sin tocar nada porque hay una
entrevista en vivo con el micrófono abierto. Por eso el temporizador de
seguridad **no mide si el operador dispara algo: mide si sale señal.** Se arma
sobre **silencio o negro real en la salida** —audio bajo −60 dBFS o luma bajo
16 durante más de 15 segundos, configurable— y solo entonces suelta el
control y registra un incidente. Un operador narrando diez minutos seguidos
nunca lo dispara; un operador que se fue y dejó el canal mudo, sí.

> **Este detector de silencio y negro en la salida es el mismo que vigila el
> aire en automático** (§9, paso 4). No son dos mecanismos: es uno, y el
> modo manual solo cambia qué hace al dispararse.

**Elementos dentro de un bloque en vivo.** Un ID de estación, una cortinilla o
una cama musical programados *dentro* de RadioOnce Live! son `plan_item` del
deck programa con `dentro_de` apuntando al bloque: **mientras suenan, toman el
aire; al terminar, la señal en vivo regresa.** Los cortes *vendidos* dentro del
mismo bloque son deck comercial y se imponen por prioridad. **Dos mecanismos
distintos, ambos válidos, y el modelo los distingue.**

- **Cascada de respaldo: programa → relleno → cartel. Nunca negro.** El
  **cartel es el último escalón**, y lleva el identificativo de la estación y
  su comunidad de licencia — lo genera el asistente en el primer paso, así
  que un canal recién instalado ya lo tiene. Con eso, una caída larga sigue
  identificando la estación. **Las barras y el tono existen solo para la
  prueba del asistente**, no como respaldo del aire. Si el cartel pasa de 15
  minutos al aire, se registra un `incidente.tipo=cascada_extendida`.
- **Watchdog del encoder.** Si el encoder no consume un cuadro en 3 segundos
  (configurable), el motor manda el cartel al aire, mata el encoder y lo
  relanza con **el mismo acelerador**. Si vuelve a fallar dos veces en 10
  minutos, lo relanza por software y avisa en cristiano: *"tu tarjeta de
  video dejó de responder"*. Si lo que pasó es que `ffmpeg` **desapareció del
  disco**, el watchdog lo distingue y lo dice: *"tu antivirus bloqueó
  ffmpeg — vuelve a aplicar las exclusiones en Ajustes"*.
- **Detector de silencio y negro sobre nuestra propia salida** —lo que
  mandamos al encoder, no el retorno de aire (ese responde otra pregunta:
  qué salió de verdad, §9 paso 8)— y no sobre los archivos: audio bajo −60 dBFS o luma bajo 16 durante más de 15 segundos
  —configurable— dispara alarma e incidente, **diga lo que diga el plan.** Un
  archivo puede pasar el ingest perfecto y salir mudo por un desajuste de
  pista; una fuente en vivo puede quedar "conectada" y congelada en negro. Es
  el mismo detector que gobierna el regreso del modo manual (§9, paso 6).
  **Un clip marcado `negro_intencional` lo desactiva mientras dura** —hay
  material que abre con quince segundos de negro a propósito—; una fuente en
  vivo nunca lo desactiva.
- **Un archivo que falla al aire vuelve a cuarentena solo**, con el motivo,
  para que no se programe otra vez la semana que viene y falle igual. Un
  error de lectura a mitad de clip —archivo borrado, archivo en cero, el NAS
  que se reinició— cuenta como fallo de clip y dispara la cascada; pero **el
  archivo solo se pone en cuarentena tras dos fallos separados por más de
  cinco minutos**, para que un parpadeo de red no saque de la parrilla un
  programa bueno.
- **Alarmas que se distinguen entre sí.** `encoder_no_responde` no es lo
  mismo que `enlace_caido` —el cable de red desconectado, leído del estado de
  la interfaz— y ninguno es lo mismo que un fallo de clip. Cada uno dice qué
  pasó y qué hacer.
- **Si el motor encuentra dos `plan_item` solapados en el mismo deck y la
  misma salida**, reproduce el de menor `id` y registra
  `incidente.tipo=solape`. El esquema ya lo impide (§15); esto es el cinturón
  además del tirante.

> **Un punto ciego dicho de frente: la salida UDP no sabe si alguien la está
> escuchando.** Mandar un flujo a una dirección que nadie lee no produce
> ningún error. Por eso la verificación de lo que de verdad salió es local y
> se hace contra el retorno de aire (§9, paso 8), no contra el éxito del
> envío.

**La parrilla es un contrato con la hora de pared.** Cuando el equipo de
alertas interrumpe, el playout sigue corriendo y al soltar ya está en el
minuto correcto. La precisión no sale del reloj del sistema operativo sino de
la continuidad de marcas de tiempo del flujo.

### Paso 5 — Una fuente en vivo entra

El equipo de streaming —OBS, en el caso de CAtv— **empuja a Antena787**, que
escucha RTMP o SRT en la red local. De ahí sale al transmisor **y a la vez**
a YouTube y Facebook.

> **La dirección importa.** Si el transmisor jalara la señal desde YouTube,
> un fallo de YouTube sacaría del aire a una estación licenciada. El equipo
> empuja una sola vez, a Antena787, y Antena787 reparte.

**Una fuente en vivo puede ser solo audio.** `RadioOnce Live!` es un programa
de radio que llega por IP y sale por televisión de vez en cuando: el audio es
la señal, y el video es el cartel del programa —carátula, nombre, logo— que
el servidor de cuadros dibuja, igual que en la pantalla de Música. Nada de
eso se configura: si la fuente no trae video, sale el cartel.

**Se prefiere SRT sobre RTMP**: RTMP añade típicamente de 2 a 5 segundos de
latencia, y eso alimentando un transmisor es mucho. SRT baja de un segundo sin
depender de ningún SDK propietario.

El bloque en vivo reserva su hora en el plan, y **el bloque se respeta
completo aunque la señal llegue tarde, se caiga o no llegue nunca**:

- **Margen de gracia** (`gracia_s`, 30 segundos por defecto). Casi ninguna
  señal en vivo entra al segundo exacto. Dentro del margen, la salida sostiene
  el último cuadro del programa anterior o pone el cartel, **sin alarma**:
  llegar a las 10:00:45 es normal, no una falla.
- **Pasado el margen entra el relleno y suena la alarma**, pero el bloque
  sigue reservado y el motor **sigue intentando conectar** con espera
  progresiva (1, 2, 4… hasta 60 segundos). Cuando la señal aparece, el aire
  vuelve al vivo **en el siguiente borde de clip de relleno** —máximo un
  minuto: si el clip de relleno dura más, se corta con fundido de un segundo,
  porque el relleno sí se corta y el vivo vale más— con fundido cruzado.
  Nadie tiene que ir a apretar nada.
- **Si la señal se cae a mitad del bloque**, es exactamente el mismo camino:
  relleno, alarma, reintento, y regreso al vivo cuando vuelva.
- **El bloque termina a su hora.** Fundido de audio de dos segundos y corte al
  minuto planificado. El vivo no se roba el programa siguiente.
- **La señal en vivo pasa por el mismo conformado que un archivo**: escala,
  pillarbox si viene con otra forma, y volumen al objetivo de la salida. No
  hay una ruta "cruda" para el vivo.

> **Y hay una entrada de vivo siempre abierta, sin configurar nada.** Un
> puerto SRT fijo que existe desde la instalación y aparece en *Al aire* como
> "entrada rápida". Cuando pasa algo —una emergencia, un evento que nadie
> planificó— alguien apunta OBS ahí y el operador lo pone al aire con el botón
> de tomar el control. Sin crear una fuente, sin editar la parrilla, sin
> llamar a nadie.

**Y dentro del bloque en vivo también hay parrilla.** Es donde hoy se hace
todo a mano: los cortes de publicidad, los ID de estación, la música entre
segmentos, las cortinillas. Todo eso son elementos programados *dentro* del
bloque, y salen a su hora sin que nadie toque nada. La salida cambia de la
señal en vivo al material y regresa sola — **es el mismo mecanismo de cambiar
la entrada del encoder que ya usa el motor**, no un subsistema nuevo.

> **La señal en vivo no se detiene durante el corte.** Los minutos que dure el
> corte se pierden en la salida. Por eso los cortes deben alinearse con la
> fuente, y hay dos formas:
>
> - **Reloj fijo** — si la radio se va a comerciales a los 18 y a los 48, ahí
>   van los de Antena787. Sirve para radio musical y programas con estructura.
> - **Señal de corte desde la fuente** — para radio hablada, deportes o
>   eventos, donde el corte lo decide el talento en el momento. Un driver de
>   entrada de cue: **contacto seco desde la consola del estudio, tono
>   subaudible** (técnica de décadas en redes de radio), o SCTE-35 si la
>   fuente lo trae. Mismo patrón de "cualquier marca" que los drivers de
>   alerta.
>
> Y con señal de corte hace falta **margen para reaccionar**: un retardo en la
> fuente en vivo —el *profanity delay* de siempre, **7 segundos por
> defecto**, configurable— que permita cortar limpio sin comerse la primera
> palabra al volver. Es un búfer de segundos, no de minutos, y no rompe el
> reloj de pared.

### Paso 6 — Alguien toma el control, y lo suelta

Todo corre en automático por defecto. Pero hay días en que el operador quiere
manejar un programa a mano, y **eso no puede obligarlo a quedarse toda la
noche.**

```
        AUTOMÁTICO  ← por defecto, siempre
             │
   [ Tomar el control ]
             ↓
          MANUAL  ← dispara lo que quiera: música, cortinilla, corte
             │
             ├─ pulsa "Volver al automático"  → vuelve, con fundido
             ├─ termina el bloque              → vuelve solo
             ├─ silencio o negro real en la salida
             │  más de 15 s (configurable)      → vuelve solo, y registra
             │                                    un incidente
             ├─ pulsa "Parar todo"              → corta en seco, marca parcial
             └─ se cayó el sistema               → arranca en automático
```

**"Inactivo" nunca significa "el operador no tocó nada".** Significa que no
está saliendo señal. Una entrevista de diez minutos con el micrófono abierto
es actividad; un canal mudo con el operador ausente, no.

**Antes del regreso forzado hay una escalera, no un timbre al final:** la
cuenta regresiva está visible **todo el tiempo** que se retiene el control;
el color pasa a **ámbar a 60 segundos** del regreso; y en los **últimos 10
segundos** pasa a rojo y suena un tono que se repite y se acelera. Los tres
números son defaults configurables. La alarma final es distinta del aviso.
Sin la escalera, el regreso es una sorpresa — y la sorpresa es exactamente lo
que hace reaccionar mal a un humano bajo presión.

**Al regresar entra por el minuto que le toca**, con fundido, no con corte
seco ni reiniciando el bloque. Y **el regreso por inactividad queda
registrado como incidente**, con su hora: para una estación licenciada es la
evidencia de que el sistema se protegió solo.

**La tercera salida es la importante.** El modo manual es el estado más
peligroso de un playout porque el humano se puede ir, distraer o quedarse sin
luz. **Nunca se queda en manual:** si el plan dice que algo debería estar al
aire y nadie disparó nada, el sistema se devuelve al automático y avisa.

Como el motor va guiado por el reloj de pared, al soltar **ya está en el
minuto correcto**, no arrancando donde se quedó.

Mientras tiene el control aparece un panel de disparo — pero **solo mientras
lo tiene.** No es la forma de operar el canal.

**Soltar el control no corta nada por el medio.** "Volver al automático"
espera a que termine el clip que está sonando —máximo un minuto— y entonces
entrega el aire. Para el caso en que sí hay que cortar en seco existe **"Parar
todo"**, que corta al instante y marca el `plan_item` interrumpido como
`parcial`. Es la diferencia entre soltar el control a mitad de un spot pagado
y hacerlo cuando el spot terminó.

**Hay un solo dueño del control a la vez.** Si una segunda persona abre Al
Aire y pulsa Tomar el control, ve *"Rolando tiene el control desde las 3:12
PM"* y un botón explícito para quitárselo. Quitárselo se permite —a las 3 AM
puede no haber a quién llamar— y queda registrado con nombre y hora en la
bitácora de cambios.

**El motivo por el que terminó cada retención queda anotado**: soltado, fin
de bloque, regreso por inactividad, o **caída del sistema** — porque un
reinicio a mitad de una retención manual también es una explicación, y sin
ese motivo el registro miente.
### Paso 7 — Se marca un corte publicitario

Antena787 manda **SCTE-104 al encoder**, y el encoder genera el SCTE-35.
*(ADR 0004)*

Con esa señal la estación queda enchufada —sin construir nada más— a
inserción del lado del servidor, publicidad direccionable y exchanges
programáticos.

**No hay ruta alterna en la versión 1.** La idea de inyectar las secciones
aguas abajo, en el flujo ya multiplexado, cuesta de tres a cuatro veces lo que
cuesta el crawl de clasificados, **no tiene un caso de referencia documentado
en ningún proyecto libre**, y en la práctica es escribir un remuxer — justo lo
que el proyecto decidió no hacer *(ADR 0004)*. Queda apuntada para **F5** y
marcada como **sin precedente documentado**.

**Qué pasa si el encoder de la estación no habla SCTE-104.** No pasa nada
grave: se venden y se emiten los cortes **localmente**, con su evidencia de
emisión completa, y lo único que no se puede es enchufarse a sistemas de
inserción de terceros. Es exactamente lo que hace CAtv hoy, y sigue siendo un
negocio. El de CAtv sí lo habla (§25).

### Paso 8 — Se graba la salida, y de noche se retransmite

Antena787 **graba lo que sale al aire, de corrido.** La grabación sirve para
tres cosas: la bitácora de lo que de verdad se emitió, la base del driver
`signal-compare` que detecta cuándo el equipo de alertas interrumpió, y poder
retroceder para revisar sin dejar de ver el vivo.

> **Dónde se captura, y por qué es la decisión que da valor a todo lo
> demás.** La verdad de lo que salió es **la señal transmitida**, no lo que
> nuestro proceso creyó mandar. Si la grabación y `signal-compare` miran la
> salida propia de Antena787 —antes del equipo de alertas— una interrupción de
> emergencia real **no se ve nunca**: el motor siguió emitiendo tan tranquilo,
> el as-run queda limpio y falso, y la bitácora de alertas es ficción. Por eso
> **la captura va después del equipo de alertas**: un retorno de aire —entrada
> de captura, receptor de la señal, o el propio flujo del transmisor— definido
> en `capture_input`. *(ADR 0009.)*
>
> **Si la estación no puede dar un retorno de aire, se dice sin rodeos:** el
> driver corre en **modo `degradado`**, `signal-compare` no promete detectar
> nada, y las interrupciones se marcan a mano —también hacia atrás, sobre un
> tramo ya emitido— desde la pantalla de incidentes. Cada evento de alerta
> guarda si lo detectó el sistema o lo marcó una persona.

**El as-run es evidencia comercial, no un requisito de la FCC.** Conviene
decirlo claro porque en la industria se repite lo contrario: la FCC eliminó
los registros de programación, y la bitácora de estación (73.1820) cubre
otras cosas. El as-run existe para **facturar, reponer un spot tapado y poder
probarle a un anunciante que su anuncio salió** — que es plata, no papeleo.

**El diferido llena las horas vacías con lo que ya existe.** En vez de
rellenar las madrugadas con promos repetidas, **se retransmiten los programas
de la mañana.** Contenido que ya está en la biblioteca, sin trabajo de
programación.

Se configura como una regla más: *"de 1:00 a 6:00 AM, repite lo que salió de
7:00 AM a 12:00 PM."*

> **El diferido reprograma los archivos del plan; no reproduce la grabación.**
> La diferencia importa por tres razones:
>
> - **Los anuncios de la mañana no se repiten.** Si se reprodujera la
>   grabación, los spots de las 7 AM saldrían otra vez a la 1 AM — ¿se cobran
>   doble? ¿se regalan? Reprogramando los archivos, **los cortes de la
>   madrugada son inventario nuevo**, vendible aparte.
> - **Las alertas de emergencia grabadas no se retransmiten.** Un aviso de
>   tormenta de las 9 AM no tiene por qué salir otra vez a las 3 AM.
> - **Nunca hay recursión.** Un diferido solo toma programas cuyo origen fue
>   un archivo. Lo que en la ventana original fue relleno, o fue a su vez un
>   diferido, se salta y ese hueco se rellena normalmente.
>
> - **Una ventana sin programa no produce diferido.** El diferido copia solo
>   `plan_item` del deck programa. Si en la ventana original no hubo programa
>   —estaba vacía, o fue relleno— el diferido no genera nada y esa hora se
>   rellena como cualquier otra. No se inventa contenido para cumplir la
>   regla.
>
> **La única excepción es el vivo:** un bloque en vivo no tiene archivo, así
> que para ese tramo el diferido usa la grabación — y **sin sus cortes**, que
> se sustituyen por cortes nuevos.

*Costo: disco. A 720p, un búfer rodante de 24 horas son unos 50 GB. **La
retención por defecto es de 7 días, y sube a 30 si el disco alcanza** —una
estación chica tiene entre 1 y 10 TB, y el sistema calcula solo cuántos días
caben después de la biblioteca. Cuando hay un SSD chico y un disco grande
—CAtv: 500 GB y 3 TB— el sistema y la base van al SSD, y la biblioteca y la
grabación al disco grande; el asistente lo propone así y se puede cambiar. Se comunica en días de historial, no en
gigabytes.*

### Paso 9 — El anunciante paga y entrega su anuncio, solo

Hoy vender un anuncio en una estación chica son siete pasos y **todos son del
dueño**: conseguir el cliente, cotizar, facturar, perseguir el pago, recibir el
archivo por donde sea, revisarlo y programarlo.

Antena787 le da **un enlace por anunciante**, y con eso el anunciante:

1. Ve qué está comprando y cuánto cuesta
2. **Paga con tarjeta** *(Stripe, opcional)*
3. Arrastra su anuncio — sin cuenta, sin contraseña, sin FTP
4. **Recibe respuesta al momento**, porque el archivo pasa por el ingest ahí
   mismo:

> ✅ *Listo. Tu anuncio dura 30 segundos exactos y sale desde el lunes.*
> ⚠️ *Dura 32 segundos y compraste 30. ¿Lo cortamos o subes otro?*
> ❌ *Este archivo no tiene sonido.*

**El dueño de la estación no toca nada después de mandar el enlace.** No
persigue el pago, no persigue el archivo, y los archivos malos se rechazan
antes de ser su problema.

**Con una excepción a propósito: nada sale al aire por primera vez sin que
una persona lo vea.** Toda compra —spot o clasificado— entra a una **cola de
aprobación** antes de su primer aire. El portal es la única puerta pública del
sistema, y quien la cruza no es el operador: sin ese paso, cualquiera con el
enlace decide qué ve la comunidad. Aprobar es un vistazo de diez segundos y
un botón; a partir del segundo aire, el spot ya está aprobado y no vuelve a
preguntar.

**El enlace del portal tiene dueño y fecha.** Es un token largo, ligado a un
anunciante, que **vence a los 30 días** y se puede revocar de un clic. Si el
enlace se filtra —se reenvía por WhatsApp, aparece en redes— quien lo abra no
puede subir material a nombre de otro anunciante ni cambiar lo comprado.

**Límites que se explican en cristiano.** Un spot puede pesar hasta **500 MB**
y durar hasta **5 minutos**; pasado eso el portal lo dice con esas palabras y
no con un error de servidor. Lo subido se examina en una carpeta aparte —sin
red, con límite de tiempo— y solo entonces entra a la biblioteca (§19).

**Pagar y entregar son dos cosas distintas, y el sistema lo sabe.** Un pago
puede llegar sin archivo: esa orden queda en estado **"pagado, falta el
material"**, con recordatorio automático a las 24 y a las 72 horas. Y cada
pago lleva su propia referencia única, así que un aviso repetido de la
pasarela **no cobra ni acredita dos veces**.

**Donde más rinde es en el crawl de clasificados**: montos chicos y
recurrentes son justo lo insoportable de cobrar a mano y lo que un cobro
automático resuelve solo.

**Y por WhatsApp también.** En Puerto Rico un comercio de barrio manda un
video por WhatsApp sin pensarlo, y quizás nunca abra un enlace. Como el bot ya
existe para los reportes de emisión, recibir el spot por ahí sale gratis.

**Reglas del cobro:**

- **El dinero nunca pasa por Antena787.** Cada estación conecta su propia
  cuenta y **cobra directo**. El software nunca custodia fondos de terceros.
- **El método de cobro es un driver** (§10), no una integración fija. Stripe,
  ATH Móvil, Mercado Pago, transferencia, efectivo — o ninguno, y el portal
  solo entrega el anuncio.
- **Se puede apagar entero.** El cobro es un módulo, no un requisito.
- Si un pago recurrente falla, el sistema avisa —**no saca el anuncio del aire
  por su cuenta**. Esa decisión es de la estación.

*Esta pantalla la ve alguien que no es el operador. Es la única de todo el
sistema con esa condición, y por eso tiene reglas de diseño propias: sin menú,
sin jerga, sin nada que configurar, en el teléfono.*

### Paso 10 — Se reconcilia y se prueba

El plan pasa a `aired` con las horas reales. Si hubo un evento de alerta de
emergencia **real** —las pruebas semanales y mensuales no cuentan—, los
tramos tapados se marcan: **los spots tapados no se facturan, se reprograman
como make-good.**

**La reposición no es solo para las alertas.** Cualquier incidente que tape un
spot —un fallo de clip, una caída a cartel, un corte que se comió el
siguiente— genera la misma propuesta de make-good. Al anunciante le da igual
por qué no salió su anuncio.

**Cuánto tiene que taparse para contar.** Un spot que sale con hasta ±15
segundos de desfase respecto a lo planeado se cuenta como emitido. Si algo lo
tapó en **más de la mitad** de su duración, es `preempted` y va a reposición;
si fue menos, queda `parcial` y se factura, con la nota en la evidencia.

**Un corte sobrevendido tampoco produce silencio.** Si en un mismo corte cabe
menos de lo vendido, se desempata por la prioridad de la orden y, a igual
prioridad, por fecha de compra. El que queda fuera **se registra y entra como
make-good propuesto**. Nunca se acorta el corte con negro ni se estira el
reloj.

**El resolver propone el make-good; una persona lo confirma.** Propone
respetando el daypart, la separación de competidores y el precio de la orden
original — pero un make-good mal puesto daña la relación con el cliente, y es
el mismo patrón de "propone, no aplica" que ya rige todo lo que toca
facturación.

El reporte de emisión cuenta filas, no las compone. Y de las mismas filas
salen dos cosas que hoy nadie tiene: un **reporte de ingresos por anunciante y
por mes —vendido, emitido y cobrado en tres columnas—** y una **factura en PDF
sencilla** generada desde el pago, para el que cobra en mano y necesita
entregar un papel.

---

## 10 · Los drivers

> **El modelo de drivers es para quien contribuye. El usuario nunca ve la
> palabra "driver".** El asistente detecta, propone en lenguaje llano y
> prueba.

**Salida:** `red` (**UDP-TS y RTP — el primero que se construye**, es lo que
acepta el multiplexor de CAtv) · `http-ts` · `internet` (RTMP/HLS/SRT) ·
`route-dash` (ATSC 3.0, futuro) · `archivo` · `ninguna`.

> **`http-ts` es de paridad con VLC.** Es el mismo TS que ya arma `udp-ts`,
> servido por HTTP en un puerto para que otro equipo lo tire —un VLC
> remoto, MistServer, un monitor— con varios lectores a la vez y sin que la
> salida se caiga si nadie está leyendo (F2-115, `docs/VLC-PARIDAD.md`).

> **Lo que un multiplexor exige de `udp-ts`, y por eso no es "mandar un
> stream".** Un multiplexor de transmisor junta varios programas en un ASI
> de tasa fija, así que lo que entra tiene que portarse: **tasa constante**
> (CBR con paquetes nulos, al bitrate que se le dijo), **PIDs y número de
> programa fijos** que la persona escribe una vez, **PCR cada 40 ms o
> menos**, PAT/PMT repetidas a tiempo, y **video MPEG-2** —lo que ATSC 1.0
> transmite— con audio **AC-3 o MPEG capa II**, a elegir. CAtv manda hoy
> MPEG-2 con audio MPEG y se oye en los televisores; Antena787 ofrece las
> dos y la prueba de barras del asistente —*"¿se ve y se oye en el
> televisor?"*— decide cuál se queda. Todo eso es configuración del driver,
> visible como "lo que tu multiplexor espera", nunca como flags.
>
> **Orden de prioridad de las salidas, fijado el 8 de septiembre.** Todas se
> construyen; el orden dice cuál se prueba y se pule primero:
> **1)** `udp-ts` MPEG-2 CBR hacia un multiplexor ATSC 1.0 — el caso de CAtv,
> y el de casi toda estación chica de Estados Unidos; **2)** `internet` en
> H.264: SRT, RTMP y HLS, porque la misma estación también sale online;
> **3)** `archivo`, para el modo sombra y las pruebas; **4)** los formatos de
> otros países —DVB, ISDB— con el perfil que les toca (F5); **5)** ATSC 3.0
> por `route-dash`, con HEVC y AC-4, cuando haya un transmisor donde
> probarlo. Ninguno se recorta; el que va primero es el que está al aire.
Varias por canal, simultáneas, cada una con su propio objetivo de volumen y
su reconexión con espera progresiva.

> **Paridad con VLC (Saul, 9 de septiembre de 2026).** CAtv emite hoy con
> VLC. Todo lo que Rolando hace con el *stream output* de VLC tiene que
> poder hacerse aquí: UDP unicast y multicast con TTL, RTP, PIDs y programa
> del TS, transcodificación a MPEG-2/H.264 con MPEG L2/AC-3/AAC, varias
> salidas a la vez, grabación, HTTP TS, HLS, SRT, RTMP, logo y marquesina,
> y como entrada tirar de una URL (`udp://`, `rtsp://`, `http://`) además
> de recibir SRT/RTMP. La lista completa, función por función, está en
> `docs/VLC-PARIDAD.md`; los huecos que le faltaban al diseño ya son
> criterio: multicast y TTL explícitos (F2-114), salida `http-ts`
> (F2-115), entrada `url` (F2-116) y ventana local de monitor (F2-117). La
> firma completa es el criterio F2-113: sin ella, VLC no se apaga.

**Entrada en vivo:** `srt-listen` (**el preferido, por latencia**) ·
`rtmp-listen` · `url` · `ninguna`. NDI queda fuera: su SDK obliga a enlazar
C, y SRT resuelve la latencia sin esa deuda.

> **`url` es de paridad con VLC.** Es para cuando nadie empuja la señal: el
> motor **tira** él mismo de `udp://@…`, `rtsp://`, `http://…ts`, HLS o
> `rtmp://` —lo que hoy le da MistServer—, y si la fuente se ausenta sigue
> el mismo camino que un vivo por SRT: relleno, alarma, reintento con
> espera progresiva y regreso en borde de clip (F2-116, `docs/VLC-PARIDAD.md`).

**Señalización de cortes:** `scte104-tcp` (la ruta primaria; el encoder de
CAtv lo acepta) · `gpi-out` (cierre de contacto hacia el encoder o el
insertador) · `hls-daterange` (salida web) · `ninguna`. **Se elige igual que
el protocolo de salida**: una lista, nunca una suposición. **`inyeccion-ts` no está en la
versión 1** — queda apuntado para F5 y **sin precedente documentado** (§9,
paso 7).

**Alertas de emergencia:** `gpi-serial` · `gpi-gpio` · `sage-endec` ·
`dasdec` · `syslog` · `snmp-trap` · `cap-poll` · **`signal-compare`** ·
`ninguna`. **En Estados Unidos los equipos son pocos y se conocen, y se
traen todos**: Sage Digital ENDEC **1822** (serial RS-232 y relés) y **3644**
(serial, relés, y red: HTTP y syslog); DASDEC de Digital Alert Systems (red:
HTTP, SNMP, syslog; y relés); Gorman-Redlich (relés y serial); TFT (relés).
No hace falta saber el modelo antes de instalar: el asistente pregunta
**por dónde está conectado** —cable serial, cable de relés, cable de red, o
varios— y prueba cada uno. Si hay dos caminos, se usan los dos y se cruzan.
`signal-compare` sobre el retorno de aire verifica a cualquiera, incluido
el que no se conoce. El de CAtv es un Sage con los relés cableados.

> **`signal-compare` es la respuesta a "cualquier marca, cualquier país".**
> No le habla al equipo: compara **la señal transmitida** contra la que el
> plan decía. **Funciona con hardware que el proyecto nunca va a tener en la
> mano** — incluido el EWBS de Sudamérica.
>
> **Y por eso mira el retorno de aire, no nuestra propia salida** (§9, paso
> 8). Comparar contra lo que nosotros mandamos no detecta nada: el motor
> siempre cree que emitió. Sin retorno de aire el driver corre en modo
> `degradado` y lo dice.

**Retorno de aire** (`capture_input`): `receptor-tv` (tarjeta sintonizadora
en la propia máquina — la de CAtv la tiene) · `captura` (entrada de video
alimentada por el RF MONITOR del excitador o por un receptor externo) ·
`stream` (URL de monitoreo del transmisor o de la red) · `ninguno`. Del
retorno sale también el **monitor por streaming**: Antena787 sirve una copia
de baja calidad de lo que de verdad está al aire, para verlo desde el
teléfono, dentro y fuera de la estación.

**Transmisor** (telemetría, solo lectura): `snmp` · `http` (página web del
equipo) · `serial-usb` · `gpi-estado` (contactos de estado: al aire, falla,
reflejada alta) · `ninguno`. **Se traen todos y se pueden combinar**: el
asistente pregunta qué cables hay —red de manejo, USB, contactos— y prueba
cada uno; lo que responda, se usa. Lee lo que el excitador y el amplificador
ya muestran en su pantalla —potencia directa y reflejada, corriente, voltaje, temperatura— y
lo pone en *Al aire*. **Es la única respuesta física a "¿estamos al aire?"**:
potencia directa en cero es fuera del aire, diga lo que diga la red; reflejada
subiendo es antena o cable; temperatura subiendo es ventilación. Alarma en
los tres. **SNMP es el común denominador** —GatesAir, Rohde & Schwarz,
Anywave, Comark en TV; Nautel, Broadcast Electronics y Elenos en radio— y
lo que no habla SNMP suele tener página web o puerto serial/USB. El driver
se elige por cómo se llega, no por marca; la marca solo carga la tabla de
nombres. El de CAtv es un excitador RVR con amplificador ADR, por USB.

**Respaldo:** `disco` (segundo disco o USB) · `red` (carpeta compartida) ·
`nube` (S3, Backblaze, Google Drive) · `ninguno`. El contenido de CAtv vive
en un servidor con respaldo a la web; eso es `nube`.

**Canal de avisos:** `telegram` (gratis) · `whatsapp` (por proveedor) ·
`correo` · `ninguno`. Es por donde llegan las alarmas, los avisos de
vencimiento a 7 días y el *"estuve fuera 6 horas 12 minutos"* de después de un
apagón. Como todo lo que necesita internet, si no hay salida a la red el aviso
se guarda y se manda cuando vuelva.

Regla: **`ninguna` nunca miente.** Si no hay forma de saberlo, el reporte lo
dice en la cara en vez de fingir certeza.

**Cobro del anunciante:** `stripe` · `ath-movil` (Puerto Rico) ·
`mercado-pago` (LATAM) · `paypal` · `transferencia` · `efectivo` · `ninguno`.

> **El cobro es un driver como cualquier otro, y esto importa en este
> mercado:** Stripe no está disponible en buena parte de LATAM, y donde está a
> menudo no es lo que la gente usa. **ATH Móvil es probablemente más
> importante que Stripe para el primer usuario** — una ferretería paga por ATH
> sin pensarlo, pero sacar una tarjeta para un anuncio de $20 al mes es más
> fricción de la que vale el anuncio.
>
> `transferencia` y `efectivo` **no procesan nada: solo registran que el
> cliente pagó.** Así el portal sirve igual para quien cobra en mano, que es
> como se mueve buena parte de este negocio.
>
> **La versión libre trae la abstracción completa y las opciones manuales.**
> Nadie queda obligado a una pasarela para usar el software, y cualquiera
> puede contribuir el método de su país — el mismo patrón que las salidas, las
> alertas y los perfiles de cumplimiento.

**Fichas y carátulas.** El orden es **local primero, red al final**, y dentro
de la red **primero lo que no pide clave**:

| Nivel | Driver | Clave | Uso comercial |
|---|---|---|---|
| **1 · sin red** | `tags-embebidas` · `nfo-local` (Kodi) · `caratula-embebida` | — | libre |
| **2 · sin clave** | `coverart-archive` (MusicBrainz) — **música** | **no** | libre |
| | `tvmaze` — **series de TV** | **no** | libre |
| **3 · clave gratis** | `tmdb` — **el de mejores datos** | sí, gratis | **sí, con atribución** |
| **4 · con reservas** | `discogs` | sí | límites por minuto |
| | `omdb` | sí | 1,000/día; **alta resolución solo con patrocinio** |
| | `fanart-tv` · `theaudiodb` | sí | **⚠ gratis solo NO comercial** |
| | `lastfm` | sí | restricciones en alta resolución |
| | `thetvdb` | **de pago** | **⚠ suscripción anual, ya no es gratis** |

> **Los dos por defecto son los que no piden clave: Cover Art Archive para
> música y TVmaze para series.** Pedirle a alguien que se registre, saque una
> API key y la pegue en algún lado es justo el paso que lo hace abandonar la
> instalación, que es el riesgo #1 del proyecto.
>
> **TMDB es el mejor de todos en datos** —pósters en alta, fondos, logos,
> reparto— y **es gratis incluso para uso comercial.** Cuesta un paso de
> configuración, así que se ofrece pero no se exige. **Su atribución es
> obligación de sus términos y va visible en el producto, no escondida en un
> archivo.**
>
> **⚠ Tres avisos de licencia.** **TheAudioDB y Fanart.tv** dan su acceso
> gratuito solo a proyectos **no comerciales**, y **una emisora que vende
> anuncios es uso comercial**. **TheTVDB** pasó a suscripción de pago incluso
> para uso personal. Los tres son drivers donde **cada estación pone su propia
> clave bajo los términos que a ella le apliquen** — no se mete a un usuario en
> un incumplimiento sin que se entere.

> **Nada se consulta en el momento de salir al aire.** Lo que se descarga se
> guarda junto al archivo, una sola vez. **La cadena de aire funciona con el
> internet caído.**

**Superposiciones:** `logo` (el bug del canal, estático) · `clasificados`
(crawl de texto comercial) · `ninguna`. **Las dos se dibujan en el servidor de
cuadros** (§14.1), antes de entregarle el flujo al encoder — no como filtros
que hay que recargar en ffmpeg. Cambiar el texto del crawl, o quitarlo,
**nunca reinicia el encoder**.

> **El logo puede tener fecha de fin.** Un logo de temporada —navidad, un
> aniversario, una campaña— se pone con su fecha y **se quita solo**. Nadie
> tiene que acordarse en enero.

> **Y hay spots que piden salir sin logo.** Un `media_asset` marcado
> `sin_logo` apaga la superposición mientras dura. Lo piden agencias y
> anunciantes grandes, y es un requisito que hoy obliga a hacer malabares.

> **El crawl de clasificados es un producto comercial, no un adorno.** Pedirle
> a un comercio local un spot de 30 segundos producido es una venta difícil;
> pedirle una línea de texto es una venta de cinco minutos. **Y no consume
> tiempo de programa** — corre encima, así que monetiza horas que hoy están
> vacías.

**Formatos de casa:** `1080i59.94` · `1080p29.97` · `1080p59.94` ·
`1080i50` · `1080p25` · `720p50` · `720p59.94` · `480i59.94` · `576i50` ·
`custom`. Códec: H.264, MPEG-2, HEVC+AC-4 (ATSC 3.0), AV1. **Se traen
todos; `720p59.94` con MPEG-2 a un multiplexor es el que se prueba
primero** (§10, orden de salidas).
**Contenedor y códec son propiedades del perfil, nunca constantes del
motor** — es lo que permite que ATSC 3.0 entre como perfil y no como
reescritura.

---

## 11 · Escala

**Un proceso, un canal. El núcleo sirve un canal.**

Operar varios canales es una **capa de administración** encima, y **es libre
como todo lo demás** *(ADR 0006)*. Comparte biblioteca, almacenamiento,
anunciantes y grabación; cada canal conserva su parrilla, su plan, su proceso
y sus salidas.

**Para el usuario de un canal nada de esto existe.** Aparece cuando añade el
segundo. Los roles, igual: sin cuentas, sin permisos y sin nombres de usuario
hasta que hay una segunda persona. Lo único que hay antes es **una clave para
la estación** (§19), que no es un login: es una llave de la puerta.

---

## 12 · Internacional y cumplimiento

**Antena787 no asume Estados Unidos en ninguna parte.**

| | Américas (ATSC) | Sudamérica y Japón (ISDB-T) | Europa, África, Asia (DVB) |
|---|---|---|---|
| Cuadros | 59.94 / 29.97 Hz | 59.94 / 29.97 Hz | **50 / 25 Hz** |
| Subtítulos | CEA-608/708 | **ARIB** | **DVB, teletexto** |
| Volumen | **−24 LKFS** | −24 LKFS | **−23 LUFS** |
| Alertas | EAS | **EWBS** (en la norma) | sistemas nacionales |

*Colombia usa DVB-T2 estando en Sudamérica — por eso el perfil se elige por
país, no por continente.*

> **−24 LKFS contra −23 LUFS parece trivial y no lo es.** Una universidad en
> España o Colombia que instale esto sale con el volumen de otro país. El
> perfil pone el número de la casa sin que nadie tenga que buscarlo.

**Lo que no cambia:** la norma de transmisión la resuelve el encoder aguas
abajo. **Por eso el proyecto puede ser internacional sin volverse
inmanejable.**

Cada perfil requiere **fuente citada y un mantenedor que opere en esa
jurisdicción**, o se marca experimental. Al lanzar: `us-fcc`, `internet`.

### El cumplimiento se ofrece, no se exige

**Así corren muchos canales locales en Puerto Rico y en el mundo, y no pasa
nada.** El software **nunca regaña**. No le dice a nadie que está en falta, no
bloquea nada por cumplimiento y no da por sentado que el operador le debe algo
a alguien.

Lo que hace es tener **listo lo que a veces alguien pide**. La bitácora de
alertas, el archivo de anuncios políticos, el registro del medidor de volumen,
la línea de "anuncio pagado por" en el crawl: todo eso se activa con el perfil
del país, corre solo por debajo, y el día que llega una carta o una llamada
está ahí, exportable, sin haber tenido que acordarse de nada. **Ese es el
argumento: no un deber, un seguro que no cuesta trabajo.**

Y **se puede apagar**. Un canal por internet no enciende ninguna de estas
funciones, y no tiene que explicar por qué.

**Lo que trae el perfil `us-fcc`, todo activable.** Lo primero que pregunta
el perfil es la clase de licencia —**potencia completa, Class A o LPTV**— y
cada una enciende solo lo que le toca; se cambia cuando se quiera:

| Función | Qué hace |
|---|---|
| **Volumen** | −24 LKFS / TP −2 dBTP (ATSC A/85), puesto solo en el ingest. |
| **Subtítulos** | CEA-608/708 conservados y reinsertados en la salida (§9, paso 1). Hay estaciones exentas de rotularlos: es un ajuste con tres estados —**obligada**, **exenta** (con el motivo: ingresos, canal nuevo, programación exenta), o **no sé todavía**— y **los tres funcionan igual**: los subtítulos embebidos siempre se conservan y siempre se pueden subir. El ajuste solo cambia si el sistema avisa cuando un programa sale sin subtítulos. "No sé" es el default y no bloquea nada. |
| **Identificación de estación** | Cerca de cada hora (47 CFR 73.1201). El cartel de respaldo también la lleva, así que una caída larga sigue identificando la estación. Si pasa una hora sin ella, queda anotado en incidentes — anotado, no bloqueado. |
| **Bitácora de alertas** | Distingue pruebas de activaciones reales. **Se guarda 24 meses**, que es lo que se suele pedir, y nada la borra antes. Las pruebas semanales se comparan contra la ventana horaria configurada; si una cae fuera, avisa. |
| **Archivo de anuncios políticos** | Si un anunciante se registra como político, se le piden candidato, cargo y elección, y de ahí sale un archivo exportable con la solicitud, la aceptación o el rechazo, las tarifas y los horarios (73.1942/73.1943), guardado dos años. La tarifa mínima aparece como **aviso**, no como un cálculo que el software imponga. |
| **Identificación de patrocinio** | Todo clasificado sale con el prefijo *"Anuncio pagado por ‹nombre›"* (73.1212). En este perfil el prefijo no se quita — es una línea de texto y evita el único lío que sí es fácil de evitar. |
| **Registro del medidor de volumen** | Una anotación diaria automática: *"medidor activo, N archivos normalizados, 0 fallos"* (73.682(e)). Exportable. |
| **Archivo público en línea** | Exportación lista para subir, **solo si la licencia lo pide** (Class A y potencia completa). Un LPTV no lo ve. |
| **Programación infantil (E/I)** | La ficha del título tiene un interruptor «programa infantil educativo (E/I)» **desde F1**; el conteo de las 156 horas al año (Class A, Children's Television Act) y el reporte para el **FCC Form 2100 Schedule H** llegan con el reporte de emisión (F4). |

> **El as-run no está en esta tabla a propósito.** La FCC eliminó los
> registros de programación; la bitácora de estación (73.1820) cubre otras
> cosas. **El as-run es evidencia comercial** —facturar, reponer, probarle al
> anunciante que su spot salió— y vale por eso, no por un reglamento. No se
> vende cumplimiento que no existe.

> **Y hay cosas que sencillamente no son de este software.** Las luces de la
> torre (Part 17) no las toca Antena787, y `COMPLIANCE.md` lo dice así de
> claro.

> **Nota legal, en el README sin adornos:** Antena787 ayuda a cumplir, **no
> certifica**. No sustituye asesoría legal ni al ingeniero de la estación.
> **Las alertas de emergencia se cumplen con hardware certificado, no con
> este software.**

*Qué funciones aplican depende de la clase de licencia. CAtv es **Class A**
(§25), así que en el despliegue de referencia se encienden las de Class A.*

### Lo que viene: ATSC 3.0

La FCC votó 3-2 en mayo de 2026 obligar a las estaciones **de potencia
completa** a completar la transición hacia el cuarto trimestre de 2027; el
simulcast vence el 17 de julio de 2027. **LPTV, traductores y Class A están
hoy exentos**, y buena parte de los usuarios de Antena787 cae ahí.

ATSC 3.0 es otra pila: HEVC, AC-4, y **ROUTE/DASH sobre IP en vez de
MPEG-TS**. El diseño lo absorbe porque el formato ya es un perfil y la salida
ya es un driver.

---

## 13 · Las pantallas

Cinco entradas de menú. Seis con Anuncios, que aparece al registrar el primer
anunciante.

| Pantalla | Qué resuelve |
|---|---|
| **Al aire** | El cuadro grande es la señal real saliendo. Punto rojo pulsante como tally, siempre en el mismo sitio. Semáforo de estado y salidas. Al lado, más chico, **el retorno de aire** —lo que de verdad está en el aire, después del equipo de alertas— y, si hay telemetría, la potencia del transmisor. |
| **En vivo** | La fuente entrante con medidores de audio, bitrate, cuadros perdidos y retraso. Números honestos, no un "todo bien". |
| **Parrilla · Semana** | Siete días apilados, el ancho es la duración real, los huecos se ven rojos. |
| **Parrilla · Mes** | Cada día una barra de 24 horas. La salud del mes de un vistazo. |
| **Parrilla · Guía** | *Lo que dice la guía* sobre *lo que va a salir*, en el mismo eje. |
| **Reglas** | Tarjetas con patrón de días y semáforo de vencimiento. Aquí se arma el canal. |
| **Biblioteca** | Pared de carátulas. Inspirada en Plex, sin depender de Plex. |
| **Anuncios** | Cuánto le cabe a cada hora contra el tope, clientes y evidencia de emisión. |
| **Ajustes** | Las buenas prácticas del sistema **vigiladas para siempre**, no solo aplicadas una vez. |

> **Ninguna pantalla interrumpe el aire para mostrar algo** *(ADR 0008)*.
> Revisar el pasado, editar la parrilla, previsualizar el crawl o mirar una
> alarma nunca tapa ni pausa lo que está saliendo. Es un segundo panel al
> lado del vivo, jamás un modal encima.

**El portal del anunciante** es la única pantalla que **no ve el operador**.
La abre el anunciante desde un enlace, en su teléfono: ve qué compra, paga,
arrastra su anuncio y recibe respuesta al momento. Sin menú, sin cuenta, sin
jerga, sin nada que configurar. Por eso tiene reglas de diseño propias.

**Tomar el control** no es una pantalla: es un botón **siempre visible** en
*Al aire* — no detrás de un menú, no en Ajustes, no solo cuando el sistema
cree que hace falta. Mientras está activo aparece un panel de disparo y una
cuenta regresiva del regreso automático. Al soltarse, todo vuelve a su sitio
(§9, paso 6).

**Ajustes muestra siempre tres números que no se ven en ningún otro lado:** la
deriva del reloj contra el servidor de hora, cuál aceleración por hardware
quedó elegida y con qué resultado, y el umbral de silencio y negro que está
vigente.

### El editor: reglas como entrada, línea de tiempo como vista

**La programación de televisión son reglas, no celdas.** *"Kojak, lunes a
viernes, 8 AM, de agosto a diciembre"* es **una frase** — y en el Sheet son
110 celdas escritas a mano.

- La **regla es una tarjeta**, y eso *es* la parrilla.
- La **línea de tiempo es una vista**: bloques de ancho proporcional a la
  duración real, y **el hueco de 6 minutos que sobra de un slot es un hueco
  visible**. Se arrastra directo y el sistema pregunta *"¿solo hoy, o
  siempre?"*.
- **Nunca se ve una parrilla vacía.** Al terminar de cargar contenido, el
  sistema propone la semana completa.
- Hay además una **vista de cuadrícula de solo lectura**, para quien viene
  del Sheet o quiere imprimirla. Se mira, no se edita.
- **Se puede pegar desde Excel o Google Sheets** — herramienta de migración,
  se usa una vez.

**Después de importar, las horas vacías se ven y se llenan de un clic.** Una
hoja traída de otro sistema casi nunca trae relleno ni reglas de
retransmisión: por eso Parrilla marca los tramos sin nada y ofrece **"Llenar
con diferido"**, que crea la regla que retransmite la mañana en la madrugada.
Es un botón, no un proyecto.

> **El importador nunca rechaza la hoja entera.** Trae lo que sirve y lista
> **fila por fila** lo que no pasó y por qué, en cristiano. Sabe además que
> una hoja hecha a mano usa fechas de calendario: para las filas cuya hora cae
> entre las 12:00 y las 5:59 AM corre las fechas un día atrás —para que
> coincidan con el día de emisión— y lo reporta fila por fila en vez de
> hacerlo callado. Las filas cuyo título coincide con una fuente en vivo se
> importan como bloque en vivo, y la columna de episodios se ignora con aviso.
> Y cuando una regla empieza justo al día siguiente de que termina otra, en la
> misma franja y con el mismo patrón, pregunta: *"¿Zoids releva a Magic
> Knight?"* — para que un relevo no se cuente como un vencimiento sin
> reemplazo.

### El asistente de instalación

**Meta: seis preguntas o menos.**

1. **¿Cómo se llama tu canal, y quién entra aquí?** Nombre, identificativo y
   comunidad de licencia —con eso se genera el **cartel de respaldo**, que es
   el último escalón de la cascada (§9, paso 4)— y una **clave de estación**
   de cuatro a seis dígitos. No es un usuario con contraseña ni un sistema de
   permisos: es **una sola clave para la estación**, que se pide una vez por
   navegador y no vuelve a estorbar. Existe para que el sobrino que se conecta
   al wifi no saque el canal del aire.
2. **¿Qué vas a hacer?** Internet · Transmisor · No sé
3. **Revisión automática** — reporta en lenguaje llano lo que encontró
4. **¿A dónde va tu señal, y puedes verla de vuelta?** Nunca se pide elegir
   un driver. La segunda mitad es el retorno de aire (§9, paso 8); "todavía
   no" es una respuesta válida que se muestra, no se esconde.
5. **La prueba de barras** — *"¿Ves las barras de color?"*
6. **¿Cómo se ve tu canal?** País y calidad — la lista trae 480i, 720p,
   1080i y 1080p, a 29.97/59.94 y 25/50, y la persona elige lo que su
   multiplexor espera. 1080i se codifica entrelazado de verdad.
7. **Tu contenido** — arrastrar o señalar carpeta
8. **Tu primera parrilla** — con propuesta automática
9. **Al aire** — un botón

*(Nueve pasos, seis preguntas: los pasos 3, 8 y 9 no preguntan nada.)*

> **El paso 5 es el más importante del producto.** Antes de subir un video,
> el sistema manda barras y tono a la salida y hace una pregunta que
> cualquiera puede contestar. Convierte una tarde de frustración en un sí o
> un no. **Las barras y el tono son de esta prueba**, no del respaldo del
> aire: ese termina en el cartel.

> **Si no hay nada con qué rellenar, el asistente lo dice.** Una biblioteca de
> relleno vacía significa que el primer hueco sale en negro, y eso no puede
> descubrirse a las 3 AM. El asistente avisa y ofrece **crear un relleno por
> defecto de un clic**: el cartel de la estación con una cama musical. Se
> cambia después; lo que no se puede es quedarse sin él.

### Pendientes de diseño

Trece pantallas están dibujadas: las nueve de la tabla de arriba más Manual,
Diferido, Portal y Música, que se llegan desde ellas. **Estas todavía no, y
hasta que lo estén el producto no está completo:**

- **El asistente de instalación**, incluida la prueba de barras y el paso de
  la clave de estación.
- **Cuarentena** — qué falló, en cristiano, y el botón *"Dejarlo pasar bajo mi
  responsabilidad"*.
- **Bitácora de incidentes** — el historial de todo lo que el sistema hizo
  solo, y desde donde se marca a mano una interrupción de alerta.
- **Revisión de cambios propuestos por el asistente de IA** — el diff que
  devuelve el MCP dentro de la ventana de protección (§20).
- **Configurar la salida** — la pantalla que hoy solo existe dentro del
  asistente.
- **Aprobación de una reposición** (make-good).
- **Clasificados** — crear, previsualizar sobre el aire y aprobar.
- **Reloj de cortes de una fuente en vivo** — a qué minutos de cada hora se va
  a comerciales.
- **Aprobación del envío de un reporte** al anunciante.
- **Configuración del logo**, con su fecha de fin.

---

## 14 · El stack

**Go** para el motor y el servidor. Escogido por **compilación cruzada
trivial** —Windows, Linux y ARM desde una máquina—, por binario único sin
runtime, porque se lee sin haberlo escrito (lo que importa para conseguir
contribuidores) y porque es el más fácil de auditar. **No por rendimiento:**
el trabajo en tiempo real está en ffmpeg, no en nuestro código.

**TypeScript y React** para la interfaz, compilados con **Node en tiempo de
compilación** y **embebidos en el binario** con `go:embed`. En la máquina de
la estación no corre Node.

**SQLite** en modo WAL, con `modernc.org/sqlite` (SQLite traducido a Go) —
**nunca con CGo**. *(ADR 0002)* La base va en **disco local, nunca en red**:
el modo WAL no funciona sobre SMB ni NFS y la corrupción es silenciosa. **La
biblioteca de video sí puede vivir en un NAS** —es lo normal en una estación
chica— y por eso un error de lectura a mitad de clip se trata como un
tropiezo, no como un archivo malo (§9, paso 4).
Postgres es opcional, para instalaciones con varios canales o varios usuarios concurrentes.

### Dependencias: una sola externa

| Qué | Cómo se resuelve |
|---|---|
| **ffmpeg / ffprobe** | **La única dependencia externa.** Binario estático empaquetado dentro del instalador, invocado como proceso. *(ADR 0003)* |
| SCTE-35 | `Comcast/scte35-go` — Go puro |
| SCTE-104 | Código propio en Go puro. No hay librería, ni hace falta |
| Recepción RTMP / SRT | Librerías Go embebidas en el propio binario |
| Base de datos | `modernc.org/sqlite` — Go puro, sin CGo |
| Fichas y carátulas | Etiquetas embebidas y `.nfo` de Kodi sin red · **Cover Art Archive** (música) y **TVmaze** (series) **sin clave** · **TMDB** con clave gratis y atribución visible · Discogs, OMDb, Fanart.tv, TheAudioDB, Last.fm y TheTVDB como drivers con clave propia y sus reservas |
| Validación de la guía | **Verificador propio en Go.** El `tv_validate_file` de referencia es Perl, y empaquetar Perl rompería la promesa de un solo ejecutable |
| Servicio del sistema | `kardianos/service` — Go puro |
| PDF de reportes | Librería Go pura |
| WhatsApp y Telegram | Librerías Go puras |
| Miniaturas | ffmpeg extrae el cuadro, Go puro hace el resto |

**Eliminados a propósito:** CasparCG, TSDuck, Plex, Jellyfin, un servidor de
streaming aparte, y CGo en SQLite.

---

### 14.1 · Decisiones técnicas tomadas para poder empezar

Un PRD dice qué; esto es lo que un implementador necesita decidido antes de
la primera línea. **Cada una es reversible y barata ahora; ninguna lo es en
la semana doce.**

| Decisión | Resolución |
|---|---|
| **Procesos** | **Uno solo.** `antena` es un binario con el servidor web, el resolver y el motor como goroutines. "Un proceso, un canal" significa una instancia por canal, no un motor aparte. Aislar el motor en otro proceso se reconsidera solo si la prueba de 30 días muestra que un fallo del servidor web tumba el aire. |
| **Que un fallo interno no tumbe el aire** | Un solo proceso no puede significar que un error en cualquier rincón saque el canal del aire. **Cada goroutine crítica —servidor de cuadros, watchdog, resolver— atrapa su propio pánico, registra un incidente y se relanza sola**, sin tumbar el proceso. El proceso, a su vez, corre bajo el supervisor del sistema —servicio de Windows o systemd— con reinicio automático; si el reinicio ocurre, la recuperación es la de más abajo: seek al segundo que toca, nunca reiniciar el bloque. Y la memoria se vigila: si el consumo pasa del umbral, **suena la alarma antes de que el sistema operativo mate el proceso**, no después. |
| **Estructura de Go** | `cmd/antena/` · `internal/engine` · `internal/resolver` · `internal/ingest` · `internal/drivers/{output,input,alert,billing,metadata,overlay}` · `internal/store` · `web/` (React, embebido con `go:embed`). Un paquete por concepto del glosario. |
| **Decodificador → encoder** | **Un servidor de cuadros en Go entre los dos.** Cada clip lo decodifica un `ffmpeg` a video crudo (`rawvideo` yuv420p) y audio PCM; un componente en Go recibe esos cuadros, aplica el conformado, mantiene el pre-roll del siguiente clip y **entrega un solo flujo continuo al encoder persistente** por dos conexiones TCP en `127.0.0.1` —una de video crudo, otra de PCM—, porque stdin es una sola y las tuberías con nombre no son portables entre Windows y Linux. Es la decisión que la F0 valida o tumba (§22.1). |
| **Reloj** | El motor no confía en `time.Sleep`. **Cuenta el tiempo del aire con un reloj monotónico** —el que no salta cuando alguien cambia la hora— derivado de las marcas de tiempo del encoder, y lo compara contra la hora de pared cada minuto. Diferencias pequeñas se corrigen por **deriva gradual, nunca por salto**. **NTP forzado en el instalador.** |
| **Salto de reloj** | Una corrección de más de 60 segundos —hacia adelante o hacia atrás— no es deriva: es un salto, y se trata aparte. **Nada que ya salió al aire se vuelve a emitir**, nada se marca vencido de golpe, se levanta una alarma `salto_de_reloj` distinta de la de deriva, y el resolver recalcula el plan **desde el instante real**. Es el caso que rompe un playout después de un apagón largo o de un NTP que llega tarde. |
| **Reinicio a mitad de programa** | Todo `cued` vuelve a `planned`; el motor calcula qué debería estar al aire ahora, abre ese archivo con seek al segundo correcto y arranca. **Nunca reinicia el bloque desde cero.** |
| **Configuración** | **Ninguna en archivo.** Una tabla `settings` en SQLite. Coherente con el principio 5 y evita que configuración y datos se desincronicen. |
| **Migraciones** | Herramienta pura Go, sin CGo, con versión en `PRAGMA user_version`. **Respaldo automático de la base antes de cada migración.** |
| **API** | Prefijo `/api/v1/` desde F1 aunque el único cliente sea el frontend embebido. Estado en vivo del tablero por **WebSocket**; todo lo demás REST con JSON. |
| **ffmpeg** | Binario estático de una fuente fija (la build de BtbN o propia), **una versión exacta por release**, colocado junto al ejecutable por el instalador. No se embebe con `go:embed` — extraerlo en cada arranque es lento y frágil. |
| **Aceleración por hardware** | Listar `-hwaccels` **no basta**: los drivers mienten. Al arrancar, el motor **codifica diez segundos de prueba con cada encoder candidato** —QuickSync, NVENC, VAAPI, software— mide CPU y elige el mejor que funcionó. El resultado se muestra en Ajustes. |
| **Relleno que no cuadra** | Si no hay combinación exacta de filler para un hueco, se permite **exceder hasta 5 segundos**, y lo que se recorta es **el último clip de relleno del hueco**, con un fundido de un segundo. El relleno no lleva subtítulos por definición, así que no hay nada que romper. Nunca queda un hueco residual. |
| **El crawl y el logo** | Se dibujan **en el servidor de cuadros, en Go**, no como filtros de ffmpeg. Cambiar el texto del crawl "en caliente" sobre un ffmpeg corriendo no es una operación documentada, y la alternativa —reiniciar el encoder cada vez que cambia una línea de texto— es inaceptable en un canal al aire. Dibujarlo nosotros cuesta un poco de CPU y nos deja cambiarlo cuando queramos. |
| **Negro y silencio en el ingest** | Umbral: luma media bajo 16, audio bajo −60 dBFS, más de 0.5 s. **En cabeza y cola se recorta; en medio del archivo se registra como marca de corte candidata**, que el programador confirma. |
| **Volumen** | `loudnorm` de ffmpeg en **dos pasadas** —medir y luego corregir— al objetivo del perfil. Una pasada es más rápida y más imprecisa. |

## 15 · El modelo de datos

```
── el canal y sus salidas ──────────────────────────────────────────────
channel         nombre, tipo (tv | radio), perfil_de_formato,
                perfil_regulatorio, modo, zona_horaria (IANA),
                hora_inicio_dia_emision (6:00 AM por defecto),
                carga_maxima_por_hora (12 min por defecto)
output          channel, driver, parametros, objetivo_volumen,
                estado_conexion, reintentos, ultimo_error
capture_input   channel, tipo (receptor-tv | captura | stream), punto_de_origen,
                modo (activo | degradado), ultima_senal_vista
                ← el RETORNO DE AIRE: la señal ya transmitida, después del
                  equipo de alertas. Es lo que graban air_recording y compara
                  signal-compare. Sin él, modo degradado y marcado manual
driver_config   channel (nulo = global), tipo (salida | alerta | cobro |
                fichas | entrada | avisos), driver, credenciales (cifradas),
                parametros

── el contenido ────────────────────────────────────────────────────────
media_asset     channel (nulo = compartido), ruta, hash, códec, resolucion,
                fps, canales_audio, duración_medida_ms, lufs, true_peak,
                tiene_subtitulos, formato_subtitulos,
                subtitulos_externos (ruta .scc/.srt/.vtt, nulo si no hay),
                negro_cabeza_ms,
                negro_cola_ms, marcas_de_corte_ms[], cuadro_miniatura,
                ← las marcas viven en el ARCHIVO: la emisión original y el
                  diferido generan sus cortes desde aquí
                estado (ingiriendo | listo | cuarentena | fallido),
                motivo_en_cristiano, estado_normalizacion,
                negro_intencional (bool), sin_logo (bool)
                ← negro_intencional apaga el detector de negro y silencio
                  mientras dura el clip; sin_logo apaga la superposición
title           nombre, tipo (serie | pelicula | promo | id | spot | cortinilla),
                sinopsis, año, género, clasificacion_contenido,
                clasificacion_audiencia, carátula, media_asset (si no es serie)
episode         title, temporada, numero, media_asset
filler_asset    media_asset, channel (nulo = genérico), tipo, duración exacta
live_source     channel, tipo (srt | rtmp | captura), punto_de_escucha,
                duración_prevista, filler_de_respaldo,
                reloj_de_cortes[] | driver_de_cue, retardo_ms (7 s),
                gracia_s (30 s por defecto)
                ← dentro del margen de gracia se sostiene el cuadro o el
                  cartel, sin alarma: llegar tarde unos segundos es normal

── la programación ─────────────────────────────────────────────────────
schedule_rule   channel, tipo (normal | diferido | bloque_arrendado),
                title (nulo si diferido), patrón_de_días, hora,
                fecha_inicio, fecha_fin, ultimo_aviso_enviado,
                episodios_por_corrida, ultimo_episodio_emitido,
                releva_a, repite_a (schedule_rule),
                advertiser, cobro   (solo bloque_arrendado)
                ventana_origen_inicio, ventana_origen_fin   (solo diferido)
                ← repite_a: la repetición emite el MISMO episodio que su
                  regla primaria puso ese día de emisión. Un solo contador
                  por serie en su franja; nunca dos cuentas separadas
                ← las fechas son días de emisión: fecha_fin llega hasta el
                  cierre del día (5:59:59 AM del calendario siguiente)
deck            channel, tipo (manual | comercial | programa | relleno),
                prioridad
plan_item       channel, deck, instante_planeado, duracion_planeada,
                instante_real, duracion_real,
                origen (asset | live_source), media_asset | live_source,
                dentro_de (plan_item padre, si va dentro de un vivo),
                corte (si es un spot dentro de un corte),
                estado: planned → cued → aired
                        → (skipped | preempted | fallido | manual_hold)
                parcial (bool), cued_en, error
                ← ES el as-run: lo planeado queda intacto, lo real se llena
                ← RESTRICCIÓN: dentro de un mismo deck y una misma salida no
                  pueden existir dos plan_item con intervalos solapados.
                  Vive en el esquema (índice + verificación al insertar),
                  no en el código de validación
manual_hold     channel, inicio, fin, motivo_fin (soltado | fin_de_bloque |
                timeout | caida_del_sistema), usuario
                ← un solo tenedor a la vez; quitárselo a otro queda en
                  audit_log
air_recording   channel, inicio, fin, ruta, retencion_hasta
                ← se cruza con plan_item por canal y solape de tiempo

── la publicidad ───────────────────────────────────────────────────────
advertiser      nombre, tipo (comercial | politico | no_lucrativo),
                categoria (para separar competidores), contacto,
                email, whatsapp,
                candidato, cargo, eleccion   (solo si tipo = politico)
                ← lo político alimenta el archivo exportable de §12, con
                  solicitud, aceptación o rechazo, tarifas y horarios
insertion_order channel, advertiser, spot (media_asset), cantidad,
                duracion_tramo (15 | 30 | 45 | 60), ventana, daypart,
                tope_por_hora, prioridad,
                estado (borrador | pagado_sin_material | activo | terminado)
                ← prioridad desempata un corte sobrevendido; el que cae va a
                  make-good propuesto, nunca a silencio
corte           channel, plan_item_interrumpido (nulo si entre programas),
                instante, duracion_total
                ← agrupa los spots de un mismo corte; un solo SCTE-104
break_marker    corte, offset, duración, tipo   ← origen del SCTE-104
spot_airing     insertion_order, plan_item, instante_real, verificado,
                estado (emitido | tapado | make_good),
                es_makegood_de (spot_airing original, si aplica)
classified      channel, advertiser, texto, ventana, rotacion,
                estado (pendiente | aprobado | rechazado), linea_patrocinio
                ← la línea "Anuncio pagado por ‹nombre›" se antepone sola y
                  en perfil us-fcc no se puede quitar
classified_airing classified, channel, instante_inicio, instante_fin
portal_link     advertiser, insertion_order | classified, token (32 bytes),
                precio, estado, media_asset_recibido, vence, revocado
                ← ligado a UN anunciante: si el enlace se filtra, quien lo
                  abra no puede subir a nombre de otro
payment         portal_link, driver, monto, estado, recurrencia,
                referencia_externa, clave_idempotencia (= referencia_externa,
                única)
                ← nunca custodia fondos: el cobro va directo a la estación
                ← la clave única impide que un aviso repetido cobre dos veces

── lo que el sistema hizo solo ─────────────────────────────────────────
alert_event     channel, tipo (real | prueba_semanal | prueba_mensual),
                inicio, fin, driver, confianza, alcance_geográfico,
                origen (automatico | manual)
                ← las pruebas RWT/RMT no cuentan como interrupción ni
                  generan make-goods
                ← retención mínima de 24 meses: nada lo purga antes
                ← origen = manual cuando no hay retorno de aire y una
                  persona marcó la interrupción, incluso hacia atrás
incidente       channel, tipo (encoder_reiniciado | vivo_ausente |
                manual_por_timeout | caida_a_cartel | salida_caida |
                solape | cascada_extendida | apagon | salto_de_reloj |
                enlace_caido | ...),
                inicio, fin, detalle
audit_log       entidad, entidad_id, campo, valor_anterior, valor_nuevo,
                autor, origen (humano | mcp | sistema), instante,
                aplica_en (inmediato | pendiente_de_aprobacion),
                tipo (cambio | loudness | ...), hash_prev
                ← hash_prev encadena cada entrada con la anterior: una
                  bitácora que no se puede editar por atrás sin que se note
                ← tipo=loudness es la anotación diaria automática de §12
settings        clave, valor
                ← toda la configuración vive aquí, incluidos los umbrales y
                  el hash de la clave de estación. Ningún archivo de
                  configuración (principio 5)
overlay         channel, tipo (logo | clasificados), parametros,
                fecha_inicio, fecha_fin
                ← un logo de temporada se quita solo el día que toca
user / role     ausentes hasta que hay una segunda persona. La clave de
                estación no es un usuario: es una sola clave para la
                máquina, en settings
```

**Convenciones que evitan bugs enteros:**

- **Todo instante se guarda en UTC.** El día de emisión, el horario de verano
  y la hora local se calculan con `channel.zona_horaria` al mostrar, nunca al
  guardar. Es el caso borde que un modelo comete con más facilidad (§16).
  **Política en el cambio de horario:** una regla en la hora que *no existe*
  (primavera) se corre a la siguiente hora válida; una regla en la hora que
  *existe dos veces* (otoño) sale en la primera. El diferido de esa noche usa
  la ventana en UTC, así que no se duplica ni se salta. Puerto Rico no cambia
  de horario; otros perfiles sí.
- **No existen reglas "anuales".** El patrón de una regla es semanal y sus
  fechas son absolutas, así que el 29 de febrero no es un caso borde: no hay
  nada que se repita "cada año" y pueda caer en un día que no existe.
- **`channel` va en cada tabla desde F1**, aunque el sistema corra un solo
  canal hasta F5b. Migrar a multi-canal sin eso duele.
- **`plan_item` guarda lo planeado y lo real por separado.** Sin eso la
  pantalla de Guía —*"lo que dice la guía sobre lo que va a salir"*— y la
  métrica de exactitud de la guía no se pueden calcular.
- **Al reiniciar el servicio, todo `cued` vuelve a `planned`** y el motor
  entra por el minuto que le toca, con seek dentro del archivo — no reinicia
  el bloque.


> **Regla de integridad:** una `schedule_rule` no se guarda con `fecha_fin`
> anterior a `fecha_inicio`, y un `plan_item` no puede existir fuera del rango
> de fechas de su regla. **La restricción vive en el esquema, no en el código
> de validación.** Así muere de raíz el bug del *Hellsing*.
>
> **Antena787 no gestiona derechos de contenido** — sin territorios, sin vías
> de distribución, sin conteo de corridas licenciadas. Ningún software de este
> segmento lo hace, y mantenerlo es trabajo manual que nadie quiere. Lo que
> sobrevive son dos campos de fecha que el programador **ya llena hoy**, y de
> los que salen los avisos de vencimiento y la muerte del bug del *Hellsing*.

---

## 16 · Cómo se verifica algo construido por IA

El código lo escribe mayormente una IA con revisión humana. **Un motor de
playout no falla con un error 500 — falla a las 3:14 AM de un martes, en
silencio.** Y los errores de este dominio son justo los que un modelo comete
con más facilidad y que menos se notan al revisar: deriva de marcas de tiempo
que aparece a las 40 horas, un contador que se desborda, un caso borde de
horario de verano.

**Tres reglas no negociables:**

1. **El motor, el conformado y el watchdog se leen línea por línea por un
   humano.** Son ~20% del código y 95% del riesgo. La interfaz, el editor y
   los reportes no requieren ese nivel.
2. **Prueba de resistencia de 30 días** contra salida a archivo, con
   verificación automática de cada cambio de clip **e instrumentación de la
   deriva acumulada entre audio y video**, **antes de tocar una antena**. Un
   desfase de un cuadro cada pocas horas es invisible en minutos y
   catastrófico al mes.
3. **Prueba de 48-72 horas de la señalización de cortes contra el encoder
   real**, verificada **en la señal transmitida** y no en lo que nosotros
   creemos haber mandado: que cada SCTE-104 que sale produzca su marca aguas
   abajo, con su latencia medida. Antes de que los ingresos de nadie dependan
   de ella.

Y una consecuencia legal que sale gratis: una obra sin autoría humana podría
no estar sujeta a derechos de autor, y el copyleft necesita un titular. **La
revisión humana documentada resuelve las dos cosas a la vez.** *(ADR 0005)*

---

## 17 · Cómo entra al aire de Rolando

Rolando aceptó ser el despliegue de referencia. **Aceptar no puede significar
apagar VLC un lunes.**

**La cadena de CAtv, confirmada por Rolando el 8 de septiembre:**

```
Antena787 ──UDP/RTP (Ethernet)──▶ Technalogix TP1000 ──ASI──▶ excitador RVR ──▶ amplificador ADR ──▶ antena
                                   (encoder/mux, 720p59.94)      (605 MHz)          (~300 W)
                                        ▲
            Sage Digital ENDEC ─────────┘  audio XLR + video compuesto; relés en el bloque verde
                  ▲
            TFT EAS 930A: WCMN 1280 AM · WKAQ-FM 104.7 · NOAA ch 1
```

**Lo que hay hoy en esa cadena, y lo que Antena787 reemplaza:** la PC está
en la torre; MistServer recibe los streams, VLC los convierte a **720p con
video MPEG-2 y audio MPEG** y los manda por **UDP** al TP1000, que es un
**multiplexor**: junta ese programa con lo demás y saca el ASI para el
transmisor. Antena787 ocupa exactamente el lugar de MistServer + VLC en esa
PC, y le entrega al multiplexor lo mismo que hoy recibe —solo que sin
huecos, con as-run, y con cortes marcados. El multiplexor y todo lo que hay
después no cambian.

El TP1000 tiene dos entradas y dos salidas ASI y dos puertos de red —uno de
manejo y otro para el stream de entrada—; el excitador tiene entrada ASI y
red de manejo, pero **no recibe transporte por IP** (los modelos nuevos sí).
Por eso `udp-ts` al TP1000 es la única puerta, y por eso el retorno de aire
es la tarjeta receptora, no un stream del transmisor. **En el mismo sitio
está WRBM 89.3 FM, Océano Radio** —`RadioOnce Live!` es un programa suyo—
así que la parte de radio (F4b) es para CAtv, no para después.

1. **Modo sombra** — Antena787 arma el plan, genera la guía y registra el
   as-run, **pero Rolando sigue emitiendo con VLC**. Se compara lo que
   Antena787 *habría* puesto contra lo que salió. **Cero riesgo, semanas de
   evidencia real.**
2. **Salida paralela a archivo** — emite de verdad, a un archivo o stream
   privado. Una semana de observación.
3. **Madrugadas primero** — hoy son aire vacío. Una falla ahí casi nadie la
   ve, **y es donde el sistema además aporta valor inmediato.** El peor
   problema de Rolando resulta ser el terreno de pruebas más seguro.
4. **Aire completo**, con la caja de respaldo lista.

---

## 18 · Hardware

**La aceleración por hardware es estructural, no una conveniencia.** Con
encoder persistente, recodificar 1080p cuesta **~15% del CPU con QuickSync
en un N100** *(estimado)*, pero **un núcleo completo por software**
*(estimado)*. Sin aceleración se rompe el costo casi lineal por canal.

**Con una excepción que favorece al caso chico:** un transmisor ATSC 1.0
recibe **MPEG-2**, que ninguna tarjeta acelera y que **no hace falta
acelerar** — a 720p59.94 cabe en un núcleo por software, y es lo que VLC ya
hace hoy en la PC de CAtv. Ahí la aceleración se gasta en **decodificar** la
biblioteca (H.264) y en las salidas de internet (H.264), no en la del
transmisor. F0 lo mide en vez de suponerlo.

| Escenario | Máquina | Aprox. |
|---|---|---|
| **Presupuesto cero** | **La máquina que ya tienes.** Es el caso de CAtv: Windows 10 existente, sin comprar nada | $0 |
| Canal por internet, 720p | PC de oficina reciclado con gráficos Intel recientes | $0 |
| Canal 1080 con transmisor | Mini PC Intel N100/N305, 16 GB | ~$200 |
| 4 a 6 canales | Servidor de 8+ núcleos con GPU, 32 GB | ~$1,500 |

> **Todas las cifras de esta sección son estimadas, no medidas por este
> proyecto.** La Fase 0 las mide de verdad y esta tabla se corrige antes de
> publicar la guía de compra.

> **El despliegue de referencia corre con presupuesto cero.** Rolando emite en
> **720p** —lo confirmó él: al encoder le manda UDP o RTP, y al multiplexor le
> pasa 720p— sobre un **Windows 10 que ya tiene**, sin comprar hardware.
> Emitir a 720p en vez de 1080 es justo lo que da ese margen. **Si funciona
> ahí, la tesis del proyecto se sostiene sola.**

**La recomendación de Raspberry Pi queda retirada** hasta verificar si el Pi
5 tiene encoder H.264 por hardware. Sin él, no se sostiene.

**Dos máquinas idénticas** son lo recomendable para una instalación con
licencia de transmisión — **pero con presupuesto cero no existen.** Cuando no
hay segunda máquina, **la cascada de respaldo por software deja de ser una red
de seguridad y pasa a ser la única**: programa → relleno → cartel. Eso sube el
listón de la prueba de resistencia de 30 días (§16).

---

## 19 · Instalación y operación

**En ambas plataformas:** el instalador registra el servicio, configura el
arranque automático, **aplica los ajustes del sistema operativo solo**,
**verifica y fuerza la sincronización de hora por NTP**, y termina con un
**reporte de verificación**.

> **NTP no es opcional.** Toda la promesa de "la parrilla es un contrato con
> la hora de pared" depende de que el reloj esté bien puesto — y en un mini PC
> sin presupuesto nadie lo garantiza. El instalador lo configura, y la
> pantalla de Ajustes muestra la deriva contra el servidor de hora y avisa si
> pasa de un segundo. **Si no hay salida a internet**, se puede apuntar a un
> servidor de hora de la red local; y si el reloj lleva **24 horas** sin
> sincronizar, avisa, **a los 7 días** levanta alarma. Nunca deja de emitir
> por eso.

Tailscale para acceso remoto sin abrir puertos. Respaldo cada hora.
**Actualizaciones nunca automáticas**, y **firmadas**: una actualización cuya
firma no verifica no se instala, porque esta máquina emite.

### Quién puede entrar

**La interfaz escucha solo en la máquina y en Tailscale.** No se publica en la
red local ni en internet por defecto. **El portal del anunciante es el único
punto público**, y por eso es el único que tiene límite de peticiones y cola
de aprobación (§9, paso 9).

**La clave de estación es una sola clave, no un sistema de usuarios.** Se pone
en el primer paso del asistente, se pide una vez por navegador y se guarda en
una cookie de sesión estricta. No hay nombres de usuario, no hay permisos, no
hay roles — **los roles llegan cuando hay una segunda persona** (§11, F5b), y
hasta entonces serían complejidad que nadie pidió. Lo que sí evita esta clave
es que cualquiera que se conecte al wifi pueda sacar el canal del aire.

**Las credenciales no se guardan en claro.** Las claves de las pasarelas, de
los servicios de fichas y del canal de avisos van al almacén del sistema
—DPAPI en Windows, el llavero en Linux—. Un respaldo que sale de la máquina va
**cifrado**.

**Las entradas en vivo llevan contraseña.** SRT no se abre sin ella —tampoco la
entrada rápida del §9, paso 5—, porque un puerto de vivo abierto en una red
local es una forma bastante directa de poner cualquier cosa al aire.

**El texto de un clasificado pasa por una lista blanca de caracteres.** Lo que
escribe un anunciante desde el portal termina dibujado sobre la señal: se
acepta el alfabeto, los números y la puntuación normal, y nada más.

**`ffmpeg` se verifica por SHA-256** al instalarlo y en cada arranque. Es la
única dependencia externa; si el archivo cambió sin que nadie lo actualizara,
hay que saberlo antes de salir al aire y no a mitad de un programa.

### Cuando algo del sistema operativo se pone en el medio

**Windows** — **Windows 10/11 IoT Enterprise LTSC es lo ideal** (sin
actualizaciones de funcionalidad por 10 años, diseñado para equipos
dedicados) **pero cuesta dinero, y con presupuesto cero no es una opción.**

> **Antena787 tiene que hacer que un Windows 10 normal aguante 24/7.** El
> instalador controla Windows Update **por política** —diferimiento, sin
> reinicio automático, horas activas— en vez de exigir cambiar de edición. Es
> menos robusto que LTSC y hay que decirlo, pero es lo que hay. La pantalla de
> Ajustes lo vigila permanentemente y avisa si la política se desconfigura.

El instalador aplica **exclusiones de antivirus** (escanear
terabytes de video **puede provocar errores de archivo en pleno aire**) —
**incluida la protección anti-ransomware "Acceso controlado a carpetas"**,
que bloquea escrituras a carpetas de medios en silencio aunque el escaneo
esté excluido, y produce fallos intermitentes casi imposibles de diagnosticar
a distancia —,
energía sin suspensión, arranque tras corte, rutas largas, y política de
actualizaciones sin reinicio.

> **Windows Update reiniciando la máquina a las 3 AM es el riesgo operativo
> #1** de una instalación en Windows. La pantalla de Ajustes lo vigila
> permanentemente, no solo el instalador.

> **Una exclusión de antivirus no puede convertirse en un agujero.** La
> carpeta donde caen los archivos del portal —`portal/entrada/`— **no hereda
> la exclusión**. Lo que sube un anunciante se examina ahí: límite de tamaño y
> duración, medición con `ffprobe` **sin acceso a la red y con límite de
> tiempo**, y el escaneo del antivirus del sistema si existe. Solo después de
> pasar todo eso el archivo se mueve a la biblioteca. **Nada que venga de
> afuera llega a la carpeta excluida sin pasar por ahí.**

> **Y si el antivirus se lleva `ffmpeg`**, el sistema lo distingue de
> cualquier otro fallo del encoder y lo dice con esas palabras: *"tu antivirus
> bloqueó ffmpeg — vuelve a aplicar las exclusiones en Ajustes"* (§9, paso 4).

**Linux** — systemd con reinicio automático y límites por cgroup. Paquetes
`.deb` y `.rpm`. **Docker opcional, nunca requerido.**

### El disco, la base y la corriente

**El disco lleno nunca saca el canal del aire.** Los umbrales son defaults
configurables: por debajo del **10 %** libre suena la alarma; por debajo del
**5 %** la grabación se purga sola hasta liberar espacio; por debajo del
**2 %** la base deja de escribir el as-run —y lo dice en rojo— **pero el aire
sigue**, saliendo del plan que ya está en memoria. Se pierde el registro, no
la señal.

**La base se revisa al arrancar.** Si la verificación de integridad falla, el
sistema **restaura solo el respaldo más reciente**, arranca y avisa lo que
pasó. Además, el plan de las próximas 48 horas vive también en memoria, así
que un problema de base no deja la pantalla en blanco ni el aire en silencio.

**Después de un apagón, el sistema cuenta lo que pasó.** Al volver manda por
el canal de avisos *"estuve fuera 6 horas 12 minutos"* y deja el incidente
anotado con su hora de inicio y de fin. Nadie tiene que reconstruirlo de
memoria para explicárselo a alguien después.

### Cuando no hay internet

**Nada de lo que necesita internet saca el canal del aire, y nada falla en
silencio.** Todo degrada con un aviso en cristiano:

- **La hora** — servidor de la red local como alternativa, y los avisos de
  arriba.
- **La guía** — se sigue generando y sirviendo localmente; la publicación
  remota se reintenta.
- **El canal de avisos** — los mensajes se acumulan y salen cuando vuelve la
  conexión.
- **Las fichas y carátulas** — ya están guardadas junto al archivo desde el
  ingest. **La cadena de aire funciona con el internet caído** (§10).
- **El portal del anunciante** — necesita que la estación sea alcanzable desde
  afuera. Se resuelve con Tailscale Funnel o un túnel de Cloudflare, y el
  asistente lo explica paso a paso. Sin eso, el portal sirve solo en la red
  local y el material entra por WhatsApp o a mano.
- **El respaldo** — a un segundo disco, a un USB o a una carpeta de red. La
  nube es una opción, no el camino por defecto.

> **Si Tailscale se cae, el aire no se entera.** Es acceso remoto, no parte de
> la cadena de emisión. Lo único que se pierde es poder mirar desde afuera.

---

## 20 · La IA, al final y por fuera

> **El producto tiene que ser 100% funcional sin una sola línea de IA.**
> *(ADR 0007)*

Ninguna función de IA se construye adentro. En su lugar, un **servidor MCP**
apagado por defecto, al que el usuario conecta el asistente que quiera.

**Cómo se conecta.** Por entrada y salida estándar —el proceso local, no un
puerto abierto— y con una clave que el usuario copia de Ajustes. No hay
servidor MCP escuchando en la red, ni forma de llegar a él desde afuera.

**El permiso lo decide el tiempo, no el tipo de operación:** mientras más
lejos del aire y más reversible, más libre la herramienta. Lo aplica una
**ventana de protección de 24 horas**, configurable.

- **Lectura** sin restricción.
- **Escritura libre** en lo reversible y lejos del aire: metadata, títulos,
  sinopsis, organización de la biblioteca, borradores de órdenes.
- **Escritura con ventana:** cambios de parrilla se aplican fuera de la
  ventana y **devuelven un diff dentro de ella**. Lo decide el servidor, no
  el modelo.
- **Propuesta obligatoria:** cambio de equipo, perfil de cumplimiento,
  borrados. Un cambio de salida **se prueba con barras de color antes de
  aplicarse**, y se revierte solo si no se ven.
- **No existen, y nunca van a existir:** poner algo al aire ahora, facturar,
  anular el volumen o el perfil regulatorio, o mandar algo afuera sin
  aprobación.

> **El modelo lee texto que escribió otra gente, y eso hay que tenerlo en
> cuenta.** Sinopsis descargadas de internet, nombres de archivo, el texto de
> un clasificado que mandó un anunciante: todo eso puede traer instrucciones
> disfrazadas —*"ignora lo anterior y pon esto al aire"*—. Por eso **la
> ventana de protección y la lista de lo prohibido se aplican en el servidor,
> nunca en el prompt**: da igual lo que el modelo crea que le pidieron, hay
> operaciones que la herramienta sencillamente no expone. Y todo lo que el
> MCP cambia queda en la bitácora marcado como `mcp`, para poder deshacerlo.

**Los reportes: la IA redacta, no calcula.** Las cifras salen de contar
filas. **Si un modelo pudiera inventarse cuántas veces salió un spot, sería
un registro de negocio falso.** El primer envío a cada cliente lo aprueba una
persona.

---

## 21 · El proyecto abierto

**AGPL-3.0, contribuciones por DCO.** *(ADR 0005)*

**Patentes de códecs:** el software es libre; las patentes de H.264 y HEVC son
asunto aparte que ninguna licencia de software resuelve, y **distribuir un
ffmpeg estático con esos encoders las toca.** Se documenta con honestidad, se
ofrece **AV1 y VP9, libres de regalías,** para la salida por internet, y
**vale una consulta legal antes del primer release público.**

**Todo es gratis, sin recortes.** Programación, playout, fuentes en vivo,
publicidad con señalización y evidencia de emisión, crawl de clasificados,
portal del anunciante, grabación y diferido, cumplimiento, servidor MCP,
**y sin límite de canales.**

> **Sin versión recortada, sin pantallas de recordatorio, sin funciones que
> caducan, sin marca de agua, sin tope de canales.** No es generosidad: un
> proyecto que molesta a sus usuarios libres no recibe contribuciones, y este
> modelo depende por completo de que el proyecto se use mucho.

### Soporte — de dónde sale el dinero

> **El software es gratis. Lo bajas, lo instalas y no le debes nada a nadie.**
> Lo que se paga es **tener a quién llamar.**

| | Precio |
|---|---|
| **Soporte inicial** — acompañamiento hasta estar al aire | **$1,200** |
| **Soporte continuo** — línea disponible, atención a fallos | **$100 / mes** |

Y aparte, a quien lo pida: drivers a medida para equipo poco común, perfiles de
cumplimiento de un país nuevo, migración desde otro sistema, adiestramiento.

**Por qué esto vale lo que cuesta y no es una comodidad:** una estación que
sale en negro a las 3 AM tiene un problema esa misma noche, y a esa hora no
hay a quién llamar. No se paga por instalar un programa — **se paga por que
haya alguien despierto del otro lado.**

Y por eso encaja con la AGPL sin fricción: **la licencia obliga a liberar el
código, no el trabajo ni la disponibilidad.**

> **La consecuencia se acepta de frente:** cualquiera puede bajarlo,
> instalarlo solo y no pagar nunca. **Eso no es una fuga — es lo que sostiene
> la comunidad.** Quien tiene tiempo y ganas lo hace solo; quien tiene una
> estación que atender prefiere tener a quién llamar.

Diez estaciones con soporte continuo son **$12,000 al año recurrentes** sin
haber vendido una sola licencia.

```
README.md         de cero al aire en menos de una hora — español e inglés
LICENSE           AGPL-3.0
CONTEXT.md        el vocabulario del dominio
CONTRIBUTING.md   DCO, asistencia de IA con revisión humana, cómo aportar
COMPLIANCE.md     qué hace y qué NO hace por tu cumplimiento legal
COMPRAR.md        guía de compra de equipo, con precios reales
docs/adr/         las decisiones de arquitectura y su porqué
docs/drivers/     cómo añadir soporte para tu equipo
docs/profiles/    cómo contribuir el cumplimiento de tu país
docs/hardware/    MATRIZ DE COMPATIBILIDAD mantenida por la comunidad
examples/         universidad · ONG · low-power · internet
```

**La matriz de compatibilidad decide la adopción.** *"¿Sirve con mi equipo?"*
es la primera pregunta de todo el que llega.

GitHub Actions compila cada release para Windows, Linux y ARM, con
instaladores, sumas de verificación y binarios firmados. **Sin builds
nocturnas: un canal al aire no corre código sin versionar.**

**Documentación en español e inglés desde el día uno.** El mercado natural es
LATAM y casi todo el software de broadcast existe solo en inglés.

---

## 22 · Las fases

| Fase | Entrega |
|---|---|
| **F0 · Prueba de concepto** | **Un experimento con criterio de pase o fallo, no un objetivo.** Ver §22.1. Decide la pregunta técnica más cara del proyecto —el contrato entre decodificador y encoder— y convierte en medidas las cifras estimadas del §18. **Son cinco días de trabajo, no cinco días de calendario**, y hay que presupuestar una segunda corrida de 8 horas: la primera casi nunca es la buena. |
| **F1 · Fundación** | Esquema con `channel` en cada tabla desde el día uno · contenido con medición real y fichas desde etiquetas, `.nfo`, Cover Art Archive y TVmaze (sin clave) · **editor de reglas y línea de tiempo** · resolver · **guía validada** · avisos de vencimiento · **plan y guía revisables** — el modo sombra en su forma de F1: Antena787 propone y Rolando compara a ojo contra lo que emite con VLC. La comparación automática contra el aire real necesita la grabación de F2. **No toca el aire.** |
| **F2 · Playout** | La fase grande. Motor completo · conformado · **los cuatro decks y su prioridad** · **salida `udp-ts`, la de CAtv** · **fuentes en vivo por SRT/RTMP, con IDs, cortinillas y música programados adentro** (los cortes *vendidos* llegan en F4) · **manual con regreso automático** · **grabación de la salida y diferido** · **logo del canal** · asistente con la prueba de barras · cascada de respaldo · relleno · tablero · alarmas · **registro de incidentes**. Desglosada abajo. **Prueba de resistencia de 30 días con instrumentación de deriva A/V.** |
| **F2.5 · Endurecimiento** | Que un Windows 10 sin presupuesto aguante 24/7: exclusiones de antivirus verificadas, NTP, política de Windows Update, vigilancia permanente en Ajustes, umbrales de disco, recuperación de la base, respaldos. **Corre en paralelo al soak de 30 días**, porque es justo lo que el soak pone a prueba. **Rolando necesita esta fase aunque el proyecto nunca se publique** — por eso no vive dentro de "abrir el repositorio". |
| **F3 · Público** | Repositorio abierto · CI · instaladores · documentación bilingüe · matriz de hardware · guía de compra · lo legal. Modo internet completo. **Solo eso**: el endurecimiento ya pasó en F2.5. |
| **F4 · Emisora** | Publicidad con prioridad y rotación · **el deck comercial: cortes vendidos, dentro y fuera de bloques en vivo** · **crawl de clasificados** · **portal del anunciante: enlace, entrega y cobro** · **SCTE-104**, con su prueba de 48-72 h · reconciliación con alertas · evidencia de emisión. Perfil `us-fcc`. |
| **F4b · Radio** | Formato de casa solo audio · salida de audio · el resto ya funciona tal cual. **Para CAtv no es después: WRBM 89.3 FM está en el mismo sitio** (§17). **Sin rotación musical** — sirve para radio hablada, deportiva y de programas. |
| **F4c · Rotación musical** | Categorías, relojes por hora y reglas de separación. **Es lo que falta para radio musical, y es un subsistema propio, no un ajuste.** |
| **F5 · Internacional** | Perfiles `eu-ebu` e `isdb-latam` · subtítulos DVB y ARIB · exportación PSIP y DVB-SI. Y aquí, si alguna vez, la ruta `inyeccion-ts` de señalización de cortes — **marcada sin precedente documentado** (§9, paso 7). **Libre, en el núcleo.** |
| **F5b · Escala** | Multi-canal · roles · redundancia · **API REST completa e integraciones** · Postgres opcional. **Libre, como todo.** |
| **F6 · MCP** | **La última fase, y solo cuando todo lo anterior funcione sin ella.** |

### 22.1 · La F0, definida como experimento

**Pregunta que responde:** ¿el diseño "servidor de cuadros en Go entre
decodificadores por clip y un encoder persistente" produce salida continua y
limpia durante horas en hardware modesto?

**Archivos de prueba, todos con contenido real y distinto entre sí:**

| # | Qué prueba |
|---|---|
| 1 | H.264 1080p29.97 AAC estéreo — el caso normal |
| 2 | H.264 720p59.94 — cambio de resolución y de cuadros |
| 3 | MPEG-2 480i29.97 AC-3 — material de archivo viejo |
| 4 | HEVC 1080p25 — cuadros PAL en un canal NTSC |
| 5 | Cuadros **variables** (VFR, típico de grabaciones de OBS) |
| 6 | Audio mono, y otro con 5.1 |
| 7 | Uno con **subtítulos CEA-608 embebidos** |
| 8 | Uno con el audio **200 ms más corto** que el video, a propósito |
| 9 | Uno **corrupto** en el último segundo |

**Qué se hace:** se normalizan los nueve al formato de casa 720p59.94, y se
encadenan en bucle durante **8 horas** hacia un archivo de salida, con dos
salidas simultáneas de volumen distinto — **una de ellas en el formato exacto
que recibe el multiplexor de CAtv: MPEG-2 720p59.94, audio MPEG capa II y
AC-3, CBR por UDP** (§10), porque de nada sirve probar el motor en un
formato que el transmisor no va a ver. Se corre en **el Windows 10 de
Rolando o una máquina equivalente**, no en la laptop del desarrollador.

**Criterio de pase, todo medido con herramientas, no con el oído:**

| Medida | Pasa si |
|---|---|
| Discontinuidad de muestras de audio en cada cambio de clip | ninguna por encima de −40 dBFS |
| Desfase audio-video acumulado a las 8 horas | **menos de 20 ms** |
| Cuadros duplicados o perdidos en cada cambio | 0 |
| Continuidad de marcas de tiempo en la salida | monotónica, sin saltos |
| Tasa de la salida `udp-ts` y jitter de PCR | bitrate constante ±1 %, PCR ≤ 40 ms, sin errores de continuidad |
| Los subtítulos del archivo 7 | llegan a la salida |
| El archivo 9 corrupto | cae a relleno sin negro y queda registrado |
| CPU y RAM con una salida, y con dos | **medidos**, y reemplazan los estimados del §18 |
| ¿Cuántas sesiones de encoder de hardware aguanta la máquina? | medido |

**Ocho horas es el criterio en la máquina de destino. En desarrollo bastan
dos:** el límite de deriva son 20 ms en 8 horas —2.5 ms por hora— y el
analizador resuelve 1 ms, así que a las 2 horas cualquier deriva real ya se
ve. La primera corrida larga (Mac M4, 8 de septiembre de 2026, 1 h 55 min,
413,302 cuadros, 276 cortes) pasó todo lo medible con deriva máxima de
1 ms: `docs/f0/REPORTE-mac-m4-2026-09-08.md`. La de la PC de CAtv sigue
pendiente y es la que cierra F0.

**Si falla el desfase o los cuadros perdidos, el diseño del servidor de
cuadros se replantea antes de la F1.** Cinco días de trabajo en F0 valen más
que doce semanas construidas sobre un supuesto.

**Al terminar F5 el producto está completo.** F6 es aditiva: si nunca se
hiciera, Antena787 seguiría siendo un sistema entero. *Esa es la prueba de
que la IA está en el lugar correcto.*

**F3 va antes que F4 a propósito.** Un canal universitario no necesita
publicidad — necesita que el proyecto exista y funcione.

### 22.2 · Cuánto tarda esto de verdad

**El supuesto que manda:** unas **10 horas a la semana**, que es más o menos
**un día de trabajo efectivo**. No es una queja, es el ritmo real de un
proyecto que se hace al lado de un trabajo. Todo lo de abajo sale de ahí.

| Hito | Optimista | Probable | Pesimista |
|---|---|---|---|
| **F0 decidida** | 2 semanas | 1 mes | 2 meses |
| **F1 · modo sombra** (Antena787 propone, Rolando compara) | 7 meses | **~1 año** | 1 año y medio |
| **F2 · madrugadas al aire** | 2 años | **~3.4 años** | 4 años y medio |
| **F2.5 + soak · aire completo** | 2 años y medio | **~3.9 años** | 5 años |
| **F4 · primer anunciante cobrado** | 4 años | **~5.6 años** | 7 años |

**Esto no es pesimismo, es aritmética.** ffplayout —un proyecto más chico, sin
publicidad, sin portal y sin cumplimiento— tomó más de dos años en llegar a
algo estable. Decirlo aquí evita la conversación incómoda del mes catorce.

> **La única palanca real es más horas en F1 y F2.** Recortar F4 no acelera
> nada: la publicidad viene después del aire, y quitarla no adelanta el aire
> ni un día. Duplicar el ritmo a 20 horas semanales sí parte los números por
> la mitad. Es la decisión que hay que tomar con los ojos abiertos.

### 22.3 · F2 por dentro

Una sola fila de tabla escondía **cuatro o cinco subsistemas, cada uno del
tamaño de F1**, y todos con revisión humana línea por línea (§16): son unas
7,000 líneas críticas, que son unas 35 horas **solo de leerlas**. Este es el
orden, y cada paso se apoya en el anterior:

1. **Motor y conformado** — el servidor de cuadros, el pre-roll, el encoder
   persistente. Es lo que valida la F0.
2. **Los decks y su prioridad** — quién tiene el aire en cada instante.
3. **La salida `udp-ts`** — la de CAtv, la primera que existe.
4. **El detector de silencio y negro sobre la salida** — porque sin él nada de
   lo anterior se puede verificar solo.
5. **Fuentes en vivo** — SRT/RTMP, margen de gracia, reconexión, elementos
   programados adentro.
6. **Manual** — tomar y soltar el control, la escalera de regreso, el único
   tenedor.
7. **Grabación y diferido** — con el retorno de aire de `capture_input`.
8. **Cascada y watchdog** — el último escalón y el que vigila al encoder.

**Los ocho no se pueden reordenar mucho**, y ninguno se puede saltar sin
dejar el canal sin una de las promesas del §4.

---

## 23 · Métricas de éxito

| Métrica | Meta |
|---|---|
| **De cero a barras de color en pantalla** | **< 15 minutos** |
| **De cero a canal al aire con programación** | **< 1 hora** |
| **Instalaciones abandonadas en el asistente** | **< 10%**, medido en las instalaciones con soporte — no hay telemetría, y no la habrá sin consentimiento explícito |
| Preguntas al usuario durante la instalación | ≤ 6 |
| **Funcionalidad completa sin IA** | **100%** |
| Horas sin llenar en el despliegue de referencia | de ~30% a < 1% |
| Exactitud de la guía contra el as-run | 100% de los programas en su espacio; tolerancia de ±30 s en el instante |
| Interrupciones no planificadas — negro, silencio o cartel sin que lo pidiera el plan | 0 en 30 días. Una fuente en vivo ausente cubierta por relleno **no** cuenta: eso es el sistema funcionando |
| Reinicios no planificados del sistema operativo | 0 |
| Instalaciones de terceros | 10 canales en 12 meses |
| **Países distintos con instalación activa** | **3 en 12 meses** |
| Contribuidores externos con código mezclado | 3 en 12 meses |

---

## 24 · Riesgos

| Riesgo | Mitigación |
|---|---|
| **La gente abandona en la instalación** | El asistente y la prueba de barras son trabajo principal. Medir dónde se abandona. **Riesgo #1.** |
| **El motor es nuestro y lo escribe una IA** | Revisión humana línea por línea y prueba de 30 días (§16). **No negociable.** |
| **El encoder de la estación no habla SCTE-104** | No se construye una ruta alterna en v1 (§9, paso 7): los cortes se venden y se emiten localmente, con su evidencia completa. Lo único que se pierde es enchufarse a sistemas de inserción de terceros. |
| **El calendario real es de años, no de meses** | Dicho de frente en §22.2, con tres escenarios. La palanca es más horas en F1 y F2; recortar F4 no adelanta el aire. |
| **Sin retorno de aire, la verificación no verifica** | Se dice, no se disimula: modo `degradado`, marcado manual, y `signal-compare` no promete nada *(ADR 0009)*. |
| **Servir a grandes complica al pequeño** | Revelación progresiva. Probar el flujo del usuario único en cada release. |
| **El editor no resulta fácil** | Reglas como entrada, línea de tiempo como vista, propuesta automática. **Probarlo con alguien no técnico antes de publicar.** |
| **Internacionalizar sin conocer las normas** | Fuente citada y mantenedor local, o experimental |
| **Se le vende a alguien un cumplimiento que no existe** | `COMPLIANCE.md` dice qué hace y qué no. **El as-run es evidencia comercial, no un requisito de la FCC** (§12). **Las alertas se cumplen con hardware certificado.** |
| **Windows Update reinicia el aire** | Política de actualizaciones y vigilancia permanente en Ajustes (F2.5). IoT LTSC sería mejor, pero cuesta dinero y el despliegue de referencia no lo tiene. |
| **Antivirus bloquea o degrada** | Exclusiones automáticas verificadas, y la carpeta del portal **fuera** de la exclusión (§19) |
| **La versión libre se percibe como recortada** | No hay versión recortada: **todo es libre, sin tope de canales.** **Regla no negociable.** |
| **El MCP como puerta trasera al aire** | Ventana de protección aplicada **en el servidor, no en el prompt** |
| **Reporte de emisión con cifras inventadas** | Determinista. **La IA redacta, no calcula.** |
| **Nadie contribuye** | Publicar en F3, no al final. Matriz de hardware desde el principio. |

---

## 25 · Lo que todavía no sabemos

**Supuestos de diseño** *(fijados el 8 de septiembre)*. Antena787 no se
diseña para CAtv: se diseña para cualquier estación, con cualquier equipo, en
cualquier país, y CAtv es el primer sitio donde se prueba. Por eso **cada
respuesta de abajo es una opción del sistema, no una suposición**: LPTV y
Class A los dos; 29.97 y 59.94 los dos; de 1 a 10 TB; telemetría por USB,
red o SNMP; ENDEC por serial, relés o red; reloj de cortes por fuente; y
tanto un canal de TV como una radio que también sale online o por TV.

**Lo que ya se sabe de CAtv** *(Rolando, 8 de septiembre)*
- **Cadena completa**: Antena787 → UDP (hoy: MistServer + VLC, MPEG-2 720p con audio MPEG) → **Technalogix TP1000, multiplexor** (2 ASI in, 2 ASI out, 2 Ethernet: manejo y stream) → ASI → excitador **RVR Blue Digital Video** (605 MHz, ASI in, red de manejo sin IP de transporte) → amplificador **ADR** ~300 W. Detalle en §17.
- **720p a 59.94** al multiplexor → formato de casa 720p59.94; la lista sigue ofreciendo todo lo demás (§13, paso 6).
- **Class A** → el perfil `us-fcc` enciende lo de Class A: archivo político, archivo público en línea, subtítulos según ingresos (§12).
- **`RadioOnce Live!` corta cada 15 minutos** (se supone) → `reloj_de_cortes = [0, 15, 30, 45]` en la fuente; se ajusta si el programa dice otra cosa. Es un programa de **WRBM 89.3 FM, Océano Radio**, que está en el mismo sitio → F4b (radio) es para CAtv.
- **Sage Digital ENDEC** con audio XLR de entrada y salida y **relés en el bloque verde** de atrás; monitorea WCMN 1280 AM, WKAQ-FM 104.7 y NOAA canal 1 por un TFT EAS 930A. Entra al TP1000 por audio y video compuesto. → `gpi-serial` sobre los relés primero; el modelo exacto lo confirma el ingeniero.
- **Disco: 3.5 TB** — SSD M.2 de 500 GB y HDD de 3 TB → sistema y base en el SSD; biblioteca y grabación en el HDD; con ~1.2 TB de biblioteca caben 30 días de grabación a 720p (§9, paso 8).
- **Windows 10**, sin presupuesto → sin IoT LTSC y sin segunda máquina.
- **Día de emisión** a la hora que la estación diga (§8). **Subtítulos** se conservan y se suben (§9, paso 1). **Contenido** en el servidor con respaldo a la web → driver `nube`.
- **Retorno de aire**: tarjeta receptora de TV en la máquina + monitoreo por streaming → `capture_input` tipo `receptor-tv`; `signal-compare` verifica de verdad (ADR 0009).
- **Telemetría**: el ADR muestra potencia directa/reflejada, corriente, voltaje y temperatura; el RVR tiene USB y red de manejo → driver `serial-usb` o `snmp`, lo que el ingeniero confirme.

**A verificar antes de publicar**
1. ¿El Raspberry Pi 5 tiene encoder H.264 por hardware? Decide si la
   recomendación vuelve (§18).

**Del despliegue de referencia (CAtv) — lo que sigue abierto.** Nada de esto
bloquea el diseño ni una fase: el sistema asume todas las variantes. Solo
decide qué driver se prueba primero en CAtv.
2. **Modelo exacto del Sage** (1822 o 3644) — no hace falta antes de
   instalar: el asistente prueba serial, relés y red y usa lo que responda.
3. **Camino de la telemetría** (USB, SNMP, web, contactos) — ídem: se prueban
   todos los que tengan cable.
4. **Obligación de subtítulos** — el ajuste arranca en "no sé" y los subtítulos
   se conservan y se suben igual; solo cambia si avisa.

**Ninguna bloquea F0 ni F1.**

---

## 26 · Por qué vale la pena

Lanzar un canal de televisión hoy cuesta más en software que en transmisor.

Pero el software barato no basta si hace falta un ingeniero para encenderlo,
ni si solo sirve a un tamaño, ni si solo funciona en un país. **La facilidad
de uso, la escala y la portabilidad no son funciones de este proyecto: son el
proyecto.**

Una universidad con un canal de práctica. Una comunitaria que repite el mismo
bloque porque no tiene quién arme la parrilla. Una emisora regional que
perdió su anunciante porque no pudo probar que el spot salió.

Que cualquiera de ellos compre un mini PC, baje un archivo, conteste seis
preguntas y esté al aire en una hora.

**Que solo le falte comprar el equipo.**
