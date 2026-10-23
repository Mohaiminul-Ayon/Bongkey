package emailotp

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
)

// TestGmailSenderRejectsUnsafeHeaders checks input before any network connection.
func TestGmailSenderRejectsUnsafeHeaders(t *testing.T) {
	sender := GmailSender{From: "sender@gmail.com", AppPassword: "synthetic-app-password"}
	for _, address := range []string{"buyer@example.com\r\nBcc: victim@example.com", "Buyer <buyer@example.com>", ""} {
		if err := sender.Send(context.Background(), address, "123456"); err == nil || strings.Contains(err.Error(), sender.AppPassword) {
			t.Fatal("unsafe recipient accepted or credential leaked")
		}
	}
	if err := (GmailSender{}).Send(context.Background(), "buyer@example.com", "123456"); err == nil {
		t.Fatal("unconfigured sender accepted a code")
	}
}

// TestGmailFailureLogsOnlyStage keeps provider diagnostics free of message and credential data.
func TestGmailFailureLogsOnlyStage(t *testing.T) {
	var output bytes.Buffer
	originalWriter, originalFlags, originalPrefix := log.Writer(), log.Flags(), log.Prefix()
	log.SetOutput(&output)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
		log.SetPrefix(originalPrefix)
	})

	if err := gmailFailure("authentication"); err == nil || err.Error() != "email delivery failed" {
		t.Fatal("provider failure was not returned as a generic error")
	}
	if logged := strings.TrimSpace(output.String()); logged != "email OTP provider failure: stage=authentication" {
		t.Fatalf("unexpected provider diagnostic: %q", logged)
	}
}
