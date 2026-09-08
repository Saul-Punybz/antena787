# AGPL-3.0, with contributions under a DCO

Antena787 is licensed AGPL-3.0. Anyone may run it free, forever; anyone who
builds something commercial on top must release their own source under the
same terms. The Affero clause is deliberate — it closes the hosted-service
gap, and a hosted playout service is the obvious way someone would
commercialise this without giving anything back.

Being open source by the formal definition is load-bearing rather than
symbolic: it is what lets the project into Debian, Ubuntu, Fedora and
Homebrew, and what lets employees at companies contribute. A
non-commercial licence would have been the intuitive way to prevent
exploitation and would have backfired — it would leave the actual target
users in a grey zone, since a community station selling advertising to
sustain itself is arguably commercial use, and the advertising module exists
precisely for them.

Contributions come in under a DCO — a `Signed-off-by` line, as the Linux
kernel does. This accepts the consequence that once third-party code is
merged, relicensing is no longer a unilateral decision. That is coherent
with the core being a public good rather than an asset with an exit.

## Consequences

Because much of the code is written with AI assistance, human review is not
only an engineering safeguard but a legal one: works without human
authorship may not attract copyright, and copyleft has nothing to attach to
without a rightsholder. The documented human review already required for
the engine (ADR 0001) covers both.
