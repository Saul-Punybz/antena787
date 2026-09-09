# CONTINUAR — dónde quedamos y qué sigue

_Última sesión: 8 de septiembre de 2026, noche. Siguiente: 9 de septiembre, temprano._

## Dónde estamos

- **F0 cerrada.** El motor (decodificador por clip → servidor de cuadros en Go →
  encoder persistente) aguantó 2 h en el Mac (413,302 cuadros, 276 cortes
  limpios, deriva 1 ms) y corrió en la PC Windows de CAtv. Reporte en
  `docs/f0/REPORTE-mac-m4-2026-09-08.md`. Issue #1 e hito F0 cerrados.
- **F1 construida y subida** (commit `6fdcd87`, rama `main`): 21,000 líneas
  de Go, un binario `antena` con la interfaz dentro, todo verde en el CI de
  Mac, Linux y Windows con ffmpeg real.
  - `internal/store` · `internal/resolver` · `internal/ingest` ·
    `internal/importer` · `internal/app` · `internal/api` · `cmd/antena` ·
    `web/` (Vite + React).
  - Humo real hecho: arrancar → asistente paso 1 → entrar → pegar la hoja
    real de CAtv → 34 reglas, Hellsing rechazado con su frase, 2 relevos y
    5 repeticiones propuestas, 17 erratas emparejadas con el catálogo →
    recalcular → `guia.xml` válido → interfaz en el navegador.
- **Repo:** https://github.com/Saul-Punybz/antena787 (privado). Docs del
  proyecto abierto completas (README ES/EN, CONTRIBUTING, AGPL, COMPLIANCE,
  COMPRAR, ROADMAP, ARQUITECTURA, drivers, profiles, matriz, examples), wiki
  con 4 páginas, 18 etiquetas, 9 hitos, 13 issues.

## Cómo arrancar mañana

```
cd ~/Downloads/_Projects/antena787
git pull
make antena                      # npm ci + build de la interfaz + go build
bin/antena -datos ~/antena-datos # abre http://127.0.0.1:7870
go test ./... -count=1           # todo debe estar verde
```
La clave de estación del humo de anoche era `7870` (datos en el scratchpad,
ya borrados: al arrancar de nuevo, el asistente pide una nueva).

## Lo próximo a construir, en orden

1. **Verificación de los 57 criterios F1** de `docs/ACEPTACION.md` (F1-01 a
   F1-57) contra lo construido: un agente que los recorra uno por uno con
   pruebas o a mano y marque PASA / FALLA / no aplica. Lo que falle, se
   arregla antes de seguir. (Es el paso que cierra F1 de verdad.)
2. **Asistente de instalación con lógica real en la interfaz** (issue #5):
   hoy es un placeholder con los 9 pasos; la API `POST /instalacion/paso/{n}`
   ya funciona para los 9. Falta la pantalla paso a paso, con la prueba de
   barras del paso 5 marcada como "llega en F2".
3. **Pantalla de emparejar títulos** (issue #13): lo que el importador deja
   en `Unmatched` (`Samurai X` ↔ `Rurouni Kenshin`, `SaberMarionette` J/R).
4. **UX pendiente que salió del humo:** si `entraste: false`, la interfaz
   debe llevar a Entrar en vez de mostrar Al aire vacío; el editor de regla
   debe llamar a `/plan/recalcular` al guardar.
5. **Modo sombra con archivos de verdad:** poner videos reales en la carpeta
   vigilada (o los de `f0/media`), ver que el ingest los mide, normaliza y
   el resolver los programa (hoy la hoja importa sin material → 38 avisos
   `sin_material`, correcto pero vacío). Es la primera vez que la Parrilla
   se llena con datos medidos.
6. **Cuarentena e incidentes en pantalla** (issues #6, #7): la API ya las
   sirve.
7. Después de eso, **F2 · Playout**: mover `internal/engine` de F0 al motor
   real (decks, salida `udp-ts` CBR al TP1000, fuentes en vivo SRT, manual,
   detector de silencio/negro, grabación y diferido, cascada, watchdog).
   Orden interno en PRD §22.3. Es la fase que se revisa línea por línea.

## Cosas pequeñas pendientes

- Correos `[correo por definir]` en `CODE_OF_CONDUCT.md` y `SECURITY.md`.
- `dist/` en el Mac tiene el zip de Windows de F0 (114 MB) y ffmpeg
  descomprimido; se puede borrar (`make clean` no lo toca a propósito… sí
  lo toca: borra `bin/` y `dist/`).
- `f0/out/` (11 GB de salida de la corrida de 2 h) se puede borrar;
  `f0/media/` (460 MB) vale la pena guardarla.
- Cuándo hacer público el repo: cuando el asistente y el modo sombra con
  archivos reales funcionen de punta a punta.

## Reglas que no cambian

- Commits limpios, sin atribución a herramientas. Autor: Saul A. González Alonso.
- Agentes con `model: "opus"`, nunca el modelo de la sesión.
- El software nunca regaña; el cumplimiento se ofrece. Nunca negro, nunca
  silencio, nada manual que no vuelva solo.
- CAtv es donde se prueba, no el molde.
