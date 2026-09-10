# API de Antena787 — contrato de F1

Prefijo `/api/v1/`. JSON, con los nombres de campo del modelo (`internal/model`,
etiquetas `json`: en español, iguales a las columnas de la base). Todo
instante viaja como RFC 3339 en UTC; los días de emisión como `AAAA-MM-DD`;
las horas del día como `"HH:MM"` en la zona del canal. Errores: `4xx` con
`{"error": "frase en cristiano", "campo": "opcional"}`. Nada de códigos
crípticos: el texto del error es el que ve la persona.

Autenticación (F1): **clave de estación** (PRD §13, §19). `POST /api/v1/entrar`
con `{"clave": "1234"}` deja una cookie `antena_sesion` (HttpOnly,
SameSite=Strict, larga). Sin cookie válida, todo lo demás responde `401`.
El servidor escucha en `127.0.0.1:7870` y en la interfaz de Tailscale si
existe; nunca en `0.0.0.0` por defecto. Si no hay clave configurada aún,
`/api/v1/estado` responde `{"necesita_instalacion": true}` y solo el
asistente está abierto.

## Estado y canal

| Método y ruta | Qué hace |
|---|---|
| `GET /estado` | `{canal, modo, ahora, dia_emision, al_aire: plan_item\|null, siguiente, alarmas[], version, entraste, instalacion_completa, hay_anunciantes}`. `alarmas` son objetos: `{"tipo","nivel":"bien"\|"aviso"\|"problema","texto","detalle","accion":{"texto","ruta"}}`. `hay_anunciantes` enciende la sexta entrada del menú (Anuncios). `salidas`, `retorno_de_aire` y `control_manual` son de F2 y no se sirven todavía. |
| `GET /canal` · `PUT /canal` | El canal (modelo `Channel`). Cambiar `zona_horaria` u `hora_inicio_dia_emision` recalcula el plan. |
| `GET /ajustes` · `PUT /ajustes` | Mapa clave→valor de `settings` (sin la clave de estación). Los secretos (`clave_tmdb`, `avisos_telegram_token`, `avisos_smtp_clave`) salen tapados con `••••••`; devolverlos tapados en un `PUT` no los cambia. |
| `WS /ws` | Empuja `{"tipo":"estado", ...}` cada segundo y `{"tipo":"evento", ...}` en cada incidente o cambio de plan. |

## Reglas y plan

| | |
|---|---|
| `GET /reglas` | Todas las reglas del canal, con `titulo` embebido y `dias_restantes`. |
| `POST /reglas` · `PUT /reglas/{id}` · `DELETE /reglas/{id}` | Crear, editar (`?solo_hoy=1` crea una excepción de un día en vez de cambiar la regla), borrar. Validación en cristiano: fechas cruzadas, patrón inválido, título sin material listo. |
| `GET /plan?dia=AAAA-MM-DD` | Los `plan_item` de ese día de emisión, en orden, con título/episodio/duración y huecos calculados (`{"hueco": true, "inicio", "fin"}`). |
| `PUT /plan/{id}` | Mueve un bloque a mano, o lo suelta. Cuerpo: cualquier subconjunto de `{"instante_planeado":"2026-09-08T15:30:00-04:00","duracion_planeada_ms":1800000,"fijado":false}`. Con hora o duración el bloque queda **fijado** y la corrida siguiente del resolver ni lo mueve ni lo borra (F1-26); `{"fijado": false}` lo suelta. Devuelve `200` con el bloque en la misma forma que los items de `GET /plan`, `fijado` incluido. Errores: `409` si choca con otro bloque («a esa hora ya está …»), `400` si la hora no se entiende o se sale de las fechas de la regla, `404` si el bloque no existe. Después de cambiar, la guía se rehace en la misma corrida y se publica el mismo evento que el recálculo. |
| `GET /plan/semana?desde=AAAA-MM-DD` | Siete días: `{desde, dias:[{dia, franjas[48], horas_vacias}], horas_vacias_semana, nota?}`. Cada franja son 30 min desde la medianoche local: `{hora, titulo, en_vivo, duracion_ms, plan_id, fijado, estado}`, con `titulo` y `plan_id` en **nulo** cuando el tramo está vacío. |
| `GET /plan/mes?mes=AAAA-MM` | `{mes, dias:[{dia, horas_sin_llenar, franjas_llenas[48], vencimientos, estrenos}], horas_vacias_mes, porcentaje_vacio}`. |
| `POST /plan/recalcular` | Fuerza una corrida del resolver ahora. Devuelve avisos: `[{"tipo":"sobrecupo"\|"hueco"\|"vencimiento"\|"sin_relleno", "texto"}]`. |
| `POST /plan/llenar-con-diferido` | `{"desde":"01:00","hasta":"06:00","origen_desde":"07:00","origen_hasta":"12:00"}` crea la regla de diferido de un clic. |
| `GET /guia.xml` | El XMLTV vigente (sin autenticación: lo lee el transmisor / MistServer). |
| `GET /guia?dia=` | La guía contra el plan real: `{dia, filas:[{guia, plan, coincide}], identificador_de_canal, revalidada, por_que_no_coinciden?}`. `guia` y `plan` son `{titulo, inicio, duracion_ms}` o `null`. |

## Biblioteca

| | |
|---|---|
| `GET /biblioteca` | Títulos con carátula, tipo, `episodios` (cuántos), `duracion_ms`, `estado_material` (`listo` / `aún no listo para aire` / `cuarentena`), `en_la_parrilla`, `hora` / `regla_hasta` de la regla que lo programa, e `infantil_core`. `material` y `duracion` son los mismos dos primeros en texto, y se mantienen por compatibilidad. Si el título tiene archivo propio, lleva además el sonido de ese archivo (ver abajo). |
| `GET /biblioteca/{id}` · `PUT /biblioteca/{id}` | Ficha del título: los mismos campos de la lista más `lista_de_episodios[]`, cada episodio con su `duracion_ms`, su `estado_material` y el sonido de su archivo. `PUT` edita nombre, sinopsis, tipo, carátula y `infantil_core` (programa de educación o información para niños; cuenta para las horas de programación infantil de una estación Class A, F1-76). |
| `GET /material` · `GET /material/{id}` | `media_asset` con medidas. |
| `PUT /material/{id}` | `negro_intencional`, `sin_logo`, `subtitulos_externos`, `marcas_de_corte_ms` (confirmar marcas candidatas) y `pista_audio_aire` (ver abajo). |
| `GET /cuarentena` | Los assets en cuarentena con `titulo` (cómo se llama para una persona: el título o el episodio que lo usa, o el nombre del archivo si nadie lo fichó), `motivo_en_cristiano` y `motivo_codigo`. Mientras haya alguno, `/estado` lleva la alarma «N archivos en cuarentena» (nivel aviso, acción → `/biblioteca`). |
| `POST /cuarentena/{id}/dejar-pasar` | `{"quien":"Rolando"}` → estado `listo`, `dejado_pasar_por`, entrada en `audit_log`. `409` si el archivo no trae sonido (ver abajo). |
| `POST /material/subir` | multipart; cae en la carpeta vigilada. |
| `GET /relleno` | La biblioteca de relleno; vacía → aviso. |

### El sonido del material (F1-58 a F1-63)

Todo lo que sale al aire lleva audio. Cada título con archivo propio y cada
entrada de `lista_de_episodios[]` llevan estos cinco campos, todos
**opcionales**: no salen cuando el título no tiene archivo, y las rutas de al
lado no salen cuando no hubo ninguna.

| Campo | Qué es |
|---|---|
| `material_id` | El número del archivo (`media_asset`), que es el que se le pasa a `PUT /material/{id}`. |
| `pistas_audio` | Las pistas de sonido que trae el archivo: `[{"indice":0,"idioma":"en","canales":2,"titulo":"Original en inglés"}]`. `indice` empieza en cero y es el que entiende quien arma la copia de casa. `idioma` viene del propio archivo, ya unificado (`spa` y `esp` salen como `es`); vacío si el archivo no lo dice. |
| `pista_audio_aire` | El `indice` de la que sale al aire. De fábrica, la primera en el idioma de `idioma_audio_preferido`; si el archivo no trae ninguna en ese idioma, la primera que trae. |
| `audio_sidecar` | Ruta del archivo de sonido que estaba al lado del video y se metió en la copia de casa. Vacío cuando el video ya traía su sonido. |
| `subtitulos_sidecar` | Ruta del archivo de subtítulos que estaba al lado. Los `.srt` y `.vtt` entran en la copia de casa; los `.scc` y los `.mcc` (608/708 nativos) se guardan tal cual. |

`PUT /material/{id}` con `{"pista_audio_aire": 1}` cambia la pista que se va a
oír. Devuelve `200` con el archivo entero más `material_id` y
`estado_material`: al cambiar la pista, la copia de casa hay que rehacerla, así
que el archivo vuelve a la cola de normalización y su `estado_material` pasa a
`"aún no listo para aire"` hasta que esté. Errores: `400` con
`{"error":"ese archivo no tiene esa pista de sonido: escoge una de las que
trae","campo":"pista_audio_aire"}` si el índice no existe —no se cambia nada—
y `404` si el archivo no está. El cambio queda en `audit_log`. El cuerpo
admite a la vez los demás campos de `PUT /material/{id}`.

`GET /cuarentena` añade `motivo_codigo` a cada archivo parado. Desde el
esquema 5 vive en su columna (`media_asset.motivo_codigo`); una fila anterior
solo tiene el texto y el único código que existía entonces se reconoce por él
(`ingest.TextoSinAudio`). Los códigos:

| `motivo_codigo` | Qué pasó | ¿Se puede dejar pasar? |
|---|---|---|
| `sin_audio` | El archivo no trae sonido (F1-59). | **No**: se pone el audio al lado y se reprocesa solo. |
| `duracion_av_no_coincide` | La imagen y el sonido duran distinto más de 4 s (`ingest.DesfaseAVMaximo`, F1-70): el archivo llegó incompleto o se cortó al copiarlo. El motivo dice las dos duraciones. | Sí: hay material así a propósito. |
| `normalizacion_fallida` | No se pudo dejar el archivo en el formato de casa tras los intentos de la cola, o la preparación se pasó de su plazo y se canceló (F1-71). El plazo es `max(15 min, 4 × duración del archivo)`. | Sí: sale el original tal cual, sin ajustar el volumen. |
| `""` | Todo lo demás (no se pudo leer, dura cero, sin imagen ni sonido…). | Sí. |

Un archivo cuya normalización falla **no se queda «aún no listo para aire»
para siempre**: pasa a `cuarentena` con ese código, aparece en la lista, Al
aire lo cuenta en el aviso y queda el incidente `normalizacion_fallida`.

Un archivo con `motivo_codigo` `"sin_audio"` **no** se puede dejar pasar:
`POST /cuarentena/{id}/dejar-pasar` contesta `409` con
`{"error":"Este archivo no trae sonido y todo lo que sale al aire lleva audio:
pon a su lado un archivo de audio con el mismo nombre y se procesa solo."}`.
Ese es el camino: se deja el `.wav` (o `.m4a`, `.aac`, `.mp3`, `.flac`) con el
mismo nombre en la carpeta vigilada y el archivo se vuelve a procesar solo,
sobre la misma ficha, sin que nadie tenga que apretar nada.

## Importar

| | |
|---|---|
| `POST /importar/hoja` | `{"texto": "<pegado desde Sheets/Excel>"}` → `{"reglas_creadas": n, "titulos_creados": n, "relevos_propuestos": [...], "repeticiones_propuestas": [...], "filas_con_error": [{"fila": 12, "texto": "...", "motivo": "fin antes que inicio"}], "fechas_corridas": [...], "posibles_duplicados": [...], "avisos": [...], "titulos_sin_emparejar": [...], "resumen": "..."}`. Nunca rechaza la hoja entera. |
| `POST /importar/confirmar-relevos` | `[{"regla": id, "releva_a": id}]`. |

`titulos_creados` cuenta solo fichas de verdad: un título que quedó **por
emparejar** no es una ficha del catálogo todavía y no se cuenta ahí, sino en
`titulos_sin_emparejar`. Todas las listas de la respuesta llegan siempre como
lista, nunca `null`.

## Emparejar títulos (F1-64 a F1-67)

Una hoja hecha a mano llama a las cosas como quiere: la ficha se llama
«Rurouni Kenshin» y la hoja dice «Samurai X». El importador empareja solo lo
que puede decidir sin dudar; lo demás **no lo adivina y no lo crea callado**:
la regla se importa igual —la hoja nunca se rechaza—, el título queda marcado
«por emparejar» y sale en esta lista hasta que una persona diga cuál es.

| | |
|---|---|
| `GET /titulos/sin-emparejar` | `[{"id": 42, "nombre": "Samurai X", "texto": "«Samurai X» no tiene ficha en el catálogo", "candidatos": [{"id": 7, "nombre": "Saber Marionette J", "puntuacion": 0.93}], "reglas": 2, "franjas": ["lunes a viernes a las 2:30 PM"]}]`. Siempre una lista. |
| `GET /titulos/buscar?q=` | `[{"id": 7, "nombre": "Rurouni Kenshin", "tipo": "serie"}]`, 20 como mucho, para elegir la ficha buena. Solo fichas del canal, y nunca las que también están por emparejar. Sin `q` devuelve las primeras por orden alfabético. |
| `POST /titulos/{id}/emparejar` | La decisión: `{"accion": "usar", "title_id": n}`, `{"accion": "propio"}` o `{"accion": "quitar"}`. |

`titulos_sin_emparejar` de `POST /importar/hoja` trae exactamente estos mismos
objetos, ya con el identificador guardado: la pantalla de Reglas puede
emparejar de un clic sin volver a pedir la lista.

**`usar`** — es esta ficha del catálogo (F1-66). Las reglas que usaban el
título provisional pasan a la ficha, el provisional desaparece, el plan se
recalcula y la guía se vuelve a publicar. El nombre de la hoja queda guardado
como **alias** de la ficha: la próxima hoja que traiga ese nombre se empareja
sola, sin preguntar y sin que el parecido tenga que acertarlo (el alias se
busca por la clave del nombre, así que «Samurai X», «SAMURAIX» y «samurai-x»
son el mismo). Responde `200`:

```json
{
  "reglas_movidas": 2,
  "alias": "Samurai X",
  "texto": "«Samurai X» ahora es «Rurouni Kenshin»: 2 reglas pasaron a esa ficha y la próxima hoja que diga «Samurai X» se empareja sola"
}
```

**`propio`** — es un título nuevo de verdad (F1-67). La ficha se queda como
propia del canal y deja de estar por emparejar; las reglas no se tocan.
Responde `200` con `{"texto": "«Los Simuladores» se queda como título propio
del canal: ya no está por emparejar"}`.

**`quitar`** — no es un programa (F1-67): el nombre de una fuente en vivo, por
ejemplo. Se van el título y las reglas que lo usaban, diciendo cuántas.
Responde `200` con `{"reglas_quitadas": 2, "texto": "«SaberMarionette» ya no
está en la parrilla: se fueron con él 2 reglas que lo usaban"}`.

Los peros: `404` si el título no existe; `400` si falta `title_id` (campo
`title_id`), si la acción no es una de las tres (campo `accion`) o si se
empareja un título consigo mismo; `409` si la ficha destino también está por
emparejar —hay que resolver esa primero— o si se quiere quitar como
provisional una ficha del catálogo. Las tres rutas piden la clave de estación
como cualquier otra, y las tres decisiones quedan en la auditoría.

Mientras quede algún título por emparejar, `GET /estado` trae la alarma:

```json
{
  "tipo": "emparejar",
  "nivel": "aviso",
  "texto": "3 títulos por emparejar",
  "detalle": "«Los Simuladores», «SaberMarionette» y «Samurai X»",
  "accion": {"texto": "emparejar", "ruta": "/reglas"}
}
```

Se recalcula al arrancar, después de cada importación y después de cada
decisión, y se apaga sola cuando no queda ninguno. El nombre de una fuente en
vivo (`RadioOnce Live!`) nunca entra al catálogo como título, así que nunca
aparece en esta lista.

## Lo que el sistema hizo solo

| | |
|---|---|
| `GET /incidentes?desde=&hasta=` | Bitácora de incidentes: lo que el sistema hizo solo (PRD §15). Cada fila trae `id`, `tipo`, `inicio`, `fin` (nulo si sigue abierto), `detalle` y **`texto`**, la frase en cristiano del tipo (`app.TextoDeIncidente`: *«Un archivo quedó en cuarentena»*, *«El reloj de la máquina saltó y el plan se rehizo»*…; un `panico_<x>` sale como *«Una parte del sistema falló y se relanzó sola (x)»*; un tipo desconocido, legible con espacios). Sin fechas, la última semana. Fechas en RFC 3339; una mal escrita es `400`. Al aire la pinta como tarjeta con lo último y un panel al lado con 7/30/90 días. |
| `GET /auditoria?entidad=&id=` | `audit_log`, con la cadena de hash verificable (`GET /auditoria/verificar`). |

## Asistente de instalación (sin clave hasta terminar)

Nueve pasos (PRD §13); los pasos 3, 8 y 9 no preguntan nada, hacen o cuentan
algo solos. Mientras no exista clave de estación, `/api/v1/instalacion/*` es
la única puerta abierta: no hace falta cookie de sesión. El paso 1 es el que
pone la clave, y en ese mismo momento deja la cookie puesta — el resto de los
pasos sigue sin volver a pedirla. En cuanto ya hay una clave puesta —la ponga
el paso 1 o viniera de una instalación anterior— el asistente vuelve a pedir
la misma sesión que cualquier otra ruta.

### `GET /instalacion`

| Campo | Qué trae |
|---|---|
| `paso` | En qué paso va, según `instalacion.paso` (1 si no hay nada guardado). |
| `pasos` | Siempre 9. |
| `completa` | `si` guardado en `instalacion_completa`, como booleano. |
| `necesita_instalacion` | Aquí es solo «no hay clave de estación puesta»: a diferencia de `GET /estado` (que también mira si se llegó al paso 9), aquí no depende de si el asistente se terminó de contestar. |
| `canal` | El canal completo (modelo `Channel`). |
| `detectado` | Ver tabla de abajo. |
| `opciones` | El catálogo de respuestas posibles: ver tabla de abajo. |
| `respuestas` | Lo ya contestado, paso por paso. Ver tabla de abajo. |
| `tiempos` | `{"1": "2026-…", "4": "2026-…", …}`: cuándo se contestó cada paso, en RFC 3339. Solo trae los pasos que ya se contestaron. |

`detectado`:

| Campo | Qué dice |
|---|---|
| `ffmpeg` · `ffprobe` | Dónde están las herramientas de video en esta máquina; vacío si no se encontraron. |
| `problema` | Por qué no se encontraron, en cristiano. Solo sale cuando hay problema. |
| `carpeta_datos` | La carpeta de datos de la aplicación. |
| `carpeta_contenido` | La carpeta de contenido, si ya se puso (paso 7). |
| `carpeta_respaldo` | A dónde van los respaldos. |
| `relleno` | Cuántas piezas de relleno hay en la biblioteca ahora mismo (un número). Cero significa que el primer hueco sale al cartel. |
| `aceleracion` | Nunca un nombre de tarjeta: siempre la frase «se mide al arrancar el motor (F2); todavía no hay motor». Listar `-hwaccels` no basta porque los controladores mienten. |
| `disco` | Una frase en cristiano, p. ej. «890 GB libres de 2.0 TB en el disco de datos», o «no pude medir el disco» si no se pudo. |
| `red` | «conectado (nombre_de_la_tarjeta, dirección)» o «sin red: se puede seguir, la guía y el aire no la necesitan». |

`opciones` (el `valor` es lo que se manda de vuelta; nunca se enseña un
nombre técnico — PRD §4, principio 1):

| Lista | Valores | Nota |
|---|---|---|
| `modo` (paso 2) | `internet` · `transmisor` · `no_se` | «Todavía no sé» es una respuesta válida y no bloquea nada. |
| `destino` (paso 4) | `red` · `internet` · `route-dash` · `archivo` · `ninguna` | `route-dash` es la próxima generación de transmisión por antena: queda apuntado, todavía no está construido. `ninguna` es «todavía no lo sé». |
| `retorno` (paso 4) | `receptor-tv` · `captura` · `stream` · `ninguno` | `ninguno` («todavía no») es válida: sin retorno de aire se sigue igual, y se avisa. |
| `calidad` (paso 6) | `480i59.94` · `576i50` · `720p50` · `720p59.94` · `1080i50` · `1080i59.94` · `1080p25` · `1080p29.97` · `1080p59.94` | El formato de casa del canal. |

`respuestas` (la clave de estación **nunca** sale por aquí, ni cifrada ni en
claro; un paso solo aparece si ya quedó contestado):

| Paso | Forma |
|---|---|
| `1` | `{"nombre","identificativo","comunidad_licencia","nombre_operador"}`. Se da por contestado en cuanto hay clave de estación puesta, aunque venga de antes de que existiera este apunte. |
| `2` | `{"modo"}` |
| `4` | `{"destino","retorno_de_aire","nota"}` |
| `5` | `{"ve_barras": true\|false}` |
| `6` | `{"pais","calidad"}` |
| `7` | `{"carpeta"}` |
| `8` | `{"propuesta"}` |

Los pasos 3 y 9 no guardan respuesta propia y no aparecen en `respuestas`
(sí pueden aparecer en `tiempos` una vez contestados).

### `POST /instalacion/paso/{n}`

Cada paso contesta `{"paso": n, "siguiente": n+1, …lo suyo}` (200); el
paso 9 se queda apuntando a sí mismo como `siguiente`. Un error de
validación contesta `400` con `{"error","campo"}`. `n` fuera de 1–9 contesta
`400` con `campo: "n"`. Contestar un paso siempre deja dos cosas guardadas:
`instalacion.paso` (el paso siguiente) e `instalacion.paso_N_en` (el instante
en RFC 3339 en que se contestó ese paso n) — es la única forma de saber en
qué paso se abandona una instalación sin poner telemetría en la máquina de
nadie (F2-108).

| Paso | Cuerpo | Además en la respuesta | Errores propios (400) | Qué queda guardado |
|---|---|---|---|---|
| **1** | `{nombre, identificativo?, comunidad_licencia?, clave, nombre_operador?}` | `canal` (el canal completo) · `cartel` (`"<identificativo> · <comunidad_licencia>"`) | `campo: "nombre"` si viene vacío · `campo: "clave"` si no son 4 a 6 dígitos | Nombre/identificativo/comunidad en el canal · la clave de estación (`SetPIN`) · `nombre_operador` · **la cookie de sesión** (el único paso que la pone). Dejar `clave` en blanco cuando ya había una puesta **conserva la que hay**: no se obliga a escribirla otra vez. |
| **2** | `{modo}` | `modo_del_canal: "sombra"` (siempre, en F1 no hay motor) · `aviso` explicándolo | `campo: "modo"` si no es una de las opciones | `modo_previsto` |
| **3** | ninguno | `ffmpeg`, `ffprobe`, `problema?` | — | nada; no pregunta, cuenta lo que la máquina encontró sola |
| **4** | `{destino?, retorno_de_aire?, nota?}` | `aviso` cuando el retorno queda vacío o es `"ninguno"`: sin retorno de aire no se puede comparar lo que sale con lo que emites, y se dice — se puede seguir igual | `campo: "destino"` o `campo: "retorno_de_aire"` si no están en el catálogo (el mensaje lista las opciones) | `salida.destino`, `salida.retorno_de_aire`, `salida.nota` |
| **5** | `{ve_barras: bool}` | `aviso` fijo: la prueba de barras necesita el motor de emisión (F2), que todavía no existe; la respuesta queda apuntada igual | — | `instalacion.ve_barras` (`si`/`no`). Este paso **solo apunta la respuesta**: la prueba de barras de verdad sobre lo que sale al aire llega con el motor, en F2. |
| **6** | `{pais?, calidad?}` | `canal` (actualizado) · `formato` (`"AxB a FPS"`) | `campo: "calidad"` si no es una de las nueve opciones (el mensaje las lista) | `pais` y el perfil regulatorio del canal (`pr`/`puerto rico`/`us`/`usa`/`eeuu`/`estados unidos` → `us-fcc`; vacío no cambia nada; cualquier otro texto → `abierto`, el perfil que no exige nada) · `calidad` y el perfil de formato del canal |
| **7** | `{carpeta}` | `carpeta` (ruta absoluta) · `aviso_vigilancia` (fijo) · `aviso_relleno?` (solo si la biblioteca de relleno está vacía: avisa que el primer hueco sale al cartel) | `campo: "carpeta"` si viene vacía, o si la carpeta no se puede crear/usar | Se crea la carpeta si hace falta, se guarda como `carpeta_contenido` y queda vigilada desde ese instante |
| **8** | `{propuesta: "automatica"\|"ninguna"}` (vacío cuenta como `"ninguna"`) | `reglas_creadas`, `bloques`, `avisos[]`, `titulos_sin_material`, `aviso` | `campo: "propuesta"` si no es una de las dos | `instalacion.propuesta`. Ver abajo qué hace cada valor. |
| **9** | ninguno | `completa: true` · `modo_del_canal: "sombra"` · `aviso` | — | `instalacion_completa = si` |

**Paso 8, qué hace cada `propuesta`:**

- `"ninguna"` — no crea ninguna regla: arma el plan con las reglas que ya
  hubiera (`reglas_creadas: 0`). Es lo que pasaba antes de que la propuesta
  existiera; se deja la parrilla para hacerla después en Reglas, o para pegar
  una hoja de programación.
- `"automatica"` — si el canal **ya tenía reglas puestas** (a mano o por una
  hoja pegada), no toca nada: solo arma el plan con esas reglas
  (`reglas_creadas: 0`, aviso «ya tenías la parrilla puesta…»). Si no había
  ninguna, arma una regla por cada título programable que tenga material
  listo (series, películas y programas — las cortinillas, identificativos,
  promos y anuncios no cuentan): las **series** se colocan una detrás de
  otra desde que empieza el día de emisión del canal; las **películas y lo
  que va una sola vez** se colocan desde las **19:00** (o más tarde si el
  día de emisión empieza después de esa hora); cada espacio dura lo que dura
  el material, redondeado hacia arriba a la media hora; se proponen los
  siete días de la semana y valen **30 días** desde hoy. Lo que no cabe antes
  de las 19:00 se intenta después; lo que no cabe en ningún sitio se queda
  en Biblioteca sin regla, y se avisa. Si no hay material listo con qué
  armar nada, `reglas_creadas` queda en 0 y el aviso lo dice.

### `POST /instalacion/relleno-por-defecto`

Sin cuerpo. De un clic prepara el **cartel de la estación**: el nombre del
canal, y debajo el identificativo y la comunidad de licencia que dejó el
paso 1 — con una cama musical suave por debajo (tres tonos graves con una
ondulación lenta, a volumen bajo), un minuto de duración. **Nunca son las
barras y el tono**: eso es solo de la prueba del paso 5; el respaldo del aire
siempre termina en el cartel (F2-69). El texto se dibuja aparte, no con un
filtro sobre el video.

| Respuesta | Cuándo |
|---|---|
| `400` `{error, campo: ""}` | No hay herramientas de video en esta máquina. |
| `400` `{error, campo: "carpeta"}` | Todavía no se sabe la carpeta de contenido (falta el paso 7). |
| `409` `{error, archivo}` | El cartel ya está hecho, o ya se está haciendo: un segundo clic no lo repite. |
| `202` `{archivo, aviso}` | Arrancó. El trabajo de verdad tarda y se hace aparte: la petición no espera a que termine, y se avisa por `WS /ws` cuando está listo. |

Queda en `carpeta_contenido/relleno/cartel-de-la-estacion.mkv` (la ruta
también se guarda en el ajuste `relleno.por_defecto`). En cuanto termina
entra solo a la biblioteca, por el mismo camino de vigilancia de carpeta que
cualquier otro archivo, y queda marcado como relleno.

## Ajustes que reconoce el servidor

Todos viven en `settings` y se leen y escriben por `GET`/`PUT /ajustes`.

| Clave | Qué hace |
|---|---|
| `carpeta_contenido` · `carpeta_respaldo` | Las dos carpetas del asistente. |
| `ruta_guia_xml` | Dónde se escribe el XMLTV en disco. Vacío = solo `/guia.xml`. |
| `guia_destino_http` | Destino opcional al que se le manda la guía por `POST` (`application/xml`, 10 s de espera) cada vez que se publica. Que falle deja alarma y nada más: la guía local y el aire siguen igual (F1-49). |
| `fichas_en_linea` | `si` / `no` (de fábrica `no`). Enciende la búsqueda de fichas por internet: TVmaze y la portada de los discos, que no piden clave, y TMDB si hay clave. |
| `clave_tmdb` | La clave de TMDB. Secreto: sale tapado. |
| `idioma_audio_preferido` | `es` / `en` (de fábrica `es`). Cuando un archivo trae varias pistas de sonido, sale al aire la primera en ese idioma; si no trae ninguna, la primera del archivo (F1-60). Vale para lo que entre a partir de ahí: cambiarlo no vuelve a procesar lo que ya está fichado. |
| `avisos_canal` | `ninguno` / `telegram` / `correo`. Por dónde sale el aviso de vencimiento de los 7 días (F1-46). |
| `avisos_telegram_token` · `avisos_telegram_chat` | La clave del bot y el chat al que se escribe. La clave es secreta. |
| `avisos_correo_para` · `avisos_smtp_servidor` · `avisos_smtp_usuario` · `avisos_smtp_clave` | A quién se le escribe y por qué servidor (`servidor:puerto`, con cifrado en cuanto el servidor lo ofrezca). La contraseña es secreta. |
| `modo_previsto` | Lo que se contestó en el paso 2 del asistente: `internet` / `transmisor` / `no_se`. En F1 no cambia nada del aire: el canal se queda en modo sombra igual. |
| `salida.destino` · `salida.retorno_de_aire` · `salida.nota` | Lo que se contestó en el paso 4: a dónde va la señal, si hay manera de verla de vuelta, y una nota libre. Siempre uno de los valores del catálogo del asistente, nunca un nombre técnico. |
| `instalacion.ve_barras` | `si` / `no`: lo que se contestó en el paso 5. La prueba de verdad se hace con el motor de emisión, que llega en F2; aquí solo queda apuntada la respuesta. |
| `instalacion.propuesta` | `automatica` / `ninguna`: lo que se contestó en el paso 8. |
| `instalacion.paso` | En qué paso va el asistente; de ahí arranca `GET /instalacion` la próxima vez que se abra. |
| `instalacion.paso_1_en` … `instalacion.paso_9_en` | Cuándo se contestó cada paso del asistente, en RFC 3339. Sirve para saber en qué paso se abandona una instalación sin poner telemetría en la máquina de nadie (F2-108). |
| `relleno.por_defecto` | La ruta del cartel de la estación que generó `POST /instalacion/relleno-por-defecto`, una vez que ya está hecho o en camino. |
| `instalacion_completa` | `si` cuando se contestaron los nueve pasos del asistente (lo pone el paso 9). Junto con la clave de estación, decide `necesita_instalacion` en `GET /estado`. |

Todo lo que cambia datos escribe en `audit_log` con `origen: humano` y el
autor de la sesión. El MCP (F6) usará exactamente estas rutas con `origen: mcp`.
