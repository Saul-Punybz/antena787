> 🇵🇷 [Léelo en español](README.md)

# Antena787

**Free software for launching and running television and radio channels.**

Antena787 decides what airs, plays it to air, sells and proves the
advertising around it, and publishes the electronic programme guide. It
serves a non-profit with an internet channel exactly as well as a community
broadcaster with a licensed transmitter. It is a single executable, it is
operated from a browser, and its only external dependency is ffmpeg, which
ships inside it.

Author: Saul A. González Alonso · Licence [AGPL-3.0](LICENSE) ·
Contributions under a [DCO](CONTRIBUTING.md) · Reference deployment:
Caribbean Advantage TV, Puerto Rico.

---

## Status: Phase 0 in progress. It cannot air anything today.

Said plainly, before anything else: **this is not a product you can
install.** There is no installer, no interface, no database, no channel, and
`cmd/antena` — the product executable — does not exist yet.

What does exist is **Phase 0**: an experiment with a pass-or-fail criterion
that answers the most expensive technical question in the project — whether
the engine design (a frame server in Go between per-clip decoders and a
persistent encoder) produces continuous, clean output for hours on modest
hardware. It is run, measured with tools, and reported. If it fails, the
engine design is reworked before the first line of Phase 1 is written.

See [`f0/README.md`](f0/README.md) and PRD §22.1.

**How far off it is, with eyes open** *(all estimated, assuming about 10
hours of work per week)*: shadow mode — the system proposes and a human
compares — roughly a year; a channel on air roughly three and a half years.
The full arithmetic is in PRD §22.2. This project makes no promises about
dates: it publishes its own.

---

## Who it is for

There is no single user, there is a range, and the system serves both ends
without punishing either.

| | Small end | Large end |
|---|---|---|
| Who | NGO, university, community broadcaster | Group with several signals |
| Channels | 1 | 2 to 20 |
| People | **one person** | a team with roles |
| Knowledge | knows how to install software and nothing more | has an engineer |
| Budget | close to zero | limited but real |
| What runs | a TV channel, or a radio station that also goes out online or over TV | TV and radio, several of each |
| Equipment | whatever they already have, whatever the brand | same |

**No brand or model is assumed**, and no country is assumed: the compliance
profile is chosen, never imposed.

Both ends are served through **progressive disclosure**: the system starts in
its simplest form and every advanced capability appears only when somebody
asks for it. Whoever runs one channel never sees multi-channel, roles, or
advertising until they register their first advertiser.

---

## What it will do

It schedules, airs, fills gaps, publishes the guide, takes live sources,
inserts and accounts for advertising, bills the advertiser and receives their
material, and proves what aired. **For television and for radio** — a radio
station is a television channel without video, and the Rules, the Plan, the
As-run, the Breaks and the Live Sources are identical.

And all of it is free, with nothing held back: no crippled edition, no nag
screens, no expiring features, no watermark, no channel cap.

### What it does NOT do, and why

- **It does not build emergency alert equipment.** That is certified hardware
  downstream; Antena787 integrates with it.
- **It does not build codecs.** That is ffmpeg. *(ADR [0003](docs/adr/0003-one-bundled-binary.md))*
- **It does not fork CasparCG or anyone else's engine.** *(ADR [0001](docs/adr/0001-own-playout-engine-in-go.md))*
- **No SDI or NDI card output.** Those SDKs force C linkage.
  *(ADR [0002](docs/adr/0002-no-cgo.md))*
- **No Plex or Jellyfin integration.**
- **It does not manage content rights.** No territories, no distribution
  windows, no run counts.
- **It carries no AI inside.** Zero models, zero API keys, zero cost imposed.
  *(ADR [0007](docs/adr/0007-ai-only-through-mcp.md))*

---

## The architecture, in one paragraph

It is all **Go**, chosen for trivial cross-compilation — Windows, Linux and
ARM from a single machine —, for a single binary with no runtime, and because
it reads well to someone who did not write it; not for performance, which
lives in ffmpeg rather than in our code. The product is **one process**: web
server, resolver and engine as goroutines, each catching its own panic and
relaunching itself, with the operating system's supervisor underneath. The
interface is written in TypeScript and React, built with Node at compile time
and embedded in the binary with `go:embed`; no Node runs on the station's
machine. The database is SQLite in WAL mode, through `modernc.org/sqlite` —
SQLite transpiled to Go — and **never through CGo**. The only external
dependency is **ffmpeg** (with ffprobe), bundled as a version-pinned static
binary and invoked as a subprocess. The engine is our own: each clip is
decoded by an ffmpeg into raw video and PCM audio, a **frame server in Go**
receives those frames, applies the conform, keeps the next clip's pre-roll
ready, and hands a single continuous stream to a **persistent encoder** over
its stdin. That is exactly the decision Phase 0 validates or kills.

The reasoning behind each of these decisions lives in [`docs/adr/`](docs/adr/).

---

## Start today: build and run Phase 0

You need **Go 1.26 or newer**, and **ffmpeg with ffprobe** on the PATH, next
to the executable, or pointed at by the `ANTENA_FFMPEG` variable.

```sh
go build -o bin/f0 ./cmd/f0

bin/f0 media                 # builds the test files in f0/media (≈1 min)
bin/f0 run -hours 8          # runs the engine; writes f0/out/{catv.ts,web.ts,events.jsonl,stats.csv}
bin/f0 analyze               # measures and writes f0/out/REPORTE.md
```

`bin/f0 all -hours 8` does all three. With `-udp udp://IP:PORT` the MPEG-2
output also goes to the real multiplexer; with `-one` it runs a single output,
to measure CPU with one and with two.

On Windows: `go build -o f0.exe .\cmd\f0`, then the same with `f0.exe`.

To check quickly that everything builds and runs, six minutes instead of eight hours:

```sh
make f0        # media + a 0.1 h run + analyze
```

The full 8-hour run takes about **48 GB** of disk. What is measured — and what
Phase 0 deliberately does not prove — is in [`f0/README.md`](f0/README.md).

To set up the full development environment — ffmpeg per operating system,
cross-compilation, package layout — see
[`docs/DESARROLLO.md`](docs/DESARROLLO.md) (in Spanish).

---

## Repository layout

```
PRD.md            what the system does and in what order it gets built (Spanish)
CONTEXT.md        the domain vocabulary — the single source of the terms
README.md         the Spanish version · README.en.md is this one
CONTRIBUTING.md   DCO, AI policy, conventions, how to contribute
CODE_OF_CONDUCT.md
SECURITY.md       how to report a vulnerability privately
LICENSE           AGPL-3.0, full text
Makefile          build · vet · test · f0 · windows · linux · arm64 · clean

cmd/f0/           the Phase 0 experiment executable
internal/engine/  the engine: decoder, frame server, encoder
internal/f0/      test-file generation and the analyzer
internal/ts/      transport stream reading, packet by packet
f0/               the experiment's README; media/ and out/ are generated, not versioned

docs/adr/         the architecture decisions and their reasoning
docs/ACEPTACION.md  the acceptance criteria
docs/DESARROLLO.md  how to set up the environment and build
docs/ROADMAP.md     the phases and where we are
docs/historial/     earlier versions of the PRD, to trace what changed

diseno/           screen mockups (generated HTML; not product code)
```

And what does not exist yet but is already decided: `cmd/antena/`,
`internal/resolver`, `internal/ingest`, `internal/drivers/`,
`internal/store`, `web/` (PRD §14.1).

---

## Documentation

| File | What it holds |
|---|---|
| [`PRD.md`](PRD.md) | The whole document: what it is, who it is for, how it works step by step, the data model, the phases. Start at §1 and §22. Spanish. |
| [`CONTEXT.md`](CONTEXT.md) | The glossary. Code, UI copy and documentation use these words and no others. |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | How to contribute: DCO, AI policy, conventions, drivers and country profiles. Spanish, with an English summary at the end. |
| [`docs/adr/`](docs/adr/) | The big decisions and why. One decision, one place. |
| [`docs/ACEPTACION.md`](docs/ACEPTACION.md) | The acceptance criteria. |
| [`docs/ARQUITECTURA.md`](docs/ARQUITECTURA.md) | How it is built inside, for whoever will touch code (Spanish). |
| [`docs/ROADMAP.md`](docs/ROADMAP.md) | The phases, in order, and where we are. |
| [`COMPLIANCE.md`](COMPLIANCE.md) | What it does and what it does **not** do for your legal compliance. |
| [`COMPRAR.md`](COMPRAR.md) | Equipment buying guide, with real prices. |
| [`SECURITY.md`](SECURITY.md) | How to report a vulnerability. |

Documentation in Spanish and English from day one: the natural market is
Latin America, and almost all broadcast software exists in English only.

---

## The model: the software is free, what you pay for is having someone to call

Everything is in the free version, with nothing held back: scheduling,
playout, live sources, advertising with break signalling and proof of play,
the classified crawl, the advertiser portal, recording and time-shift,
compliance profiles, the MCP server, and no channel limit.

| | Price |
|---|---|
| **Onboarding support** — hand-holding until you are on air | **$1,200** |
| **Ongoing support** — a line that answers, attention when it breaks | **$100 / month** |

And separately, on request: custom drivers for uncommon equipment,
compliance profiles for a new country, migration from another system,
training.

**Why that is worth what it costs.** A station that goes to black at 3 AM has
a problem that same night, and at that hour there is nobody to call. You are
not paying to install a program — you are paying for someone awake on the
other end. And it fits the AGPL with no friction: the licence obliges you to
release the code, not the labour or the availability.

**The consequence is accepted head-on:** anyone can download it, install it
alone and never pay. That is not leakage — it is what sustains a community.
Whoever has the time does it themselves; whoever has a station to run prefers
having someone to call. The full reasoning, with the two alternatives that
were rejected, is in ADR [0006](docs/adr/0006-support-not-features.md).

---

## Reference deployment

**Caribbean Advantage TV, Puerto Rico.** A community channel that today airs
with a hand-made Google Sheet, one human, VLC, a streaming server, another
VLC, the alert equipment and the transmitter — on a Windows 10 machine it
already owned, with no hardware purchased. It is where things get tested
first, not the mould: every piece of equipment that shows up there is one
example of a family that has a driver.

Agreeing to be the reference deployment cannot mean switching VLC off on a
Monday. That is why the system runs in shadow mode — proposing, and being
compared — long before it touches air.

---

## Authorship

Antena787 is designed and directed by **Saul A. González Alonso**. The code
and the documentation are written with the help of **Claude (Anthropic)**,
with human review — the engine, the conform and the watchdog are read line by
line, and not out of habit: it is what the copyleft stands on (see
[CONTRIBUTING.md](CONTRIBUTING.md) and ADR
[0005](docs/adr/0005-agpl-with-dco.md)).

Copyright (C) 2026 Saul A. González Alonso. Details in
[AUTHORS.md](AUTHORS.md).

---

## Licence

**AGPL-3.0.** The full text is in [`LICENSE`](LICENSE). Anyone may run it
free, forever; anyone who builds something commercial on top must release
their own source under the same terms. The Affero clause is deliberate: it
closes the hosted-service gap, which is the obvious way someone would
commercialise this without giving anything back. The reasoning is in ADR
[0005](docs/adr/0005-agpl-with-dco.md).

**On codec patents:** the software is free, but the H.264 and HEVC patents
are a separate matter that no software licence resolves, and distributing a
static ffmpeg with those encoders touches them. It is stated honestly, AV1
and VP9 — royalty-free — are offered for internet output, and it is worth a
legal consultation before the first public release.
