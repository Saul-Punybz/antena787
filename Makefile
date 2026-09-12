# Antena787. Nada de esto hace magia: son los mismos comandos de
# docs/DESARROLLO.md.
#
# Para entregarle algo a una estacion: `make paquete-windows`.

BIN  ?= bin
DIST ?= dist
GO   ?= go

.PHONY: build vet test f0 windows linux arm64 clean ui antena \
	antena-windows paquete-windows paquete-mac paquete-linux paquetes

## build — compila el ejecutable de la F0 en bin/
build:
	$(GO) build -o $(BIN)/f0 ./cmd/f0

## vet — analisis estatico; tiene que salir limpio
vet:
	$(GO) vet ./...

## test — las pruebas
test:
	$(GO) test ./...

## f0 — corrida corta de verificacion: 0.1 h (6 minutos), no la de 8 horas.
##      Ocupa disco y tarda; la corrida real es: bin/f0 all -hours 8
f0: build
	$(BIN)/f0 media
	$(BIN)/f0 run -hours 0.1
	$(BIN)/f0 analyze

## windows / linux / arm64 — compilacion cruzada del ARNES de medicion.
## Para el PRODUCTO son los objetivos `paquete-*` de mas abajo: durante
## meses estos tres fueron lo unico que existia, y cmd/antena no se
## compilaba nunca fuera de un Mac. Que el nombre no vuelva a enganar.
windows:
	GOOS=windows GOARCH=amd64 $(GO) build -o $(DIST)/f0-windows-amd64.exe ./cmd/f0

linux:
	GOOS=linux GOARCH=amd64 $(GO) build -o $(DIST)/f0-linux-amd64 ./cmd/f0

arm64:
	GOOS=linux GOARCH=arm64 $(GO) build -o $(DIST)/f0-linux-arm64 ./cmd/f0

## clean — borra los binarios. NO toca f0/media ni f0/out: son 48 GB que
##         cuestan una hora de rehacer, y puede haber una corrida en curso.
##         Para eso: rm -rf f0/media f0/out
clean:
	rm -rf $(BIN) $(DIST)

## ui: compila la interfaz (web/) y la deja donde el binario la embebe
ui:
	npm --prefix web ci
	npm --prefix web run build
	rm -rf internal/api/ui/dist && cp -R web/dist internal/api/ui/dist

## antena: el binario completo con la interfaz dentro
antena: ui
	go build -o bin/antena ./cmd/antena


## antena-windows — el producto entero para Windows x86-64, con la interfaz
##                  dentro. Es lo que se le manda a una estacion.
##
## Sin CGo a proposito: asi el .exe no necesita nada instalado en la maquina
## de destino mas que ffmpeg, y corre en un Windows 10 tal como viene.
antena-windows: ui
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build \
		-ldflags "-s -w" -o $(DIST)/antena.exe ./cmd/antena

## paquete-windows — la carpeta que se le entrega: el ejecutable, la guia y
##                   un arranque de un clic. Sin instalador de los de
##                   siguiente-siguiente-siguiente: este programa no necesita
##                   tocar el registro ni pedir permisos de administrador.
paquete-windows: antena-windows
	rm -rf $(DIST)/Antena787-windows
	mkdir -p $(DIST)/Antena787-windows
	cp $(DIST)/antena.exe $(DIST)/Antena787-windows/
	cp docs/INSTALAR-WINDOWS.md $(DIST)/Antena787-windows/LEEME.md
	cp scripts/Arrancar-Antena787.bat $(DIST)/Antena787-windows/
	cp LICENSE $(DIST)/Antena787-windows/
	cd $(DIST) && zip -qr Antena787-windows.zip Antena787-windows
	@echo "Listo: $(DIST)/Antena787-windows.zip"


## paquete-mac — el producto para Mac, Apple Silicon e Intel en un solo
##               ejecutable. Se entrega igual que el de Windows: se
##               descomprime y se abre.
paquete-mac: ui
	rm -rf $(DIST)/Antena787-mac
	mkdir -p $(DIST)/Antena787-mac
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -ldflags "-s -w" -o $(DIST)/antena-arm64 ./cmd/antena
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -ldflags "-s -w" -o $(DIST)/antena-amd64 ./cmd/antena
	lipo -create -output $(DIST)/Antena787-mac/antena $(DIST)/antena-arm64 $(DIST)/antena-amd64 \
		|| cp $(DIST)/antena-arm64 $(DIST)/Antena787-mac/antena
	rm -f $(DIST)/antena-arm64 $(DIST)/antena-amd64
	cp docs/INSTALAR-MAC-LINUX.md $(DIST)/Antena787-mac/LEEME.md
	cp scripts/arrancar-antena787.sh $(DIST)/Antena787-mac/
	chmod +x $(DIST)/Antena787-mac/arrancar-antena787.sh
	cp LICENSE $(DIST)/Antena787-mac/
	@echo "Listo: $(DIST)/Antena787-mac"

## paquete-linux — el producto para Linux, x86-64 y ARM64 (una Raspberry
##                 grande o un servidor chico de la estacion).
paquete-linux: ui
	rm -rf $(DIST)/Antena787-linux
	mkdir -p $(DIST)/Antena787-linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -ldflags "-s -w" -o $(DIST)/Antena787-linux/antena-amd64 ./cmd/antena
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -ldflags "-s -w" -o $(DIST)/Antena787-linux/antena-arm64 ./cmd/antena
	cp docs/INSTALAR-MAC-LINUX.md $(DIST)/Antena787-linux/LEEME.md
	cp scripts/arrancar-antena787.sh $(DIST)/Antena787-linux/
	chmod +x $(DIST)/Antena787-linux/arrancar-antena787.sh
	cp LICENSE $(DIST)/Antena787-linux/
	@echo "Listo: $(DIST)/Antena787-linux"

## paquetes — los tres de una vez.
paquetes: paquete-windows paquete-mac paquete-linux
