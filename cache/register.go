package cache

// Light drivers register themselves under canonical names via init()
// when the parent cache package is imported. Heavy drivers (redis)
// require an explicit blank import in the consumer's main package so
// binaries that only need the in-process stores stay free of the
// redis SDK.
import (
	_ "github.com/godx-jp/godx-platform-framework/cache/drivers/file"
	_ "github.com/godx-jp/godx-platform-framework/cache/drivers/memory"
)
