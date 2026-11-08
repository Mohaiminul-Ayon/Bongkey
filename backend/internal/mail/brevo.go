package mail

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// brevoEndpoint is Brevo's transactional email API.
const brevoEndpoint = "https://api.brevo.com/v3/smtp/email"

// Brevo sends messages through Brevo's HTTPS API, which works on hosts that block outgoing mail ports.
// From must be an address on a domain verified in the Brevo account.
type Brevo struct {
	APIKey   string
	From     string
	FromName string
	// Endpoint and Client are replaced only in tests.
	Endpoint string
	Client   *http.Client
}

// brevoContact is one sender or recipient in the API request.
type brevoContact struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// brevoAttachment is one base64 file in the API request.
type brevoAttachment struct {
	Content string `json:"content"`
	Name    string `json:"name"`
}

// brevoRequest is the JSON body Brevo expects for one transactional email.
type brevoRequest struct {
	Sender      brevoContact      `json:"sender"`
	To          []brevoContact    `json:"to"`
	Subject     string            `json:"subject"`
	TextContent string            `json:"textContent"`
	Attachment  []brevoAttachment `json:"attachment,omitempty"`
}

// Send posts one message with a bounded timeout; it logs only the HTTP status, never the response body,
// which can echo addresses.
func (s Brevo) Send(ctx context.Context, m Message) error {
	if s.APIKey == "" || !ValidAddress(s.From) {
		return errors.New("email is not configured")
	}
	if err := m.check(); err != nil {
		return err
	}
	request := brevoRequest{Sender: brevoContact{Email: s.From, Name: s.FromName}, To: []brevoContact{{Email: m.To}}, Subject: m.Subject, TextContent: m.Text}
	if m.Attachment != nil {
		request.Attachment = []brevoAttachment{{Content: base64.StdEncoding.EncodeToString(m.Attachment.Content), Name: m.Attachment.Name}}
	}
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	endpoint, client := s.Endpoint, s.Client
	if endpoint == "" {
		endpoint = brevoEndpoint
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpRequest.Header.Set("api-key", s.APIKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	response, err := client.Do(httpRequest)
	if err != nil {
		return failure("brevo", "connection")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return failure("brevo", fmt.Sprintf("status %d", response.StatusCode))
	}
	return nil
}
