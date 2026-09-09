# Desarrollo

Cómo montar el entorno, compilar y correr lo que hay hoy. Para las
convenciones —idiomas, commits, DCO, política de IA— ver
[`CONTRIBUTING.md`](../CONTRIBUTING.md).

> **Estado.** El repositorio está en la **Fase 0**: el experimento del motor
> (PRD §22.1). El ejecutable del producto, `cmd/antena`, **todavía no
> existe**. Lo único que compila y corre es `cmd/f0`.

---

## 1 · Lo que hace falta

| | Versión | Para qué |
|---|---|---|
| **Go** | **1.26 o más nuevo** | Es lo que declara `go.mod`. Todo el proyecto es Go puro |
| **ffmpeg** y **ffprobe** | reciente (probado con la serie 9.0) | La única dependencia externa. El motor decodifica, codifica y mide con ellos |
| **make** | cualquiera | Comodidad, no requisito: el `Makefile` no hace magia |

Nada más. **No hace falta Docker, ni Node, ni un compilador de C** — y no va
a hacer falta: nada de CGo, jamás (ADR
[0002](adr/0002-no-cgo.md)). Node aparecerá cuando exista `web/`, y solo en
tiempo de compilación.

### Instalar ffmpeg

**macOS** (Homebrew):

```sh
brew install ffmpeg
```

**Linux** (Debian, Ubuntu y derivados):

```sh
sudo apt update && sudo apt install ffmpeg
```

En Fedora: `sudo dnf install ffmpeg` (con RPM Fusion habilitado). En Arch:
`sudo pacman -S ffmpeg`.

**Windows**: bajar una build estática de **BtbN**
(<https://github.com/BtbN/FFmpeg-Builds/releases>), la variante
`win64-gpl`, descomprimirla y poner su carpeta `bin` en el PATH — o
apuntarle con `ANTENA_FFMPEG` (abajo). Es la misma fuente que el instalador
del producto va a empaquetar: **una versión exacta por release**.

### Si no quieres tocar el PATH

El motor busca `ffmpeg` y `ffprobe` en este orden:

1. La variable de entorno **`ANTENA_FFMPEG`** — la carpeta donde están los
   dos binarios.
2. **Junto al ejecutable** que se está corriendo. Es como los encuentra el
   producto instalado.
3. El **PATH**.

```sh
export ANTENA_FFMPEG=/ruta/a/ffmpeg/bin     # macOS y Linux
```

```powershell
$env:ANTENA_FFMPEG = "C:\ffmpeg\bin"        # Windows, PowerShell
```

### Instalar Go

Desde <https://go.dev/dl/>, o `brew install go` en macOS. Verificar:

```sh
go version      # debe decir go1.26 o más nuevo
```

---

## 2 · Compilar

```sh
git clone <el repositorio>
cd antena787
go build ./...              # compila todo
go build -o bin/f0 ./cmd/f0 # el ejecutable de la F0
```

O con el `Makefile`:

```sh
make build      # compila cmd/f0 en bin/
make vet        # go vet ./...
make test       # go test ./...
```

`bin/` y `dist/` están en el `.gitignore`: los binarios no se versionan.

---

## 3 · Antes de cada PR

```sh
gofmt -l .        # no debe imprimir nada; si imprime, gofmt -w esos archivos
go vet ./...      # limpio
go test ./...     # verde
```

`make vet test` hace las dos últimas. El CI corre exactamente esto en cada
push y cada PR, más la compilación cruzada. **Que `go vet` esté limpio es
requisito, no recomendación.**

---

## 4 · Correr la Fase 0

La F0 responde una pregunta y da un resultado de pase o fallo: *¿el servidor
de cuadros en Go entre decodificadores por clip y un encoder persistente
produce salida continua y limpia durante horas en hardware modesto?*

```sh
bin/f0 media                 # fabrica los archivos de prueba en f0/media (≈1 min)
bin/f0 run -hours 8          # corre el motor; escribe f0/out/
bin/f0 analyze               # mide y escribe f0/out/REPORTE.md
```

`bin/f0 all -hours 8` hace las tres seguidas.

| Bandera | Qué hace |
|---|---|
| `-hours N` | Duración de la corrida. Acepta decimales: `-hours 0.1` son 6 minutos |
| `-udp udp://IP:PUERTO` | Manda además el TS MPEG-2 al multiplexor de verdad |
| `-one` | Una sola salida en vez de dos, para medir el CPU con una y con dos |

Para una pasada rápida que solo verifica que todo funciona:

```sh
make f0        # media + run de 0.1 h + analyze
```

**La corrida completa de 8 horas ocupa unos 48 GB** — unos 36 del transport
stream a 10 Mb/s y unos 12 de la salida web. `f0/media/` y `f0/out/` se
generan y **no se versionan**.

**La F0 se corre en la máquina de destino**, no en la laptop del
desarrollador: el punto es medir un Windows 10 modesto, que es donde va a
vivir esto. Qué se mide exactamente, cómo, y qué la F0 deliberadamente **no**
prueba, está en [`f0/README.md`](../f0/README.md).

En Windows:

```powershell
go build -o f0.exe .\cmd\f0
.\f0.exe all -hours 8
```

---

## 5 · Compilar para otras plataformas

Compilación cruzada trivial es la razón principal por la que se escogió Go, y
funciona porque **no hay CGo en ninguna parte**. Desde cualquier máquina:

```sh
GOOS=windows GOARCH=amd64 go build -o dist/f0-windows-amd64.exe ./cmd/f0
GOOS=linux   GOARCH=amd64 go build -o dist/f0-linux-amd64      ./cmd/f0
GOOS=linux   GOARCH=arm64 go build -o dist/f0-linux-arm64      ./cmd/f0
```

O con el `Makefile`: `make windows`, `make linux`, `make arm64`.

macOS ARM (`GOOS=darwin GOARCH=arm64`) compila igual y sirve para
desarrollar; **no es una plataforma de destino** — el producto corre en
Windows, Linux y ARM.

Ojo: el binario cruza, **ffmpeg no**. Cada plataforma necesita el suyo, y en
el producto lo pone el instalador junto al ejecutable.

---

## 6 · Dónde vive cada cosa

### Hoy

```
cmd/f0/main.go            el ejecutable de la F0: media | run | analyze | all

internal/engine/          EL MOTOR — revisión humana línea por línea, siempre
  ffmpeg.go                 encuentra ffmpeg y ffprobe (ANTENA_FFMPEG, junto al binario, PATH)
  format.go                 el formato de casa: resolución, cuadros, audio
  decoder.go                un ffmpeg por clip → video crudo y PCM
  frameserver.go            el servidor de cuadros: conformado, pre-roll, eventos
  encoder.go                el encoder persistente y sus salidas

internal/f0/
  media.go                  fabrica los archivos de prueba, con marcador y pitido
  analyze.go                mide la salida cuadro a cuadro y escribe el reporte
  stats.go                  CPU y RAM de todos los procesos, cada 10 s

internal/ts/ts.go         lee el transport stream paquete a paquete: continuidad, PCR, tasa

f0/README.md              qué mide la F0 y qué no
f0/media/  f0/out/        se generan; no se versionan

docs/adr/                 las decisiones y su porqué (en inglés)
docs/ACEPTACION.md        los criterios de aceptación
docs/ROADMAP.md           las fases y dónde estamos
diseno/                   mockups de pantallas; no es código del producto
```

### Lo que va a existir, y ya está decidido *(PRD §14.1)*

```
cmd/antena/               el ejecutable del producto — TODAVÍA NO EXISTE
internal/engine           el motor (ya existe, en su forma de F0)
internal/resolver         de reglas a plan, 48 horas por adelantado
internal/ingest           entrada de archivos, medición, normalización
internal/drivers/         output · input · alert · billing · metadata · overlay
internal/store            SQLite (modernc.org/sqlite, WAL, sin CGo) y migraciones
web/                      React y TypeScript, embebido con go:embed
```

**Un paquete por concepto del glosario**, y los conceptos son los de
[`CONTEXT.md`](../CONTEXT.md).

Y el producto es **un solo proceso**: servidor web, resolver y motor como
goroutines dentro del mismo binario. Cada goroutine crítica atrapa su propio
pánico, registra un incidente y se relanza sola; el proceso corre bajo el
supervisor del sistema —servicio de Windows o systemd— con reinicio
automático. Aislar el motor en otro proceso se reconsidera **solo** si la
prueba de resistencia de 30 días muestra que un fallo del servidor web tumba
el aire.

---

## 7 · Dos cosas que se aprendieron construyendo la F0

Están en [`f0/README.md`](../f0/README.md), pero se repiten aquí porque
cuestan medio día si se descubren solas:

- **ffmpeg con un solo `filter_complex` que mezcla audio y video se traba** al
  tercer cuadro: pide las entradas por marca de tiempo y espera al audio con
  el video bloqueado. Con filtros por salida (`-filter:a:0`) no pasa. El
  encoder se arma así a propósito.
- **ffmpeg no abre su segunda entrada hasta haber leído algo de la primera.**
  El encoder acepta la conexión de audio aparte y guarda lo que llegue antes.

---

## 8 · Antes de tocar `internal/engine`

Es el 20% del código y el 95% del riesgo. Un motor de playout no falla con un
error 500 — **falla a las 3:14 AM de un martes, en silencio.**

- **Se lee línea por línea por un humano**, siempre, sin excepción, venga de
  donde venga (PRD §16, ADR [0005](adr/0005-agpl-with-dco.md)).
- La plantilla de PR tiene una casilla para eso. Márcala.
- Los errores que importan aquí no se ven en una corrida de cinco minutos:
  deriva de marcas de tiempo a las 40 horas, contadores que se desbordan,
  casos borde de horario de verano. Si tu cambio toca el reloj o el conteo de
  cuadros, corre la F0 completa y adjunta el `REPORTE.md`.
