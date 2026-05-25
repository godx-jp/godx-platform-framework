package notifications

import (
	"context"
	"fmt"
	"net/http"

	"github.com/godx-jp/godx-platform-framework/httpclient"
	"github.com/godx-jp/godx-platform-framework/mail"
	ndriver "github.com/godx-jp/godx-platform-framework/notifications/driver"
	mailchannel "github.com/godx-jp/godx-platform-framework/notifications/drivers/mail"
	databasechannel "github.com/godx-jp/godx-platform-framework/notifications/drivers/database"
	"github.com/godx-jp/godx-platform-framework/notifications/drivers/discord"
	"github.com/godx-jp/godx-platform-framework/notifications/drivers/slack"
	"github.com/godx-jp/godx-platform-framework/notifications/drivers/webhook"
)

type buildDeps struct {
	mailMgr       *mail.Manager
	httpClient    *httpclient.Client
	databaseStore ndriver.DatabaseStore
}

func buildChannel(ctx context.Context, cc ChannelConfig, deps buildDeps) (ndriver.Channel, error) {
	spec := cc.Spec
	if spec.Name == "" {
		spec.Name = cc.Driver
	}
	switch cc.Driver {
	case ndriver.DriverMail:
		if deps.mailMgr == nil {
			return nil, fmt.Errorf("notifications: mail channel requires mail.Module")
		}
		return mailchannel.New(deps.mailMgr, spec.Mailer), nil
	case ndriver.DriverDatabase:
		if deps.databaseStore == nil {
			return nil, fmt.Errorf("notifications: database channel requires DatabaseStore")
		}
		return databasechannel.New(deps.databaseStore), nil
	default:
		ch, err := ndriver.New(ctx, spec)
		if err != nil {
			return nil, err
		}
		if deps.httpClient != nil {
			doer := &httpClientDoer{client: deps.httpClient}
			switch cc.Driver {
			case ndriver.DriverWebhook:
				webhook.SetHTTPClient(ch, doer)
			case ndriver.DriverSlack:
				slack.SetHTTPClient(ch, doer)
			case ndriver.DriverDiscord:
				discord.SetHTTPClient(ch, doer)
			}
		}
		return ch, nil
	}
}

type httpClientDoer struct {
	client *httpclient.Client
}

func (h *httpClientDoer) Do(req *http.Request) (*http.Response, error) {
	if h.client == nil {
		return http.DefaultClient.Do(req)
	}
	return h.client.Do(req.Context(), req)
}
