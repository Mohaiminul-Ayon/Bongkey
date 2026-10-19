package emailotp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// GmailSender sends plain-text OTP messages through Gmail over verified TLS.
type GmailSender struct {
	From        string
	AppPassword string
}

// Send uses a bounded SMTP connection and never includes credentials in errors.
func (s GmailSender) Send(ctx context.Context, to, code string) error {
	if s.From == "" || s.AppPassword == "" {
		return errors.New("email is not configured")
	}
	if !validAddress(s.From) || !validAddress(to) || len(code) != 6 {
		return errors.New("invalid email message")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", "smtp.gmail.com:465")
	if err != nil {
		return gmailFailure("connection")
	}
	defer conn.Close()
	// The request owns cancellation; closing the socket stops SMTP I/O, and stop closes the callback on return.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	tlsConn := tls.Client(conn, &tls.Config{ServerName: "smtp.gmail.com", MinVersion: tls.VersionTLS12})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return gmailFailure("TLS handshake")
	}
	client, err := smtp.NewClient(tlsConn, "smtp.gmail.com")
	if err != nil {
		return gmailFailure("SMTP connection")
	}
	defer client.Close()
	if err := client.Auth(smtp.PlainAuth("", s.From, s.AppPassword, "smtp.gmail.com")); err != nil {
		return gmailFailure("authentication")
	}
	if err := client.Mail(s.From); err != nil {
		return gmailFailure("sender rejection")
	}
	if err := client.Rcpt(to); err != nil {
		return gmailFailure("recipient rejection")
	}
	writer, err := client.Data()
	if err != nil {
		return gmailFailure("message start")
	}
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Bongkey email verification code\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nYour Bongkey verification code is %s. It expires in 5 minutes. If you did not request it, ignore this email.\r\n", s.From, to, code)
	if _, err := writer.Write([]byte(message)); err != nil {
		_ = writer.Close()
		return gmailFailure("message write")
	}
	if err := writer.Close(); err != nil {
		return gmailFailure("message finish")
	}
	return nil
}

// gmailFailure records only the failed SMTP stage, never an address, code, or credential.
func gmailFailure(stage string) error {
	log.Printf("email OTP provider failure: stage=%s", stage)
	return errors.New("email delivery failed")
}

// validAddress accepts one plain mailbox and prevents SMTP header injection.
func validAddress(value string) bool {
	if strings.ContainsAny(value, "\r\n") || len(value) > 254 {
		return false
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}
