# No CGo, ever

Antena787 is pure Go with no C linkage. The moment any dependency is linked
through CGo, `GOOS=windows go build` stops producing a Windows binary from a
Linux machine, and trivial cross-compilation is the main reason Go was
chosen — not performance, which lives in ffmpeg rather than in our code.
When a C library is genuinely needed, we invoke its command-line binary as a
separate process instead of linking it.

This is the shortcut an implementer reaches for without thinking, so it is
written down as a hard rule rather than a preference.

## Consequences

**SQLite** uses `modernc.org/sqlite` (SQLite transpiled to Go), never
`mattn/go-sqlite3`; `ncruces/go-sqlite3` (WASM) was the runner-up.
The measured penalty — roughly 10% to 2x slower on inserts, 75-90% of CGo
throughput on selects — is irrelevant for a workload of schedule reads and
periodic writes.

**SDI and NDI output are out of scope.** The Blackmagic DeckLink and NDI
SDKs have no command-line equivalent for real-time frame injection; using
them means linking C. Rather than quietly carve out an exception, capture
cards are excluded from the official builds and from the no-CGo guarantee.
If someone needs SDI badly enough, it is a community build variant that
knowingly gives up cross-compilation — documented as such, never shipped by
us.
