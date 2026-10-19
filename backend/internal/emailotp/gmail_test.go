package emailotp

import (
	"context"
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
