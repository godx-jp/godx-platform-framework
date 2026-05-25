// Light ratelimit drivers register themselves on import. The redis
// driver requires an explicit blank import to keep go-redis out of
// binaries that only need in-process limiting.
package ratelimit

import (
	_ "github.com/godx-jp/godx-platform-framework/ratelimit/drivers/memory"
)
