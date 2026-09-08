# Ad breaks are signalled as SCTE-104 to the encoder, not written as SCTE-35

Antena787 sends SCTE-104 messages to the downstream encoder and lets the
encoder generate the SCTE-35 in the transport stream. It does not write
SCTE-35 itself, and it does not patch ffmpeg to do so.

This is the standard industry split — an automation system talks SCTE-104 to
a compression system, which is exactly what Antena787 and the encoder are.
AWS Elemental MediaLive converts SCTE-104 to SCTE-35 on ingest; Wowza only
relays SCTE-35 rather than generating it; Google Ad Manager and MediaTailor
assume it is already in the stream. Earlier drafts had this backwards and
had Antena787 emitting SCTE-35 in-band.

It is also the only path that works. `grep -i scte libavformat/mpegtsenc.c`
returns nothing: ffmpeg's muxer has no SCTE-35 code at all. Three patch
series (2021, 2023 from LTN Global, 2025 from Net Insight) were never
merged — ignored for want of reviewers rather than rejected — and the only
fork carrying them is based on ffmpeg 4.3.1 from 2020. Maintaining that
forward is open-ended work in someone else's C, forever.

SCTE-104 is also far cheaper to implement: a self-contained TCP message with
no PCR, PTS or continuity counters to manage. No Go library exists, but the
specification is clear and the protocol is simple enough to write directly.

## Consequences

Emitting the marker is a small feature with large leverage — it plugs a
station into server-side and dynamic ad insertion, addressable advertising
and programmatic exchanges without building any of that. For channels with
no SCTE-104-capable encoder, a fallback that injects sections into the
transport stream downstream of ffmpeg is possible — but it is, in practice,
writing part of a remuxer, three to four times the work of the crawl, with
no documented 24/7 production precedent anywhere. It is out of v1. A station
whose encoder cannot take SCTE-104 sells its breaks locally, with no
third-party insertion. If that path is ever built, it needs its own 48-72
hour soak before anyone's revenue depends on it.
