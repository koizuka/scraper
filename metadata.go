package scraper

import (
	"encoding/json"
	"fmt"
	"os"
)

const MetadataFileExtension = ".meta"

// PageMetadata holds metadata for saved pages
type PageMetadata struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Title       string `json:"title,omitempty"`
}

// savePageMetadata saves metadata to a .meta file
func savePageMetadata(filename string, metadata PageMetadata) error {
	metadataFilename := filename + MetadataFileExtension
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %v", err)
	}
	err = os.WriteFile(metadataFilename, metadataBytes, os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("failed to write metadata file %s: %v", metadataFilename, err)
	}
	return nil
}

// loadPageMetadata loads metadata from a .meta file
func loadPageMetadata(filename string) (PageMetadata, error) {
	var metadata PageMetadata
	metadataFilename := filename + MetadataFileExtension
	metadataBytes, err := os.ReadFile(metadataFilename)
	if err != nil {
		return metadata, fmt.Errorf("failed to read metadata file %s: %v", metadataFilename, err)
	}

	err = json.Unmarshal(metadataBytes, &metadata)
	if err != nil {
		return metadata, fmt.Errorf("failed to parse metadata file %s: %v", metadataFilename, err)
	}
	return metadata, nil
}
