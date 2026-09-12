# La cadena entera, corrida de verdad · 12 de septiembre de 2026

Primera vez que el camino completo se corre junto, con procesos de verdad y
sin simular nada. Hasta ahora cada pieza tenía sus pruebas y se había abierto
por separado; **juntas, nunca**.

## El montaje

1. **Una señal en vivo de verdad**: `ffmpeg` generando HLS —`testsrc2` 1280×720
   a 30 fps con un tono de 440 Hz— servido por HTTP en `127.0.0.1:8090`. No es
   un archivo haciéndose pasar por un vivo: es HLS con segmentos que se crean y
   se borran mientras corre.
2. **Antena787** sobre una copia de la base de sombra real, que subió de la
   versión 10 a la 11 sola al abrir.
3. **Dos salidas**: `udp-ts` MPEG-2 a `127.0.0.1:12345` —el sitio del
   multiplexor— y `http-ts` H.264 en el `:8099`, que es el monitor.
4. **Una regla en vivo** que pone esa señal al aire.

## Qué pasó, paso por paso

| | Resultado |
|---|---|
| Probar la señal **antes** de guardarla | `h264 1280x720 a 30/1 · 1 canal · en vivo (sin final)` |
| Guardarla | «se va a buscar a http://127.0.0.1:8090/vivo.m3u8» |
| Las seis comprobaciones de la puerta del aire | **Pasa**, con un aviso honesto de que no hay relleno guardado |
| Salir al aire | «el canal está al aire: la señal empieza a salir hacia el equipo configurado» |
| **Lo que recibe el multiplexor** | **`mpeg2video 1280x720 @59.94 · mp2 48000 Hz 2 canales · mpegts 4928 kb/s`** |
| **Lo que recibe el navegador** | **`h264 1280x720 · aac 48000 Hz`**, 397 KB en 6 s |
| ¿Quién está al aire? | **`en vivo · live_source`**, sin incidente `vivo_ausente` |
| **El cuadro capturado del UDP** | El patrón del vivo con su contador `00:04:13.567` — **no el cartel** |

Ese último renglón es el que cierra la pregunta que se arrastraba desde el
principio: **lo que sale por el cable al multiplexor es la señal remota**, no
un relleno ni una simulación.

Y un detalle que vale la pena: el audio sale a **48 000 Hz**, que es lo que
ATSC pide. CAtv emite hoy a 44 100 con VLC
(`docs/equipos/CADENA-CATV-2026-09-11.md`).

## Lo que la pasada encontró, y no habría encontrado ninguna prueba

**1 · El binario estaba viejo.** Se compiló y se probó el monitor, pero no se
rehízo `bin/antena`, así que `/estado` no traía el campo `monitor` y la
pantalla no tenía de dónde sacar nada. Error de proceso, no de código: **una
pasada de punta a punta empieza por reconstruir el binario**, y queda dicho.

**2 · Un servidor de pruebas ocupando el puerto del monitor.** El servidor
estático que se había levantado para las capturas tenía el `:8099`. Con él
vivo, `curl` al monitor devolvía un 404 de Python en vez de la señal.

Eso llevó a un **diagnóstico equivocado**: se anotó como «la salida dice
conectada con el puerto ocupado». Al revisarlo, `salidaNoAbre`
(`internal/app/salidas.go`) **sí** deja la salida en `apagada`, con su motivo y
su incidente `enlace_caido`. La ruta del código está bien y el fallo no se pudo
reproducir. Lo más probable es que el motor reabriera las salidas en un
relanzamiento justo después de liberar el puerto.

**Queda como sospecha, no como hallazgo.** No se apunta un fallo que no se
puede demostrar.

**3 · Un aviso operativo que sí es real:** el monitor escucha en todas las
direcciones (`:puerto`). Si otro programa de esa misma máquina ocupa el puerto
antes, el canal se entera y lo dice — pero conviene escoger un puerto que no
use nadie más en el PC de la torre.

## Lo que NO se probó aquí, dicho en voz alta

- **Que la señal aguante horas.** Esto corrió minutos. La prueba de
  resistencia es T10.
- **Que reconecte de verdad cuando la señal remota se cae.** La lógica tiene
  sus pruebas, pero aquí no se cortó el HLS a propósito para verlo.
- **El navegador pintando el monitor.** Se comprobó que el flujo es H.264 +
  AAC y que `mpegts.js` sabe pintarlo; abrirlo en un navegador de verdad, no.
- **Nada en Windows.** Todo esto es un Mac.
