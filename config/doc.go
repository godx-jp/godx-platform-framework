// Package config implements a layered configuration repository for
// framework apps — Laravel's Config facade reimagined for Go.
//
// A Repository merges values from one or more Sources (env vars,
// files, remote KV stores) in registration order — later sources
// override earlier ones (envFromOS by default sits last so process
// env always wins). Typed accessors mirror Laravel's
// Config::get/string/int/bool/array helpers; the generic Get[T]
// function returns a typed value or a default.
//
//	app := framework.New("svc", "1.0.0").Use(config.Module)
//	if err := app.Init(ctx); err != nil { return err }
//	cfg, _ := config.FromApp(app)
//	port := cfg.GetInt("server.port", 8080)
//
// Drivers live under config/drivers/<name>/ and follow the project's
// driver-pattern convention. Light drivers (env, file) auto-register;
// heavy drivers (remote KV systems like etcd or consul) require a
// blank import.
//
// Laravel mapping:
//
//	Laravel                                | Framework
//	---------------------------------------|----------------------------
//	Config::get('app.name', 'default')     | cfg.Get("app.name", "default")
//	Config::string / int / bool / array    | cfg.GetString / GetInt / GetBool / GetSlice
//	Config::set('app.name', 'X')           | cfg.Set("app.name", "X")
//	Config::has('app.name')                | cfg.Has("app.name")
//	config:cache (compile to flat array)   | repository.AllFlat()
//	Source order in config/config.php      | CONFIG_SOURCES env var
//
// Nested keys use dot notation; map and slice values can be addressed
// through the same path. See docs/modules/config.md for the full
// reference.
package config
