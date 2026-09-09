# Antena787 — Fase 0. El producto (cmd/antena) todavia no existe.
# Nada de esto hace magia: son los mismos comandos de docs/DESARROLLO.md.

BIN  ?= bin
DIST ?= dist
GO   ?= go

.PHONY: build vet test f0 windows linux arm64 clean

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

## windows — compilacion cruzada, Windows x86-64
windows:
	GOOS=windows GOARCH=amd64 $(GO) build -o $(DIST)/f0-windows-amd64.exe ./cmd/f0

## linux — compilacion cruzada, Linux x86-64
linux:
	GOOS=linux GOARCH=amd64 $(GO) build -o $(DIST)/f0-linux-amd64 ./cmd/f0

## arm64 — compilacion cruzada, Linux ARM64
arm64:
	GOOS=linux GOARCH=arm64 $(GO) build -o $(DIST)/f0-linux-arm64 ./cmd/f0

## clean — borra los binarios. NO toca f0/media ni f0/out: son 48 GB que
##         cuestan una hora de rehacer, y puede haber una corrida en curso.
##         Para eso: rm -rf f0/media f0/out
clean:
	rm -rf $(BIN) $(DIST)
