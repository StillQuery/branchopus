package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

const defaultRenderLimit = 1600

type NodeKind string

const (
	Directory NodeKind = "directory"
	File      NodeKind = "file"
)

type indexedNode struct {
	ID        int
	ParentID  int
	Name      string
	Path      string
	SearchKey string
	Extension string
	Kind      NodeKind
	Depth     int
}

type TreeIndex struct {
	Root         string
	Nodes        []indexedNode
	Children     map[int][]int
	SubtreeFiles []int
	ByExt        map[string][]int
	Extensions   []ExtensionCount
}

type ExtensionCount struct {
	Extension string `json:"extension"`
	Count     int    `json:"count"`
}

type ScanSummary struct {
	Cancelled   bool             `json:"cancelled"`
	Root        string           `json:"root"`
	RootName    string           `json:"rootName"`
	Files       int              `json:"files"`
	Directories int              `json:"directories"`
	Skipped     int              `json:"skipped"`
	DurationMS  int64            `json:"durationMs"`
	Extensions  []ExtensionCount `json:"extensions"`
}

type QueryRequest struct {
	Text       string   `json:"text"`
	Extensions []string `json:"extensions"`
	Limit      int      `json:"limit"`
	RootID     int      `json:"rootId"`
	FocusID    int      `json:"focusId"`
}

type RenderNode struct {
	ID            int      `json:"id"`
	ParentID      int      `json:"parentId"`
	Name          string   `json:"name"`
	Path          string   `json:"path"`
	Extension     string   `json:"extension"`
	Kind          NodeKind `json:"kind"`
	Depth         int      `json:"depth"`
	Matched       bool     `json:"matched"`
	HiddenMatches int      `json:"hiddenMatches"`
	LeafCount     int      `json:"leafCount"`
}

type QueryResult struct {
	Nodes     []RenderNode `json:"nodes"`
	Matches   int          `json:"matches"`
	Truncated bool         `json:"truncated"`
	QueryMS   int64        `json:"queryMs"`
}

func BuildIndex(root string) (*TreeIndex, ScanSummary, error) {
	started := time.Now()
	info, err := os.Stat(root)
	if err != nil {
		return nil, ScanSummary{}, err
	}
	if !info.IsDir() {
		return nil, ScanSummary{}, errors.New("所选路径不是文件夹")
	}

	root = filepath.Clean(root)
	idx := &TreeIndex{Root: root, ByExt: make(map[string][]int), Children: make(map[int][]int)}
	idx.Nodes = append(idx.Nodes, indexedNode{
		ID: 0, ParentID: -1, Name: filepath.Base(root), Path: root,
		SearchKey: normalize(filepath.Base(root)), Kind: Directory,
	})
	pathIDs := map[string]int{root: 0}
	files, dirs, skipped := 0, 1, 0

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			skipped++
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		name := entry.Name()
		// Dot-directories are treated as hidden. Skipping them early prevents
		// expensive scans of .git and tool caches.
		if strings.HasPrefix(name, ".") {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		parentPath := filepath.Dir(path)
		parentID, ok := pathIDs[parentPath]
		if !ok {
			return nil
		}
		n := indexedNode{
			ID: len(idx.Nodes), ParentID: parentID, Name: name, Path: path,
			// SearchKey intentionally contains only this node's name. Ancestor
			// paths are added after a match; putting the relative path here makes
			// every descendant look like a direct match when a folder matches.
			SearchKey: normalize(name), Depth: idx.Nodes[parentID].Depth + 1,
		}
		if entry.IsDir() {
			n.Kind = Directory
			dirs++
			pathIDs[path] = n.ID
		} else {
			n.Kind = File
			n.Extension = strings.ToLower(filepath.Ext(name))
			if n.Extension == "" {
				n.Extension = "无扩展名"
			}
			files++
			idx.ByExt[n.Extension] = append(idx.ByExt[n.Extension], n.ID)
		}
		idx.Nodes = append(idx.Nodes, n)
		idx.Children[n.ParentID] = append(idx.Children[n.ParentID], n.ID)
		return nil
	})
	if err != nil {
		return nil, ScanSummary{}, err
	}
	idx.SubtreeFiles = make([]int, len(idx.Nodes))
	for id := len(idx.Nodes) - 1; id >= 0; id-- {
		if idx.Nodes[id].Kind == File {
			idx.SubtreeFiles[id] = 1
		}
		if parent := idx.Nodes[id].ParentID; parent >= 0 {
			idx.SubtreeFiles[parent] += idx.SubtreeFiles[id]
		}
	}

	for ext, ids := range idx.ByExt {
		idx.Extensions = append(idx.Extensions, ExtensionCount{Extension: ext, Count: len(ids)})
	}
	sort.Slice(idx.Extensions, func(i, j int) bool {
		if idx.Extensions[i].Count == idx.Extensions[j].Count {
			return idx.Extensions[i].Extension < idx.Extensions[j].Extension
		}
		return idx.Extensions[i].Count > idx.Extensions[j].Count
	})
	return idx, ScanSummary{
		Root: root, RootName: filepath.Base(root), Files: files, Directories: dirs,
		Skipped: skipped, DurationMS: time.Since(started).Milliseconds(), Extensions: idx.Extensions,
	}, nil
}

type ChildrenRequest struct {
	ParentID   int      `json:"parentId"`
	Text       string   `json:"text"`
	Extensions []string `json:"extensions"`
	Offset     int      `json:"offset"`
	Limit      int      `json:"limit"`
}

type ChildrenResult struct {
	Nodes   []RenderNode `json:"nodes"`
	Total   int          `json:"total"`
	HasMore bool         `json:"hasMore"`
}

// Children returns one directory level from the in-memory index. A filtered
// result contains only terminal matches and the branches leading to them.
func (idx *TreeIndex) ChildrenOf(req ChildrenRequest) ChildrenResult {
	if req.ParentID < 0 || req.ParentID >= len(idx.Nodes) || idx.Nodes[req.ParentID].Kind != Directory {
		return ChildrenResult{Nodes: []RenderNode{}}
	}
	limit := req.Limit
	if limit <= 0 || limit > 2000 {
		limit = 400
	}
	tokens := tokenize(req.Text)
	exts := make(map[string]struct{}, len(req.Extensions))
	for _, ext := range req.Extensions {
		exts[strings.ToLower(ext)] = struct{}{}
	}
	filtered := len(tokens) > 0 || len(exts) > 0
	directMatches := make([]bool, len(idx.Nodes))
	subtreeDirectMatches := make([]int, len(idx.Nodes))
	terminalMatches := make([]bool, len(idx.Nodes))
	subtreeTerminalMatches := make([]int, len(idx.Nodes))
	matchingFiles := make([]int, len(idx.Nodes))
	if filtered {
		parentDepth := idx.Nodes[req.ParentID].Depth
		for id := req.ParentID + 1; id < len(idx.Nodes) && idx.Nodes[id].Depth > parentDepth; id++ {
			directMatches[id] = nodeMatches(idx.Nodes[id], tokens, exts)
		}
		for id := len(idx.Nodes) - 1; id > req.ParentID; id-- {
			if directMatches[id] {
				subtreeDirectMatches[id]++
			}
			if parent := idx.Nodes[id].ParentID; parent >= req.ParentID {
				subtreeDirectMatches[parent] += subtreeDirectMatches[id]
			}
		}
		for id := req.ParentID + 1; id < len(idx.Nodes) && idx.Nodes[id].Depth > parentDepth; id++ {
			terminalMatches[id] = directMatches[id] && subtreeDirectMatches[id] == 1
			if terminalMatches[id] {
				subtreeTerminalMatches[id] = 1
				if idx.Nodes[id].Kind == File {
					matchingFiles[id] = 1
				}
			}
		}
		for id := len(idx.Nodes) - 1; id > req.ParentID; id-- {
			if parent := idx.Nodes[id].ParentID; parent >= req.ParentID {
				subtreeTerminalMatches[parent] += subtreeTerminalMatches[id]
				matchingFiles[parent] += matchingFiles[id]
			}
		}
	}
	eligible := make([]int, 0, len(idx.Children[req.ParentID]))
	for _, id := range idx.Children[req.ParentID] {
		if !filtered || subtreeTerminalMatches[id] > 0 {
			eligible = append(eligible, id)
		}
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		a, b := idx.Nodes[eligible[i]], idx.Nodes[eligible[j]]
		if a.Kind != b.Kind {
			return a.Kind == Directory
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	start := max(0, min(req.Offset, len(eligible)))
	end := min(start+limit, len(eligible))
	result := ChildrenResult{Total: len(eligible), HasMore: end < len(eligible), Nodes: make([]RenderNode, 0, end-start)}
	for _, id := range eligible[start:end] {
		n := idx.Nodes[id]
		leafCount := matchingFiles[id]
		if !filtered && id < len(idx.SubtreeFiles) {
			leafCount = idx.SubtreeFiles[id]
		}
		result.Nodes = append(result.Nodes, RenderNode{ID: n.ID, ParentID: n.ParentID, Name: n.Name, Path: n.Path, Extension: n.Extension, Kind: n.Kind, Depth: n.Depth, Matched: filtered && terminalMatches[id], LeafCount: leafCount})
	}
	return result
}

func (idx *TreeIndex) RootNode() RenderNode { return idx.renderNode(0) }

func (idx *TreeIndex) PathTo(id, rootID int) []RenderNode {
	if id < 0 || id >= len(idx.Nodes) {
		return []RenderNode{}
	}
	if rootID < 0 || rootID >= len(idx.Nodes) {
		rootID = 0
	}
	ids := []int{}
	for current := id; current >= 0; current = idx.Nodes[current].ParentID {
		ids = append(ids, current)
		if current == rootID {
			break
		}
	}
	if ids[len(ids)-1] != rootID {
		return []RenderNode{}
	}
	result := make([]RenderNode, 0, len(ids))
	for i := len(ids) - 1; i >= 0; i-- {
		result = append(result, idx.renderNode(ids[i]))
	}
	return result
}

func (idx *TreeIndex) renderNode(id int) RenderNode {
	n := idx.Nodes[id]
	leafCount := 0
	if id < len(idx.SubtreeFiles) {
		leafCount = idx.SubtreeFiles[id]
	}
	return RenderNode{ID: n.ID, ParentID: n.ParentID, Name: n.Name, Path: n.Path, Extension: n.Extension, Kind: n.Kind, Depth: n.Depth, LeafCount: leafCount}
}

func (idx *TreeIndex) Query(req QueryRequest) QueryResult {
	started := time.Now()
	limit := req.Limit
	if limit <= 0 || limit > 5000 {
		limit = defaultRenderLimit
	}
	tokens := tokenize(req.Text)
	rootID := req.RootID
	if rootID < 0 || rootID >= len(idx.Nodes) || idx.Nodes[rootID].Kind != Directory {
		rootID = 0
	}
	focusID := req.FocusID
	if focusID < 0 || focusID >= len(idx.Nodes) || !idx.isDescendantOrSelf(focusID, rootID) {
		focusID = rootID
	}
	extSet := make(map[string]struct{}, len(req.Extensions))
	for _, ext := range req.Extensions {
		extSet[strings.ToLower(ext)] = struct{}{}
	}

	rawMatched := make([]int, 0, min(limit, len(idx.Nodes)))
	candidates := make([]int, 0, len(idx.Nodes)-1)
	if len(tokens) == 0 && len(extSet) > 0 {
		for ext := range extSet {
			candidates = append(candidates, idx.ByExt[ext]...)
		}
		sort.Ints(candidates)
	} else {
		for i := 1; i < len(idx.Nodes); i++ {
			candidates = append(candidates, i)
		}
	}
	for _, i := range candidates {
		if !idx.isDescendantOrSelf(i, focusID) || i == focusID {
			continue
		}
		n := &idx.Nodes[i]
		if !nodeMatches(*n, tokens, extSet) {
			continue
		}
		rawMatched = append(rawMatched, i)
	}
	if len(tokens) == 0 && len(extSet) == 0 {
		rawMatched = rawMatched[:0]
		if focusID != rootID {
			rawMatched = append(rawMatched, focusID)
		}
		for i := focusID + 1; i < len(idx.Nodes) && idx.Nodes[i].Depth > idx.Nodes[focusID].Depth; i++ {
			rawMatched = append(rawMatched, i)
		}
	}

	matched := rawMatched
	if len(tokens) > 0 || len(extSet) > 0 {
		direct := make([]bool, len(idx.Nodes))
		subtreeDirect := make([]int, len(idx.Nodes))
		for _, id := range rawMatched {
			direct[id] = true
		}
		for id := len(idx.Nodes) - 1; id >= 0; id-- {
			if direct[id] {
				subtreeDirect[id]++
			}
			if parent := idx.Nodes[id].ParentID; parent >= 0 {
				subtreeDirect[parent] += subtreeDirect[id]
			}
		}
		matched = make([]int, 0, len(rawMatched))
		for _, id := range rawMatched {
			// If a matching directory contains a deeper match, the deeper node is
			// the result and this directory remains visible only as its path.
			if subtreeDirect[id] == 1 {
				matched = append(matched, id)
			}
		}
	}

	totalMatches := len(matched)
	highlightMatches := len(tokens) > 0 || len(extSet) > 0
	needed := make([]bool, len(idx.Nodes))
	directMatch := make([]bool, len(idx.Nodes))
	subtreeMatches := make([]int, len(idx.Nodes))
	matchingFiles := make([]int, len(idx.Nodes))
	needed[rootID] = true
	for _, id := range matched {
		directMatch[id] = true
		for current := id; current >= 0; current = idx.Nodes[current].ParentID {
			needed[current] = true
			subtreeMatches[current]++
			if current == rootID {
				break
			}
		}
		if idx.Nodes[id].Kind == File {
			for current := id; current >= 0; current = idx.Nodes[current].ParentID {
				matchingFiles[current]++
				if current == rootID {
					break
				}
			}
		}
	}

	// Build the required child lists once. Directories and files are kept
	// separate so the hierarchy always receives the render budget first.
	directoryChildren := make(map[int][]int)
	fileChildren := make(map[int][]int)
	neededCount := 0
	for id, n := range idx.Nodes {
		if !needed[id] {
			continue
		}
		neededCount++
		if id == rootID {
			continue
		}
		if n.Kind == Directory {
			directoryChildren[n.ParentID] = append(directoryChildren[n.ParentID], id)
		} else if directMatch[id] {
			fileChildren[n.ParentID] = append(fileChildren[n.ParentID], id)
		}
	}

	keep := map[int]bool{rootID: true}
	if neededCount <= limit {
		for id, value := range needed {
			if value {
				keep[id] = true
			}
		}
	} else {
		// When the complete result is unsafe to draw, admit only complete
		// root-to-match paths. Files come first; each top-level branch gets a
		// turn so a dense early branch cannot consume the whole canvas.
		buckets := make(map[int][]int)
		branchOrder := []int{}
		for _, wantKind := range []NodeKind{File, Directory} {
			for _, id := range matched {
				if idx.Nodes[id].Kind != wantKind {
					continue
				}
				branch := idx.topBranch(id, rootID)
				if _, ok := buckets[branch]; !ok {
					branchOrder = append(branchOrder, branch)
				}
				buckets[branch] = append(buckets[branch], id)
			}
		}
		for position := 0; len(keep) < limit; position++ {
			progress := false
			for _, branch := range branchOrder {
				items := buckets[branch]
				if position >= len(items) {
					continue
				}
				path := idx.missingPath(items[position], rootID, keep)
				if len(keep)+len(path) > limit {
					continue
				}
				for _, id := range path {
					keep[id] = true
				}
				progress = true
			}
			if !progress {
				break
			}
		}
	}

	// Count materialised matches under each visible directory. The difference
	// becomes a compact +N marker instead of silently deleting a branch.
	includedSubtreeMatches := make([]int, len(idx.Nodes))
	for id := range keep {
		if !directMatch[id] {
			continue
		}
		for current := id; current >= 0; current = idx.Nodes[current].ParentID {
			includedSubtreeMatches[current]++
			if current == rootID {
				break
			}
		}
	}
	truncated := len(keep) < neededCount

	result := QueryResult{Matches: totalMatches, Truncated: truncated}
	result.Nodes = make([]RenderNode, 0, len(keep))
	// Index order is parent-before-child, which the renderer can consume directly.
	for id, n := range idx.Nodes {
		if !keep[id] {
			continue
		}
		parentID := n.ParentID
		if id == rootID {
			parentID = -1
		}
		leafCount := matchingFiles[id]
		if !highlightMatches && id < len(idx.SubtreeFiles) {
			leafCount = idx.SubtreeFiles[id]
		}
		result.Nodes = append(result.Nodes, RenderNode{
			ID: n.ID, ParentID: parentID, Name: n.Name, Path: n.Path,
			Extension: n.Extension, Kind: n.Kind, Depth: n.Depth,
			Matched:       highlightMatches && directMatch[id],
			HiddenMatches: max(0, subtreeMatches[id]-includedSubtreeMatches[id]),
			LeafCount:     leafCount,
		})
	}
	result.QueryMS = time.Since(started).Milliseconds()
	return result
}

func (idx *TreeIndex) isDescendantOrSelf(id, rootID int) bool {
	for current := id; current >= 0; current = idx.Nodes[current].ParentID {
		if current == rootID {
			return true
		}
	}
	return false
}

func (idx *TreeIndex) topBranch(id, rootID int) int {
	current := id
	for idx.Nodes[current].ParentID >= 0 && idx.Nodes[current].ParentID != rootID {
		current = idx.Nodes[current].ParentID
	}
	return current
}

func (idx *TreeIndex) missingPath(id, rootID int, keep map[int]bool) []int {
	reversed := []int{}
	for current := id; current >= 0 && !keep[current]; current = idx.Nodes[current].ParentID {
		reversed = append(reversed, current)
		if current == rootID {
			break
		}
	}
	path := make([]int, 0, len(reversed))
	for i := len(reversed) - 1; i >= 0; i-- {
		path = append(path, reversed[i])
	}
	return path
}

func normalize(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func tokenize(s string) []string {
	return strings.FieldsFunc(normalize(s), func(r rune) bool { return unicode.IsSpace(r) })
}

func containsAll(text string, tokens []string) bool {
	for _, token := range tokens {
		if !strings.Contains(text, token) {
			return false
		}
	}
	return true
}

func nodeMatches(node indexedNode, tokens []string, extensions map[string]struct{}) bool {
	if node.Kind == Directory {
		return len(tokens) > 0 && containsAll(node.SearchKey, tokens)
	}
	if len(extensions) > 0 {
		if _, ok := extensions[node.Extension]; !ok {
			return false
		}
	}
	return len(tokens) == 0 || containsAll(node.SearchKey, tokens)
}
