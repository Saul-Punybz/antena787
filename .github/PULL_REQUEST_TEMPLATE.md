## Qué cambia

<!-- Una o dos líneas, en palabras claras. Si el PR cierra un issue: "Cierra #123". -->

## Por qué

<!-- El problema que resuelve. El "qué" ya está en el diff; aquí va el porqué. -->

## Cómo se probó

<!--
Qué corriste y qué salió. Sé concreto: "go test ./... verde" no dice nada por
sí solo si el cambio toca el motor.

Si tocaste internal/engine, dilo con medidas: la corrida de F0, su REPORTE.md,
la máquina donde corrió y cuántas horas.
-->

- [ ] `gofmt -l .` no imprime nada
- [ ] `go vet ./...` limpio
- [ ] `go test ./...` verde
- [ ] Corrí la F0: `bin/f0 all -hours ___` en ___________ *(si aplica)*

---

## DCO

- [ ] **Todos mis commits están firmados** (`git commit -s`), y entiendo que
      firmo el [Developer Certificate of Origin
      1.1](https://developercertificate.org/): tengo derecho a aportar este
      código bajo la AGPL-3.0 del proyecto.

## ¿Usaste IA?

Se acepta código escrito con ayuda de IA — este proyecto se construye así.
Lo que hace falta es que un humano lo haya revisado y que se diga aquí. No
penaliza el PR; le dice al revisor dónde mirar más despacio.

- [ ] **No.** Lo escribí yo.
- [ ] **Sí**, con ayuda de IA. Qué parte fue asistida y **qué revisé yo**:

<!-- Ejemplo: "El analizador de TS lo escribió el modelo; verifiqué a mano el
     cálculo de PCR contra la especificación y corrí la F0 de 8 horas." -->

## ¿Toca `internal/engine`?

El motor, el conformado y el watchdog son el 20% del código y el 95% del
riesgo. **Se leen línea por línea por un humano, siempre.** *(PRD §16, ADR
[0005](../docs/adr/0005-agpl-with-dco.md))*

- [ ] **No toca `internal/engine`.**
- [ ] **Sí lo toca**, y un humano lo leyó **línea por línea**, entendió qué
      hace y responde por él. Revisó especialmente: manejo del tiempo y del
      reloj, conteo de cuadros y muestras, desbordes de contadores, y qué
      pasa cuando algo falla a las 40 horas y no a los 5 minutos.

## Otras casillas

- [ ] Si el PR introduce un término nuevo del dominio, lo añadí a `CONTEXT.md`
      en el mismo commit.
- [ ] Si añade una dependencia, cambia el contrato con ffmpeg, cambia el
      modelo de datos de forma no aditiva o mete un proceso nuevo: **trae su
      ADR** en `docs/adr/`.
- [ ] No introduce CGo *(ADR 0002)*.
- [ ] Los mensajes de commit van en español, en imperativo, y **sin trailers
      de herramientas** — el único trailer es `Signed-off-by`.
