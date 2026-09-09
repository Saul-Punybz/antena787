# Antena787

**Software libre para lanzar y operar canales de televisión y de radio.**

Antena787 decide qué sale al aire, lo emite, vende y prueba la publicidad
alrededor, y publica la guía electrónica. Sirve igual a una organización sin
fines de lucro con un canal por internet que a una televisora comunitaria con
transmisor licenciado.

- Un solo ejecutable. Sin Docker, sin runtime externo, sin archivos de
  configuración que editar a mano.
- Windows, Linux y ARM desde el mismo código.
- Todo se opera desde el navegador.
- **Una sola dependencia externa: ffmpeg, empaquetado adentro.**
- **Cero inteligencia artificial en el producto.**

> ## La promesa
>
> **Que a quien quiere lanzar un canal solo le falte comprar el equipo.**

**Licencia [AGPL-3.0](https://github.com/Saul-Punybz/antena787/blob/main/LICENSE)** ·
contribuciones por [DCO](https://github.com/Saul-Punybz/antena787/blob/main/CONTRIBUTING.md) ·
autor: Saul A. González Alonso · despliegue de referencia: Caribbean Advantage
TV, Puerto Rico.

---

## Estado: Fase 0. Hoy no sirve para emitir nada

Que quede dicho antes que nada: **esto no es un producto que se pueda
instalar.** No hay instalador, no hay interfaz, no hay base de datos, no hay
canal, y `cmd/antena` —el ejecutable del producto— todavía no existe.

Lo que sí existe es la **Fase 0**: un experimento con criterio de pase o fallo
que responde la pregunta técnica más cara del proyecto — si el diseño del motor
produce salida continua y limpia durante horas en hardware modesto.

**Cuánto falta, con los ojos abiertos** *(estimado, sobre un supuesto de 10
horas de trabajo a la semana)*: modo sombra alrededor de un año; canal al aire
alrededor de tres años y medio. **Este proyecto no promete fechas: publica las
suyas.**

Detalle y fecha en **[Estado del proyecto](Estado-del-proyecto)**.

---

## Por dónde empezar

| Si eres… | Empieza por |
|---|---|
| **Alguien que quiere saber si le sirve** | Las **[Preguntas frecuentes](Preguntas-frecuentes)** |
| **Alguien que va a leer el proyecto entero** | [`PRD.md`](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md), §1 y §22 |
| **Alguien que va a escribir código** | [`docs/ARQUITECTURA.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/ARQUITECTURA.md) y luego [`CONTEXT.md`](https://github.com/Saul-Punybz/antena787/blob/main/CONTEXT.md) |
| **Alguien con un equipo que no soporta** | [`docs/drivers/README.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/drivers/README.md) |
| **Alguien de otro país** | [`docs/profiles/README.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/profiles/README.md) |
| **Alguien que se topó con una palabra rara** | El **[Glosario](Glosario)** |

---

## Cómo empezar hoy: compilar y correr la Fase 0

Hace falta **Go 1.26 o más nuevo**, y **ffmpeg con ffprobe** en el PATH, junto
al ejecutable, o en la variable `ANTENA_FFMPEG`.

```sh
go build -o bin/f0 ./cmd/f0

bin/f0 media                 # fabrica los archivos de prueba en f0/media (≈1 min)
bin/f0 run -hours 8          # corre el motor; escribe f0/out/{catv.ts,web.ts,events.jsonl,stats.csv}
bin/f0 analyze               # mide y escribe f0/out/REPORTE.md
```

`bin/f0 all -hours 8` hace las tres seguidas. Con `-udp udp://IP:PUERTO` la
salida MPEG-2 va además al multiplexor de verdad; con `-one` corre una sola
salida, para medir el CPU con una y con dos. En Windows:
`go build -o f0.exe .\cmd\f0`.

**La corrida completa de 8 horas ocupa unos 48 GB de disco.** Qué se mide, y qué
la Fase 0 deliberadamente **no** prueba, está en
[`f0/README.md`](https://github.com/Saul-Punybz/antena787/blob/main/f0/README.md).

Para montar el entorno completo:
[`docs/DESARROLLO.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/DESARROLLO.md).

---

## El mapa de la documentación

Todo vive en el repositorio. Esta wiki es la puerta de entrada, no la fuente.

### Lo que hay que leer primero

| Documento | Qué trae |
|---|---|
| [`README.md`](https://github.com/Saul-Punybz/antena787/blob/main/README.md) | El resumen del proyecto y su estado |
| [`PRD.md`](https://github.com/Saul-Punybz/antena787/blob/main/PRD.md) | El documento entero: qué es, para quién, cómo funciona paso a paso, el modelo de datos, las fases |
| [`CONTEXT.md`](https://github.com/Saul-Punybz/antena787/blob/main/CONTEXT.md) | **El glosario.** El código, la interfaz y la documentación usan estas palabras y ninguna otra |

### Para quien va a escribir código

| Documento | Qué trae |
|---|---|
| [`docs/ARQUITECTURA.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/ARQUITECTURA.md) | Cómo está armado por dentro y por qué, con lo que existe hoy y lo que no |
| [`docs/adr/`](https://github.com/Saul-Punybz/antena787/tree/main/docs/adr) | Las nueve decisiones grandes y su porqué. **Una decisión, un sitio** |
| [`docs/DESARROLLO.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/DESARROLLO.md) | Cómo montar el entorno y compilar |
| [`docs/ACEPTACION.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/ACEPTACION.md) | Los criterios de aceptación |
| [`docs/ROADMAP.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/ROADMAP.md) | Las fases, en orden, y dónde estamos |
| [`f0/README.md`](https://github.com/Saul-Punybz/antena787/blob/main/f0/README.md) | El experimento de la Fase 0: qué mide, cómo, y qué no prueba |
| [`examples/`](https://github.com/Saul-Punybz/antena787/tree/main/examples) | Ejemplos |

### Para quien va a contribuir algo concreto

| Documento | Qué trae |
|---|---|
| [`CONTRIBUTING.md`](https://github.com/Saul-Punybz/antena787/blob/main/CONTRIBUTING.md) | DCO, política de IA, convenciones, cómo aportar |
| [`docs/drivers/README.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/drivers/README.md) | Cómo añadir soporte para un equipo |
| [`docs/profiles/README.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/profiles/README.md) | Cómo contribuir el perfil de cumplimiento de un país |
| [`docs/hardware/MATRIZ.md`](https://github.com/Saul-Punybz/antena787/blob/main/docs/hardware/MATRIZ.md) | En qué máquinas y con qué equipo se ha probado |
| [`SECURITY.md`](https://github.com/Saul-Punybz/antena787/blob/main/SECURITY.md) | Cómo reportar una vulnerabilidad en privado |

### Para quien opera una estación

| Documento | Qué trae |
|---|---|
| [`COMPLIANCE.md`](https://github.com/Saul-Punybz/antena787/blob/main/COMPLIANCE.md) | Qué hace y qué **no** hace por tu cumplimiento legal |
| [`COMPRAR.md`](https://github.com/Saul-Punybz/antena787/blob/main/COMPRAR.md) | Guía de compra de equipo, con precios reales |

---

## Cómo contribuir

**Lo más útil ahora mismo no es código.** El producto todavía no existe: lo que
falta son decisiones bien informadas, y ahí es donde alguien con una estación
en las manos vale más que un pull request.

1. **Lee [`CONTEXT.md`](https://github.com/Saul-Punybz/antena787/blob/main/CONTEXT.md).**
   El vocabulario no se negocia: el código, la interfaz y la documentación usan
   esas palabras y ninguna otra.
2. **Abre un issue antes de escribir nada**, sobre todo para un driver o un
   perfil de país — hay
   [plantillas](https://github.com/Saul-Punybz/antena787/tree/main/.github/ISSUE_TEMPLATE)
   para los dos. Hoy las interfaces de driver en Go **todavía no existen**, así
   que una propuesta bien hecha da más forma al proyecto que una
   implementación.
3. **Un tema, una rama, un PR.**
4. **`Signed-off-by` en cada commit.** Las contribuciones entran por DCO, como
   en el kernel de Linux
   *(ADR [0005](https://github.com/Saul-Punybz/antena787/blob/main/docs/adr/0005-agpl-with-dco.md))*.
5. **Di cómo lo probaste, con esas palabras.** *"No lo he probado contra
   hardware real"* es una respuesta aceptable. Fingir que sí, no.

**Lo que siempre se agradece:** el equipo que tienes y cómo se llega a él ·
las reglas de tu país con sus fuentes primarias · una grabación de lo que tu
equipo manda · el resultado de correr la Fase 0 en tu máquina · y decir dónde
esta documentación miente o no se entiende.

Detalle completo en
[`CONTRIBUTING.md`](https://github.com/Saul-Punybz/antena787/blob/main/CONTRIBUTING.md).

---

## El modelo: el software es gratis, lo que se paga es tener a quién llamar

**Todo está en la versión libre, sin recortes** — programación, playout, fuentes
en vivo, publicidad con señalización y evidencia de emisión, crawl de
clasificados, portal del anunciante, grabación y diferido, cumplimiento,
servidor MCP, **y sin límite de canales**. Sin versión recortada, sin pantallas
de recordatorio, sin funciones que caducan, sin marca de agua.

Lo que se vende es **soporte**: $1,200 de acompañamiento hasta estar al aire,
$100 al mes de línea disponible. **Una estación que sale en negro a las 3 AM
tiene un problema esa misma noche, y a esa hora no hay a quién llamar.** El
razonamiento completo, con las dos alternativas que se descartaron, está en el
ADR [0006](https://github.com/Saul-Punybz/antena787/blob/main/docs/adr/0006-support-not-features.md).

**Y la consecuencia se acepta de frente:** cualquiera puede bajarlo, instalarlo
solo y no pagar nunca. **Eso no es una fuga — es lo que sostiene la comunidad.**
