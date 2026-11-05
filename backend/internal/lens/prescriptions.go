package lens

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg" // registers JPEG for DecodeConfig
	_ "image/png"  // registers PNG for DecodeConfig
	"os"
	"path/filepath"
	"regexp"

	"bongkey/backend/internal/security"
)

// MaxPrescriptionBytes bounds one uploaded prescription file.
const MaxPrescriptionBytes = 5 << 20

// minPrescriptionSide rejects thumbnails and screenshots too small to read a prescription from.
const minPrescriptionSide = 400

var (
	// ErrInvalidPrescription means the file is not an accepted, readable prescription document.
	ErrInvalidPrescription = errors.New("invalid prescription file")
	// prescriptionName matches only names this store generated, so a stored value cannot point outside the directory.
	prescriptionName = regexp.MustCompile(`^[0-9a-f]{32}\.(pdf|jpg|png|heic)$`)
)

// PrescriptionStore keeps prescription files in a private directory that no public route serves.
// Files are health data: only the admin download route reads them, and names are random, not order-based.
type PrescriptionStore struct{ Dir string }

// PrescriptionType names an accepted file by its content, never by the uploaded file name or browser type.
func PrescriptionType(data []byte) (string, error) {
	if len(data) == 0 || len(data) > MaxPrescriptionBytes {
		return "", ErrInvalidPrescription
	}
	switch {
	case bytes.HasPrefix(data, []byte("%PDF-")):
		return "pdf", nil
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}), bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		config, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width < minPrescriptionSide && config.Height < minPrescriptionSide {
			return "", ErrInvalidPrescription
		}
		if format == "jpeg" {
			return "jpg", nil
		}
		return "png", nil
	case len(data) >= 12 && string(data[4:8]) == "ftyp":
		switch string(data[8:12]) {
		case "heic", "heix", "heim", "heis", "mif1", "msf1":
			return "heic", nil
		}
	}
	return "", ErrInvalidPrescription
}

// Save writes one checked file under a random name with owner-only permissions and returns that name.
func (s PrescriptionStore) Save(data []byte) (string, error) {
	ext, err := PrescriptionType(data)
	if err != nil {
		return "", err
	}
	random, err := security.RandomHex(16)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return "", err
	}
	name := random + "." + ext
	file, err := os.OpenFile(filepath.Join(s.Dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		os.Remove(filepath.Join(s.Dir, name))
		return "", err
	}
	if err := file.Close(); err != nil {
		os.Remove(filepath.Join(s.Dir, name))
		return "", err
	}
	return name, nil
}

// Path resolves a stored name to its file, refusing anything this store could not have generated.
func (s PrescriptionStore) Path(name string) (string, error) {
	if !prescriptionName.MatchString(name) {
		return "", ErrInvalidPrescription
	}
	return filepath.Join(s.Dir, name), nil
}

// Remove deletes a file whose order change did not commit, or that was replaced; a failure leaves a private orphan only.
func (s PrescriptionStore) Remove(name string) {
	_ = s.Delete(name)
}

// Delete removes a stored file and reports any failure except the file already being gone.
func (s PrescriptionStore) Delete(name string) error {
	path, err := s.Path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
