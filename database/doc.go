// Package database is a Laravel-style multi-connection database manager for
// the godx platform framework.
//
//	app := framework.New("svc", "1.0.0").Use(database.Module)
//	if err := app.Init(ctx); err != nil { return err }
//	mgr, _ := database.FromApp(app)
//	pool := mgr.Default().Postgres()
package database
