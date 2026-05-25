package driver

// Spec is the uniform input to every secrets driver constructor.
type Spec struct {
	// Name is the driver name (env, file, vault, gcpsm, awssm).
	Name string

	// Prefix scopes a backend to a key subtree.
	//   env    - SECRETS_<PREFIX> + key (uppercased)
	//   file   - root directory; key becomes a sub-path
	//   vault  - KV mount path prefix
	//   gcpsm  - secret id prefix
	//   awssm  - secret id prefix
	Prefix string

	// ── file driver ──────────────────────────────────────────────
	// Path is the on-disk root for the file driver. Required.
	Path string

	// ── vault driver ─────────────────────────────────────────────
	// Address is the Vault API endpoint (e.g. https://vault:8200).
	Address string
	// Token authenticates against Vault.
	Token string
	// KVMount is the KV-v2 mount path (default "secret").
	KVMount string

	// ── gcpsm driver ─────────────────────────────────────────────
	// Project is the GCP project id (overrides ADC default when set).
	Project string

	// ── awssm driver ─────────────────────────────────────────────
	// Region is the AWS region (overrides AWS_REGION when set).
	Region string

	// Extra carries driver-specific extension config.
	Extra map[string]string
}
