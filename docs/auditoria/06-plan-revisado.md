# Plan de F2 revisado — auditoría del 11 de septiembre de 2026

Reemplaza en autoridad a `docs/f2/PLAN-F2.md` (escrito el 9 de septiembre,
antes de construir nada de F2). Todo lo de abajo se comprobó contra el
código, no contra las marcas de ningún documento. Build limpio (`go build
./...`, 11 sept), `main` verde.

**Prioridad fijada por Saul el 11 sept, y que manda sobre todo lo demás:**
esto es para Rolando, una persona con un canal chico, no para un mercado.
Lo más cercano a funcional y completo para que él lo use **y lo rompa**.
Manda el camino de la señal —entra, se ve, sale, se graba, se comprueba— y
la resistencia a que él lo rompa sin tumbar el aire. El dashboard del
anunciante y los pagos (F4) se posponen: no están en el camino crítico.

---

## 0 · El mínimo para que Rolando apague VLC

En el orden del camino de la señal. `[HECHO]` = comprobado en el código.
`[FALTA]` = no existe ningún archivo que lo haga.

1. `[HECHO]` Un archivo entra, se conforma (pillarbox, audio, cortes,
   pre-roll) y sale sin reinicios — T1.
2. `[FALTA]` El canal puede salir de modo sombra y emitir de verdad —
   **nadie lo enciende**; es el hallazgo nuevo de esta auditoría (§1).
3. `[API hecha, pantalla falta]` Hay dónde configurar a dónde va la señal
   (IP, puerto, PIDs, multicast/TTL) — T2 construyó `/api/v1/salidas`
   entero; no hay pantalla que la use.
4. `[HECHO]` El motor sale por `udp-ts` con lo que el multiplexor de CAtv
   exige, con varias salidas a la vez y su propio volumen cada una — T2.
5. `[FALTA]` Una fuente en vivo se puede dar de alta (sin pasar por el
   importador de hojas) y el motor la toma a su hora — ni el alta ni el
   motor existen.
6. `[Detección hecha, control falta]` Si hay silencio o negro real se
   detecta y se avisa (T3); falta que suelte el control manual de verdad
   (T5, el gancho ya está puesto y nadie lo llama).
7. `[FALTA]` Rolando puede tomar el control a mano y soltarlo sin que se
   quede pegado — T5 no existe ni un archivo.
8. `[FALTA]` Si el encoder se cuelga o el proceso falla, se repone solo —
   el watchdog de 3 s (F2-11) no existe.
9. `[FALTA]` Rolando puede ver lo que de verdad está saliendo sin abrir
   VLC — el monitor local (F2-117) no existe.
10. `[FALTA]` Lo transmitido se graba sin huecos — T7 (grabación) no
    existe.
11. `[Bibliotecas hechas, cableado falta]` Se puede comprobar contra el
    retorno de aire que lo transmitido es lo pautado — `sage`, `same` y
    `hdhomerun` están escritos, probados y documentados; nadie los llama
    desde `app` (T8).
12. `[FALTA]` Firma de Rolando de que no le falta nada de lo que hacía con
    VLC (F2-113) — depende de todo lo anterior y de su cadena `sout`, que
    sigue sin respuesta.

**De doce piezas: tres completas de punta a punta (1, 4, la mitad de 6),
dos a medias (3 y 11), siete no existen.** Esa es la distancia real a que
Rolando pueda usar esto y romperlo.

---

## 1 · Estado real, tanda por tanda

**T1 (motor y conformado) y T2 (decks, prioridad, `udp-ts`) — hechas de
verdad.** `internal/app/motor.go`, `internal/engine/frameserver.go`,
`internal/drivers/salida/` (`salida.go`, `udpts.go`, `archivo.go`),
`internal/app/salidas.go`. Contrato `engine.ClipSource`/`engine.Sink` y
`salida.Driver` construidos, con pruebas de punta a punta reales (leen el
TS por UDP en la propia prueba). `docs/ACEPTACION.md` marca "Construido en
T1/T2" en F2-02 a 17, 46, 47, 49, 50, 114.

**Hallazgo nuevo — el canal nunca llega a `Mode == "aire"` por ningún
camino de producto.** `internal/app/motor.go:90` espera `ch.Mode ==
ModoAire` para arrancar de verdad. Pero: el asistente
(`internal/api/instalacion.go`, pasos 1 y 6) actualiza `Channel` sin tocar
`Mode`; `PUT /api/v1/canal` (`internal/api/estado.go:159`) **fuerza
`nuevo.Mode = "sombra"` en cada llamada**, comentario de F1 sin actualizar
("dígase lo que se diga"); `AlAire.tsx:39` solo *lee* el modo, no hay botón
que lo cambie en ninguna pantalla. Las únicas líneas que ponen `Mode =
ModoAire` en todo el repo están en tests. Con T1+T2+T3 fusionadas, una
instalación real **no puede emitir nunca** — más urgente que cualquier
pantalla pendiente.

**T3 (detector de silencio/negro) — hecha de verdad.**
`internal/engine/detector.go`, `internal/app/vigilancia.go`. F2-51 a 54, y
la mitad de F2-30 que no necesita el control manual real (`app.
ControlDelAire` existe; nadie lo llama porque T5 no existe).

**Fuera del plan, construido y sin dueño de tanda: cuatro bibliotecas de
protocolo, cero wireadas.** Verificado por `grep`: ninguna referencia desde
`internal/app` o `internal/api` fuera de sus propios tests.

| Biblioteca | Qué hace | Para qué |
|---|---|---|
| `drivers/alerta/sage` | Decodifica —solo lectura— un Sage ENDEC, serial y TCP | T8 |
| `drivers/alerta/same` | Decodifica SAME en el retorno de aire (ADR 0010, capa 2) | T8 |
| `drivers/captura/hdhomerun` | Retorno de aire por HTTP, sintonizadores SiliconDust | T8 |
| `drivers/senal/scte104` | Cliente TCP de cortes hacia el encoder, escrito a mano | sin tanda propia (§2) |

`internal/resolver/pmcp.go` **sí está wireada** (`GET /guia.pmcp` real).
9,449 líneas entre las cuatro sin cablear — T8 ya no tiene que escribir
drivers a ciegas, solo conectar lo que existe.

**`agente/parrilla-atajos` — rama abierta, cero commits.** Su punta
(`816f295`) es idéntica al `merge-base` con `main`: nadie escribió una
línea. Confirmado en `main` tal cual la dejó ese commit: el botón "Escoger
yo" (`ParrillaSemana.tsx:560`) sin `onClick`; el hueco vacío sin handler
propio; tarjetas de Biblioteca no `draggable`; solo `PUT /plan/{id}`, no
existe `POST /plan`.

**Alta de fuentes en vivo — no existe.** Sin ruta `/vivos`. Solo el
importador de hojas crea una `LiveSource`. `/en-vivo` en `App.tsx:75` es
`<PorHacer nombre="En vivo" />`, el mismo componente placeholder que
`/anuncios`.

**Pantalla de conexiones — no existe.** El CRUD (`/api/v1/salidas`) está
completo; ninguna pantalla lo consume. `Asistente.tsx` paso 4 solo pregunta
el *tipo* de destino y cierra con "se configura de verdad más adelante" —
nunca pide dirección, puerto, PIDs ni puerto serial.

**Datos del canal editables — a medias, con una trampa.** `GET/PUT
/api/v1/canal` existen los dos; `Ajustes.tsx:219-225` solo pinta texto, sin
inputs, sin llamar nunca al `PUT`. Y el `PUT` fuerza sombra en cada
llamada (arriba) — cualquiera que construya la pantalla de edición tiene
que arreglar `estado.go:159` primero.

**No se ha tocado en absoluto** (ausencia total de archivo): `internal/
app/manual.go`, `vivo.go`, `watchdog.go`, `grabacion.go`, `diferido.go`,
`asrun.go`; `internal/engine/overlay.go`; `internal/drivers/vivo/`,
`salida/internet.go`, `salida/httpts.go`; `internal/api/manual.go`,
`tablero.go`. T4, T5, T6, T7, T8(app), T9 — ni empezados.
`docs/ACEPTACION.md` lo confirma: F2-18 a 45 y F2-55 a 113 sin ninguna nota
"Construido en…".

**Portal del anunciante** (`portal/entrada` → `acceptFromPortal`) sí existe
en el servidor —valida y mueve a biblioteca—; no existe la pantalla que
suba ahí. Confirmado y, por decisión de Saul, pospuesto (§3).

---

## 2 · Qué del plan ya no tiene sentido

**T8 se hace más chica.** Su riesgo explícito era "construir tres drivers a
ciegas"; `sage`, `same`, `hdhomerun` ya están terminados y probados contra
manuales/documentación real. Le queda: el modelo (`CaptureInput`,
`AlertEvent`), los repos con retención, `signal-compare.go`, y cablear.

**SCTE-104 no tiene tanda.** Ni T7 ni T8 lo nombran en el plan original. La
biblioteca existe y espera consumidor: cortes hacia el inyector del
encoder. No es del camino crítico de Rolando (su inyector, si existe, no
es parte de "conecte, transmita, se grabe" del mínimo); se asigna a una
tanda de baja prioridad (T-SCTE, §3).

**T6 se hace más chica en la parte de reintentos de encoder.** T2 ya dejó
la espera progresiva y el conteo (`salida.EsperaDe`) para `udp-ts`; T6
sigue debiendo entero el watchdog de 3 s (F2-11) y disco/base/reloj.

**Contratos de la §3 del plan original, cambiados por el código:**
- `engine.ClipSource` — igual en espíritu; T1 añadió `engine.Sink`
  (`WriteFrame`/`WriteAudio`) como intermedio, que ninguna tanda nueva debe
  saltarse.
- `salida.Driver` — el código real tiene **tres** métodos, no dos:
  `Abrir`, `Vigilar`, y `Descripcion() string` (la frase en cristiano). T7
  (`internet.go`, `httpts.go`) tiene que implementar los tres.
- El catálogo de incidentes creció más de lo anotado: además de los ocho
  que el plan proponía, el código usa `IncFalloDeClip`, `IncCartel`,
  `IncSilencioDetectado`, `IncNegroDetectado`. Ninguna tanda nueva inventa
  un string suelto.

**La dependencia "T9 espera a todo" ya no es literal.** El plan original
asumía que nadie podía probar nada hasta el final — eso ya es un problema
demostrado (no se puede probar en casa de Rolando). La pantalla de
conexiones se adelanta; el resto de T9 (tablero completo, firma) sigue al
final porque de verdad depende de todo lo anterior.

---

## 3 · El plan nuevo — ordenado por el camino de la señal

Regla de archivos: ninguna tanda que corra en paralelo con otra toca el
mismo archivo; donde coinciden, se marca la dependencia en vez de
paralelizar.

### Bloque A — Encender y conectar (bloquea todo lo demás)

**P0 · Salir de sombra.** *Camino: enciende todo el camino.* Objetivo: que
exista un camino de producto para pasar `channel.Mode` a `aire`, y que
nada lo regrese sin que alguien lo pida. Cierra: propuesto como **F2-118**.
Toca: `internal/api/estado.go` (quitar la línea 159), endpoint nuevo
(`POST /canal/salir-de-sombra`), `AlAire.tsx` (el botón). Depende de: nada.
Tamaño: ~150 Go + ~100 TS. Se paraleliza con todo el Bloque B salvo P4 (§3,
Bloque D — mismo archivo).

**P1 · Pantalla de conexiones.** *Camino: "conecte".* Objetivo: escribir
IP, puerto, multicast/TTL, PIDs, programa, tsid, nombre de puerto serial,
consumiendo la API que T2 ya construyó. Cierra: condición de F2-113 y de
poder probar cualquier tanda contra hardware real. Toca:
`Conexiones.tsx` (nuevo), `App.tsx` (ruta). Depende de: nada para
construirse; de P0 para probarse con motor real. Tamaño: ~150 Go
(ejemplos de puerto serial por sistema, validación) + ~500 TS. Se
paraleliza con P0, P2, P3, P6.

### Bloque B — Entra: vivo

**P2 · Alta de fuentes en vivo.** *Camino: "entra".* Objetivo: crear,
editar, borrar una `LiveSource` sin pasar por el importador. Toca:
`internal/api/vivos.go` (nuevo), sección en `Reglas.tsx`. Depende de:
nada — el modelo ya existe entero. Tamaño: ~300 Go + ~250 TS. Se
paraleliza con P0, P1, P3, P6 — ojo si P3 toca `Reglas.tsx` al mismo
tiempo (coordinar orden de fusión, no tamaño).

**T4 · Motor de fuentes en vivo.** *Camino: "entra" (vivo, de verdad).*
Sin cambios de fondo sobre el plan original (`drivers/vivo/`, `app/
vivo.go`). Cierra F2-18 a 26, 74-76. Depende de: T1, T2 (hechas), P2 (para
poder probarse a mano). Tamaño: ~1,100 Go.

### Bloque C — Resiliencia: que lo pueda romper sin tumbar el aire

**Sube de prioridad por decisión de Saul: deja de ser el final del plan.**

**T5 · Manual: tomar, soltar, un solo tenedor.** *Camino: resiliencia +
"conecte" (el operador no se queda pegado).* Sin cambios sobre el plan
original; el gancho `app.ControlDelAire` ya está puesto por T3. Cierra
F2-27 a 35, 77-79. Depende de: T1, T2 (hechas). Tamaño: ~600 Go + ~300 TS.
Se paraleliza con T4 (archivos distintos).

**T6 · Watchdog y cascada (encoder, disco, base, reloj).** *Camino:
resiliencia.* Más chica que el plan original: ya no inventa la espera
progresiva del encoder (T2 la cubrió para `udp-ts`); sigue debiendo el
watchdog de 3 s (F2-11) entero y el endurecimiento sobre
`mantenimiento.go`/`disco_*.go` (F1, adaptar, no reescribir). Cierra F2-11,
55-58, 67-73, 86-102. Depende de: T1, T2, T3 (hechas). Tamaño: ~1,500 Go.
Se paraleliza con T4, T5.

### Bloque D — Se ve, sale, se graba

**T9a · Monitor local y Al Aire real (subconjunto de T9, adelantado).**
*Camino: "se ve".* Objetivo: que Rolando vea la salida real sin abrir VLC
(F2-117: baja latencia, `http-ts`/HLS, ≤3 s de retraso) y que Al Aire deje
de decir "Aquí se vería tu señal". Toca: `AlAire.tsx`, `internal/api/
tablero.go` (parcial), `internal/drivers/salida/httpts.go` (comparte con
T7a — coordinar, no paralelizar los dos a la vez sobre ese archivo).
Depende de: T1, T2, P0, P1. Tamaño: ~300 Go + ~400 TS.

**T7a · Salidas de internet y `http-ts`.** *Camino: "sale" (también por
internet).* Sin cambios de fondo. Toca: `salida/internet.go`,
`salida/httpts.go` (compartido con T9a, ver arriba). Cierra F2-48 (resto),
F2-115. Depende de: T1, T2. Tamaño: ~700 Go.

**T7b · Grabación.** *Camino: "se graba".* Sin cambios de fondo
(`app/grabacion.go`). Cierra F2-42, 43, 44. Depende de: T1, T2. Tamaño:
~400 Go. Se paraleliza con T7a, T9a (archivos distintos).

**T7c · Diferido y logo al aire.** *Camino: derivado de "se graba", no del
mínimo de Rolando.* Sin cambios de fondo (`app/diferido.go`,
`engine/overlay.go`). Cierra F2-36 a 41, 80. Depende de: T7b (usa
`air_recording`). Tamaño: ~500 Go + ~100 TS. Puede esperar al Bloque F si
hace falta espacio en las 48 horas.

### Bloque E — Se comprueba en el retorno

**T8 · Telemetría y ENDEC.** *Camino: "se comprueba".* Más chica que el
plan original: `sage`, `same`, `hdhomerun` ya están escritos; falta el
modelo (`CaptureInput`, `AlertEvent`), los repos con retención de 24
meses, `signal-compare.go`, y cablear. Cierra F2-81 a 85, 111. Depende de:
T1, T2, T6 (catálogo de incidentes compartido). Tamaño: ~400 Go (bajó de
700).

### Bloque F — Cierre: tablero completo y firma

**T9 · Tablero completo y firma de paridad con VLC.** *Camino: cierra el
camino entero en pantalla.* Reducida: ya no incluye conexiones (P1), alta
de vivo (P2) ni el interruptor de sombra (P0). Le queda: deck/alarmas/
grabación completos en pantalla, escaneo de red (paso 4) y barras reales
(paso 5) del asistente, as-run. Depende de: todo lo anterior fusionado.
Tamaño: ~700 Go + ~600 TS.

**T10 · Soak de 30 días.** Sin cambios; no se paraleliza; espera a T9.

### Sin bloquear el camino, en paralelo con cualquier bloque

- **P3 · Parrilla: atajos, panel de biblioteca.** Cierra F1-78, 79. Toca
  `ParrillaSemana.tsx`, `Reglas.tsx` (coordinar con P2). Sin `POST /plan`:
  Saul ya decidió (11 sept) que la parrilla no acepta contenido directo.
  Tamaño: ~400 TS.
- **P4 · Canal editable + dueño/licenciatario.** Toca `estado.go` — **no
  paralelizable con P0**, fusionar después. Tamaño: ~200 Go + ~200 TS.
- **P6 · Términos, ayuda, demo visible, logo de estación en la interfaz.**
  Sin dependencias. Tamaño: ~100 Go + ~400 TS (sobre todo texto).
- **T-SCTE · SCTE-104 hacia el inyector.** No es del mínimo de Rolando (su
  inyector, si existe, es un dato por confirmar). Propuesto F2-119.
  Depende de T2. Tamaño: ~250 Go.

### Pospuesto por decisión de Saul (11 sept) — no en el camino crítico

- **P5 · Portal del anunciante** (subida de video sobre `portal/entrada`,
  que ya valida en el servidor). Sin fecha; se retoma cuando el camino de
  la señal esté cerrado.
- **F4 completo**: pago/cobro, aprobación antes del primer aire, reporte de
  ingresos, dashboard del anunciante. Pospuesto entero, no olvidado.

---

## 4 · Próximas 48 horas de trabajo, en orden

1. **P0 (salir de sombra).** Sin esto nada de lo demás se prueba con el
   motor real. Es código que nadie sabía que faltaba, y es chico.
2. **P1 (pantalla de conexiones), en paralelo con P0 desde el minuto uno.**
   Es "conecte": la API ya existe, es casi todo interfaz.
3. **P2 (alta de fuentes en vivo), en paralelo con las dos anteriores.**
   Sin ella, T4 no se puede ni probar a mano, y el motor de vivo es parte
   del mínimo.
4. **T5 (manual: tomar y soltar), tan pronto como haya un agente libre.**
   Es resiliencia y es parte de que Rolando pueda tocar el sistema sin que
   se quede pegado — subió de prioridad hoy.
5. **Fusionar P0, P1, P2 en cuanto pasen pruebas** — son independientes.
6. **P4 (canal editable) inmediatamente después de fusionar P0** — mismo
   archivo, no puede ir antes ni en paralelo.
7. Si queda ventana: **T6 (watchdog del encoder)**, la pieza de resiliencia
   más chica y más urgente de T6 (F2-11 solo, sin el resto de disco/base/
   reloj todavía). Es lo que evita que "romperlo" signifique que el aire
   se cuelga de verdad.
8. **P3 (parrilla) y P6 (legal/ayuda/demo/logo) donde quepan** — no
   bloquean nada, pero tampoco compiten por agentes con lo de arriba.

**Lo que se saca deliberadamente de las 48 horas:** P5 (portal del
anunciante) y todo T7c/T8/T9 — ninguno es parte del mínimo para que
Rolando encienda, vea, transmita y grabe. Entran después de que el Bloque
A-C de §3 esté fusionado y probado.

---

## 5 · Riesgos del plan nuevo y decisiones para Saul

1. **P0 enciende el aire de verdad por primera vez.** Probar primero en
   Mac con salida `archivo`, no directo a `udp-ts` contra el multiplexor.
2. **`PUT /api/v1/canal` fuerza sombra en cada guardado hoy** — confirmar
   que quitar esa línea no rompe una prueba de F1 que dependa de eso.
3. **La cadena `sout` de Rolando sigue sin respuesta** (pregunta 2 del
   plan original, 7b de `PARA-ROLANDO.md`). P1 se construye con valores de
   ejemplo configurables; F2-113 no cierra sin ella.
4. **Resiliencia elevada cambia el tamaño de las 48 horas.** T5 y el
   watchdog mínimo de T6 no estaban en el radar de esta semana; decidir si
   se les da un agente dedicado o se reparten entre los mismos que hacen
   P0-P2.
5. **F2-91 a 102 (disco/base/corriente) siguen sin respuesta**: ¿cuentan
   para el cierre de F2 o solo para F2.5? T6 no sabe su tamaño final sin
   esto.
6. **SCTE-104 no tiene tanda ni número de criterio** — decidir si se le da
   F2-119 ahora o se deja esperando un inyector real.
7. **La rama `agente/parrilla-atajos` lleva dos días sin un commit** —
   relanzar con el mismo encargo o reemplazarla directamente con P3.
8. **Postergar P5/F4 es una decisión de producto, no técnica** — el
   código de `portal/entrada` ya existe y no se degrada por esperar;
   confirmar con Saul que no hay un anunciante real esperando ya.
