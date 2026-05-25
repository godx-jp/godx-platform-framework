// Package auth provides swappable authentication guards (jwt, apikey,
// introspect) with principal context helpers and named gate registry.
//
//	app := framework.New("svc", "1.0.0").Use(auth.Module)
//	_ = app.Init(ctx)
//	mgr, _ := auth.FromApp(app)
//	principal, err := mgr.Authenticate(ctx, &driver.CredentialRequest{Token: bearer})
//
// Drivers:
//
//	jwt        - Bearer JWT validated against JWKS (blank import)
//	apikey     - static API keys via header (blank import)
//	introspect - OAuth2 token introspection stub (blank import)
package auth
