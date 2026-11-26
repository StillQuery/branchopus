package main

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App owns the native window and the in-memory directory index.
type App struct {
	ctx   context.Context
	mu    sync.RWMutex
	index *TreeIndex
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// ChooseAndScan opens a native folder picker and builds an index in the
// background Wails call. The UI remains responsive while this method runs.
func (a *App) ChooseAndScan() (ScanSummary, error) {
	root, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择要查看的文件夹",
	})
	if err != nil {
		return ScanSummary{}, err
	}
	if root == "" {
		return ScanSummary{Cancelled: true}, nil
	}
	return a.Scan(root)
}

// Scan builds a fresh immutable index, then swaps it in atomically.
func (a *App) Scan(root string) (ScanSummary, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return ScanSummary{}, err
	}
	idx, summary, err := BuildIndex(abs)
	if err != nil {
		return ScanSummary{}, err
	}
	a.mu.Lock()
	a.index = idx
	a.mu.Unlock()
	return summary, nil
}

func (a *App) Query(req QueryRequest) (QueryResult, error) {
	a.mu.RLock()
	idx := a.index
	a.mu.RUnlock()
	if idx == nil {
		return QueryResult{}, errors.New("请先选择文件夹")
	}
	return idx.Query(req), nil
}

func (a *App) RootNode() (RenderNode, error) {
	a.mu.RLock()
	idx := a.index
	a.mu.RUnlock()
	if idx == nil {
		return RenderNode{}, errors.New("请先选择文件夹")
	}
	return idx.RootNode(), nil
}

func (a *App) LoadChildren(req ChildrenRequest) (ChildrenResult, error) {
	a.mu.RLock()
	idx := a.index
	a.mu.RUnlock()
	if idx == nil {
		return ChildrenResult{}, errors.New("请先选择文件夹")
	}
	return idx.ChildrenOf(req), nil
}

func (a *App) NodePath(id int, rootID int) ([]RenderNode, error) {
	a.mu.RLock()
	idx := a.index
	a.mu.RUnlock()
	if idx == nil {
		return nil, errors.New("请先选择文件夹")
	}
	return idx.PathTo(id, rootID), nil
}

func (a *App) OpenPath(path string) error {
	runtime.BrowserOpenURL(a.ctx, fileURL(path))
	return nil
}

func fileURL(path string) string {
	p := filepath.ToSlash(path)
	return (&url.URL{Scheme: "file", Path: "/" + p}).String()
}
