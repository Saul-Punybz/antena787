# Antena787 desde cero — 11 de septiembre de 2026

Encargo de Saul: volver a mirar el proyecto entero, decir cuál es la meta, si
el camino lleva a ella, y qué sobra. Solo lectura: no se tocó una línea de
código.

Método: `CONTINUAR.md`, las seis auditorías `01`–`06`, el `PRD.md` entero,
`ACEPTACION.md`, los diez ADR, `PLAN-F2.md`, `VLC-PARIDAD.md`, `COMPRAR.md`,
`CATALOGO.md`, `PARA-ROLANDO.md`, el de WhatsApp, y el código de `internal/` y
`web/src/`. Las auditorías 01–06 se usan como insumo, no se repiten — y en dos
casos se corrigen (§1.3).

**Una corrección de arranque.** El encargo habla de «estas dos semanas». El
repositorio tiene **cuatro días**: 14 commits el 8 de septiembre, 38 el 9, 10
el 10 y 18 el 11. Ochenta commits, 31,563 líneas de Go de producción, 21,503
de prueba, 10,075 de TypeScript y 16,779 de documentos, en cuatro días. El
juicio de abajo se hace contra ese número.

---

## 1 · La meta, y si el camino lleva a ella

### 1.1 La meta, en una frase

**Que la PC de la torre de CAtv saque el canal al aire sin VLC ni MistServer,
24 horas al día, y que cuando Rolando la toque o algo se rompa, el aire no se
caiga.**

Todo lo demás —el repositorio público, los perfiles de otros países, la
publicidad, el MCP— es consecuencia posible de eso, no parte de eso. El PRD lo
dice a su manera en §17 («aceptar no puede significar apagar VLC un lunes») y
Saul lo fijó el 11 de septiembre. La meta no es un producto: es una entrega a
una persona.

### 1.2 Lo que acerca

- **El motor y la salida al multiplexor existen y están medidos.** T1
  (`internal/app/motor.go`, `internal/engine/frameserver.go`) y T2
  (`internal/drivers/salida/udpts.go`) con una prueba de punta a punta que
  **abre un socket UDP y lee el TS**: desvío de tasa 0.00 %, PCR 30.5 ms, CC 0
  (`internal/app/salidas_test.go`). Es la columna vertebral de la meta, y está.
- **El ingest es lo que evita el negro.** 3,997 líneas en `internal/ingest`:
  medición real, cuarentena por audio ausente, desfase A/V > 4 s,
  normalización colgada, ficha desde el nombre del archivo. Es el dolor diario
  de Rolando —103 de 343 espacios vacíos en su hoja (`PARA-ROLANDO.md:31`)—
  resuelto.
- **La guía se valida antes de publicarse** (`internal/app/resolve.go:308-330`).
  El bug de `sports1.channel` de su hoja no puede volver.
- **Evidencia en Windows, por primera vez.** La F0 corta pasa entera en el
  runner de Windows (issue #14). Es la plataforma de Rolando y es el primer
  dato real que hay de ella. `internal/despierto` salió del mismo tipo de
  hallazgo: el Mac durmió 19 minutos con el canal encendido.
- **Los diez ADR se respetan al pie de la letra**, verificado en la auditoría
  03 §3, incluido el 0002: `CGO_ENABLED=0` compila limpio para darwin, linux y
  windows. Esa decisión es la que hace que «un solo binario en la PC de la
  torre» sea cierto y no un deseo.

### 1.3 Lo que se desvía

**Lo primero, y no es un matiz: nadie puede poner el canal al aire.**
`internal/api/estado.go:159` dice `nuevo.Mode = "sombra"` en cada llamada a
`PUT /canal`, con un comentario de F1 sin actualizar («en F1 no hay motor: el
canal se queda en sombra dígase lo que se diga»). El asistente no toca `Mode`.
Ninguna pantalla lo cambia. Las únicas líneas del repo que escriben
`ModoAire` están en pruebas. `internal/app/motor.go:90` espera ese modo para
arrancar. Resultado: **T1, T2 y T3 —el motor, las salidas y el detector— no
se pueden alcanzar por ningún camino de producto.** Lo encontró la auditoría
06 §1; queda confirmado leyendo las tres líneas.

Eso no es un defecto. Es la prueba de que después de construir el motor nadie
abrió el navegador e intentó usar el producto como lo usaría Rolando.

**No hay dónde escribir una dirección IP.** T2 construyó
`GET/POST/PUT/DELETE /api/v1/salidas` completo —IP, puerto, multicast, TTL,
PIDs, programa, tsid, PCR, bitrate— y su pantalla quedó asignada a T9, la
última tanda (`docs/f2/PLAN-F2.md`). El paso 4 del asistente pregunta el
*tipo* de destino y nunca la dirección. El producto, hoy, no se puede apuntar
a la TP1000 de Rolando.

**9,173 líneas que nada importa.** Cuatro paquetes huérfanos, verificados con
grep del import path: `senal/scte104` (2,030 + 1,495 de prueba), `alerta/same`
(1,217 + 946), `alerta/sage` (1,204 + 1,001), `captura/hdhomerun` (679 + 601).
Ninguno se importa desde `app`, `api` ni `cmd`. Y **13 de las 28 tablas del
esquema** no tienen una sola sentencia SQL en todo el repo.

**Lo único que Rolando dijo que le falta está al final del plan.** Confirmó el
9 de septiembre que MistServer ya no le abre la ventana del video y que hoy
**no tiene forma de ver su propia salida** (`docs/VLC-PARIDAD.md:22,70-72`).
Ese monitor es F2-117, en T9a: la mejor razón entre valor y tamaño del
proyecto, y la última de la fila.

**Media pantalla de Ajustes no puede funcionar.** `Ajustes.tsx` lee 44 claves
`ajustes.*`; unas 25 no existen en ningún archivo `.go` (grep verificado: cero
resultados para `respaldo_ultimo`, `tailscale`, `aceleracion_tarjeta`,
`dias_al_aire`, `energia_plan`, `antivirus_exclusiones`). Cuatro tarjetas de
«salud de la máquina» quedan en rojo para siempre y la de respaldo pinta «hace
NaN días» (auditoría 02 §3). Es interfaz de F2.5 construida antes de F2.5.

**El importador de la hoja de CAtv está roto en su último paso.**
`internal/api/importar.go:27-28` manda `regla_que_vence`/`regla_que_releva`;
`web/src/pantallas/Reglas.tsx:558` lee `r.regla`/`r.releva_a`. Cada clic en
«confirmar relevos» manda `undefined` y el servidor contesta 400. Sigue roto
hoy.

**Y las auditorías heredaron el mismo problema que medían.** Dos de las seis
reportan como roto lo que ya estaba arreglado en el árbol que auditaron:

- Las auditorías 02 §4 y 06 §1 dicen que «Escoger yo» no tiene `onClick`.
  `git show ad055e7:web/src/pantallas/ParrillaSemana.tsx` —el commit de las
  propias auditorías— lo tiene con `onClick` en la línea 628.
- La auditoría 02 §1 y §5 dicen que `GET /plan` devuelve un objeto contra un
  `FilaDelPlan[]` esperado y que `for...of` lanza `TypeError`. El commit
  `8f24424`, **anterior** al de las auditorías, ya dejó `api.ts:158` aceptando
  las dos formas.
- La auditoría 06 §1 afirma que `agente/parrilla-atajos` no escribió una línea.
  Cierto de la rama, falso del producto: el trabajo entró por `f7009c6`, con
  `ParrillaDia.tsx` (316 líneas) y `BibliotecaAlLado.tsx` (249).

Seis agentes auditando en paralelo, y dos juzgaron un árbol viejo. No invalida
los cuatro hallazgos grandes —la línea de sombra, la carrera de datos,
`RelevoPropuesto`, el validador PMCP sin conectar—, pero dice que el mecanismo
de agentes en paralelo falla igual auditando que construyendo.

### 1.4 Veredicto

El camino lleva a la meta en su eje —entra, se conforma, sale por `udp-ts`— y
se desvía en todo lo que rodea al eje. Lo construido de más está en los
extremos del PRD (protocolos sin equipo delante, cumplimiento, publicidad) y
lo que falta está en el medio, en las dos piezas más baratas: una pantalla
para escribir una IP y un interruptor para salir de sombra. **Cuatro días de
motor detrás de una línea de código de F1 que nadie volvió a mirar.**

---

## 2 · Qué sobra

El PRD tiene 2,128 líneas, **once fases** (F0, F1, F2, F2.5, F3, F4, F4b, F4c,
F5, F5b, F6) y 206 criterios reales en `ACEPTACION.md` (9 + 80 + 117; el
Resumen de la línea 1404 dice 184 y se contradice a sí mismo, auditoría 03
§1). Para un cliente que es una persona.

Dos datos que ordenan todo lo que sigue:

- **De F3 a F6 no hay un solo criterio de aceptación escrito.** Cero para F3,
  F4, F4b, F4c, F5, F5b y F6 (`docs/ACEPTACION.md:1576-1578` lo dice de F4 con
  nombre y apellido). Los 206 criterios son de F0, F1 y F2. Esas siete fases
  son prosa, no compromiso: recortarlas no cuesta trabajo tirado, cuesta borrar
  párrafos.
- **El PRD se escribió sabiendo que lo hace una persona sola** (`PRD.md:1534`,
  `PRD.md:1985-1987`: «unas 10 horas a la semana… al lado de un trabajo») y el
  alcance se decidió igual como si hubiera un equipo. El §9+§10+§15 —cómo
  funciona el motor por dentro— es el 45 % del documento; el negocio entero
  (§21) son 75 líneas: soporte a $1,200 y $100 al mes.

**Corrección a la premisa del encargo:** no hay «cinco países» en el PRD. Hay
tres regiones de transmisión (`PRD.md:1060-1065`) y dos perfiles
internacionales nombrados —`eu-ebu` e `isdb-latam`, ambos F5— además de
`us-fcc` e `internet`, los dos activos al lanzar. El alcance internacional es
más chico de lo que se cuenta, pero sigue siendo F5 entera y sin un criterio.

### 2.1 No debería construirse

| Qué | Por qué |
|---|---|
| **F4c · rotación musical** | Un subsistema entero —categorías, relojes por hora, reglas de separación— para radio musical. La radio de CAtv es WRBM, hablada y de programas, que el PRD §6 ya dice que funciona sin esto. Sacarlo del PRD, no posponerlo. |
| **F5b · escala** (multi-canal, roles, Postgres, API REST completa) | El PRD §11 dice «para el usuario de un canal nada de esto existe». Entonces no es una fase, es una nota. |
| **F6 · MCP** | Su propia definición (`docs/adr/0007`) dice que si nunca se construyera el producto seguiría entero. El ADR es bueno y gratis; la fase es una promesa que no hace falta. Borrar la fase, conservar el ADR. |
| **ATSC 3.0** (`PRD.md` §12, ~15 líneas) | El propio texto dice que Class A está exento. Rolando es Class A. Una línea, no una sección. |
| **T-SCTE / F2-119** (cablear SCTE-104) | Ya hay 3,525 líneas escritas. Cablearlas es multiplicar el error, no recuperarlo. Ver §2.3. |

### 2.2 Debería posponerse sin fecha

| Qué | Nota |
|---|---|
| **F4 entero** — portal, pagos, clasificados, archivo político, reporte de ingresos | Ya decidido el 11 de septiembre. Falta la consecuencia: quitar «Anuncios» del menú y **corregir `PARA-ROLANDO.md`**, que hoy vende exactamente eso (§2 punto 5: «518,400 segundos al mes… hoy no hay dónde anotar un cliente»; §3 promete la pantalla «Anuncios»). El documento que Rolando tiene en la mano contradice la prioridad. |
| **F5 · internacional** (`eu-ebu`, `isdb-latam`, subtítulos DVB/ARIB, DVB-SI) | Y con él la métrica «3 países en 12 meses» de §23. Cero criterios escritos, así que posponerla es gratis. PMCP se queda: ya está construido y cableado en `/guia.pmcp`, es chico, y la TP1000 puede necesitar PSIP. |
| **F4b · radio** | El PRD §17 dice «para CAtv no es después». La lista de prioridades de Saul del 11 de septiembre no la menciona. Preguntarle a Rolando; hasta entonces sin fecha. |
| **F3 · público** (abrir el repo, instaladores, docs bilingües) | Ya hay ~1,100 líneas escritas (`README.en.md`, `COMPRAR.md`, `MATRIZ.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`). Congelar lo escrito y no añadir. Rolando no necesita que el repo sea público. |
| **Cumplimiento salvo lo que corre gratis** | Conservar volumen, identificación de estación y bitácora de alertas: corren solos por debajo. Posponer el archivo político, el archivo público en línea, el conteo de 156 h de E/I y el registro del medidor. `COMPLIANCE.md` (314 líneas) es barato como documento y caro como código. |

### 2.3 Trabajo ya hecho que se perdió

| Pieza | Líneas | Por qué se perdió |
|---|---|---|
| `internal/drivers/senal/scte104` | 3,525 | Sirve a F4, la fase pospuesta. Escrito contra «las dos únicas implementaciones libres que existen» porque SCTE cobra el estándar; el orden de campos de `inject_section` no se pudo cotejar con nadie; el puerto 5167 sale de un datasheet, no de IANA. Y que la TP1000 acepte SCTE-104 está escrito en condicional (`PARA-ROLANDO.md:106`). Es el caso más claro del repo. |
| `internal/drivers/alerta/same` | 2,163 | Un decodificador SAME que lee a −4 dB de SNR sin asignaciones. Es la **capa 2** del ADR 0010; la capa 1 —leer lo que el ENDEC ya dijo, por serial o relés— es `sage`, y es la que importa. Dos formas de saber lo mismo, y el modelo del ENDEC sigue sin confirmar. |
| `internal/drivers/captura/hdhomerun` | 1,280 | Rolando tiene una tarjeta receptora de TV, modelo desconocido (`docs/hardware/MATRIZ.md:63`). El HDHomeRun es una compra de ~$110 que nadie acordó (`COMPRAR.md:160`). Es una suposición con pruebas. |
| Ajustes «salud de la máquina» | ~200 TS | Cinco tarjetas y ~25 claves que el servidor no manda nunca. F2.5 pintada antes de F2.5. |
| `diseno/` | 6 de 13 maquetas | `Anunciantes`, `Portal`, `Musica`, `Diferido`, `EnVivo`, `Manual`: pantallas que no existen, y tres de ellas de fases que se acaban de posponer. |

Suma honesta: entre **11,000 y 13,000 líneas** de Go aparcadas o perdidas
sobre 53,000. Un 20-25 %. Para cuatro días no es un desastre, pero es
prácticamente todo el trabajo de la última noche.

### 2.4 Y el PRD mismo

Sí, hay que recortarlo. Dos razones concretas:

1. **§22.2 ya está desmentido.** Predice F2 en ~3.4 años y F4 en ~5.6 a diez
   horas semanales. En cuatro días hay F1 verificada y tres tandas de F2. Un
   estimado desmentido dentro del PRD hace menos creíble todo lo demás que el
   PRD dice. Borrarlo, no corregirlo: con agentes, el cuello de botella dejó
   de ser construir.
2. **Es un documento de producto para una entrega de un cliente.** Partirlo:
   un `PRD.md` de F0 a F3 más F4 pospuesto (~900 líneas) y un `IDEAS.md`
   congelado con F4b, F4c, F5, F5b, F6, ATSC 3.0, los cinco países y la
   escala.

### 2.5 Costo de mantener lo que se quede

- **Pruebas: 21,503 líneas sobre 31,563 de producción.** Cada `go test ./...`
  es la puerta, y la puerta ya tiene agujeros (auditoría 05): una carrera de
  datos confirmada en `internal/drivers/salida`, `internal/ts` con **0 % de
  cobertura** (300 líneas que sostienen F0-01 a F0-04 y F2-114), el `Encoder`
  real sin una sola prueba, once pruebas que no pueden fallar y diez criterios
  `[AUTO]` sin respaldo. Esa es la factura, y está vencida.
- **Documentos: 16,779 líneas de markdown para una persona.** Mantener
  sincronizados `PRD.md` (2,128), `ACEPTACION.md` (1,580), `PLAN-F2.md` (518)
  y `CONTINUAR.md` (418) a través de fusiones en paralelo cuesta del orden de
  **una tanda por semana de pura reconciliación**, y aun así quedan mal:
  auditoría 03 lista seis documentos en desacuerdo entre sí y el Resumen de
  `ACEPTACION.md` en desacuerdo con su propia aritmética.
- **Las 9,173 líneas huérfanas** no cuestan tiempo de ejecución. Cuestan cada
  `go test`, cada cambio en `internal/model`, y dejan ciego a `staticcheck`,
  que no ve paquetes enteros sin importar.

Lo que **está bien donde está** y no se toca: los diez ADR, las 13 tablas
vacías del esquema (`schema.sql:250` ya explica que existen para que el
esquema no cambie), el ingest, el resolver, el validador de la guía, el
importador de la hoja de CAtv y el asistente. Eso es el producto.

---

## 3 · El mínimo para que Rolando apague VLC un lunes

La lista más corta que existe, en el orden del camino de la señal.

| # | Pieza | Estado | Falta |
|---|---|---|---|
| 0 | Quitar `estado.go:159` y un botón «salir de sombra» | **FALTA** | media tanda |
| 1 | Un archivo entra, se mide, se normaliza, y lo que no sirve no llega al aire | **HECHO** | — |
| 2 | El plan sale de las reglas y la guía se valida antes de publicarse | **HECHO** | — |
| 3 | El motor conforma y sale por `udp-ts` con PIDs, PCR y tsid | **HECHO** | — |
| 4 | Pantalla para escribir IP, puerto, PIDs y programa (la API existe entera) | **FALTA** | 1 tanda |
| 5 | Un monitor de la salida en pantalla — lo único que él dijo que hoy no tiene | **FALTA** | 1 tanda |
| 6 | Dar de alta una fuente en vivo y que el motor la tome (RadioOnce, 15 h/sem) | **FALTA** | 2 tandas |
| 7 | Watchdog de 3 s sobre el encoder y cascada a relleno (F2-11) | **FALTA** | 1 tanda |
| 8 | Tomar y soltar el control a mano sin quedarse pegado (T5) | **FALTA** | 1 tanda |
| 9 | Grabar lo que salió (`air_recording` existe, vacía) | **FALTA** | 1 tanda |
| 10 | La cadena `sout` de Rolando y su firma de F2-113 | **FALTA** | un mensaje, no una tanda |

Lo que **no** está en esta lista, a propósito: el portal del anunciante, los
pagos, SCTE-104, el ENDEC, la telemetría del transmisor, el diferido, el logo
al aire, el as-run comercial, el repositorio público y los perfiles de otros
países.

### 3.1 Cuánto falta de verdad

**Siete tandas y media de construcción.** Al ritmo observado —F1 verificada
más tres tandas de F2 en cuatro días— eso son una o dos semanas de trabajo.

Y no es la respuesta a la pregunta. Construir dejó de ser el cuello de
botella. Lo que separa hoy de un lunes sin VLC es:

- **Nada ha corrido en la PC de Rolando** salvo una F0 corta en el CI de
  Windows. El modo sombra se probó en un Mac.
- **Tres criterios manuales de F1 sin firmar** (F1-32, F1-43, F1-53) y 33
  criterios `[MANUAL]` en total esperando a alguien.
- **La cadena `sout` sigue sin respuesta**, así que los PIDs y el programa de
  T2 son valores de ejemplo.
- **No hay soak de 30 días**, y el PRD lo pone como puerta de F2 (T10, §22).

Honestamente: las siete tandas son una o dos semanas. Llegar al lunes son
**meses**, y la mayor parte de esos meses no es código — es el soak, la
agenda de Rolando y su confianza.

### 3.2 Lo que más riesgo tiene de descarrilarlo

1. **La cadena `sout` y los PIDs de la TP1000.** Si el multiplexor remapea
   PIDs o espera multicast, el módulo de salida de T2 se reescribe en la
   torre. Lo previene un mensaje de WhatsApp que ya está redactado
   (`docs/PARA-ROLANDO-WHATSAPP.md`, bloques 1-2). Es el punto de más
   apalancamiento del proyecto y está desbloqueado.
2. **Que `estado.go:159` sobreviviera dos días sin que nadie lo notara.** El
   riesgo no es la línea: es que el bucle «construir → abrir el navegador →
   usarlo» no está en el proceso. La próxima línea así va a estar en el
   watchdog, y el watchdog es justo lo que sostiene el aire cuando Rolando lo
   rompe.
3. **La carrera de datos confirmada en `internal/drivers/salida` y el
   `Encoder` real sin pruebas.** Es el código que tiene el aire en la mano.
4. **Windows.** Todo lo que funciona, funciona en un Mac. Una F0 corta en CI
   es toda la evidencia que existe de la plataforma de destino.
5. **Que la marca del amplificador no existe.** «ADR» no es un fabricante
   (`docs/drivers/CATALOGO.md:57`). Si la telemetría se escribe contra
   Adrenalin por hipótesis, es otro driver contra una suposición.

---

## 4 · Lo que haría distinto

- **Una tanda no cierra hasta que alguien abre `127.0.0.1:7870` contra el
  binario real y hace la cosa como la haría Rolando.** Diez minutos por
  tanda. Es lo único que habría evitado `estado.go:159`, y es lo que las 21,503
  líneas de prueba no evitaron.
- **Ninguna API entra sin su pantalla en la misma tanda.** T2 construyó
  `/api/v1/salidas` completo y dejó su pantalla para la última tanda. Así se
  construye algo que nadie puede apuntar a nada.
- **Ningún driver antes de tener el equipo delante o su manual confirmado.**
  `scte104`, `same` y `hdhomerun` son 6,968 líneas contra suposiciones. La
  regla: modelo confirmado, o el aparato en la mesa, o no se escribe.
- **Un solo documento dice qué está construido.** Hoy lo dicen `ACEPTACION`,
  `PRD`, `PLAN-F2`, `CONTINUAR` y `VLC-PARIDAD`, y se contradicen. Que sea
  `ACEPTACION.md`, que solo lo actualice la fusión, y que el PRD deje de
  afirmar estado.
- **Sobre los agentes, lo concreto.** El problema no es que no se hablen: es
  que la fusión no tiene dueño.
  - Cada agente rebasa sobre `main` **antes** de escribir su informe, y el
    informe va contra `main`, no contra su worktree. Un agente que audita
    verifica cada afirmación con `git show HEAD:archivo`. Dos de seis no lo
    hicieron y sus hallazgos falsos quedaron en `main`.
  - Seis auditores en paralelo produjeron 1,461 líneas y cuatro hallazgos que
    importan. Una sola pasada de integración por fusión, con el producto
    corriendo, habría encontrado los mismos cuatro más barato.
  - **Ningún agente en paralelo inventa un tipo de incidente, una clave de
    ajustes ni un campo de API.** Son los tres sitios donde rompió todo lo que
    rompió: `guia_pmcp_rechazada` sin catálogo, 25 claves de Ajustes que el
    servidor no manda, `regla_que_vence` contra `releva_a`. Se cierran con un
    `CHECK` en `incidente.tipo`, enums cerrados, y una prueba de contrato que
    compare **tipos** y no presencia de clave (hoy `exige` solo mira que la
    clave exista, auditoría 02 §6).
- **Empezar por lo que el cliente dijo que le falta.** Rolando dijo que no
  tiene forma de ver su propia salida. Es pequeño, es evidente, y está en la
  última tanda.

---

## 5 · Las cinco cosas que decidir esta semana

**1 · Mandar hoy los bloques 1 y 2 de `docs/PARA-ROLANDO-WHATSAPP.md`.**
Recomendación: mandarlos antes de escribir una línea más de salida. La cadena
`sout` exacta, si la TP1000 recibe unicast o multicast y con qué PIDs, y si
MistServer empuja o VLC tira. Por qué: es lo único que bloquea F2-113, es
gratis, y sin ello los PIDs de T2 son valores de ejemplo que se reescriben en
la torre.

**2 · Recortar el PRD a lo de CAtv.** Recomendación: sacar F4b, F4c, F5, F5b,
F6, ATSC 3.0, los cinco países y §22.2 a un `IDEAS.md` congelado; el PRD queda
en F0–F3 con F4 pospuesto. Por qué: 2,128 líneas de promesa para un cliente
que es una persona, y §22.2 ya lo desmiente el propio `git log`.

**3 · No cablear SCTE-104, `same` ni `hdhomerun`.** Recomendación: dejarlos
donde están, con una línea al inicio de cada paquete que diga «escrito sin el
equipo delante, sin tanda asignada», y **no** abrir F2-119. Por qué: 6,968
líneas contra suposiciones; el soporte de SCTE-104 de la TP1000 está en
condicional y el modelo del ENDEC sigue sin confirmar. Cablearlas multiplica
el costo del error en vez de recuperarlo.

**4 · Cambiar la regla de las tandas y escribirla en `CONTINUAR.md`:** ninguna
cierra sin abrir el navegador contra el binario, y ninguna API entra sin su
pantalla. Recomendación: adoptarla hoy, antes de P0. Por qué: `estado.go:159`
dejó tres tandas inalcanzables y `/api/v1/salidas` existe completa sin una
pantalla que la use. Las dos cosas son el mismo agujero de proceso.

**5 · Corregir `PARA-ROLANDO.md` antes de que Rolando lo relea.**
Recomendación: quitar la promesa de la pantalla «Anuncios» del §3 y el punto 5
del §2 —los 518,400 segundos vendibles—, y decirle por qué cambió el orden.
Por qué: el documento que él tiene en la mano vende justo lo que se pospuso el
11 de septiembre. Una expectativa mal puesta cuesta más que una función que
falta.

---

### Nota final

El proyecto no va mal. En cuatro días tiene un motor que emite, un ingest que
protege el aire, un esquema que no va a cambiar y diez decisiones de
arquitectura que el código respeta. Lo que va mal es el borde: se construyó
hacia afuera —protocolos, cumplimiento, países— cuando la meta estaba a dos
pantallas de distancia hacia adentro. Y una línea de código de F1 que nadie
volvió a mirar mantuvo apagado todo lo que se construyó para encenderlo.
