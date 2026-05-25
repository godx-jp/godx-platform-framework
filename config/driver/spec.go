package driver

// Spec is the uniform input to every config driver constructor.
// Drivers consume only the fields they recognise; extension config
// goes into Extra so adding a new driver does not require modifying
// this struct.
type Spec struct {
	// Name is the driver name (env, file, …). The framework
	// dispatches Constructor lookups by this field.
	Name string

	// Prefix scopes an env or remote source to a subset of keys
	// (env driver: e.g. "APP_" → only APP_* vars; remote driver:
	// node prefix). Optional.
	Prefix string

	// ── file driver ──────────────────────────────────────────────
	// Path points at the file to load. Required for the file driver.
	// Format is inferred from the extension (.yaml/.yml, .json, .toml).
	Path string

	// Format optionally overrides the auto-detected file format
	// ("yaml", "json", "toml"). Useful when the file extension lies.
	Format string

	// Optional, when true a missing file becomes an empty tree
	// instead of an error. Used for layering optional overrides.
	Optional bool

	// ── remote driver ────────────────────────────────────────────
	// Address is host:port for the remote KV system (etcd, consul, …).
	Address string

	// URL takes precedence over Address when set, e.g.
	// "etcd://user:pass@host:2379/services/svc".
	URL string

	// Token authenticates against the remote system (consul ACL,
	// etcd JWT, …). Optional.
	Token string

	// Extra carries driver-specific extension config so adding a new
	// driver does not require modifying this struct.
	Extra map[string]string
}
