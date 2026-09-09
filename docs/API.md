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
| `GET /biblioteca` | Títulos con carátula, tipo, `episodios` (cuántos), `duracion_ms`, `estado_material` (`listo` / `aún no listo para aire` / `cuarentena`), `en_la_parrilla`, y `hora` / `regla_hasta` de la regla que lo programa. `material` y `duracion` son los mismos dos primeros en texto, y se mantienen por compatibilidad. Si el título tiene archivo propio, lleva además el sonido de ese archivo (ver abajo). |
| `GET /biblioteca/{id}` · `PUT /biblioteca/{id}` | Ficha del título: los mismos campos de la lista más `lista_de_episodios[]`, cada episodio con su `duracion_ms`, su `estado_material` y el sonido de su archivo. `PUT` edita nombre, sinopsis, tipo, carátula. |
| `GET /material` · `GET /material/{id}` | `media_asset` con medidas. |
| `PUT /material/{id}` | `negro_intencional`, `sin_logo`, `subtitulos_externos`, `marcas_de_corte_ms` (confirmar marcas candidatas) y `pista_audio_aire` (ver abajo). |
| `GET /cuarentena` | Los assets en cuarentena con `motivo_en_cristiano` y `motivo_codigo`. |
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
| `subtitulos_sidecar` | Ruta del archivo de subtítulos que estaba al lado. Los `.srt` y `.vtt` entran en la copia de casa; los `.scc` se guardan tal cual. |

`PUT /material/{id}` con `{"pista_audio_aire": 1}` cambia la pista que se va a
oír. Devuelve `200` con el archivo entero más `material_id` y
`estado_material`: al cambiar la pista, la copia de casa hay que rehacerla, así
que el archivo vuelve a la cola de normalización y su `estado_material` pasa a
`"aún no listo para aire"` hasta que esté. Errores: `400` con
`{"error":"ese archivo no tiene esa pista de sonido: escoge una de las que
trae","campo":"pista_audio_aire"}` si el índice no existe —no se cambia nada—
y `404` si el archivo no está. El cambio queda en `audit_log`. El cuerpo
admite a la vez los demás campos de `PUT /material/{id}`.

`GET /cuarentena` añade `motivo_codigo` a cada archivo parado: hoy
`"sin_audio"` cuando el archivo no trae sonido, y `""` en todo lo demás. La
base guarda el motivo escrito para una persona y no el código, así que el
código se deduce de ese texto (`ingest.TextoSinAudio`); cuando haya más de un
código valdrá la pena guardarlo en su propia columna.

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
| `POST /importar/hoja` | `{"texto": "<pegado desde Sheets/Excel>"}` → `{"reglas_creadas": n, "titulos_creados": n, "relevos_propuestos": [...], "filas_con_error": [{"fila": 12, "texto": "...", "motivo": "fin antes que inicio"}], "fechas_corridas": [...]}`. Nunca rechaza la hoja entera. |
| `POST /importar/confirmar-relevos` | `[{"regla": id, "releva_a": id}]`. |

## Lo que el sistema hizo solo

| | |
|---|---|
| `GET /incidentes?desde=&hasta=` | Bitácora de incidentes. |
| `GET /auditoria?entidad=&id=` | `audit_log`, con la cadena de hash verificable (`GET /auditoria/verificar`). |

## Asistente de instalación (sin clave hasta terminar)

| | |
|---|---|
| `GET /instalacion` | Paso actual y lo detectado (ffmpeg, aceleración, discos, red). |
| `POST /instalacion/paso/{n}` | Respuesta de cada paso (§13): 1 nombre/identificativo/comunidad/clave · 2 qué vas a hacer · 4 a dónde va la señal y retorno de aire · 5 prueba de barras (`{"ve_barras": true}`) · 6 país y calidad · 7 carpeta de contenido · 8 primera parrilla · 9 al aire. |

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

Todo lo que cambia datos escribe en `audit_log` con `origen: humano` y el
autor de la sesión. El MCP (F6) usará exactamente estas rutas con `origen: mcp`.
