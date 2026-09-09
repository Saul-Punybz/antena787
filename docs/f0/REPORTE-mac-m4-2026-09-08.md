# F0 — reporte del experimento

_2026-09-08 20:51_

Corrida: 1 h 55 min en un Mac M4 (parada a propósito a las 2 h; §22.1 dice por qué) · 413302 cuadros en la salida · TS: paquetes 45845917 (nulos 13.2%) · PIDs 7 · errores CC 0 · PCR 358428, brecha max 20.5 ms, media 19.2 ms · tasa media 10.000 Mb/s (min 10.000, max 10.000, desvío 0.00%) · marcas no monotónicas 0 · saltos >1s 0 · 6895.2 s

| Criterio | Qué mide | Resultado | |
|---|---|---|---|
| F0-04 | Marcas de tiempo monotónicas, sin saltos | 0 retrocesos, 0 saltos >1 s | **PASA** |
| F0-TS | Tasa constante ±1 %, PCR ≤ 40 ms, sin errores de continuidad | 10.000 Mb/s ±0.00 % · PCR máx 20.5 ms · CC 0 | **PASA** |
| F0-03 | Cuadros duplicados o perdidos en cada cambio de clip | 276 cambios revisados, 0 con problema | **PASA** |
| F0-NEGRO | Ningún cuadro negro en la salida | 0 cuadros negros de 413302 | **PASA** |
| F0-01 | Discontinuidad de audio en cada cambio de clip (< −40 dBFS) | peor: -120.0 dBFS en 275 cortes | **PASA** |
| F0-02 | Desfase audio-video acumulado (< 20 ms) | deriva máxima 1.0 ms (02-h264-720p5994) comparando cada clip consigo mismo en 275 cortes; desfase fijo por archivo entre 9.0 y 14.0 ms | **PASA** |
| F0-05 | Los subtítulos CEA-608 llegan a la salida | no hay subtítulos en la salida: la reinserción es código propio de F2, y el archivo 7 sintético no trae 608 reales (ver f0/README.md) | **—** |
| F0-06 | El archivo corrupto cae a relleno sin negro y queda registrado | 25 veces detectado como corto, 25 entradas de relleno, 0 cuadros negros | **PASA** |
| F0-07 | CPU y RAM medidos (dos salidas) | CPU media 110 % (de un núcleo), RAM media 699 MB, 689 muestras | **PASA** |
| F0-08 | Sesiones de encoder por hardware que aguanta la máquina | manual: no se midió en esta corrida | **—** |

## Eventos del servidor de cuadros

- `audio_corto`: 75
- `clip_corto`: 25
- `corte`: 251
- `relleno`: 25

## Veredicto

Los criterios medibles pasan. Lo marcado con — no se pudo medir en esta corrida y se dice por qué.
