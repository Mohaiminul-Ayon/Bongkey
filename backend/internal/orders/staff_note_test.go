package orders

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// noteRecorder keeps the last note saved for each order.
type noteRecorder map[int]string

// SaveStaffNote records the note like the PostgreSQL store.
func (r noteRecorder) SaveStaffNote(_ context.Context, orderID int, note string) error {
	r[orderID] = note
	return nil
}

// TestSetStaffNote trims notes, allows clearing, counts characters (not bytes) and rejects bad input.
func TestSetStaffNote(t *testing.T) {
	store := noteRecorder{}
	service := StaffNoteService{Store: store}
	ctx := context.Background()
	if err := service.SetStaffNote(ctx, 7, "  called twice, no answer \n"); err != nil || store[7] != "called twice, no answer" {
		t.Fatalf("trimmed note: %q %v", store[7], err)
	}
	if err := service.SetStaffNote(ctx, 7, "   "); err != nil || store[7] != "" {
		t.Fatalf("clear note: %q %v", store[7], err)
	}
	// 1000 Bangla letters are 3000 bytes but still within the limit.
	if err := service.SetStaffNote(ctx, 7, strings.Repeat("ক", maxStaffNote)); err != nil {
		t.Fatalf("1000 characters refused: %v", err)
	}
	if err := service.SetStaffNote(ctx, 7, strings.Repeat("a", maxStaffNote+1)); !errors.Is(err, ErrInvalidStaffNote) {
		t.Fatalf("too long note: %v", err)
	}
	if err := service.SetStaffNote(ctx, 0, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bad id: %v", err)
	}
}
