# CONTINUAR — dónde quedamos y qué sigue

_Última sesión: 9 de septiembre de 2026 (cinco tandas). Siguiente: la que venga._

## Dónde estamos

- **F0 cerrada.** Reporte en `docs/f0/REPORTE-mac-m4-2026-09-08.md`.
- **F1 construida y verificada.** Los 57 criterios (63 desde la segunda tanda) de `docs/ACEPTACION.md`
  recorridos uno por uno con pruebas (`f1verif_*_test.go` en cada paquete);
  informe completo en **`docs/f1/VERIFICACION-F1-2026-09-09.md`**. De 18
  defectos encontrados, 16 corregidos en la misma sesión y 2 diferidos con
  nota en `ACEPTACION.md` (F1-04 subtítulos → F2; reporte de ingresos de
  F1-55 → F4). Lo que se construyó al corregir:
  - Esquema **versión 2** (migración automática): `plan_item.fijado`,
    `fundido_salida_ms`, triggers de vigencia y de solape por `estado`.
  - **Edición a mano de la parrilla** que sobrevive al resolver:
    `PUT /api/v1/plan/{id}`, panel «¿Solo hoy, o siempre?» cableado, chincheta
    y «Soltar».
  - **Validador de la guía** corriendo antes de publicar (lo grave no sale; los
    huecos se publican y se avisan).
  - **Avisos de vencimiento** en Al aire y por **Telegram o correo** a 7 días
    (Ajustes → Avisos).
  - **Fichas en línea** (TVmaze → TMDB con clave) encendibles desde Ajustes.
  - **Envío opcional de la guía por HTTP** (`guia_destino_http`).
  - La API emite exactamente el contrato de `web/src/lib/tipos.ts` (prueba de
    contrato en `internal/api/contrato_test.go`); Biblioteca, Parrilla
    semana/mes/guía y Al aire pintan datos reales.
  - Un archivo **sin audio va a cuarentena** (se puede soltar bajo
    responsabilidad del operador); LUFS y pico verdadero se guardan.
  - La cola de normalización prioriza por hora de aire calculada desde las
    reglas.
- **Repo:** https://github.com/Saul-Punybz/antena787 (privado). `main` está verde.
- **Investigación y decisiones nuevas (9 sept):**
  `docs/investigacion/PROYECTOS-SIMILARES-2026-09-09.md` (27 proyectos
  comparables) y `docs/adr/0010-eas-integrate-the-endec-never-replace-it.md`
  (EAS: integrar el ENDEC, evidencia SAME en el retorno, CAP informativo).

## Cómo arrancar

```
cd ~/Downloads/_Projects/antena787
git pull
make antena                      # npm ci + build de la interfaz + go build
bin/antena -datos ~/antena-datos # abre http://127.0.0.1:7870
go test ./... -count=1           # todo debe estar verde
```

## Lo próximo, en orden

1. ~~Audio de todo el material~~ **Hecho (9 sept, segunda tanda):** criterios
   F1-58 a F1-63 en `ACEPTACION.md`, esquema **versión 3** (`pistas_audio`,
   `pista_audio_aire`, `pista_audio_sap` reservado para F2, `audio_sidecar`,
   `subtitulos_sidecar`). Archivos de al lado por nombre (audio `.wav/.m4a/
   .aac/.mp3/.flac`, subtítulos `.srt/.vtt` muxeados, `.scc` guardado para
   F2), reingesta sola cuando el audio llega tarde, selector de pista en
   Biblioteca (default por `idioma_audio_preferido`, Ajustes → Audio), y un
   archivo mudo **no se puede soltar**: todo lo que sale al aire lleva audio.
   ~~Pendiente menor: `GET /cuarentena` no manda `titulo`.~~ Resuelto en la
   quinta tanda (punto 4).
2. ~~Asistente de instalación~~ **Hecho (9 sept, tercera tanda; cierra el issue
   #5):** los nueve pasos con lógica real en `web/src/pantallas/Asistente.tsx`
   (riel de progreso, reanudar desde `paso`, respuestas precargadas, barras
   SMPTE dibujadas en el navegador con las dos respuestas activas y una línea
   honesta de que la prueba en la salida llega con el motor). El servidor
   detecta disco y red, entrega las listas de opciones de los pasos 2/4/6
   (nunca se pide un nombre de driver), guarda respuestas y tiempos por paso
   (para saber dónde se abandona, F2-108), valida `calidad`, y en el paso 8
   **arma una propuesta** con lo que haya en la biblioteca (`App.ProponerParrilla`:
   series a diario desde el inicio del día de emisión, películas a las 19:00,
   30 días; no toca nada si ya hay reglas). Nuevo
   `POST /instalacion/relleno-por-defecto`: cartel de la estación (dibujado en
   Go, fuentes Go embebidas) con cama de tonos suaves, 60 s, nunca barras
   (F2-69). `necesita_instalacion` se mantiene hasta el paso 9. Volver al paso 1
   con la clave en blanco conserva la que hay. Modo demo completo
   (`?instalar=1`, `&reiniciar=1` para empezar de cero). Humo de punta a punta
   con el binario real: verde.
   Pendiente menor: `schedule_rule` no tiene campo de nota, así que la
   procedencia «propuesta del asistente» queda en el incidente y en
   `instalacion.propuesta`, no en cada regla (columna `nota` + migración es el
   arreglo). La opción `custom` de formato no se ofrece en el asistente.
3. ~~Pantalla de emparejar títulos~~ **Hecho (9 sept, cuarta tanda; cierra el
   issue #13):** criterios F1-64 a F1-67. Esquema **v4** (`title.pendiente_emparejar`,
   `title.candidatos`, tabla `title_alias`), importador con alias antes del
   difuso y `Unmatched` con reglas/franjas/puntuaciones, las fuentes en vivo
   ya no crean título, `GET /titulos/sin-emparejar`, `GET /titulos/buscar`,
   `POST /titulos/{id}/emparejar` (usar / propio / quitar), alarma «N títulos
   por emparejar» con acción a Reglas, sección «Títulos por emparejar» en
   Reglas con buscador y demo completa. Probado con la hoja real de CAtv:
   3 por emparejar (Samurai X, SaberMarionette con dos candidatos, Los
   Simuladores); tras emparejar Samurai X → Rurouni Kenshin, la reimportación
   ya no pregunta («lo recordaba de otra hoja»).
   Nota: si la hoja pegada no trae su catálogo, el alias se aplica por la API
   al crear la regla (enlaza bien, pero sin el aviso de «lo recordaba»).
4. ~~Cuarentena e incidentes en pantalla~~ **Hecho (9 sept, quinta tanda;
   cierra los issues #6 y #7):** criterios F1-68 y F1-69. `GET /cuarentena`
   ya manda `titulo` con nombre de persona (título o «Serie · T1E4 Nombre»,
   o el nombre del archivo si nadie lo fichó: `TitleRepo.NombreDelArchivo`),
   y mientras quede algo parado Al aire enseña el aviso «N archivos en
   cuarentena» → Biblioteca (`App.RefreshCuarentena`: al arrancar, tras cada
   ingest y al dejar pasar). La **bitácora** vive en Al aire
   (`componentes/Bitacora.tsx`): tarjeta con lo último y panel al lado con
   7/30/90 días agrupado por día; `GET /incidentes` manda `texto`, la frase
   en cristiano de cada tipo (`app.TextoDeIncidente`, con los tipos de F2
   ya escritos), y la pantalla se refresca con cada `{"tipo":"evento"}` del
   WebSocket (`suscribirseAEventos` en `lib/api.ts`). Cambio de semántica:
   **`App.Incident` cierra el suceso en el mismo instante** (`fin` =
   `inicio`); antes quedaban abiertos y «la última semana» arrastraba todo
   lo viejo. Lo que dura (vivo ausente, apagón, F2) se inserta abierto y se
   cierra con `Store.Incident.Close`. Humo con el binario real y un video
   mudo hecho con ffmpeg: cuarentena con nombre, alarma, incidente con
   frase, 409 al intentar soltarlo. Verde.
   Pendiente menor: las filas de `incidente` anteriores a esta tanda siguen
   con `fin` nulo (en una base de pruebas, nada que migrar).
5. **Modo sombra con archivos de verdad** (y firma de los tres criterios
   manuales F1-32, F1-43, F1-53 con Rolando): videos reales en la carpeta
   vigilada, ver que el ingest mide, normaliza y el resolver programa.
6. Después, **F2 · Playout** (PRD §22.3), donde además se cierran F1-04
   (CEA-608 con detección propia, ffprobe ≥ 9 ya no emite `closed_captions`),
   el fundido de 1 s del clip recortado (`fundido_salida_ms` ya viene en el
   plan) y los ítems `dentro_de` de un vivo.

## Cosas pequeñas pendientes

- `MarkAired` ya escribe el contador en la regla dueña; el motor (F2) es quien
  lo llamará.
- `media_asset` no tiene columna para el número de pasadas de volumen; hoy el
  registro de las dos pasadas son los valores medidos + evento `material/volumen`.
- `horas_vacias` de Parrilla · Semana cuenta sobre el día natural (la cuadrícula
  arranca a medianoche, como asume la interfaz); si se quiere por día de
  emisión, es un cambio pequeño en `internal/api/plan.go`.
- Cuándo hacer público el repo: cuando el asistente y el modo sombra con
  archivos reales funcionen de punta a punta.
- Los incidentes se pintan con la frase del servidor; si un servidor viejo no
  manda `texto`, la interfaz enseña el tipo legible (`textoDe` en
  `Bitacora.tsx`).

## Reglas que no cambian

- Commits limpios, sin atribución a herramientas. Autor: Saul A. González Alonso.
- Agentes con `opus` solo para lo esencial; `sonnet`/`haiku` para lo demás.
  Nunca el modelo de la sesión.
- El software nunca regaña; el cumplimiento se ofrece. Nunca negro, nunca
  silencio, nada manual que no vuelva solo.
- CAtv es donde se prueba, no el molde.
