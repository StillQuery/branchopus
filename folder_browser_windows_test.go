package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestBrowseFolderListsOnlyFoldersAndCapsResult(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < folderPageLimit+5; i++ {
		if err := os.Mkdir(filepath.Join(root, fmt.Sprintf("folder-%04d", i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "not-a-folder.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	listing, err := NewApp().BrowseFolder(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Folders) != folderPageLimit {
		t.Fatalf("folders=%d, want %d", len(listing.Folders), folderPageLimit)
	}
	if !listing.Truncated {
		t.Fatal("expected truncated listing")
	}
	if listing.Folders[0].Name != "folder-0000" {
		t.Fatalf("folders are not sorted: first=%q", listing.Folders[0].Name)
	}
}
