# The interface never interrupts air to show something

No screen in Antena787 covers, replaces or pauses the view of what is
currently going out. Looking at the past, editing a schedule, previewing a
crawl, reviewing an alarm — none of it takes the operator's eyes off air.
Reviewing something is always a second panel beside the live one, never a
modal on top of it.

This came out of five independent investigations into the best interfaces in
five different fields, and all five landed on the same rule from different
directions.

**Surveillance (Milestone XProtect, "Independent Playback")** lets an operator
watch recorded footage from one camera while others keep playing live in the
same screen — their documentation is explicit that the operator "has full
control without losing track of what is happening in real time." The
anti-pattern is the cheap DVR: a PLAYBACK button that swaps the whole screen,
which you then have to explicitly exit.

**Live streaming (Twitch Stream Rewind)** scrubs backwards on the same player
with one button to return, and everything else on the page keeps updating in
real time — so "this is still happening" never disappears while you look at
the past.

**Aviation** puts the mode in a fixed always-visible strip and in full-field
colour rather than an icon, because peripheral vision detects luminance change
without the eye being pointed at it — which is exactly the case of a distracted
operator. Asiana 214 flew into a seawall partly because a mode indicator
required active reading under workload.

**Live media triggering (QLab, cart walls)** puts content preparation on a
different screen from the one used live. mAirList's Design Mode sits on the
live screen and invites reorganising the panel while on air.

**Traffic and ad sales (Google Ad Manager, Resource Guru)** show a conflict
where the sale is being placed, not in a separate report — which is what lets
one person sell without being a traffic manager.

## What this forbids

No modal dialog over the on-air view. No "playback mode" that replaces the
screen. No mode indicated only by an icon or a corner badge. No conflict,
warning or validation result that lives in a report the operator has to go
find. No content-preparation controls on a screen being used live.

## What this requires instead

A live panel that never stops, with review as a second panel marked distinctly
("reviewing: 12 min ago"). Mode shown as a full background wash plus explicit
text, never colour alone and never an icon alone. Problems surfaced at the
place where the action is taken. Preparation screens separated from live
screens.

## Why it is a decision and not a style

It is exactly the kind of rule a future contributor breaks without knowing why
it is wrong — a modal is the obvious way to show a detail view, and every web
framework makes one easy. The cost of getting it wrong is not aesthetic: for a
licensed station, losing sight of air to inspect something is how dead air goes
unnoticed.
