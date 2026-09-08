# F0 — el experimento del motor

Responde la pregunta del PRD §22.1: ¿el diseño "servidor de cuadros en Go
entre decodificadores por clip y un encoder persistente" produce salida
continua y limpia durante horas?

## Correrlo

```
go build -o bin/f0 ./cmd/f0
bin/f0 media                 # fabrica los 10 archivos de prueba en f0/media (≈1 min)
bin/f0 run -hours 8          # corre el motor; escribe f0/out/{catv.ts,web.ts,events.jsonl,stats.csv}
bin/f0 analyze               # mide y escribe f0/out/REPORTE.md
```

`bin/f0 all -hours 8` hace las tres. Con `-udp udp://IP:PUERTO` la salida
MPEG-2 va además al multiplexor de verdad. Con `-one` corre una sola salida
(para F0-07: CPU con una y con dos). Hace falta `ffmpeg` y `ffprobe` en el
PATH, junto al ejecutable, o en `ANTENA_FFMPEG`.

En Windows: `go build -o f0.exe .\cmd\f0` y lo mismo con `f0.exe`. La
corrida de 8 horas ocupa unos 48 GB de disco (36 del TS a 10 Mb/s, 12 de
la salida web).

## Qué se mide y cómo

Cada archivo de prueba lleva un **marcador** arriba a la izquierda: siete
bloques grises —dos de calibración, el id del clip, y cuatro nibbles con el
número de cuadro— que sobreviven el escalado y la recompresión. Y un
**pitido** de 2 kHz de 20 ms en su primera muestra. Con eso el analizador
lee la salida cuadro a cuadro y sabe, sin oído ni ojo:

- **F0-03** en cada cambio de clip, si el cuadro 0 del que entra aparece
  exactamente las veces que la conversión de tasa manda (2 a 29.97→59.94) y
  si el número de cuadro avanza sin saltos.
- **F0-02** dónde cae el pitido respecto al cuadro 0 del mismo clip; se
  compara cada clip consigo mismo entre vueltas (cada archivo fuente trae su
  propio retardo de arranque de audio), y la diferencia es la deriva.
- **F0-01** el peor salto entre muestras consecutivas alrededor del corte,
  por encima de lo normal del tono, en dBFS.
- **F0-04 / TS** el transport stream leído paquete a paquete: continuidad,
  PCR, tasa por ventanas de un segundo, marcas de tiempo por PID.
- **F0-06** que el archivo corrupto quede registrado como `clip_corto`, que
  entre `relleno`, y que no haya un solo cuadro con luma media bajo 16.
- **F0-07** CPU y RAM de todos los ffmpeg más el propio proceso, cada 10 s.

## Lo que F0 no prueba, y por qué

- **F0-05 (subtítulos CEA-608).** El archivo 7 es sintético y no trae 608
  reales —ffmpeg no puede generarlos— y la reinserción en la salida es
  código propio que el PRD pone en F2 (§9, paso 1). Para probarlo hace falta
  un archivo real con 608: ponlo en `f0/media/` con el nombre que diga
  `clips.json` y el analizador lo reporta. Hasta entonces se marca "—", no
  "pasa".
- **F0-08 (sesiones de encoder por hardware).** Es manual, en la máquina de
  destino.

## Dos cosas que se aprendieron construyéndolo

- **ffmpeg con un solo `filter_complex` que mezcla audio y video se traba**
  al tercer cuadro: pide las entradas por marca de tiempo y espera al audio
  con el video bloqueado. Con filtros por salida (`-filter:a:0`) no pasa. El
  encoder se arma así a propósito.
- **ffmpeg no abre su segunda entrada hasta haber leído algo de la primera.**
  El encoder acepta la conexión de audio aparte y guarda lo que llegue antes.
