# Grabaciones de SAME para las pruebas

**Aquí no hay ninguna grabación real todavía, y eso es a propósito hasta que
haya una con procedencia clara.**

Todas las pruebas del paquete corren con audio fabricado por el modulador de
`modulador_test.go` (que vive solo en las pruebas: ver el aviso de ese archivo y
el ADR 0010). Eso prueba el algoritmo contra el protocolo tal como lo escribe
47 CFR 11.31, y mide hasta qué relación señal/ruido aguanta, pero **no prueba
contra las mañas de un ENDEC de verdad**: el nivel con que sale, el filtro del
transmisor, el recorte del compresor de audio de la estación, un preámbulo más
corto o más largo de lo que dice la norma.

Buscando una el 11 de septiembre de 2026 no se consiguió ninguna con licencia
limpia y verificable en el tiempo de la sesión. Lo que sí sirve, cuando aparezca:

- **Una prueba semanal (RWT) grabada del retorno de aire de CAtv.** Es la mejor
  de todas, porque es el equipo real, la cadena real y la estación real. La
  prueba sale todas las semanas; solo hay que grabar el retorno unos minutos.
- **Audio de la radio meteorológica de NOAA (NWR) publicado por el gobierno
  federal de Estados Unidos.** Una obra del gobierno federal es dominio público,
  así que se puede guardar aquí; hay que dejar el enlace exacto y la fecha.

## Cómo agregarla

1. Guardar el audio como **PCM s16le, mono, 48 000 muestras por segundo, sin
   cabecera** (extensión `.s16`). Con ffmpeg:

   ```
   ffmpeg -i grabacion.wav -f s16le -acodec pcm_s16le -ac 1 -ar 48000 rwt-catv.s16
   ```

2. Al lado, un archivo `.txt` con el mismo nombre y **la cabecera que se espera
   leer**, una por línea, tal como debe salir en `Crudo`. Por ejemplo:

   ```
   ZCZC-EAS-RWT-072000+0015-2531200-WPRT/TV -
   ```

3. Anotar abajo de dónde salió, con fecha y licencia.

`TestGrabacionReal` recoge solo lo que encuentre: si no hay archivos, se salta.

## Procedencia de lo que haya aquí

| Archivo | De dónde salió | Licencia | Fecha |
|---|---|---|---|
| _(ninguno todavía)_ | | | |
