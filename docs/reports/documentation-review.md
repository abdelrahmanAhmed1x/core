# Documentation rewrite review

This change rewrites documentation and adds example/check tests only. Hash checks
against the source/dependency snapshot taken at the start of the task found no
changes to pre-existing Go files, go.mod or go.sum. Earlier implementation work
already present in the working tree was preserved.

## Structure

The root README now starts with installation, a linked package chooser, shared
setup and small examples. Each of the 19 public packages has the same what/when,
usage, API, common configuration and important-notes pattern. Package READMEs use
the same examples and supply extra package-specific recipes rather than repeating
whole HTTP servers.

The six advanced guides cover authentication, caching, configuration, security,
operations and benchmarks. Historical implementation artifacts were moved into
reports; old AUTH.md, CACHE.md and UTILITIES.md links redirect to the current guides.
No existing guide was simply copied wholesale as the new introductory material.

The README remains longer than the suggested 200–350 lines to retain usage and API
coverage for all 19 packages. It removes repeated full programs and implementation
explanations; the chooser links directly to short individual sections.

## Corrected documentation mismatches

- The old authentication guide said invalid core JWT credentials could all be
  checked using transport.ErrInvalid. That is correct for single/stateless, but
  hybrid uses ErrInvalidAccessToken and ErrInvalidRefreshToken. The new guides
  distinguish them explicitly.
- Hybrid exposes no public standalone Verify method. Its documented verification
  path is Gin Middleware plus Payload/MustPayload; other strategies also expose
  core verification methods. No convenience API was invented to conceal this.
- Hybrid Refresh maps every extraction failure to ErrMissingRefreshToken. The
  advanced error reference now includes this behavior rather than suggesting
  callers will receive the original conflict/CSRF sentinel.

These are documentation corrections, not runtime fixes.

## Checks executed for this rewrite

- Inspected exported constructors, methods, configs and implementation behavior.
- Added 30 compiled source blocks in examples/handbook, with synchronization checks
  for the marked README examples (54 occurrences at the recorded check).
- Validated local Markdown paths and anchors with TestDocumentationLinks.
- Ran the memory cache example with an asserted output and the no-op metrics example.
- go test ./... passed with loopback networking enabled for the existing HTTP test.
  The first restricted attempt could not open that test listener; the rerun passed.
- go test -race ./examples/handbook passed.
- go vet ./examples/... and git diff --check passed.

Service integration tests remain opt-in and were not rerun for documentation-only
changes. Historical Redis/Dragonfly/race/performance evidence is kept separately;
this rewrite does not claim a new production benchmark or integration campaign.
