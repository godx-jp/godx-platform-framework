// Light notification channels register on import. The mail and
// database channels are constructed by notifications.Module when
// mail.Module / DatabaseStore are available.
package notifications

import (
	_ "github.com/godx-jp/godx-platform-framework/notifications/drivers/discord"
	_ "github.com/godx-jp/godx-platform-framework/notifications/drivers/log"
	_ "github.com/godx-jp/godx-platform-framework/notifications/drivers/slack"
	_ "github.com/godx-jp/godx-platform-framework/notifications/drivers/webhook"
)
