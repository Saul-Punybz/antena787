# Auditoría de lógica del resolver — 12 de septiembre de 2026

Alcance: `internal/resolver/` y `internal/app/resolve.go`, más la parte de
`internal/app/motor.go` que elige el bloque (`queToca`/`corteDe`). Cada
hallazgo se verificó leyendo el código y, los dos primeros, con una prueba
puntual corrida y borrada después (el árbol de trabajo queda igual que antes).

## Hallazgos, de más a menos grave

1. **`internal/resolver/resolver.go:598-605` y `:681-683`** → una repetición
   de una repetición no repite el episodio real. `airedToday` solo se anota
   para reglas sin `repite_a` (línea 682), así que si C repite a B y B repite
   a A, C nunca encuentra lo que puso B ese día y cae a `pickEpisodes` con el
   contador de B, que nunca se usa y por eso queda desfasado. Probado: A va
   por el episodio 6 de su serie, B lo repite bien (6), C sale con el
   episodio 1. De paso escribe `ultimo_episodio_emitido` en B, que no debería
   tener contador propio. `ruleProblem` (líneas 356-360) no rechaza un
   `repite_a` que apunte a otra regla que también repite.
2. **`internal/resolver/resolver.go:960-968`** → `expiryWarnings` no
   comprueba que la regla ya esté corriendo (`hoy >= fecha_inicio`), solo mira
   `fecha_fin`. Con una regla del 1 al 5 de octubre evaluada el 7 de
   septiembre, avisa "quedan 28 días" antes de que la regla haya salido al
   aire ni una vez.
3. **`internal/resolver/resolver.go:408-419`** → el aviso de solape sin
   relevo solo nombra `group[0]` y `group[1]`; con tres o más reglas a la
   misma hora y ningún relevo único, el texto calla la tercera en adelante
   (el ganador, el de menor id, sí sale correcto).

## Revisado, sin problema

Bordes del día de emisión (`instantOf`/`Channel.BroadcastDay`,
`model.go:126-140`, consistentes con `hora_inicio_dia_emision`); horario de
verano (`localInstant`, `resolver.go:270-292`, probado en el salto y el
retroceso de Nueva York — el bloque que busca "la primera de las dos" es
redundante porque `time.Date` de Go ya devuelve la primera ocurrencia, pero el
resultado que sale es el correcto); solapes entre bloques nuevos
(`resolveHardEnds` acota cada corrida al arranque de la siguiente regla, a lo
fijado a mano y a lo ya cargado, `resolver.go:438-475`); relevo simple
(`releva_a`) y repetición de un solo nivel (`repite_a` directo a la
primaria), los dos con prueba verde; vuelta al principio del catálogo de
episodios sin ciclo infinito (`pickEpisodes`, `resolver.go:688-738`);
vigencia inclusiva de `fecha_fin` (`Covers`, `model.go:399-401`); elegir el
bloque en `motor.go` (`queToca`/`corteDe`) respeta la prioridad de decks y
usa un mínimo corrido correcto para el corte, sin depender del orden de la
lista de entrada.
