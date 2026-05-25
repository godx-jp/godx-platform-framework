package observability

// Light, dependency-free drivers are auto-registered so out-of-the-box every
// service has stdout, file, and stack support without extra blank imports.
//
// Heavy drivers (otlp, cloudwatch, and future cloud-provider integrations)
// stay opt-in — consumers add an explicit blank import to enable them, e.g.
//
//	import _ "github.com/godx-jp/godx-platform-framework/observability/drivers/otlp"
//
// This mirrors the database/sql / image-format convention in the Go
// standard library: you pay for the drivers you actually use.
import (
	_ "github.com/godx-jp/godx-platform-framework/observability/drivers/file"
	_ "github.com/godx-jp/godx-platform-framework/observability/drivers/stack"
	_ "github.com/godx-jp/godx-platform-framework/observability/drivers/stdout"
)
