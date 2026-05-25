// Package featureflag evaluates feature toggles from pluggable providers —
// Laravel feature-flag parity with optional per-eval caching.
//
//	ok, err := eval.Enabled(ctx, "new-checkout", userID, map[string]any{"tier": "pro"})
//
// Drivers: config (reads bool keys from the config module), plus heavy
// opt-in stubs for openfeature, launchdarkly, unleash, and flagsmith.
//
//	app := framework.New("svc", "1.0.0").
//	    Use(config.Module).
//	    Use(featureflag.Module)
//	eval, _ := featureflag.FromApp(app)
package featureflag
