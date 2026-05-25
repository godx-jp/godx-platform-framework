# Versioning policy

godx-platform-framework follows [Semantic Versioning 2.0.0](https://semver.org/).

Version source: [VERSION](../VERSION). Release notes: [CHANGELOG.md](../CHANGELOG.md).

## SemVer interpretation

Given `MAJOR.MINOR.PATCH`:

| Change | Bump |
|--------|------|
| Breaking change to any **exported Go symbol** | MAJOR |
| Breaking change to any **environment variable name** | MAJOR |
| Breaking change to log/trace/metric attribute names | MAJOR |
| New module, new backend driver, new config var, new exported symbol | MINOR |
| Bug fix, doc-only change, dependency bump (non-breaking) | PATCH |

## v0.x history

Releases `v0.1.0` through `v0.14.0` were active design. Minor bumps could introduce breaking changes — see [CHANGELOG.md](../CHANGELOG.md) per release.

## v1.x guarantees (from v1.0.0)

**`MAJOR == 1`** signals API freeze for the Laravel-parity module set:

| Change | Bump |
|--------|------|
| Breaking change to any **exported Go symbol** in a stable module | MAJOR (2.0.0) |
| Breaking change to any **documented environment variable name** | MAJOR |
| Breaking change to log/trace/metric attribute names | MAJOR |
| New module, new backend driver, new config var, new exported symbol | MINOR |
| Bug fix, doc-only change, dependency bump (non-breaking) | PATCH |

Consumers may pin `@v1` or an exact tag (`v1.0.0`) and expect compatible upgrades within `1.x` PATCH and MINOR releases.

## v0.x caveat (historical)

While `MAJOR == 0`, minor bumps could introduce breaking changes. **Reach `1.0.0` signals API freeze** — achieved 2026-05-25.

## Support window

| Channel | Window |
|---------|--------|
| `latest` (default branch) | Always current; may contain unreleased work |
| Tagged releases (`v0.1.x`) | Patch releases for at least 6 months after the next MINOR |

## Release process (internal)

1. Update [VERSION](../VERSION) and [CHANGELOG.md](../CHANGELOG.md).
2. `make ci` (must pass `vet`, race tests).
3. `git tag -a vX.Y.Z -m "Release vX.Y.Z" && git push --tags`.
4. GitHub Action drafts release notes from `CHANGELOG.md`.
5. Announce in #godx-platform-eng.

## Companion product compatibility

godx-platform-framework `0.x` is designed to interoperate with godx-platform-observability `0.x` over OTLP. Both products evolve independently; the wire format (OTLP) is the only contract between them.
