# Auditoría 04 · Dominio y esquema

Auditoría de solo lectura sobre `main`, esquema SQLite versión 6. Todo lo que
sigue se comprobó leyendo el archivo citado; donde no encontré evidencia lo
digo en vez de suponerlo.

## 1. Vocabulario del dominio

### 1.1 El catálogo de incidentes no es el único que escribe

`internal/model/incidentes.go` se declara "catálogo único", pero nueve de los
once tipos de la tanda F1 se escriben como cadena suelta en vez de la
constante, y hay tres tipos que ni siquiera están en el catálogo:

| tipo · quién lo escribe | constante existe | forma usada | evidencia |
|---|---|---|---|
| `cuarentena` | `IncCuarentena` | literal `"cuarentena"` (2x) y `.String()` (1x, otro archivo) | `internal/app/media.go:148,191` vs `internal/app/motor.go:650` |
| `subida_rechazada` | `IncSubidaRechazada` | literal (2x) | `internal/app/media.go:112,122` |
| `normalizacion_fallida` | `IncNormalizacionFallida` | literal | `internal/app/media.go:736` |
| `subtitulos` | `IncSubtitulos` | literal | `internal/app/media.go:764` |
| `disco_bajo` | `IncDiscoBajo` | literal | `internal/app/mantenimiento.go:132` |
| `salto_de_reloj` | `IncSaltoDeReloj` | literal | `internal/app/mantenimiento.go:151` |
| `relleno_por_defecto` | `IncRellenoPorDefecto` | literal | `internal/app/relleno.go:106` |
| `base_restaurada` | `IncBaseRestaurada` | literal | `internal/app/app.go:317` |
| `propuesta_del_asistente` | `IncPropuestaDelAsistente` | literal | `internal/app/asistente.go:304` |
| `vencimiento` | `IncVencimiento` | literal (2x) | `internal/app/resolve.go:264,276` |
| `guia_rechazada` | `IncGuiaRechazada` | literal | `internal/app/resolve.go:314` |
| `encoder_reiniciado`, `solape`, `fallo_de_clip`, `cuarentena` (motor), `cartel` (2x), `enlace_caido`, `manual_por_timeout`, `silencio_detectado`, `negro_detectado` | sí, tanda F2 | **`model.Inc*.String()`** consistente | `internal/app/motor.go:123,456,626,650,714,718`, `internal/app/vigilancia.go:315`, `internal/app/salidas.go:93` |
| `maquina_despierta` | **no existe** | literal, con frase propia en el mapa de textos | `internal/app/despierto.go:150`, mapa en `internal/app/cuarentena.go:96` |
| `guardian_caido` | **no existe** | literal, con frase propia | `internal/app/despierto.go:167`, mapa en `internal/app/cuarentena.go:97` |
| `guia_pmcp_rechazada` | **no existe, y tampoco tiene frase** | literal, nuevo de hoy | `internal/app/resolve.go:352` |
| `apagon`, `vivo_ausente`, `cascada_extendida` | sí están en el catálogo y en el mapa de frases | **nunca se llaman**: no hay ningún `Incident(...)` con estos tres en código no-test | — |
| `encoder_colgado`, `timeout_manual` | no están en el catálogo | nombres viejos, el propio comentario dice "nadie escribe ya con ellos" | `internal/app/cuarentena.go:113-114` |

Es justo el hueco que el propio proyecto ya se apuntó como pendiente:
`docs/f2/PLAN-F2.md:479` — *"F2-63 revisado contra la lista real de tipos de
incidente en producción: (...) sin dos incidentes contando la misma cosa con
nombres distintos"* — sigue sin marcar `[ ]`. El criterio que lo mide,
`docs/ACEPTACION.md:1118` (F2-63), pide que "cada tipo se distinga de los
demás"; con literales sueltos nada impide que alguien escriba `"cuarentena "`
con un espacio o `"Cuarentena"` con mayúscula el día que toque tocar ese
archivo, porque **no hay ningún `CHECK` en `incidente.tipo`** (ver §3) que lo
atrape, y tampoco hay una prueba que compare el mapa de `cuarentena.go` contra
las constantes de `incidentes.go` (busqué, no existe).

`guia_pmcp_rechazada` es el caso más nuevo y más claro: se agregó hoy junto
con la guía PMCP (`internal/resolver/pmcp.go`), no entró al catálogo, y como
tampoco está en `textosDeIncidente`, `TextoDeIncidente` cae al genérico
(`internal/app/cuarentena.go:127`) y lo pinta como *"guia pmcp rechazado"* con
el guion bajo cambiado a espacio — funciona, pero es la prueba de que el
catálogo se puede esquivar sin que nada avise.

### 1.2 El resto del vocabulario, capa por capa

| concepto | Go (`model`) | JSON / API | esquema SQL | `tipos.ts` | ¿coherente? | evidencia |
|---|---|---|---|---|---|---|
| bloque del plan | `PlanItem` | `plan_item` (ruta), campos en español | `plan_item` | `ElementoDelPlan` | sí, mismo concepto, nombre inglés en Go/tabla y español en tipo TS — es la convención documentada | `internal/model/model.go:411`, `internal/store/schema.sql:176`, `web/src/lib/tipos.ts:69` |
| el material en sí | `MediaAsset` | `material` (rutas `/material`) | `media_asset` | `TituloDeBiblioteca`/`AudioDelMaterial` (no hay `MediaAsset` en TS) | sí | `internal/api/biblioteca.go` |
| la ficha (programa/serie) | `Title` | `título`/`biblioteca` | `title` | `TituloDeBiblioteca`, `FichaDeTitulo` | sí | — |
| el relleno | `FillerAsset`, `OriginFiller="relleno"`, `DeckFiller="relleno"` | `origen: "relleno"` | `filler_asset`, `origen IN (...,'relleno')` | `origen: ... \| 'relleno'` | sí | `internal/model/model.go:499` |
| el cartel (slate) | `OriginSlate="cartel"`, `IncCartel="cartel"` | `origen: "cartel"`, `cartel` en `/estado` | `origen IN (...,'cartel')` | `origen: ... \| 'cartel'` | sí | `internal/api/instalacion.go:307` |
| la salida | `Output` | `salidas` | `output` | `Salida` | sí | — |
| la fuente en vivo | `LiveSource` | `live_source_id` | `live_source` | `Regla.live_source_id` (no hay tipo `FuenteEnVivo` propio) | sí, pero TS no tiene un tipo dedicado para la fuente en vivo, solo su id — no es incoherencia, es que la pantalla para darla de alta todavía no existe (lo dice `CONTINUAR.md`, commit `2c208d1`) | — |

## 2. Español/inglés mezclado

- **`PlanState` mezcla los dos idiomas dentro del mismo conjunto**: seis
  valores en inglés (`planned`, `cued`, `aired`, `skipped`, `preempted`,
  `manual_hold`) y uno en español (`fallido` en vez de `failed`) —
  `internal/model/model.go:399-407`, reflejado igual en el `CHECK` de
  `plan_item.estado` (`internal/store/schema.sql:192-193`) y en
  `EstadoPlan` de `web/src/lib/tipos.ts:60-67`. Las tres capas están de
  acuerdo entre sí (mismo conjunto, mismo orden de ideas), así que no es un
  desacuerdo entre capas — es un mismo conjunto que no decidió un idioma.
  Comprobé que la interfaz nunca pinta estos valores en crudo (no hay
  `'planned'`/`'aired'`/etc. fuera de `tipos.ts` y del servidor de demo, ver
  §5), así que hoy no es una fuga de jerga al operador, pero si algún día una
  pantalla nueva imprime `elemento.estado` directo, la mitad va a salir en
  inglés sin que nadie lo haya decidido así.
- `MotivoAlarma`/`AccionAlarma` payloads mezclan claves ya españolas con un
  puñado de tipos de alarma en `tipos.ts:116-124` (`sobrecupo`, `hueco`,
  `vencimiento`, `sin_relleno`, `material`, `emparejar`,
  `subtitulos_sin_decidir`) — todos en español, consistente.
- `TitleKind`, `RuleKind`, `DeckKind` (ver §4) sí están en un solo idioma
  cada uno y coinciden en las tres capas — cito esto como control, no como
  hallazgo.
- Comentarios y nombres de función en el código de drivers de hoy están en
  español incluso en paquetes que hablan con protocolos en inglés (SAME,
  SCTE-104): `Decodificador`, `Cliente`, `Sencillo` — consistente con el
  resto del repo.

## 3. Esquema SQLite

| hallazgo | evidencia | severidad |
|---|---|---|
| `incidente.tipo` no tiene `CHECK`, a diferencia de casi todas las demás columnas-enum del esquema (`channel.tipo`, `media_asset.estado`, `title.tipo`, `live_source.tipo`, `schedule_rule.tipo`, `deck.tipo`, `plan_item.estado`, `plan_item.origen`, `manual_hold.motivo_fin`, `alert_event.tipo`/`origen`, `audit_log.origen`/`aplica_en`, `insertion_order.estado`, `spot_airing.estado`, `classified.estado`, `overlay.tipo`) | `internal/store/schema.sql:364-371` | real: es la puerta por la que entran los literales sueltos de §1.1 |
| `plan_item.corte_id` no tiene `REFERENCES corte(id)`, pero `break_marker.corte_id` sí la tiene para el mismo concepto | `internal/store/schema.sql:191` vs `:290` | real, aunque de bajo impacto práctico (SQLite no exige orden de declaración para FK) |
| `output.estado_conexion`, `driver_config.tipo`, `motivo_codigo` de `media_asset`: documentados con un comentario de valores posibles pero sin `CHECK` | `internal/store/schema.sql:36,53,82` | aseo — hoy los valores en Go coinciden con el comentario (`Conectada`/`Apagada`/`Reintentando`/`sin_probar` en `internal/drivers/salida/salida.go:39-41`), pero nada en la base lo obliga |
| `parseInt64s`/`parseInts` (usados para `marcas_de_corte_ms` y `reloj_de_cortes`) tragan el error de `json.Unmarshal` y devuelven `nil` en silencio ante un JSON corrupto | `internal/store/store.go:393-402,416-424` | real: un archivo de esas dos columnas dañado a mano no avisa, simplemente se comporta como si la lista estuviera vacía |
| `driver_config` no tiene ningún repositorio en `internal/store` que lo lea o escriba (busqué `driver_config` fuera del `schema.sql` y no aparece) | — | informativo: tabla del §14.1 "para que el esquema no cambie", todavía sin cablear |

### Migraciones 1→6 contra `schema.sql` de cero

Sí está cubierto, y a fondo: `internal/store/store_test.go:873`
(`TestBaseNuevaYBaseMigradaQuedanIguales`) crea bases paradas en cada versión
publicada —1 (`schemaSQL` solo), 2, 3, 4 y 5— las deja migrar con
`Open()`, y compara el volcado completo de `sqlite_master` (tablas, índices y
triggers, `internal/store/store_test.go:847-868`) contra una base nueva. Eso
cubre en la práctica los saltos 1→6, 2→6, 3→6, 4→6 y 5→6. Además comprueba
que cada paso deja respaldo (`internal/store/store_test.go:924-928`). No hay
un caso explícito para "una base que ya estaba en 6", pero es innecesario:
no hay migración 7 que la mueva. Lo único a vigilar es que quien agregue la
migración 7 tenga que **añadir el caso `{6, ...}` a esa lista** — hoy nada
se lo recuerda salvo leer el comentario de arriba.

## 4. Estados y enumeraciones

| conjunto | Go | esquema `CHECK` | `tipos.ts` | ¿coinciden los tres? |
|---|---|---|---|---|
| `RuleKind` (tipo de regla) | normal, diferido, bloque_arrendado, vivo | idéntico | idéntico | sí |
| `DeckKind` | manual, comercial, programa, relleno | idéntico | (no hay tipo TS propio; se usa en `Regla`/`FranjaSemana` indirectamente) | sí en las dos capas que existen |
| `TitleKind` | serie, pelicula, promo, id, spot, cortinilla, programa | idéntico (mismo set, otro orden) | `tipo: string` en `TituloDeBiblioteca` — **no está tipado como unión**, es `string` suelto | parcial: TS no fija el conjunto, así que un valor nuevo del lado del servidor no lo marca el compilador |
| `PlanState` | ver §2 | idéntico | idéntico | sí (mismo set; idioma mezclado, no valores distintos) |
| `AssetState` (estado de un `media_asset`) | ingiriendo, listo, cuarentena, fallido | idéntico | **`EstadoMaterial` en TS es un conjunto distinto**: `'listo' \| 'aún no listo para aire' \| 'cuarentena'` | no son el mismo conjunto — pero es a propósito: `internal/api/biblioteca.go:26-28` colapsa `ingiriendo` y `fallido` en la frase `EstadoNoListo` antes de que el JSON salga (`estado_material`), así que el operador nunca ve las cuatro claves de la base, ve tres frases. Documentar esto ayudaría — hoy solo está en el comentario de `biblioteca.go`, no en `tipos.ts` |
| tipos de incidente | catálogo de 22 en `incidentes.go` | sin `CHECK` (§3) | `Incidente.tipo: string` (abierto a propósito) | ver §1.1: en la práctica hay 25 valores circulando (22 del catálogo + `maquina_despierta`, `guardian_caido`, `guia_pmcp_rechazada`) |
| motivo por el que un archivo quedó parado (`motivo_codigo`) | comentario dice `sin_audio`, `duracion_av_no_coincide`, `normalizacion_fallida` (`internal/model/model.go:172`) | sin `CHECK` | mismo trío documentado en `tipos.ts:345-349` | los nombres coinciden entre Go y TS, pero ninguna de las dos capas usa un tipo cerrado (Go: `string`; TS: `string` con JSDoc) — un cuarto código se puede escribir sin que nadie lo note |
| modos del canal | `sombra`, `aire` | idéntico | idéntico (`ModoCanal`) | sí |

## 5. Jerga que llega al operador (F1-56)

La prueba automática que exige el criterio, `TestF1Verif56SinJergaEnLaInterfaz`
(`internal/api/f1verif_jerga_test.go:44`), existe y corre — pero es una lista
**cerrada de cinco términos** (`driver`, `códec`, `GOP`, `LKFS`,
`transport stream`, línea 22), que es exactamente lo que pide el PRD en su
Principio 3 (`PRD.md:137`). El código nuevo de hoy no toca ninguno de esos
cinco directamente en texto de pantalla — no encontré una violación de la
prueba tal como está escrita.

Lo que sí encontré, y que la prueba no puede atrapar por ser una lista
cerrada, es en `internal/drivers/salida/udpts.go` (tocado hoy, commit
`10a63c9`): los mensajes de validación de la salida al multiplexor usan
`PID`, `PCR`, `TTL` y `tsid` en las frases que llegan sin filtrar hasta
`ultimo_error` — y el propio comentario de `internal/app/salidas.go:79-81`
dice que **"el texto es el que ve la persona: la pantalla de Al aire pinta
`ultimo_error` tal cual"**:

```
internal/drivers/salida/udpts.go:184: "el PCR tiene que salir cada %d ms o menos..."
internal/drivers/salida/udpts.go:214: "...no pueden ir en el mismo PID (%d, %d y %d)"
internal/drivers/salida/udpts.go:223: "el PID de %s va del %d al %d..."
internal/drivers/salida/udpts.go:189: "los saltos de red (TTL) van de 1 a 255..."
internal/drivers/salida/udpts.go:202: "...el identificador de la señal (tsid) va del 1 al 65535..."
```

Con un matiz: el propio PRD, en la sección técnica del multiplexor
(`PRD.md:845-868`), sí usa `PID`/`PCR`/`TTL` al describir lo que **la persona
que instala** escribe una vez. La pregunta que dejo abierta —no la contesto
por mí, es una decisión de producto— es si esas frases de validación
aparecen solo en el asistente de instalación (donde el PRD ya acepta el
término) o también pueden aparecer más tarde en Al Aire si alguien cambia mal
un parámetro ya en operación; si es lo segundo, ahí sí es jerga llegando al
operador del día a día, y la lista de F1-56 debería crecer con estos cuatro
términos.

`salidaNoAbre` (`internal/app/salidas.go:84-93`) tiene el mismo problema en
general, no solo con `udpts.go`: guarda `err.Error()` tal cual como
`ultimo_error` para **cualquier** driver de salida, así que cualquier error
de Go sin envolver en cristiano —un futuro driver que devuelva
`"dial tcp: connection refused"`, por ejemplo— saldría directo a pantalla sin
que F1-56 lo detecte, porque el test de jerga no revisa mensajes armados en
tiempo de ejecución con `fmt.Errorf`/`%w`, solo cadenas literales del código
fuente.

## 6. Husos horarios

- `model.Day`/`Channel.BroadcastDay`/`DayStart`/`DayEnd` exigen `*time.Location`
  explícito en cada conversión (`internal/model/model.go:30,114-126`); no
  encontré un sitio donde se parsee un `Day` o una hora `HH:MM` sin pasar la
  zona del canal.
- `App.Now()` es UTC siempre (`internal/app/app.go:360`), y casi todos los
  sitios que escriben a la base lo hacen con `time.Now().UTC()` explícito
  (`internal/store/media.go:70,117,195,240`, `internal/store/log.go:45,172`,
  `internal/store/emparejar.go:77`, `internal/store/plan.go:349`). La única
  excepción es el nombre del archivo de respaldo,
  `internal/store/migrate.go:260` (`backupName(s.path, fromVersion,
  time.Now())`, sin `.UTC()`) — solo afecta el nombre del archivo en disco,
  no ningún dato; aseo, no bug.
- `resolver.PMCP` deja explícito que todo instante va en UTC porque así lo
  exige ATSC A/76 (`internal/resolver/pmcp.go:119-121,140,165`) — correcto y
  bien comentado.
- `resolver.XMLTV` usa `ch.Location()` para pintar la hora local con su
  desfase (`internal/resolver/xmltv.go:151`) — el formato XMLTV lleva el
  offset explícito en la cadena, así que no hay ambigüedad aunque la zona del
  canal cambie.
- El decodificador SAME (`internal/drivers/alerta/same/`, todo de hoy) marca
  bien que el campo `JJJHHMM` de la cabecera es hora UTC
  (`internal/drivers/alerta/same/cabecera.go:99`, `same.go:106`) y lo guarda
  como cadena cruda (`Instante string`) sin convertirlo a `time.Time`; el
  `Desplazamiento` que sí calcula es un `time.Duration` contado en muestras
  de audio desde el primer byte, no una resta contra el reloj de pared
  (`same.go:419,443,457`) — no hay mezcla de zonas en esta cuenta. Lo que
  **no pude verificar** es qué hace la capa que todavía no existe: ni
  `internal/app` ni `internal/api` referencian el paquete `same` ni `sage`
  todavía (comprobé con una búsqueda completa), así que la alerta decodificada
  hoy no llega aún a escribirse en `alert_event.inicio_ms`. Cuando se cablee,
  hay que revisar ahí, no aquí, que la cabecera UTC y el reloj de pared del
  driver se combinen sin restar o sumar el desfase del canal por error.
- No encontré ningún `time.Parse` (que asume UTC/sin zona) usado donde se
  esperaba `time.ParseInLocation` con la zona del canal.

## Prioridad

**Puede causar un fallo real (o ya lo está escondiendo):**

1. `incidente.tipo` sin `CHECK` + literales sueltos en `media.go`,
   `mantenimiento.go`, `resolve.go`, `asistente.go`, `relleno.go`, `app.go`
   en vez de las constantes de `incidentes.go` que ya existen para esas
   mismas cadenas (§1.1). Es el criterio F2-63 que el propio equipo dejó
   pendiente en `docs/f2/PLAN-F2.md:479`.
2. `guia_pmcp_rechazada` (nuevo de hoy) sin catálogo y sin frase en
   `cuarentena.go` — hoy solo se ve fea en pantalla, pero es la puerta por la
   que un tipo de incidente puede quedar sin trazabilidad real.
3. `parseInt64s`/`parseInts` tragando errores de JSON en silencio
   (§3) sobre `marcas_de_corte_ms` y `reloj_de_cortes` — un dato corrupto se
   comporta como dato vacío sin ninguna señal.
4. `salidaNoAbre` expone `err.Error()` crudo en `ultimo_error`
   (§5) para cualquier driver de salida presente o futuro, sin pasar por un
   texto en cristiano curado — hoy ya deja pasar `PID`/`PCR`/`TTL`/`tsid` del
   driver `udpts`.
5. `plan_item.corte_id` sin `REFERENCES corte(id)` mientras
   `break_marker.corte_id` sí la tiene para el mismo concepto (§3).

**Aseo (no rompe nada hoy, pero vale la pena):**

6. `maquina_despierta`/`guardian_caido` con frase propia pero sin entrada en
   el catálogo de `incidentes.go`; `apagon`/`vivo_ausente`/`cascada_extendida`
   catalogados pero nunca emitidos; `encoder_colgado`/`timeout_manual`
   dejados a propósito para filas viejas (documentado, no confundir con un
   error).
7. `PlanState` mezcla seis valores en inglés con uno en español
   (`fallido`) dentro del mismo enum (§2) — hoy no llega a pantalla en crudo.
8. `TitleKind` y el trío de `motivo_codigo` no están tipados como unión
   cerrada en `tipos.ts`, solo documentados en comentario (§4).
9. `output.estado_conexion` y `driver_config.tipo` sin `CHECK` aunque los
   valores de Go hoy coinciden con el comentario del esquema (§3).
10. `backupName` usa hora local para el nombre del archivo de respaldo en
    vez de UTC como el resto del sistema (§6) — cosmético.
