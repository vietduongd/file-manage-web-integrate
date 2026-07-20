package handlers

import (
	"testing"

	"github.com/ckfindercompatible/backend/internal/config"
)

func TestValidateZipExtractionEntryRejectsTraversal(t *testing.T) {
	rt := imageResourceType()

	if name, _, err := validateZipExtractionEntry("../secret.jpg", 1, 0, 0, rt); err == nil {
		t.Fatalf("validateZipExtractionEntry returned %q, want error", name)
	}
}

func TestValidateZipExtractionEntryRejectsTooManyEntries(t *testing.T) {
	rt := imageResourceType()

	if name, _, err := validateZipExtractionEntry("photo.jpg", 1, maxExtractedZipEntries, 0, rt); err == nil {
		t.Fatalf("validateZipExtractionEntry returned %q, want error", name)
	}
}

func TestValidateZipExtractionEntryRejectsTooMuchUncompressedData(t *testing.T) {
	rt := imageResourceType()

	if name, _, err := validateZipExtractionEntry("photo.jpg", 2, 0, maxExtractedZipBytes-1, rt); err == nil {
		t.Fatalf("validateZipExtractionEntry returned %q, want error", name)
	}
}

func TestValidateZipExtractionEntryAllowsNestedAllowedFile(t *testing.T) {
	rt := imageResourceType()

	name, total, err := validateZipExtractionEntry("nested/photo.jpg", 10, 0, 5, rt)
	if err != nil {
		t.Fatalf("validateZipExtractionEntry returned error: %v", err)
	}
	if name != "nested/photo.jpg" {
		t.Fatalf("normalized name = %q, want %q", name, "nested/photo.jpg")
	}
	if total != 15 {
		t.Fatalf("total bytes = %d, want %d", total, 15)
	}
}

func imageResourceType() *config.ResourceTypeConfig {
	return &config.ResourceTypeConfig{
		Name:              "Images",
		Prefix:            "images",
		AllowedExtensions: []string{"jpg", "png"},
		MaxSizeMB:         50,
		PublicRead:        true,
	}
}
