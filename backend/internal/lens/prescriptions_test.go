package lens

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"testing"
)

// pngOfSize builds a real PNG so the size check reads genuine image headers.
func pngOfSize(t *testing.T, side int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, side, side))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestPrescriptionTypeChecksContent accepts documents by their bytes and rejects everything else.
func TestPrescriptionTypeChecksContent(t *testing.T) {
	heic := append([]byte{0, 0, 0, 24}, []byte("ftypheic0000")...)
	for name, tc := range map[string]struct {
		data []byte
		want string
	}{
		"pdf":        {[]byte("%PDF-1.7 rest"), "pdf"},
		"png":        {pngOfSize(t, 800), "png"},
		"heic":       {heic, "heic"},
		"tiny png":   {pngOfSize(t, 100), ""},
		"html":       {[]byte("<html><script>alert(1)</script>"), ""},
		"renamed js": {[]byte("alert(1) // pretend.pdf"), ""},
		"empty":      {nil, ""},
		"too big":    {append([]byte("%PDF-"), make([]byte, MaxPrescriptionBytes)...), ""},
	} {
		got, err := PrescriptionType(tc.data)
		if tc.want == "" {
			if !errors.Is(err, ErrInvalidPrescription) {
				t.Fatalf("%s: got %q, want rejection", name, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("%s: got %q err %v, want %q", name, got, err, tc.want)
		}
	}
}

// TestPrescriptionStoreSaveAndPath writes privately and refuses names that could escape the folder.
func TestPrescriptionStoreSaveAndPath(t *testing.T) {
	store := PrescriptionStore{Dir: t.TempDir()}
	name, err := store.Save([]byte("%PDF-1.7 test"))
	if err != nil {
		t.Fatal(err)
	}
	path, err := store.Path(name)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "%PDF-1.7 test" {
		t.Fatalf("stored file = %q, err %v", data, err)
	}
	for _, bad := range []string{"../secret.pdf", "abc.pdf", "", name + "/x", "0123456789abcdef0123456789abcdef.exe"} {
		if _, err := store.Path(bad); !errors.Is(err, ErrInvalidPrescription) {
			t.Fatalf("path %q was accepted", bad)
		}
	}
	store.Remove(name)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("remove left the file behind")
	}
}
