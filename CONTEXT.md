# Antena787

Broadcast playout and traffic automation: it decides what airs, plays it to
air, sells and proves the advertising around it, and publishes the guide.
This glossary is the project's vocabulary. Code, UI copy and documentation
use these words and no others.

## Language

### Scheduling

**Rule**:
A human's standing intent — a title, a day pattern, a time, and a start and
end date. Rules are what a person edits; the schedule is derived from them.
The dates are not a rights system: they are two fields the programmer already
keeps, from which expiry warnings come.
_Avoid_: recurrence, series entry, slot definition

**Plan Item**:
One resolved airing at an exact instant, naming a concrete asset and its
measured duration. Resolved from Rules 48 hours ahead.
_Avoid_: schedule entry, event, booking

**As-run**:
A Plan Item after it aired, carrying the times it actually happened. Not a
separate record — the same row in a later state.
_Avoid_: playout log, air log, history

**Broadcast Day**:
The programming day, which starts at a configured hour (6:00 AM by default),
not at midnight. Something airing at 12:30 AM Tuesday belongs to Monday's
broadcast day. A Rule's dates and day pattern are read in broadcast days,
and a Plan Item belongs to the broadcast day it starts in, even if it runs
past the boundary. A Rule's end date includes the whole of that broadcast day.
_Avoid_: calendar day, air date

**Slot**:
A nominal position on the grid, usually 30 minutes. A Plan Item's real
duration is almost always shorter; the difference is a Gap. When it would be
longer, that is an Overrun.
_Avoid_: timeslot, cell, block

**Overrun**:
Content that would run past the next hard start. It is never started: the
clock wins, the resolver leaves it out, fills the rest, and says so. Nothing
is ever cut in the middle to make room.
_Avoid_: overflow, spill, over-length

**Gap**:
Air time with no Plan Item. Anything from the seconds left over inside a
Slot to hours of unprogrammed night.
_Avoid_: hole, dead air, empty space
_Industry says_: "dead air" is the universal word for silence actually on air — that is an Incident, not a Gap.

**Run**:
One firing of a Rule, which may air several consecutive episodes.

**Episodes per Run**:
How many consecutive episodes one Run airs, advancing through the series and
remembering where it stopped.
_Avoid_: count, repeat count, duration

**Handoff**:
One Rule ending in a time position and another taking it over the next day.
A handoff is not a conflict.
_Avoid_: replacement, swap, transition

**Encore**:
A second airing, under its own Rule, of the same episode its primary Rule
aired earlier that Broadcast Day (in code: `repite_a`). An Encore has no
episode counter of its own; if the primary Rule aired nothing that day, the Encore takes the next
episode and advances the shared counter, so the series never stalls.
_Avoid_: repeat, rerun, replay, second pass

**Deck**:
One of a channel's parallel queues — manual, commercial, programme, or filler,
in that priority order. At any instant the air belongs to the highest-priority
deck that has something to play. Programme runs by sequence, commercial runs
by clock, filler runs when nothing else does.
_Avoid_: track, layer, channel, playlist
_Industry says_: in radio automation "Deck A/B" usually means two crossfading players — not this. Name the distinction when onboarding radio people.

**Manual Hold**:
The state in which an operator has taken the air. While held, the manual deck
owns the air even between things the operator fires. It ends when the
operator releases it, when the block it was taken during ends, or when the
output has carried no signal — silence or black — past a configured limit,
in which case the system releases it itself and raises an alarm — or when
the system itself went down, which is recorded as such. Idle never
means the operator stopped touching things; it means nothing is going out.
One person holds it at a time; a second person can take it over by name.
_Avoid_: manual mode, override, assist mode
_Industry says_: "Live Assist" (WideOrbit), "override". Same thing.

**Daypart**:
A named stretch of the Broadcast Day that Rules can be grouped under —
"weekday mornings".
_Avoid_: strip, band, zone

### Media

**Asset**:
One media file plus everything measured about it: real duration, loudness,
captions, head and tail black.
_Avoid_: file, media, video, clip

**Title**:
The work — a series, film, promo, station ID or spot. An Asset is a copy of
part of a Title.
_Avoid_: show, program, content

**House Format**:
The single encoding every Asset is converted to on the way in: one codec,
one resolution, one frame rate, closed GOPs, one audio configuration, one
loudness target. Configured per Channel.
_Avoid_: profile, preset, standard

**Normalization**:
Converting an Asset to House Format. Happens once, on ingest, in the
background.
_Avoid_: transcode, conversion, processing

**Conform**:
The per-clip corrections applied at play time — padding short audio with
silence, holding the last frame of short video, pillarboxing wrong
geometry. Distinct from Normalization: Normalization happens once on the
file, Conform happens every time it plays.
_Avoid_: fixup, adjustment, correction

**Filler**:
Short Assets — promos, station IDs, interstitials — the resolver uses to
fill a Gap exactly.
_Avoid_: padding, bumper, interstitial

**Slate**:
A static card, generated at install from the station's name and community,
aired when there is nothing else to air. The last step of the fallback
cascade — it carries the station's identification, so the cascade never ends
in bare bars. Bars and tone exist only for the install test.
_Avoid_: card, holding image, placeholder

**Quarantine**:
Where an Asset goes when ingest finds a problem, or when it fails twice on
air more than five minutes apart. Quarantined Assets never reach air unless
a person lets one through under their own name, which is recorded.
_Avoid_: rejected, failed, error queue

### Live

**Live Source**:
A Plan Item fed by an incoming signal instead of a file. It reserves its
time; if the signal is still absent past a short grace period, Filler airs
and an alarm fires; when the signal returns inside the reserved time the
air goes back to it at the next Filler clip boundary, a minute at most. Its end is a hard start for whatever follows.
A Live Source is conformed like any Asset, and may carry audio only — the
video is then the programme's card, drawn by the system.
_Avoid_: live input, feed, stream

### Advertising

**Advertiser**:
Whoever buys air time.
_Avoid_: client, customer, sponsor

**Insertion Order**:
An Advertiser's purchase: how many Spots of which length, over which dates,
in which dayparts.
_Avoid_: contract, campaign, buy

**Spot**:
A commercial Asset, sold by length — 15, 30, 45 or 60 seconds.
_Avoid_: ad, commercial, advert

**Break**:
A scheduled stretch of air, at a clock time, into which one or more Spots are
inserted. Inside a file programme it pauses the programme, which resumes
after; inside a Live Source it replaces the live signal, which does not stop,
so it must align with the source's own break clock.
_Avoid_: ad break, pod, interruption

**Avail**:
Sellable seconds inside a Break. A Break has a position; an Avail has a
price.
_Avoid_: inventory slot, opening

**Load**:
How many minutes per hour of Avails the Channel allows. Configured per
Channel; twelve by default.
_Avoid_: ad density, saturation

**Spot Airing**:
The record that one Spot aired at one instant, reconciled against Emergency
Alert Events. The unit Proof of Play counts.
_Avoid_: impression, play, insertion

**Proof of Play**:
The report given to an Advertiser showing which Spot Airings happened.
Counted from records, never composed.
_Avoid_: affidavit, air report, verification
_Industry says_: "affidavit" in broadcast, especially political; "proof of play" comes from digital signage. Same document.

**Make-good**:
A Spot Airing rescheduled because an Emergency Alert Event or any other
Incident covered the original. Proposed by the system, confirmed by a person,
never billed twice.
_Avoid_: credit, compensation, rerun

**Classified**:
A paid line of text run on the crawl. It always carries who paid for it, and
it airs only after a person approved it.
_Avoid_: ticker item, message, ad text

**Approval Queue**:
Where anything bought through the portal — a Spot or a Classified — waits
for a person before its first airing. Technical checks are not approval.
_Avoid_: moderation, review list, pending

### Emergency and compliance

**Emergency Alert Event**:
A period when certified hardware downstream replaced the Channel's output.
Playout keeps running and never learns of it directly, so the event is
recorded from an Alert Driver — or by hand, when no driver could see it —
and reconciled into the As-run. Kept for at least two years. In code:
`alert_event`.
_Avoid_: EAS, alert, interruption

**Incident**:
Anything the system had to do on its own to protect the air, recorded with
its time: an encoder restart, a Live Source that did not arrive, a Manual Hold
released by timeout, a fallback to Slate, a clock jump, a power outage, a
cascade that lasted longer than minutes. The count of Incidents is a success
metric, so every one is written down.
_Avoid_: alarm, error, event

**Time-shift**:
A Rule that re-airs, at a later time, the programmes that aired in an earlier
window of the same Broadcast Day. It re-schedules the same files as new Plan
Items; it does not replay the recording, except for the stretch that was a
Live Source. Breaks in the re-aired window are fresh Avails, never copies of
the original ones. A source window with no programme produces nothing.
_Avoid_: delay, replay, rebroadcast
_In Spanish_: "diferido" — the term the PRD uses.

**Preempted**:
A Plan Item state meaning an Emergency Alert Event covered it. Distinct
from Skipped, which means the system chose not to play it.
_Avoid_: interrupted, overridden, cancelled

**Compliance Profile**:
The rules of the country the Channel broadcasts in — loudness target,
caption format, recordkeeping. Chosen by country, never assumed, and offered
rather than imposed: it keeps the records ready for whoever asks for them,
and it never scolds. The As-run is not one of those records; it is business
evidence.
_Avoid_: region, locale, standard

### Output

**Channel**:
One programmed signal, with its own House Format, Outputs, Compliance
Profile and schedule. One playout process serves one Channel.
_Avoid_: feed, stream, service

**Output**:
One destination a Channel's signal is sent to. A Channel may have several
at once, each with its own loudness target, connection state and reconnect
policy.
_Avoid_: destination, target, endpoint

**Output Driver**:
The implementation behind an Output — how the signal reaches a particular
kind of destination. Internal vocabulary; never shown to a user.

**Return Feed**:
The transmitted signal fed back into the system, captured after the
emergency alert equipment. The only place where what actually aired can be
observed; the recording and the comparison against the plan read from here,
never from the Channel's own output. Without one, alert detection is
degraded and events are marked by hand. It is also what the monitor stream
shows to a phone. In code: `capture_input`.
_Avoid_: loopback, monitor, our output
_Industry says_: "off-air return", "confidence feed".

**Alert Driver**:
The implementation that learns when an Emergency Alert Event happened, from
a contact closure, a network interface, or by comparing the Return Feed
against the plan. Internal vocabulary; never shown to a user.

**Break Marker**:
The signal sent downstream telling an encoder that a Break starts, so ad
insertion systems can act on it.
_Avoid_: cue, trigger, splice point
_Industry says_: SCTE-104/35 call this a "splice point"; encoder docs will use that word.
