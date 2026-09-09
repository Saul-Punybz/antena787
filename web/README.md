# Interfaz web de Antena787

Vite + React 19 + TypeScript. Sin frameworks de UI: CSS propio con variables,
`react-router` en modo hash y `fetch` nativo contra `docs/API.md`.

La build es **estática**: `npm run build` deja `web/dist/`, que el binario Go
embebe con `go:embed` y sirve tal cual. Por eso las rutas van por hash y la
base es relativa — no hay servidor que reescriba rutas.

## Comandos

```sh
npm install       # o npm ci
npm run dev       # desarrollo; hace proxy de /api a 127.0.0.1:7870
npm run check     # tsc --noEmit
npm run build     # tsc + vite build → dist/
npm run preview   # sirve dist/ tal como lo servirá el binario
```

## Modo demo

El servidor Go todavía no existe. `src/demo/` trae la semana de CAtv del 6 al
12 de septiembre de 2026 (de `docs/catv-sheet-2026-09-04.md`) y responde las
mismas rutas de `docs/API.md`.

```sh
VITE_DEMO=1 npm run dev
VITE_DEMO=1 npm run build && npm run preview
```

Sin `VITE_DEMO`, la interfaz habla con el servidor de verdad y **cae sola al
modo demo** si `/api/v1/estado` no contesta, para no quedarse en blanco. El
encabezado lo dice con todas sus letras.

## Cómo está armado

```
src/
  lib/        api.ts (cliente tipado + WebSocket con reconexión),
              tipos.ts (el contrato de docs/API.md), fechas.ts (Intl en la
              zona del canal), estado.tsx (el estado en vivo)
  demo/       datos.ts (CAtv) y servidor.ts (las mismas rutas, sin servidor)
  componentes/ Armazon, EncabezadoDeAire, Panel, Caratula, PatronDeDias
  pantallas/  Entrar, Asistente, AlAire, Parrilla{Semana,Mes,Guia},
              Reglas, Biblioteca, Ajustes
  estilos/    global.css — la paleta y las piezas de diseno/
```

## Reglas que no se rompen

- **Ningún modal encima de la vista de aire** (`docs/adr/0008`). Lo que se
  edita se abre en `<Panel>`, un panel al lado que no tapa ni pausa el aire.
- **El modo se ve como color de fondo y texto**, nunca como un icono solo.
- **El punto rojo de tally siempre en el mismo sitio** del encabezado.
- **Nada de jerga**: nunca *driver*, *códec*, *LKFS* ni *bitrate* fuera de
  En vivo. El volumen se enseña como "volumen de televisión de EE. UU.".
- **Todo el texto en español**, con el vocabulario de `CONTEXT.md`: regla,
  plan, día de emisión, relleno, cuarentena, incidente.

## Lo que falta

En vivo, Anuncios, Manual, Diferido, Portal y Música — más la lógica real del
asistente de instalación. La estructura de rutas y el armazón ya los esperan.
