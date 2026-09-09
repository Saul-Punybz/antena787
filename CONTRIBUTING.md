# Cómo se contribuye a Antena787

Gracias por mirar. Antes de nada, el estado real: **el proyecto está en la
Fase 0 y todavía no emite nada.** Lo que hay es el experimento del motor
(`f0/`) y los documentos que lo gobiernan. Si vienes buscando un playout
funcionando, todavía no es.

Lo que sí sirve hoy: **correr la F0 en tu máquina y reportar lo que midió**
(ver [`f0/README.md`](f0/README.md) y la plantilla de reporte de hardware),
leer el [PRD](PRD.md) y decir dónde se contradice, y discutir un driver o el
perfil de cumplimiento de tu país antes de que exista el código que los
carga.

---

## 1 · El DCO es obligatorio

Toda contribución entra bajo el **Developer Certificate of Origin 1.1**, el
mismo que usa el kernel de Linux. No hay CLA, no se cede copyright a nadie:
firmas que tienes derecho a aportar lo que aportas.

Se firma añadiendo `-s` al commit:

```sh
git commit -s -m "corrige la deriva de audio en el cambio de clip"
```

Eso añade al final del mensaje una línea así, con tu nombre real y tu correo:

```
Signed-off-by: Nombre Apellido <correo@ejemplo.com>
```

**Un commit sin `Signed-off-by` no se puede aceptar**, ni siquiera de una
línea. Si se te olvidó: `git commit --amend -s` para el último, o
`git rebase --signoff origin/main` para varios.

El texto completo del DCO 1.1 está en <https://developercertificate.org/>.
Resumido, al firmar certificas que: (a) lo escribiste tú y tienes derecho a
aportarlo bajo la licencia del proyecto; o (b) lo derivaste de algo con una
licencia compatible, que también puedes aportar; o (c) te lo pasó alguien que
certificó (a) o (b) y no lo modificaste; y (d) entiendes que la contribución
y tu firma son públicas y quedan en el registro para siempre.

El razonamiento —por qué DCO y no CLA, y por qué AGPL— está en el ADR
[0005](docs/adr/0005-agpl-with-dco.md).

---

## 2 · Política de inteligencia artificial

**Se acepta código escrito con ayuda de IA.** Este proyecto se construye así:
lo diseña y dirige Saul A. González Alonso y el código lo escribe mayormente
Claude con revisión humana (ver [AUTHORS.md](AUTHORS.md)). Sería incoherente
prohibírselo a los demás.

Las dos condiciones:

1. **Un humano lo revisó, entendió y responde por él.** Tu firma del DCO dice
   que tienes derecho a aportarlo; la revisión dice que sabes lo que hace.
2. **Se dice en el PR.** Hay una casilla en la plantilla: qué parte fue
   asistida y qué revisaste tú. No es una confesión ni penaliza el PR — es
   información que el revisor necesita para saber dónde mirar más despacio.

### La excepción que no se negocia

**El motor, el conformado y el watchdog —todo lo que vive en
`internal/engine`— se leen línea por línea por un humano. Siempre.** Sin
excepción, sin "es un cambio chiquito", sin "lo probé y funciona".

Son alrededor del 20% del código y el 95% del riesgo. Un motor de playout no
falla con un error 500 — **falla a las 3:14 AM de un martes, en silencio.** Y
los errores de este dominio son justo los que un modelo comete con más
facilidad y que menos se notan al revisar: una deriva de marcas de tiempo que
aparece a las 40 horas, un contador que se desborda, un caso borde de horario
de verano. La interfaz, el editor de reglas y los reportes no requieren ese
nivel; el motor sí. *(PRD §16)*

### Y hay una razón legal, no solo de ingeniería

Una obra sin autoría humana podría **no estar sujeta a derechos de autor**, y
el copyleft necesita un titular al que agarrarse: sin derechos de autor no
hay nada que licenciar bajo AGPL, y la obligación de compartir el código se
queda sin base. **La revisión humana documentada resuelve las dos cosas a la
vez** — la calidad y la titularidad. Por eso se anota en el PR y queda en el
registro. *(ADR [0005](docs/adr/0005-agpl-with-dco.md))*

---

## 3 · Convenciones de código

### Idiomas: cada cosa en el suyo

| Qué | Idioma | Por qué |
|---|---|---|
| Identificadores de Go (paquetes, tipos, funciones, campos, variables) | **inglés** | Go idiomático; el código se lee como Go, no como Spanglish |
| Comentarios | **español** | Explican el porqué a quien mantiene esto |
| Mensajes al usuario, textos de la interfaz, errores | **español** (con su inglés cuando exista traducción) | El usuario es una estación en LATAM |
| Documentación | **español**, y en inglés lo de cara al mundo | El mercado natural es LATAM y casi todo el software de broadcast existe solo en inglés |
| Términos del dominio | los de [`CONTEXT.md`](CONTEXT.md) | Una palabra por concepto, en todas partes |

Así se ve en la práctica, y así está el código de hoy:

```go
// Server es el servidor de cuadros: la pieza en Go entre los decodificadores
// por clip y el encoder persistente (PRD §14.1). Entrega un flujo continuo,
// conformado y a tiempo, pase lo que pase con los archivos.
type Server struct {
	Format  Format
	Preroll int // cuadros de pre-roll por decodificador
}
```

**Los términos del dominio salen de [`CONTEXT.md`](CONTEXT.md) y de ningún
otro sitio.** El glosario dice, para cada concepto, la palabra que se usa y
las que **no** se usan. Es `Plan Item`, no *event* ni *booking*; es `As-run`,
no *playout log*; es `Gap`, no *dead air* (que es otra cosa: un Incidente).
Si te falta una palabra para algo, el PR que la introduce añade la entrada al
glosario en el mismo commit. Si crees que una entrada está mal, cámbiala —
pero cámbiala ahí, no solo en tu código.

Y una regla que viene del principio 3 del PRD: **el usuario nunca ve jerga.**
Ni *driver*, ni *códec*, ni *GOP*, ni *LKFS*, ni *transport stream*. En el
código esas palabras están bien; en un texto que alguien lee en pantalla, no.

### Commits

- **En español**, en imperativo, una línea clara: *"corrige la deriva de
  audio en el cambio de clip"*, no *"fix"* ni *"cambios varios"*.
- **Sin trailers de herramientas.** Nada de `Co-Authored-By:` de un asistente,
  ni enlaces a sesiones, ni firmas generadas. El único trailer que va es
  `Signed-off-by:`, el del DCO.
- El cuerpo, si hace falta, explica **por qué**, no qué: el qué ya está en el
  diff.

### Antes de abrir el PR

```sh
gofmt -l .        # no debe imprimir nada
go vet ./...      # limpio
go test ./...     # verde
```

O `make vet test`. **`gofmt` y `go vet` limpios son requisito, no
recomendación**, y el CI los corre en cada push y cada PR.

### Las cuatro reglas duras

1. **Nada de CGo. Jamás.** En el momento en que una dependencia se enlaza por
   CGo, `GOOS=windows go build` deja de producir un binario de Windows desde
   Linux — y la compilación cruzada trivial es la razón principal por la que
   se escogió Go. Si de verdad hace falta una librería en C, se llama su
   ejecutable como proceso aparte. *(ADR [0002](docs/adr/0002-no-cgo.md))*
2. **Ninguna dependencia nueva sin un ADR.** Hoy el árbol de dependencias es
   deliberadamente diminuto y la única dependencia externa es ffmpeg. Añadir
   una es una decisión de arquitectura, y se escribe como tal antes de
   escribir el `go get`. *(ADR [0003](docs/adr/0003-one-bundled-binary.md))*
3. **Sin builds nocturnas.** Se compila por release, con versión, sumas de
   verificación y binarios firmados. **Un canal al aire no corre código sin
   versionar.**
4. **Nada que pueda tumbar el aire entra sin prueba.** Cada goroutine crítica
   atrapa su propio pánico y se relanza sola; si tu cambio puede hacer que un
   fallo saque el canal del aire, no está terminado. *(ADR
   [0008](docs/adr/0008-never-interrupt-air.md))*

---

## 4 · Proponer un driver para tu equipo

*"¿Sirve con mi equipo?"* es la primera pregunta de todo el que llega, y la
matriz de compatibilidad es lo que decide la adopción. Por eso los drivers
son la contribución más valiosa que existe aquí.

Hay siete familias: **salida**, **entrada**, **alertas**, **cortes**,
**transmisor**, **retorno de aire** y **cobro**. Un driver nuevo es una
implementación de la interfaz de su familia, más su ficha en la matriz.

Cómo hacerlo:

1. **Abre un issue con la plantilla "Driver"** antes de escribir código:
   marca y modelo, familia, protocolo, documentación del fabricante, y si
   puedes probarlo con el equipo delante. Un driver que nadie puede probar
   contra el aparato real se acepta marcado como **no verificado**, y se dice
   en la matriz.
2. Lee [`docs/drivers/`](docs/drivers/) — cómo se estructura uno, qué tiene
   que reportar y cómo se declara degradado cuando el equipo no está.
3. El PR trae el driver, su prueba, y la fila de la matriz de hardware.

**Ninguna marca se asume, y ninguna se privilegia.** El equipo del despliegue
de referencia no es el molde: es un ejemplo de una familia.

---

## 5 · Proponer el perfil de cumplimiento de un país

Un perfil de cumplimiento son las reglas del país donde transmite el canal:
objetivo de volumen, formato de subtítulos, registros que hay que guardar y
por cuánto tiempo. **Se escoge, nunca se asume, y nunca regaña** — deja los
registros listos por si alguien los pide.

1. **Abre un issue con la plantilla "Perfil de país"**: país, norma de
   volumen, subtítulos, alertas de emergencia, registros y su retención, con
   la referencia oficial de cada cosa.
2. Lee [`docs/profiles/`](docs/profiles/).
3. Lo que hace falta de verdad es **la referencia normativa**, no la opinión:
   qué documento lo dice y en qué artículo.

Y lee [`COMPLIANCE.md`](COMPLIANCE.md) antes: el sistema **no** certifica el
cumplimiento de nadie, guarda evidencia. La diferencia importa.

---

## 6 · Cómo se decide algo grande

Las decisiones de arquitectura viven en [`docs/adr/`](docs/adr/), numeradas,
**en inglés** (son el registro público del proyecto), y cada una en un solo
sitio: es lo que impide que la documentación se contradiga consigo misma.

Necesita ADR cualquier cosa que: añada o quite una dependencia, cambie el
contrato entre el motor y ffmpeg, cambie el modelo de datos de forma no
aditiva, cambie el modelo de licencia o de negocio, o meta un proceso o un
servicio nuevo.

El formato es el del [ADR
0009](docs/adr/0009-truth-is-the-transmitted-signal.md), y es corto a
propósito:

```markdown
---
status: accepted
---

# Un título que es la decisión, en una oración afirmativa

Dos o tres párrafos: qué se decide, qué alternativa se descartó y por qué
la descartada era peor — no en abstracto, sino con la consecuencia concreta
que la hundió.

## Consequences

Qué queda obligado, qué queda prohibido y qué deja de hacer falta por haber
decidido esto.
```

Un ADR **no** lleva lista de opciones con puntuación, ni diagramas, ni
promesas. Lleva la decisión y su porqué. Se abre como PR igual que el código,
y se discute ahí.

---

## 7 · Qué NO se acepta

No por gusto: cada una de estas está decidida y escrita, y reabrirla necesita
un ADR que tumbe el anterior, no un PR.

- **Integraciones con Plex, Jellyfin o similares.** Plex es inspiración de
  cómo se *ve* una biblioteca, no de dónde salen sus datos. Las fichas se
  resuelven con etiquetas embebidas, `.nfo` de Kodi, y APIs sin clave.
  *(ADR [0003](docs/adr/0003-one-bundled-binary.md))*
- **IA dentro del producto.** Cero modelos, cero claves, cero costo impuesto a
  una organización que no lo tiene — incluidos los subtítulos automáticos. La
  IA llega solo por un servidor MCP, apagado por defecto, en la última fase, y
  que a propósito **no expone ninguna herramienta capaz de poner algo al
  aire.** *(ADR [0007](docs/adr/0007-ai-only-through-mcp.md))*
- **Gestión de derechos de contenido.** Sin territorios, sin vías de
  distribución, sin conteo de corridas. Las fechas de inicio y fin de una
  regla no son un sistema de derechos: son dos campos de los que salen los
  avisos de vencimiento.
- **Funciones de pago, de cualquier forma.** Ni versión recortada, ni
  pantallas de recordatorio, ni funciones que caducan, ni marca de agua, ni
  tope de canales, ni telemetría. El dinero sale del soporte y de nada más, y
  **ninguna parte del código tiene que saber si alguien pagó.**
  *(ADR [0006](docs/adr/0006-support-not-features.md))*
- **Salida por SDI o NDI.** Sus SDK obligan a enlazar C.
  *(ADR [0002](docs/adr/0002-no-cgo.md))*
- **Forks de motores ajenos ni parches a ffmpeg.** *(ADR
  [0001](docs/adr/0001-own-playout-engine-in-go.md), [0004](docs/adr/0004-scte104-to-the-encoder.md))*

Si crees que una de estas está equivocada, el sitio para discutirlo es un
issue o un ADR — no un PR de 2,000 líneas.

---

## 8 · Conducta

Aplica el [Código de Conducta](CODE_OF_CONDUCT.md). Resumido: gente adulta
tratándose con respeto, y desacuerdos sobre el código, no sobre las personas.

---

# Contributing — English summary

**Status.** The project is in **Phase 0** and does not air anything yet. What
exists is the engine experiment (`f0/`) and the documents that govern it. The
most useful thing you can do today is run F0 on your machine and report what
it measured (see [`f0/README.md`](f0/README.md) and the hardware issue
template).

**DCO is mandatory.** Every contribution comes in under the [Developer
Certificate of Origin 1.1](https://developercertificate.org/). Sign your
commits with `git commit -s`; a commit without a `Signed-off-by` line cannot
be accepted. There is no CLA and no copyright assignment.

**AI policy.** AI-assisted code is welcome — this project is built that way —
on two conditions: a human reviewed it, understood it and stands behind it,
and the PR says so (there is a checkbox). The exception is absolute: **the
engine, the conform and the watchdog (`internal/engine`) are read line by
line by a human, always.** They are ~20% of the code and 95% of the risk, and
a playout engine does not fail with a 500 — it fails silently at 3:14 AM on a
Tuesday. There is also a legal reason: a work without human authorship may
not attract copyright, and copyleft needs a rightsholder to attach to.
Documented human review solves both at once (ADR
[0005](docs/adr/0005-agpl-with-dco.md)).

**Code conventions.** Go identifiers in **English** (idiomatic Go); comments,
user-facing strings and documentation in **Spanish**; domain terms exactly as
defined in [`CONTEXT.md`](CONTEXT.md) and nowhere else. Commit messages in
Spanish, imperative, one clear line, **no tool trailers** — the only trailer
is `Signed-off-by`. `gofmt` and `go vet` must be clean, and `go test ./...`
green, before you open a PR.

**Hard rules.** No CGo, ever (ADR
[0002](docs/adr/0002-no-cgo.md)). No new dependency without an ADR (ADR
[0003](docs/adr/0003-one-bundled-binary.md)). No nightly builds — a channel on
air does not run unversioned code.

**Drivers and country profiles.** Open an issue with the "Driver" or "Perfil
de país" template first; then see [`docs/drivers/`](docs/drivers/) and
[`docs/profiles/`](docs/profiles/). Drivers nobody can test against real
equipment are accepted, marked unverified in the compatibility matrix.

**Big decisions** are made as ADRs in [`docs/adr/`](docs/adr/), in English,
in the short format of ADR
[0009](docs/adr/0009-truth-is-the-transmitted-signal.md): the decision, the
alternative that was rejected and the concrete consequence that sank it, then
`## Consequences`.

**Not accepted:** Plex/Jellyfin integrations, AI inside the product, content
rights management, paid features of any kind, SDI/NDI output, forks of other
engines or patches to ffmpeg. Each is already decided and written down;
reopening one takes an ADR, not a pull request.
