package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExplorerArgs(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "中文 note.md")
	if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("directory opens itself", func(t *testing.T) {
		args, err := explorerArgs(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(args) != 1 || args[0] != root {
			t.Fatalf("args = %q, want [%q]", args, root)
		}
	})

	t.Run("file is selected", func(t *testing.T) {
		args, err := explorerArgs(file)
		if err != nil {
			t.Fatal(err)
		}
		want := "/select," + file
		if len(args) != 1 || args[0] != want {
			t.Fatalf("args = %q, want [%q]", args, want)
		}
	})

	t.Run("missing path returns an error", func(t *testing.T) {
		if _, err := explorerArgs(filepath.Join(root, "missing")); err == nil {
			t.Fatal("expected an error for a missing path")
		}
	})
}

func TestAbsolutePathRejectsEmptyValue(t *testing.T) {
	if _, err := absolutePath("   "); err == nil {
		t.Fatal("expected an error for an empty path")
	}
}
