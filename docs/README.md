# Documentation

Index for the godx-platform-framework documentation.

## Start here

- **[GETTING_STARTED](./GETTING_STARTED.md)** — five-minute tutorial: install, write a service, swap drivers.
- **[ARCHITECTURE](./ARCHITECTURE.md)** — framework backbone, module lifecycle, repository layout.

## Conventions (applies to every module)

- **[DRIVER_PATTERN](./DRIVER_PATTERN.md)** — the shared swappable-backend convention. Read once, applies to observability, future storage, cache, queue.
- **[SHARED_INFRA.md](./SHARED_INFRA.md)** — one Redis for cache + ratelimit (Laravel-style prefixes), module composition, use cases, testing bar.
- **[CONFIGURATION](./CONFIGURATION.md)** — full environment variable reference.
- **[VERSIONING](./VERSIONING.md)** — SemVer policy and compatibility guarantees.

## Modules

| Module | Status | Docs |
|--------|--------|------|
| `framework` | stable | [ARCHITECTURE — backbone](./ARCHITECTURE.md#backbone-the-framework-package) |
| `observability` | stable | [modules/observability](./modules/observability.md) |
| `storage` | stable (v0.6.x) | [modules/storage](./modules/storage.md) |
| `cache` | stable (v0.7.0) | [modules/cache](./modules/cache.md) |
| `config` | stable (v0.8.0) | [modules/config](./modules/config.md) |
| `events` | stable (v0.8.1) | [modules/events](./modules/events.md) |
| `hashing` | stable (v0.8.2) | [modules/hashing](./modules/hashing.md) |
| `encryption` | stable (v0.8.3) | [modules/encryption](./modules/encryption.md) |
| `pipeline` | stable (v0.8.4) | [modules/pipeline](./modules/pipeline.md) |
| `secrets` | stable (v0.8.5) | [modules/secrets](./modules/secrets.md) |
| `httpclient` | stable (v0.9.1) | [modules/httpclient](./modules/httpclient.md) |
| `validation` | stable (v0.9.0) | [modules/validation](./modules/validation.md) |
| `ratelimit` | stable (v0.9.2) | [modules/ratelimit](./modules/ratelimit.md) |
| `auth` | stable (v1.1.0) | [modules/auth](./modules/auth.md) |
| `mail` | stable (v0.10.0) | [modules/mail](./modules/mail.md) |
| `notifications` | stable (v0.10.1) | [modules/notifications](./modules/notifications.md) |
| `scheduler` | stable (v0.13.0) | [modules/scheduler](./modules/scheduler.md) |
| `featureflag` | stable (v0.13.1) | [modules/featureflag](./modules/featureflag.md) |
| `resilience` | stable (v0.13.2) | [modules/resilience](./modules/resilience.md) |
| `queue` | stable (v0.11.0) | [modules/queue](./modules/queue.md) |
| `httpx` | roadmap (v0.12) | — |
| `health` | roadmap (v0.14) | — |

Future modules each get their own file under `docs/modules/`. The driver pattern, naming convention, and layout are pinned in [DRIVER_PATTERN](./DRIVER_PATTERN.md) so every new module is structurally identical.
