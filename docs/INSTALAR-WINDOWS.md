# Antena787 en Windows — cómo ponerlo a andar

Esto no es un instalador de los de siguiente-siguiente-siguiente. **Es un solo
programa.** No toca el registro de Windows, no pide permisos de administrador,
no instala servicios, y para quitarlo se borra la carpeta.

Eso es a propósito: en una estación, lo que se puede borrar y volver a poner en
un minuto da menos miedo que lo que se mete por dentro del sistema.

---

## Antes de empezar: te hace falta ffmpeg

Antena787 no comprime video por su cuenta — **usa ffmpeg**, que es el programa
que ya usan casi todas las estaciones del mundo por debajo, incluido VLC.

Si no lo tienes:

1. Ve a **https://www.gyan.dev/ffmpeg/builds/** y baja el que dice
   **«release essentials»** (un `.7z` o `.zip`).
2. Descomprímelo en `C:\ffmpeg`.
3. Dentro va a haber una carpeta `bin` con `ffmpeg.exe` y `ffprobe.exe`.

**Cómo saber si ya lo tienes:** abre el símbolo del sistema (busca `cmd` en el
menú de inicio) y escribe:

```
ffmpeg -version
```

Si contesta con un montón de texto, ya está. Si dice que no reconoce el
comando, hay que bajarlo.

> Antena787 lo busca solo en los sitios normales. Si no lo encuentra, te lo
> dice en la pantalla en vez de fallar raro, y puedes decirle dónde está.

---

## Ponerlo a andar

1. **Descomprime la carpeta `Antena787-windows`** donde quieras. En el
   escritorio está bien. En `C:\Antena787` está mejor, porque una ruta corta da
   menos problemas.

2. **Doble clic en `Arrancar-Antena787.bat`.**

   Se abre una ventana negra con letras. **Esa ventana es el programa: no la
   cierres mientras el canal esté al aire.** Se puede minimizar.

3. **Abre el navegador** —Chrome o Edge— y entra a:

   ```
   http://127.0.0.1:7870
   ```

4. **La primera vez te va a llevar por nueve preguntas.** Son de una en una,
   en español, y ninguna te pide saber qué es un códec. Ponle una clave a la
   estación cuando te la pida: es la que te va a pedir para entrar después.

Ya está. No hay paso 5.

---

## Lo que Windows te puede decir, y qué hacer

**«Windows protegió tu PC»** — sale en azul con un botón de «No ejecutar».

Es porque el programa no está firmado con un certificado, que cuesta unos
cientos de dólares al año. Haz clic en **«Más información»** y después en
**«Ejecutar de todas formas»**.

Si prefieres no fiarte de eso —y haces bien en preguntar—, el código está
completo y público en **github.com/Saul-Punybz/antena787**: se puede leer, y
cualquiera puede compilarlo él mismo y comparar.

**El cortafuegos pregunta si permites el acceso** — sale la primera vez que
el canal manda señal.

Dale a **permitir en redes privadas**. Es lo que deja que la señal salga hacia
tu multiplexor y que puedas ver el canal desde otra computadora de la torre.
**No hace falta permitirlo en redes públicas.**

---

## Dónde queda todo

Al lado del ejecutable se crea una carpeta **`datos`**. Ahí dentro está todo:
la base de datos del canal, los respaldos de cada hora, y la llave que cifra
las claves de tus proveedores.

**Copiar esa carpeta es copiar el canal entero.** Y si algún día quieres
llevarlo a otra computadora, se copia y ya — con una advertencia: la llave está
atada a tu cuenta de Windows, así que **las claves guardadas de proveedores hay
que volver a escribirlas** en la máquina nueva. Todo lo demás viaja.

---

## Para que siga andando solo

Antena787 **impide que la máquina se duerma mientras el canal está al aire**,
así que eso no hay que tocarlo. Lo que sí conviene, y es de Windows, no nuestro:

- **Que arranque solo después de un corte de luz.** En el BIOS de la máquina
  suele haber una opción tipo «Restore on AC Power Loss» → ponla en «Power On».
- **Que Antena787 arranque con Windows.** Pulsa `Windows + R`, escribe
  `shell:startup`, y arrastra ahí un acceso directo de `Arrancar-Antena787.bat`.
- **Que Windows no reinicie solo** para actualizarse a mitad de una película.
  En Configuración → Windows Update → Horas activas, pon el horario en que
  estás al aire.

---

## Si algo no funciona

**La ventana negra se cierra sola.** Vuelve a abrirla y lee lo último que dice
antes de cerrarse: ahí está el motivo. El `.bat` está hecho para que la ventana
se quede abierta justo para eso.

**El navegador no abre nada.** Comprueba que la ventana negra siga abierta.
Si lo está, prueba con `http://localhost:7870`.

**El puerto 7870 ya lo usa otro programa.** Cambia la última línea del `.bat`
por: `antena.exe -datos "datos" -escucha 127.0.0.1:7871` y entra por el 7871.

**Cualquier otra cosa:** dentro del programa, la pantalla **Al aire** tiene
abajo una **bitácora** que dice qué hizo el sistema y por qué, en español. Casi
siempre la respuesta está ahí. Si no, mándame una foto de esa pantalla.

---

## Lo que todavía NO hace, para que no te lleves una sorpresa

Esto es una versión para probar, no para poner el canal encima todavía.

- **No graba lo que salió al aire.** Se está construyendo.
- **No se puede tomar el control a mano** para meter algo ahora mismo: el botón
  está pero todavía no hace nada.
- **No pone el logo del canal** en la señal.
- **No hay pantalla de anuncios ni cobros.** Es lo último del plan, a propósito:
  de nada sirve poder venderle a un cliente si el aire todavía se te puede caer.
- **Nada de esto se ha corrido en Windows durante horas.** Se probó entero en
  un Mac y las pruebas automáticas sí corren en Windows, pero **tú vas a ser el
  primero en tenerlo andando de verdad en una máquina como la tuya.** Si algo
  se rompe, no es que lo hayas roto: es que ahí no había mirado nadie.
