package emailotp

import (
	"context"
	"errors"

	"bongkey/backend/internal/mail"
)

// CodeMailer turns a verification code into an email for the configured provider.
type CodeMailer struct {
	Mail mail.Sender
}

// Send emails one 6-digit code; codes go out directly, not through the notification queue, so they are never delayed behind it.
func (m CodeMailer) Send(ctx context.Context, to, code string) error {
	if len(code) != 6 {
		return errors.New("invalid email message")
	}
	return m.Mail.Send(ctx, mail.Message{
		To:      to,
		Subject: "Bongkey email verification code",
		Text:    "Your Bongkey verification code is " + code + ". It expires in 5 minutes. If you did not request it, ignore this email.",
	})
}
