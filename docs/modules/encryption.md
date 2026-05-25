# Encryption

> Authenticated symmetric encryption — Laravel's `Crypt` facade
> reimagined for Go. Self-describing tokens, a versioned key ring,
> and rotation-safe decryption.

## Concepts

An `Encrypter` wraps a `Cipher` (AES-GCM or ChaCha20-Poly1305) plus a versioned key ring. Each encryption uses the **primary** key; each decryption resolves the token's embedded key id against the ring — so `AddKey` + `SetPrimary` rotates without invalidating old ciphertext.

```
Encrypter ── key ring [id → 32-byte key] + "primary" pointer
   │
   ├─ Encrypt(plain)   → "v1:<primary-id>:<base64(nonce|ct|tag)>"
   └─ Decrypt(token)   → lookup id in ring → cipher.Decrypt
        └─ driver.Cipher (aesgcm · chacha20poly1305)
```

## Quick start

```go
package main

import (
    "context"

    "github.com/godx-jp/godx-platform-framework/encryption"
    "github.com/godx-jp/godx-platform-framework/framework"
)

func main() {
    ctx := context.Background()
    app := framework.New("svc", "1.0.0").Use(encryption.Module)
    if err := app.Init(ctx); err != nil { panic(err) } // requires ENCRYPTION_KEY
    defer app.Shutdown(ctx)

    enc, _ := encryption.FromApp(app)
    token, _ := enc.EncryptString(ctx, "secret-value")
    plain, _ := enc.DecryptString(ctx, token)
    _ = plain
}
```

`ENCRYPTION_KEY` is required (Laravel-style `APP_KEY`):

```bash
ENCRYPTION_KEY=base64:$(openssl rand -base64 32)
```

For tests and scripts, `encryption.MustNew("base64:<32-byte key>")` builds an aesgcm-backed `*Encrypter` with the single key registered under id `k1`, panicking on bad input.

## Programmatic config

```go
cfg := encryption.Config{
    Driver:       "aesgcm",
    PrimaryKeyID: "k2",
    PrimaryKey:   newKey,                       // 32 bytes
    Previous:     []encryption.KeyEntry{{ID: "k1", Key: oldKey}},
}
app := framework.New(...).Use(encryption.ModuleWithConfig(cfg))
```

## Token format

```
v1:<key-id>:<base64-url-no-pad(nonce || ciphertext || tag)>
```

- `v1` — version prefix, lets the format evolve later.
- `<key-id>` — which key from the ring was used. Defaults to `k1`; override with `ENCRYPTION_PRIMARY_KEY_ID`.
- Encoded body — `nonce` first, then ciphertext+tag (the AEAD `Seal` appends the tag). Encoded with `base64.RawURLEncoding`.

`encryption.KeyIDOf(token)` exposes the embedded id for rotation tooling that wants to skip already-current rows.

## Key rotation

```go
enc, _ := encryption.FromApp(app)

// Phase 1 — push the new key into the ring, keep primary unchanged.
//           Old and new tokens both decrypt; new writes still use the old primary.
_ = enc.AddKey("k2", newKey)

// Phase 2 — flip the primary. Old tokens still decrypt under the prior key.
_ = enc.SetPrimary("k2")

// Phase 3 — after a sweep re-encrypts every old row, drop the old key
//           by omitting it from the next ENCRYPTION_PREVIOUS_KEYS value.
```

Rotation can also happen at deploy time by setting `ENCRYPTION_KEY` to the new key and listing the old key under `ENCRYPTION_PREVIOUS_KEYS`:

```bash
ENCRYPTION_KEY=base64:NEWKEY
ENCRYPTION_PRIMARY_KEY_ID=k2
ENCRYPTION_PREVIOUS_KEYS="k1=base64:OLDKEY"
```

`ENCRYPTION_PREVIOUS_KEYS` is a comma-separated list of `id=<encoded key>` entries; each key uses the same `ParseKey` formats as `ENCRYPTION_KEY`.

## Driver matrix

| Driver | Status | Registration | Key size | Notes |
|--------|--------|--------------|----------|-------|
| `aesgcm` | stable | auto | 32 | Laravel default. AES-256-GCM. Hardware-accelerated on AES-NI. 12-byte nonce |
| `chacha20poly1305` | stable | auto | 32 | Faster than AES where there's no AES-NI (ARM/embedded). 12-byte nonce |

Both ciphers auto-register when the `encryption` package is imported (they are blank-imported by `encryption/register.go`), so neither needs an explicit import. Selecting an unregistered driver fails at module init with a hint naming the missing import path.

## Env var reference

| Var | Purpose | Default |
|-----|---------|---------|
| `ENCRYPTION_DRIVER` | Cipher driver name | `aesgcm` |
| `ENCRYPTION_KEY` | Primary key, e.g. `base64:<32-byte key>` or `hex:<64 hex chars>` | **required** |
| `ENCRYPTION_PRIMARY_KEY_ID` | ID assigned to the primary key | `k1` |
| `ENCRYPTION_PREVIOUS_KEYS` | Comma list of `id=<encoded key>` registered alongside the primary | _empty_ |

`encryption.ParseKey` accepts three formats:

- `base64:<RawStdBase64>` — Laravel `APP_KEY` layout (standard base64 after the prefix).
- `hex:<hex>` — handy for `openssl rand -hex 32` output.
- raw bytes — anything not prefixed is treated as a literal key.

## API reference

| Symbol | Description |
|--------|-------------|
| `enc.Encrypt(ctx, []byte)` / `EncryptString(ctx, string)` | Seal with the primary key → token |
| `enc.Decrypt(ctx, token)` / `DecryptString(ctx, token)` | Resolve the token's key id and open |
| `enc.AddKey(id, key)` / `SetPrimary(id)` | Manage the key ring |
| `enc.PrimaryKeyID()` / `KeyIDs()` / `CipherName()` | Introspection |
| `encryption.NewEncrypter(cipher)` | Construct an empty Encrypter (add a key before use) |
| `encryption.MustNew(keyEncoded)` | Test/script constructor (aesgcm, id `k1`) |
| `encryption.ParseKey(s)` / `KeyIDOf(token)` | Decode a key string / read a token's key id |
| `encryption.FromApp(app)` | Encrypter from the framework store |

## Error model

```go
plain, err := enc.Decrypt(ctx, token)
switch {
case errors.Is(err, driver.ErrInvalidToken): // malformed token (version/id/base64)
case errors.Is(err, driver.ErrUnknownKey):   // key id not in the ring
case errors.Is(err, driver.ErrAuthFailed):   // tag mismatch — tampered or wrong key
}
```

`AddKey` returns `driver.ErrInvalidKeySize` (wrapped) when the key length does not match the cipher; the ciphers also return `driver.ErrShortCiphertext` when sealed input is shorter than the AEAD overhead. Never log ciphertext on the `ErrAuthFailed` path.

## Context propagation

`encryption.ContextWithEncrypter(ctx, enc)` attaches the Encrypter to a context; `encryption.FromContext(ctx)` reads it back. `encryption.FromApp(app)` is the canonical way to retrieve the Encrypter built by `encryption.Module`.

## Lifecycle

`encryption.Module` publishes a single `*Encrypter` into the framework store under `StoreKey`. The ciphers hold no external resources, so the module registers **no** `OnShutdown` hook — there is nothing to tear down. Only one `encryption.Module` per `App`; a second `Init` returns `encryption: Module already initialised`.

## Laravel API mapping

| Laravel | Framework |
|---------|-----------|
| `Crypt::encryptString($plain)` | `enc.EncryptString(ctx, plain)` |
| `Crypt::decryptString($token)` | `enc.DecryptString(ctx, token)` |
| `Crypt::encrypt(...)` with serialise | `enc.Encrypt(ctx, json.Marshal(payload))` |
| `Crypt::generateKey($cipher)` | `crypto/rand.Read(make([]byte, 32))` |
| `APP_KEY` rotation | `enc.AddKey + SetPrimary`, or `ENCRYPTION_PREVIOUS_KEYS` |

## Security notes

- Always use the Encrypter — calling `cipher.Encrypt` directly exposes the raw key surface and skips the version+id envelope.
- Nonces are random per `Encrypt` call; never reuse a key past ~2³² encryptions. Rotate well before then.
- `Decrypt` returns `ErrAuthFailed` on a tag mismatch; treat it as "tampered or wrong key" and never echo the input.
- 32-byte keys only. Generate with `openssl rand -base64 32` or `crypto/rand`.

## Out of scope

- **Asymmetric crypto** — use stdlib `crypto/rsa` or `crypto/ed25519` directly.
- **JWT signing** — handled by the auth layer; encryption ≠ signing.
- **Key management** — see the upcoming `secrets` module (v0.8.5) for Vault / GCP SM / AWS SM integration.
