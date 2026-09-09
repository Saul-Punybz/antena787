---
status: accepted
---

# Emergency alerts: integrate the certified ENDEC, never replace it

Antena787 does not implement the Emergency Alert System. It integrates the
station's certified ENDEC (Sage, DASDEC, Gorman-Redlich, TFT) through
whichever path is wired, proves on the return feed that the alert actually
aired, and offers informational alerts from public CAP feeds on top. The
three layers are independent: a station with no ENDEC still runs, still gets
informational alerts, and the software says plainly what it can and cannot
certify.

## Considered options

**Rebuild OpenBroadcaster's alert module in Go.** OpenBroadcaster (OBServer +
OBPlayer, PHP and Python) is the only other free project aimed at community
TV and radio, and it ships a CAP ingest (Canada's NAAD, NOAA) that turns an
alert into audio, text and a log entry. It is the closest thing to a
reference, and it is the wrong thing to port. In the United States the
alert obligation is a certified-hardware obligation (FCC Part 11): the
station must run a certified ENDEC that decodes SAME from its monitored
sources and IPAWS, and no software running on the playout PC can take its
place. A Go rewrite of OpenBroadcaster's module would leave the station
with the same box it already has, plus a second, uncertified path that
invites the operator to trust the wrong one. What is worth keeping from
OpenBroadcaster is the idea, not the code: a CAP poller is a few hundred
lines against `encoding/xml`, and it belongs in layer three below, labelled
as informational.

**Talk to every ENDEC model natively.** The equipment list in the US is
short and known (PRD §10: Sage 1822 and 3644, DASDEC, Gorman-Redlich, TFT),
but models change and stations abroad use other systems entirely (EWBS in
South America). A per-model protocol library would always be incomplete and
would never cover the box the project has never had in hand.

**Integrate by connection path, verify on the return feed (chosen).** The
installer asks *how the box is connected* — relay closure, serial, network,
or several — and the driver for each path is small and model-agnostic:
`gpi-serial` and `gpi-gpio` for relays, `sage-endec` and `dasdec` for the
network protocols that publish their own state, `syslog` and `snmp-trap` for
whatever else reports. When two paths exist both are used and cross-checked.
Whatever triggered the alert, the engine does the same thing: hands the air
to the ENDEC's audio and video (or runs the crawl where the ENDEC only
delivers text), returns to the schedule at the next clean edge, and writes
the whole event into the as-run log, which is what the FCC asks to see.

## The three layers

1. **ENDEC integration (F2, required for a licensed station).** The drivers
   above, the air handoff, and the as-run entry. This is the only layer with
   a compliance meaning, and it never originates an alert.
2. **Evidence on the return feed (F2, the differentiator).** `signal-compare`
   already reads the return feed (ADR 0009). It gains a SAME decoder in pure
   Go: the header and end-of-message tones are a public protocol
   (FSK 520.83 baud, the `ZCZC-ORG-EEE-PSSCCC+TTTT-JJJHHMM-LLLLLLLL-` header),
   and decoding them from the return audio proves, with a timestamp, that the
   alert went out over the air rather than merely into the ENDEC. That line
   in the as-run is evidence no free project produces today. `dsame` is the
   readable reference implementation; nothing is linked.
3. **Informational alerts from CAP (F2 or F4, optional).** A poller for the
   IPAWS public feed and NOAA in plain Go shows alerts on screen and can feed
   the crawl. It is labelled *informativo, no sustituye al ENDEC* everywhere
   it appears, and it is the layer a radio or online-only channel with no
   Part 11 obligation can use on its own.

## What CAtv does today when the Sage fires (Saul, 9 September 2026)

The station's actual practice, which fixes the air handoff rule for the
motor (F2): **the alert stops everything while the message plays; the
commercial continues; and the lost time is taken out of the programme** (a
piece of the series, the film or the radio programme is dropped). So when
the ENDEC releases the air:

1. the commercial deck resumes and finishes the break in full — the spot the
   alert cut restarts from its head, it is never shortened and never replaced
   by black;
2. the programme absorbs the loss: it rejoins in progress or is trimmed at
   its end, so the clock and the next break stay where they were;
3. a spot that then went out in full counts as *aired* and needs no make-good;
   only a spot that could not be replayed inside its break (the break ran
   out of room) becomes `preempted` and goes to the make-good proposal of
   PRD §9 step 10.

This is the opposite of WideOrbit's "protect the revenue over the alert"
widget: the alert is never delayed for a commercial. What is protected is
what comes *after* it.

## Consequences

- No EAS encoder, no SAME generator, and no tone synthesis ever ship in
  Antena787. Bars and tone exist only for the installation test (F2-69), and
  attention signals only ever come from the ENDEC.
- Compliance is offered, not forced: the wizard's "todavía no" for the ENDEC
  path is a valid answer, the channel runs, and Al aire says what is and is
  not verified.
- The as-run gains two kinds of alert entries: *triggered* (from a driver)
  and *observed* (from the return feed). A triggered alert with no observed
  counterpart within the expected window is an incident, because it means
  the box fired and the air did not change.
- Layer two depends on the return feed being wired; the installer already
  asks for it and says what is lost without it.
- OpenBroadcaster's `mapping` repository stays on the reading list for the
  CAP layer's data model, and nothing from it is copied.
