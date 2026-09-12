# La cadena real de CAtv, leída de su propia pantalla · 11 sep 2026

Rolando mandó diez capturas de la PC de la torre (Arecibo), tomadas por
escritorio remoto, con **VLC 3.0.23 Vetinari** y el asistente de emisión abierto
paso por paso. Esto sustituye a todas las suposiciones anteriores.

> **La dirección multicast NO se escribe aquí.** Rolando la tachó a propósito en
> la captura del formulario y solo se le escapó en el texto generado. Este
> repositorio es público. El grupo va en el **`224.x.x.x`** del rango multicast y
> el puerto es **1234**; el número exacto vive en la instalación, no en git.

## La línea de emisión, tal cual

```
:sout=#transcode{vcodec=mp2v,vb=4800,scale=Auto,width=1280,height=720,
                 acodec=mpga,ab=128,channels=2,samplerate=44100,scodec=dvbs}
      :udp{mux=ts,dst=224.x.x.x:1234}
      :sout-all :sout-keep
```

## De dónde sale el video — y esto cambia el plan

El título de la ventana de VLC dice:

```
Converting http://localhost:8080/hls/for_tv/index.m3u8 - VLC media player
```

y en la lista hay entradas de `video2.getstreamhosting.com:19360/…` y un
`file:///C:/Users/CUBE/Desktop/ID-Tone.mp4`.

**El canal de CAtv no se alimenta de archivos: se alimenta de streams.** HLS
desde MistServer en la propia máquina, y HLS desde un proveedor externo. El
único archivo local es el tono de identificación.

**Consecuencia:** `F2-116` —tirar de una URL como fuente en vivo— **deja de ser
una función opcional del final del plan**. Es cómo funciona su canal hoy. Sin
eso, Antena787 no puede sustituir a VLC en CAtv por mucho que la salida sea
perfecta.

## Valor por valor, contra lo que teníamos

| Qué | Rolando | Antena787 | Veredicto |
|---|---|---|---|
| Códec de video | `mp2v` (MPEG-2) | MPEG-2 | Acertado |
| Bitrate de video | **4800 kb/s** | 4000 de ejemplo | **Ajustar el ejemplo** |
| Resolución | 1280×720 | 1280×720 | Acertado |
| Cuadros por segundo | «Same as source» — no lo fuerza | 59.94 de casa | Ver abajo |
| Códec de audio | `mpga` (MPEG capa II) | `mp2` | Acertado |
| Bitrate de audio | **128 kb/s** | 192 | **Ajustar el ejemplo** |
| **Muestreo de audio** | **44100 Hz** | **48000 Hz** | **Discrepancia real** |
| Canales | 2 | 2 | Acertado |
| Salida | `udp{mux=ts}` a grupo multicast :1234 | `udp-ts`, grupo detectado por la dirección | Acertado |
| TTL | **no lo escribe** → VLC usa 1 | 1 de fábrica | **Acertado** |
| PIDs, programa, tsid | **no escribe ninguno** | opcionales | **Confirma que el mux los reasigna** |
| Subtítulos | `scodec=dvbs` | — | Ver abajo |
| Continuidad entre clips | **`:sout-keep`** | encoder persistente (ADR 0001) | **Confirma el diseño** |

## Los tres que hay que hablar con él

**1 · 44100 Hz.** El estándar de emisión es 48 kHz y el formato de casa de
Antena787 es 48000. MPEG capa II admite 44.1, así que no es ilegal, pero es
raro en una cadena de televisión y es el tipo de cosa que hace que un equipo
río abajo se comporte distinto sin que nadie sepa por qué. **No se cambia en
silencio:** se le dice y él decide.

**2 · `scodec=dvbs`.** La línea lleva subtítulos DVB puestos, pero él contestó
que **no emite closed captions** y que «si tengo algo con closed caption se
supone lo pase». Hay que aclarar si eso es un valor que VLC dejó ahí solo o si
de verdad está pasando algo.

**3 · «Same as source» en los cuadros por segundo.** No fuerza la tasa: sale lo
que traiga la fuente. Con fuentes HLS de distinta procedencia eso significa que
**la tasa de cuadros de su emisión puede cambiar según qué esté dando**, que es
justo lo que un multiplexor y un excitador no quieren. Antena787 conforma todo
al formato de casa, que es lo correcto — conviene que sepa que eso es una
mejora, no un capricho.

## Lo que esto confirma de lo ya construido

- **El encoder persistente apuntaba al problema correcto.** `:sout-keep` es VLC
  intentando lo mismo y haciéndolo a medias.
- **Los PIDs nunca fueron el bloqueo.** No escribe ninguno.
- **TTL 1 de fábrica era lo correcto** para un equipo en el mismo switch.
- **Multicast como camino principal**, no como opción.

---

## El síntoma que lo confirma todo · Rolando, 11 sep 2026 (tarde)

Después de leer el análisis del audio, contestó:

> «El mpeg y aac son los estandares. Los q tu pones los tira vlc, los coge el
> tp1000 los transmite el transmisor **pero no todos los tvs lo cogen**.»

**Esa última frase es el hallazgo más valioso del despliegue hasta ahora**, y no
vino como pregunta: vino como algo que él ya sabía y daba por normal.

### Por qué encaja

La cadena «funciona» hasta el transmisor porque **el TP1000 y el excitador no
validan el estándar: solo pasan los bits**. Quien lo valida es el televisor del
televidente. Un fallo de cumplimiento no se ve en la sala de control — se ve en
las casas, y solo en algunas.

Y una corrección de vocabulario que importa: **en ATSC 1.0 el audio de emisión
es AC-3, no AAC**. AAC es de ATSC 3.0, de ISDB y de DVB. MPEG capa II es de DVB.
El 47 CFR 73.682 incorpora el A/52 (AC-3) y el A/53 Parte 5 lo confirma
(`docs/investigacion/BITRATE-ATSC-EEUU-2026-09-11.md`).

### Las dos causas candidatas, las dos ya identificadas hoy

| | Qué le pasa al televidente | Evidencia |
|---|---|---|
| **Audio en MPEG capa II, no AC-3** | Sintoniza pero **sin sonido**, o el televisor descarta el programa | La línea de VLC: `acodec=mpga` |
| **PSIP en blanco** | **El canal no aparece en el barrido**: sin TVCT el televisor no sabe a qué canal virtual mapearlo | Él mismo: «al no tener contenido desde el multicast lo pone en blanco» |

De fondo, además, el muestreo a 44100 Hz cuando ATSC pide 48.

**Las dos explican «no todos»**, que es la palabra clave: un televisor con
decodificador de capa II «de más» sí lo coge; uno que cae al canal físico sin
PSIP también. Los estrictos, no.

### La prueba que no cuesta nada

Se le propuso cambiar en su VLC `acodec=mpga` → **`a52`**, `samplerate` 44100 →
**48000**, `ab` 128 → **192**, y mirar si los televisores que no lo cogían ahora
sí. **Cinco minutos, reversible, y no depende de Antena787.**

- Si lo arregla → era el audio, y Antena787 lo resuelve con un desplegable:
  ya trabaja a 48 kHz y ya sabe emitir AC-3 (`internal/drivers/salida/udpts.go`).
- Si no lo arregla → es el PSIP, y entonces la guía en PMCP que se arregló hoy
  deja de ser «una función más» y pasa a ser **lo que hace que lo vean**.

### Lo que esto le hace al producto

Si la hipótesis se confirma, Antena787 deja de ser «un sustituto de VLC» para
CAtv y pasa a ser **lo que hace que sus televidentes lo vean**. Y el valor de
fábrica `mp2` de `udpts.go` —que se puso copiando lo que CAtv emite hoy— sería
copiar el defecto. **No se cambia hasta tener su respuesta**, pero queda
señalado aquí para no olvidarlo.
