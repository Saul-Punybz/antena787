# Cómo añadir soporte para un equipo

> **¿Hay un sitio de donde sacar los drivers?** No: cada equipo trae un
> protocolo, no un driver. Qué protocolo habla cada uno, dónde está su
> manual y con qué biblioteca de Go se escribe está en
> [`CATALOGO.md`](CATALOGO.md) (10 sept 2026).

Antena787 no asume ninguna marca ni ningún modelo. El despliegue de
referencia es **donde se prueba primero, no el molde**: cada equipo que
aparece ahí es un ejemplo de una familia que tiene driver (PRD §2).

Este documento es para quien quiere hacer que el sistema hable con un equipo
que todavía no soporta.

> ## Antes que nada: hoy un driver empieza por una propuesta, no por código
>
> **`internal/drivers/` ya existe, y solo una familia tiene interfaz.** La
> de **salida** la definió T2 el 10 de septiembre de 2026
> ([`internal/drivers/salida`](../../internal/drivers/salida)): un `Driver`
> con `Abrir`, `Vigilar` y `Descripcion`, con `udp-ts` y `archivo`
> construidos. Las demás familias todavía no tienen interfaz común: lo que
> hay son **paquetes de un equipo concreto**, que hablan su protocolo y
> entregan lo suyo —
> [`alerta/sage`](../../internal/drivers/alerta/sage) (11 sept) y
> [`captura/hdhomerun`](../../internal/drivers/captura/hdhomerun) (10 sept)—
> sin nada que los generalice todavía, y sin cablear a la aplicación:
> `capture_input`/`signal-compare` y el ENDEC son trabajo de F2 (T8).
> Aparte está `engine.Output` en
> [`internal/engine/encoder.go`](../../internal/engine/encoder.go), la
> estructura que describe una salida del encoder, que tampoco es la interfaz de
> un driver.
>
> Así que hoy un driver nuevo empieza **abriendo un issue** con lo que el
> equipo necesita, no con una implementación. Esa propuesta es la que va a dar
> forma a la interfaz cuando se escriba, y es más útil ahora que el código.
> Ver [el flujo](#el-flujo-issue--rama--pr).

---

## Las familias, y qué driver tiene cada una

Todo lo que toca el mundo exterior es un driver, y todo se escoge de una
lista — **nunca de una suposición** (PRD [§10](../../PRD.md#10--los-drivers)).

### Salida

Por dónde sale la señal del canal. **Varias por canal, simultáneas, cada una
con su propio objetivo de volumen y su reconexión.**

| Driver | Qué es |
|---|---|
| `red` | **UDP-TS y RTP. El primero que se construye**: es lo que acepta un multiplexor de transmisor |
| `internet` | RTMP, HLS, SRT |
| `route-dash` | ATSC 3.0. Futuro |
| `archivo` | Para el modo sombra y las pruebas |
| `ninguna` | |

**Lo que un multiplexor exige de `udp-ts`, y por eso no es "mandar un
stream".** Un multiplexor de transmisor junta varios programas en un ASI de
tasa fija, así que lo que entra tiene que portarse: **tasa constante** (CBR con
paquetes nulos, al bitrate que se le dijo), **PIDs y número de programa fijos**
que la persona escribe una vez, **PCR cada 40 ms o menos**, PAT/PMT repetidas a
tiempo, y **video MPEG-2** —lo que ATSC 1.0 transmite— con audio **AC-3 o MPEG
capa II**, a elegir. Todo eso es configuración del driver, visible como *"lo
que tu multiplexor espera"*, nunca como flags.

**Orden de prioridad, fijado el 8 de septiembre.** Todas se construyen; el
orden dice cuál se prueba y se pule primero: **1)** `udp-ts` MPEG-2 CBR hacia
un multiplexor ATSC 1.0 · **2)** `internet` en H.264 (SRT, RTMP, HLS) ·
**3)** `archivo` · **4)** los formatos de otros países, DVB e ISDB, con su
perfil (F5) · **5)** ATSC 3.0 por `route-dash`, con HEVC y AC-4, cuando haya
un transmisor donde probarlo. **Ninguno se recorta; el que va primero es el
que está al aire.**

### Entrada en vivo

| Driver | Qué es |
|---|---|
| `srt-listen` | **El preferido, por latencia.** Baja de un segundo sin depender de ningún SDK propietario |
| `rtmp-listen` | Añade típicamente de 2 a 5 segundos de latencia, y eso alimentando un transmisor es mucho |
| `ninguna` | |

**NDI queda fuera**: su SDK obliga a enlazar C, y SRT resuelve la latencia sin
esa deuda *(ADR [0002](../adr/0002-no-cgo.md))*.

**Y las entradas en vivo llevan contraseña.** SRT no se abre sin ella —tampoco
la entrada rápida del §9 paso 5—, porque un puerto de vivo abierto en una red
local es una forma bastante directa de poner cualquier cosa al aire (§19).

### Señalización de cortes

| Driver | Qué es |
|---|---|
| `scte104-tcp` | **La ruta primaria.** Antena787 manda SCTE-104 y el encoder genera el SCTE-35 *(ADR [0004](../adr/0004-scte104-to-the-encoder.md))* |
| `gpi-out` | Cierre de contacto hacia el encoder o el insertador |
| `hls-daterange` | Salida web |
| `ninguna` | |

**El protocolo de `scte104-tcp` ya está escrito** (11 sept 2026), en
[`internal/drivers/senal/scte104`](../../internal/drivers/senal/scte104): los
dos sobres del estándar (`single_operation_message` y
`multiple_operation_message`), el encuadre de TCP —donde el largo va **dentro**
del mensaje, en su `message_size`, y no como un prefijo aparte—, y los mensajes
que hacen falta para el diálogo completo con un inyector: `init_request`,
`alive_request` cada 10 s, `splice_request_data` en sus cinco tipos (empezar y
terminar, normal e inmediato, y cancelar), `time_signal_request`, `insert_DTMF`,
`inject_section` y `proprietary_command`. El `Cliente` es el *automation system*
de la jerga del estándar: sostiene el enlace, reconecta con espera progresiva
1, 2, 4… con tope de 60 s sin rendirse nunca, y cuenta reintentos y último
error para la pantalla con el mismo vocabulario que las salidas (F2-48). **No
está cableado al motor ni al plan**: eso es otra tanda. Como no hay biblioteca
de SCTE-104 en ningún lenguaje salvo una en TypeScript, y el estándar está tras
registro, la forma de cada mensaje se sacó de las dos únicas implementaciones
libres que existen —`astronautlabs/scte104` (TypeScript) y el SCTE-104 de
`stoth68000/libklvanc` (C, en producción en Open Broadcast Encoder)— leídas byte
a byte y comparadas entre sí; **el `splice_request_data` está verificado de ida
y vuelta contra un vector de bytes ajeno** y el resto solo contra la estructura
que documentan las dos fuentes, que es lo que queda anotado en el código, campo
por campo. Los dos desacuerdos encontrados también: el `pre_roll_time` del corte
va en milisegundos (no en décimas, como dice un comentario de libklvanc) y el
segundo vector de prueba de la referencia de TypeScript tiene dos bytes de
sobra. La prueba contra un inyector real, con su soak de 48-72 horas, sigue
pendiente.

**`inyeccion-ts` no está en la versión 1.** Inyectar las secciones aguas abajo,
en el flujo ya multiplexado, cuesta de tres a cuatro veces lo que cuesta el
crawl de clasificados, es en la práctica escribir un remuxer, y **no tiene un
caso de referencia documentado en ningún proyecto libre.** Queda apuntado para
F5 y marcado **sin precedente documentado**.

**Y si el encoder de una estación no habla SCTE-104, no pasa nada grave:** se
venden y se emiten los cortes localmente, con su evidencia de emisión completa.
Lo único que no se puede es enchufarse a sistemas de inserción de terceros.

### Alertas de emergencia

`gpi-serial` · `gpi-gpio` · `sage-endec` · `dasdec` · `syslog` · `snmp-trap` ·
`cap-poll` · **`signal-compare`** · `ninguna`.

**En Estados Unidos los equipos son pocos y se conocen, y se traen todos:**
Sage Digital ENDEC **1822** (serial RS-232 y relés; manual público, verificado)
y **3644** (serial, relés y red: HTTP y la interfaz de automatización por
TCP —el mismo protocolo del serial, por red— confirmados; syslog y SNMP
**no aparecen en ninguna fuente pública**, se confirman contra el equipo); DASDEC
de Digital Alert Systems (red: HTTP confirmado, SNMP/syslog no encontrados;
y relés); Gorman-Redlich (relés y serial); TFT (relés). Equipo por equipo,
con fuentes: [`CATALOGO.md`](CATALOGO.md).

**No hace falta saber el modelo antes de instalar.** El asistente pregunta **por
dónde está conectado** —cable serial, cable de relés, cable de red, o varios— y
prueba cada uno. **Si hay dos caminos, se usan los dos y se cruzan.**

**`sage-endec` ya existe en Go** (`internal/drivers/alerta/sage`, 11 sept 2026),
y es el primer driver de alerta escrito. Decodifica —y solo decodifica, ADR
[0010](../adr/0010-eas-integrate-the-endec-never-replace-it.md)— el estado que
el ENDEC publica por su cuenta: las líneas `local:` / `match:` / `nomatch:` /
`dup:` del *decoder device*, la cabecera SAME
`ZCZC-ORG-EEE-PSSCCC…+TTTT-JJJHHMM-LLLLLLLL-` partida en campos (con las hasta
31 zonas FIPS, el `+TTTT` que es HHMM y no minutos, y el año del día juliano
resuelto por cercanía), el `NNNN` de fin de mensaje, la copia cruda del
*encoder device* con sus bytes de sincronismo `0xAB`, y los bloques
`<ENDECSTART>`/`<ENDECEND>` del *news feed*. Separa **prueba semanal (RWT),
prueba mensual (RMT) y alerta real** con los mismos tres valores de
`alert_event.tipo`, porque una prueba no es una interrupción comercial. Los dos
transportes —serial con `go.bug.st/serial` para el 1822 y el 3644, y TCP contra
la interfaz de automatización del 3644— comparten un solo analizador, tolerante
a lecturas partidas y a ruido, y reconectan con espera progresiva de 1 a 60 s.
**No genera nada**: ni cabeceras, ni tonos, ni `NNNN`, y no le escribe al ENDEC.
Y dice lo que no puede saber: el *decoder device* **no manda un mensaje de
«alerta terminada»**, así que por ese cable se sabe cuándo empieza la
interrupción y no cuándo acaba — eso lo cierra el relé PTT, el retorno de aire o
nada, y el as-run tiene que decir cuál. Los relés están **documentados y sin
implementar** (`reles.go`: qué significa cada programa del bloque verde, y por
qué la entrada *Manual Override* del «commercial tally» es una escritura que un
driver no decide solo), con la interfaz mínima `EntradaDeContacto` esperando a
que se sepa qué placa serial hay en la estación. Cablearlo al motor es T8.

> **`signal-compare` es la respuesta a "cualquier marca, cualquier país".** No
> le habla al equipo: compara **la señal transmitida** contra la que el plan
> decía. **Funciona con hardware que el proyecto nunca va a tener en la mano**
> — incluido el EWBS de Sudamérica. Y por eso mira el **retorno de aire**, no
> nuestra propia salida: comparar contra lo que nosotros mandamos no detecta
> nada, porque el motor siempre cree que emitió *(ADR
> [0009](../adr/0009-truth-is-the-transmitted-signal.md))*.

**Antena787 no construye el equipo de alertas.** Es hardware certificado aguas
abajo, y **las alertas de emergencia se cumplen con hardware certificado, no
con este software** (§5, §12).

**El decodificador de SAME, que es la mitad de `signal-compare`.**
`internal/drivers/alerta/same` oye el protocolo SAME —el de 47 CFR 11.31: FSK a
520.83 baudios, marca 2083.3 Hz, espacio 1562.5 Hz, la cabecera
`ZCZC-ORG-EEE-PSSCCC+TTTT-JJJHHMM-LLLLLLLL-` tres veces y el `NNNN` del final—
en el audio del **retorno de aire**, y por un canal va soltando qué alerta se
oyó, de qué zonas, con cuánta duración y **a qué hora exacta**. Para qué sirve:
para que el as-run diga *«la alerta salió al aire»* y no *«el ENDEC dijo que la
mandó»*, que son dos cosas distintas — un ENDEC que disparó sin que el aire
cambiara es un incidente, y esta es la única manera de verlo sin creerle a
nuestro propio motor (ADR [0009](../adr/0009-truth-is-the-transmitted-signal.md),
ADR [0010](../adr/0010-eas-integrate-the-endec-never-replace-it.md) capa 2). Es
la línea de evidencia que hoy no produce ningún proyecto libre, y no hay ningún
decodificador de SAME en Go: este se escribió a mano, sin enlazar nada. Lo que
**no** es: no emite alertas, no genera cabeceras SAME, no sintetiza la señal de
atención de 853+960 Hz —solo la oye, para poder decir que el mensaje salió
completo— y no reemplaza al ENDEC ni vale como cumplimiento de la Parte 11. En
el paquete no hay modulador ni lo va a haber: el que hace falta para las pruebas
vive en un archivo `_test.go`, así que el binario **no puede** emitir SAME ni por
accidente. Y no decide nada: cuenta lo que oyó, con un campo de confianza que
dice con cuántas de las tres repeticiones se armó la cabecera; quien decide qué
hacer con eso es el motor.

### Retorno de aire (`capture_input`)

| Driver | Qué es |
|---|---|
| `receptor-tv` | Tarjeta sintonizadora en la propia máquina |
| `captura` | Entrada de video alimentada por el RF MONITOR del excitador o por un receptor externo |
| `stream` | URL de monitoreo del transmisor o de la red |
| `ninguno` | **Modo `degradado` declarado**: `signal-compare` no promete detectar nada y las interrupciones se marcan a mano |

Del retorno sale también el **monitor por streaming**: una copia de baja
calidad de lo que de verdad está al aire, para verla desde el teléfono.

**`stream` por un SiliconDust HDHomeRun ya tiene paquete** en
[`internal/drivers/captura/hdhomerun`](../../internal/drivers/captura/hdhomerun)
(10 sept 2026, catálogo §1): `Descubrir`/`DescubrirEnHost` (discover.json),
`Lineup` (lineup.json) y, sobre la URL de cada canal, `Abrir` —el TS crudo con
reconexión de 1 a 60 s— y `Medir` —una ventana de tiempo analizada con
`internal/ts` para PIDs, continuidad, PCR y si hay video y audio—. Es el
ejemplo más barato de `stream` (catálogo, "por qué `stream` es el tipo más
barato de construir"): sin CGo, con `net/http` puro, y exactamente lo que
`signal-compare` necesita cuando la estación no tiene ya un retorno de aire
(ADR 0009). El nivel de señal (`Señal.FuerzaPct/RuidoPct/SimboloPct`, ss/snq/seq
en la Guía de Desarrollo de SiliconDust) es mejor esfuerzo: el equipo lo
publica por el protocolo binario de `hdhomerun_config`, documentado, y por un
`status.json` HTTP que traen los modelos recientes pero que SiliconDust no
documenta por escrito — si no aparece, `Medir` no falla por eso, deja la señal
sin dato.

### Transmisor (telemetría, solo lectura)

`snmp` · `http` (página web del equipo) · `serial-usb` · `gpi-estado`
(contactos: al aire, falla, reflejada alta) · `ninguno`.

**Se traen todos y se pueden combinar:** el asistente pregunta qué cables hay
—red de manejo, USB, contactos— y prueba cada uno; lo que responda, se usa.

Lee lo que el excitador y el amplificador ya muestran en su pantalla —potencia
directa y reflejada, corriente, voltaje, temperatura— y lo pone en *Al aire*.
**Es la única respuesta física a "¿estamos al aire?"**: potencia directa en cero
es fuera del aire, diga lo que diga la red; reflejada subiendo es antena o
cable; temperatura subiendo es ventilación. Alarma en los tres.

**SNMP es el común denominador** —GatesAir, Rohde & Schwarz, Anywave, Comark en
TV; Nautel, Broadcast Electronics y Elenos en radio— y lo que no habla SNMP
suele tener página web o puerto serial/USB. **El driver se elige por cómo se
llega, no por marca; la marca solo carga la tabla de nombres.**

### Respaldo

`disco` (segundo disco o USB) · `red` (carpeta compartida) · `nube` (S3,
Backblaze, Google Drive) · `ninguno`. **Un respaldo que sale de la máquina va
cifrado** (§19).

### Canal de avisos

`telegram` (gratis) · `whatsapp` (por proveedor) · `correo` · `ninguno`.

Es por donde llegan las alarmas, los avisos de vencimiento a 7 días y el
*"estuve fuera 6 horas 12 minutos"* de después de un apagón. **Si no hay salida
a la red, el aviso se guarda y se manda cuando vuelva.**

### Cobro del anunciante

`stripe` · `ath-movil` (Puerto Rico) · `mercado-pago` (LATAM) · `paypal` ·
`transferencia` · `efectivo` · `ninguno`.

**El cobro es un driver como cualquier otro, y esto importa en este mercado.**
Stripe no está disponible en buena parte de LATAM, y donde está a menudo no es
lo que la gente usa: **ATH Móvil es probablemente más importante que Stripe
para el primer usuario** — una ferretería paga por ATH sin pensarlo, pero sacar
una tarjeta para un anuncio de $20 al mes es más fricción de la que vale el
anuncio.

`transferencia` y `efectivo` **no procesan nada: solo registran que el cliente
pagó**, así el portal sirve igual para quien cobra en mano.

**Dos reglas duras:** el dinero **nunca pasa por Antena787** —cada estación
conecta su propia cuenta y cobra directo, el software nunca custodia fondos de
terceros—, y **cada pago lleva su clave de idempotencia**, así que un aviso
repetido de la pasarela **no cobra ni acredita dos veces** (§15).

### Fichas y carátulas

El orden es **local primero, red al final**, y dentro de la red **primero lo
que no pide clave**:

| Nivel | Driver | Clave | Uso comercial |
|---|---|---|---|
| **1 · sin red** | `tags-embebidas` · `nfo-local` (Kodi) · `caratula-embebida` | — | libre |
| **2 · sin clave** | `coverart-archive` (MusicBrainz) — **música** | **no** | libre |
| | `tvmaze` — **series de TV** | **no** | libre |
| **3 · clave gratis** | `tmdb` — **el de mejores datos** | sí, gratis | **sí, con atribución** |
| **4 · con reservas** | `discogs` | sí | límites por minuto |
| | `omdb` | sí | 1,000/día; alta resolución solo con patrocinio |
| | `fanart-tv` · `theaudiodb` | sí | **⚠ gratis solo NO comercial** |
| | `lastfm` | sí | restricciones en alta resolución |
| | `thetvdb` | **de pago** | **⚠ suscripción anual** |

**Los dos por defecto son los que no piden clave**, porque pedirle a alguien
que se registre, saque una API key y la pegue en algún lado es justo el paso
que lo hace abandonar la instalación — **el riesgo #1 del proyecto**.

**⚠ Y hay tres avisos de licencia que un contribuidor tiene que respetar.**
TheAudioDB y Fanart.tv dan su acceso gratuito solo a proyectos **no
comerciales**, y **una emisora que vende anuncios es uso comercial**. TheTVDB
pasó a suscripción de pago. Los tres son drivers donde **cada estación pone su
propia clave bajo los términos que a ella le apliquen** — no se mete a un
usuario en un incumplimiento sin que se entere. La atribución de TMDB es
obligación de sus términos y **va visible en el producto, no escondida en un
archivo.**

### Superposiciones

`logo` (el bug del canal, estático) · `clasificados` (crawl de texto comercial)
· `ninguna`.

**Las dos se dibujan en el servidor de cuadros, en Go** (§14.1), antes de
entregarle el flujo al encoder — no como filtros que hay que recargar en
ffmpeg. **Cambiar el texto del crawl, o quitarlo, nunca reinicia el encoder.**
Dibujarlo nosotros cuesta un poco de CPU y nos deja cambiarlo cuando queramos.

---

## El contrato de un driver, en palabras

Seis obligaciones. Valen para cualquier familia.

### 1 · La configuración vive en la base, nunca en un archivo

Todo lo que un driver necesita saber va en la tabla **`driver_config`** de
SQLite (PRD [§15](../../PRD.md#15--el-modelo-de-datos)):

```
driver_config   channel (nulo = global), tipo (salida | alerta | cobro |
                fichas | entrada | avisos), driver, credenciales (cifradas),
                parametros
```

**Cero archivos de configuración** — es el principio 5 del §4, y existe para
que la configuración y los datos no se desincronicen. **Las credenciales no se
guardan en claro**: las claves de las pasarelas, de los servicios de fichas y
del canal de avisos van al almacén del sistema —DPAPI en Windows, el llavero en
Linux— (§19).

### 2 · La persona nunca ve la palabra "driver"

*(PRD [§13](../../PRD.md#13--las-pantallas), principio 3 del §4.)*

La pregunta del asistente es **"¿a dónde va tu señal, y puedes verla de
vuelta?"** — nunca *"elige un driver de salida"*. El usuario tampoco ve
*códec*, *GOP*, *LKFS* ni *transport stream*. Un driver que necesita un dato
tiene que poder pedirlo en palabras que entienda alguien que sabe instalar
software y nada más: no `pcr_period`, sino *"lo que tu multiplexor espera"*.

**Y todo error se explica en cristiano.** No *"no audio stream detected"* sino
**"Este video no tiene sonido"**, y qué hacer.

### 3 · `ninguna` nunca miente

Es **la regla del §10**, y la que más se rompe sin querer.

Si un driver no puede saber algo, **lo dice en la cara en vez de fingir
certeza.** No hay estado "probablemente bien". Los tres casos que ya están
escritos y sirven de modelo:

- **La salida UDP no sabe si alguien la está escuchando.** Mandar un flujo a
  una dirección que nadie lee no produce ningún error. Es un punto ciego
  estructural, dicho de frente, y por eso la verificación se hace contra el
  retorno de aire.
- **Sin retorno de aire, `signal-compare` corre en modo `degradado`** y no
  promete detectar nada.
- **`ninguno` es una opción válida en casi todas las familias**, y "todavía
  no" es una respuesta que **se muestra, no se esconde**.

### 4 · Prueba de 10 segundos al elegir

**"Probar, no asumir"** es el principio 2 del §4: *la pregunta "¿ves las barras
de color?" vale más que cincuenta campos correctos.*

Todo driver trae una prueba corta que se corre **en el momento en que se
elige**, y que termina en una pregunta que cualquiera puede contestar o en un
resultado medido. La prueba de barras del asistente —*"¿se ve y se oye en el
televisor?"*— es **el paso más importante del producto** (§13): convierte una
tarde de frustración en un sí o un no.

Para un driver nuevo, la prueba tiene que responder algo comprobable: el equipo
contestó, el contacto cerró, el pago de prueba se registró, la señal apareció.
**No basta con que la conexión no diera error.**

### 5 · Reconexión con espera progresiva: 1 → 60 segundos

**Un envío de 24/7 se cae siempre** (§7). Todo driver que mantenga una conexión
reconecta solo, con espera progresiva de **1, 2, 4… hasta un tope de 60
segundos** (§9 paso 5; auditoría C12).

Y mientras reconecta, **el aire no se detiene**: si lo que falta es una fuente
en vivo, entra el relleno, suena la alarma, el bloque **sigue reservado**, y
cuando la señal vuelve el aire regresa al vivo en el siguiente borde de clip de
relleno —máximo un minuto—, con fundido cruzado. **Nadie tiene que ir a apretar
nada.**

### 6 · Nada se consulta en el momento de salir al aire

*(§10.)* Lo que un driver de fichas descarga se guarda junto al archivo, **una
sola vez**, en el ingest. **La cadena de aire funciona con el internet caído**
(§19). Un driver que necesite la red durante la emisión está mal diseñado, con
la única excepción de los que *son* la red: la salida a internet y la entrada
en vivo.

---

## Qué información pedirle al fabricante

Antes de escribir nada, esto es lo que hace falta saber. **Casi todo está en el
manual del equipo o en la página de soporte; lo que no, se pregunta.**

**Cómo se llega al equipo** — y esta es la pregunta que ordena todas las
demás, porque **el driver se elige por cómo se llega, no por marca**:

- ¿Red (Ethernet, Wi-Fi)? ¿IP fija o DHCP? ¿Qué puertos?
- ¿Serial (RS-232, RS-485, USB)? ¿Qué velocidad, bits, paridad?
- ¿Contactos secos (relés, GPIO)? ¿Cuántos, normalmente abiertos o cerrados?
- ¿Varios a la vez? **Si hay dos caminos, se usan los dos y se cruzan.**

**Qué habla**

- ¿SNMP? Pide **el MIB** y la versión (v1, v2c, v3), y las comunidades.
- ¿HTTP? ¿Hay API documentada, o solo una página web que hay que leer? ¿Cómo
  se autentica?
- ¿Syslog? ¿Qué mensajes manda, con qué formato exacto?
- ¿Un protocolo propio? Pide **la especificación**, no un SDK — un SDK en C no
  sirve *(ADR [0002](../adr/0002-no-cgo.md))*.

**Qué se puede leer y qué se puede escribir.** Los drivers de transmisor son
**solo lectura**, a propósito. Si el equipo permite escribir, hay que saber
qué y con qué consecuencia.

**Qué pasa cuando algo falla**

- ¿Cómo se ve una desconexión desde afuera? ¿Da error o se queda callado?
- ¿Cuánto tarda en volver? ¿Aguanta reconexiones seguidas?
- ¿Hay algún estado en el que el equipo mienta —dice "conectado" y no pasa
  nada por ahí? **Ese es el caso que hay que documentar en el driver**, porque
  es el que hace que un reporte finja certeza.

**Los límites** — cuántas peticiones por minuto aguanta, si hay tope diario, si
la licencia del fabricante permite lo que vamos a hacer.

**Y para un método de cobro, además:** en qué países opera, si hay pagos
recurrentes, cómo son los avisos (webhooks) y **qué campo sirve de clave de
idempotencia** — porque un aviso repetido no puede cobrar dos veces.

---

## Cómo se prueba sin el equipo en la mano

**Es la situación normal, no la excepción.** El proyecto nunca va a tener en la
mano la mayoría del hardware que soporta, y eso está asumido en el diseño.

**1 · `signal-compare`, la respuesta de fondo.** Para todo lo que interrumpe o
modifica la señal, no hace falta hablar con el equipo: **se compara la señal
transmitida contra la que el plan decía.** Funciona con hardware que nadie del
proyecto va a tocar nunca, incluido el EWBS de Sudamérica. Si tu driver detecta
algo que también se puede ver en el aire, **`signal-compare` es tu prueba
cruzada.**

**2 · Simuladores.** Un equipo que habla un protocolo se puede fingir: un
servidor SNMP con el MIB del fabricante, un puerto serial virtual que contesta
lo que contestaría el equipo, un endpoint HTTP que devuelve la misma página, un
GPIO simulado. **Lo que se prueba así es el driver, no el equipo** — y hay que
decirlo en el PR con esas palabras.

**3 · Grabaciones.** Una captura de lo que el equipo manda —una sesión serial,
un volcado de syslog, un transport stream, un webhook real con los datos
tachados— es la mejor prueba que existe sin el equipo: es **material real**, se
versiona con el código y sirve para siempre. Si consigues una, adjúntala.

**4 · La salida a archivo.** El driver `archivo` existe para el modo sombra y
las pruebas (§10). Todo lo que produce señal se puede medir contra un archivo
antes de tocar nada: es lo que hace la Fase 0 con `internal/ts`, que lee el
transport stream paquete a paquete —continuidad, PCR, tasa, marcas de tiempo—
sin TSDuck y sin creerle a nadie.

**5 · Y cuando el equipo sí está.** Antes de que dependa de él algo real:
**prueba de 48-72 horas de la señalización de cortes contra el encoder real**,
verificada **en la señal transmitida** y no en lo que creemos haber mandado
(§16). Para el motor, **30 días** contra salida a archivo antes de tocar una
antena.

---

## El flujo: issue → rama → PR

### 1 · Un issue con la plantilla `driver.yml`

**Empieza siempre por aquí, y hoy más que nunca**, porque las interfaces de Go
todavía no existen y tu propuesta es lo que va a darles forma.

Lo que el issue tiene que traer:

- **La familia** — salida, entrada en vivo, señalización de cortes, alertas,
  retorno de aire, transmisor, respaldo, canal de avisos, cobro, o fichas.
- **El equipo o el servicio**, con marca y modelo, y **por dónde se llega**.
- **Qué habla** y dónde está la especificación. Enlace, no adjunto, si es
  público.
- **Cómo se va a probar** — equipo en la mano, simulador, grabación, o
  `signal-compare`. **Dilo aunque la respuesta sea "no tengo el equipo".**
- **Qué pasa cuando falla**, y si el equipo tiene algún estado en el que
  miente.
- **Si hay un usuario esperándolo.** Un driver con una estación detrás pesa
  más que uno hipotético.

> **La plantilla existe:**
> [`.github/ISSUE_TEMPLATE/driver.yml`](../../.github/ISSUE_TEMPLATE/driver.yml).
> Pide marca y modelo, familia, protocolo, documentación del fabricante, si
> puedes probarlo contra el equipo de verdad, y dónde está en uso.

### 2 · Una rama

Una rama por driver. **Un driver, un PR** — un PR que trae dos familias es un
PR que no se puede revisar.

### 3 · Un PR con prueba

El PR tiene que traer, además del código:

- **La prueba de 10 segundos** del punto 4 del contrato, y qué contesta.
- **Cómo se probó**, con esas palabras: *"contra el equipo"*, *"contra un
  simulador"*, *"contra una grabación"*, *"no se ha probado contra hardware
  real"*. **Lo último es una respuesta aceptable; fingir lo primero no.**
- **Qué reporta cuando no puede saber algo** — la regla de que `ninguna` nunca
  miente, aplicada a tu driver.
- **La configuración que pide**, en las palabras que va a ver la persona.
- **`Signed-off-by`** — las contribuciones entran por DCO, como en el kernel de
  Linux *(ADR [0005](../adr/0005-agpl-with-dco.md))*. Detalle en
  [`CONTRIBUTING.md`](../../CONTRIBUTING.md).

**Y el vocabulario del PR es el de [`CONTEXT.md`](../../CONTEXT.md).** Output
Driver, Alert Driver y Return Feed son vocabulario interno: **nunca aparecen
delante de un usuario.**

---

## Para seguir

| Documento | Qué trae |
|---|---|
| [`PRD.md` §10](../../PRD.md#10--los-drivers) | Las familias completas, con el porqué de cada una |
| [`docs/ARQUITECTURA.md`](../ARQUITECTURA.md) | Cómo encaja un driver en el resto del sistema |
| [`CONTEXT.md`](../../CONTEXT.md) | El glosario. Las palabras que usa el código |
| [`CONTRIBUTING.md`](../../CONTRIBUTING.md) | DCO, política de IA, convenciones |
| [`docs/profiles/README.md`](../profiles/README.md) | Lo mismo, para el perfil de cumplimiento de un país |
| [`docs/hardware/MATRIZ.md`](../hardware/MATRIZ.md) | En qué equipo se ha probado, y con qué resultado |
| [`docs/adr/`](../adr/) | Las decisiones grandes y su porqué |
