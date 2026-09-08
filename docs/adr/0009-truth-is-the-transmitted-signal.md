---
status: accepted
---

# What aired is observed from the transmitted signal, never from our own output

The recording, the As-run reconciliation and the `signal-compare` Alert
Driver all read from a Return Feed — the signal captured **after** the
emergency alert equipment — and never from the Channel's own output.

The alternative was cheaper: tap the frames we already have on the way to
the encoder. It is also useless for the one thing it exists for. When the
alert hardware downstream replaces the signal, Antena787 keeps playing and
never learns of it; comparing our output against our plan would show two
identical things and record a clean As-run for a Spot that nobody saw. The
alert log the `us-fcc` profile keeps would be fiction, and the make-goods
the advertising module promises would never be proposed. The simulation of
one real week at CAtv found this before any code did.

A Return Feed costs the station something — a capture input, an ATSC
receiver, or the transmitter's own monitoring stream — and some stations
will not have one. That is allowed: without it the driver runs in a
declared `degraded` mode, the interface says so, and Emergency Alert Events
are marked by hand. What is not allowed is pretending to verify from a
place where verification is impossible.

## Consequences

- The install assistant asks where the return feed comes from, and accepts
  "none yet" as an answer that is shown, not hidden.
- Silence-and-black detection on the output stays on our own signal (it
  watches what we send); only the *truth of what aired* comes from the
  return feed. The two are different questions and use different taps.
- The `capture_input` table exists for this and nothing else.
