# Cuánta máquina gasta de verdad un canal · 11 de septiembre de 2026

Medido, no estimado. Salió de una pregunta de Saul —«¿por qué hay acelerador
para encoding y no para decoding?»— y de una optimización que yo iba a hacer
y que **los números desmintieron antes de escribirla**.

## Con qué se midió

- **Máquina:** Apple M4 (`Mac16,1`), 10 núcleos, macOS. ffmpeg de Homebrew.
- **Material:** un archivo real de `~/antena-sombra/contenido`, *For All Mankind
  S05E09*, **HEVC 1080p** — el caso más pesado que maneja el sistema.
- **Trabajo:** 30 segundos, con los filtros de casa de verdad
  (`scale=1280:720`, `fps=60000/1001`, `format=yuv420p`, `aresample=48000`).
- **Medida:** `/usr/bin/time -p`. «CPU» es `user`; «núcleos» es `user ÷ 30 s`.

## Lo que cuesta cada etapa

| Etapa | Reloj | CPU | Núcleos |
|---|---|---|---|
| Decode, como está hoy: **dos procesos** | 1.58 s | 11.98 s | **0.40** |
| Decode, **un solo proceso con dos salidas** | 3.64 s | 14.65 s | 0.49 |
| Decode, solo el video (referencia) | 1.59 s | 11.91 s | 0.40 |
| Encode **MPEG-2 4 Mb/s 720p59.94** (al transmisor) | 0.96 s | 4.38 s | **0.15** |
| Encode **H.264 veryfast** (a internet) | 1.55 s | 9.98 s | 0.33 |

**El camino completo al transmisor —decode + encode— son 0.55 núcleos.** De diez.

## Los tres hallazgos

**1. Juntar los dos ffmpeg del decodificador habría empeorado las cosas.**
Iba a hacerlo: `decoder.go:67` abre dos procesos por clip y parecía un
desperdicio obvio. Con uno solo el reloj se va de 1.58 s a **3.64 s** y la CPU
sube. Los dos procesos corren en núcleos distintos; uno solo serializa. **Los
dos procesos son la decisión correcta aunque nadie la escribiera**, y el
comentario que lo explique hace falta para que a nadie más se le ocurra.

**2. El audio es gratis.** Solo video: 11.91 s. Video y audio en paralelo:
11.98 s. El segundo proceso no cuesta nada porque corre al lado.

**3. Encodear MPEG-2 es lo más barato de toda la cadena.** 0.15 núcleos, menos
de la mitad que el decode. **El acelerador que se construyó el 11 de septiembre
acelera el tercio más barato del trabajo.** Lo que está caro es el decode, y es
justo lo que quedó sin tocar (`internal/engine/decoder.go`: cero `-hwaccel`).

## Qué se hace con esto

**A estas cifras, la aceleración por hardware resuelve un problema que este
sistema no tiene.** Medio núcleo de diez no justifica meter dependencias de
tarjeta de video, caminos distintos por plataforma, ni un modo más que probar.

**Lo que NO invalida esto:**

- **La máquina de la medición no es la de la torre.** Un M4 de 10 núcleos no es
  un PC con Windows 10 del que no sabemos ni el procesador. La **proporción**
  entre las etapas se mantiene; los números absolutos no. **Hay que preguntarle
  a Rolando qué procesador tiene esa máquina** antes de dar esto por cerrado.
- **Un canal no es un archivo.** Esto mide una cadena. Varias salidas a la vez,
  la normalización de la cola corriendo al mismo tiempo y el detector de
  silencio suman. Falta medir el proceso entero al aire durante una hora, que
  es la prueba de resistencia de T10.
- **El acelerador construido no sobra.** Sigue siendo lo que hace falta para
  F2-11 —relanzar con el mismo, caer a software— y para una estación con una
  máquina mucho más floja que ésta. Lo que cambia es su **prioridad**: deja de
  ser urgente.

## Lo que se deja escrito para no repetirlo

Antes de construir aceleración por hardware para el decoding, **medir en la
máquina de la torre**. Si ahí también son décimas de núcleo, no se construye:
se escribe por qué no, y se cierra. Una función que no hace falta cuesta para
siempre.
