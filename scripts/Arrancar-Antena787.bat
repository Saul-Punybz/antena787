@echo off
rem Arrancar Antena787. Doble clic y ya.
rem
rem No hace falta ser administrador y no se toca el registro de Windows:
rem este programa es un solo ejecutable que guarda todo lo suyo en la
rem carpeta de datos que se le diga.

setlocal
cd /d "%~dp0"

rem Los datos van al lado del ejecutable a proposito: asi la estacion sabe
rem donde esta todo lo suyo, y copiar esa carpeta es copiar el canal entero.
if not exist "datos" mkdir "datos"

echo.
echo   Antena787 arrancando...
echo   Cuando diga que esta listo, abre esta direccion en el navegador:
echo.
echo       http://127.0.0.1:7870
echo.
echo   Para parar: cierra esta ventana, o Ctrl-C.
echo.

antena.exe -datos "datos"

rem Si se cierra solo, que la ventana se quede: si no, nadie llega a leer
rem por que fue.
echo.
echo   Antena787 se detuvo. Lo de arriba dice por que.
pause
