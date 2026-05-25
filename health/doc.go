// Package health provides Kubernetes-style liveness and readiness probes.
//
//	app.Use(health.Module)
//	health.FromApp(app).RegisterProbe("db", dbPing)
package health
