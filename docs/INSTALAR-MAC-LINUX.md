# Antena787 en Mac y en Linux

Es un solo programa. No instala nada, no pide permisos de administrador, no
deja servicios corriendo, y para quitarlo se borra la carpeta.

---

## Lo único que hace falta: ffmpeg

A diferencia del paquete de Windows —donde ffmpeg va dentro porque bajarlo
a mano es donde se pierde una tarde— aquí se instala con una línea:

```
# Mac
brew install ffmpeg

# Debian / Ubuntu / Raspberry Pi OS
sudo apt install ffmpeg

# Fedora
sudo dnf install ffmpeg
```

Si prefieres no instalarlo en el sistema, pon `ffmpeg` y `ffprobe` **dentro de
esta misma carpeta**: Antena787 los busca a su lado antes que en el sistema.

---

## Ponerlo a andar

```
./arrancar-antena787.sh
```

Y en el navegador: **http://127.0.0.1:7870**

La primera vez te lleva por nueve preguntas, de una en una, en español.
Ninguna te pide saber qué es un códec.

**No lo pongas en una carpeta que se sincronice sola** —iCloud Drive, Dropbox,
Google Drive—. La base de datos del canal se escribe todo el tiempo y un motor
de sincronización copiando archivos vivos la corrompe. Una carpeta normal en tu
casa (`~/antena787`) está bien.

---

## En Mac: «no se puede abrir porque es de un desarrollador no identificado»

Sale porque el programa no está firmado con Apple, que cuesta cien dólares al
año. La primera vez:

**Clic derecho sobre el ejecutable → Abrir → Abrir.** Solo hace falta una vez.

O desde la terminal: `xattr -d com.apple.quarantine ./antena`

El código está completo y público en **github.com/Saul-Punybz/antena787**:
quien no quiera fiarse puede leerlo y compilarlo él mismo.

---

## Para que siga andando solo

**En Linux, con systemd** — crea `~/.config/systemd/user/antena787.service`:

```ini
[Unit]
Description=Antena787
After=network-online.target

[Service]
WorkingDirectory=%h/antena787
ExecStart=%h/antena787/antena -datos %h/antena787/datos
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
```

Y después:

```
systemctl --user enable --now antena787
loginctl enable-linger $USER     # que siga corriendo sin sesión abierta
```

Esa última línea importa: sin ella, el canal se apaga cuando cierras sesión.

**En Mac**, lo más simple es dejar la terminal abierta. Si hace falta que
arranque solo, un `launchd` en `~/Library/LaunchAgents/` hace lo mismo.

---

## Dónde queda todo

En la carpeta `datos`, junto al ejecutable: la base del canal, los respaldos de
cada hora y la llave que cifra las claves de tus proveedores.

**Copiar esa carpeta es copiar el canal entero.**

Una advertencia sobre esa llave: en Mac y en Linux **solo la protegen los
permisos del archivo** (`0600`). En Windows va atada a la cuenta con DPAPI, que
es más fuerte. Aquí, cualquiera que pueda leer tu carpeta personal puede leer
las claves de proveedores guardadas. Se dice para que lo sepas, no para
asustar: es la misma protección que tienen tus llaves de SSH.

---

## Lo que todavía NO hace

Lo mismo que en Windows, y por las mismas razones:

- **No graba lo que salió al aire.** Se está construyendo.
- **No pone el logo del canal** en la señal.
- **No hay anuncios ni cobros.** Es lo último del plan a propósito: de nada
  sirve poder venderle a un cliente si el aire todavía se te puede caer.
