// Package events implements an in-process event bus — Laravel's
// Event facade reimagined for Go. Listeners subscribe by event name
// or wildcard pattern; the dispatcher fans every fired event out to
// every matching listener.
//
//	bus := events.New()
//	bus.Listen("user.created", func(ctx context.Context, e events.Event) error {
//	    log.Printf("welcome email for %s", e.Payload)
//	    return nil
//	})
//	bus.Listen("user.*", auditListener)              // wildcard
//	_ = bus.Dispatch(ctx, events.Event{
//	    Name:    "user.created",
//	    Payload: user,
//	})
//
// The default Bus dispatches synchronously: Dispatch returns after
// every listener has run, with any errors joined. Wrap a Bus in
// NewAsync(bus, AsyncOptions{Workers: 4, OnError: hook}) when
// fire-and-forget semantics are required (callers no longer see
// listener errors — the async layer surfaces them through its error
// hook).
//
// Module wiring publishes the bus under events.StoreKey so any
// other module (mail, notifications, queue) can subscribe at init
// time.
//
//	app := framework.New("svc", "1.0.0").Use(events.Module)
//	if err := app.Init(ctx); err != nil { return err }
//	bus, _ := events.FromApp(app)
//	bus.Listen("user.*", ...)
//
// Laravel mapping:
//
//	Laravel                                  | Framework
//	-----------------------------------------|----------------------------
//	Event::listen('user.created', fn(...))    | bus.Listen("user.created", fn)
//	Event::listen('user.*', fn(...))          | bus.Listen("user.*", fn)  (wildcard)
//	Event::dispatch(new UserCreated($user))    | bus.Dispatch(ctx, events.Event{Name: "user.created", Payload: u})
//	Event::forget('user.created')              | bus.Forget("user.created")
//	queue listeners (ShouldQueue)             | NewAsync(bus, AsyncOptions{Workers: N})
package events
