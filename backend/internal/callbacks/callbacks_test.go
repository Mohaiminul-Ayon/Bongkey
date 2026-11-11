package callbacks

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// storeStub records what reached the database layer.
type storeStub struct {
	created []Request
}

func (s *storeStub) Create(_ context.Context, r Request) (Request, error) {
	r.ID = int64(len(s.created) + 1)
	r.Status = StatusOpen
	s.created = append(s.created, r)
	return r, nil
}
func (s *storeStub) List(context.Context, string, int) ([]Request, error) { return nil, nil }
func (s *storeStub) MarkDone(context.Context, int64) error                { return nil }

// counterStub counts attempts per key like the limiter table does within one window.
type counterStub map[string]int

func (c counterStub) Count(_ context.Context, key string) (int, error) {
	c[key]++
	return c[key], nil
}

// notifierStub records the alerts it was asked to queue.
type notifierStub struct{ sent []Request }

func (n *notifierStub) CallbackRequested(_ context.Context, r Request) { n.sent = append(n.sent, r) }

// TestCreateNormalizesAndAlerts saves what customers really type in local form and alerts the owner once.
func TestCreateNormalizesAndAlerts(t *testing.T) {
	store, alerts := &storeStub{}, &notifierStub{}
	service := Service{Store: store, Limiter: counterStub{}, Notifier: alerts, Secret: "test-secret"}
	got, err := service.Create(context.Background(), Request{Name: "  Rahim   Uddin ", Phone: "+880 1712-345678", ProductSlug: "bongkey-eid-edit-gold", Message: "  -8.50 both eyes, thin lens  "}, "203.0.113.5")
	if err != nil || got.ID != 1 || got.Name != "Rahim Uddin" || got.Phone != "01712345678" || got.Message != "-8.50 both eyes, thin lens" {
		t.Fatalf("saved = %+v %v", got, err)
	}
	if len(alerts.sent) != 1 || alerts.sent[0].ID != 1 {
		t.Fatalf("owner alerts = %+v", alerts.sent)
	}
}

// TestCreateRefusesBadFields keeps malformed requests away from the database.
func TestCreateRefusesBadFields(t *testing.T) {
	store := &storeStub{}
	service := Service{Store: store, Limiter: counterStub{}, Secret: "test-secret"}
	good := Request{Name: "Rahim", Phone: "01712345678", Message: "progressive lens please"}
	for name, change := range map[string]func(*Request){
		"short name":     func(r *Request) { r.Name = "R" },
		"long name":      func(r *Request) { r.Name = strings.Repeat("a", 81) },
		"landline":       func(r *Request) { r.Phone = "0212345678" },
		"short message":  func(r *Request) { r.Message = "hi" },
		"long message":   func(r *Request) { r.Message = strings.Repeat("x", 501) },
		"slug with path": func(r *Request) { r.ProductSlug = "../admin" },
	} {
		r := good
		change(&r)
		// Each case uses its own client so the client limit never hides a validation error.
		if _, err := service.Create(context.Background(), r, "198.51.100."+name); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	if len(store.created) != 0 {
		t.Fatalf("invalid requests reached the store: %d", len(store.created))
	}
	if _, err := service.Create(context.Background(), good, ""); err == nil {
		t.Fatal("request without a client address was accepted")
	}
}

// TestCreateLimitsPhoneAndClient stops repeated requests from one phone, and many phones from one client.
func TestCreateLimitsPhoneAndClient(t *testing.T) {
	store := &storeStub{}
	service := Service{Store: store, Limiter: counterStub{}, Secret: "test-secret"}
	r := Request{Name: "Rahim", Phone: "01712345678", Message: "progressive lens please"}
	for i := range requestsPerPhone {
		if _, err := service.Create(context.Background(), r, "203.0.113."+string(rune('a'+i))); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}
	if _, err := service.Create(context.Background(), r, "203.0.113.z"); !errors.Is(err, ErrLimited) {
		t.Fatalf("phone over its limit: %v", err)
	}
	for i := 0; i <= requestsPerClient; i++ {
		other := r
		other.Phone = "0181234567" + string(rune('0'+i))
		_, err := service.Create(context.Background(), other, "192.0.2.1")
		if i < requestsPerClient && err != nil {
			t.Fatalf("client request %d: %v", i+1, err)
		}
		if i == requestsPerClient && !errors.Is(err, ErrLimited) {
			t.Fatalf("client over its limit: %v", err)
		}
	}
}

// TestListRefusesUnknownStatus accepts only open, done or every status.
func TestListRefusesUnknownStatus(t *testing.T) {
	service := Service{Store: &storeStub{}}
	if _, err := service.List(context.Background(), "deleted"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown status: %v", err)
	}
	if _, err := service.List(context.Background(), ""); err != nil {
		t.Fatalf("every status: %v", err)
	}
	if err := service.MarkDone(context.Background(), 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("id 0: %v", err)
	}
}
