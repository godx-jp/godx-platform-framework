package mail

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/events"
	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
)

// Mailer is a fluent builder for one outbound message.
type Mailer struct {
	mgr       *Manager
	transport mdriver.Transport
	bus       events.Bus

	from    string
	to      []string
	subject string
	body    string
	html    string
}

// To sets recipients.
func (ml *Mailer) To(addrs ...string) *Mailer {
	ml.to = append([]string(nil), addrs...)
	return ml
}

// From overrides the default sender.
func (ml *Mailer) From(addr string) *Mailer {
	ml.from = addr
	return ml
}

// Subject sets the subject line.
func (ml *Mailer) Subject(s string) *Mailer {
	ml.subject = s
	return ml
}

// Body sets the plain-text body.
func (ml *Mailer) Body(b string) *Mailer {
	ml.body = b
	return ml
}

// HTML sets an HTML body (used by HTML-capable transports).
func (ml *Mailer) HTML(h string) *Mailer {
	ml.html = h
	return ml
}

// Send delivers the message via the bound transport and emits
// mail.sending / mail.sent / mail.failed when an events Bus is wired.
func (ml *Mailer) Send(ctx context.Context) error {
	if ml.transport == nil {
		return fmt.Errorf("mail: no transport configured")
	}
	if len(ml.to) == 0 {
		return fmt.Errorf("mail: at least one recipient is required")
	}
	from := ml.from
	if from == "" && ml.mgr != nil {
		ml.mgr.mu.RLock()
		from = ml.mgr.from
		ml.mgr.mu.RUnlock()
	}
	msg := mdriver.Message{
		From:     from,
		To:       ml.to,
		Subject:  ml.subject,
		Body:     ml.body,
		HTMLBody: ml.html,
	}
	payload := map[string]any{
		"driver":  ml.transport.Name(),
		"from":    from,
		"to":      ml.to,
		"subject": ml.subject,
	}
	if ml.mgr != nil {
		ml.mgr.dispatch(ctx, EventSending, payload)
	} else if ml.bus != nil {
		_ = ml.bus.Dispatch(ctx, events.Event{Name: EventSending, Payload: payload})
	}
	if err := ml.transport.Send(ctx, msg); err != nil {
		payload["error"] = err.Error()
		if ml.mgr != nil {
			ml.mgr.dispatch(ctx, EventFailed, payload)
		} else if ml.bus != nil {
			_ = ml.bus.Dispatch(ctx, events.Event{Name: EventFailed, Payload: payload})
		}
		return err
	}
	if ml.mgr != nil {
		ml.mgr.dispatch(ctx, EventSent, payload)
	} else if ml.bus != nil {
		_ = ml.bus.Dispatch(ctx, events.Event{Name: EventSent, Payload: payload})
	}
	return nil
}
