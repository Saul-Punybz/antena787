# Auditoría de la guía en PMCP contra ATSC A/76B

Fecha: 11 de septiembre de 2026. Archivo auditado: `internal/resolver/pmcp.go`,
servido en `/guia.pmcp`.

## 0 · La fuente, que sí existe

El comentario que había en `pmcp.go` decía que el esquema XSD del Anexo A ya no
estaba alojado y que por eso el exportador «sigue el texto del estándar» de
memoria. **El esquema sí está**, completo y con los ejemplos del propio
estándar, en el repositorio oficial de esquemas de ATSC:

- <https://www.atsc-schemas.org/pmcp/2006/3.0/> — los XSD de PMCP 3.0
  (`pmcp30.xsd`, `channel.xsd`, `event.xsd`, `essencemetadata.xsd`,
  `captions.xsd`, `regionrating.xsd`, `pmcptype.xsd`).
- <https://www.atsc-schemas.org/pmcp/2006/3/XMLSamples/> — los documentos de
  ejemplo, entre ellos `ScheduleDownload.xml` (una descarga de parrilla, que es
  exactamente lo que Antena787 manda), `Captions.xml` y `USRatingTable.xml`.
- El documento A/76B en PDF: <https://www.atsc.org/wp-content/uploads/2021/04/A76B-2016.pdf>

Todo lo que sigue sale de ahí, comprobado el 11 de septiembre de 2026. Como en
Go no hay validador de XSD en la biblioteca estándar y no se añaden
dependencias (ADR 0002/0003), `ValidatePMCP` reproduce a mano las
comprobaciones del esquema; cada una está anotada con lo que reemplaza.

## 1 · Lo que estaba mal (nueve hallazgos, todos corregidos)

| # | Qué decía el código | Qué dice el esquema | Efecto |
|---|---|---|---|
| 1 | `xmlns="http://www.atsc.org/pmcp/2006/3.0"` | `targetNamespace="http://www.atsc.org/XMLSchemas/pmcp/2006/3.0"` (`pmcp30.xsd`, y los tres ejemplos) | Un generador que valide contra el esquema descarta el documento entero. La ruta donde vive el archivo (`/pmcp/2006/3.0/`) no es el namespace. |
| 2 | `id` = nanosegundos del reloj (≈1.8·10¹⁸) | `id` es `xsd:unsignedInt` **obligatorio** (máximo 4 294 967 295) | Fuera de rango. Ahora van los segundos de época, que caben y siguen subiendo. |
| 3 | `<Channel sourceId="catv.antena787">` | `sourceId` es `xsd:unsignedShort` | Un identificador de texto en un atributo numérico. Se quitó: Antena787 no tiene un source_id del VCT. |
| 4 | `<ShortName>CAtv</ShortName>` como elemento hijo | `shortName` es **atributo**, de 7 caracteres como máximo; el hijo `Name` es el nombre largo | Elemento inexistente. Ahora salen los dos, cada uno en su sitio. |
| 5 | `<PsipEvent channelNumber="…" startTime="…">` y `<EventId>texto</EventId>` | `channelNumber` es atributo **obligatorio de `EventId`**, no de `PsipEvent`; `EventId` es un elemento compuesto (`PmcpEventId` / `InitialSchedule` / `PsipEventId`), no texto; y `startTime` de `PsipEvent` es la hora **real**, «when different from the scheduled start time» | Era la parte más rota: el evento no decía a qué canal iba de forma que el esquema entienda, y su identificador era texto libre. Ahora sale el par que usa `ScheduleDownload.xml`: `<PmcpEventId creator="Antena787" id="…"/>` + `<InitialSchedule startTime="…"/>`, y la hora real solo cuando el bloque ya salió y difiere. |
| 6 | `<Genre lang="es">Infantil</Genre>` | **`ShowData` no tiene elemento `Genre`.** Sus hijos son `Name`, `Description`, `ParentalRating`, `Audios`, `Captions`, `RedistributionControl`, `DataBroadcast`, en ese orden | Elemento inexistente. El género, el nombre del episodio y la marca de infantil pasan a `Description`, que es el texto del ETT del evento. |
| 7 | `lang="es"` / `lang="en"` | `languageType` = `[a-z]{3}` (ISO 639-2) y `lang` es **obligatorio** en todo texto | Atributo inválido en cada línea del documento. Ahora `spa` / `eng`. |
| 8 | `<Rating dimension="Rating" value="TV-14"/>` suelto dentro de `ShowData` | El elemento es `ParentalRating` con `region` obligatorio, y dentro los `Rating` con el nombre de dimensión **de la RRT**: `Entire Audience`, `Children`, `Dialogue`, `Language`, `Sex`, `Violence`, `Fantasy Violence`, `MPAA` (`USRatingTable.xml`) | «Rating» no es una dimensión que exista. Ahora `TV-G/PG/14/MA` → *Entire Audience*, `TV-Y/TV-Y7` → *Children*, `PG-13`, `R`, `NR`… → *MPAA*, y las letras que siguen (`TV-14-DLV`, `TV-Y7-FV`) salen cada una como su propia dimensión. La región es la 1, «US (50 states + possessions)», que incluye a Puerto Rico. |
| 9 | Un bloque de menos de medio segundo se anunciaba con `duration="PT0H0M0S"` | `duration` es `xs:duration`; el EIT mide en segundos enteros | Un evento de duración cero. Ahora el mínimo es un segundo, y la duración se escribe en la forma corta de los ejemplos (`PT30M`, `PT1H30M`). |

## 2 · Lo que ya estaba bien, y conviene no tocar

- El **criterio de qué se publica** es correcto y es el mismo que el de XMLTV:
  solo `asset` y `live_source`, nunca relleno ni cartel, y nunca un bloque de
  duración no positiva (`esContenidoPublicable`, `resolveTitle` compartidos).
- **Las horas van en UTC**, que es lo que pide §5.10 («PMCP time will be
  ultimately referenced to UTC»).
- El **orden de los hijos** de `PmcpMessage` —`Channel` antes que `PsipEvent`—
  ya coincidía con la `xsd:sequence` del esquema.
- Mandar el `ShowData` **dentro** del evento, sin un elemento `Show` aparte, es
  correcto: el esquema lo declara opcional dentro de `PsipEventType`.
- `type="information"` es uno de los tres valores del enumerado, y no llevar
  `PmcpReply` es lo coherente con no implementar el protocolo con acuse de
  recibo (§5.7).
- `originType="Automation"` describe bien lo que Antena787 es dentro de la
  cadena.

## 3 · Lo que sigue faltando

1. **El número de canal virtual. Es lo único que bloquea de verdad.**
   `EventId/@channelNumber` es obligatorio y solo admite un número de una parte
   («5») o de dos con guion («57-2»); no existe «no lo sé». `model.Channel` no
   guarda el número mayor-menor de ATSC de la estación. Hoy sale
   `NumeroDeCanalPMCPPorDefecto` = `"1"`, y **un generador PSIP que no conozca
   el canal 1 descarta los eventos**. Lo que hace falta: un campo
   `numero_canal` en `model.Channel` (texto, validado con el mismo patrón), que
   el asistente pregunte, y pasarlo a `PMCPCompleto`. Mientras tanto la vía ya
   está abierta: `PMCPCompleto(ch, "57-2", …)` funciona y está probada.
2. **Los subtítulos, conectados.** `PMCPCompleto` ya escribe el Caption Service
   Descriptor a partir de las fichas de los archivos
   (`MediaAsset.HasCaptions`, `CaptionFormat`, `SubtitulosSidecar`,
   `ExternalCaptions`), y dice tanto que un bloque los lleva como que no los
   lleva —las dos cosas son información útil para el televidente—. Falta una
   línea en `internal/app/resolve.go`: pasarle el mapa de fichas, que esa
   función ya tiene a mano por `guideCatalog`. Sin él, `PMCP` se calla.
3. **El validador no corre antes de publicar la PMCP.** `internal/app/resolve.go`
   llama a `resolver.PMCP` y empuja el resultado con `pushGuidePMCP` **sin
   pasar por `ValidatePMCP`**, mientras que el XMLTV sí tiene su puerta
   (`ValidateXMLTVPorGravedad`, F1-28, y una prueba en
   `internal/app/f1verif_guia_test.go` que comprueba que alguien la llama). La
   PMCP no tiene ni la puerta ni esa prueba. `ValidatePMCPPorGravedad` ya
   devuelve graves y avisos con la misma forma que su gemela, listo para
   enchufar.
4. **El idioma de los subtítulos y del audio.** Hoy se declara `spa` porque es
   lo que emite el canal, igual que los títulos. No hay campo de idioma por
   archivo ni por título; si alguna vez el canal emite en inglés, esto miente.
   El elemento `Audios` (`Ac3Audio audioId/lang/serviceType`) no se manda en
   absoluto: `MediaAsset.PistasAudio` tiene los datos, pero el SAP/DVS es
   trabajo de F2 y no se adelanta.
5. **El TSID y la red del multiplexor** (`Channel/@tsid`, `@network`) no se
   mandan. Existen en los parámetros de la salida `udp-ts`
   (`internal/drivers/salida/udpts.go`), no en el canal. Son opcionales en el
   esquema; solo hacen falta si algún día hay más de un canal.
6. **El atributo `action`.** Los ejemplos del estándar mandan `action="add"` en
   cada evento de una descarga de parrilla. Antena787 reescribe la guía entera
   en cada corrida, así que «add» podría duplicar y «update» podría no crear.
   Se omite a propósito hasta saber qué equipo la consume y cómo la trata: es
   el invariante 3, y la pregunta ya está con el cliente (`SUBTITULOS-Y-METADATA-2026-09-09.md`, §C.1).

## 4 · Lo que cambió en el criterio del validador

`ValidatePMCP` ahora devuelve **solo lo grave** —lo que un generador PSIP
rechaza o entiende mal— y `ValidatePMCPPorGravedad` devuelve graves y avisos
por separado, como hace el XMLTV. Es la misma regla de siempre («lo grave no
sale; los huecos se publican y se avisan») con la diferencia de que aquí la
lista corta es la que hace de puerta, porque un hueco en PSIP es literalmente
la guía en blanco que ve el televidente y hay que avisarlo sin que impida
publicar el resto.

Lo que el validador caza hoy y antes no: namespace equivocado, `id` fuera de
rango o ausente, `originType` ausente, tipo de mensaje inventado, número de
canal con formato inválido o que el documento no declara, identificativo corto
de más de siete caracteres, canal sin nombre, idioma que no es de tres letras,
duración que no es `xs:duration` o que vale cero, servicio de subtítulos fuera
del 1-63, un `Captions` que dice a la vez que sí y que no, y **eventos que se
pisan** —que antes no se miraba y en XMLTV sí—.
