# CONTINUAR — dónde quedamos y qué sigue

_Última sesión: 9 de septiembre de 2026. Siguiente: la que venga._

## Dónde estamos

- **F0 cerrada.** Reporte en `docs/f0/REPORTE-mac-m4-2026-09-08.md`.
- **F1 construida y verificada.** Los 57 criterios de `docs/ACEPTACION.md`
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
- **Repo:** https://github.com/Saul-Punybz/antena787 (privado).

## Cómo arrancar

```
cd ~/Downloads/_Projects/antena787
git pull
make antena                      # npm ci + build de la interfaz + go build
bin/antena -datos ~/antena-datos # abre http://127.0.0.1:7870
go test ./... -count=1           # todo debe estar verde
```

## Lo próximo, en orden

1. **Audio de todo el material (decisión de Saul, 9 sept): todo lo que sale
   al aire lleva audio.** Construir, en este orden:
   (a) **archivos sidecar** en la carpeta vigilada: `programa.mp4` +
   `programa.wav`/`.m4a` (audio) y `programa.srt`/`.scc` (subtítulos) se
   asocian por nombre y se muxean al normalizar — tamaño S;
   (b) **selector de pista** en Biblioteca cuando el archivo trae varias
   pistas de audio, con idioma cuando viene etiquetado y un default por canal
   (primero `es`, luego la primera) — tamaño M;
   (c) decidir si la salida «dejarlo pasar» de un archivo mudo se quita del
   todo o se deja como excepción explícita.
   **Reservar en el esquema** el sitio para una segunda pista al aire (SAP:
   español/inglés) que el motor de F2 llevará al mux; no mezclar pistas ni
   armar programas de varios clips (eso es edición, antes de la carpeta).
2. **Asistente de instalación con lógica real en la interfaz** (issue #5): la
   API `POST /instalacion/paso/{n}` funciona para los 9 pasos; falta la
   pantalla paso a paso (la prueba de barras del paso 5 «llega en F2»).
3. **Pantalla de emparejar títulos** (issue #13): lo que el importador deja en
   `Unmatched` (`Samurai X` ↔ `Rurouni Kenshin`, `SaberMarionette` J/R).
4. **Modo sombra con archivos de verdad** (y firma de los tres criterios
   manuales F1-32, F1-43, F1-53 con Rolando): videos reales en la carpeta
   vigilada, ver que el ingest mide, normaliza y el resolver programa.
5. **Cuarentena e incidentes en pantalla** (issues #6, #7): la API ya las sirve.
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

## Reglas que no cambian

- Commits limpios, sin atribución a herramientas. Autor: Saul A. González Alonso.
- Agentes con `opus` solo para lo esencial; `sonnet`/`haiku` para lo demás.
  Nunca el modelo de la sesión.
- El software nunca regaña; el cumplimiento se ofrece. Nunca negro, nunca
  silencio, nada manual que no vuelva solo.
- CAtv es donde se prueba, no el molde.
