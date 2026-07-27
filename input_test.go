package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadURLs(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "urls.txt")

	content := "https://example.com\nnot-a-url\nhttps://another.com\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	urls, warnings, err := readURLs(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantURLs := []string{"https://example.com", "https://another.com"}
	if !reflect.DeepEqual(urls, wantURLs) {
		t.Errorf("got urls %v, want %v", urls, wantURLs)
	}

	if len(warnings) != 1 || !strings.Contains(warnings[0], "not-a-url") {
		t.Errorf("got warnings %v, want a warning mentioning %q", warnings, "not-a-url")
	}
}
