package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"mime"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"time"
)

// Gmail sends messages through Gmail SMTP over verified TLS. It suits local work; many VPS hosts block
// outgoing mail ports, which is why the server uses Brevo's HTTPS API instead.
type Gmail struct {
	From        string
	AppPassword string
}

// Send uses a bounded SMTP connection and never includes credentials or addresses in errors.
func (s Gmail) Send(ctx context.Context, m Message) error {
	if s.From == "" || s.AppPassword == "" {
		return errors.New("email is not configured")
	}
	if !ValidAddress(s.From) {
		return errors.New("invalid sender address")
	}
	if err := m.check(); err != nil {
		return err
	}
	body, err := mimeMessage(s.From, m)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", "smtp.gmail.com:465")
	if err != nil {
		return failure("gmail", "connection")
	}
	defer conn.Close()
	// The caller owns cancellation; closing the socket stops SMTP I/O, and stop removes the callback on return.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	tlsConn := tls.Client(conn, &tls.Config{ServerName: "smtp.gmail.com", MinVersion: tls.VersionTLS12})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return failure("gmail", "TLS handshake")
	}
	client, err := smtp.NewClient(tlsConn, "smtp.gmail.com")
	if err != nil {
		return failure("gmail", "SMTP connection")
	}
	defer client.Close()
	if err := client.Auth(smtp.PlainAuth("", s.From, s.AppPassword, "smtp.gmail.com")); err != nil {
		return failure("gmail", "authentication")
	}
	if err := client.Mail(s.From); err != nil {
		return failure("gmail", "sender rejection")
	}
	if err := client.Rcpt(m.To); err != nil {
		return failure("gmail", "recipient rejection")
	}
	writer, err := client.Data()
	if err != nil {
		return failure("gmail", "message start")
	}
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return failure("gmail", "message write")
	}
	if err := writer.Close(); err != nil {
		return failure("gmail", "message finish")
	}
	return nil
}

// mimeMessage builds a UTF-8 plain-text email, as multipart/mixed when it carries an attachment.
func mimeMessage(from string, m Message) ([]byte, error) {
	var out bytes.Buffer
	fmt.Fprintf(&out, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\n", from, m.To, mime.QEncoding.Encode("utf-8", m.Subject))
	if m.Attachment == nil {
		fmt.Fprintf(&out, "Content-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n%s\r\n", wrap(base64.StdEncoding.EncodeToString([]byte(m.Text))))
		return out.Bytes(), nil
	}
	parts := multipart.NewWriter(&out)
	fmt.Fprintf(&out, "Content-Type: multipart/mixed; boundary=%s\r\n\r\n", parts.Boundary())
	text, err := parts.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=UTF-8"}, "Content-Transfer-Encoding": {"base64"}})
	if err != nil {
		return nil, err
	}
	if _, err := text.Write([]byte(wrap(base64.StdEncoding.EncodeToString([]byte(m.Text))))); err != nil {
		return nil, err
	}
	file, err := parts.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {m.Attachment.ContentType},
		"Content-Transfer-Encoding": {"base64"},
		"Content-Disposition":       {mime.FormatMediaType("attachment", map[string]string{"filename": m.Attachment.Name})},
	})
	if err != nil {
		return nil, err
	}
	if _, err := file.Write([]byte(wrap(base64.StdEncoding.EncodeToString(m.Attachment.Content)))); err != nil {
		return nil, err
	}
	if err := parts.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// wrap breaks base64 text into 76-character lines, the limit mail servers expect.
func wrap(encoded string) string {
	var out bytes.Buffer
	for len(encoded) > 76 {
		out.WriteString(encoded[:76] + "\r\n")
		encoded = encoded[76:]
	}
	out.WriteString(encoded)
	return out.String()
}

// failure records only the provider and failed stage, never an address, message text or credential.
func failure(provider, stage string) error {
	log.Printf("email provider failure: provider=%s stage=%s", provider, stage)
	return errors.New("email delivery failed")
}
