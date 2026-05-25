package config

// Light drivers register themselves under canonical names via init()
// when the parent config package is imported. Heavy drivers (remote
// KV stores like etcd or consul) require an explicit blank import in
// the consumer's main package so binaries that only need env+file
// stay free of the etcd/consul SDKs.
import (
	_ "github.com/godx-jp/godx-platform-framework/config/drivers/env"
	_ "github.com/godx-jp/godx-platform-framework/config/drivers/file"
	_ "github.com/godx-jp/godx-platform-framework/config/drivers/static"
)
