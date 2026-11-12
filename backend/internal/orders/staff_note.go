package orders

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
)

// maxStaffNote matches the orders_staff_note_length database check.
const maxStaffNote = 1000

// ErrInvalidStaffNote means the note is longer than maxStaffNote characters.
var ErrInvalidStaffNote = errors.New("invalid staff note")

// StaffNoteStore saves the team's private note on one order.
type StaffNoteStore interface {
	SaveStaffNote(ctx context.Context, orderID int, note string) error
}

// StaffNoteService checks a staff note before it replaces the order's current note.
type StaffNoteService struct{ Store StaffNoteStore }

// SetStaffNote trims and saves the note; an empty note clears it. Any order status may carry a note.
func (s StaffNoteService) SetStaffNote(ctx context.Context, orderID int, note string) error {
	note = strings.TrimSpace(note)
	if orderID <= 0 {
		return ErrNotFound
	}
	if utf8.RuneCountInString(note) > maxStaffNote {
		return ErrInvalidStaffNote
	}
	return s.Store.SaveStaffNote(ctx, orderID, note)
}

// SaveStaffNote replaces one order's note; an unknown order is ErrNotFound.
func (s PostgresFulfillmentStore) SaveStaffNote(ctx context.Context, orderID int, note string) error {
	var id int
	err := s.DB.QueryRowContext(ctx, `UPDATE orders SET staff_note = $2, updated_at = NOW() WHERE id = $1 RETURNING id`, orderID, note).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
