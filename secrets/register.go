// Light secrets drivers register themselves on import. Heavy
// drivers (vault, gcpsm, awssm) require an explicit blank import
// from the consumer to keep their large SDKs out of binaries that
// only ever need env / file storage.
package secrets

import (
	_ "github.com/godx-jp/godx-platform-framework/secrets/drivers/env"
	_ "github.com/godx-jp/godx-platform-framework/secrets/drivers/file"
)
