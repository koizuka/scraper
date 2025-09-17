package scraper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSaveAndLoadPageMetadata(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "metadata_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	metadata := PageMetadata{
		URL:         "https://example.com/test",
		ContentType: "text/html",
		Title:       "Test Page",
	}

	filename := filepath.Join(tempDir, "test.html")

	// Test save
	err = savePageMetadata(filename, metadata)
	if err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Test load
	loaded, err := loadPageMetadata(filename)
	if err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	// Verify data integrity
	if diff := cmp.Diff(metadata, loaded); diff != "" {
		t.Errorf("Metadata mismatch (-expected +got):\n%s", diff)
	}
}

func TestLoadPageMetadata_Errors(t *testing.T) {
	// Test non-existent file
	_, err := loadPageMetadata("/non/existent/file.html")
	if err == nil {
		t.Error("Expected error when loading non-existent file")
	}

	// Test invalid JSON
	tempDir, err := os.MkdirTemp("", "metadata_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filename := filepath.Join(tempDir, "invalid.html")
	metaFile := filename + ".meta"
	err = os.WriteFile(metaFile, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid JSON file: %v", err)
	}

	_, err = loadPageMetadata(filename)
	if err == nil {
		t.Error("Expected error when loading invalid JSON")
	}
}
