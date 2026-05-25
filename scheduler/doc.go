// Package scheduler runs recurring jobs from cron expressions — Laravel
// Schedule parity for Go services.
//
//	sched := scheduler.New()
//	sched.EveryMinute().Do("heartbeat", func(ctx context.Context) error {
//	    return ping(ctx)
//	})
//	sched.Cron("0 2 * * *").WithoutOverlapping().OnOneServer().Do("nightly", nightly)
//	sched.Start(ctx)
//	defer sched.Stop()
//
// WithoutOverlapping uses an in-process mutex so a slow run cannot stack
// on top of itself. OnOneServer acquires a distributed lock through
// scheduler/lock.CacheStore (the cache module's Store satisfies this
// interface) so only one replica executes the job.
//
// Module wiring publishes the Scheduler under scheduler.StoreKey:
//
//	app := framework.New("svc", "1.0.0").Use(scheduler.Module)
//	if err := app.Init(ctx); err != nil { return err }
//	sched, _ := scheduler.FromApp(app)
package scheduler
