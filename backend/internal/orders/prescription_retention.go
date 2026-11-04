package orders

import (
	"context"
	"log"
	"time"
)

// PrescriptionKeepFor is how long a prescription is kept after its order was delivered or cancelled:
// long enough for a remake or a frame exchange, short enough not to hoard medical data.
const PrescriptionKeepFor = 180 * 24 * time.Hour

// prescriptionPurgeBatch bounds one cleanup pass; the next daily pass continues where this one stopped.
const prescriptionPurgeBatch = 200

// StoredPrescription is one lens line's private file.
type StoredPrescription struct {
	ItemID int
	Name   string
}

// PrescriptionRetentionStore finds expired files and forgets them once deleted.
type PrescriptionRetentionStore interface {
	ExpiredPrescriptions(ctx context.Context, before time.Time, limit int) ([]StoredPrescription, error)
	ForgetPrescription(ctx context.Context, file StoredPrescription) error
}

// PrescriptionDeleter removes a stored file and reports failure, so a file is never forgotten while still on disk.
type PrescriptionDeleter interface {
	Delete(name string) error
}

// PrescriptionRetention deletes prescription files of finished orders after PrescriptionKeepFor.
type PrescriptionRetention struct {
	Store PrescriptionRetentionStore
	Files PrescriptionDeleter
}

// Purge deletes one batch of expired files and returns how many were removed.
// The file goes first: if the database update then fails, the next pass retries and finds the file already gone.
func (r PrescriptionRetention) Purge(ctx context.Context, now time.Time) (int, error) {
	files, err := r.Store.ExpiredPrescriptions(ctx, now.Add(-PrescriptionKeepFor), prescriptionPurgeBatch)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		if err := r.Files.Delete(file.Name); err != nil {
			return removed, err
		}
		if err := r.Store.ForgetPrescription(ctx, file); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// RunDaily purges at start and then once a day until ctx is cancelled. The caller owns the goroutine
// that runs it and waits for it to return before closing the database. Failures are logged and retried next day.
func (r PrescriptionRetention) RunDaily(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		removed, err := r.Purge(ctx, time.Now())
		if err != nil && ctx.Err() == nil {
			log.Printf("prescription cleanup stopped after %d files: %v", removed, err)
		} else if removed > 0 {
			log.Printf("prescription cleanup deleted %d expired files", removed)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
