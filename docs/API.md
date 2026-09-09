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
| `GET /biblioteca` | Títulos con carátula, tipo, `episodios` (cuántos), `duracion_ms`, `estado_material` (`listo` / `aún no listo para aire` / `cuarentena`), `en_la_parrilla`, y `hora` / `regla_hasta` de la regla que lo programa. `material` y `duracion` son los mismos dos primeros en texto, y se mantienen por compatibilidad. |
| `GET /biblioteca/{id}` · `PUT /biblioteca/{id}` | Ficha del título: los mismos campos de la lista más `lista_de_episodios[]`, cada episodio con su `duracion_ms` y su `estado_material`. `PUT` edita nombre, sinopsis, tipo, carátula. |
| `GET /material` · `GET /material/{id}` | `media_asset` con medidas. |
| `PUT /material/{id}` | `negro_intencional`, `sin_logo`, `subtitulos_externos`, `marcas_de_corte_ms` (confirmar marcas candidatas). |
| `GET /cuarentena` | Los assets en cuarentena con `motivo_en_cristiano`. |
| `POST /cuarentena/{id}/dejar-pasar` | `{"quien":"Rolando"}` → estado `listo`, `dejado_pasar_por`, entrada en `audit_log`. |
| `POST /material/subir` | multipart; cae en la carpeta vigilada. |
| `GET /relleno` | La biblioteca de relleno; vacía → aviso. |

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
| `avisos_canal` | `ninguno` / `telegram` / `correo`. Por dónde sale el aviso de vencimiento de los 7 días (F1-46). |
| `avisos_telegram_token` · `avisos_telegram_chat` | La clave del bot y el chat al que se escribe. La clave es secreta. |
| `avisos_correo_para` · `avisos_smtp_servidor` · `avisos_smtp_usuario` · `avisos_smtp_clave` | A quién se le escribe y por qué servidor (`servidor:puerto`, con cifrado en cuanto el servidor lo ofrezca). La contraseña es secreta. |

Todo lo que cambia datos escribe en `audit_log` con `origen: humano` y el
autor de la sesión. El MCP (F6) usará exactamente estas rutas con `origen: mcp`.
