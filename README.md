> 🇺🇸 [Read this in English](README.en.md)

# Antena787

**Software libre para lanzar y operar canales de televisión y de radio.**

Antena787 decide qué sale al aire, lo emite, vende y prueba la publicidad
alrededor, y publica la guía electrónica. Sirve igual a una organización sin
fines de lucro con un canal por internet que a una televisora comunitaria con
transmisor licenciado. Es un solo ejecutable, se opera desde el navegador, y
su única dependencia externa es ffmpeg, que va empaquetado adentro.

Autor: Saul A. González Alonso · Licencia [AGPL-3.0](LICENSE) ·
Contribuciones por [DCO](CONTRIBUTING.md) · Despliegue de referencia:
Caribbean Advantage TV, Puerto Rico.

---

## Estado: Fase 0 en curso. Hoy no sirve para emitir nada.

Que quede dicho antes que nada: **esto no es un producto que se pueda
instalar.** No hay instalador, no hay interfaz, no hay base de datos, no hay
canal, y `cmd/antena` —el ejecutable del producto— todavía no existe.

Lo que sí existe es la **Fase 0**: un experimento con criterio de pase o
fallo que responde la pregunta técnica más cara del proyecto — si el diseño
del motor (un servidor de cuadros en Go entre decodificadores por clip y un
encoder persistente) produce salida continua y limpia durante horas en
hardware modesto. Se corre, se mide con herramientas y se reporta. Si falla,
el diseño del motor se replantea antes de escribir la primera línea de la
Fase 1.

Ver [`f0/README.md`](f0/README.md) y el PRD §22.1.

**Cuánto falta, con los ojos abiertos** *(todo estimado, sobre un supuesto de
10 horas de trabajo a la semana)*: modo sombra —el sistema propone y un
humano compara— alrededor de un año; canal al aire alrededor de tres años y
medio. La aritmética completa está en el PRD §22.2. Este proyecto no promete
fechas: publica las suyas.

---

## Para quién

No hay un usuario, hay un rango, y el sistema sirve a los dos extremos sin
castigar a ninguno.

| | Extremo pequeño | Extremo grande |
|---|---|---|
| Quién | ONG, universidad, televisora comunitaria | Grupo con varias señales |
| Canales | 1 | 2 a 20 |
| Gente | **una persona** | equipo con roles |
| Conocimiento | sabe instalar software y nada más | tiene ingeniero |
| Presupuesto | cercano a cero | limitado pero real |
| Qué corre | un canal de TV, o una radio que también sale online o por TV | TV y radio, varias de cada una |
| Equipo | el que ya tiene, de la marca que sea | ídem |

**Ninguna marca ni modelo se asume**, y ningún país se asume: el perfil de
cumplimiento se escoge, nunca se impone.

Se sirve a ambos con **revelación progresiva**: el sistema arranca en su
forma más simple y cada capacidad avanzada aparece solo cuando alguien la
pide. Quien tiene un canal nunca ve multi-canal, ni roles, ni publicidad
hasta que registra su primer anunciante.

---

## Qué va a hacer

Programa, emite, rellena huecos, publica guía, recibe señal en vivo, inserta
y contabiliza publicidad, cobra y recibe el material del anunciante, y prueba
lo que salió al aire. **Para televisión y para radio** — una emisora de radio
es un canal de televisión sin video, y las reglas, el plan, el as-run, los
cortes y las fuentes en vivo son idénticos.

Y todo es gratis, sin recortes: sin versión recortada, sin pantallas de
recordatorio, sin funciones que caducan, sin marca de agua, sin tope de
canales.

### Qué NO hace, y por qué

- **No construye el equipo de alertas de emergencia.** Es hardware
  certificado aguas abajo; Antena787 se integra con él.
- **No construye códecs.** Eso es ffmpeg. *(ADR [0003](docs/adr/0003-one-bundled-binary.md))*
- **No forkea CasparCG ni ningún motor ajeno.** *(ADR [0001](docs/adr/0001-own-playout-engine-in-go.md))*
- **No tiene salida por tarjeta SDI ni NDI.** Esos SDK obligan a enlazar C.
  *(ADR [0002](docs/adr/0002-no-cgo.md))*
- **No se integra con Plex ni Jellyfin.**
- **No gestiona derechos de contenido.** Sin territorios, sin vías de
  distribución, sin conteo de corridas.
- **No lleva IA adentro.** Cero modelos, cero claves, cero costo impuesto.
  *(ADR [0007](docs/adr/0007-ai-only-through-mcp.md))*

---

## La arquitectura, en un párrafo

Todo es **Go**, escogido por compilación cruzada trivial —Windows, Linux y
ARM desde una sola máquina—, por binario único sin runtime y porque se lee
sin haberlo escrito; no por rendimiento, que vive en ffmpeg y no en nuestro
código. El producto es **un solo proceso**: servidor web, resolver y motor
como goroutines, cada una atrapando su propio pánico y relanzándose sola, con
el supervisor del sistema operativo debajo. La interfaz se escribe en
TypeScript y React, se compila con Node en tiempo de compilación y se embebe
en el binario con `go:embed`; en la máquina de la estación no corre Node. La
base es SQLite en modo WAL, con `modernc.org/sqlite` —SQLite traducido a Go—
y **nunca con CGo**. La única dependencia externa es **ffmpeg** (con
ffprobe), empaquetado como binario estático de versión fija y llamado como
proceso aparte. El motor es propio: cada clip lo decodifica un ffmpeg a video
crudo y audio PCM, un **servidor de cuadros en Go** recibe esos cuadros,
aplica el conformado, mantiene el pre-roll del siguiente clip y entrega un
solo flujo continuo a un **encoder persistente** por su stdin. Esa es
exactamente la decisión que la Fase 0 valida o tumba.

El porqué de cada una de estas decisiones está en [`docs/adr/`](docs/adr/).

---

## Empezar hoy: compilar y correr la Fase 0

Hace falta **Go 1.26 o más nuevo**, y **ffmpeg con ffprobe** en el PATH,
junto al ejecutable, o en la variable `ANTENA_FFMPEG`.

```sh
go build -o bin/f0 ./cmd/f0

bin/f0 media                 # fabrica los archivos de prueba en f0/media (≈1 min)
bin/f0 run -hours 8          # corre el motor; escribe f0/out/{catv.ts,web.ts,events.jsonl,stats.csv}
bin/f0 analyze               # mide y escribe f0/out/REPORTE.md
```

`bin/f0 all -hours 8` hace las tres seguidas. Con `-udp udp://IP:PUERTO` la
salida MPEG-2 va además al multiplexor de verdad; con `-one` corre una sola
salida, para medir el CPU con una y con dos.

En Windows: `go build -o f0.exe .\cmd\f0`, y lo mismo con `f0.exe`.

Para probar rápido que todo compila y corre, seis minutos en vez de ocho horas:

```sh
make f0        # media + run de 0.1 h + analyze
```

La corrida completa de 8 horas ocupa unos **48 GB** de disco. Qué se mide, y
qué la Fase 0 deliberadamente no prueba, está en [`f0/README.md`](f0/README.md).

Para montar el entorno de desarrollo completo —ffmpeg por sistema operativo,
compilación cruzada, estructura de paquetes— ver
[`docs/DESARROLLO.md`](docs/DESARROLLO.md).

---

## Estructura del repositorio

```
PRD.md            qué hace el sistema y en qué orden se construye
CONTEXT.md        el vocabulario del dominio — la única fuente de los términos
README.md         esto · README.en.md en inglés
CONTRIBUTING.md   DCO, política de IA, convenciones, cómo aportar
CODE_OF_CONDUCT.md
SECURITY.md       cómo reportar una vulnerabilidad en privado
LICENSE           AGPL-3.0, texto completo
Makefile          build · vet · test · f0 · windows · linux · arm64 · clean

cmd/f0/           el ejecutable del experimento de la Fase 0
internal/engine/  el motor: decodificador, servidor de cuadros, encoder
internal/f0/      fabricación de los archivos de prueba y el analizador
internal/ts/      lectura de transport stream, paquete a paquete
f0/               README del experimento; media/ y out/ se generan, no se versionan

docs/adr/         las decisiones de arquitectura y su porqué
docs/ACEPTACION.md  los criterios de aceptación
docs/DESARROLLO.md  cómo montar el entorno y compilar
docs/ROADMAP.md     las fases y en cuál vamos
docs/historial/     versiones anteriores del PRD, para rastrear qué cambió

diseno/           mockups de las pantallas (HTML generado; no es código del producto)
```

Y lo que todavía no existe pero está decidido: `cmd/antena/`,
`internal/resolver`, `internal/ingest`, `internal/drivers/`,
`internal/store`, `web/` (PRD §14.1).

---

## Documentación

| Archivo | Qué trae |
|---|---|
| [`PRD.md`](PRD.md) | El documento entero: qué es, para quién, cómo funciona paso a paso, el modelo de datos, las fases. Empieza por §1 y §22. |
| [`CONTEXT.md`](CONTEXT.md) | El glosario. El código, la interfaz y la documentación usan estas palabras y ninguna otra. |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Cómo se contribuye: DCO, política de IA, convenciones, drivers y perfiles de país. |
| [`docs/adr/`](docs/adr/) | Las decisiones grandes y su porqué. Una decisión, un sitio. |
| [`docs/ACEPTACION.md`](docs/ACEPTACION.md) | Los criterios de aceptación. |
| [`docs/ROADMAP.md`](docs/ROADMAP.md) | Las fases, en orden, y dónde estamos. |
| [`docs/ARQUITECTURA.md`](docs/ARQUITECTURA.md) | Cómo está hecho por dentro, para quien va a tocar código. |
| [`COMPLIANCE.md`](COMPLIANCE.md) | Qué hace y qué **no** hace por tu cumplimiento legal. |
| [`COMPRAR.md`](COMPRAR.md) | Guía de compra de equipo, con precios reales. |
| [`SECURITY.md`](SECURITY.md) | Cómo reportar una vulnerabilidad. |

Documentación en español e inglés desde el día uno: el mercado natural es
LATAM y casi todo el software de broadcast existe solo en inglés.

---

## El modelo: el software es gratis, lo que se paga es tener a quién llamar

Todo está en la versión libre, sin recortes: programación, playout, fuentes
en vivo, publicidad con señalización y evidencia de emisión, crawl de
clasificados, portal del anunciante, grabación y diferido, cumplimiento,
servidor MCP, y sin límite de canales.

| | Precio |
|---|---|
| **Soporte inicial** — acompañamiento hasta estar al aire | **$1,200** |
| **Soporte continuo** — línea disponible, atención a fallos | **$100 / mes** |

Y aparte, a quien lo pida: drivers a medida para equipo poco común, perfiles
de cumplimiento de un país nuevo, migración desde otro sistema,
adiestramiento.

**Por qué esto vale lo que cuesta.** Una estación que sale en negro a las 3
AM tiene un problema esa misma noche, y a esa hora no hay a quién llamar. No
se paga por instalar un programa — se paga por que haya alguien despierto del
otro lado. Y encaja con la AGPL sin fricción: la licencia obliga a liberar el
código, no el trabajo ni la disponibilidad.

**La consecuencia se acepta de frente:** cualquiera puede bajarlo, instalarlo
solo y no pagar nunca. Eso no es una fuga — es lo que sostiene la comunidad.
Quien tiene tiempo y ganas lo hace solo; quien tiene una estación que atender
prefiere tener a quién llamar. El razonamiento completo, con las dos
alternativas que se descartaron, está en el ADR
[0006](docs/adr/0006-support-not-features.md).

---

## Despliegue de referencia

**Caribbean Advantage TV, Puerto Rico.** Un canal comunitario que hoy opera
al aire con un Google Sheet hecho a mano, un humano, VLC, un servidor de
streaming, otro VLC, el equipo de alertas y el transmisor — sobre un Windows
10 que ya tenía, sin comprar hardware. Es donde se prueba primero, no el
molde: cada equipo que aparece ahí es un ejemplo de una familia que tiene
driver.

Aceptar ser el despliegue de referencia no puede significar apagar VLC un
lunes. Por eso el sistema entra en modo sombra —propone y se compara— mucho
antes de tocar el aire.

---

## Autoría

Antena787 lo diseña y dirige **Saul A. González Alonso**. El código y la
documentación se escriben con ayuda de **Claude (Anthropic)**, con revisión
humana — el motor, el conformado y el watchdog se leen línea por línea, y no
por costumbre: es lo que sostiene el copyleft (ver
[CONTRIBUTING.md](CONTRIBUTING.md) y el ADR
[0005](docs/adr/0005-agpl-with-dco.md)).

Copyright (C) 2026 Saul A. González Alonso. Detalle en
[AUTHORS.md](AUTHORS.md).

---

## Licencia

**AGPL-3.0.** El texto completo está en [`LICENSE`](LICENSE). Cualquiera
puede correrlo gratis, para siempre; quien construya algo comercial encima
tiene que liberar su propio código bajo los mismos términos. La cláusula
Affero es deliberada: cierra el hueco del servicio alojado, que es la manera
obvia de comercializar esto sin devolver nada. El razonamiento está en el ADR
[0005](docs/adr/0005-agpl-with-dco.md).

**Sobre las patentes de códecs:** el software es libre, pero las patentes de
H.264 y HEVC son asunto aparte que ninguna licencia de software resuelve, y
distribuir un ffmpeg estático con esos encoders las toca. Se dice con
honestidad, se ofrece AV1 y VP9 —libres de regalías— para la salida por
internet, y vale una consulta legal antes del primer release público.
