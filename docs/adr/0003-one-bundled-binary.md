# ffmpeg is the only external binary, and it ships with us

The project depends on exactly one piece of software it did not write:
ffmpeg (with ffprobe). It is bundled as a version-pinned static binary
inside the installer and invoked as a subprocess, so nothing is ever
installed separately by the user.

No pure-Go video pipeline exists that can decode and encode the real range
of broadcast content at 24/7 reliability, and pretending otherwise would
mean writing codecs. The alternatives are honest about their limits: the
most promising pure-Go H.264 work states outright that it is not a
general-purpose encoder. So the question is not how to avoid ffmpeg but how
to keep the user from ever having to think about it. ErsatzTV solves it the
same way — bundling static builds, not forking ffmpeg.

## Consequences

Everything else that looked like an external dependency was eliminated:

- **TSDuck** is unnecessary once ad signalling goes to the encoder as
  SCTE-104 (ADR 0004). `Comcast/scte35-go` builds SCTE-35 sections in pure
  Go if they are ever needed directly.
- **Plex and Jellyfin** are not integrated. Kodi `.nfo` files, container
  metadata and the keyless Cover Art Archive and TVmaze APIs — TMDb with a
  free key — cover library metadata
  without a third-party server. Plex informs how the library *looks*, not
  where its data comes from.
- **RTMP and SRT reception** are embedded as Go libraries, not a separate
  streaming server, so an encoder pushing to Antena787 talks to the same
  process that airs it.
- **The XMLTV validator is written in Go.** The reference `tv_validate_file`
  is a Perl script; bundling a Perl runtime would break the single-binary
  promise for what amounts to an XML schema check.
- **PostgreSQL** stays optional and is never required for a single channel.
