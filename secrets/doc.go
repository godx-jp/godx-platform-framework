// Package secrets implements a Laravel-style secrets manager — a
// uniform Get/Put/Forget interface backed by env vars, file mounts,
// HashiCorp Vault, Google Secret Manager, or AWS Secrets Manager.
// Drivers expose the same surface, so applications fetch credentials
// the same way in dev (env) and production (vault / cloud KMS).
//
//	app := framework.New("svc", "1.0.0").Use(secrets.Module)
//	if err := app.Init(ctx); err != nil { return err }
//	mgr, _ := secrets.FromApp(app)
//	dbPass, _ := mgr.GetString(ctx, "db/password")
//
// Secrets are bytes by convention; GetString / PutString are thin
// wrappers around the byte API. Use the encryption module if you
// also need ciphertext-at-rest semantics on top of the chosen
// secret backend.
//
// Driver matrix:
//
//	env     - read SECRETS_ENV_PREFIX + key (light, auto)
//	file    - read <root>/<key> as raw file content (light, auto)
//	vault   - HashiCorp Vault KV v2 (heavy, opt-in)
//	gcpsm   - Google Cloud Secret Manager (heavy, opt-in)
//	awssm   - AWS Secrets Manager (heavy, opt-in)
//
// Light drivers register themselves via init(); heavy drivers
// require an explicit blank import so binaries that only need env
// or file stay free of the cloud SDKs.
package secrets
