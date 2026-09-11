# Antes de construir la entrada por URL: qué hay ya, y qué estorba

_11 de septiembre de 2026. Saul pidió verificar, antes de escribir una línea, si
se había quedado a medias algún sitio donde guardar una URL o un enlace. Se
había quedado. Cuatro veces._

## Las cuatro tablas de entrada, y en qué estado están

| Tabla | Qué es | Repo | API | Pantalla |
|---|---|---|---|---|
| `live_source` | La fuente en vivo del programa | **Sí** (`List`, `Upsert`) | **No** | **No** |
| `driver_config` | Configuración de conexión **con credenciales cifradas** | No | No | No |
| `capture_input` | El retorno de aire (ver la señal propia) | No | No | No |
| `portal_link` | El portal del anunciante | No | No | No |

`portal_link` está muerta **y es correcto**: es F4, pospuesto por decisión de
Saul. Las otras tres son huecos.

## Lo que `live_source` ya tiene pensado, y nadie puede tocar

Está en `internal/store/schema.sql:126`. No es un esqueleto: alguien pensó el
problema entero y lo dejó escrito.

- `punto_de_escucha` — dónde se recibe o de dónde se tira.
- `retardo_ms`, **de fábrica 7000** — los siete segundos de margen antes de dar
  la señal por ausente.
- `gracia_s`, **de fábrica 30** — cuánto se aguanta antes de soltar el aire.
- `filler_de_respaldo` — qué sale si el vivo no llega. **La cascada ya está
  pensada para una fuente remota.**
- `reloj_de_cortes` — los minutos de corte, `[0,15,30,45]`, que es exactamente
  como funciona RadioOnce Live! (corta cada 15 minutos).
- `solo_audio`, `duracion_prevista_ms`, `driver_de_cue`.

**Nada de eso es alcanzable hoy:** no hay una sola ruta en el router.

## El bloqueo que hay que quitar primero

```sql
tipo TEXT NOT NULL CHECK (tipo IN ('srt','rtmp','captura'))
```

**`url` no está permitido.** F2-116 no es «añadir una opción a un desplegable»:
la base **rechaza la fila**. Hace falta una migración que amplíe el CHECK.
SQLite no sabe alterar un CHECK, así que es tabla nueva, copiar y renombrar —
el patrón normal, pero hay que hacerlo bien porque `schedule_rule` y `plan_item`
apuntan a `live_source`.

## `driver_config`: el sitio para las claves, ya diseñado y nunca construido

`internal/store/schema.sql:50`:

```sql
tipo         TEXT NOT NULL,   -- salida | alerta | cobro | fichas | entrada | ...
credenciales BLOB,            -- cifradas (DPAPI / keyring)
parametros   TEXT NOT NULL DEFAULT '{}'
```

El `tipo` **ya contempla `entrada`**, y el sitio para la clave **ya está pensado
como cifrado**, no como texto.

Y hace falta de verdad: el cliente tira de `video2.getstreamhosting.com`, que
casi seguro pide usuario y clave. **Una clave de un proveedor no puede vivir en
un campo de texto plano de una tabla que se respalda cada hora y se copia a
otra máquina.**

## Lo que esto le hace al trabajo de F2-116

No es una pantalla. Son, en orden:

1. **La migración que abre el CHECK** a `url`. Sin esto no hay nada.
2. **Dónde vive la clave.** `driver_config` está diseñado; hay que construirlo,
   y decidir con qué se cifra en cada sistema operativo sin meter CGo.
3. **La API de fuentes**, que no existe: crear, listar, cambiar, borrar.
4. **La pantalla**, con lo que la investigación paralela diga que hace falta
   —probar antes de guardar, y qué enseñar cuando la señal se cae—.
5. **El driver que de verdad abre la URL** (T4), con reconexión, y enganchado a
   la cascada que `filler_de_respaldo` ya prevé.

**Y una advertencia, que es la lección que ya costó cara en este proyecto:** la
pantalla sola sería una mentira. Crearía filas bonitas que no reproducen nada.
Los cinco pasos, o ninguno.
