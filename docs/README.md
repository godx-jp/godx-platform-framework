# Documentation

Index for the godx-platform-framework documentation.

## Start here

- **[GETTING_STARTED](./GETTING_STARTED.md)** — five-minute tutorial: install, write a service, swap drivers.
- **[ARCHITECTURE](./ARCHITECTURE.md)** — framework backbone, module lifecycle, repository layout.

## Conventions (applies to every module)

- **[DRIVER_PATTERN](./DRIVER_PATTERN.md)** — the shared swappable-backend convention. Read once, applies to observability, future storage, cache, queue.
- **[CONFIGURATION](./CONFIGURATION.md)** — full environment variable reference.
- **[VERSIONING](./VERSIONING.md)** — SemVer policy and compatibility guarantees.

## Modules

| Module | Status | Docs |
|--------|--------|------|
| `framework` | stable | [ARCHITECTURE — backbone](./ARCHITECTURE.md#backbone-the-framework-package) |
| `observability` | stable | [modules/observability](./modules/observability.md) |
| `storage` | stable (v0.6.x) | [modules/storage](./modules/storage.md) |
| `cache` | roadmap (v0.8+) | — |
| `queue` | roadmap (v0.9+) | — |
| `httpx` | roadmap (v0.10+) | — |

Future modules each get their own file under `docs/modules/`. The driver pattern, naming convention, and layout are pinned in [DRIVER_PATTERN](./DRIVER_PATTERN.md) so every new module is structurally identical.
