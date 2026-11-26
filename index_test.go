package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkQueryHundredThousandNodes(b *testing.B) {
	idx := &TreeIndex{ByExt: map[string][]int{}}
	idx.Nodes = append(idx.Nodes, indexedNode{ID: 0, ParentID: -1, Name: "root", SearchKey: "root", Kind: Directory})
	for i := 1; i <= 100_000; i++ {
		ext := ".md"
		if i%3 == 0 {
			ext = ".pdf"
		}
		n := indexedNode{ID: i, ParentID: 0, Name: fmt.Sprintf("note-%d%s", i, ext), SearchKey: fmt.Sprintf("knowledge project %d", i), Extension: ext, Kind: File, Depth: 1}
		idx.Nodes = append(idx.Nodes, n)
		idx.ByExt[ext] = append(idx.ByExt[ext], i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Query(QueryRequest{Text: "knowledge 99", Extensions: []string{".md"}, Limit: 1600})
	}
}

func TestQueryKeepsOnlyMatchingBranches(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "工作", "项目A", "安全报告.md"))
	writeTestFile(t, filepath.Join(root, "工作", "项目B", "原型.png"))
	writeTestFile(t, filepath.Join(root, "学习", "安全笔记.txt"))
	idx, _, err := BuildIndex(root)
	if err != nil {
		t.Fatal(err)
	}

	result := idx.Query(QueryRequest{Text: "安全 报", Extensions: []string{".md"}})
	if result.Matches != 1 {
		t.Fatalf("matches=%d, want 1", result.Matches)
	}
	if len(result.Nodes) != 4 {
		t.Fatalf("nodes=%d, want root + 2 dirs + file", len(result.Nodes))
	}
	if got := result.Nodes[len(result.Nodes)-1].Name; got != "安全报告.md" {
		t.Fatalf("last node=%q", got)
	}
}

func TestExtensionFilterAddsAncestors(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a", "b", "one.md"))
	writeTestFile(t, filepath.Join(root, "a", "other.pdf"))
	idx, _, err := BuildIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	result := idx.Query(QueryRequest{Extensions: []string{".md"}})
	if result.Matches != 1 || len(result.Nodes) != 4 {
		t.Fatalf("matches=%d nodes=%d", result.Matches, len(result.Nodes))
	}
}

func TestSearchRequiresNameSubstring(t *testing.T) {
	if !containsAll("hgagent-notes.md", []string{"hgagent"}) {
		t.Fatal("expected direct substring match")
	}
	if containsAll("security-report.md", []string{"scty"}) {
		t.Fatal("subsequence-only text must not match")
	}
}

func TestSearchDoesNotMatchDescendantsThroughAncestorPath(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "hgagent", "unrelated.md"))
	writeTestFile(t, filepath.Join(root, "other", "hgagent-guide.md"))
	idx, _, err := BuildIndex(root)
	if err != nil {
		t.Fatal(err)
	}

	result := idx.Query(QueryRequest{Text: "hgagent", Limit: 100})
	names := map[string]RenderNode{}
	for _, node := range result.Nodes {
		names[node.Name] = node
	}
	if result.Matches != 2 {
		t.Fatalf("matches=%d, want folder and file", result.Matches)
	}
	if _, ok := names["unrelated.md"]; ok {
		t.Fatal("unrelated descendant matched through its ancestor path")
	}
	if !names["hgagent"].Matched || !names["hgagent-guide.md"].Matched {
		t.Fatalf("expected terminal matches, got %#v", names)
	}
}

func TestDeepestMatchingNodeWins(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "hgagent", "docs", "hgagent-guide.md"))
	writeTestFile(t, filepath.Join(root, "hgagent", "unrelated.txt"))
	idx, _, err := BuildIndex(root)
	if err != nil {
		t.Fatal(err)
	}

	result := idx.Query(QueryRequest{Text: "hgagent", Limit: 100})
	if result.Matches != 1 {
		t.Fatalf("matches=%d, want deepest match only", result.Matches)
	}
	names := map[string]RenderNode{}
	for _, node := range result.Nodes {
		names[node.Name] = node
	}
	if names["hgagent"].Matched {
		t.Fatal("matching ancestor should be path-only when a deeper node matches")
	}
	if !names["hgagent-guide.md"].Matched {
		t.Fatal("deepest matching file was not marked")
	}
	if _, ok := names["unrelated.txt"]; ok {
		t.Fatal("unrelated sibling leaked into the result")
	}
}

func TestQueryLimitKeepsCompleteLeafPaths(t *testing.T) {
	root := t.TempDir()
	for _, branch := range []string{"alpha", "beta", "gamma"} {
		for i := 0; i < 4; i++ {
			writeTestFile(t, filepath.Join(root, branch, fmt.Sprintf("note-%d.md", i)))
		}
	}
	idx, _, err := BuildIndex(root)
	if err != nil {
		t.Fatal(err)
	}

	result := idx.Query(QueryRequest{Extensions: []string{".md"}, Limit: 5})
	if len(result.Nodes) != 5 {
		t.Fatalf("nodes=%d, want 5", len(result.Nodes))
	}
	found := map[int]RenderNode{}
	for _, node := range result.Nodes {
		found[node.ID] = node
	}
	leaves := 0
	for _, node := range result.Nodes {
		if node.Kind != File {
			continue
		}
		leaves++
		if _, ok := found[node.ParentID]; !ok {
			t.Fatalf("file %q was returned without its parent", node.Name)
		}
	}
	if leaves == 0 {
		t.Fatal("unsafe result contained no file leaves")
	}
	if !result.Truncated {
		t.Fatal("expected truncated result")
	}
	if result.Nodes[0].HiddenMatches == 0 {
		t.Fatal("expected root to report omitted matches")
	}
}

func TestQueryReturnsEveryNodeWhenSafe(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a", "b", "one.md"))
	writeTestFile(t, filepath.Join(root, "c", "two.txt"))
	idx, _, err := BuildIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	result := idx.Query(QueryRequest{Limit: 100})
	if result.Truncated || len(result.Nodes) != len(idx.Nodes) {
		t.Fatalf("safe tree was truncated: nodes=%d index=%d", len(result.Nodes), len(idx.Nodes))
	}
}

func TestSearchLimitNeverHidesEveryMatchedFile(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 20; i++ {
		writeTestFile(t, filepath.Join(root, fmt.Sprintf("branch-%02d", i), "deep", fmt.Sprintf("target-%02d.md", i)))
	}
	idx, _, err := BuildIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	result := idx.Query(QueryRequest{Text: "target", Limit: 8})
	files := 0
	ids := map[int]bool{}
	for _, node := range result.Nodes {
		ids[node.ID] = true
		if node.Kind == File {
			files++
		}
	}
	if files == 0 {
		t.Fatal("search returned directories but no matched files")
	}
	for _, node := range result.Nodes {
		if node.Kind == File && !ids[node.ParentID] {
			t.Fatalf("matched file %q has no visible parent", node.Name)
		}
	}
}

func TestFocusBranchKeepsAncestorsButHidesSiblings(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "keep", "deep", "target.md"))
	writeTestFile(t, filepath.Join(root, "hide", "other.md"))
	idx, _, err := BuildIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	focusID := -1
	for _, node := range idx.Nodes {
		if node.Name == "deep" {
			focusID = node.ID
		}
	}
	result := idx.Query(QueryRequest{RootID: 0, FocusID: focusID, Limit: 100})
	names := map[string]bool{}
	for _, node := range result.Nodes {
		names[node.Name] = true
	}
	for _, name := range []string{filepath.Base(root), "keep", "deep", "target.md"} {
		if !names[name] {
			t.Fatalf("focused result misses %q: %#v", name, names)
		}
	}
	if names["hide"] || names["other.md"] {
		t.Fatalf("focused result retained sibling branch: %#v", names)
	}
}

func TestDirectoryLeafCountsAreKnownBeforeExpansion(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "docs", "a", "one.md"))
	writeTestFile(t, filepath.Join(root, "docs", "b", "two.txt"))
	writeTestFile(t, filepath.Join(root, "docs", "three.pdf"))
	idx, _, err := BuildIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	first := idx.ChildrenOf(ChildrenRequest{ParentID: 0, Limit: 100})
	if len(first.Nodes) != 1 || first.Nodes[0].LeafCount != 3 {
		t.Fatalf("unexpanded directory leaf count = %#v, want 3", first.Nodes)
	}
}

func TestFilteredLeafCountMeansMatchingFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "docs", "one.md"))
	writeTestFile(t, filepath.Join(root, "docs", "two.md"))
	writeTestFile(t, filepath.Join(root, "docs", "other.txt"))
	idx, _, err := BuildIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	result := idx.Query(QueryRequest{Extensions: []string{".md"}, Limit: 100})
	for _, node := range result.Nodes {
		if node.Name == "docs" && node.LeafCount != 2 {
			t.Fatalf("filtered leaf count=%d, want 2", node.LeafCount)
		}
	}
}

func TestChildrenOfLoadsOnlyOneLevel(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "alpha", "deep", "note.md"))
	writeTestFile(t, filepath.Join(root, "beta", "other.txt"))
	idx, _, err := BuildIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	first := idx.ChildrenOf(ChildrenRequest{ParentID: 0, Limit: 100})
	if first.Total != 2 || len(first.Nodes) != 2 {
		t.Fatalf("first level=%d/%d", len(first.Nodes), first.Total)
	}
	for _, node := range first.Nodes {
		if node.Kind != Directory {
			t.Fatalf("unexpected leaf at root: %#v", node)
		}
	}
}

func TestPathToKeepsOnlySelectedLeafChain(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a", "b", "target.md"))
	idx, _, err := BuildIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	leaf := len(idx.Nodes) - 1
	path := idx.PathTo(leaf, 0)
	if len(path) != 4 || path[len(path)-1].Name != "target.md" {
		t.Fatalf("path=%#v", path)
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}
