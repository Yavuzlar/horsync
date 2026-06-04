package transfer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWipeEXIF(t *testing.T) {
	// Create a temporary file with mock JPEG EXIF data
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_photo.jpg")

	exifData := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xE1, 0x00, 0x0A, 'E', 'x', 'i', 'f', 0x00, 0x00, 0x12, 0x34, // APP1 Exif segment
		0xFF, 0xDB, 0x00, 0x04, 0x05, 0x06, // DQT segment
		0xFF, 0xD9, // EOI
	}

	if err := os.WriteFile(filePath, exifData, 0644); err != nil {
		t.Fatalf("failed to write mock JPEG: %v", err)
	}

	mgr := &Manager{}
	if err := mgr.WipeEXIF(filePath); err != nil {
		t.Fatalf("WipeEXIF failed: %v", err)
	}

	cleanedData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read cleaned JPEG: %v", err)
	}

	if bytes.Contains(cleanedData, []byte("Exif")) {
		t.Error("EXIF segment was not stripped from JPEG")
	}

	// Make sure the SOI (0xFFD8) and DQT (0xFFDB) are preserved
	if !bytes.Contains(cleanedData, []byte{0xFF, 0xD8}) {
		t.Error("JPEG SOI header was corrupted or lost")
	}
	if !bytes.Contains(cleanedData, []byte{0xFF, 0xDB}) {
		t.Error("JPEG DQT segment was corrupted or lost")
	}
}

func TestWipeDocMetadata(t *testing.T) {
	// Create a temporary file with mock PDF containing metadata fields and XML XMP streams
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_doc.pdf")

	pdfContent := []byte("%PDF-1.4\n1 0 obj\n<< /Author (John Doe) /Creator (WordProcessor) >>\nendobj\n<x:xmpmeta>XML Metadata Stream</x:xmpmeta>\n%%EOF")

	if err := os.WriteFile(filePath, pdfContent, 0644); err != nil {
		t.Fatalf("failed to write mock PDF: %v", err)
	}

	mgr := &Manager{}
	if err := mgr.WipeDocMetadata(filePath); err != nil {
		t.Fatalf("WipeDocMetadata failed: %v", err)
	}

	cleanedData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read cleaned PDF: %v", err)
	}

	if bytes.Contains(cleanedData, []byte("John Doe")) {
		t.Error("Author metadata was not wiped from PDF")
	}
	if bytes.Contains(cleanedData, []byte("WordProcessor")) {
		t.Error("Creator metadata was not wiped from PDF")
	}
	if bytes.Contains(cleanedData, []byte("XML Metadata Stream")) {
		t.Error("XMP XML metadata stream was not wiped from PDF")
	}

	// Confirm that file size is maintained and PDF format header remains
	if len(cleanedData) != len(pdfContent) {
		t.Errorf("PDF length changed from %d to %d, violating non-corruption bounds", len(pdfContent), len(cleanedData))
	}
	if !bytes.HasPrefix(cleanedData, []byte("%PDF")) {
		t.Error("PDF prefix was corrupted")
	}
}
