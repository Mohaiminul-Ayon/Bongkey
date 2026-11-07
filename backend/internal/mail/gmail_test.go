package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureLog collects log output for one test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
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
	return &output
}

// TestSendersRejectUnsafeMessages checks input before any network connection.
func TestSendersRejectUnsafeMessages(t *testing.T) {
	senders := []Sender{
		Gmail{From: "sender@gmail.com", AppPassword: "synthetic-app-password"},
		Brevo{APIKey: "synthetic-key", From: "orders@example.com", Endpoint: "http://127.0.0.1:1"},
	}
	bad := []Message{
		{To: "buyer@example.com\r\nBcc: victim@example.com", Subject: "Hi", Text: "x"},
		{To: "Buyer <buyer@example.com>", Subject: "Hi", Text: "x"},
		{To: "buyer@example.com", Subject: "Hi\r\nBcc: victim@example.com", Text: "x"},
		{To: "buyer@example.com", Subject: "Hi", Text: ""},
		{To: "buyer@example.com", Subject: "Hi", Text: "x", Attachment: &Attachment{Name: "a\r\n.pdf", ContentType: "application/pdf", Content: []byte("x")}},
	}
	for _, sender := range senders {
		for _, m := range bad {
			if err := sender.Send(context.Background(), m); err == nil || strings.Contains(err.Error(), "synthetic") {
				t.Fatalf("%T accepted an unsafe message or leaked a credential", sender)
			}
		}
	}
	if err := (Gmail{}).Send(context.Background(), Message{To: "buyer@example.com", Subject: "Hi", Text: "x"}); err == nil {
		t.Fatal("unconfigured Gmail sender accepted a message")
	}
}

// TestFailureLogsOnlyStage keeps provider diagnostics free of message and credential data.
func TestFailureLogsOnlyStage(t *testing.T) {
	output := captureLog(t)
	if err := failure("gmail", "authentication"); err == nil || err.Error() != "email delivery failed" {
		t.Fatal("provider failure was not returned as a generic error")
	}
	if logged := strings.TrimSpace(output.String()); logged != "email provider failure: provider=gmail stage=authentication" {
		t.Fatalf("unexpected provider diagnostic: %q", logged)
	}
}

// TestMimeMessageCarriesAttachment builds a multipart message with the text and the file.
func TestMimeMessageCarriesAttachment(t *testing.T) {
	body, err := mimeMessage("sender@gmail.com", Message{To: "buyer@example.com", Subject: "Your invoice ৳", Text: "Thanks", Attachment: &Attachment{Name: "INV-000001.pdf", ContentType: "application/pdf", Content: []byte("%PDF-1.7")}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"multipart/mixed", "filename=INV-000001.pdf", "Subject: =?utf-8?q?", "VGhhbmtz", "JVBERi0xLjc="} {
		if !strings.Contains(text, want) {
			t.Fatalf("message lacks %q", want)
		}
	}
}

// TestBrevoSendsJSONAndHidesResponse posts the expected request and keeps a failing response body out of logs.
func TestBrevoSendsJSONAndHidesResponse(t *testing.T) {
	var got brevoRequest
	status := http.StatusCreated
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("api-key") != "synthetic-key" || r.Method != http.MethodPost {
			t.Error("missing API key or wrong method")
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"message":"buyer@example.com is blocked"}`))
	}))
	defer server.Close()
	sender := Brevo{APIKey: "synthetic-key", From: "orders@example.com", FromName: "Bongkey", Endpoint: server.URL, Client: server.Client()}
	m := Message{To: "buyer@example.com", Subject: "Order", Text: "Thanks", Attachment: &Attachment{Name: "INV.pdf", ContentType: "application/pdf", Content: []byte("pdf")}}
	if err := sender.Send(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if got.Sender.Email != "orders@example.com" || len(got.To) != 1 || got.To[0].Email != "buyer@example.com" || got.TextContent != "Thanks" || len(got.Attachment) != 1 || got.Attachment[0].Content != "cGRm" {
		t.Fatalf("unexpected Brevo request: %+v", got)
	}
	output := captureLog(t)
	status = http.StatusUnauthorized
	if err := sender.Send(context.Background(), m); err == nil {
		t.Fatal("a 401 was treated as sent")
	}
	if strings.Contains(output.String(), "buyer@example.com") || !strings.Contains(output.String(), "status 401") {
		t.Fatalf("unexpected Brevo diagnostic: %q", output.String())
	}
}
