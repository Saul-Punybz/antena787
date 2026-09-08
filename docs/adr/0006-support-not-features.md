# The money comes from support, not from the software

Antena787 is free under AGPL-3.0 and nothing is held back: a station downloads
it, installs it, and owes nothing. Every capability is in the free version —
scheduling, playout, live sources, advertising with break signalling and proof
of play, the classified crawl, the advertiser portal, recording and time-shift,
compliance profiles, the MCP server. What is sold is **having someone to call**:
$1,200 for support through getting on air, $100/month for an ongoing line.

## Why this boundary and not another

Two obvious alternatives were rejected.

**Charging per feature** — putting advertising or social streaming behind a
paywall — was rejected because those are exactly what the intended users need.
Social streaming is the *primary* use for a university or an NGO, and
advertising is how a community station survives. Charging for the seatbelt is
not the same as charging for the air conditioning.

**Charging for scale** — free for one channel, paid above — was the earlier
decision and it was replaced. It drew the line in a defensible place, but it
still meant the software itself was the product being metered, and it
introduced a boundary the codebase had to defend forever (a separate
supervisory process, a CI test proving the core runs without it). Support has
no such cost: nothing in the code has to know whether anyone paid.

Support is also the only boundary that is honest about what a licensed station
is actually buying. A station that goes dark at 3 AM has regulatory
consequences. It is not paying to install a program; it is paying for someone
awake on the other end. This is what the commercial products in this segment
already charge for — Dinesat sells a "24/7 emergency line" alongside its
software.

## Consequences

**Anyone can take it and never pay, and that is fine.** It is not leakage —
it is what sustains a community. Whoever has the time does it themselves;
whoever has a station to run prefers having someone to call.

The free version must never be crippled: no nag screens, no expiring features,
no watermark, no scale limit. A project that annoys its free users receives no
contributions, and this model depends entirely on the project being widely
used.

Because nothing is metered, **no part of the codebase has to enforce a
boundary** — which removes the multi-channel supervisory split, its licence
seam, and the CI test that guarded it.
