# Encryption

> Authenticated symmetric encryption — Laravel's `Crypt` facade
> reimagined for Go. Self-describing tokens, versioned key ring,
> rotation-safe.

## Concepts

An `Encrypter` wraps a `Cipher` (AES-GCM or ChaCha20-Poly1305) plus a versioned key ring. Each encryption uses the **primary** key; each decryption resolves the token's embedded key id against the ring — so adding `AddKey + SetPrimary` rotates without invalidating old ciphertext.

```
Encrypter ── key ring [id → 32-byte key, plus a "primary" pointer]
   │
   ├─ Encrypt(plain)   → "v1:<primary-id>:<base64(nonce|ct|tag)>"
   └─ Decrypt(token)   → lookup id in ring → cipher.Decrypt
        └─ driver.Cipher (aesgcm · chacha20poly1305)
```

## Quick start

```go
import (
    "github.com/godx-jp/godx-platform-framework/encryption"
    "github.com/godx-jp/godx-platform-framework/framework"
)

app := framework.New("svc", "1.0.0").Use(encryption.Module)
_ = app.Init(ctx)        // requires ENCRYPTION_KEY

enc, _ := encryption.FromApp(app)
token, _ := enc.EncryptString(ctx, "secret-value")
plain, _ := enc.DecryptString(ctx, token)
```

`ENCRYPTION_KEY` is required (Laravel-style `APP_KEY`):

```bash
ENCRYPTION_KEY=base64:$(openssl rand -base64 32)
```

## Token format

```
v1:<key-id>:<base64-url-no-pad(nonce || ciphertext || tag)>
```

- `v1` — version prefix, lets us evolve the format later.
- `<key-id>` — which key from the ring was used. Defaults to `k1`; override with `ENCRYPTION_PRIMARY_KEY_ID`.
- Encoded body — `nonce` first (12 bytes for both AEADs), then ciphertext+tag (AEAD `Seal` appends the tag).

`encryption.KeyIDOf(token)` exposes the embedded id for rotation tooling that wants to skip already-current rows.

## Key rotation

```go
enc, _ := encryption.FromApp(app)

// Phase 1 — push the new key into the ring, keep primary unchanged.
//           Old tokens continue to decrypt; new writes still use k1.
_ = enc.AddKey("k2", newKey)

// Phase 2 — flip the primary. Old tokens still decrypt under k1.
_ = enc.SetPrimary("k2")

// Phase 3 — later, after a sweep that re-encrypts every k1 row to k2,
//           drop k1 from the ring (no API call needed today — just
//           omit it from the next ENCRYPTION_PREVIOUS_KEYS env var).
```

The Manager publishes a single `*Encrypter` into the App; rotation can also happen at deploy time by setting `ENCRYPTION_KEY` to the new key and listing the old key under `ENCRYPTION_PREVIOUS_KEYS`:

```bash
ENCRYPTION_KEY=base64:NEWKEY
ENCRYPTION_PRIMARY_KEY_ID=k2
ENCRYPTION_PREVIOUS_KEYS="k1=base64:OLDKEY"
```

## Drivers

| Driver | Status | Key size | Notes |
|---|---|---|---|
| `aesgcm`           | stable | 32 | Laravel default. AES-256-GCM. Hardware-accelerated on AES-NI. |
| `chacha20poly1305` | stable | 32 | Faster than AES on platforms without AES-NI (ARM/embedded). |

Both auto-register when the `encryption` package is imported.

## Env var reference

| Var | Purpose | Default |
|---|---|---|
| `ENCRYPTION_DRIVER` | Cipher driver name | `aesgcm` |
| `ENCRYPTION_KEY` | Primary key, e.g. `base64:<32-byte key>` or `hex:<64 hex chars>` | **required** |
| `ENCRYPTION_PRIMARY_KEY_ID` | ID assigned to the primary key | `k1` |
| `ENCRYPTION_PREVIOUS_KEYS` | Comma list of `id=base64:<key>` registered alongside primary | _empty_ |

`ParseKey` accepts three formats:

- `base64:<RawStdBase64>` — Laravel `APP_KEY` layout.
- `hex:<hex>` — handy for `openssl rand -hex 32` output.
- raw bytes — anything not prefixed is treated as a literal key.

## Laravel API mapping

| Laravel | Framework |
|---|---|
| `Crypt::encryptString($plain)` | `enc.EncryptString(ctx, plain)` |
| `Crypt::decryptString($token)` | `enc.DecryptString(ctx, token)` |
| `Crypt::encrypt(...)` with serialise | `enc.Encrypt(ctx, json.Marshal(payload))` |
| `Crypt::generateKey($cipher)` | `crypto/rand.Read(make([]byte, 32))` |
| `APP_KEY` rotation | `Encrypter.AddKey + SetPrimary`, or `ENCRYPTION_PREVIOUS_KEYS` |

## Migrating from go-common

`umbrella/packages/go-common` has hand-rolled `crypto/aes` + nonce management in a handful of services. Replace those:

| Before | After |
|---|---|
| `aes.NewCipher / Seal / Open` in each service | `enc.Encrypt / Decrypt` |
| Per-service base64 envelope | Built into the `v1:<id>:<base64>` token |
| One key per service in env | One `ENCRYPTION_KEY`; service ids in token decouple consumers |
| No rotation story | `AddKey + SetPrimary` or `ENCRYPTION_PREVIOUS_KEYS` |

## Security notes

- Always use the Manager — direct `cipher.Encrypt` exposes the raw 32-byte key surface and skips the version+id envelope.
- Nonces are random per `Encrypt` call; never reuse a key after >2³² encryptions (about 4 billion). Rotate well before then.
- `Decrypt` returns `ErrAuthFailed` on tag-mismatch; treat it as "tampered or wrong key" and never echo the input.
- 32-byte keys only. Generate with `openssl rand -base64 32` or `crypto/rand`.

## Out of scope

- **Asymmetric crypto** — out of scope; use stdlib `crypto/rsa` or `crypto/ed25519` directly.
- **JWT signing** — handled by your auth layer; encryption ≠ signing.
- **Key management** — see the upcoming `secrets` module (v0.8.5) for Vault / GCP SM / AWS SM integration.
