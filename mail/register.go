// Light mail drivers register themselves on import. Heavy drivers
// (ses, sendgrid, mailgun, postmark) require an explicit blank import
// so their SDKs / HTTP clients stay out of binaries that only log mail.
package mail

import (
	_ "github.com/godx-jp/godx-platform-framework/mail/drivers/log"
	_ "github.com/godx-jp/godx-platform-framework/mail/drivers/smtp"
)
