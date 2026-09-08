# Own playout engine in Go, driving a persistent encoder

We build the playout engine ourselves in Go rather than adopting CasparCG,
and it works by keeping one hardware-accelerated ffmpeg encoder alive
forever and swapping decoded clips into it through a pipe — never by
splicing MPEG-TS with stream copy.

## Considered options

**Stream-copy TS splicing** (normalize everything to identical parameters,
then concatenate transport stream packets without re-encoding) was the
original design and three independent investigations refuted it. Video
frames at 29.97 fps and AAC frames at 48 kHz coincide only every ~21.36
seconds, so an arbitrary cut point essentially never aligns; real systems
reset the presentation clock at each splice and substitute silence frames at
the boundary. ffmpeg's concat path has documented failures — non-monotonous
DTS, start-time shift, silent failure on resolution mismatch, no
discontinuity signalling to HLS. The three projects that actually run 24/7
linear channels (ffplayout, Tunarr, ErsatzTV) all decode and re-encode;
ffplayout v2 was rewritten specifically to abandon `-c copy`. The argument
for accepting that fragility was CPU cost, and it collapsed: hardware
encoding measures around 15% of the CPU on a low-power N100.

**CasparCG** is mature (Sveriges Television has run national channels on it
since 2006) and is explicitly designed to be driven by an external
scheduler over AMCP. We rejected it on size and shape, not quality: it is a
real-time graphics compositor requiring a GPU with OpenGL 4.5, for a problem
that here is sequential file playback. More decisively, it is a separate
service with its own installation, configuration, logs and failure modes,
which breaks the product's central promise of one file and one installer.
For a volunteer running a community channel alone, "something failed in
CasparCG" is unfixable.

## Consequences

The engine is our risk. Human line-by-line review of the engine, conform
and watchdog is mandatory, as is a 30-day soak test against file output
before it feeds a licensed transmitter. We give up live CG beyond what the
frame server draws itself — the station logo and the classified crawl, both
composited in Go on the way to the encoder, so a logo can expire and a spot
can suppress it. Lower thirds and full graphics are not v1.
Hardware acceleration stops being a convenience and becomes structural:
software encoding costs a full core per channel and breaks the near-linear
cost that makes cheap hardware viable.
