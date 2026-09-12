#!/bin/sh
# Arrancar Antena787 en Mac o en Linux.
#
# No instala nada, no pide permisos de administrador y no toca el sistema:
# es un solo ejecutable que guarda lo suyo en la carpeta `datos` de al lado.

cd "$(dirname "$0")" || exit 1

# Los datos van junto al ejecutable a propósito: la estación sabe dónde está
# todo lo suyo, y copiar esa carpeta es copiar el canal entero.
mkdir -p datos

# En Linux se entregan dos ejecutables, uno por tipo de procesador.
exe=./antena
if [ ! -x "$exe" ]; then
  case "$(uname -m)" in
    x86_64|amd64) exe=./antena-amd64 ;;
    aarch64|arm64) exe=./antena-arm64 ;;
  esac
fi
if [ ! -x "$exe" ]; then
  echo "No encuentro el ejecutable de Antena787 en esta carpeta." >&2
  exit 1
fi

# ffmpeg: en Mac y en Linux casi siempre está instalado o se instala con una
# línea, así que aquí no va dentro del paquete como en Windows. Se avisa antes
# de arrancar, que es mejor que fallar después sin explicar.
if ! command -v ffmpeg >/dev/null 2>&1 && [ ! -x ./ffmpeg ]; then
  echo
  echo "  Falta ffmpeg, que es lo que comprime el video."
  echo
  echo "  En Mac:            brew install ffmpeg"
  echo "  En Debian/Ubuntu:  sudo apt install ffmpeg"
  echo "  En Fedora:         sudo dnf install ffmpeg"
  echo
  echo "  O pon ffmpeg y ffprobe en esta misma carpeta."
  echo
fi

echo
echo "  Antena787 arrancando..."
echo "  Cuando diga que está listo, abre en el navegador:"
echo
echo "      http://127.0.0.1:7870"
echo
echo "  Para parar: Ctrl-C."
echo

exec "$exe" -datos datos
