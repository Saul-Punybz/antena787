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
| `GET /estado` | `{canal, modo, ahora, dia_emision, al_aire: plan_item\|null, siguiente, alarmas[], version}` |
| `GET /canal` · `PUT /canal` | El canal (modelo `Channel`). Cambiar `zona_horaria` u `hora_inicio_dia_emision` recalcula el plan. |
| `GET /ajustes` · `PUT /ajustes` | Mapa clave→valor de `settings` (sin la clave de estación). |
| `WS /ws` | Empuja `{"tipo":"estado", ...}` cada segundo y `{"tipo":"evento", ...}` en cada incidente o cambio de plan. |

## Reglas y plan

| | |
|---|---|
| `GET /reglas` | Todas las reglas del canal, con `titulo` embebido y `dias_restantes`. |
| `POST /reglas` · `PUT /reglas/{id}` · `DELETE /reglas/{id}` | Crear, editar (`?solo_hoy=1` crea una excepción de un día en vez de cambiar la regla), borrar. Validación en cristiano: fechas cruzadas, patrón inválido, título sin material listo. |
| `GET /plan?dia=AAAA-MM-DD` | Los `plan_item` de ese día de emisión, en orden, con título/episodio/duración y huecos calculados (`{"hueco": true, "inicio", "fin"}`). |
| `GET /plan/semana?desde=AAAA-MM-DD` | Siete días, resumido por franja de 30 min (para Parrilla · Semana). |
| `GET /plan/mes?mes=AAAA-MM` | Por día: horas sin llenar, vencimientos, estrenos (para Parrilla · Mes). |
| `POST /plan/recalcular` | Fuerza una corrida del resolver ahora. Devuelve avisos: `[{"tipo":"sobrecupo"\|"hueco"\|"vencimiento"\|"sin_relleno", "texto"}]`. |
| `POST /plan/llenar-con-diferido` | `{"desde":"01:00","hasta":"06:00","origen_desde":"07:00","origen_hasta":"12:00"}` crea la regla de diferido de un clic. |
| `GET /guia.xml` | El XMLTV vigente (sin autenticación: lo lee el transmisor / MistServer). |
| `GET /guia?dia=` | La guía contra el plan real: `[{"guia": ..., "plan": ..., "coincide": bool}]`. |

## Biblioteca

| | |
|---|---|
| `GET /biblioteca` | Títulos con carátula, tipo, episodios, estado del material (`listo` / `aún no listo para aire` / `cuarentena`). |
| `GET /biblioteca/{id}` · `PUT /biblioteca/{id}` | Ficha del título; editar nombre, sinopsis, tipo, subir carátula. |
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

Todo lo que cambia datos escribe en `audit_log` con `origen: humano` y el
autor de la sesión. El MCP (F6) usará exactamente estas rutas con `origen: mcp`.
