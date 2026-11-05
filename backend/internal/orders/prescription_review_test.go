package orders

import (
	"context"
	"errors"
	"testing"
	"time"
)

// retentionStoreStub serves a fixed list of expired files and records which ones were forgotten.
type retentionStoreStub struct {
	expired   []StoredPrescription
	forgotten []StoredPrescription
	before    time.Time
}

func (s *retentionStoreStub) ExpiredPrescriptions(_ context.Context, before time.Time, _ int) ([]StoredPrescription, error) {
	s.before = before
	return s.expired, nil
}

func (s *retentionStoreStub) ForgetPrescription(_ context.Context, file StoredPrescription) error {
	s.forgotten = append(s.forgotten, file)
	return nil
}

// deleterStub fails for one chosen file name.
type deleterStub struct {
	failOn  string
	deleted []string
}

func (d *deleterStub) Delete(name string) error {
	if name == d.failOn {
		return errors.New("disk error")
	}
	d.deleted = append(d.deleted, name)
	return nil
}

// TestPurgeNeverForgetsAFileItCouldNotDelete keeps the database pointing at a file that is still on disk.
func TestPurgeNeverForgetsAFileItCouldNotDelete(t *testing.T) {
	now := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	store := &retentionStoreStub{expired: []StoredPrescription{{1, "a.pdf"}, {2, "b.pdf"}, {3, "c.pdf"}}}
	files := &deleterStub{failOn: "b.pdf"}
	removed, err := PrescriptionRetention{Store: store, Files: files}.Purge(context.Background(), now)
	if err == nil || removed != 1 || len(store.forgotten) != 1 || store.forgotten[0].Name != "a.pdf" {
		t.Fatalf("removed=%d forgotten=%v err=%v", removed, store.forgotten, err)
	}
	if !store.before.Equal(now.Add(-PrescriptionKeepFor)) {
		t.Fatalf("cutoff = %v", store.before)
	}
}

// reviewStoreStub answers ReplacePrescription with a fixed old name or error.
type reviewStoreStub struct {
	LensReviewStore
	oldName string
	err     error
}

func (s reviewStoreStub) ReplacePrescription(context.Context, int, int, string) (string, error) {
	return s.oldName, s.err
}

// TestAttachPrescriptionCleansUpFiles removes the old file on success and the new file when the change is refused.
func TestAttachPrescriptionCleansUpFiles(t *testing.T) {
	file := []byte("%PDF-1.7")
	saver := &prescriptionSaverStub{}
	err := LensReviewService{Store: reviewStoreStub{oldName: "old.pdf"}, Files: saver}.AttachPrescription(context.Background(), 1, 2, file)
	if err != nil || len(saver.removed) != 1 || saver.removed[0] != "old.pdf" {
		t.Fatalf("success: removed=%v err=%v", saver.removed, err)
	}
	saver = &prescriptionSaverStub{}
	err = LensReviewService{Store: reviewStoreStub{err: ErrLensReviewClosed}, Files: saver}.AttachPrescription(context.Background(), 1, 2, file)
	if !errors.Is(err, ErrLensReviewClosed) || len(saver.removed) != 1 || saver.removed[0] != saver.saved[0] {
		t.Fatalf("refused: saved=%v removed=%v err=%v", saver.saved, saver.removed, err)
	}
}
