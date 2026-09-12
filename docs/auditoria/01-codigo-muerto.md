# Auditoría de código muerto — 11 sept 2026

Alcance: lo que entró hoy y ayer de siete tandas/agentes distintos (T1, T2,
T3, y los paquetes `sage`, `same`, `hdhomerun`, `scte104`, `pmcp`). Solo
lectura: no se tocó código. `go build ./...` y `go test ./... -count=1`
verdes; `go vet ./...` sin avisos; `staticcheck` (con `-checks U1000`) no
encontró código muerto dentro de los paquetes. Lo que sigue es lo que
**staticcheck no puede ver**: paquetes enteros que nadie importa, tablas que
nadie toca, y pantallas que nunca llaman a una ruta que sí existe.

## 1 · Los cinco paquetes nuevos y `pmcp`

| Paquete | Importado fuera de sí mismo/sus pruebas | (A)/(B)/(C) | Evidencia | Qué hacer |
|---|---|---|---|---|
| `internal/drivers/alerta/sage` | No (0 resultados) | **(B)** | `grep -rn "drivers/alerta/sage"` solo devuelve archivos del propio paquete. El doc del paquete lo dice él mismo: "quien decide es el motor (tanda T8 de docs/f2/PLAN-F2.md)" (`sage.go:15`) | No borrar. Lo cablea **T8** (`docs/f2/PLAN-F2.md:278-293`, nuevo `endec-serial.go`/`endec-rele.go` que usarán este paquete). |
| `internal/drivers/alerta/same` | No (0 resultados) | **(B)** | Igual método; doc propio: "esta es la única manera de saberlo" pero no dice quién la llama todavía (`same.go:1-14`) | No borrar. **T8** (mismo paquete `alerta`, mismo objetivo F2-81 a F2-85). |
| `internal/drivers/captura/hdhomerun` | No (0 resultados) | **(B)** | Doc propio: "driver de retorno de aire (`capture_input` tipo `stream`, ADR 0009)" (`hdhomerun.go:1-4`); `capture_input` no tiene una sola fila leída ni escrita en todo el repo (ver §2) | No borrar. **T8** (`docs/f2/PLAN-F2.md:285`: "toca `internal/drivers/captura/` (nuevo: `receptor-tv.go`, `tarjeta.go`)" — este paquete es lo que esos archivos van a usar). |
| `internal/drivers/senal/scte104` | No (0 resultados) | **(B)**, sin tanda asignada en el plan | El propio archivo lo dice: "Cablearlo al plan y a la venta de cortes es otra tanda" (`cliente.go:14`). `docs/f2/PLAN-F2.md` no menciona SCTE-104 en ninguna tanda (T1-T10): se escribió después del plan, por decisión de `docs/drivers/CATALOGO.md` | No borrar. Falta que Saul le asigne una tanda (candidatas: T2, por ser también "salidas hacia el multiplexor", o una tanda nueva). La tabla `break_marker` que este paquete referencia en un comentario (`mensajes.go:463,639`) tampoco la toca nadie (ver §2): apunta al mismo hueco. |
| `internal/resolver/pmcp.go` | **Sí** — `internal/app/resolve.go:351`, `internal/api/plan.go:658`, `internal/api/server.go:118-119`, `internal/app/avisos.go:302` | Cableado (no es caso B) | Se llama desde `App.updateGuide` (resolve.go), se sirve en `GET /guia.pmcp` y `/api/v1/guia.pmcp`, y se empuja por HTTP si hay destino puesto | Ninguna acción de cableado — **pero ver el hallazgo (C) del §3 y el §4**: el destino HTTP no tiene dónde ponerse, y el validador nunca corre. |

**Conclusión del §1:** de las cinco piezas nuevas, cuatro son bibliotecas
aisladas *a propósito* — cada una lo dice en su propio comentario de
paquete — y compilan y prueban solas porque ese es el diseño (T8 las
cablea). No hay "vestigio" (A) entre ellas. `pmcp.go` es distinto: sí está
cableado al motor y a la API, pero le falta la mitad del cableado que su
propio código promete (§3, §4).

## 2 · Esquema SQLite: tablas sin una sola sentencia

Comprobado con `grep -rn "FROM <tabla>\|INTO <tabla>\|UPDATE <tabla>"` sobre
todo `internal/`, a mano, tabla por tabla.

| Tabla | SELECT/INSERT/UPDATE encontrados | (A)/(B)/(C) | Evidencia | Qué hacer |
|---|---|---|---|---|
| `capture_input` | 0 | **(B)** | Solo aparece en comentarios (`internal/app/asistente.go:76`, doc de `hdhomerun.go:1`) | **T8** la llena (`docs/f2/PLAN-F2.md:285`: "nuevo: `CaptureInput`... nuevos repos"). |
| `alert_event` | 0 | **(B)** | Cero coincidencias en todo el repo | **T8**, con la retención de 24 meses de F2-85. |
| `overlay` | 0 | **(B)** | Cero coincidencias | Documentado ya en `CONTINUAR.md`: "ponerlo en la señal es F2 (**T7**)". |
| `manual_hold` | 0 | **(B)** | Cero coincidencias | **Cerrado el 12 sept 2026** (T5): `internal/store/manual.go`, `internal/app/manual.go`, `internal/api/manual.go`, panel en `AlAire.tsx`. |
| `air_recording` | 0 | **(B)** | Cero coincidencias; se menciona en `docs/ACEPTACION.md:1230` como lo que leerá el driver `signal-compare` | **T8** (mismo `signal-compare.go` de la tanda). |
| `corte` | 0 INSERT/UPDATE (solo se **lee** su id como columna de `plan_item.corte_id`, `internal/store/plan.go:28-48`) | **(B)**, sin tanda clara | Nadie inserta una fila en `corte`; solo existe la columna que la referenciaría | Ligado al mismo hueco de `scte104`/`break_marker`: falta decidir quién escribe un corte real. |
| `break_marker` | 0 | **(B)**, sin tanda clara | Cero sentencias SQL; solo dos comentarios en `scte104/mensajes.go:463,639` que la nombran como "el puente" | Misma nota que `corte` y `scte104`: falta asignar tanda. |
| `advertiser` / `insertion_order` / `classified` / `classified_airing` / `portal_link` / `payment` / `spot_airing` | 0 salvo `advertiser` (1 archivo, lectura del menú "por hacer") | **(B)**, ya documentado | El propio `schema.sql:250` lo dice: "tablas desde F1 para que el esquema no cambie; se usan en F4" | Nada que hacer ahora; es F4 (`CONTINUAR.md` ya lo tiene). |

**Nota de método:** no encontré ninguna tabla que fuera pura sobra sin
destino (caso A). Todo lo que está vacío tiene, o un comentario propio, o
una tanda de `docs/f2/PLAN-F2.md`, o una nota de `CONTINUAR.md` que explica
para qué se hizo. Las dos excepciones son `corte` y `break_marker`: existen
para que `scte104` tenga dónde escribir, pero ninguna tanda del plan las
menciona por nombre — es el mismo hueco de "SCTE-104 sin tanda asignada" del
§1, no un hueco nuevo.

## 3 · Claves de `settings`

Comparado `internal/app/app.go` (los `Key*`) contra `internal/api/estado.go`
(qué lee/reacciona al guardar) y `web/src/pantallas/Ajustes.tsx` (qué
pantalla ofrece).

| Clave | El servidor la escribe/lee | La pantalla la ofrece | (A)/(B)/(C) | Evidencia |
|---|---|---|---|---|
| `guia_pmcp_destino_http` (`KeyGuidePMCPHTTP`) | Sí — se lee en `internal/app/avisos.go:303` (`pushGuidePMCP`) y `internal/api/estado.go:326` reacciona a su cambio | **No.** Cero coincidencias de `guia_pmcp` o `GuidePMCPHTTP` en todo `web/src` | **(C) — roto de verdad** | La tarjeta "GUÍA" de Ajustes (`Ajustes.tsx:410-419`) solo tiene el campo para `guia_destino_http` (XMLTV). Su hermana PMCP, con exactamente el mismo propósito y el mismo código de servidor, no tiene campo. Hoy la única forma de poner esa dirección es con una llamada a la API a mano. |
| `salida.destino` / `salida.retorno_de_aire` / `salida.nota` (`KeyOutputTarget`, `KeyAirReturn`, `KeyOutputNote`) | Sí — paso 4 del asistente (`internal/api/instalacion.go:101-103,330-332`) | Sí, en el asistente (no en Ajustes) | Cableado, pero **duplica** con la tabla `output` de T2 — ver §6 | Ver hallazgo de duplicación abajo. |
| Todas las demás (`silencio_umbral_s`, `negro_umbral_s`, `subtitulos_estado`, `avisos_*`, `clave_tmdb`, `guia_destino_http`, `fichas_en_linea`, `idioma_audio_preferido`, `pais`…) | Sí | Sí, cada una tiene su campo en Ajustes | Cableado | Comparación campo por campo entre `app.go` y `Ajustes.tsx` (`grep -oE "ajustes\.[a-zA-Z_]+"`). |

## 4 · Rutas de API sin pantalla

Comparado `internal/api/server.go` (44 rutas) contra `web/src/lib/api.ts` y
`web/src/pantallas/*.tsx`.

| Ruta | Quién la implementó | La usa algo del frontend | (A)/(B)/(C) | Evidencia |
|---|---|---|---|---|
| `GET/POST/PUT/DELETE /api/v1/salidas` | T2 (`internal/api/salidas.go`, tabla `output`) | **No.** `api.ts` no tiene ningún método `salidas*`; la única mención de "salidas" en pantallas es `AlAire.tsx:237`, que solo **lee** `estado.salidas` del feed de estado — no llama a esta ruta | **(C) — roto de verdad, ya documentado por el propio equipo** | Coincide exactamente con lo que dice `CONTINUAR.md`: "T2 ya construyó toda la API... pero su pantalla está asignada a **T9**". No es un hallazgo nuevo, pero queda confirmado con evidencia de código: la ruta existe y funciona (`internal/api/salidas.go`), y nadie del lado del navegador la llama todavía. |
| `GET /api/v1/relleno` | F1 (`internal/api/biblioteca.go:516`) | **No.** `api.ts` no tiene wrapper; solo aparece mockeada en `web/src/demo/servidor.ts:1396` para el modo demo | **(B)**, sin tanda asignada | El mock de demo existe pero nada de producción lo consume: parece una pantalla que se planeó (mostrar la biblioteca de relleno aparte) y no se construyó. No está en `ACEPTACION.md` como criterio pendiente explícito. |
| `GET /api/v1/auditoria`, `GET /api/v1/auditoria/verificar` | F1 (compliance, cadena de hash) | **No.** Cero coincidencias en `web/src` | **(B)** | `docs/API.md:238` y `docs/AUDITORIA_2026-09-04.md` lo describen como bitácora exportable/verificable, sin pantalla asignada todavía. No es parte de lo que entró hoy; se anota para no perderlo. |

No encontré funciones de `api.ts` que ninguna pantalla use: el archivo
expone un solo objeto `api` con ~25 métodos y cada uno tiene al menos un
llamador en `web/src/pantallas/` (verificado a mano, uno por uno).

## 5 · `pmcp.go`: el validador que nunca corre

Esto es el hallazgo más concreto de la auditoría, y es nuevo (no está en
`CONTINUAR.md`).

- `internal/resolver/pmcp.go:227` define `ValidatePMCP`, con el mismo
  propósito que `ValidateXMLTVPorGravedad`: reemplazar el XSD normativo que
  ya no se puede descargar (comentario en `pmcp.go:23-29`).
- `internal/app/resolve.go:308-330` (la guía XMLTV) sí corre su validador
  **antes de publicar** — "la puerta de F1-28" — y si hay algo grave, no
  publica y deja la guía vieja puesta.
- `internal/app/resolve.go:347-360` (la guía PMCP, líneas nuevas de hoy)
  **no llama a `ValidatePMCP` en ningún momento**: solo mira el error de
  construcción de `resolver.PMCP(...)`. Una guía PMCP mal armada pero sin
  error de Go (namespace faltante, PmcpMessage sin id/dateTime — exactamente
  lo que `ValidatePMCP` sabe cazar, según `pmcp_test.go:122-158`) se
  publicaría igual en `/guia.pmcp`.
- `ValidatePMCP` solo se llama desde pruebas (`pmcp_test.go`,
  `internal/api/api_test.go:509`, esta última como aserción de la prueba,
  no como parte del servidor).

**(C) — roto de verdad.** Quien escribió `pmcp.go` construyó el validador
copiando el patrón de XMLTV punto por punto (el propio comentario de
`ValidatePMCP` dice "F1-28 es la misma idea para XMLTV") pero no conectó la
puerta en `resolve.go`. Es exactamente el perfil que pide el encargo:
alguien creyó que quedaba conectado y no quedó.

## 6 · Duplicados

| Qué | Dónde | Veredicto | Evidencia |
|---|---|---|---|
| `encoder_colgado`/`encoder_reiniciado` y `timeout_manual`/`manual_por_timeout` | `internal/app/cuarentena.go:83-115` | **No es un defecto — verificado.** T1 sí alineó los literales; los dos nombres viejos siguen en el mapa de texto **solo como alias de lectura**, con un comentario explícito: "Los dos nombres que se usaron antes de que el catálogo existiera... nadie escribe ya con ellos" | `grep -rn '"encoder_colgado"\|"timeout_manual"'` no encuentra ninguna escritura, solo esta lectura de compatibilidad. El informe de T1 es correcto: los literales de F1 (`cuarentena`, `subida_rechazada`, etc., que nunca cambiaron de nombre) tampoco quedaron duplicados. |
| Representación de "la salida" | Asistente (`salida.destino`/`salida.retorno_de_aire`/`salida.nota`, solo texto libre) vs. tabla `output` de T2 (IP, puerto, PIDs, TSID, multicast/TTL, códec) | **(C) — dos sistemas que no se hablan** | El paso 4 del asistente (`internal/api/instalacion.go:101-103,330-332`) guarda el *tipo* de destino como texto; nada en el repo crea una fila de `output` a partir de esa respuesta (`grep` de `Outputs\.` / `CrearSalida` en `instalacion.go` y `asistente.go`: cero). Son dos formas de "decir cuál es la salida" que hoy no se cruzan. Coincide con el hueco #1 de `CONTINUAR.md` ("dónde escribir IPs, puertos y direcciones. El más urgente"), pero el ángulo nuevo aquí es que además de faltar la pantalla, **el dato que el asistente ya pidió no se traslada** cuando la pantalla exista. |
| Analizador de TS | `internal/ts.Analyze` | **No hay duplicado** | Lo usan tanto `internal/f0/analyze.go` como `internal/drivers/captura/hdhomerun/medir.go`, y la prueba de punta a punta de T2 (`internal/app/salidas_test.go`) también lo reutiliza. Un solo analizador, tres consumidores. |

## Prioridad — arreglar ya

1. **Conectar `ValidatePMCP` en `internal/app/resolve.go`** antes de
   publicar `/guia.pmcp`, igual que XMLTV. Sin esto, una guía PSIP mal
   armada sale al aire del generador sin que nadie se entere (§5).
2. **Añadir el campo que falta en Ajustes → Guía** para
   `guia_pmcp_destino_http`, o quitarle a `pushGuidePMCP` la posibilidad de
   mandarla si de verdad no se va a exponer (§3). Hoy es una función
   completa sin una manija.
3. **Decidir la tanda de `scte104`/`corte`/`break_marker`**: es el único de
   los cinco paquetes nuevos sin dueño en `docs/f2/PLAN-F2.md` (§1, §2).

## Anotar, no urgente

- Los cuatro paquetes de drivers (`sage`, `same`, `hdhomerun` y las tablas
  `capture_input`/`alert_event`/`air_recording`) esperan a **T8**; ya está
  escrito en su propio código y en `docs/f2/PLAN-F2.md`. No tocar.
- `overlay` espera a **T7**; `manual_hold` se cerró el 12 sept 2026. Ya documentado
  en `CONTINUAR.md`.
- `/api/v1/salidas` sin pantalla: ya documentado como el hueco #1 de
  `CONTINUAR.md` ("adelantar T9"); esta auditoría lo confirma con evidencia
  de código y no encontró nada adicional que agregar.
- `GET /api/v1/relleno` y `GET/…/auditoria*` sin consumidor en el
  navegador: no son parte de lo integrado hoy; quedan anotados por si se
  pierden de vista.
- Las tablas de publicidad (`advertiser` y las otras seis) están vacías a
  propósito, para F4; el propio `schema.sql` lo dice.

## Lo que NO encontré (y se buscó)

- Ningún **vestigio puro** (caso A: sobra y nadie lo va a usar) entre lo que
  entró hoy. Las cuatro bibliotecas de drivers, aunque hoy no las llama
  nadie, tienen dueño de tanda declarado.
- Ningún literal de incidente de F1 que quedara con dos nombres activos
  (verificado el reclamo de T1 punto por punto).
- Ninguna función exportada de `api.ts` sin pantalla que la use.
- Ningún `TODO`/`FIXME` real en el código Go de hoy (los `grep` de
  "pendiente"/"por hacer" devuelven solo comentarios en español que
  describen el estado normal del dato, no marcadores de trabajo a medias).
