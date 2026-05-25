package slack

import ndriver "github.com/godx-jp/godx-platform-framework/notifications/driver"

// SetHTTPClient replaces the HTTP client on a channel from New.
func SetHTTPClient(ch ndriver.Channel, client ndriver.HTTPDoer) {
	if c, ok := ch.(*channel); ok && client != nil {
		c.client = client
	}
}
