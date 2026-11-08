// Package mail sends one email through the configured provider: Gmail SMTP locally, Brevo's HTTPS API on the server.
package mail

import (
	"context"
	"errors"
	netmail "net/mail"
	"strings"

	"bongkey/backend/internal/config"
)

// Message is one plain-text email with an optional attachment.
type Message struct {
	To         string
	Subject    string
	Text       string
	Attachment *Attachment
}

// Attachment is one file sent with a message, such as an invoice PDF.
type Attachment struct {
	Name        string
	ContentType string
	Content     []byte
}

// Sender delivers one message; both providers implement it so the shop can switch by configuration.
type Sender interface {
	Send(context.Context, Message) error
}

// maxAttachment keeps one email well under provider limits; an invoice PDF is a few kilobytes.
const maxAttachment = 2 << 20

// New returns the sender chosen by MAIL_PROVIDER; config validation has already checked its settings.
func New(cfg config.Config) Sender {
	if cfg.MailProvider == "brevo" {
		return Brevo{APIKey: cfg.BrevoAPIKey, From: cfg.MailFrom, FromName: cfg.MailFromName}
	}
	return Gmail{From: cfg.GmailFrom, AppPassword: cfg.GmailAppPassword}
}

// check refuses header injection and oversized content before any provider is contacted.
func (m Message) check() error {
	if !ValidAddress(m.To) || m.Subject == "" || strings.ContainsAny(m.Subject, "\r\n") || len(m.Subject) > 200 || m.Text == "" {
		return errors.New("invalid email message")
	}
	if a := m.Attachment; a != nil && (a.Name == "" || strings.ContainsAny(a.Name, "\r\n\"/\\") || a.ContentType == "" || len(a.Content) == 0 || len(a.Content) > maxAttachment) {
		return errors.New("invalid email attachment")
	}
	return nil
}

// ValidAddress accepts one plain mailbox such as buyer@example.com and refuses display names and header injection.
func ValidAddress(value string) bool {
	if strings.ContainsAny(value, "\r\n") || len(value) > 254 {
		return false
	}
	parsed, err := netmail.ParseAddress(value)
	return err == nil && parsed.Address == value
}
