package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/windows"
)

const folderPageLimit = 500

type FolderEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type FolderListing struct {
	Path      string        `json:"path"`
	Parent    string        `json:"parent"`
	Folders   []FolderEntry `json:"folders"`
	Truncated bool          `json:"truncated"`
}

// BrowseFolder powers the lightweight in-app picker. Unlike the Windows
// Common File Dialog it never initializes Quick Access, previews, or shell
// extensions; it only reads one directory level.
func (a *App) BrowseFolder(path string) (FolderListing, error) {
	if strings.TrimSpace(path) == "" {
		mask, err := windows.GetLogicalDrives()
		if err != nil {
			return FolderListing{}, err
		}
		result := FolderListing{}
		for i := 0; i < 26; i++ {
			if mask&(1<<i) == 0 {
				continue
			}
			root := string(rune('A'+i)) + `:\`
			result.Folders = append(result.Folders, FolderEntry{Name: root, Path: root})
		}
		return result, nil
	}

	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return FolderListing{}, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return FolderListing{}, err
	}
	result := FolderListing{Path: filepath.Clean(abs)}
	parent := filepath.Dir(result.Path)
	if parent != result.Path {
		result.Parent = parent
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if len(result.Folders) >= folderPageLimit {
			result.Truncated = true
			break
		}
		result.Folders = append(result.Folders, FolderEntry{
			Name: entry.Name(), Path: filepath.Join(result.Path, entry.Name()),
		})
	}
	sort.Slice(result.Folders, func(i, j int) bool {
		return strings.ToLower(result.Folders[i].Name) < strings.ToLower(result.Folders[j].Name)
	})
	return result, nil
}
