# Changelog

All notable changes are documented here. Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) · versioning: [SemVer](https://semver.org/).

## [Unreleased]

## [0.8.2] — 2026-05-25

Ships the `hashing/` module — Laravel's `Hash` facade with three modern drivers, self-describing encoded output, and explicit `NeedsRehash` semantics for safe work-factor upgrades. Third release in the Laravel-parity reshuffle.

### Added

- **`hashing/` module** — `Manager` holds one or more named `Hasher` instances; `Default()` returns the primary, `Hasher(name)` returns a specific one, `CheckAny(ctx, plain, hash)` resolves a stored hash to whichever driver understands its prefix (`$2y$`, `$argon2id$`, `$scrypt$`) — perfect for migrations.
- **`hashing/driver/`** — public contract: `Hasher` interface with `Make / Check / NeedsRehash / Info` plus `Info{Algorithm, Params}` for diagnostics. `Spec` carries every driver's tunables. Sentinel errors `ErrInvalidHash`, `ErrUnknownFormat`, `ErrPasswordTooLong`, `ErrIncompatibleParams`. Registry mirrors the cache/config shape.
- **`drivers/bcrypt`** — Laravel-default. Cost 4..31 (default 12). 72-byte plaintext hard limit surfaces as `ErrPasswordTooLong`. `NeedsRehash` true when stored cost < configured cost.
- **`drivers/argon2id`** — PHC-format encoded output (`$argon2id$v=19$m=…,t=…,p=…$salt$digest`) — interoperates with Laravel, PHP, Python passlib, and the Argon2 reference. Defaults match OWASP 2024 (64 MiB / 3 iterations / 2 threads). `NeedsRehash` upgrades on any weaker param.
- **`drivers/scrypt`** — RFC 7914 with a `$scrypt$ln=…,r=…,p=…$salt$digest` custom encoding (stores log2(N) so rehash math stays exact). Defaults match RFC 7914 interactive-login guidance.
- **`hashing.Module`** — env-driven (`HASHING_DEFAULT`, `HASHING_HASHERS`, `HASHING_BCRYPT_COST`, `HASHING_ARGON2ID_*`, `HASHING_SCRYPT_*`). `ModuleWithConfig` for code-driven wiring. `MustDefault()` returns a bcrypt hasher without the App boilerplate for tests and scripts.
- **`examples/hashing/main.go`** — runnable demo: Make / Check correct + wrong / Info / NeedsRehash across driver choice.
- **`docs/modules/hashing.md`** — full reference: drivers, mixed-driver deployments, env vars, Laravel mapping, Migrating from go-common section.

### Tests

- **`hashing/conformance_test.go`** — driver-agnostic suite that asserts every Hasher round-trips, rejects wrong plaintext, returns useful `Info`, refuses garbage encodings, and produces salt-randomised output. Runs against all three drivers.
- **`hashing/manager_test.go`** — Default / named lookup, sorted Hashers list, duplicate-name + nil-hasher rejection, mixed-driver `CheckAny` resolution, idempotent Shutdown.
- **`hashing/edges_test.go`** — bcrypt rehash threshold, bcrypt 72-byte limit, bcrypt invalid hash, bcrypt out-of-range cost rejection, argon2id rehash threshold, argon2id malformed-segment / wrong-variant / unsupported-version rejection, argon2id memory-vs-threads constraint, scrypt rehash threshold, scrypt non-power-of-2 N rejection, scrypt invalid hash, cross-driver `Info` returns `ErrUnknownFormat`, `MustDefault` returns bcrypt-prefixed output.
- **`hashing/driver/registry_test.go`** — Register / Lookup / Names / New round trip, empty-name + nil-constructor panics, sentinel errors distinct under `errors.Is`.
- **`hashing/module_test.go`** — Module wires the manager into the App, env loading (defaults + overrides), Validate rejects malformed configs, context helpers.

### Dependencies

- `golang.org/x/crypto` promoted to a direct dependency (already pulled in indirectly).

## [0.8.1] — 2026-05-25

Ships the `events/` module — an in-process event bus with wildcard listeners, optional async worker pool, and `framework.App` wiring. Second release in the Laravel-parity roadmap reshuffle.

### Added

- **`events/` module** — `Bus` interface with `Listen / Forget / Dispatch / Patterns / Close`. `New()` returns the default synchronous in-process bus. Concurrency-safe; idempotent `Close`.
- **Wildcard patterns** — `*` (all), `user.*` (trailing), `*.deleted` (leading), `user.*.email` (mid-segment). Multi-segment trailing wildcards handle nested namespaces (`user.*` matches `user.profile.updated`).
- **`Subscription`** — handle returned by `Listen`; `Cancel()` removes that exact listener without affecting siblings. `Forget(pattern)` bulk-removes by exact pattern match.
- **Error joining + panic safety** — listener errors are joined via `errors.Join`; siblings still run. A panicking listener is converted to an error so other listeners stay protected.
- **`events.NewAsync(inner, AsyncOptions{...})`** — fire-and-forget wrapper backed by a worker pool. `Dispatch` returns when the queue accepts the job (or the context is canceled, or the bus is closed). `Close(ctx)` drains pending jobs before returning. `Options.OnError` surfaces listener errors.
- **`events.Module`** — env-driven (`EVENTS_ASYNC`, `EVENTS_ASYNC_WORKERS`, `EVENTS_ASYNC_QUEUE_SIZE`). Publishes the Bus under `events.StoreKey`. `ModuleWithConfig(cfg, onError)` for code-driven wiring.
- **`examples/events/main.go`** — runnable demo covering exact-match, wildcard fan-out, and the global `*` catch-all.
- **`docs/modules/events.md`** — full reference + Laravel API mapping + Migrating from go-common.

### Tests

- **`events/bus_test.go`** — exact-match dispatch, fan-out across exact + multiple wildcards, default `CreatedAt` stamping, joined listener errors, panic recovery, `Subscription.Cancel` (idempotent), `Forget` count, empty `Event.Name` rejection, empty-pattern + nil-listener panic, context cancellation surfacing, idempotent `Close` returning `ErrClosed` on subsequent `Dispatch`, concurrent Listen+Dispatch+Cancel sweep.
- **`events/async_test.go`** — fire-and-forget high-throughput, `OnError` plumbing, `Close` drains pending jobs, post-close `Dispatch` returns `ErrClosed`, context cancellation while queue is full, `Listen / Forget / Patterns` proxied through to the inner Bus.
- **`events/matcher_test.go`** — pattern-matching table covering every wildcard placement.
- **`events/module_test.go`** — `Module` wires the Bus into `framework.App`, async config from env, `ContextWithBus / FromContext` round trip, env-loading boundary cases.

## [0.8.0] — 2026-05-25

Foundation release for the Laravel-parity roadmap. Ships the `config/` module — a layered configuration repository with typed accessors and pluggable sources. Module skeleton is the same shape as `cache/` and `storage/` so adding new sources stays mechanical.

### Added

- **`config/` module** — `Repository` with `Get / GetString / GetInt / GetBool / GetFloat / GetDuration / GetSlice / GetStringSlice / GetMap / Set / Forget / Has / All / AllFlat` typed accessors plus the generic `config.Get[T](repo, key, def)` helper. Dot-separated nested keys. Concurrency-safe.
- **`config/manager.go`** — multi-source `Manager` that merges layers in registration order (last source wins). Stable `Repository` pointer across `Reload`s; `OnChange` callback fan-out; cleanly handles `AddSource` rollback when a new source's Load errors.
- **`config/driver/`** — public contract: `Source`, optional `Watcher`, `Spec`, `Constructor`, plus sentinel errors (`ErrNotSupported`, `ErrClosed`, `ErrFileMissing`, `ErrUnsupportedFormat`) and the standard `Register / Lookup / Names / New` registry that mirrors `cache/driver/registry.go` shape for shape (and now 100 % covered by `registry_test.go`).
- **`config/drivers/env`** — light, auto-registered. Strips `Spec.Prefix`, lowercases, splits `__` to dots. Configurable separator via the manual `env.New(prefix, separator)` constructor.
- **`config/drivers/file`** — light, auto-registered. YAML (`gopkg.in/yaml.v3`), JSON (stdlib), TOML (`github.com/BurntSushi/toml`). Format inferred from extension or set explicitly via `Spec.Format`. Implements `Watcher` via a 1-second mtime poll — zero `fsnotify` dependency, works on every OS.
- **`config/drivers/static`** — in-process map source for tests and compile-time defaults. Exposes `Update(map[string]any)` for live mutation.
- **`config/module.go`** — `config.Module` reads `Config` from env (`CONFIG_SOURCES`, `CONFIG_AUTO_ENV`, `CONFIG_ENV_PREFIX`, plus per-source `CONFIG_SOURCE_<NAME>_*`) and publishes `*Manager` under `config.StoreKey`. `ModuleWithConfig(cfg)` for code-driven wiring. Auto-env source appended last so process env always overrides files (matches Laravel `.env` precedence).
- **`examples/config/main.go`** — runnable demo covering env-only, file-only, and layered file+env-override flows.
- **`docs/modules/config.md`** — full module reference: env table, driver matrix, typed accessor table, Laravel API mapping, and a "Migrating from go-common" section for Tiximax services.

### Tests

- **`config/repository_test.go`** — typed accessors, generic `Get[T]`, deep merge, concurrent reader/writer mix.
- **`config/manager_test.go`** — merge precedence, reload, OnChange, idempotent Shutdown, source-name uniqueness, nil-source rejection, ErrClosed semantics.
- **`config/conformance_test.go`** — driver-agnostic Load / Name / idempotent Shutdown across `env`, `file`, `static`. New drivers shipped under `config/drivers/...` must keep this green to be considered config-compatible.
- **`config/edges_test.go`** — nil-data Repository, empty keys, non-map intermediates overwritten by `Set`, AddSource rollback on Load error, concurrent `Reload`.
- **`config/drivers/file/file_test.go`** — YAML / JSON / TOML round-trips, optional-vs-required missing path, format-extension mismatch, explicit `Format` override, idempotent Shutdown returning `ErrClosed`, mtime-based watch firing on change.
- **`config/drivers/env/env_test.go`** — prefix filtering, `__` → `.` nesting, ErrClosed after Shutdown, registry auto-registration.

### Roadmap reshuffle

`docs/ARCHITECTURE.md` and `README.md` roadmap tables updated to announce the new Laravel-parity sequence: `events` → `hashing` → `encryption` → `pipeline` → `secrets` → `validation` → `httpclient` → `ratelimit` → `mail` → `notifications` → `scheduler` → `featureflag` → `resilience` first, then resume `queue` / `httpx` / `cloudwatch` / `health` toward the `v1.0` API freeze. No removal of previously promised modules — only an order change to ship the higher-leverage pieces first.

### Dependencies

- `gopkg.in/yaml.v3` (new, BSD-3) — file driver YAML decoding.
- `github.com/BurntSushi/toml` (new, MIT) — file driver TOML decoding.

## [0.7.1] — 2026-05-25

Test-hardening patch. No public-API changes other than one new sentinel error in `cache/driver`. The release is driven by a fresh "audit every nook and cranny" pass over the cache + storage modules.

### Fixed

- **`cache/drivers/memory`** — calling `Put`, `Add`, `Forget`, `Increment`, `Decrement`, `Flush` (or `Has`) **after** `Shutdown` no longer panics with a `nil map` write. The driver now tracks a `closed` flag and returns the new `cdriver.ErrClosed` sentinel from every entry point. Affects only callers that hold a `*Store` past the framework's lifecycle (`Manager.Shutdown` always disposes of stores) — was a sharp edge nobody had hit yet but trivial to trigger from a test.

### Added

- **`cache/driver.ErrClosed`** — public sentinel for "this driver was shut down". Compatible with `errors.Is`.
- **`cache/conformance_test.go`** — single, parametrised matrix that runs **every** Laravel-spirit scenario (round trip, TTL, Add atomicity, counters + concurrent counters, Pull, Flush scope, Flush isolation across two prefixes on the same backend, boundary values incl. empty/binary/1 MiB/UTF-8/512-byte keys, context cancellation) against `memory`, `file`, **and** live `redis`. New drivers shipped under `cache/drivers/...` must keep this suite green to be considered cache-compatible.
- **`cache/edges_test.go`** — Manager + Module + Config edge cases: nil store rejection, empty-name rejection, missing default, `MustStore` panic semantics, idempotent `Shutdown`, sorted `Stores()`, double-init rejection, `AddStore` before `Module` diagnostic, `FromApp` pre-init error, global+per-store prefix isolation, partial-init cleanup on bad driver, hyphen/underscore/case normalisation in env names, default-not-in-`STORES` validation, JSON corruption surfacing through `GetJSON`, negative TTL clamped to "forever", concurrent `Default()` / `Store()` / `Stores()` under contention.
- **`cache/driver/registry_test.go`** — `Register` panics on empty name and nil constructor, last `Register` wins, `Lookup` returns nil for missing, `Names()` sorted and includes auto-registered `memory`, `New()` returns helpful diagnostics for empty name and missing driver (hint at the `drivers/<name>` import path), every sentinel distinct under `errors.Is`. Coverage of the registry package went from **0 % to 100 %**.
- **`cache/drivers/memory/lifecycle_test.go`** — idempotent `Shutdown`, every operation returns `ErrClosed` after `Shutdown`, sweeper goroutine actually exits (50 short-lived drivers must not leak goroutines), negative/zero TTL semantics, concurrent readers cannot mutate cached values via the returned slice.
- **`cache/drivers/file/lifecycle_test.go`** — corrupt envelope is recovered (broken file removed, next `Put` succeeds), temporary `.cache-*` write artefacts are cleaned, 32 concurrent writers × 50 ops do not corrupt a shared key, `Flush` over a deleted root is a noop, TTL is preserved across `Increment` of an already-expiring key.
- **`storage/drivers/local/edges_test.go`** — absolute-key inputs (`/etc/passwd`) stay sandboxed under the configured root, backslashes normalised to forward slashes (Laravel parity), `Exists` on a directory returns false, `Delete`/`Attributes` on missing keys return `stordriver.ErrNotFound`, `List` on empty/missing/regular-file prefixes behaves correctly, default visibility from `Spec` honoured, idempotent `Shutdown`, 16 × 25 concurrent writers across distinct keys, 4 MiB round trip, URL encoding preserves slashes while escaping spaces.
- **`storage/drivers/memory/edges_test.go`** — write-after-`Close` errors, empty/whitespace/`/`/backslash key rejection, metadata + content-type + cache-control + visibility round trip, returned metadata maps are defensive copies (mutation does not leak into the cache), 32 × 100 concurrent writers, default visibility from `Spec` honoured, overwrite replaces every field, traversal segments collapsed before storage, idempotent `Shutdown`.

### Coverage

| Package | Before | After |
| --- | --- | --- |
| `cache` | 73.7 % | **84.4 %** |
| `cache/driver` | 0 % | **100 %** |
| `cache/drivers/memory` | 90.2 % | **91.6 %** |
| `cache/drivers/file` | 79.2 % | **82.5 %** |
| `storage/drivers/local` | 79.2 % | **85.8 %** |
| `storage/drivers/memory` | 82.2 % | **95.0 %** |

Heavy-driver coverage (`redis`, `gcs`, `azure`) is unchanged at the unit level — those branches are still exercised exclusively under `-tags integration` against the corresponding emulators.

### Verification

- `go test -race -count=5 ./...` — all packages green over five consecutive runs (catches sleep-based flakes).
- `go test -race -count=1 -tags integration ./cache/drivers/redis/ ./storage/drivers/internal/s3core/` — live redis + MinIO regression.
- `go vet ./...` and `go vet -tags integration ./...` clean.

## [0.7.0] — 2026-05-25

New module — Laravel-faithful **`cache`** with three drivers: `memory`, `file`, `redis`. By explicit user direction, database-backed cache is out of scope.

### Added — `cache` module

- **`cache` package** — same boot-time shape as `observability` and `storage`. `cache.Module` reads `CACHE_*` from the environment, builds every configured store, and publishes a `*Manager` into the framework `App` under `cache.StoreKey`. `cache.ModuleWithConfig` and `cache.AddStore` cover code-driven and "extra store after boot" wiring respectively.
- **`Manager` + `Store`** — Laravel `Cache` parity. `mgr.Default()` / `mgr.Store(name)` returns a `*Store`; the store exposes `Get`, `Put`, `Forever`, `Add`, `Pull`, `Forget`, `Has`, `Missing`, `Flush`, `Remember`, `RememberForever`, `Increment`, `Decrement`. JSON convenience helpers (`GetJSON`, `PutJSON`, `RememberJSON`) cover the most common "cache a struct" pattern without forcing callers to repeat marshal/unmarshal boilerplate.
- **`cache/driver`** — public contract package. `Driver` interface (8 methods); typed sentinels `ErrNotSupported`, `ErrNotImplemented`, `ErrNotInteger`; in-process registry mirroring the storage module so out-of-tree drivers register identically (`driver.Register("mydb", construct)`).
- **`cache/context`** — `ContextWithManager` / `FromContext` for handlers that prefer context-attached state; `FromApp` is the canonical way to retrieve the manager from a `framework.App`.

### Added — drivers

- **`cache/drivers/memory`** (light, auto-registered) — `sync.Mutex`-guarded map with a 30-second sweeper goroutine that purges expired entries so memory does not grow unbounded under TTL-heavy workloads. Returned slices are copies so callers cannot poison the cache by mutating them. Prefix-scoped Flush.
- **`cache/drivers/file`** (light, auto-registered) — Laravel `FileStore` layout: one `*.cache` file per key, JSON envelope `{ "exp": <unix-ms>, "val": <base64> }`, two-level hash sharding (`<root>/XX/YY/<sha1>.cache`) so directories stay browsable on every filesystem. Writes go via tmp + `os.Rename` so partial writes never corrupt a key. Per-key locking keeps `Add` and `Increment` atomic in-process; `os.Rename` provides cross-process atomicity on POSIX/NT filesystems.
- **`cache/drivers/redis`** (heavy, opt-in) — `github.com/redis/go-redis/v9` client. `Put` -> `SET (PX)`, `Add` -> `SET NX (PX)`, `Increment`/`Decrement` -> native `INCRBY`/`DECRBY` (full atomicity even under heavy contention), `Flush` -> `SCAN`+`UNLINK` scoped to the configured prefix (or `FLUSHDB` when no prefix is set). Supports both `URL` (`redis://user:pass@host:port/db`) and component-wise (`ADDRESS` + `USERNAME` + `PASSWORD` + `DB`) configuration. Ping at construct time so misconfigurations crash on boot rather than at first cache call.

### Config

- New env-var family `CACHE_*` (no abbreviations, matching the rest of the SDK):
  - `CACHE_DEFAULT_STORE` (default `memory`), `CACHE_STORES` (CSV), `CACHE_PREFIX` (global prefix prepended to every key across every store).
  - Per-store `CACHE_STORE_<NAME>_DRIVER`, `_PREFIX`, `_DEFAULT_TTL`, `_PATH` (file), and `_URL` / `_ADDRESS` / `_USERNAME` / `_PASSWORD` / `_DB` / `_TLS` (redis).
- Driver-name shortcut: `CACHE_STORES=redis` infers `CACHE_STORE_REDIS_DRIVER=redis` automatically, so very small services don't need to repeat the name.
- File-driver path defaults to `./storage/framework/cache` (Laravel-faithful) when `DRIVER=file` and no `PATH` is supplied.

### Tests

- Per-driver unit tests cover put/get/forget round trip, TTL expiry, `Add` atomicity (100-goroutine contention test on file driver — exactly one wins), atomic increment/decrement, `ErrNotInteger` rejection on non-numeric values, prefix-scoped `Flush`, and (for file) cross-instance persistence + on-disk layout invariants.
- `Store` + `Manager` tests cover `Remember` (caches once), `Pull` (atomic read-and-delete), JSON helpers, manager wiring through `framework.Module`, duplicate-store rejection, and the "driver not registered" diagnostic path.
- `//go:build integration` test for the redis driver hits a live redis-server: round trip + TTL + `SET NX` + concurrent counter (50 goroutines × 10 incrs = exactly 500) + prefix-scoped Flush. Verified live against `redis:7-alpine`.

### Example

- `examples/cache/main.go` is a 100-line walkthrough of every Store method against the default store, with three documented configurations (in-memory, file, redis).

### Docs

- New `docs/modules/cache.md` reference covering concepts, env-driven configuration, programmatic wiring, the full Store API, the driver matrix, per-driver guides, error model, lifecycle, and the rationale for not shipping a database-backed cache driver.
- `README.md`, `docs/CONFIGURATION.md`, `docs/ARCHITECTURE.md` updated for v0.7.0: cache module added to the module list, cache env vars added to the configuration reference, roadmap shifted (CloudWatch driver moves to 0.8.x).

### Roadmap

- 0.7.x cache module closes here for now (memory + file + redis).
- 0.8.x reopens the observability track with the CloudWatch driver.
- 0.9.x queue · 0.10.x httpx.

## [0.6.2] — 2026-05-25

Heavy storage drivers, round 2 — Google Cloud Storage and Azure Blob Storage now ship as real implementations. With this release the storage matrix is complete (six drivers: `local`, `memory`, `s3`, `minio`, `gcs`, `azure`) and `v0.6.x` closes.

### Added — `gcs` driver (full implementation)

- **`storage/drivers/gcs`** — Google Cloud Storage backend on `cloud.google.com/go/storage`. Implements every `storage.Driver` method:
  - `NewReader` / `NewWriter` — uses the SDK's native resumable upload writer, so writes of arbitrary size stream without buffering. Forwards ContentType, CacheControl, and user metadata when supplied.
  - `Delete` — surfaces `gcs.ErrObjectNotExist` as `driver.ErrNotFound` so the manager and stack drivers can react consistently.
  - `Exists` / `Attributes` — single `Attrs` round trip; returns Size, Updated (LastModified), ContentType, ETag, and Metadata.
  - `List` — paginated `Objects` walk with `Delimiter="/"`, emitting files plus synthetic directory entries from CommonPrefixes. Matches the local/s3 list semantics.
  - `URL` — concatenates `Spec.PublicURL` + the URL-escaped key.
  - `SignedURL` — V4 signed GET via the SDK's `Bucket.SignedURL`; default expiry 15 minutes. Surfaces `driver.ErrNotSupported` with a helpful diagnostic when the resolved credential cannot sign locally (metadata-server creds need IAM SignBlob, which is out of scope for `v0.6.x`).
- **UBLA-safe visibility** — the driver only sets `PredefinedACL` when the caller explicitly requests a visibility. Uniform Bucket-Level Access buckets (the default for new GCS buckets) therefore work transparently; callers needing public exposure either disable UBLA or grant bucket-level IAM.
- **Credentials** — Application Default Credentials chain (env `GOOGLE_APPLICATION_CREDENTIALS`, `gcloud auth application-default login`, Workload Identity / GKE / GCE). Supplying `Spec.Endpoint` switches to no-auth mode for `fake-gcs-server` and other emulators.

### Added — `azure` driver (full implementation)

- **`storage/drivers/azure`** — Azure Blob Storage backend on `github.com/Azure/azure-sdk-for-go/sdk/storage/azblob`. Implements every `storage.Driver` method:
  - `NewReader` / `NewWriter` — writer pipes into `Client.UploadStream`, so arbitrary-size writes stream as a block-blob upload without buffering. Forwards ContentType, CacheControl, and metadata.
  - `Delete` — translates `bloberror.BlobNotFound` / `ContainerNotFound` / `ResourceNotFound` to `driver.ErrNotFound`.
  - `Exists` / `Attributes` — `GetProperties` round trip; returns Size, LastModified, ContentType, ETag, and metadata.
  - `List` — `NewListBlobsHierarchyPager` with `Delimiter="/"`, paginating files plus synthetic directory entries from BlobPrefixes.
  - `URL` — concatenates `Spec.PublicURL` + the URL-escaped key. Azure scopes visibility at container level, so per-write Visibility flags are intentionally ignored (set container access via portal / `az`).
  - `SignedURL` — Shared Access Signature (SAS) with V2 protocol and Read permission, signed locally with the shared-key credential. Returns `driver.ErrNotSupported` when the driver was constructed with `DefaultAzureCredential` (OAuth user-delegation SAS is out of scope for `v0.6.x`).
- **Credentials** — `Spec.AccessKey` + `Spec.SecretKey` map to Azure's storage-account name and key (shared-key auth, required for SAS issuance). Without them the driver falls back to `DefaultAzureCredential` (AZURE_* env, managed identity, `az login`).
- **Spec mapping** — Azure terminology mapped to the generic `driver.Spec`: `Bucket` = container, `Endpoint` = service URL (`https://<account>.blob.core.windows.net`), `AccessKey`/`SecretKey` = account name/key.

### Tests

- **Wrapper unit tests** — both drivers ship registration + required-field validation tests that run in the standard `go test` pass (no docker, no network).
- **Live integration tests** behind `//go:build integration`:
  - `TestGCS_Integration_FakeServer` — verifies the gcs driver end-to-end against `fsouza/fake-gcs-server`. Boots the emulator, writes/reads/deletes a blob, and confirms `Delete` of a missing key returns `ErrNotFound`. Boot instructions in the test file header. Verified live.
  - `TestAzure_Integration_Azurite` — verifies the azure driver end-to-end against Microsoft's Azurite emulator. Self-bootstraps the container if missing, writes/reads/inspects/SAS-signs/deletes a blob, and confirms `Delete` of a missing key returns `ErrNotFound`. Verified live (azurite must run with `--skipApiVersionCheck` because the SDK's API version is newer than Azurite's pinned default).

### Storage matrix (v0.6.2)

| Driver | Status | Notes |
|---|---|---|
| `local` | stable | Filesystem; defaults to `./storage/app/private`. |
| `memory` | stable | In-process; useful for tests. |
| `s3` | stable (heavy) | AWS S3 via `s3core` + ProfileAWS. |
| `minio` | stable (heavy) | MinIO / R2 / Spaces / B2 via `s3core` + ProfileMinIO. |
| `gcs` | **stable (heavy, v0.6.2)** | Google Cloud Storage. |
| `azure` | **stable (heavy, v0.6.2)** | Azure Blob Storage. |

### Roadmap

- `v0.6.x` storage track closes here.
- `v0.7.x` ships the **`cache` module** (Laravel-faithful), with `memory` + `file` + `redis` drivers — by explicit user direction, DB-backed cache is out of scope.
- `v0.8.x` resumes the observability track with the CloudWatch driver.

## [0.6.1] — 2026-05-25

Heavy storage drivers, round 1 — AWS S3 and MinIO (any S3-compatible store) now ship as real, production-grade implementations instead of stubs. Backward-compatible for v0.6.0 consumers that did not yet rely on the stubbed heavy drivers; the only behaviour change is the Laravel-faithful local-disk default root.

### Added — `s3` and `minio` drivers (full implementations)

- **`storage/drivers/internal/s3core`** — shared S3-protocol driver built on `github.com/aws/aws-sdk-go-v2`. Implements every storage.Driver method end-to-end:
  - `NewReader` / `NewWriter` — the writer pipes into `feature/s3/manager.Uploader`, so writes of arbitrary size stream as a multipart upload without buffering the whole body in memory.
  - `Delete` — does a pre-flight `HeadObject` so missing keys surface as `driver.ErrNotFound` (vanilla S3 `DeleteObject` is idempotent and would otherwise mask the miss).
  - `Exists` / `Attributes` — single `HeadObject` round trip; returns the canonical metadata record (Size, LastModified, ContentType, ETag, user metadata).
  - `List` — `ListObjectsV2` with `Delimiter="/"`, returning files plus synthetic directory entries built from `CommonPrefixes`. Matches the local/memory `List` semantics exactly.
  - `URL` — concatenates `Spec.PublicURL` + key (URL-encoded). Returns `driver.ErrNotSupported` when no PublicURL is configured.
  - `SignedURL` — presigned GET via the SDK's `PresignClient`; default expiry 15 minutes.
- **Compatibility profiles** — a `Profile` parameter selects defaults:
  - `ProfileAWS` (virtual-hosted-style addressing, AWS regional endpoint when none supplied) for the `s3` driver.
  - `ProfileMinIO` (path-style forced, endpoint required, region defaults to `us-east-1`) for the `minio` driver.
  Both wrappers (`storage/drivers/s3`, `storage/drivers/minio`) are 5-line files that delegate everything to `s3core.NewConstructor(<profile>)`. The same impl already covers Cloudflare R2, DigitalOcean Spaces, Backblaze B2, and any other S3-compatible store via the `minio` wrapper.
- **Credentials** — the SDK's default credential chain (env → `~/.aws/credentials` → IRSA → EC2 IMDS) is used unless `Spec.AccessKey` + `Spec.SecretKey` are set explicitly. `Spec.SessionToken` is honoured for STS short-term credentials.
- **Tests** —
  - 10 new unit tests in `s3core` that drive the implementation through an in-memory fake `API` (no network, no docker). Cover round-trip, ACL mapping, metadata/content-type/cache-control forwarding, list grouping by directory, public/signed URL paths, and required-field validation for both profiles. Race-clean.
  - New integration test `TestS3Core_Integration_MinIO` behind `//go:build integration` that runs against a real MinIO. Verified live: bucket `mb` → `PutObject` via the multipart uploader → `HeadObject` → `GetObject` → `PresignGetObject` → `DeleteObject` → second delete returns `ErrNotFound`. Boot instructions in the file header.
  - Wrapper tests in `drivers/s3` and `drivers/minio` confirm auto-registration on blank import and surface clear errors for missing required fields (bucket, region, endpoint).

### Changed — Laravel-faithful local-disk default (minor breaking)

- `STORAGE_DISK_<NAME>_ROOT` for the `local` driver now defaults to `./storage/app/private` instead of `./storage`. Mirrors Laravel's bare-install `storage_path('app/private')`. The conventional matching `public` disk is rooted at `./storage/app/public`, with `VISIBILITY=public` and a `PUBLIC_URL` so `disk.URL()` works.
- Existing consumers that relied on the previous bare `./storage` root must set `STORAGE_DISK_<NAME>_ROOT=./storage` explicitly, or migrate their on-disk layout to `./storage/app/private`. Documented in CONFIGURATION and modules/storage.

### Roadmap

- v0.6.x continues: `gcs` and `azure` drivers move from stub to full impl in subsequent patches.
- v0.7.x remains the CloudWatch observability driver.

## [0.6.0] — 2026-05-25

Storage-module release — adds a Laravel `Storage`-style multi-disk file/object abstraction. **Additive only** for existing consumers (observability is unchanged); the new `storage` module is opt-in via `app.Use(storage.Module)`.

### Added — storage module

- **`storage` package** — Laravel `Storage` facade parity for Go. A single `Manager` holds one or more named `Disk` handles; each disk is backed by a driver chosen at configuration time. Disks expose the full Laravel ergonomic API: `Put`, `Get`, `Exists`, `Missing`, `Delete`, `Copy`, `Move`, `Size`, `LastModified`, `Files`, `Directories`, `ReadStream`, `WriteStream`, `Append`, `Prepend`, `URL`, `TemporaryURL`, plus typed write options (`WithContentType`, `WithVisibility`, `WithMetadata`, `WithCacheControl`).
- **Driver pattern** — identical to observability: public `storage/driver` package (interface, `Spec`, registry, `Visibility`, `Attributes`, `Entry`, `WriteOptions`, `ErrNotFound`/`ErrNotSupported`/`ErrNotImplemented`), one package per implementation under `storage/drivers/<name>/`. See [docs/DRIVER_PATTERN.md](docs/DRIVER_PATTERN.md).
- **Light drivers (auto-registered)** — `local` (filesystem with path-traversal guard, visibility-to-file-mode mapping, optional public URL base) and `memory` (in-memory; great for tests).
- **Heavy drivers (opt-in blank import)** — `s3`, `gcs`, `azure`, `minio` registered as stubs returning `driver.ErrNotImplemented` until their full implementation lands in v0.6.x patch releases. Misconfigurations surface at startup, not at first write.
- **Multi-disk + default** — `STORAGE_DEFAULT_DISK` selects the default; `STORAGE_DISKS=avatars,uploads,cache` declares additional disks. Each disk loads its config from `STORAGE_DISK_<NAME>_*` env vars. Zero env vars set → single `local` disk rooted at `./storage`, private visibility.
- **Programmatic API** — `storage.ModuleWithConfig(cfg)` for code-driven configuration; `storage.AddDisk(name, cfg)` for incrementally registering disks after the primary `Module`; `storage.NewDiskFromDriver(name, drv, cfg)` for wrapping a custom driver.
- **Context plumbing** — `storage.ContextWithManager` / `storage.FromContext` / `storage.FromApp`, matching the observability ergonomics.
- **Tests** — 27 new test functions covering local + memory drivers, multi-disk manager, env-driven disks, all stubs, ordering errors, path traversal, visibility round-trip, list semantics, append/prepend, copy/move, signed-URL fallback, and the heavy-driver blank-import error message. Race detector clean.

### Roadmap shift

- Storage promoted to v0.6.x (was: v0.7.x).
- Full CloudWatch driver shifted to v0.7.x (was: v0.6.x).
- Cache, queue, httpx remain on the roadmap in their original relative order.

## [0.5.0] — 2026-05-25

Channel-system maturity release — closes the Laravel `config/logging.php` parity gap. All additions are **backward-compatible**; no consumer code change is required to upgrade from 0.4.0.

### Added — per-channel level filter

- `Config.LogLevel` on a `NewChannel(name, cfg)` config is now formally documented as the per-channel minimum level (Laravel `'level' => 'warning'`). The driver's `slog.Handler` enforces it, so records below threshold never reach the wire. Verified end-to-end by `TestChannel_PerChannelLevelFilter`.

### Added — env-driven channels (zero Go code)

- `observability.ChannelsFromEnv()` — new framework module that reads `OBSERVABILITY_CHANNELS` (comma list) and, for each name X, builds a `Config` from `OBSERVABILITY_CHANNEL_<X>_*` env vars, then registers the channel on the primary provider. Mirrors Laravel `config/logging.php` for projects that prefer 12-factor over Go code.
- `observability.LoadChannelConfigFromEnv(name)` — exported helper that returns the per-channel `Config`. Same field shape as `LoadConfigFromEnv` but with the `OBSERVABILITY_CHANNEL_<NAME>_` prefix; OTLP keys are namespaced (the global `OTEL_EXPORTER_OTLP_*` cannot be repeated per-channel).
- `observability.ChannelsEnvVar` constant (`"OBSERVABILITY_CHANNELS"`).
- Channel name normalisation: case-insensitive; hyphens/spaces convert to underscores. `audit-trail` ⇒ `OBSERVABILITY_CHANNEL_AUDIT_TRAIL_*`.
- Startup validation: reserved name (`primary`), duplicate names, wrong wiring order (`ChannelsFromEnv()` before `Module`), unknown driver, and any per-channel construction error all fail fast with a clear message.
- Wiring (one line, safe to leave in `main` permanently — no-op when `OBSERVABILITY_CHANNELS` is unset):

  ```go
  app := framework.New("svc", "1.0.0").
      Use(observability.Module).
      Use(observability.ChannelsFromEnv())
  ```

### Added — stack driver per-sub minimum level

- `OBSERVABILITY_STACK_DRIVERS` now accepts an inline `name:level` syntax per entry — Laravel "info to stdout, warn+ to file" pattern without defining named channels:

  ```bash
  OBSERVABILITY_DRIVER=stack
  OBSERVABILITY_STACK_DRIVERS=stdout:info,file:warn
  OBSERVABILITY_LOG_FILE_PATH=/var/log/app.log
  ```

  Without `:level` the sub-driver inherits the parent's `OBSERVABILITY_LOG_LEVEL` (existing behaviour). Unknown levels fail at construction with a clear error pointing at the offending sub.

### Added — `driver.ParseLogLevel` helper

- New exported helper `driver.ParseLogLevel(s string) (slog.Level, bool)` in `observability/driver` — single source of truth for the four supported levels (`debug` · `info` · `warn`/`warning` · `error`). Used internally by the observability config loader and the stack driver; available to third-party drivers via the public `driver` package.

### Tests

Nine new tests cover the additions:

- `TestStackDriver_PerSubLevelFilter` — fan-out filter respects per-sub min level.
- `TestStackDriver_PerSubLevel_RejectsUnknownLevel` — bad level fails at construction.
- `TestChannel_PerChannelLevelFilter` — `Config.LogLevel` drops records below threshold on a named channel.
- `TestChannelsFromEnv_RegistersDeclaredChannels` — full env path, per-channel level enforced.
- `TestChannelsFromEnv_NoopWhenUnset` — empty `OBSERVABILITY_CHANNELS` registers nothing.
- `TestChannelsFromEnv_RejectsPrimaryReserved` — `primary` cannot be listed.
- `TestChannelsFromEnv_RejectsDuplicate` — same name listed twice fails.
- `TestChannelsFromEnv_OrderingErrorWhenBeforeModule` — must be `.Use`d after `Module`.
- `TestLoadChannelConfigFromEnv_NormalisesName` — hyphenated channel names map to upper-case underscore segments.

### Changed (non-breaking)

- Roadmap: cloudwatch driver pushed 0.5.x → 0.6.x, storage 0.6.x → 0.7.x, cache → 0.8.x, queue → 0.9.x, httpx → 0.10.x. v0.5.x is dedicated to nailing Laravel `config/logging.php` parity.
- `observability.parseLogLevel` (internal) now delegates to `driver.ParseLogLevel` — single canonical implementation.
- `cloudwatch.ErrNotImplemented` and its package doc reference the new 0.6.0 target.

### Migration

None required. Existing wiring keeps working:

- `OBSERVABILITY_STACK_DRIVERS=stdout,file` continues to inherit `OBSERVABILITY_LOG_LEVEL` for both.
- `NewChannel(name, cfg)` is unchanged.
- `ChannelsFromEnv()` is opt-in — add it only when you want declarative channels.

## [0.4.0] — 2026-05-25

This release is a **structural breaking change** to put the framework on the same layout convention as `go-kit`, `OpenTelemetry Go`, `kratos`, and the team's own `go-common` — one package per concern at the root, public driver contract under `<module>/driver/`, built-in driver implementations split into one package each under `<module>/drivers/<name>/`, optional integration sub-packages (e.g. `<module>/middleware/`). The shape is fixed so that every future module (storage, cache, queue, httpx, ...) slots in identically.

No behaviour changes for the primary user paths (`obs.Logger()`, `obs.Tracer()`, `obs.Meter()`, env-driven driver selection, Laravel-style channels). The breaking changes are import paths and one method rename.

### Added — layout convention

- `observability/driver/` — new **public** package: `Driver` interface, `Spec`, `Constructor`, `Register / Lookup / Names / New`, plus shared helpers `SamplerFor` and `ResourceFor` for third-party drivers.
- `observability/register.go` — blank-imports the **light** built-in drivers (`stdout`, `file`, `stack`) so they auto-register when `observability` is imported, mirroring `database/sql` defaults.
- `observability/middleware/` — sub-package holding HTTP middleware. Optional dependency on `net/http`; non-HTTP services no longer transitively import it.
- `observability/doc.go` — package overview documenting the new layout.
- `docs/DRIVER_PATTERN.md` — **shared** convention for every current and future module. Describes the `<module>/driver/` + `<module>/drivers/<name>/` + `<module>/middleware/` skeleton, light vs heavy drivers, blank-import discipline, and the recipe for authoring a custom driver.
- `docs/modules/observability.md` — per-module reference (replaces and merges the old `OBSERVABILITY.md` and `DRIVERS.md`).
- `docs/README.md` — docs index.

### Changed (breaking)

- **Per-driver subpackages.** Each built-in driver now lives in its own package:

  | Old | New |
  |-----|-----|
  | `observability/drivers/stdout.go`     | `observability/drivers/stdout/stdout.go` (`package stdout`) |
  | `observability/drivers/file.go`       | `observability/drivers/file/file.go` (`package file`) |
  | `observability/drivers/otlp.go`       | `observability/drivers/otlp/otlp.go` (`package otlp`) |
  | `observability/drivers/stack.go`      | `observability/drivers/stack/stack.go` (`package stack`) |
  | `observability/drivers/cloudwatch.go` | `observability/drivers/cloudwatch/cloudwatch.go` (`package cloudwatch`) |
  | `observability/drivers/driver.go`     | `observability/driver/{driver,spec,registry,helpers}.go` (`package driver`) |

- **Driver registry instead of monolithic switch.** `drivers.New(ctx, spec)` is gone; each driver package registers itself via `init() { driver.Register(Name, New) }`. The observability package calls `driver.New(ctx, spec)` which dispatches via the registry. Missing registrations return a clear error pointing at the required blank import.

- **Light vs heavy driver split.** Light drivers (`stdout`, `file`, `stack`) stay auto-registered through `observability/register.go`. Heavy drivers (`otlp`, `cloudwatch`) now require an explicit blank import in consumer code:

  ```go
  import _ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp"
  ```

  This mirrors the `database/sql` convention and prevents non-OTLP services from pulling ~20MB of exporter dependencies. The future S3 / GCS / Redis / Kafka drivers will follow the same rule.

- **HTTP middleware moved to sub-package.** `Provider.Middleware(handler)` → `middleware.HTTP(provider)(handler)`. New import:

  ```go
  import "github.com/godx-jp/godx-platform-framework/observability/middleware"
  srv := &http.Server{Handler: middleware.HTTP(obs)(mux)}
  ```

  Constant moved: `observability.CorrelationHeader` → `middleware.CorrelationHeader`.

- **`file` driver constant rename.** `drivers.LogFileRotationNone/Daily/Size` → `file.RotationNone/Daily/Size` (in `observability/drivers/file`).

- **`cloudwatch` driver constant rename.** `drivers.ErrCloudWatchNotImplemented` → `cloudwatch.ErrNotImplemented` (in `observability/drivers/cloudwatch`).

- **Roadmap update.** CloudWatch driver moves 0.4.x → 0.5.0. Storage / cache / queue / httpx pushed back accordingly. v0.4 is dedicated to nailing the framework layout before more modules ship.

### Removed

- `docs/DRIVERS.md` — content merged into `docs/DRIVER_PATTERN.md` (cross-module) and `docs/modules/observability.md` (per-module).
- `docs/OBSERVABILITY.md` — content merged into `docs/modules/observability.md`.
- `observability/middleware.go` (root) — moved into `observability/middleware/http.go`.

### Migration

Drop-in upgrade for the common cases. Most apps need three textual changes:

```diff
 import (
     "github.com/godx-jp/godx-platform-framework/framework"
     "github.com/godx-jp/godx-platform-framework/observability"
+    "github.com/godx-jp/godx-platform-framework/observability/middleware" // HTTP only
+    _ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp" // OTLP only
 )

-srv := &http.Server{Handler: obs.Middleware(mux)}
+srv := &http.Server{Handler: middleware.HTTP(obs)(mux)}
```

Custom drivers built against the v0.3 `drivers.Driver` interface: change the package import from `.../observability/drivers` to `.../observability/driver` and rename the type references (`drivers.Driver` → `driver.Driver`, `drivers.Spec` → `driver.Spec`). Then register via `init() { driver.Register("yourname", New) }` instead of patching `drivers.New`.

## [0.3.0] — 2026-05-25

### Added — Laravel-style multi-channel logging

- `drivers.stack` — meta-driver that fans out every log record to N sub-drivers. Mirrors Laravel's `stack` log channel. Configure with `OBSERVABILITY_DRIVER=stack` and `OBSERVABILITY_STACK_DRIVERS=stdout,file` (comma-separated). Sub-drivers inherit the rest of the spec, so OTLP / file settings flow through. Nesting (`stack` inside `stack`) is rejected at construction. Traces and metrics use the first sub-driver only — duplicating spans across exporters would produce double-counted distributed traces.
- `observability.NewChannel(name, cfg)` — framework module that registers an additional named channel on top of the primary one. Order matters: `observability.Module` must be `Use`d before any `NewChannel`.
- `Provider.Channel(name) *slog.Logger` — Laravel-style per-call channel selection (`obs.Channel("audit").Info(...)`). Unknown channels fall back to the primary logger with a warn line (never panics).
- `Provider.Channels() []string` — list registered channel names for diagnostics / admin endpoints.
- `PrimaryChannel = "primary"` constant — reserved name for the default channel; cannot be overridden via `NewChannel`.
- `Config.StackDrivers []string` — new field; `OBSERVABILITY_STACK_DRIVERS` env var (comma-separated, whitespace tolerated).
- Five new test files / suites: `stack_test.go` (fan-out, nested rejection, missing list, unknown sub-driver, env parsing), `channel_test.go` (routing, reserved name, fallback, ordering error, `Channels()` lookup).

### Changed
- Roadmap: CloudWatch driver moves 0.3.0 → 0.4.0; httpx moves 0.4.0 → 0.5.0.
- Default channel is now formally named `"primary"`. Existing code that calls `obs.Logger()` is unchanged.

## [0.2.0] — 2026-05-25

This release is a **breaking rename** to clarify naming. No behaviour changes. Pin to `v0.1.0` if you need the old names; upgrade by replacing `OBS_*` → `OBSERVABILITY_*` and `Backend` → `Driver` in your code and env files.

### Added (carried from unreleased 0.1.x)
- `observability/drivers/file.go` — Laravel-style local file log driver (`none` / `daily` / `size` rotation, gzip, retention) for zero-budget / bare-metal / VM deployments where no log collector is available. Uses `gopkg.in/natefinch/lumberjack.v2`. Parent directory auto-created.

### Changed (breaking)
- **Package rename**: `observability/backends/` → `observability/drivers/`.
- **Interface rename**: `backends.Backend` → `drivers.Driver`. The Go convention (`database/sql.Driver`) and Laravel terminology both use "driver" for the swappable implementation.
- **Type rename**: `backends.Spec` → `drivers.Spec`.
- **Const rename**: `BackendStdout`/`BackendFile`/`BackendOTLP`/`BackendCloudWatch` → `DriverStdout`/`DriverFile`/`DriverOTLP`/`DriverCloudWatch`.
- **Config field rename**: `Config.Backend` → `Config.Driver`. `Config.FilePath` → `Config.LogFilePath`. `Config.FileRotate` → `Config.LogFileRotation`. `Config.File*` → `Config.LogFile*` (all file-driver fields prefixed with `LogFile`). `Config.LogGroupName` → `Config.CloudWatchLogGroup`.
- **Method rename**: `Provider.Backend()` → `Provider.Driver()`.
- **File rotation consts**: `FileRotateNone/Daily/Size` → `LogFileRotationNone/Daily/Size` (now in `drivers` package).
- **Env var rename** — all SDK env vars now use the full word `OBSERVABILITY_*` (no abbreviations); industry-standard vars (`OTEL_*`, `AWS_*`, `DEPLOYMENT_ENVIRONMENT`) are unchanged.

  | Old | New |
  |-----|-----|
  | `OBS_BACKEND` | `OBSERVABILITY_DRIVER` |
  | `OBS_LOG_LEVEL` | `OBSERVABILITY_LOG_LEVEL` |
  | `OBS_TRACE_SAMPLE` | `OBSERVABILITY_TRACE_SAMPLE_RATE` |
  | `OBS_LOG_FILE` | `OBSERVABILITY_LOG_FILE_PATH` |
  | `OBS_LOG_ROTATE` | `OBSERVABILITY_LOG_FILE_ROTATION` |
  | `OBS_LOG_MAX_SIZE_MB` | `OBSERVABILITY_LOG_FILE_MAX_SIZE_MB` |
  | `OBS_LOG_MAX_AGE_DAYS` | `OBSERVABILITY_LOG_FILE_MAX_AGE_DAYS` |
  | `OBS_LOG_MAX_BACKUPS` | `OBSERVABILITY_LOG_FILE_MAX_BACKUPS` |
  | `OBS_LOG_COMPRESS` | `OBSERVABILITY_LOG_FILE_COMPRESS` |
  | `OBS_LOG_GROUP` | `OBSERVABILITY_CLOUDWATCH_LOG_GROUP` |

- **Doc rename**: `docs/BACKENDS.md` → `docs/DRIVERS.md`, with a new "Vocabulary" section explaining driver-vs-backend.
- Roadmap: CloudWatch driver moves from 0.2.0 → 0.3.0; 0.3.0 now lands AWS ADOT + configurable correlation header; httpx targeted for 0.2.x → 0.3.x.

## [0.1.0] — 2026-05-25

### Added
- `framework/` — module-based application backbone with graceful shutdown.
- `observability/` — drop-in observability module covering structured logging, OpenTelemetry traces, and metrics.
- `observability/backends/` — pluggable backend drivers:
  - `stdout` — JSON to stdout, in-process tracer (dev / tests).
  - `otlp` — OTLP gRPC / HTTP exporter (Loki/Tempo/Prometheus, Datadog, New Relic, any OTLP-compatible).
  - `cloudwatch` — placeholder stub for AWS Distro for OpenTelemetry (full implementation tracked for 0.2.0).
- `observability` HTTP middleware: trace context, correlation ID, request log.
- `examples/minimal`, `examples/http-server`.
- Configuration via environment variables (12-factor) with sensible defaults.
- Apache 2.0 license.
